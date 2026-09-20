package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func reviewNextRequest(t *testing.T, router http.Handler, token, query string) []conceptStateLite {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/review/next?"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("review next failed: %d %s", w.Code, w.Body.String())
	}
	var rows []conceptStateLite
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

// conceptTopics maps concept id -> topic id straight from the DB, so tests can
// verify deck scoping even though /api/review/next no longer ships concept rows.
func conceptTopics(t *testing.T, app *App) map[string]string {
	t.Helper()
	var concepts []Concept
	if err := app.DB.Select("id", "topic_id").Find(&concepts).Error; err != nil {
		t.Fatal(err)
	}
	m := make(map[string]string, len(concepts))
	for _, c := range concepts {
		m[c.ID] = c.TopicID
	}
	return m
}

func markConcept(t *testing.T, router http.Handler, token, conceptID, response string) {
	t.Helper()
	body := bytes.NewBufferString(`{"conceptId":"` + conceptID + `","response":"` + response + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/review/events", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("review event failed: %d %s", w.Code, w.Body.String())
	}
}

func newFlashcardTestApp(t *testing.T) (*App, http.Handler, string) {
	t.Helper()
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := app.DB.DB(); err == nil {
			sqlDB.Close()
		}
	})
	router := app.Router()
	token := registerTestUser(t, router, "flashcards@example.com")
	return app, router, token
}

func topicWithConcepts(t *testing.T, app *App, offset int) (Topic, int64) {
	t.Helper()
	var topics []Topic
	if err := app.DB.Order("id").Find(&topics).Error; err != nil {
		t.Fatal(err)
	}
	topic := topics[offset]
	var count int64
	if err := app.DB.Model(&Concept{}).Where("topic_id = ?", topic.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count < 3 {
		t.Fatalf("topic %s has only %d concepts, need at least 3", topic.ID, count)
	}
	return topic, count
}

func TestReviewNextTopicIDsFilter(t *testing.T) {
	app, router, token := newFlashcardTestApp(t)
	topicA, countA := topicWithConcepts(t, app, 0)
	topicB, countB := topicWithConcepts(t, app, 1)
	topicC, countC := topicWithConcepts(t, app, 2)

	// topicIds selects across multiple topics; topicId unions into the list.
	rows := reviewNextRequest(t, router, token,
		fmt.Sprintf("topicIds=%s,%s&topicId=%s&limit=200&order=outline", topicA.ID, topicB.ID, topicC.ID))
	if int64(len(rows)) != countA+countB+countC {
		t.Fatalf("expected %d concepts across three topics, got %d", countA+countB+countC, len(rows))
	}
	topics := conceptTopics(t, app)
	allowed := map[string]bool{topicA.ID: true, topicB.ID: true, topicC.ID: true}
	for _, row := range rows {
		if !allowed[topics[row.ConceptID]] {
			t.Fatalf("concept %s has unexpected topic %s", row.ConceptID, topics[row.ConceptID])
		}
	}

	// Topic ids take precedence over unitId.
	rows = reviewNextRequest(t, router, token,
		fmt.Sprintf("unitId=%s&topicIds=%s&limit=200", topicB.UnitID, topicA.ID))
	for _, row := range rows {
		if topics[row.ConceptID] != topicA.ID {
			t.Fatalf("topicIds should win over unitId, got topic %s", topics[row.ConceptID])
		}
	}

	// The deck must stay slim: no concept payload on the wire.
	req := httptest.NewRequest(http.MethodGet, "/api/review/next?limit=5", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("review next failed: %d %s", w.Code, w.Body.String())
	}
	var raw []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, row := range raw {
		for key := range row {
			switch key {
			case "conceptId", "status", "reviewCount", "shortTermReview", "starred":
			default:
				t.Fatalf("deck row carries concept payload field %q: %v", key, row)
			}
		}
	}
}

// order=outline must reproduce unit/topic/position order exactly. The two
// orderings are mutually exclusive: appending the outline order after the
// random one leaves random() ahead of the position columns, so the deck comes
// out random despite the request.
func TestReviewNextOutlineOrder(t *testing.T) {
	app, router, token := newFlashcardTestApp(t)

	var expected []string
	if err := app.DB.Raw(`
		select c.id from concepts c
		join units u on u.id = c.unit_id
		join topics tp on tp.id = c.topic_id
		order by u.position asc, tp.position asc, c.position asc
		limit 200
	`).Scan(&expected).Error; err != nil {
		t.Fatal(err)
	}

	rows := reviewNextRequest(t, router, token, "limit=200&order=outline")
	if len(rows) != len(expected) {
		t.Fatalf("expected %d cards, got %d", len(expected), len(rows))
	}
	for i, row := range rows {
		if row.ConceptID != expected[i] {
			t.Fatalf("position %d: got %s, want %s — deck is not in outline order", i, row.ConceptID, expected[i])
		}
	}
}

func TestReviewNextStatusFilters(t *testing.T) {
	app, router, token := newFlashcardTestApp(t)
	topic, total := topicWithConcepts(t, app, 0)

	var concepts []Concept
	if err := app.DB.Where("topic_id = ?", topic.ID).Order("position").Find(&concepts).Error; err != nil {
		t.Fatal(err)
	}
	fuzzyID, unknownID := concepts[0].ID, concepts[1].ID
	markConcept(t, router, token, fuzzyID, "fuzzy")
	markConcept(t, router, token, unknownID, "unknown")

	unmarked := reviewNextRequest(t, router, token,
		fmt.Sprintf("topicId=%s&status=unmarked&limit=200", topic.ID))
	if int64(len(unmarked)) != total-2 {
		t.Fatalf("expected %d unmarked concepts, got %d", total-2, len(unmarked))
	}
	for _, row := range unmarked {
		if row.ConceptID == fuzzyID || row.ConceptID == unknownID {
			t.Fatalf("marked concept %s leaked into unmarked filter", row.ConceptID)
		}
	}

	multi := reviewNextRequest(t, router, token,
		fmt.Sprintf("topicId=%s&status=fuzzy,unknown&limit=200", topic.ID))
	if len(multi) != 2 {
		t.Fatalf("expected 2 concepts for status=fuzzy,unknown, got %d", len(multi))
	}
	seen := map[string]bool{}
	for _, row := range multi {
		seen[row.ConceptID] = true
	}
	if !seen[fuzzyID] || !seen[unknownID] {
		t.Fatalf("status=fuzzy,unknown should return both marked concepts, got %#v", seen)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/review/next?status=bogus", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid status should be rejected: %d %s", w.Code, w.Body.String())
	}
}
