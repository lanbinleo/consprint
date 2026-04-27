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

func TestRatingAndReviewFlow(t *testing.T) {
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
			Mastery float64 `json:"mastery"`
		} `json:"state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &concepts); err != nil || len(concepts) != 1 {
		t.Fatalf("expected one concept, got %d: %v", len(concepts), err)
	}
	conceptID := concepts[0].ID

	req = httptest.NewRequest(http.MethodPatch, "/api/concepts/"+conceptID+"/rating", bytes.NewBufferString(`{"rating":3}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("rating failed: %d %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/review/events", bytes.NewBufferString(`{"conceptId":"`+conceptID+`","response":"know"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("review failed: %d %s", w.Code, w.Body.String())
	}
	var payload struct {
		State struct {
			Mastery float64 `json:"mastery"`
		} `json:"state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.State.Mastery <= 3 || payload.State.Mastery > 5 {
		t.Fatalf("expected mastery to increase from 3, got %f", payload.State.Mastery)
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
	req := httptest.NewRequest(http.MethodPatch, "/api/me", bytes.NewBufferString(`{"name":"David","tenantName":"AP Room","avatarDataUrl":"`+avatar+`"}`))
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
	if profile.User.Name != "David" || profile.Tenant.Name != "AP Room" || profile.User.AvatarDataURL != avatar || profile.User.Role != "admin" {
		t.Fatalf("profile was not updated: %#v", profile)
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
