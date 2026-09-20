package backend

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// Stage-1 coverage for the practice upgrade: shared stimuli, AAQ/EBQ
// per-part answers and self-ratings, the attempt history list, set coverage
// rollups, and question image uploads.

const aaqStimulusDocs = `[
	{"title":"Study: Sleep and Exam Performance","text":"Researchers surveyed 200 college students about sleep duration and recorded their exam scores. ![figure](/files/qimg-example.png)"}
]`

func createAAQStimulus(t *testing.T, router http.Handler, token string) string {
	t.Helper()
	body := `{"title":"Sleep and Exam Performance","kind":"article","documents":` + aaqStimulusDocs + `}`
	stimulus := postJSON(t, router, token, "POST", "/api/admin/stimuli", body)
	id, _ := stimulus["id"].(string)
	if id == "" {
		t.Fatalf("stimulus create failed: %v", stimulus)
	}
	return id
}

func firstConceptID(t *testing.T, app *App) string {
	t.Helper()
	var concept Concept
	if err := app.DB.Order("position asc").First(&concept).Error; err != nil {
		t.Fatal(err)
	}
	return concept.ID
}

func TestStimulusCRUD(t *testing.T) {
	_, router := newTestApp(t)
	root := registerTestUser(t, router, "stimulus-root@example.com")
	token := registerTestUser(t, router, "stimulus-admin@example.com")
	promoteUser(t, router, root, "stimulus-admin@example.com", "teacher")
	student := registerTestUser(t, router, "stimulus-student@example.com")

	// Students cannot manage stimuli.
	w := doRequestRaw(t, router, student, "POST", "/api/admin/stimuli", `{"title":"x","documents":[{"title":"d","text":"t"}]}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("student stimulus create should be forbidden: %d", w.Code)
	}

	// Validation: kind and documents.
	w = doRequestRaw(t, router, token, "POST", "/api/admin/stimuli", `{"title":"No docs","kind":"article","documents":[]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty documents should 400: %d %s", w.Code, w.Body.String())
	}
	w = doRequestRaw(t, router, token, "POST", "/api/admin/stimuli", `{"title":"Bad kind","kind":"video","documents":[{"title":"d","text":"t"}]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad kind should 400: %d", w.Code)
	}

	id := createAAQStimulus(t, router, token)
	list := listJSON(t, router, token, "/api/admin/stimuli?search=sleep")
	if len(list) != 1 {
		t.Fatalf("expected 1 stimulus in search, got %d", len(list))
	}
	row := list[0]
	if row["kind"] != "article" || row["documentCount"].(float64) != 1 {
		t.Fatalf("unexpected stimulus row: %v", row)
	}
	if _, hasQuestions := row["questions"]; !hasQuestions {
		t.Fatal("stimulus row should carry a questions usage count")
	}

	// PATCH updates the title and keeps documents.
	patched := postJSON(t, router, token, "PATCH", "/api/admin/stimuli/"+id, `{"title":"Sleep study (revised)"}`)
	if patched["title"] != "Sleep study (revised)" {
		t.Fatalf("patch failed: %v", patched)
	}
}

// doRequestRaw is doRequest without the 200 assertion, for negative cases.
func doRequestRaw(t *testing.T, router http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func listJSON(t *testing.T, router http.Handler, token, path string) []map[string]any {
	t.Helper()
	w := doRequest(t, router, token, "GET", path, "")
	var out []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid list json from %s: %v", path, err)
	}
	return out
}

func TestAAQPerPartFlowAndRevealGating(t *testing.T) {
	app, router := newTestApp(t)
	root := registerTestUser(t, router, "aaq-root@example.com")
	teacher := registerTestUser(t, router, "aaq-teacher@example.com")
	promoteUser(t, router, root, "aaq-teacher@example.com", "teacher")
	student := registerTestUser(t, router, "aaq-student@example.com")

	stimulusID := createAAQStimulus(t, router, teacher)
	conceptID := firstConceptID(t, app)

	// AAQ without a stimulus is rejected; with stimulus + parts + concepts
	// it is created and carries concept chips.
	w := doRequestRaw(t, router, teacher, "POST", "/api/admin/questions", `{
		"type":"subjective","format":"aaq","stem":"Use the source to answer the questions.",
		"parts":[{"label":"A","prompt":"Identify the research method.","referenceAnswer":"Correlational survey.","rubric":["Names the method"]},{"label":"B","prompt":"Operationally define the DV.","referenceAnswer":"Exam score.","rubric":["Measurable definition"]}],
		"concepts":["`+conceptID+`"],"unit":"2"
	}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("aaq without stimulus should 400: %d %s", w.Code, w.Body.String())
	}
	created := postJSON(t, router, teacher, "POST", "/api/admin/questions", `{
		"type":"subjective","format":"aaq","stimulusId":"`+stimulusID+`","stem":"Use the source to answer the questions.",
		"parts":[{"label":"A","prompt":"Identify the research method.","referenceAnswer":"Correlational survey.","rubric":["Names the method"]},{"label":"B","prompt":"Operationally define the DV.","referenceAnswer":"Exam score.","rubric":["Measurable definition"]}],
		"concepts":["`+conceptID+`"],"unit":"2"
	}`)
	aaqID, _ := created["id"].(string)
	if aaqID == "" || created["format"] != "aaq" {
		t.Fatalf("aaq create failed: %v", created)
	}
	if concepts, ok := created["concepts"].([]any); !ok || len(concepts) != 1 {
		t.Fatalf("concept chip missing: %v", created["concepts"])
	}

	// An MCQ sharing the same stimulus forms a cluster in the set.
	mcq := postJSON(t, router, teacher, "POST", "/api/admin/questions", `{
		"type":"mcq","stimulusId":"`+stimulusID+`","stem":"Which variable was manipulated?",
		"choices":[{"key":"A","text":"Sleep duration"},{"key":"B","text":"Exam score"}],
		"answerKey":"A","explanation":"Survey measured both.","unit":"2"
	}`)
	mcqID, _ := mcq["id"].(string)

	// Sibling lookup by stimulus powers the assembly "add whole group" flow.
	siblings := listJSON(t, router, teacher, "/api/admin/questions?stimulusId="+stimulusID+"&limit=50")
	if len(siblings) != 2 {
		t.Fatalf("expected 2 questions sharing stimulus, got %d", len(siblings))
	}

	set := postJSON(t, router, teacher, "POST", "/api/admin/sets", `{"title":"AAQ practice","mode":"instant","status":"published","questionIds":["`+mcqID+`","`+aaqID+`"]}`)
	setID, _ := set["id"].(string)

	detail := postJSON(t, router, student, "GET", "/api/practice/sets/"+setID, "")
	stimuli := detail["stimuli"].([]any)
	if len(stimuli) != 1 {
		t.Fatalf("expected 1 stimulus in set detail, got %d", len(stimuli))
	}
	coverage := detail["coverage"].(map[string]any)
	if coverage["unlinked"].(float64) != 0 {
		t.Fatalf("expected full coverage, got %v", coverage)
	}
	units := coverage["units"].([]any)
	if len(units) != 1 {
		t.Fatalf("expected 1 covered unit, got %v", units)
	}
	// Student detail must leak neither part reference answers nor rubrics.
	for _, raw := range detail["questions"].([]any) {
		question := raw.(map[string]any)
		if question["id"] == aaqID {
			for _, partRaw := range question["parts"].([]any) {
				part := partRaw.(map[string]any)
				if _, has := part["referenceAnswer"]; has {
					t.Fatal("student set detail leaked a reference answer")
				}
				if _, has := part["rubric"]; has {
					t.Fatal("student set detail leaked a rubric")
				}
			}
		}
	}

	attempt := postJSON(t, router, student, "POST", "/api/practice/attempts", `{"setId":"`+setID+`"}`)
	attemptID, _ := attempt["attempt"].(map[string]any)["id"].(string)

	// Per-part submission: unknown labels and all-empty answers are rejected.
	w = doRequestRaw(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/answers", `{"questionId":"`+aaqID+`","parts":[{"label":"Z","text":"nope"}]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown part label should 400: %d %s", w.Code, w.Body.String())
	}
	w = doRequestRaw(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/answers", `{"questionId":"`+aaqID+`","parts":[{"label":"A","text":" "},{"label":"B","text":""}]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("all-empty parts should 400: %d %s", w.Code, w.Body.String())
	}

	stored := postJSON(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/answers",
		`{"questionId":"`+aaqID+`","parts":[{"label":"A","text":"It is a correlational survey."},{"label":"B","text":"Exam score percentage."}]}`)
	answer := stored["answer"].(map[string]any)
	parts := answer["parts"].([]any)
	if len(parts) != 2 || parts[0].(map[string]any)["text"] != "It is a correlational survey." {
		t.Fatalf("per-part answer not stored: %v", answer)
	}
	if stored["question"].(map[string]any)["id"] != aaqID {
		t.Fatal("instant mode should return the question with reveal")
	}

	// Per-part self-ratings aggregate into the wrong-book verdict.
	rated := postJSON(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/answers/"+aaqID+"/self-rating",
		`{"parts":[{"label":"A","rating":"proficient"},{"label":"B","rating":"weak"}]}`)
	if rated["selfRating"] != "weak" {
		t.Fatalf("aggregate should be weak: %v", rated["selfRating"])
	}
	postJSON(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/finish", "")

	wrongBook := listJSON(t, router, student, "/api/practice/wrongbook")
	found := false
	for _, entry := range wrongBook {
		if entry["question"].(map[string]any)["id"] == aaqID {
			found = true
			if entry["reason"] != "weak" {
				t.Fatalf("aaq should land in the wrong book as weak: %v", entry["reason"])
			}
		}
	}
	if !found {
		t.Fatal("aaq question missing from wrong book")
	}

	// All-proficient re-rating evicts it from the book.
	postJSON(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/answers/"+aaqID+"/self-rating",
		`{"parts":[{"label":"A","rating":"proficient"},{"label":"B","rating":"proficient"}]}`)
	wrongBook = listJSON(t, router, student, "/api/practice/wrongbook")
	for _, entry := range wrongBook {
		if entry["question"].(map[string]any)["id"] == aaqID {
			t.Fatal("all-proficient aaq should leave the wrong book")
		}
	}
}

func TestAAQExamModeGatingAndHistory(t *testing.T) {
	_, router := newTestApp(t)
	// Register a placeholder admin first so the teacher account stays a plain
	// user and can be promoted (the first user otherwise auto-admins).
	root := registerTestUser(t, router, "exam-teacher-root@example.com")
	teacher := registerTestUser(t, router, "exam-teacher@example.com")
	promoteUser(t, router, root, "exam-teacher@example.com", "teacher")
	student := registerTestUser(t, router, "exam-student@example.com")
	other := registerTestUser(t, router, "exam-other@example.com")

	stimulusID := createAAQStimulus(t, router, teacher)
	created := postJSON(t, router, teacher, "POST", "/api/admin/questions", `{
		"type":"subjective","format":"aaq","stimulusId":"`+stimulusID+`","stem":"Answer in parts.",
		"parts":[{"label":"A","prompt":"Method?","referenceAnswer":"Survey.","rubric":["Names method"]}]
	}`)
	aaqID, _ := created["id"].(string)
	set := postJSON(t, router, teacher, "POST", "/api/admin/sets", `{"title":"AAQ exam","mode":"exam","timeLimitSec":900,"status":"published","questionIds":["`+aaqID+`"]}`)
	setID, _ := set["id"].(string)

	attempt := postJSON(t, router, student, "POST", "/api/practice/attempts", `{"setId":"`+setID+`"}`)
	attemptID, _ := attempt["attempt"].(map[string]any)["id"].(string)

	// Exam mode stores the answer without revealing anything.
	stored := postJSON(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/answers",
		`{"questionId":"`+aaqID+`","parts":[{"label":"A","text":"A survey."}]}`)
	if _, has := stored["question"]; has {
		t.Fatal("exam mode must not reveal the question on submit")
	}
	// Per-part self-rating is blocked while the exam is open.
	w := doRequestRaw(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/answers/"+aaqID+"/self-rating", `{"parts":[{"label":"A","rating":"weak"}]}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("part self-rating during open exam should 409: %d", w.Code)
	}
	// attemptDetail before finish must not leak reference answers.
	detail := postJSON(t, router, student, "GET", "/api/practice/attempts/"+attemptID, "")
	for _, raw := range detail["questions"].([]any) {
		for _, partRaw := range raw.(map[string]any)["parts"].([]any) {
			if _, has := partRaw.(map[string]any)["referenceAnswer"]; has {
				t.Fatal("open exam attempt detail leaked a reference answer")
			}
		}
	}

	// Another student's attempt is invisible.
	w = doRequestRaw(t, router, other, "GET", "/api/practice/attempts/"+attemptID, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign attempt detail should 403: %d", w.Code)
	}

	postJSON(t, router, student, "POST", "/api/practice/attempts/"+attemptID+"/finish", "")

	// The attempt history list: own attempts only, with progress + title.
	history := listJSON(t, router, student, "/api/practice/attempts")
	if len(history) != 1 {
		t.Fatalf("expected 1 history row, got %d", len(history))
	}
	row := history[0]
	if row["setTitle"] != "AAQ exam" || row["answeredCount"].(float64) != 1 || row["questionCount"].(float64) != 1 {
		t.Fatalf("unexpected history row: %v", row)
	}
	otherHistory := listJSON(t, router, other, "/api/practice/attempts")
	if len(otherHistory) != 0 {
		t.Fatalf("other student should have no history rows, got %d", len(otherHistory))
	}
}

func TestQuestionImageUpload(t *testing.T) {
	_, router := newTestApp(t)
	// Register an admin first so the teacher account stays a plain user and
	// can actually be promoted (the first user otherwise auto-admins).
	root := registerTestUser(t, router, "qimg-admin@example.com")
	teacher := registerTestUser(t, router, "qimg-teacher@example.com")
	promoteUser(t, router, root, "qimg-teacher@example.com", "teacher")
	student := registerTestUser(t, router, "qimg-student@example.com")

	upload := func(token string, filename string, content []byte) *httptest.ResponseRecorder {
		t.Helper()
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", filename)
		part.Write(content)
		writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/question-images", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	if w := upload(student, "chart.png", tinyPNG); w.Code != http.StatusForbidden {
		t.Fatalf("student image upload should be forbidden: %d", w.Code)
	}
	if w := upload(teacher, "notes.txt", []byte("hello")); w.Code != http.StatusBadRequest {
		t.Fatalf("non-image upload should 400: %d", w.Code)
	}

	w := upload(teacher, "研究设计图.png", tinyPNG)
	if w.Code != http.StatusOK {
		t.Fatalf("image upload failed: %d %s", w.Code, w.Body.String())
	}
	var stored struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stored); err != nil {
		t.Fatal(err)
	}
	// The stored name must be unguessable-random, not derived from the
	// original filename: /files is unauthenticated.
	if !strings.HasPrefix(stored.URL, "/files/qimg-") {
		t.Fatalf("unexpected image url: %s", stored.URL)
	}
	if strings.Contains(stored.URL, "研究") || filepath.Ext(stored.URL) != ".png" {
		t.Fatalf("image name should be randomized with a sniffed extension: %s", stored.URL)
	}
	// Two uploads of the same file must not collide.
	second := upload(teacher, "研究设计图.png", tinyPNG)
	var secondStored struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondStored); err != nil {
		t.Fatal(err)
	}
	if secondStored.URL == stored.URL {
		t.Fatalf("identical uploads collided on one filename: %s", stored.URL)
	}

	// /files serves the stored image (no auth header possible on img tags).
	req := httptest.NewRequest(http.MethodGet, stored.URL, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.Len() != len(tinyPNG) {
		t.Fatalf("stored image not served: %d %d bytes", w.Code, w.Body.Len())
	}
}
