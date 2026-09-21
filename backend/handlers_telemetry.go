package backend

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Telemetry aggregation endpoints for the admin activity panels. Aggregates
// are staff-visible (like /admin/analytics); the raw log endpoint is
// admin-only. All day/hour bucketing happens in SQL via strftime with the
// APP_TIMEZONE modifier so buckets match the school's local calendar, and all
// aggregates are scoped to students (staff heartbeats are excluded).

const studentScopeJoin = "join users u on u.id = %s.user_id and u.role = 'student' and u.deleted_at is null"

// telemetryWindow validates ?days= (7/14/30/90, default 30) and returns the
// inclusive local-day window start.
func telemetryWindow(c *gin.Context) (int, time.Time) {
	days := 30
	switch c.Query("days") {
	case "7", "14", "30", "90":
		days, _ = strconv.Atoi(c.Query("days"))
	}
	return days, dayStart(appNow()).AddDate(0, 0, -(days - 1))
}

func telemetryDates(start time.Time, days int) []string {
	out := make([]string, 0, days)
	for i := 0; i < days; i++ {
		out = append(out, start.AddDate(0, 0, i).Format("2006-01-02"))
	}
	return out
}

func (a *App) telemetryActivity(c *gin.Context) {
	days, start := telemetryWindow(c)
	end := start.AddDate(0, 0, days)
	join := fmt.Sprintf(studentScopeJoin, "e")

	type dayRow struct {
		Day    string
		Logins int64
		DAU    int64 `gorm:"column:dau"`
	}
	var daily []dayRow
	a.DB.Raw(`select strftime('%Y-%m-%d', e.created_at, ?) as day,
		sum(case when e.type = 'login' and e.name = 'login.success' then 1 else 0 end) as logins,
		count(distinct e.user_id) as dau
		from activity_events e `+join+`
		where e.created_at >= ? and e.created_at < ?
		group by day`, tzModifier(), start, end).Scan(&daily)
	byDay := make(map[string]dayRow, len(daily))
	for _, row := range daily {
		byDay[row.Day] = row
	}
	dailyOut := make([]gin.H, 0, days)
	for _, date := range telemetryDates(start, days) {
		row := byDay[date]
		dailyOut = append(dailyOut, gin.H{"date": date, "logins": row.Logins, "dau": row.DAU})
	}

	type hourRow struct {
		Hour   int
		Events int64
	}
	var byHour []hourRow
	a.DB.Raw(`select cast(strftime('%H', e.created_at, ?) as integer) as hour, count(*) as events
		from activity_events e `+join+`
		where e.created_at >= ? and e.created_at < ? and e.type in ('heartbeat','page_view','feature')
		group by hour`, tzModifier(), start, end).Scan(&byHour)
	hourMap := make(map[int]int64, len(byHour))
	for _, row := range byHour {
		hourMap[row.Hour] = row.Events
	}
	hoursOut := make([]gin.H, 0, 24)
	for h := 0; h < 24; h++ {
		hoursOut = append(hoursOut, gin.H{"hour": h, "events": hourMap[h]})
	}

	type providerRow struct {
		Provider string `json:"provider"`
		Count    int64  `gorm:"column:count" json:"count"`
	}
	var providers []providerRow
	a.DB.Raw(`select coalesce(json_extract(e.meta, '$.provider'), 'unknown') as provider, count(*) as count
		from activity_events e `+join+`
		where e.created_at >= ? and e.created_at < ? and e.type = 'login' and e.name = 'login.success'
		group by provider`, start, end).Scan(&providers)

	type studentRow struct {
		ID          string  `gorm:"column:id"`
		Name        string  `gorm:"column:name"`
		Email       string  `gorm:"column:email"`
		LastLoginAt *time.Time
		LastSeenAt  *time.Time
		CreatedAt   time.Time
		Logins      int64 `gorm:"column:logins"`
		ActiveDays  int64 `gorm:"column:active_days"`
	}
	var students []studentRow
	a.DB.Raw(`select u.id, u.name, u.email, u.last_login_at, u.last_seen_at, u.created_at,
		(select count(*) from activity_events e where e.user_id = u.id and e.type = 'login' and e.name = 'login.success' and e.created_at >= ?) as logins,
		(select count(distinct strftime('%Y-%m-%d', e.created_at, ?)) from activity_events e where e.user_id = u.id and e.created_at >= ?) as active_days
		from users u
		where u.role = 'student' and u.deleted_at is null
		order by (u.last_seen_at is null), u.last_seen_at desc, u.created_at desc`, start, tzModifier(), start).Scan(&students)

	start7 := dayStart(appNow()).AddDate(0, 0, -6)
	var active7d int64
	a.DB.Raw(`select count(distinct e.user_id) from activity_events e `+join+` where e.created_at >= ?`, start7).Scan(&active7d)

	today := appNow().Format("2006-01-02")
	todayRow := byDay[today]
	inactive7d := int64(0)
	studentsOut := make([]gin.H, 0, len(students))
	for _, s := range students {
		inactiveDays := int64(-1)
		if s.LastSeenAt != nil {
			inactiveDays = int64(dayStart(appNow()).Sub(dayStart(*s.LastSeenAt)).Hours() / 24)
			if s.LastSeenAt.Before(start7) {
				inactive7d++
			}
		} else {
			inactive7d++
		}
		studentsOut = append(studentsOut, gin.H{
			"id": s.ID, "name": s.Name, "email": s.Email,
			"lastLoginAt": s.LastLoginAt, "lastSeenAt": s.LastSeenAt,
			"logins": s.Logins, "activeDays": s.ActiveDays, "inactiveDays": inactiveDays,
		})
	}

	c.JSON(200, gin.H{
		"days":  days,
		"daily": dailyOut,
		"hours": hoursOut,
		"providers": providers,
		"students":  studentsOut,
		"summary": gin.H{
			"todayLogins": todayRow.Logins,
			"todayActive": todayRow.DAU,
			"active7d":    active7d,
			"inactive7d":  inactive7d,
		},
	})
}

func (a *App) telemetryFeatures(c *gin.Context) {
	days, start := telemetryWindow(c)
	end := start.AddDate(0, 0, days)
	join := fmt.Sprintf(studentScopeJoin, "e")

	type cellRow struct {
		Day   string
		Name  string `gorm:"column:name"`
		Count int64  `gorm:"column:count"`
		Users int64  `gorm:"column:users"`
	}
	var cells []cellRow
	a.DB.Raw(`select strftime('%Y-%m-%d', e.created_at, ?) as day, e.name, count(*) as count, count(distinct e.user_id) as users
		from activity_events e `+join+`
		where e.created_at >= ? and e.created_at < ? and e.type in ('page_view','feature')
		group by day, e.name`, tzModifier(), start, end).Scan(&cells)

	totals := map[string]cellRow{}
	totalUsers := map[string]int64{}
	cellsByDay := map[string]map[string]int64{}
	for _, cell := range cells {
		totals[cell.Name] = cellRow{Day: cell.Name, Name: cell.Name, Count: totals[cell.Name].Count + cell.Count}
		totalUsers[cell.Name] += cell.Users
		if cellsByDay[cell.Day] == nil {
			cellsByDay[cell.Day] = map[string]int64{}
		}
		cellsByDay[cell.Day][cell.Name] += cell.Count
	}
	names := make([]string, 0, len(totals))
	for name := range totals {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if totals[names[i]].Count != totals[names[j]].Count {
			return totals[names[i]].Count > totals[names[j]].Count
		}
		return names[i] < names[j]
	})
	totalsOut := make([]gin.H, 0, len(names))
	for _, name := range names {
		totalsOut = append(totalsOut, gin.H{"name": name, "count": totals[name].Count, "users": totalUsers[name]})
	}
	rowsOut := make([]gin.H, 0, days)
	for _, date := range telemetryDates(start, days) {
		row := gin.H{"date": date}
		for name, count := range cellsByDay[date] {
			row[name] = count
		}
		rowsOut = append(rowsOut, row)
	}

	type noteRow struct {
		ResourceID string `gorm:"column:resource_id"`
		Opens      int64  `gorm:"column:opens"`
		Users      int64  `gorm:"column:users"`
	}
	var noteRows []noteRow
	a.DB.Raw(`select json_extract(e.meta, '$.resourceId') as resource_id, count(*) as opens, count(distinct e.user_id) as users
		from activity_events e `+join+`
		where e.created_at >= ? and e.created_at < ? and e.type = 'feature' and e.name = 'note.open'
			and json_extract(e.meta, '$.resourceId') is not null
		group by resource_id order by opens desc limit 10`, start, end).Scan(&noteRows)
	notesOut := make([]gin.H, 0, len(noteRows))
	if len(noteRows) > 0 {
		ids := make([]string, 0, len(noteRows))
		for _, row := range noteRows {
			ids = append(ids, row.ResourceID)
		}
		var resources []NoteResource
		a.DB.Select("id", "title").Where("id in ?", ids).Find(&resources)
		titles := make(map[string]string, len(resources))
		for _, r := range resources {
			titles[r.ID] = r.Title
		}
		for _, row := range noteRows {
			notesOut = append(notesOut, gin.H{
				"resourceId": row.ResourceID, "title": titles[row.ResourceID],
				"opens": row.Opens, "users": row.Users,
			})
		}
	}

	c.JSON(200, gin.H{
		"days": days, "names": names, "rows": rowsOut,
		"totals": totalsOut, "notes": notesOut,
	})
}

func (a *App) telemetryReviews(c *gin.Context) {
	days, start := telemetryWindow(c)
	end := start.AddDate(0, 0, days)
	join := fmt.Sprintf(studentScopeJoin, "re")

	type dayRow struct {
		Day        string   `json:"date"`
		Total      int64    `gorm:"column:total" json:"total"`
		Proficient int64    `gorm:"column:proficient" json:"proficient"`
		Fuzzy      int64    `gorm:"column:fuzzy" json:"fuzzy"`
		Unknown    int64    `gorm:"column:unknown" json:"unknown"`
		AvgMS      *float64 `gorm:"column:avg_ms" json:"avgMs"`
	}
	var daily []dayRow
	a.DB.Raw(`select strftime('%Y-%m-%d', re.created_at, ?) as day, count(*) as total,
		sum(case when re.response = 'proficient' then 1 else 0 end) as proficient,
		sum(case when re.response = 'fuzzy' then 1 else 0 end) as fuzzy,
		sum(case when re.response = 'unknown' then 1 else 0 end) as unknown,
		avg(re.duration_ms) as avg_ms
		from review_events re `+join+`
		where re.created_at >= ? and re.created_at < ?
		group by day`, tzModifier(), start, end).Scan(&daily)
	byDay := make(map[string]dayRow, len(daily))
	for _, row := range daily {
		byDay[row.Day] = row
	}
	dailyOut := make([]gin.H, 0, days)
	var grandTotal int64
	var durationSum, durationCount float64
	for _, date := range telemetryDates(start, days) {
		row := byDay[date]
		grandTotal += row.Total
		if row.AvgMS != nil {
			durationSum += *row.AvgMS * float64(row.Total)
			durationCount += float64(row.Total)
		}
		var avg any
		if row.AvgMS != nil {
			avg = int64(*row.AvgMS)
		}
		dailyOut = append(dailyOut, gin.H{
			"date": date, "total": row.Total,
			"proficient": row.Proficient, "fuzzy": row.Fuzzy, "unknown": row.Unknown,
			"avgMs": avg,
		})
	}

	type hourRow struct {
		Hour    int
		Reviews int64 `gorm:"column:reviews"`
	}
	var byHour []hourRow
	a.DB.Raw(`select cast(strftime('%H', re.created_at, ?) as integer) as hour, count(*) as reviews
		from review_events re `+join+`
		where re.created_at >= ? and re.created_at < ?
		group by hour`, tzModifier(), start, end).Scan(&byHour)
	hourMap := make(map[int]int64, len(byHour))
	for _, row := range byHour {
		hourMap[row.Hour] = row.Reviews
	}
	hoursOut := make([]gin.H, 0, 24)
	for h := 0; h < 24; h++ {
		hoursOut = append(hoursOut, gin.H{"hour": h, "reviews": hourMap[h]})
	}

	type weekdayRow struct {
		Weekday int
		Reviews int64 `gorm:"column:reviews"`
	}
	var byWeekday []weekdayRow
	a.DB.Raw(`select cast(strftime('%w', re.created_at, ?) as integer) as weekday, count(*) as reviews
		from review_events re `+join+`
		where re.created_at >= ? and re.created_at < ?
		group by weekday`, tzModifier(), start, end).Scan(&byWeekday)
	weekdayMap := make(map[int]int64, len(byWeekday))
	for _, row := range byWeekday {
		weekdayMap[row.Weekday] = row.Reviews
	}
	weekdaysOut := make([]gin.H, 0, 7)
	for w := 0; w < 7; w++ {
		weekdaysOut = append(weekdaysOut, gin.H{"weekday": w, "reviews": weekdayMap[w]})
	}

	type conceptRow struct {
		Term    string `json:"term"`
		Reviews int64  `gorm:"column:reviews" json:"reviews"`
	}
	var topConcepts []conceptRow
	a.DB.Raw(`select c.term, count(*) as reviews
		from review_events re `+join+` join concepts c on c.id = re.concept_id
		where re.created_at >= ? and re.created_at < ?
		group by c.term order by reviews desc limit 10`, start, end).Scan(&topConcepts)

	var avgDuration any
	if durationCount > 0 {
		avgDuration = int64(durationSum / durationCount)
	}
	c.JSON(200, gin.H{
		"days": days, "daily": dailyOut, "hours": hoursOut, "weekdays": weekdaysOut,
		"topConcepts": topConcepts, "total": grandTotal, "avgDurationMs": avgDuration,
	})
}

func (a *App) telemetryPractice(c *gin.Context) {
	days, start := telemetryWindow(c)
	end := start.AddDate(0, 0, days)
	join := fmt.Sprintf(studentScopeJoin, "pa")

	type attemptRow struct {
		Day      string
		Attempts int64 `gorm:"column:attempts"`
	}
	var attempts []attemptRow
	a.DB.Raw(`select strftime('%Y-%m-%d', pa.started_at, ?) as day, count(*) as attempts
		from practice_attempts pa `+join+`
		where pa.started_at >= ? and pa.started_at < ?
		group by day`, tzModifier(), start, end).Scan(&attempts)
	attemptByDay := map[string]int64{}
	var totalAttempts int64
	for _, row := range attempts {
		attemptByDay[row.Day] = row.Attempts
		totalAttempts += row.Attempts
	}

	type answerRow struct {
		Day     string
		Answers int64 `gorm:"column:answers"`
		Correct int64 `gorm:"column:correct"`
	}
	var answers []answerRow
	a.DB.Raw(`select strftime('%Y-%m-%d', pan.answered_at, ?) as day, count(*) as answers,
		sum(case when pan.is_correct = 1 then 1 else 0 end) as correct
		from practice_answers pan
		join practice_attempts pa on pa.id = pan.attempt_id `+join+`
		where pan.answered_at >= ? and pan.answered_at < ?
		group by day`, tzModifier(), start, end).Scan(&answers)
	answerByDay := map[string]answerRow{}
	var totalAnswers, totalCorrect int64
	for _, row := range answers {
		answerByDay[row.Day] = row
		totalAnswers += row.Answers
		totalCorrect += row.Correct
	}

	dailyOut := make([]gin.H, 0, days)
	for _, date := range telemetryDates(start, days) {
		a := attemptByDay[date]
		ans := answerByDay[date]
		dailyOut = append(dailyOut, gin.H{
			"date": date, "attempts": a, "answers": ans.Answers, "correct": ans.Correct,
		})
	}
	c.JSON(200, gin.H{
		"days": days, "daily": dailyOut,
		"totals": gin.H{"attempts": totalAttempts, "answers": totalAnswers, "correct": totalCorrect},
	})
}

// telemetryLog streams raw activity events (admin-only) as a paginated
// table: ?page=1&limit=20 with filters, plus format=csv export.
func (a *App) telemetryLog(c *gin.Context) {
	filtered := func() *gorm.DB {
		query := a.DB.Table("activity_events as e").
			Select("e.id, e.created_at, e.type, e.name, e.path, e.meta, e.user_id, u.name as user_name, u.email as user_email").
			Joins("left join users u on u.id = e.user_id")
		if userID := c.Query("userId"); userID != "" {
			query = query.Where("e.user_id = ?", userID)
		}
		if eventType := c.Query("type"); eventType != "" {
			query = query.Where("e.type = ?", eventType)
		}
		if name := c.Query("name"); name != "" {
			query = query.Where("e.name = ?", name)
		}
		if from := c.Query("from"); from != "" {
			if day, err := time.ParseInLocation("2006-01-02", from, appTimeLocation()); err == nil {
				query = query.Where("e.created_at >= ?", day)
			}
		}
		if to := c.Query("to"); to != "" {
			if day, err := time.ParseInLocation("2006-01-02", to, appTimeLocation()); err == nil {
				query = query.Where("e.created_at < ?", day.AddDate(0, 0, 1))
			}
		}
		return query
	}

	limit := 20
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	page := 1
	if v, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && v > 0 {
		page = v
	}

	if c.Query("format") == "csv" {
		type logRow struct {
			ID        string
			CreatedAt time.Time
			Type      string
			Name      string
			Path      string
			Meta      datatypes.JSON
			UserID    string `gorm:"column:user_id"`
			UserName  *string
			UserEmail *string
		}
		var rows []logRow
		filtered().Order("e.created_at desc, e.id desc").Limit(5000).Scan(&rows)
		var buf bytes.Buffer
		buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM so Excel opens Chinese correctly
		w := csv.NewWriter(&buf)
		w.Write([]string{"time", "type", "name", "path", "user", "email", "meta"})
		for _, row := range rows {
			meta := strings.TrimSpace(string(row.Meta))
			if meta == "" || meta == "null" {
				meta = ""
			}
			userName, userEmail := "", ""
			if row.UserName != nil {
				userName = *row.UserName
			}
			if row.UserEmail != nil {
				userEmail = *row.UserEmail
			}
			w.Write([]string{
				row.CreatedAt.In(appTimeLocation()).Format("2006-01-02 15:04:05"),
				row.Type, row.Name, row.Path, userName, userEmail, meta,
			})
		}
		w.Flush()
		c.Header("Content-Disposition", `attachment; filename="activity-log.csv"`)
		c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
		return
	}

	var total int64
	filtered().Count(&total)

	type logRow struct {
		ID        string
		CreatedAt time.Time
		Type      string
		Name      string
		Path      string
		Meta      datatypes.JSON
		UserID    string `gorm:"column:user_id"`
		UserName  *string
		UserEmail *string
	}
	var rows []logRow
	filtered().Order("e.created_at desc, e.id desc").Limit(limit).Offset((page - 1) * limit).Scan(&rows)
	events := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		userName, userEmail := "", ""
		if row.UserName != nil {
			userName = *row.UserName
		}
		if row.UserEmail != nil {
			userEmail = *row.UserEmail
		}
		var meta any
		if len(row.Meta) > 0 && string(row.Meta) != "null" {
			meta = row.Meta
		}
		events = append(events, gin.H{
			"id": row.ID, "createdAt": row.CreatedAt, "type": row.Type, "name": row.Name,
			"path": row.Path, "meta": meta,
			"userId": row.UserID, "userName": userName, "userEmail": userEmail,
		})
	}
	c.JSON(200, gin.H{
		"events": events, "total": total, "page": page, "pageSize": limit,
	})
}
