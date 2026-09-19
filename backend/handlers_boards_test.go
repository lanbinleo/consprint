package backend

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func newBoardsTestApp(t *testing.T) (*App, http.Handler, string, string) {
	t.Helper()
	app, router, student, teacher := newNotesTestApp(t)
	return app, router, student, teacher
}

func TestBoardsSeedAndQuote(t *testing.T) {
	app, router, student, _ := newBoardsTestApp(t)

	// The announcement board starts empty — no seeded welcome post.
	announcements := fetchAnnouncements(t, router, student)
	if len(announcements) != 0 {
		t.Fatalf("fresh board should have no announcements: %#v", announcements)
	}

	// Quote pack is seeded and the pick is deterministic within a day.
	var quotes int64
	app.DB.Model(&Quote{}).Count(&quotes)
	if quotes != int64(len(seedQuotes)) {
		t.Fatalf("expected %d seeded quotes, got %d", len(seedQuotes), quotes)
	}
	first, second := app.quoteOfToday(), app.quoteOfToday()
	if first == nil || second == nil || first.TextZh != second.TextZh {
		t.Fatalf("quote of the day is not deterministic: %v vs %v", first, second)
	}

	// The dashboard summary carries the quote for the daily greeting card.
	w := notesRequest(t, router, http.MethodGet, "/api/dashboard/summary", student, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("summary failed: %d %s", w.Code, w.Body.String())
	}
	var summary struct {
		TotalConcepts int64  `json:"totalConcepts"`
		Quote         *Quote `json:"quote"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Quote == nil || summary.Quote.TextZh == "" || summary.Quote.Source == "" {
		t.Fatalf("summary quote missing: %#v", summary.Quote)
	}

	// Seeds are idempotent.
	if err := seedBoards(app.DB); err != nil {
		t.Fatal(err)
	}
	var reseededAnnouncements, reseededQuotes int64
	app.DB.Model(&Announcement{}).Count(&reseededAnnouncements)
	if reseededAnnouncements != 0 {
		t.Fatalf("announcement seed not idempotent: %d rows", reseededAnnouncements)
	}
	app.DB.Model(&Quote{}).Count(&reseededQuotes)
	if reseededQuotes != int64(len(seedQuotes)) {
		t.Fatalf("quote seed not idempotent: %d rows", reseededQuotes)
	}
}

func TestAnnouncementCRUD(t *testing.T) {
	_, router, student, teacher := newBoardsTestApp(t)

	// Students may read but not manage the board.
	if w := notesRequest(t, router, http.MethodPost, "/api/admin/announcements", student, map[string]any{"title": "Nope"}); w.Code != http.StatusForbidden {
		t.Fatalf("student create should be forbidden: %d %s", w.Code, w.Body.String())
	}

	published := map[string]any{
		"title":  "周五模考",
		"body":   "带 ==tangerine|2B 铅笔== 和耳机，**8:00** 开始。",
		"status": "published",
		"pinned": false,
	}
	w := notesRequest(t, router, http.MethodPost, "/api/admin/announcements", teacher, published)
	if w.Code != http.StatusOK {
		t.Fatalf("teacher create failed: %d %s", w.Code, w.Body.String())
	}
	var created Announcement
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.AuthorName == "" {
		t.Fatal("created announcement should carry the author name")
	}

	// Drafts stay invisible to students; published ones show up.
	draft := map[string]any{"title": "草稿", "status": "draft"}
	if w := notesRequest(t, router, http.MethodPost, "/api/admin/announcements", teacher, draft); w.Code != http.StatusOK {
		t.Fatalf("draft create failed: %d %s", w.Code, w.Body.String())
	}
	visible := fetchAnnouncements(t, router, student)
	if len(visible) != 1 { // 周五模考 is the only published post (no seed)
		t.Fatalf("student should see 1 published announcement, got %d", len(visible))
	}

	// Pinning puts the announcement at the top of the board.
	if w := notesRequest(t, router, http.MethodPatch, "/api/admin/announcements/"+created.ID, teacher, map[string]any{"pinned": true}); w.Code != http.StatusOK {
		t.Fatalf("pin failed: %d %s", w.Code, w.Body.String())
	}
	visible = fetchAnnouncements(t, router, student)
	if visible[0].ID != created.ID {
		t.Fatalf("pinned announcement should lead the board: %#v", visible)
	}

	// The staff list sees drafts too.
	w = notesRequest(t, router, http.MethodGet, "/api/admin/announcements", teacher, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("staff list failed: %d", w.Code)
	}
	var staffList []Announcement
	if err := json.Unmarshal(w.Body.Bytes(), &staffList); err != nil {
		t.Fatal(err)
	}
	if len(staffList) != 2 { // 周五模考 + 草稿
		t.Fatalf("staff should see all 2 announcements, got %d", len(staffList))
	}

	// Validation: empty title, bad status, oversized body all 400.
	for _, bad := range []map[string]any{
		{"title": "  "},
		{"title": "ok", "status": "live"},
		{"title": "ok", "body": string(make([]byte, maxAnnouncementBody+1))},
	} {
		if w := notesRequest(t, router, http.MethodPost, "/api/admin/announcements", teacher, bad); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid payload should 400: %d %s", w.Code, w.Body.String())
		}
	}

	// Delete removes the announcement from the student board.
	if w := notesRequest(t, router, http.MethodDelete, "/api/admin/announcements/"+created.ID, teacher, nil); w.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", w.Code, w.Body.String())
	}
	visible = fetchAnnouncements(t, router, student)
	if len(visible) != 0 {
		t.Fatalf("deleted announcement still visible: %#v", visible)
	}
}

func TestCalendarCRUD(t *testing.T) {
	_, router, student, teacher := newBoardsTestApp(t)

	if w := notesRequest(t, router, http.MethodPost, "/api/admin/calendar", student, map[string]any{"title": "Nope", "date": "2026-09-01"}); w.Code != http.StatusForbidden {
		t.Fatalf("student create should be forbidden: %d %s", w.Code, w.Body.String())
	}

	// Invalid payloads are rejected.
	for _, bad := range []map[string]any{
		{"title": "bad date", "date": "2026-9-1", "kind": "event"},
		{"title": "bad kind", "date": "2026-09-01", "kind": "party"},
		{"title": "retired kind", "date": "2026-09-01", "kind": "assessment"},
		{"title": "bad time", "date": "2026-09-01", "kind": "event", "time": "9am"},
		{"title": "reversed range", "date": "2026-09-10", "endDate": "2026-09-01", "kind": "event"},
		{"title": "", "date": "2026-09-01", "kind": "event"},
	} {
		if w := notesRequest(t, router, http.MethodPost, "/api/admin/calendar", teacher, bad); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid event should 400: %d %s", w.Code, w.Body.String())
		}
	}

	// The granular test kinds are accepted.
	for _, kind := range []string{"quiz", "unit-test", "exam"} {
		payload := map[string]any{"title": "Kind check " + kind, "date": "2026-09-01", "kind": kind}
		if w := notesRequest(t, router, http.MethodPost, "/api/admin/calendar", teacher, payload); w.Code != http.StatusOK {
			t.Fatalf("kind %s should be valid: %d %s", kind, w.Code, w.Body.String())
		}
	}

	// A multi-day event starting in the previous month and reaching into this
	// one shows in both month views.
	prev := appNow().AddDate(0, -1, 0)
	prevMonth := prev.Format("2006-01")
	_, prevEnd, _ := monthRange(prevMonth)
	thisMonth := appNow().Format("2006-01")
	thisStart, _, _ := monthRange(thisMonth)
	firstOfThis, err := time.ParseInLocation("2006-01-02", thisStart, appTimeLocation())
	if err != nil {
		t.Fatal(err)
	}
	thisSecond := firstOfThis.AddDate(0, 0, 1).Format("2006-01-02")
	straddle := map[string]any{
		"title":   "国庆假期",
		"date":    prevEnd,    // last day of last month
		"endDate": thisSecond, // reaches into the current month
		"kind":    "holiday",
	}
	w := notesRequest(t, router, http.MethodPost, "/api/admin/calendar", teacher, straddle)
	if w.Code != http.StatusOK {
		t.Fatalf("create straddle failed: %d %s", w.Code, w.Body.String())
	}
	var straddleEvent CalendarEvent
	if err := json.Unmarshal(w.Body.Bytes(), &straddleEvent); err != nil {
		t.Fatal(err)
	}
	if straddleEvent.EndDate == nil || *straddleEvent.EndDate != thisSecond {
		t.Fatalf("straddle event endDate not stored: %#v", straddleEvent)
	}

	// A timed assignment inside the current month.
	_, thisEnd, _ := monthRange(thisMonth)
	timed := map[string]any{"title": "Unit 2 作业", "date": thisEnd, "time": "23:59", "kind": "assignment", "note": "提交到班级邮箱"}
	w = notesRequest(t, router, http.MethodPost, "/api/admin/calendar", teacher, timed)
	if w.Code != http.StatusOK {
		t.Fatalf("create timed failed: %d %s", w.Code, w.Body.String())
	}
	var timedEvent CalendarEvent
	if err := json.Unmarshal(w.Body.Bytes(), &timedEvent); err != nil {
		t.Fatal(err)
	}

	// The current month view includes the straddle (start before month) and
	// the timed event; empty time strings normalize to all-day.
	events := fetchCalendar(t, router, student, thisMonth)
	if !hasEvent(events, "Unit 2 作业") || !hasEvent(events, "国庆假期") {
		t.Fatalf("month view missing events: %#v", events)
	}
	if len(fetchCalendar(t, router, student, prevMonth)) == 0 {
		t.Fatal("previous month view should still include the straddle event")
	}
	// Garbage month falls back to the current month rather than 500ing.
	if len(fetchCalendar(t, router, student, "not-a-month")) == 0 {
		t.Fatal("invalid month should fall back to current month")
	}

	// Patch moves the assignment and clears its time.
	patch := map[string]any{"title": "Unit 2 作业（延期）", "date": thisEnd, "time": "", "kind": "assignment"}
	if w := notesRequest(t, router, http.MethodPatch, "/api/admin/calendar/"+timedEvent.ID, teacher, patch); w.Code != http.StatusOK {
		t.Fatalf("patch failed: %d %s", w.Code, w.Body.String())
	}
	events = fetchCalendar(t, router, student, thisMonth)
	for _, e := range events {
		if e.ID == timedEvent.ID && (e.Title != "Unit 2 作业（延期）" || e.Time != nil) {
			t.Fatalf("patch not applied: %#v", e)
		}
	}

	// Staff list sees from two months back by default.
	w = notesRequest(t, router, http.MethodGet, "/api/admin/calendar", teacher, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("staff calendar list failed: %d", w.Code)
	}

	// Delete removes the event.
	if w := notesRequest(t, router, http.MethodDelete, "/api/admin/calendar/"+straddleEvent.ID, teacher, nil); w.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", w.Code, w.Body.String())
	}
	if hasEvent(fetchCalendar(t, router, student, thisMonth), "国庆假期") {
		t.Fatal("deleted event still visible")
	}
}

func TestConceptStar(t *testing.T) {
	app, router, student, _ := newBoardsTestApp(t)

	var concept Concept
	if err := app.DB.First(&concept).Error; err != nil {
		t.Fatal(err)
	}

	// The payload must carry an explicit boolean.
	if w := notesRequest(t, router, http.MethodPatch, "/api/concepts/"+concept.ID+"/star", student, map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing starred should 400: %d", w.Code)
	}
	if w := notesRequest(t, router, http.MethodPatch, "/api/concepts/missing/star", student, map[string]any{"starred": true}); w.Code != http.StatusNotFound {
		t.Fatalf("unknown concept should 404: %d", w.Code)
	}

	if w := notesRequest(t, router, http.MethodPatch, "/api/concepts/"+concept.ID+"/star", student, map[string]any{"starred": true}); w.Code != http.StatusOK {
		t.Fatalf("star failed: %d %s", w.Code, w.Body.String())
	}

	// The star surfaces on the dashboard collection card with unit context.
	w := notesRequest(t, router, http.MethodGet, "/api/dashboard/starred", student, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("starred list failed: %d %s", w.Code, w.Body.String())
	}
	var starred []starredConceptRow
	if err := json.Unmarshal(w.Body.Bytes(), &starred); err != nil {
		t.Fatal(err)
	}
	if len(starred) != 1 || starred[0].ConceptID != concept.ID || starred[0].Term != concept.Term || starred[0].UnitTitle == "" {
		t.Fatalf("starred row mismatch: %#v", starred)
	}

	// Unstarring empties the list.
	if w := notesRequest(t, router, http.MethodPatch, "/api/concepts/"+concept.ID+"/star", student, map[string]any{"starred": false}); w.Code != http.StatusOK {
		t.Fatalf("unstar failed: %d", w.Code)
	}
	w = notesRequest(t, router, http.MethodGet, "/api/dashboard/starred", student, nil)
	json.Unmarshal(w.Body.Bytes(), &starred)
	if len(starred) != 0 {
		t.Fatalf("unstarred concept still listed: %#v", starred)
	}
}

func fetchAnnouncements(t *testing.T, router http.Handler, token string) []Announcement {
	t.Helper()
	w := notesRequest(t, router, http.MethodGet, "/api/announcements", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list announcements failed: %d %s", w.Code, w.Body.String())
	}
	var rows []Announcement
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func fetchCalendar(t *testing.T, router http.Handler, token, month string) []CalendarEvent {
	t.Helper()
	path := "/api/calendar"
	if month != "" {
		path += "?month=" + month
	}
	w := notesRequest(t, router, http.MethodGet, path, token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("calendar %s failed: %d %s", month, w.Code, w.Body.String())
	}
	var events []CalendarEvent
	if err := json.Unmarshal(w.Body.Bytes(), &events); err != nil {
		t.Fatal(err)
	}
	return events
}

func hasEvent(events []CalendarEvent, title string) bool {
	for _, e := range events {
		if e.Title == title {
			return true
		}
	}
	return false
}
