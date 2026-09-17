package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func promoteUser(t *testing.T, router http.Handler, adminToken, email, role string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users?search="+email, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("user list failed: %d %s", w.Code, w.Body.String())
	}
	var users []User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil || len(users) != 1 {
		t.Fatalf("expected one user, got %d: %v", len(users), err)
	}
	body := bytes.NewBufferString(fmt.Sprintf(`{"role":%q}`, role))
	req = httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+users[0].ID, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("promote failed: %d %s", w.Code, w.Body.String())
	}
}

func uploadImport(t *testing.T, router http.Handler, token, filename, content string) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/questions/import/preview", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import preview failed: %d %s", w.Code, w.Body.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	return preview
}

func importJSONQuestions(t *testing.T, router http.Handler, token string) []string {
	t.Helper()
	payload := `[
		{"type":"mcq","stem":"Which method best shows cause and effect?","choices":[{"key":"A","text":"Correlational study"},{"key":"B","text":"Experiment"},{"key":"C","text":"Case study"}],"answerKey":"B","explanation":"Experiments manipulate the IV.","unit":"1","tags":["research-methods"]},
		{"type":"mcq","stem":"Hindsight bias is...","choices":[{"key":"A","text":"Tendency to overestimate"},{"key":"B","text":"Tendency to predict"}],"answerKey":"A","explanation":"I-knew-it-all-along.","unit":"1"},
		{"type":"subjective","stem":"Explain hindsight bias with an example.","parts":[{"label":"A","prompt":"Explain","referenceAnswer":"After outcomes are known people overestimate predictability.","rubric":["Defines hindsight bias","Gives an example"]}],"unit":"1","tags":["frq"]}
	]`
	preview := uploadImport(t, router, token, "questions.json", payload)
	if preview["valid"].(float64) != 3 {
		t.Fatalf("expected 3 valid questions, got %v (errors: %v)", preview["valid"], preview["errors"])
	}
	body := bytes.NewBufferString(fmt.Sprintf(`{"importId":%q}`, preview["importId"].(string)))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/questions/import/commit", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import commit failed: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Created int `json:"created"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Created != 3 {
		t.Fatalf("expected 3 created, got %d", result.Created)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/questions?limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var questions []Question
	if err := json.Unmarshal(w.Body.Bytes(), &questions); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(questions))
	for _, q := range questions {
		ids = append(ids, q.ID)
	}
	return ids
}

func TestQuestionImportCSVValidation(t *testing.T) {
	_, router := newTestApp(t)

	csvContent := "type,stem,choice_a,choice_b,answer,explanation,tags\n" +
		"mcq,\"Valid stem?\",\"Yes\",\"No\",A,\"Because.\",unit-1;warmup\n" +
		"mcq,\"Missing answer\",\"Yes\",\"No\",,\"Because.\",\n" +
		"mcq,\"Bad answer key\",\"Yes\",\"No\",Z,\"Nope.\",\n" +
		",\" \",\"Yes\",\"No\",A,,\n"
	preview := uploadImport(t, router, tokenForFirstAdmin(t, router), "q.csv", csvContent)
	if preview["valid"].(float64) != 1 {
		t.Fatalf("expected 1 valid row, got %v errors %v", preview["valid"], preview["errors"])
	}
	if len(preview["errors"].([]any)) != 3 {
		t.Fatalf("expected 3 errors, got %v", preview["errors"])
	}
}

func tokenForFirstAdmin(t *testing.T, router http.Handler) string {
	t.Helper()
	return registerTestUser(t, router, "importer-admin@example.com")
}

func TestPracticeFlowInstantAndExam(t *testing.T) {
	_, router := newTestApp(t)
	adminToken := registerTestUser(t, router, "admin@example.com")
	teacherToken := registerTestUser(t, router, "teacher@example.com")
	promoteUser(t, router, adminToken, "teacher@example.com", "teacher")
	studentToken := registerTestUser(t, router, "student@example.com")

	// Teacher cannot manage users; student cannot touch the question bank.
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+teacherToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("teacher should not manage users: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/questions", nil)
	req.Header.Set("Authorization", "Bearer "+studentToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("student should not list questions: %d", w.Code)
	}

	questionIDs := importJSONQuestions(t, router, teacherToken)
	if len(questionIDs) != 3 {
		t.Fatalf("expected 3 questions, got %d", len(questionIDs))
	}

	// Assemble an instant set and an exam set from the same bank.
	instantSet := postJSON(t, router, teacherToken, "POST", "/api/admin/sets", `{"title":"Unit 1 warmup","mode":"instant","status":"published","questionIds":["`+strings.Join(questionIDs, `","`)+`"]}`)
	instantSetID := instantSet["id"].(string)
	examSet := postJSON(t, router, teacherToken, "POST", "/api/admin/sets", `{"title":"Unit 1 mock exam","mode":"exam","timeLimitSec":600,"status":"published","questionIds":["`+strings.Join(questionIDs, `","`)+`"]}`)
	examSetID := examSet["id"].(string)

	// Student sees both published sets.
	w = doRequest(t, router, studentToken, "GET", "/api/practice/sets", "")
	var sets []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &sets); err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 {
		t.Fatalf("expected 2 published sets, got %d", len(sets))
	}

	// --- Instant mode: per-question feedback, subjective reveal + self-rating.
	attempt := postJSON(t, router, studentToken, "POST", "/api/practice/attempts", `{"setId":"`+instantSetID+`"}`)
	instantAttemptID := attempt["attempt"].(map[string]any)["id"].(string)
	var mcqCorrect, mcqWrong, subjectiveID string
	detail := postJSON(t, router, studentToken, "GET", "/api/practice/attempts/"+instantAttemptID, "")
	for _, raw := range detail["questions"].([]any) {
		question := raw.(map[string]any)
		if question["type"] == "subjective" {
			subjectiveID = question["id"].(string)
			continue
		}
		if _, hasAnswer := question["answerKey"]; hasAnswer {
			t.Fatalf("student view leaked the answer key before answering: %v", question)
		}
		if strings.Contains(question["stem"].(string), "cause and effect") {
			mcqCorrect = question["id"].(string)
		} else {
			mcqWrong = question["id"].(string)
		}
	}
	answer := postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+instantAttemptID+"/answers", `{"questionId":"`+mcqCorrect+`","choiceKey":"B"}`)
	if answer["question"].(map[string]any)["answerKey"] != "B" {
		t.Fatalf("instant mode should reveal the answer: %v", answer)
	}
	answer = postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+instantAttemptID+"/answers", `{"questionId":"`+mcqWrong+`","choiceKey":"B"}`)
	if answer["answer"].(map[string]any)["isCorrect"] != false {
		t.Fatalf("wrong answer should be marked incorrect: %v", answer)
	}
	postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+instantAttemptID+"/answers", `{"questionId":"`+subjectiveID+`","textAnswer":"After results are known, people say they saw it coming."}`)
	rated := postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+instantAttemptID+"/answers/"+subjectiveID+"/self-rating", `{"rating":"partial"}`)
	if rated["selfRating"] != "partial" {
		t.Fatalf("self rating not stored: %v", rated)
	}
	summary := postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+instantAttemptID+"/finish", "")
	if summary["correct"].(float64) != 1 {
		t.Fatalf("expected 1 correct mcq, got %v", summary["correct"])
	}

	// --- Exam mode: no feedback until finish.
	examAttempt := postJSON(t, router, studentToken, "POST", "/api/practice/attempts", `{"setId":"`+examSetID+`"}`)
	examAttemptID := examAttempt["attempt"].(map[string]any)["id"].(string)
	stored := postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+examAttemptID+"/answers", `{"questionId":"`+mcqCorrect+`","choiceKey":"B"}`)
	if _, hasQuestion := stored["question"]; hasQuestion {
		t.Fatal("exam mode must not reveal answers per question")
	}
	examSummary := postJSON(t, router, studentToken, "POST", "/api/practice/attempts/"+examAttemptID+"/finish", "")
	if examSummary["correct"].(float64) != 1 {
		t.Fatalf("exam grading wrong: %v", examSummary["correct"])
	}
	attemptRow := examSummary["attempt"].(map[string]any)
	if attemptRow["totalMcq"].(float64) != 2 {
		t.Fatalf("expected 2 mcq in exam set, got %v", attemptRow["totalMcq"])
	}

	// --- Wrong book keeps the incorrectly answered MCQ question.
	w = doRequest(t, router, studentToken, "GET", "/api/practice/wrongbook", "")
	var wrongBook []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &wrongBook); err != nil {
		t.Fatal(err)
	}
	if len(wrongBook) != 1 {
		t.Fatalf("expected 1 wrong-book entry, got %d", len(wrongBook))
	}
	if wrongBook[0]["reason"] != "wrong" {
		t.Fatalf("unexpected wrong-book entry: %v", wrongBook[0])
	}

	// --- Personal stats.
	stats := postJSON(t, router, studentToken, "GET", "/api/practice/stats", "")
	if stats["attempts"].(float64) != 2 {
		t.Fatalf("expected 2 attempts, got %v", stats["attempts"])
	}
}

func doRequest(t *testing.T, router http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
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
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s: expected 200 got %d: %s", method, path, w.Code, w.Body.String())
	}
	return w
}

func postJSON(t *testing.T, router http.Handler, token, method, path, body string) map[string]any {
	t.Helper()
	w := doRequest(t, router, token, method, path, body)
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("%s %s: invalid json object: %v", method, path, err)
	}
	return payload
}
