package backend

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// tinyPNG is a valid 1x1 transparent PNG used for upload sniffing tests.
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func newNotesTestApp(t *testing.T) (*App, http.Handler, string, string) {
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
	// Register the staff account first so the local-dev "first user becomes
	// admin" fallback never lands on the student, then pin the role explicitly.
	teacher := registerTestUser(t, router, "notes-teacher@example.com")
	student := registerTestUser(t, router, "notes-student@example.com")
	app.DB.Model(&User{}).Where("email = ?", "notes-teacher@example.com").Update("role", "teacher")
	return app, router, student, teacher
}

func notesRequest(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewBuffer(raw)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func fetchNoteResources(t *testing.T, router http.Handler, token string) []NoteResource {
	t.Helper()
	w := notesRequest(t, router, http.MethodGet, "/api/note-resources", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list note resources failed: %d %s", w.Code, w.Body.String())
	}
	var resources []NoteResource
	if err := json.Unmarshal(w.Body.Bytes(), &resources); err != nil {
		t.Fatal(err)
	}
	return resources
}

func findNoteResource(resources []NoteResource, title string) *NoteResource {
	for i := range resources {
		if resources[i].Title == title {
			return &resources[i]
		}
	}
	return nil
}

func TestNoteResourcesSeed(t *testing.T) {
	app, router, student, _ := newNotesTestApp(t)

	resources := fetchNoteResources(t, router, student)
	if len(resources) != 2 {
		t.Fatalf("expected 2 seeded tabs, got %d", len(resources))
	}
	leo := findNoteResource(resources, "Leo 的复习笔记")
	if leo == nil {
		t.Fatalf("seeded Leo tab missing: %#v", resources)
	}
	if len(leo.Items) != len(seedMubuNotes) {
		t.Fatalf("expected %d unit items, got %d", len(seedMubuNotes), len(leo.Items))
	}
	for i, item := range leo.Items {
		if item.Kind != "embed" || item.URL != seedMubuNotes[i].URL || item.Label != seedMubuNotes[i].Label {
			t.Fatalf("unit item %d mismatch: %#v", i, item)
		}
	}
	pdf := findNoteResource(resources, "示例：PDF 讲义")
	if pdf == nil || len(pdf.Items) != 1 || pdf.Items[0].Kind != "pdf" || pdf.Items[0].URL != "/files/ap-psych-sample.pdf" {
		t.Fatalf("seeded pdf tab mismatch: %#v", pdf)
	}

	// The seeder must be a no-op once rows exist (repeatable seeds).
	if err := seedNoteResources(app.DB); err != nil {
		t.Fatal(err)
	}
	var count int64
	app.DB.Model(&NoteResource{}).Count(&count)
	if count != 2 {
		t.Fatalf("seed is not idempotent: %d resources after reseed", count)
	}
}

func TestNoteResourcesStaffCRUD(t *testing.T) {
	_, router, student, teacher := newNotesTestApp(t)

	// Students can read but not manage.
	if w := notesRequest(t, router, http.MethodPost, "/api/admin/note-resources", student, map[string]any{"title": "Nope"}); w.Code != http.StatusForbidden {
		t.Fatalf("student create should be forbidden: %d %s", w.Code, w.Body.String())
	}

	create := map[string]any{
		"title":  "PPT 讲义",
		"status": "published",
		"items": []map[string]string{
			{"label": "Unit 2 slides", "url": "https://example.com/u2.pdf", "kind": "pdf"},
			{"label": "内链", "url": "/files/notes-1-x.pdf", "kind": "pdf"},
		},
	}
	w := notesRequest(t, router, http.MethodPost, "/api/admin/note-resources", teacher, create)
	if w.Code != http.StatusOK {
		t.Fatalf("teacher create failed: %d %s", w.Code, w.Body.String())
	}
	var created NoteResource
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// Student list sees the published tab with normalized (non-null) items.
	resources := fetchNoteResources(t, router, student)
	got := findNoteResource(resources, "PPT 讲义")
	if got == nil || got.Items == nil || len(got.Items) != 2 || got.Items[1].URL != "/files/notes-1-x.pdf" {
		t.Fatalf("published tab not visible to student: %#v", got)
	}

	// Drafts stay hidden from students.
	draft := map[string]any{"title": "草稿", "status": "draft"}
	if w := notesRequest(t, router, http.MethodPost, "/api/admin/note-resources", teacher, draft); w.Code != http.StatusOK {
		t.Fatalf("draft create failed: %d %s", w.Code, w.Body.String())
	}
	if findNoteResource(fetchNoteResources(t, router, student), "草稿") != nil {
		t.Fatal("draft tab leaked to student list")
	}

	// Admin list sees everything.
	w = notesRequest(t, router, http.MethodGet, "/api/admin/note-resources", teacher, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin list failed: %d %s", w.Code, w.Body.String())
	}
	var adminList []NoteResource
	if err := json.Unmarshal(w.Body.Bytes(), &adminList); err != nil {
		t.Fatal(err)
	}
	if findNoteResource(adminList, "草稿") == nil {
		t.Fatal("draft tab missing from staff list")
	}

	// Patch replaces items and updates fields.
	patch := map[string]any{
		"description": "updated",
		"items":       []map[string]string{{"label": "Only one", "url": "https://example.com/one", "kind": "link"}},
	}
	if w := notesRequest(t, router, http.MethodPatch, "/api/admin/note-resources/"+created.ID, teacher, patch); w.Code != http.StatusOK {
		t.Fatalf("patch failed: %d %s", w.Code, w.Body.String())
	}
	resources = fetchNoteResources(t, router, student)
	got = findNoteResource(resources, "PPT 讲义")
	if got == nil || got.Description != "updated" || len(got.Items) != 1 || got.Items[0].Kind != "link" {
		t.Fatalf("patch not applied: %#v", got)
	}

	// Validation: bad kind and dangerous URLs are rejected with 400, not stored.
	for _, bad := range []map[string]any{
		{"title": "Bad", "items": []map[string]string{{"label": "x", "url": "https://ok.com", "kind": "iframe"}}},
		{"title": "Bad", "items": []map[string]string{{"label": "x", "url": "javascript:alert(1)", "kind": "embed"}}},
		{"title": "Bad", "items": []map[string]string{{"label": "x", "url": "//evil.com", "kind": "embed"}}},
	} {
		if w := notesRequest(t, router, http.MethodPost, "/api/admin/note-resources", teacher, bad); w.Code != http.StatusBadRequest {
			t.Fatalf("invalid payload should 400: %d %s", w.Code, w.Body.String())
		}
	}

	// Delete removes the tab and its items (hard delete).
	if w := notesRequest(t, router, http.MethodDelete, "/api/admin/note-resources/"+created.ID, student, nil); w.Code != http.StatusForbidden {
		t.Fatalf("student delete should be forbidden: %d %s", w.Code, w.Body.String())
	}
	if w := notesRequest(t, router, http.MethodDelete, "/api/admin/note-resources/"+created.ID, teacher, nil); w.Code != http.StatusOK {
		t.Fatalf("teacher delete failed: %d %s", w.Code, w.Body.String())
	}
	if findNoteResource(fetchNoteResources(t, router, student), "PPT 讲义") != nil {
		t.Fatal("deleted tab still visible")
	}
}

func TestNoteResourceUpload(t *testing.T) {
	_, router, student, teacher := newNotesTestApp(t)

	upload := func(token, filename string, content []byte, contentType string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
		mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/note-resources/upload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	if w := upload(student, "tiny.png", tinyPNG, ""); w.Code != http.StatusForbidden {
		t.Fatalf("student upload should be forbidden: %d %s", w.Code, w.Body.String())
	}
	if w := upload(teacher, "notes.txt", []byte("hello"), ""); w.Code != http.StatusBadRequest {
		t.Fatalf("non-media upload should 400: %d %s", w.Code, w.Body.String())
	}

	w := upload(teacher, "神经递质 图解.png", tinyPNG, "")
	if w.Code != http.StatusOK {
		t.Fatalf("png upload failed: %d %s", w.Code, w.Body.String())
	}
	var stored struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stored); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix([]byte(stored.URL), []byte("/files/notes-")) || stored.Filename == "" {
		t.Fatalf("unexpected upload response: %#v", stored)
	}

	// The stored file is served back publicly at /files/<name>.
	req := httptest.NewRequest(http.MethodGet, stored.URL, nil)
	served := httptest.NewRecorder()
	router.ServeHTTP(served, req)
	if served.Code != http.StatusOK {
		t.Fatalf("uploaded file not served: %d %s", served.Code, served.Body.String())
	}
	if got := served.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("served content type %q, want image/png", got)
	}

	// Unknown /files paths 404 from the file server, never the SPA shell.
	req = httptest.NewRequest(http.MethodGet, "/files/does-not-exist.pdf", nil)
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, req)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing file should 404: %d", missing.Code)
	}
}

func TestNoteResourceUploadPDF(t *testing.T) {
	_, router, _, teacher := newNotesTestApp(t)
	// "%PDF-1.4" magic prefix is what http.DetectContentType sniffs on.
	pdf := append([]byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"), bytes.Repeat([]byte("x"), 64)...)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "handout.pdf")
	fw.Write(pdf)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/note-resources/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+teacher)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pdf upload failed: %d %s", w.Code, w.Body.String())
	}
	var stored struct {
		URL string `json:"url"`
	}
	json.Unmarshal(w.Body.Bytes(), &stored)
	if !bytes.HasSuffix([]byte(stored.URL), []byte(".pdf")) {
		t.Fatalf("pdf upload returned %q", stored.URL)
	}
}
