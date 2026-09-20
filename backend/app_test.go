package backend

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
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

// With Entra live in production, email sign-up closes entirely — registration
// goes through Microsoft sign-in. Local dev (no Entra or non-production env)
// keeps email registration so the app stays usable there.
func TestEmailRegistrationClosedWithEntra(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "test-secret-that-is-long-enough-for-production")
	t.Setenv("ENTRA_TENANT_ID", "tenant")
	t.Setenv("ENTRA_CLIENT_ID", "client")
	t.Setenv("ENTRA_CLIENT_SECRET", "secret")
	t.Setenv("ENTRA_REDIRECT_URI", "https://example.com/api/auth/entra/callback")
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

	req := httptest.NewRequest(http.MethodGet, "/api/meta", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"emailRegistration":false`) {
		t.Fatalf("meta should advertise closed email registration: %d %s", w.Code, w.Body.String())
	}

	body := bytes.NewBufferString(`{"tenantName":"Test","name":"Student","email":"closed@example.com","password":"secret"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("email registration should be closed with Entra in production: %d %s", w.Code, w.Body.String())
	}

	// Email+password sign-in for existing accounts keeps working.
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	existing := User{ID: "usr_existing", TenantID: schoolTenantID, Name: "Existing", Email: "existing@example.com", Role: "student", Provider: "local", PasswordHash: string(hash)}
	if err := app.DB.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"existing@example.com","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("local login should still work: %d %s", w.Code, w.Body.String())
	}
}

// Set/change password flows: Entra accounts set a first password without a
// current one (and gain email+password sign-in); afterwards, and for local
// accounts from the start, changing requires the current password.
func TestSetPassword(t *testing.T) {
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

	// Local user (registered with password "secret").
	token := registerTestUser(t, router, "local-pw@example.com")
	setPassword := func(body, token string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/me/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	if code := setPassword(`{"newPassword":"new-secret-1"}`, token); code != http.StatusBadRequest {
		t.Fatalf("local user must confirm current password: %d", code)
	}
	if code := setPassword(`{"currentPassword":"wrong","newPassword":"new-secret-1"}`, token); code != http.StatusBadRequest {
		t.Fatalf("wrong current password should fail: %d", code)
	}
	if code := setPassword(`{"currentPassword":"secret","newPassword":"new-secret-1"}`, token); code != http.StatusOK {
		t.Fatalf("correct current password should allow change: %d", code)
	}
	if code := setPassword(`{"currentPassword":"secret","newPassword":"short"}`, token); code != http.StatusBadRequest {
		t.Fatalf("too-short new password should fail: %d", code)
	}
	// Old password stops working, the new one signs in.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"local-pw@example.com","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old password should be rejected: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"local-pw@example.com","password":"new-secret-1"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login with the changed password failed: %d %s", w.Code, w.Body.String())
	}

	// Entra user: random bootstrap hash, no usable password yet.
	oid := "entra-oid-1"
	entraUser := User{ID: "usr_entra_pw", TenantID: schoolTenantID, Name: "MS User", Email: "entra-pw@example.com", Role: "student", Provider: "entra", EntraOID: &oid, PasswordHash: dummyBcryptHash}
	if err := app.DB.Create(&entraUser).Error; err != nil {
		t.Fatal(err)
	}
	entraToken, err := app.sign(entraUser)
	if err != nil {
		t.Fatal(err)
	}

	// passwordSet is false before, true after.
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+entraToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"passwordSet":false`) {
		t.Fatalf("entra user should report passwordSet=false: %d %s", w.Code, w.Body.String())
	}

	if code := setPassword(`{"newPassword":"chosen-pass-1"}`, entraToken); code != http.StatusOK {
		t.Fatalf("entra user should set a first password without a current one: %d", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"entra-pw@example.com","password":"chosen-pass-1"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("entra user should now sign in with email+password: %d %s", w.Code, w.Body.String())
	}

	// Once set, changing needs the current password again.
	if code := setPassword(`{"newPassword":"another-pass"}`, entraToken); code != http.StatusBadRequest {
		t.Fatalf("entra user with a set password must confirm current password: %d", code)
	}
	if code := setPassword(`{"currentPassword":"chosen-pass-1","newPassword":"another-pass"}`, entraToken); code != http.StatusOK {
		t.Fatalf("entra user password change with current password failed: %d", code)
	}
}

// Admin user maintenance: avatar reset clears the stored image, password
// reset yields a working sign-in, and the own-role guard keeps holding.
func TestAdminUserMaintenance(t *testing.T) {
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
	token := registerTestUser(t, router, "admin-maint@example.com")

	target := User{ID: "usr_target", TenantID: schoolTenantID, Name: "Target", Email: "target@example.com", Role: "student", Provider: "local", PasswordHash: dummyBcryptHash, AvatarDataURL: "data:image/png;base64,AAAA"}
	if err := app.DB.Create(&target).Error; err != nil {
		t.Fatal(err)
	}

	patch := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	if w := patch("/api/admin/users/usr_target", `{"avatarReset":true}`); w.Code != http.StatusOK {
		t.Fatalf("avatar reset failed: %d %s", w.Code, w.Body.String())
	}
	var after User
	if err := app.DB.First(&after, "id = ?", "usr_target").Error; err != nil {
		t.Fatal(err)
	}
	if after.AvatarDataURL != "" {
		t.Fatalf("avatar should be cleared, got %q", after.AvatarDataURL)
	}

	if w := patch("/api/admin/users/usr_target/password", `{"newPassword":"admin-reset-1"}`); w.Code != http.StatusOK {
		t.Fatalf("password reset failed: %d %s", w.Code, w.Body.String())
	}
	if w := patch("/api/admin/users/usr_target/password", `{"newPassword":"short"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("short password should be rejected: %d", w.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"target@example.com","password":"admin-reset-1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login with admin-reset password failed: %d %s", w.Code, w.Body.String())
	}

	// The admin cannot demote themselves out of the panel.
	var me User
	if err := app.DB.First(&me, "email = ?", "admin-maint@example.com").Error; err != nil {
		t.Fatal(err)
	}
	if w := patch("/api/admin/users/"+me.ID, `{"role":"student"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("self demotion should be rejected: %d %s", w.Code, w.Body.String())
	}
}

func TestGzipCompression(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health failed: %d", w.Code)
	}
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip Content-Encoding, got %q", got)
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("unexpected decompressed body: %s", body)
	}

	// Clients that do not advertise gzip must get plain JSON.
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("plain request should not be compressed, got %q", got)
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
