package backend

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

type contentVersionPayload struct {
	ContentVersion int64 `json:"contentVersion"`
	ConceptCount   int64 `json:"conceptCount"`
	StateVersion   int64 `json:"stateVersion"`
}

func getContentVersion(t *testing.T, router http.Handler, token string) contentVersionPayload {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/content/version", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("content version failed: %d %s", w.Code, w.Body.String())
	}
	var payload contentVersionPayload
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func patchStatus(t *testing.T, router http.Handler, token, conceptID, status string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/concepts/"+conceptID+"/status", bytes.NewBufferString(`{"status":"`+status+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status update failed: %d %s", w.Code, w.Body.String())
	}
}

// The version endpoint is the client cache's change signal: marks must bump
// only stateVersion, content edits only contentVersion, so neither signal
// invalidates the other's cache unnecessarily.
func TestContentVersionSignal(t *testing.T) {
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
	token := registerTestUser(t, router, "version@example.com")

	before := getContentVersion(t, router, token)
	if before.ContentVersion == 0 || before.ConceptCount != 794 || before.StateVersion == 0 {
		t.Fatalf("unexpected baseline version: %#v", before)
	}

	// ensureStates stamps all rows in one burst; give the mark its own
	// millisecond so the version comparison cannot collide.
	time.Sleep(10 * time.Millisecond)
	conceptID := "ap-psychology.science-practices.set-a.random-assignment"
	patchStatus(t, router, token, conceptID, "fuzzy")

	afterMark := getContentVersion(t, router, token)
	if afterMark.ContentVersion != before.ContentVersion {
		t.Fatalf("mark should not bump contentVersion: %d -> %d", before.ContentVersion, afterMark.ContentVersion)
	}
	if afterMark.StateVersion <= before.StateVersion {
		t.Fatalf("mark should bump stateVersion: %d -> %d", before.StateVersion, afterMark.StateVersion)
	}

	// The first registered user bootstraps as admin, so a content edit is allowed.
	req := httptest.NewRequest(http.MethodPatch, "/api/concepts/"+conceptID+"/content", bytes.NewBufferString(`{
		"definition":[{"type":"paragraph","text":"Manual definition"}],
		"examples":[],"pitfalls":[],"notes":[],"source":"version-test"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("content update failed: %d %s", w.Code, w.Body.String())
	}

	afterEdit := getContentVersion(t, router, token)
	if afterEdit.ContentVersion <= afterMark.ContentVersion {
		t.Fatalf("content edit should bump contentVersion: %d -> %d", afterMark.ContentVersion, afterEdit.ContentVersion)
	}
}

// The concept list is the client cache's payload: concept + content only, no
// relation objects or per-user state (those live in /api/units and
// /api/concepts/states), and updatedSince returns only changed rows.
func TestConceptsListSlimAndDelta(t *testing.T) {
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
	token := registerTestUser(t, router, "slim@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/concepts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("concepts failed: %d %s", w.Code, w.Body.String())
	}
	var raw []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 794 {
		t.Fatalf("expected 794 concepts, got %d", len(raw))
	}
	for _, row := range raw {
		for key := range row {
			switch key {
			case "id", "courseId", "unitId", "topicId", "term", "normalizedTerm", "position", "contentStatus", "content", "createdAt", "updatedAt":
			default:
				t.Fatalf("concept list carries unexpected field %q", key)
			}
		}
		if row["content"] == nil {
			t.Fatalf("concept %v has no content", row["id"])
		}
	}

	// Delta: rows whose updated_at is at/after the boundary come back, the
	// rest are assumed unchanged in the caller's cache. The edited row gets a
	// deterministic future timestamp so import-burst rows written in the same
	// wall-clock second can never leak into the assertion.
	conceptID := "ap-psychology.science-practices.set-a.random-assignment"
	future := time.Now().Add(20 * time.Second)
	if err := app.DB.Model(&Concept{}).Where("id = ?", conceptID).Update("updated_at", future).Error; err != nil {
		t.Fatal(err)
	}
	since := time.Now().Add(10 * time.Second).UTC().Format(time.RFC3339Nano)
	req = httptest.NewRequest(http.MethodGet, "/api/concepts?updatedSince="+since, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delta fetch failed: %d %s", w.Code, w.Body.String())
	}
	var delta []Concept
	if err := json.Unmarshal(w.Body.Bytes(), &delta); err != nil {
		t.Fatal(err)
	}
	if len(delta) != 1 || delta[0].ID != conceptID {
		t.Fatalf("delta should return exactly the edited concept, got %d rows", len(delta))
	}

	// Malformed boundary is a client bug: reject loudly, not silently-all.
	req = httptest.NewRequest(http.MethodGet, "/api/concepts?updatedSince=not-a-date", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid updatedSince should 400, got %d", w.Code)
	}
}

// conceptStates returns only non-default rows; absent ids are the default
// state. A fresh account with one mark yields exactly that one row.
func TestConceptStatesSlim(t *testing.T) {
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
	token := registerTestUser(t, router, "states@example.com")

	conceptID := "ap-psychology.science-practices.set-a.random-assignment"
	patchStatus(t, router, token, conceptID, "unknown")

	req := httptest.NewRequest(http.MethodGet, "/api/concepts/states", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("states failed: %d %s", w.Code, w.Body.String())
	}
	var states []conceptStateLite
	if err := json.Unmarshal(w.Body.Bytes(), &states); err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 {
		t.Fatalf("expected exactly one non-default state, got %d: %s", len(states), w.Body.String())
	}
	got := states[0]
	if got.ConceptID != conceptID || got.Status != "unknown" || !got.ShortTermReview || got.ReviewCount != 1 {
		t.Fatalf("unexpected state row: %#v", got)
	}
}
