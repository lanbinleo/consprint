package backend

import (
	"time"

	"github.com/gin-gonic/gin"
)

func (a *App) dashboard(c *gin.Context) {
	summary := a.dashboardSummaryPayload(c.GetString("userID"))
	progress := a.dashboardProgressPayload(c.GetString("userID"))
	trends := a.dashboardTrendsPayload(c.GetString("userID"))
	alerts := a.dashboardAlertsPayload(c.GetString("userID"))
	c.JSON(200, gin.H{
		"totalConcepts":    summary["totalConcepts"],
		"readyConcepts":    summary["readyConcepts"],
		"reviewedConcepts": progress["reviewedConcepts"],
		"ratedConcepts":    progress["ratedConcepts"],
		"weakConcepts":     progress["weakConcepts"],
		"averageMastery":   progress["averageMastery"],
		"todayReviews":     progress["todayReviews"],
		"todayMasteryGain": progress["todayMasteryGain"],
		"shortTermReviews": progress["shortTermReviews"],
		"streakDays":       progress["streakDays"],
		"recent":           alerts["recent"],
		"weakUnits":        alerts["weakUnits"],
		"weakTopics":       alerts["weakTopics"],
		"weakConceptsList": alerts["weakConcepts"],
		"daily":            trends["daily"],
		"hourly":           trends["hourly"],
	})
}

func (a *App) dashboardSummary(c *gin.Context) {
	c.JSON(200, a.dashboardSummaryPayload(c.GetString("userID")))
}

func (a *App) dashboardProgress(c *gin.Context) {
	c.JSON(200, a.dashboardProgressPayload(c.GetString("userID")))
}

func (a *App) dashboardTrends(c *gin.Context) {
	c.JSON(200, a.dashboardTrendsPayload(c.GetString("userID")))
}

func (a *App) dashboardAlerts(c *gin.Context) {
	c.JSON(200, a.dashboardAlertsPayload(c.GetString("userID")))
}

func (a *App) dashboardSummaryPayload(userID string) gin.H {
	a.ensureStates(userID)
	var total, ready int64
	a.DB.Model(&Concept{}).Count(&total)
	a.DB.Model(&Concept{}).Where("content_status <> ?", "pending").Count(&ready)
	return gin.H{"totalConcepts": total, "readyConcepts": ready}
}

func (a *App) dashboardProgressPayload(userID string) gin.H {
	a.ensureStates(userID)
	var reviewed, weak, rated int64
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND review_count > 0", userID).Count(&reviewed)
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND mastery > 0", userID).Count(&rated)
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND mastery > 0 AND mastery < ?", userID, 3).Count(&weak)
	var shortTerm int64
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND short_term_review = ?", userID, true).Count(&shortTerm)
	type Avg struct{ Avg float64 }
	var avg Avg
	a.DB.Raw("select coalesce(avg(mastery), 0) as avg from user_concept_states where user_id = ?", userID).Scan(&avg)
	today := todayStats(a.reviewEvents(userID))
	return gin.H{
		"reviewedConcepts": reviewed,
		"ratedConcepts":    rated,
		"weakConcepts":     weak,
		"averageMastery":   avg.Avg,
		"todayReviews":     today.Reviews,
		"todayMasteryGain": today.Gain,
		"shortTermReviews": shortTerm,
		"streakDays":       a.streakDays(userID),
	}
}

func (a *App) dashboardTrendsPayload(userID string) gin.H {
	return gin.H{"daily": a.dailyStats(userID), "hourly": a.hourlyStats(userID)}
}

func (a *App) dashboardAlertsPayload(userID string) gin.H {
	a.ensureStates(userID)
	var recent []ReviewEvent
	a.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(8).Find(&recent)
	var weak []Concept
	a.DB.Model(&Concept{}).
		Select("concepts.*").
		Joins("join user_concept_states s on s.concept_id = concepts.id and s.user_id = ?", userID).
		Where("s.mastery > 0 and s.mastery < ?", 3).
		Order("s.mastery asc, s.updated_at desc").
		Limit(6).
		Find(&weak)
	return gin.H{"recent": recent, "weakConcepts": weak, "weakUnits": a.weakUnitStats(userID), "weakTopics": a.weakTopicStats(userID)}
}

type statBucket struct {
	Label          string  `json:"label"`
	Reviews        int     `json:"reviews"`
	Learned        int     `json:"learned"`
	MasteryGain    float64 `json:"masteryGain"`
	AverageMastery float64 `json:"averageMastery"`
}

func (a *App) reviewEvents(userID string) []ReviewEvent {
	events := make([]ReviewEvent, 0)
	a.DB.Where("user_id = ?", userID).Order("created_at asc").Find(&events)
	return events
}

func appTimeLocation() *time.Location {
	if envBool("APP_USE_SYSTEM_TIMEZONE", false) {
		return time.Local
	}
	name := env("APP_TIMEZONE", "Asia/Shanghai")
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}

func appNow() time.Time {
	return time.Now().In(appTimeLocation())
}

func dayStart(t time.Time) time.Time {
	loc := appTimeLocation()
	inLoc := t.In(loc)
	return time.Date(inLoc.Year(), inLoc.Month(), inLoc.Day(), 0, 0, 0, 0, loc)
}

func todayStats(events []ReviewEvent) struct {
	Reviews int
	Gain    float64
} {
	start := dayStart(appNow())
	end := start.AddDate(0, 0, 1)
	stats := struct {
		Reviews int
		Gain    float64
	}{}
	for _, event := range events {
		at := event.CreatedAt.In(appTimeLocation())
		if at.Before(start) || !at.Before(end) {
			continue
		}
		stats.Reviews++
		if event.MasteryAfter > event.MasteryBefore {
			stats.Gain += event.MasteryAfter - event.MasteryBefore
		}
	}
	return stats
}

func (a *App) dailyStats(userID string) []statBucket {
	rows := make([]statBucket, 0, 14)
	start := dayStart(appNow()).AddDate(0, 0, -13)
	for i := 0; i < 14; i++ {
		label := start.AddDate(0, 0, i).Format("2006-01-02")
		rows = append(rows, statBucket{Label: label})
	}
	indexByLabel := make(map[string]int, len(rows))
	for i, row := range rows {
		indexByLabel[row.Label] = i
	}
	averageCounts := make([]int, len(rows))
	for _, event := range a.reviewEvents(userID) {
		at := event.CreatedAt.In(appTimeLocation())
		if at.Before(start) {
			continue
		}
		label := at.Format("2006-01-02")
		index, ok := indexByLabel[label]
		if !ok {
			continue
		}
		addEventToBucket(&rows[index], event)
		averageCounts[index]++
	}
	for i := range rows {
		if averageCounts[i] > 0 {
			rows[i].AverageMastery /= float64(averageCounts[i])
		}
	}
	return rows
}

func (a *App) hourlyStats(userID string) []statBucket {
	rows := make([]statBucket, 0, 24)
	start := appNow().Truncate(time.Hour).Add(-23 * time.Hour)
	for i := 0; i < 24; i++ {
		hour := start.Add(time.Duration(i) * time.Hour)
		rows = append(rows, statBucket{Label: hour.Format("15:00")})
	}
	indexByLabel := make(map[string]int, len(rows))
	for i := range rows {
		indexByLabel[start.Add(time.Duration(i)*time.Hour).Format("2006-01-02 15:00")] = i
	}
	averageCounts := make([]int, len(rows))
	for _, event := range a.reviewEvents(userID) {
		at := event.CreatedAt.In(appTimeLocation()).Truncate(time.Hour)
		if at.Before(start) {
			continue
		}
		index, ok := indexByLabel[at.Format("2006-01-02 15:00")]
		if !ok {
			continue
		}
		addEventToBucket(&rows[index], event)
		averageCounts[index]++
	}
	for i := range rows {
		if averageCounts[i] > 0 {
			rows[i].AverageMastery /= float64(averageCounts[i])
		}
	}
	return rows
}

func addEventToBucket(bucket *statBucket, event ReviewEvent) {
	bucket.Reviews++
	if event.MasteryBefore < 4 && event.MasteryAfter >= 4 {
		bucket.Learned++
	}
	if event.MasteryAfter > event.MasteryBefore {
		bucket.MasteryGain += event.MasteryAfter - event.MasteryBefore
	}
	bucket.AverageMastery += event.MasteryAfter
}

type weakArea struct {
	Label          string  `json:"label"`
	Weak           int     `json:"weak"`
	AverageMastery float64 `json:"averageMastery"`
}

func (a *App) weakUnitStats(userID string) []weakArea {
	rows := make([]weakArea, 0)
	a.DB.Raw(`
		select u.title as label,
		       sum(case when s.mastery > 0 and s.mastery < 3 then 1 else 0 end) as weak,
		       coalesce(avg(case when s.mastery > 0 then s.mastery end), 0) as average_mastery
		from units u
		join concepts c on c.unit_id = u.id
		join user_concept_states s on s.concept_id = c.id and s.user_id = ?
		group by u.id, u.title
		having weak > 0
		order by weak desc, average_mastery asc
		limit 5
	`, userID).Scan(&rows)
	return rows
}

func (a *App) weakTopicStats(userID string) []weakArea {
	rows := make([]weakArea, 0)
	a.DB.Raw(`
		select t.title as label,
		       sum(case when s.mastery > 0 and s.mastery < 3 then 1 else 0 end) as weak,
		       coalesce(avg(case when s.mastery > 0 then s.mastery end), 0) as average_mastery
		from topics t
		join concepts c on c.topic_id = t.id
		join user_concept_states s on s.concept_id = c.id and s.user_id = ?
		group by t.id, t.title
		having weak > 0
		order by weak desc, average_mastery asc
		limit 5
	`, userID).Scan(&rows)
	return rows
}

func (a *App) streakDays(userID string) int {
	days := make(map[string]bool)
	for _, event := range a.reviewEvents(userID) {
		days[event.CreatedAt.In(appTimeLocation()).Format("2006-01-02")] = true
	}
	streak := 0
	for day := appNow(); ; day = day.AddDate(0, 0, -1) {
		if !days[day.Format("2006-01-02")] {
			break
		}
		streak++
	}
	return streak
}
