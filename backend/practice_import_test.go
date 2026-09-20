package backend

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JSON import extension: inline shared stimuli (deduped by title), concept
// refs, and base64 image localization into /files storage.
func TestQuestionImportJSONWithInlineStimulusAndImages(t *testing.T) {
	app, router := newTestApp(t)
	root := registerTestUser(t, router, "jsonimp-root@example.com")
	teacher := registerTestUser(t, router, "jsonimp-teacher@example.com")
	promoteUser(t, router, root, "jsonimp-teacher@example.com", "teacher")

	tinyPNGData := "data:image/png;base64," + base64.StdEncoding.EncodeToString(tinyPNG)
	payload := `[
		{
			"type":"subjective","format":"aaq",
			"stem":"Use Source 1 to answer in parts.",
			"stimulus":{"title":"Mindful Drinking Study","kind":"article","documents":[{"title":"Source","text":"Researchers surveyed students. ![fig](` + tinyPNGData + `)"}]},
			"parts":[{"label":"A","prompt":"Method?","referenceAnswer":"Survey.","rubric":["Names method"]}]
		},
		{
			"type":"subjective","format":"aaq",
			"stem":"Follow-up question on the same article.",
			"stimulus":{"title":"Mindful Drinking Study","kind":"article","documents":[{"title":"Source","text":"Same article shared by title."}]},
			"parts":[{"label":"A","prompt":"Variable?","referenceAnswer":"Ounces.","rubric":["Names variable"]}]
		},
		{"type":"subjective","format":"aaq","stem":"Missing stimulus should fail.","parts":[{"label":"A","prompt":"x","referenceAnswer":"y"}]}
	]`
	preview := uploadImport(t, router, teacher, "aaq.json", payload)
	if preview["valid"].(float64) != 2 {
		t.Fatalf("expected 2 valid rows, got %v errors %v", preview["valid"], preview["errors"])
	}
	commitBody := bytes.NewBufferString(`{"importId":"` + preview["importId"].(string) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/questions/import/commit", commitBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+teacher)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("commit failed: %d %s", w.Code, w.Body.String())
	}

	// Both rows share one stimulus, resolved by title.
	var stimuli []Stimulus
	app.DB.Find(&stimuli)
	if len(stimuli) != 1 {
		t.Fatalf("expected 1 shared stimulus, got %d", len(stimuli))
	}
	var questions []Question
	app.DB.Where("type = ?", "subjective").Find(&questions)
	if len(questions) != 2 {
		t.Fatalf("expected 2 imported questions, got %d", len(questions))
	}
	for _, question := range questions {
		if question.StimulusID == nil || *question.StimulusID != stimuli[0].ID {
			t.Fatalf("question %s not linked to the shared stimulus", question.ID)
		}
	}

	// The base64 figure in the article landed as a /files image and the doc
	// text now references the stored URL.
	docs := stimulusDocuments(stimuli[0].Documents)
	if len(docs) != 1 || !strings.Contains(docs[0].Text, "/files/qimg-") {
		t.Fatalf("data url not localized: %q", docs[0].Text)
	}
	if strings.Contains(docs[0].Text, "base64") {
		t.Fatal("base64 payload should be gone after localization")
	}
	text := docs[0].Text
	url := text[strings.Index(text, "/files/qimg-"):]
	if end := strings.IndexAny(url, ")\n "); end > 0 {
		url = url[:end]
	}
	stored, err := os.Stat(filepath.Join(app.PublicDir, filepath.Base(url)))
	if err != nil || stored.Size() != int64(len(tinyPNG)) {
		t.Fatalf("localized image not stored: %v (size %v)", err, stored)
	}
}
