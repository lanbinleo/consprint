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

	body := bytes.NewBufferString(`{"tenantName":"Test","name":"Student","email":"flow@example.com","password":"secret"}`)
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

	req = httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+auth.Token)
	w = httptest.NewRecorder()
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
