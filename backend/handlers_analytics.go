package backend

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Class-wide learning analytics for staff (teacher + admin).

type analyticsAccuracy struct {
	Label    string  `json:"label"`
	ID       string  `json:"id"`
	Answered int     `json:"answered"`
	Correct  int     `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

func finalizeAccuracy(rows []analyticsAccuracy) []analyticsAccuracy {
	for i := range rows {
		if rows[i].Answered > 0 {
			rows[i].Accuracy = float64(rows[i].Correct) / float64(rows[i].Answered)
		}
	}
	return rows
}

func (a *App) analyticsOverview(c *gin.Context) {
	weekAgo := time.Now().AddDate(0, 0, -7)

	var roles []struct {
		Role  string `json:"role"`
		Count int    `json:"count"`
	}
	a.DB.Raw(`select role, count(*) as count from users group by role`).Scan(&roles)

	var active7 int64
	a.DB.Raw(`
		select count(distinct user_id) from (
			select user_id from review_events where created_at > ?
			union
			select user_id from practice_attempts where started_at > ?
		)
	`, weekAgo, weekAgo).Scan(&active7)

	var totals struct {
		Attempts int `json:"attempts"`
	}
	a.DB.Raw(`select count(*) as attempts from practice_attempts`).Scan(&totals)
	var answered struct {
		Answered int `json:"answered"`
		Correct  int `json:"correct"`
	}
	a.DB.Raw(`select count(*) as answered, coalesce(sum(case when is_correct then 1 else 0 end), 0) as correct
		from practice_answers where is_correct is not null`).Scan(&answered)

	var byUnit []analyticsAccuracy
	a.DB.Raw(`
		select coalesce(u.title, 'Unlinked') as label, coalesce(u.id, '') as id,
		       count(*) as answered,
		       coalesce(sum(case when pa.is_correct then 1 else 0 end), 0) as correct
		from practice_answers pa
		join questions q on q.id = pa.question_id
		left join units u on u.id = q.unit_id
		where pa.is_correct is not null
		group by u.id, u.title
		order by answered desc
	`).Scan(&byUnit)

	var flashcard []struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	a.DB.Raw(`select status, count(*) as count from user_concept_states group by status`).Scan(&flashcard)

	var weakConcepts []struct {
		Term string `json:"term"`
		Unit string `json:"unit"`
		Weak int    `json:"weak"`
	}
	a.DB.Raw(`
		select c.term as term, u.title as unit,
		       sum(case when s.status in ('fuzzy', 'unknown') then 1 else 0 end) as weak
		from concepts c
		join units u on u.id = c.unit_id
		join user_concept_states s on s.concept_id = c.id
		group by c.id, c.term, u.title
		having weak > 0
		order by weak desc
		limit 10
	`).Scan(&weakConcepts)

	// Daily activity (flashcard reviews + practice starts) across the class.
	type dayBucket struct {
		Label    string `json:"label"`
		Reviews  int    `json:"reviews"`
		Attempts int    `json:"attempts"`
	}
	days := make([]dayBucket, 0, 14)
	start := dayStart(appNow()).AddDate(0, 0, -13)
	for i := 0; i < 14; i++ {
		days = append(days, dayBucket{Label: start.AddDate(0, 0, i).Format("2006-01-02")})
	}
	indexByLabel := map[string]int{}
	for i, day := range days {
		indexByLabel[day.Label] = i
	}
	var events []ReviewEvent
	a.DB.Where("created_at > ?", start).Find(&events)
	for _, event := range events {
		if index, ok := indexByLabel[event.CreatedAt.In(appTimeLocation()).Format("2006-01-02")]; ok {
			days[index].Reviews++
		}
	}
	var attemptRows []PracticeAttempt
	a.DB.Where("started_at > ?", start).Find(&attemptRows)
	for _, attempt := range attemptRows {
		if index, ok := indexByLabel[attempt.StartedAt.In(appTimeLocation()).Format("2006-01-02")]; ok {
			days[index].Attempts++
		}
	}

	var students []struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Email    string  `json:"email"`
		Role     string  `json:"role"`
		Attempts int     `json:"attempts"`
		Answered int     `json:"answered"`
		Correct  int     `json:"correct"`
		Marked   int     `json:"marked"`
		Accuracy float64 `json:"accuracy"`
	}
	a.DB.Raw(`
		select us.id, us.name, us.email, us.role,
		       (select count(*) from practice_attempts pa where pa.user_id = us.id) as attempts,
		       (select count(*) from practice_answers pan join practice_attempts pa2 on pa2.id = pan.attempt_id
		        where pa2.user_id = us.id and pan.is_correct is not null) as answered,
		       (select count(*) from practice_answers pan join practice_attempts pa2 on pa2.id = pan.attempt_id
		        where pa2.user_id = us.id and pan.is_correct = 1) as correct,
		       (select count(*) from user_concept_states s where s.user_id = us.id and s.status <> '') as marked,
		       0.0 as accuracy
		from users us
		order by us.created_at asc
	`).Scan(&students)
	for i := range students {
		if students[i].Answered > 0 {
			students[i].Accuracy = float64(students[i].Correct) / float64(students[i].Answered)
		}
	}

	c.JSON(200, gin.H{
		"roles":         roles,
		"activeUsers7d": active7,
		"practice": gin.H{
			"attempts": totals.Attempts,
			"answered": answered.Answered,
			"correct":  answered.Correct,
			"accuracy": float64(answered.Correct) / float64(maxInt(answered.Answered, 1)),
		},
		"accuracyByUnit": finalizeAccuracy(byUnit),
		"flashcard":      flashcard,
		"weakConcepts":   weakConcepts,
		"daily":          days,
		"students":       students,
	})
}

func (a *App) analyticsUserDetail(c *gin.Context) {
	var user User
	if err := a.DB.First(&user, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	userID := user.ID

	var flashcard []struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	a.DB.Raw(`select status, count(*) as count from user_concept_states where user_id = ? group by status`, userID).Scan(&flashcard)

	var recent []ReviewEvent
	a.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(20).Find(&recent)

	var byUnit []analyticsAccuracy
	a.DB.Raw(`
		select coalesce(u.title, 'Unlinked') as label, coalesce(u.id, '') as id,
		       count(*) as answered,
		       coalesce(sum(case when pa.is_correct then 1 else 0 end), 0) as correct
		from practice_answers pa
		join practice_attempts at on at.id = pa.attempt_id and at.user_id = ?
		join questions q on q.id = pa.question_id
		left join units u on u.id = q.unit_id
		where pa.is_correct is not null
		group by u.id, u.title
		order by answered desc
	`, userID).Scan(&byUnit)

	type attemptRow struct {
		PracticeAttempt
		SetTitle string `json:"setTitle"`
	}
	var attempts []PracticeAttempt
	a.DB.Where("user_id = ?", userID).Order("started_at desc").Limit(50).Find(&attempts)
	setIDs := map[string]bool{}
	for _, attempt := range attempts {
		setIDs[attempt.SetID] = true
	}
	titles := map[string]string{}
	if len(setIDs) > 0 {
		ids := make([]string, 0, len(setIDs))
		for id := range setIDs {
			ids = append(ids, id)
		}
		var sets []PracticeSet
		a.DB.Where("id in ?", ids).Find(&sets)
		for _, set := range sets {
			titles[set.ID] = set.Title
		}
	}
	attemptRows := make([]attemptRow, 0, len(attempts))
	for _, attempt := range attempts {
		attemptRows = append(attemptRows, attemptRow{PracticeAttempt: attempt, SetTitle: titles[attempt.SetID]})
	}

	var wrongTotal struct {
		Count int `json:"count"`
	}
	a.DB.Raw(`
		select count(*) as count from practice_answers pa
		join practice_attempts at on at.id = pa.attempt_id and at.user_id = ?
		where pa.is_correct = 0
	`, userID).Scan(&wrongTotal)

	c.JSON(200, gin.H{
		"user":           user,
		"flashcard":      flashcard,
		"streakDays":     a.streakDays(userID),
		"recent":         recent,
		"attempts":       attemptRows,
		"accuracyByUnit": finalizeAccuracy(byUnit),
		"wrongAnswers":   wrongTotal.Count,
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
