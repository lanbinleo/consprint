package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Audit-driven tests for exam-mode answer secrecy and wrong-book derivation.

// rawRequest performs a request without the 200-only assertion of doRequest,
// for the negative cases (409/410/etc.).
func rawRequest(t *testing.T, router http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
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

func importExamQuestions(t *testing.T, router http.Handler, token string) (mcqID, subjectiveID string) {
	t.Helper()
	payload := `[
		{"type":"mcq","stem":"Which method best shows cause and effect?","choices":[{"key":"A","text":"Correlational study"},{"key":"B","text":"Experiment"}],"answerKey":"B","explanation":"Experiments manipulate the IV.","unit":"1"},
		{"type":"subjective","stem":"Explain hindsight bias with an example.","parts":[{"label":"A","prompt":"Explain","referenceAnswer":"SECRET-REFERENCE-ANSWER","rubric":["SECRET-RUBRIC-POINT"]}],"unit":"1"}
	]`
	preview := uploadImport(t, router, token, "audit-questions.json", payload)
	if preview["valid"].(float64) != 2 {
		t.Fatalf("expected 2 valid questions, got %v (errors: %v)", preview["valid"], preview["errors"])
	}
	importID := preview["importId"].(string)
	if w := rawRequest(t, router, token, "POST", "/api/admin/questions/import/commit", fmt.Sprintf(`{"importId":%q}`, importID)); w.Code != 200 {
		t.Fatalf("import commit failed: %d %s", w.Code, w.Body.String())
	}
	w := rawRequest(t, router, token, "GET", "/api/admin/questions?limit=10", "")
	if w.Code != 200 {
		t.Fatalf("question list failed: %d", w.Code)
	}
	var questions []Question
	if err := json.Unmarshal(w.Body.Bytes(), &questions); err != nil {
		t.Fatal(err)
	}
	for _, q := range questions {
		if q.Type == "subjective" {
			subjectiveID = q.ID
		} else {
			mcqID = q.ID
		}
	}
	if mcqID == "" || subjectiveID == "" {
		t.Fatalf("missing imported questions: mcq=%q subjective=%q", mcqID, subjectiveID)
	}
	return mcqID, subjectiveID
}

// An open exam attempt must not accept self-ratings, and the wrong book must
// not reveal subjective reference answers through a "weak" rating before the
// attempt is finished (the answer could then be rewritten before submitting).
func TestExamSelfRatingGatedUntilFinish(t *testing.T) {
	_, router := newTestApp(t)
	// First registrant becomes admin (dev bootstrap), which also satisfies
	// the staff role needed to assemble sets.
	adminToken := registerTestUser(t, router, "gate-admin@example.com")
	studentToken := registerTestUser(t, router, "gate-student@example.com")
	mcqID, subjectiveID := importExamQuestions(t, router, adminToken)

	set := postJSON(t, router, adminToken, "POST", "/api/admin/sets", `{"title":"Audit exam","mode":"exam","timeLimitSec":600,"status":"published","questionIds":["`+mcqID+`","`+subjectiveID+`"]}`)
	setID := set["id"].(string)
	attempt := postJSON(t, router, studentToken, "POST", "/api/practice/attempts", `{"setId":"`+setID+`"}`)
	attemptID := attempt["attempt"].(map[string]any)["id"].(string)

	postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+attemptID+"/answers", `{"questionId":"`+subjectiveID+`","textAnswer":"placeholder"}`)
	postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+attemptID+"/answers", `{"questionId":"`+mcqID+`","choiceKey":"B"}`)

	// Self-rating an open exam answer must be rejected…
	w := rawRequest(t, router, studentToken, "POST", "/api/practice/attempts/"+attemptID+"/answers/"+subjectiveID+"/self-rating", `{"rating":"weak"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("self-rating an open exam answer should be 409, got %d: %s", w.Code, w.Body.String())
	}
	// …and the wrong book must not leak the reference answer or rubric.
	w = rawRequest(t, router, studentToken, "GET", "/api/practice/wrongbook", "")
	if strings.Contains(w.Body.String(), "SECRET-REFERENCE-ANSWER") || strings.Contains(w.Body.String(), "SECRET-RUBRIC-POINT") {
		t.Fatalf("wrong book leaked grading material before the exam was submitted: %s", w.Body.String())
	}

	// After finishing, rating works and the weak entry appears with reveal.
	postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+attemptID+"/finish", "")
	rated := postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+attemptID+"/answers/"+subjectiveID+"/self-rating", `{"rating":"weak"}`)
	if rated["selfRating"] != "weak" {
		t.Fatalf("post-finish self rating should be stored: %v", rated)
	}
	w = rawRequest(t, router, studentToken, "GET", "/api/practice/wrongbook", "")
	if !strings.Contains(w.Body.String(), "SECRET-REFERENCE-ANSWER") {
		t.Fatalf("finished weak subjective answer should be revealed in the wrong book: %s", w.Body.String())
	}

	// MCQ answers can never be self-rated (only subjective ones).
	w = rawRequest(t, router, studentToken, "POST", "/api/practice/attempts/"+attemptID+"/answers/"+mcqID+"/self-rating", `{"rating":"weak"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("mcq self-rating should be rejected with 400, got %d", w.Code)
	}
}

// Latest answer per question drives the wrong book: a later correct answer
// evicts, an ungraded answer from an unfinished exam attempt must NOT evict,
// and a graded wrong answer re-admits.
func TestWrongBookLatestAnswerSemantics(t *testing.T) {
	_, router := newTestApp(t)
	adminToken := registerTestUser(t, router, "wb-admin@example.com")
	studentToken := registerTestUser(t, router, "wb-student@example.com")
	mcqID, _ := importExamQuestions(t, router, adminToken)

	instantSet := postJSON(t, router, adminToken, "POST", "/api/admin/sets", `{"title":"WB instant","mode":"instant","status":"published","questionIds":["`+mcqID+`"]}`)
	instantID := instantSet["id"].(string)
	examSet := postJSON(t, router, adminToken, "POST", "/api/admin/sets", `{"title":"WB exam","mode":"exam","status":"published","questionIds":["`+mcqID+`"]}`)
	examID := examSet["id"].(string)

	entryCount := func() int {
		w := rawRequest(t, router, studentToken, "GET", "/api/practice/wrongbook", "")
		if w.Code != 200 {
			t.Fatalf("wrongbook failed: %d %s", w.Code, w.Body.String())
		}
		var entries []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}

	// Attempt 1 (instant): answer wrong -> question enters the book.
	a1 := postJSON(t, router, studentToken, "POST", "/api/practice/attempts", `{"setId":"`+instantID+`"}`)
	id1 := a1["attempt"].(map[string]any)["id"].(string)
	postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+id1+"/answers", `{"questionId":"`+mcqID+`","choiceKey":"A"}`)
	if got := entryCount(); got != 1 {
		t.Fatalf("wrong answer should enter the wrong book, got %d entries", got)
	}

	// Attempt 2 (instant): answer correctly -> latest answer evicts.
	a2 := postJSON(t, router, studentToken, "POST", "/api/practice/attempts", `{"setId":"`+instantID+`"}`)
	id2 := a2["attempt"].(map[string]any)["id"].(string)
	postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+id2+"/answers", `{"questionId":"`+mcqID+`","choiceKey":"B"}`)
	if got := entryCount(); got != 0 {
		t.Fatalf("later correct answer should evict from the wrong book, got %d entries", got)
	}

	// Attempt 3 (exam): answer stored but ungraded while the attempt is
	// open — must NOT evict (and must not add).
	a3 := postJSON(t, router, studentToken, "POST", "/api/practice/attempts", `{"setId":"`+examID+`"}`)
	id3 := a3["attempt"].(map[string]any)["id"].(string)
	postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+id3+"/answers", `{"questionId":"`+mcqID+`","choiceKey":"A"}`)
	if got := entryCount(); got != 0 {
		t.Fatalf("ungraded exam answer should leave prior wrong-book state untouched, got %d entries", got)
	}

	// Finishing grades the exam answer as wrong -> re-admitted.
	postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+id3+"/finish", "")
	if got := entryCount(); got != 1 {
		t.Fatalf("graded wrong exam answer should re-enter the wrong book, got %d entries", got)
	}
}

// Excel "CSV UTF-8" exports start with a BOM; the header row must still be
// recognized so subjective rows are not silently parsed as MCQ.
func TestQuestionImportCSVBOM(t *testing.T) {
	_, router := newTestApp(t)
	token := tokenForFirstAdmin(t, router)

	bom := string(rune(0xFEFF))
	csvContent := bom + "type,stem,choice_a,choice_b,answer,explanation,reference_answer\n" +
		"mcq,\"Valid stem?\",\"Yes\",\"No\",A,\"Because.\"\n" +
		"subjective,\"Explain hindsight bias.\",,,,\"Self-assessed\",\"Peek after outcomes.\"\n"
	preview := uploadImport(t, router, token, "bom.csv", csvContent)
	if preview["valid"].(float64) != 2 {
		t.Fatalf("expected 2 valid rows with BOM header, got %v errors %v", preview["valid"], preview["errors"])
	}
	items := preview["items"].([]any)
	last := items[len(items)-1].(map[string]any)
	if last["type"] != "subjective" {
		t.Fatalf("BOM must not break type detection, got type=%v", last["type"])
	}
}

// Accuracy stats must count each student's latest answer per question, not
// every attempt ever taken.
func TestPracticeStatsLatestPerQuestion(t *testing.T) {
	_, router := newTestApp(t)
	adminToken := registerTestUser(t, router, "stats-admin@example.com")
	studentToken := registerTestUser(t, router, "stats-student@example.com")
	mcqID, _ := importExamQuestions(t, router, adminToken)

	set := postJSON(t, router, adminToken, "POST", "/api/admin/sets", `{"title":"Stats set","mode":"instant","status":"published","questionIds":["`+mcqID+`"]}`)
	setID := set["id"].(string)
	for _, choice := range []string{"A", "B", "A"} {
		attempt := postJSON(t, router, studentToken, "POST", "/api/practice/attempts", `{"setId":"`+setID+`"}`)
		id := attempt["attempt"].(map[string]any)["id"].(string)
		postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+id+"/answers", `{"questionId":"`+mcqID+`","choiceKey":"`+choice+`"}`)
		postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+id+"/finish", "")
	}

	stats := postJSON(t, router, studentToken, "GET", "/api/practice/stats", "")
	if stats["attempts"].(float64) != 3 {
		t.Fatalf("attempt count should stay cumulative: %v", stats["attempts"])
	}
	byUnit := stats["byUnit"].([]any)
	if len(byUnit) != 1 {
		t.Fatalf("expected one unit row, got %v", byUnit)
	}
	row := byUnit[0].(map[string]any)
	// Three attempts: wrong, right, wrong — latest answer is wrong, so the
	// latest-per-question view is answered=1 correct=0.
	if row["answered"].(float64) != 1 || row["correct"].(float64) != 0 {
		t.Fatalf("stats should use the latest answer per question: %v", row)
	}

	overview := postJSON(t, router, adminToken, "GET", "/api/admin/analytics/overview", "")
	students := overview["students"].([]any)
	for _, raw := range students {
		s := raw.(map[string]any)
		if s["email"] == "stats-student@example.com" {
			if s["answered"].(float64) != 1 || s["correct"].(float64) != 0 {
				t.Fatalf("class stats should use the latest answer per question: %v", s)
			}
			if s["attempts"].(float64) != 3 {
				t.Fatalf("per-student attempt count should stay cumulative: %v", s)
			}
		}
	}
}
