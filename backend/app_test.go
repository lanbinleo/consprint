package backend

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestAuthAndDashboardFlow(t *testing.T) {
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	router := app.Router()

	token := registerTestUser(t, router, "flow@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard failed: %d %s", w.Code, w.Body.String())
	}
	var dashboard struct {
		TotalConcepts int `json:"totalConcepts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &dashboard); err != nil {
		t.Fatal(err)
	}
	if dashboard.TotalConcepts != 794 {
		t.Fatalf("expected 794 canonical concepts, got %d", dashboard.TotalConcepts)
	}
}

func TestConceptStatusAndReviewFlow(t *testing.T) {
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	router := app.Router()
	token := registerTestUser(t, router, "review@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/concepts?search=random%20assignment", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("concept search failed: %d %s", w.Code, w.Body.String())
	}
	var concepts []struct {
		ID    string `json:"id"`
		Term  string `json:"term"`
		State struct {
			Status string `json:"status"`
		} `json:"state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &concepts); err != nil || len(concepts) != 1 {
		t.Fatalf("expected one concept, got %d: %v", len(concepts), err)
	}
	conceptID := concepts[0].ID

	req = httptest.NewRequest(http.MethodPatch, "/api/concepts/"+conceptID+"/status", bytes.NewBufferString(`{"status":"fuzzy"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status update failed: %d %s", w.Code, w.Body.String())
	}
	var state struct {
		Status          string `json:"status"`
		ShortTermReview bool   `json:"shortTermReview"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != "fuzzy" || !state.ShortTermReview {
		t.Fatalf("fuzzy status should enter short-term review: %#v", state)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/review/events", bytes.NewBufferString(`{"conceptId":"`+conceptID+`","response":"proficient"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("review failed: %d %s", w.Code, w.Body.String())
	}
	var payload struct {
		State struct {
			Status          string `json:"status"`
			ShortTermReview bool   `json:"shortTermReview"`
			ReviewCount     int    `json:"reviewCount"`
		} `json:"state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State.Status != "proficient" || payload.State.ShortTermReview || payload.State.ReviewCount != 2 {
		// reviewCount is 2: one from the list-view status mark above, one
		// from this flashcard review event.
		t.Fatalf("proficient response should clear short-term review: %#v", payload.State)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/dashboard/progress", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard progress failed: %d %s", w.Code, w.Body.String())
	}
	var progress struct {
		MarkedConcepts     int `json:"markedConcepts"`
		ProficientConcepts int `json:"proficientConcepts"`
		TodayReviews       int `json:"todayReviews"`
		ShortTermReviews   int `json:"shortTermReviews"`
		StreakDays         int `json:"streakDays"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &progress); err != nil {
		t.Fatal(err)
	}
	if progress.MarkedConcepts == 0 || progress.ProficientConcepts != 1 || progress.TodayReviews != 2 || progress.StreakDays == 0 {
		// Two events today: the list-view status mark and the flashcard review.
		t.Fatalf("dashboard progress did not reflect review: %#v", progress)
	}
	if progress.ShortTermReviews != 0 {
		t.Fatalf("proficient response should not stay in short-term review: %#v", progress)
	}
}

func TestProfileAndContentUpdateFlow(t *testing.T) {
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	router := app.Router()
	token := registerTestUser(t, router, "profile@example.com")

	avatar := "data:image/png;base64,iVBORw0KGgo="
	req := httptest.NewRequest(http.MethodPatch, "/api/me", bytes.NewBufferString(`{"name":"David","avatarDataUrl":"`+avatar+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("profile update failed: %d %s", w.Code, w.Body.String())
	}
	var profile struct {
		User   User   `json:"user"`
		Tenant Tenant `json:"tenant"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	if profile.User.Name != "David" || profile.User.AvatarDataURL != avatar || profile.User.Role != "admin" {
		t.Fatalf("profile was not updated: %#v", profile)
	}
	if profile.Tenant.ID != "school" {
		t.Fatalf("expected single school tenant, got %#v", profile.Tenant)
	}

	conceptID := "ap-psychology.science-practices.set-a.random-assignment"
	req = httptest.NewRequest(http.MethodPatch, "/api/concepts/"+conceptID+"/content", bytes.NewBufferString(`{
		"definition":[{"type":"paragraph","text":"Manual definition"}],
		"examples":[{"type":"paragraph","text":"Manual example"}],
		"pitfalls":[],
		"notes":[],
		"source":"manual-test"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("content update failed: %d %s", w.Code, w.Body.String())
	}
	var concept Concept
	if err := json.Unmarshal(w.Body.Bytes(), &concept); err != nil {
		t.Fatal(err)
	}
	if concept.Content == nil || concept.Content.Source != "manual-test" {
		t.Fatalf("content was not saved: %#v", concept.Content)
	}
}

func TestAdminOnlyDataRoutes(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "admin@example.com")
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	router := app.Router()
	_ = registerTestUser(t, router, "admin@example.com")
	studentToken := registerTestUser(t, router, "student@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/import/status", nil)
	req.Header.Set("Authorization", "Bearer "+studentToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("student should not read import status: %d %s", w.Code, w.Body.String())
	}
}

func TestAdminEmailBootstrap(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "leo.huo_27@tsinglan.org")
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	router := app.Router()

	body := bytes.NewBufferString(`{"name":"Leo","email":"leo.huo_27@tsinglan.org","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var auth struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.User.Role != "admin" {
		t.Fatalf("ADMIN_EMAILS address should register as admin, got %q", auth.User.Role)
	}
}

func TestRegistrationInviteCode(t *testing.T) {
	t.Setenv("REGISTRATION_INVITE_CODE", "class-2026")
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	router := app.Router()

	body := bytes.NewBufferString(`{"tenantName":"Test","name":"Student","email":"blocked@example.com","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("registration without invite should be forbidden: %d %s", w.Code, w.Body.String())
	}

	body = bytes.NewBufferString(`{"tenantName":"Test","name":"Student","email":"invited@example.com","password":"secret","inviteCode":"class-2026"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("registration with invite failed: %d %s", w.Code, w.Body.String())
	}
}

func registerTestUser(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	body := bytes.NewBufferString(`{"tenantName":"Test","name":"Student","email":"` + email + `","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var auth struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &auth); err != nil || auth.Token == "" {
		t.Fatalf("missing auth token: %v", err)
	}
	return auth.Token
}
