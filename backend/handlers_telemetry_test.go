package backend

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/datatypes"
)

func newTelemetryTestApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return app, app.Router()
}

func telemetryCall(t *testing.T, router http.Handler, method, path, token string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestTelemetryLoginEvents(t *testing.T) {
	app, router := newTelemetryTestApp(t)
	token := registerTestUser(t, router, "telemetry-login@example.com")

	var user User
	if err := app.DB.First(&user, "email = ?", "telemetry-login@example.com").Error; err != nil {
		t.Fatal(err)
	}
	if user.LastLoginAt == nil {
		t.Fatal("successful login should set last_login_at")
	}

	var successCount int64
	app.DB.Model(&ActivityEvent{}).Where("type = ? and name = ? and user_id = ?", EventLogin, "login.success", user.ID).Count(&successCount)
	if successCount != 1 { // from registration sign-in
		t.Fatalf("expected one login.success event, got %d", successCount)
	}

	// Wrong password records a failed login tied to the user.
	telemetryCall(t, router, http.MethodPost, "/api/auth/login", "", `{"email":"telemetry-login@example.com","password":"wrong"}`)
	var failed int64
	app.DB.Model(&ActivityEvent{}).Where("type = ? and name = ? and user_id = ?", EventLogin, "login.failed", user.ID).Count(&failed)
	if failed != 1 {
		t.Fatalf("expected one login.failed event for known user, got %d", failed)
	}

	// Unknown email records an anonymous failed login with the attempted email.
	telemetryCall(t, router, http.MethodPost, "/api/auth/login", "", `{"email":"ghost@example.com","password":"wrong"}`)
	var ghost []ActivityEvent
	app.DB.Where("type = ? and name = ? and user_id = ''", EventLogin, "login.failed").Find(&ghost)
	if len(ghost) != 1 {
		t.Fatalf("expected one anonymous login.failed event, got %d", len(ghost))
	}
	if !strings.Contains(string(ghost[0].Meta), "ghost@example.com") {
		t.Fatalf("failed login meta should carry attempted email, got %s", ghost[0].Meta)
	}
	_ = token
}

func TestTelemetryHeartbeatDedupe(t *testing.T) {
	app, router := newTelemetryTestApp(t)
	token := registerTestUser(t, router, "heartbeat@example.com")

	telemetryCall(t, router, http.MethodGet, "/api/dashboard", token, "")
	telemetryCall(t, router, http.MethodGet, "/api/dashboard", token, "")
	telemetryCall(t, router, http.MethodGet, "/api/practice/sets", token, "")

	var user User
	app.DB.First(&user, "email = ?", "heartbeat@example.com")
	if user.LastSeenAt == nil {
		t.Fatal("heartbeat should set last_seen_at")
	}
	var beats int64
	app.DB.Model(&ActivityEvent{}).Where("type = ? and user_id = ?", EventHeartbeat, user.ID).Count(&beats)
	if beats != 1 {
		t.Fatalf("expected one deduped heartbeat inside the window, got %d", beats)
	}

	// Outside the window a new heartbeat lands.
	original := heartbeatWindow
	heartbeatWindow = 0
	defer func() { heartbeatWindow = original }()
	telemetryCall(t, router, http.MethodGet, "/api/dashboard", token, "")
	app.DB.Model(&ActivityEvent{}).Where("type = ? and user_id = ?", EventHeartbeat, user.ID).Count(&beats)
	if beats != 2 {
		t.Fatalf("expected a second heartbeat after the window, got %d", beats)
	}

	// Failed requests must not heartbeat.
	heartbeatWindow = 0
	telemetryCall(t, router, http.MethodGet, "/api/concepts/nonexistent-concept", token, "")
	app.DB.Model(&ActivityEvent{}).Where("type = ? and user_id = ?", EventHeartbeat, user.ID).Count(&beats)
	if beats != 2 {
		t.Fatalf("failed request should not record heartbeat, got %d", beats)
	}
}

func TestReportTelemetry(t *testing.T) {
	app, router := newTelemetryTestApp(t)
	token := registerTestUser(t, router, "report@example.com")
	var user User
	app.DB.First(&user, "email = ?", "report@example.com")

	w := telemetryCall(t, router, http.MethodPost, "/api/telemetry/events", token,
		`{"events":[{"type":"page_view","name":"Page.Review","path":"/review"},{"type":"feature","name":"note.open","meta":{"resourceId":"nr_1"}},{"type":"login","name":"login.success"},{"type":"feature","name":"bad name!"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("report failed: %d %s", w.Code, w.Body.String())
	}
	var stored struct {
		Stored int `json:"stored"`
	}
	json.Unmarshal(w.Body.Bytes(), &stored)
	if stored.Stored != 2 {
		t.Fatalf("expected 2 stored events (invalid type/name skipped), got %d", stored.Stored)
	}
	var count int64
	app.DB.Model(&ActivityEvent{}).Where("user_id = ? and type in ('page_view','feature')", user.ID).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 client events in db, got %d", count)
	}
	var pageView ActivityEvent
	app.DB.First(&pageView, "user_id = ? and type = ?", user.ID, EventPageView)
	if pageView.Name != "page.review" {
		t.Fatalf("event name should be lowercased, got %q", pageView.Name)
	}

	var oversize strings.Builder
	oversize.WriteString(`{"events":[`)
	for i := 0; i < 51; i++ {
		if i > 0 {
			oversize.WriteString(",")
		}
		oversize.WriteString(`{"type":"page_view","name":"page.x"}`)
	}
	oversize.WriteString(`]}`)
	w = telemetryCall(t, router, http.MethodPost, "/api/telemetry/events", token, oversize.String())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized batch should 400, got %d", w.Code)
	}
}

func TestTelemetryAggregatesAndPermissions(t *testing.T) {
	app, router := newTelemetryTestApp(t)
	adminToken := registerTestUser(t, router, "agg-admin@example.com")
	studentToken := registerTestUser(t, router, "agg-student@example.com")

	var student User
	app.DB.First(&student, "email = ?", "agg-student@example.com")
	var teacher User
	teacher = User{ID: NewID("usr"), TenantID: schoolTenantID, Name: "Teacher", Email: "agg-teacher@example.com", Role: "teacher", Provider: "local", PasswordHash: "x"}
	if err := app.DB.Create(&teacher).Error; err != nil {
		t.Fatal(err)
	}
	teacherToken, _ := app.sign(teacher)

	// Student usage today: a page view, a note open, two review events with
	// durations, and one graded practice answer.
	now := appNow()
	app.DB.Create(&ActivityEvent{ID: NewID("act"), TenantID: schoolTenantID, UserID: student.ID, Type: EventPageView, Name: "page.review", CreatedAt: now})
	app.DB.Create(&ActivityEvent{ID: NewID("act"), TenantID: schoolTenantID, UserID: student.ID, Type: EventPageView, Name: "page.review", CreatedAt: now.Add(-time.Minute)})
	app.DB.Create(&ActivityEvent{ID: NewID("act"), TenantID: schoolTenantID, UserID: student.ID, Type: EventFeature, Name: "note.open", Meta: datatypes.JSON(`{"resourceId":"nr_demo"}`), CreatedAt: now})
	var concept Concept
	app.DB.First(&concept)
	app.DB.Create(&ReviewEvent{ID: NewID("rev"), UserID: student.ID, ConceptID: concept.ID, Response: "proficient", DurationMS: 4000, CreatedAt: now})
	app.DB.Create(&ReviewEvent{ID: NewID("rev"), UserID: student.ID, ConceptID: concept.ID, Response: "fuzzy", DurationMS: 6000, CreatedAt: now})
	attempt := PracticeAttempt{ID: NewID("pat"), UserID: student.ID, SetID: "set_demo", Mode: "instant", StartedAt: now, TotalMCQ: 1}
	app.DB.Create(&attempt)
	correct := true
	app.DB.Create(&PracticeAnswer{ID: NewID("pan"), AttemptID: attempt.ID, QuestionID: "q_demo", ChoiceKey: "A", IsCorrect: &correct, AnsweredAt: now})
	app.DB.Create(&NoteResource{ID: "nr_demo", Title: "Unit 1 Notes", Status: "published"})

	// Students may not read any telemetry endpoint.
	if w := telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/activity", studentToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("student should get 403 on activity, got %d", w.Code)
	}
	if w := telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/log", studentToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("student should get 403 on log, got %d", w.Code)
	}

	// Teachers read aggregates but not the raw log.
	w := telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/activity?days=7", teacherToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("teacher activity failed: %d %s", w.Code, w.Body.String())
	}
	var activity struct {
		Daily []struct {
			Date   string `json:"date"`
			Logins int64  `json:"logins"`
			DAU    int64  `json:"dau"`
		} `json:"daily"`
		Hours []struct {
			Hour   int   `json:"hour"`
			Events int64 `json:"events"`
		} `json:"hours"`
		Providers []struct {
			Provider string `json:"provider"`
			Count    int64  `json:"count"`
		} `json:"providers"`
		Students []struct {
			ID         string `json:"id"`
			ActiveDays int64  `json:"activeDays"`
			Logins     int64  `json:"logins"`
		} `json:"students"`
		Summary struct {
			TodayActive int64 `json:"todayActive"`
			TodayLogins int64 `json:"todayLogins"`
			Inactive7d  int64 `json:"inactive7d"`
		} `json:"summary"`
	}
	json.Unmarshal(w.Body.Bytes(), &activity)
	if len(activity.Daily) != 7 {
		t.Fatalf("expected 7 daily buckets, got %d", len(activity.Daily))
	}
	today := now.Format("2006-01-02")
	var todayRow *struct {
		Date   string `json:"date"`
		Logins int64  `json:"logins"`
		DAU    int64  `json:"dau"`
	}
	for i := range activity.Daily {
		if activity.Daily[i].Date == today {
			todayRow = &activity.Daily[i]
		}
	}
	if todayRow == nil || todayRow.Logins < 1 || todayRow.DAU != 1 {
		t.Fatalf("today's logins/DAU wrong: %+v", todayRow)
	}
	if activity.Summary.TodayActive < 1 {
		t.Fatalf("todayActive should count the student, got %d", activity.Summary.TodayActive)
	}
	if activity.Summary.Inactive7d < 1 {
		t.Fatalf("inactive7d should count never-seen students, got %d", activity.Summary.Inactive7d)
	}
	if len(activity.Providers) == 0 || activity.Providers[0].Provider != "local" {
		t.Fatalf("provider breakdown wrong: %+v", activity.Providers)
	}
	found := false
	for _, s := range activity.Students {
		if s.ID == student.ID {
			found = true
			if s.ActiveDays < 1 || s.Logins < 1 {
				t.Fatalf("student rollup wrong: %+v", s)
			}
		}
	}
	if !found {
		t.Fatal("student missing from activity students list")
	}

	w = telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/features?days=7", teacherToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("teacher features failed: %d %s", w.Code, w.Body.String())
	}
	var features struct {
		Names  []string `json:"names"`
		Totals []struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		} `json:"totals"`
		Notes []struct {
			ResourceID string `json:"resourceId"`
			Title      string `json:"title"`
			Opens      int64  `json:"opens"`
		} `json:"notes"`
	}
	json.Unmarshal(w.Body.Bytes(), &features)
	if len(features.Names) != 2 || features.Names[0] != "page.review" {
		t.Fatalf("feature names wrong: %+v", features.Names)
	}
	if len(features.Totals) != 2 || features.Totals[0].Count != 2 {
		t.Fatalf("feature totals wrong: %+v", features.Totals)
	}
	if len(features.Notes) != 1 || features.Notes[0].Title != "Unit 1 Notes" || features.Notes[0].Opens != 1 {
		t.Fatalf("note ranking wrong: %+v", features.Notes)
	}

	w = telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/reviews?days=7", teacherToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("teacher reviews failed: %d %s", w.Code, w.Body.String())
	}
	var reviews struct {
		Total         int64 `json:"total"`
		AvgDurationMs any   `json:"avgDurationMs"`
		TopConcepts   []struct {
			Term    string `json:"term"`
			Reviews int64  `json:"reviews"`
		} `json:"topConcepts"`
		Daily []struct {
			Proficient int64 `json:"proficient"`
			Fuzzy      int64 `json:"fuzzy"`
			Unknown    int64 `json:"unknown"`
		} `json:"daily"`
	}
	json.Unmarshal(w.Body.Bytes(), &reviews)
	if reviews.Total != 2 {
		t.Fatalf("expected 2 review events, got %d", reviews.Total)
	}
	if reviews.AvgDurationMs != float64(5000) {
		t.Fatalf("avg duration should be 5000ms, got %v", reviews.AvgDurationMs)
	}
	if len(reviews.TopConcepts) == 0 || reviews.TopConcepts[0].Term != concept.Term {
		t.Fatalf("top concepts wrong: %+v", reviews.TopConcepts)
	}

	w = telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/practice?days=7", teacherToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("teacher practice failed: %d %s", w.Code, w.Body.String())
	}
	var practice struct {
		Totals struct {
			Attempts int64 `json:"attempts"`
			Answers  int64 `json:"answers"`
			Correct  int64 `json:"correct"`
		} `json:"totals"`
	}
	json.Unmarshal(w.Body.Bytes(), &practice)
	if practice.Totals.Attempts != 1 || practice.Totals.Answers != 1 || practice.Totals.Correct != 1 {
		t.Fatalf("practice totals wrong: %+v", practice.Totals)
	}

	if w := telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/log", teacherToken, ""); w.Code != http.StatusForbidden {
		t.Fatalf("teacher should get 403 on raw log, got %d", w.Code)
	}

	// Admin reads the raw log with filters, keyset pagination and CSV export.
	w = telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/log?type=login&limit=2", adminToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin log failed: %d %s", w.Code, w.Body.String())
	}
	var logPage struct {
		Events []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"events"`
		HasMore    bool   `json:"hasMore"`
		NextBefore string `json:"nextBefore"`
	}
	json.Unmarshal(w.Body.Bytes(), &logPage)
	if len(logPage.Events) == 0 || logPage.Events[0].Type != "login" {
		t.Fatalf("log filter returned wrong rows: %+v", logPage.Events)
	}
	w = telemetryCall(t, router, http.MethodGet, "/api/admin/telemetry/log?format=csv&userId="+student.ID, adminToken, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("csv export failed: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Body.String(), "note.open") || !strings.Contains(w.Body.String(), "agg-student@example.com") {
		t.Fatalf("csv content missing student rows: %s", w.Body.String())
	}
}

func TestPruneTelemetry(t *testing.T) {
	app, _ := newTelemetryTestApp(t)
	old := appNow().AddDate(0, 0, -200)
	app.DB.Create(&ActivityEvent{ID: NewID("act"), TenantID: schoolTenantID, UserID: "usr_x", Type: EventHeartbeat, Name: "heartbeat", CreatedAt: old})
	app.DB.Create(&ActivityEvent{ID: NewID("act"), TenantID: schoolTenantID, UserID: "usr_x", Type: EventLogin, Name: "login.success", CreatedAt: old})
	app.DB.Create(&ActivityEvent{ID: NewID("act"), TenantID: schoolTenantID, UserID: "usr_x", Type: EventHeartbeat, Name: "heartbeat", CreatedAt: appNow()})

	t.Setenv("TELEMETRY_RETENTION_DAYS", "180")
	pruneTelemetry(app.DB)

	var heartbeats, logins int64
	app.DB.Model(&ActivityEvent{}).Where("type = ?", EventHeartbeat).Count(&heartbeats)
	app.DB.Model(&ActivityEvent{}).Where("type = ?", EventLogin).Count(&logins)
	if heartbeats != 1 {
		t.Fatalf("prune should keep only the recent heartbeat, got %d", heartbeats)
	}
	if logins != 1 {
		t.Fatalf("prune must never drop login events, got %d", logins)
	}

	t.Setenv("TELEMETRY_RETENTION_DAYS", "0")
	pruneTelemetry(app.DB)
	app.DB.Model(&ActivityEvent{}).Where("type = ?", EventHeartbeat).Count(&heartbeats)
	if heartbeats != 1 {
		t.Fatalf("retention=0 must keep everything, got %d", heartbeats)
	}
}
