package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func newTestApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	app, err := NewApp(filepath.Join(t.TempDir(), "app.db"), filepath.Join("..", "data", "sources"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return app, app.Router()
}

func TestAuthProvidersUnconfigured(t *testing.T) {
	_, router := newTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("providers failed: %d %s", w.Code, w.Body.String())
	}
	var providers struct {
		Entra bool `json:"entra"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &providers); err != nil {
		t.Fatal(err)
	}
	if providers.Entra {
		t.Fatal("entra should be disabled when env is not configured")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/entra/login", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("entra login should be 503 when unconfigured, got %d", w.Code)
	}
}

func mockIdentityServer(t *testing.T, profile string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"graph-token","token_type":"Bearer","expires_in":3600}`))
		case strings.HasSuffix(r.URL.Path, "/me"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(profile))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func configureEntra(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("ENTRA_TENANT_ID", "test-tenant")
	t.Setenv("ENTRA_CLIENT_ID", "test-client")
	t.Setenv("ENTRA_CLIENT_SECRET", "test-secret")
	t.Setenv("ENTRA_REDIRECT_URI", "https://app.example/api/auth/entra/callback")
	t.Setenv("ENTRA_AUTH_BASE_URL", baseURL)
	t.Setenv("ENTRA_GRAPH_URL", baseURL+"/me")
	t.Setenv("ENTRA_FRONTEND_REDIRECT", "/auth/callback")
}

func runEntraCallback(t *testing.T, router http.Handler) (location string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/entra/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("entra login should redirect, got %d %s", w.Code, w.Body.String())
	}
	loginURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := loginURL.Query().Get("state")
	if state == "" {
		t.Fatal("entra login redirect missing state")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/entra/callback?code=test-code&state="+url.QueryEscape(state), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("entra callback should redirect, got %d %s", w.Code, w.Body.String())
	}
	return w.Header().Get("Location")
}

func TestEntraCallbackCreatesUser(t *testing.T) {
	profile := `{"id":"oid-123","displayName":"Test User","userPrincipalName":"test@tsinglan.org"}`
	configureEntra(t, mockIdentityServer(t, profile).URL)
	_, router := newTestApp(t)

	location := runEntraCallback(t, router)
	fragment, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(fragment.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	token := values.Get("token")
	if token == "" {
		t.Fatalf("callback missing token, location %s", location)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("me failed: %d %s", w.Code, w.Body.String())
	}
	var me struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.User.Email != "test@tsinglan.org" || me.User.Provider != "entra" {
		t.Fatalf("unexpected entra user: %#v", me.User)
	}
}

func TestEntraAllowedDomainsRejectsOutsider(t *testing.T) {
	profile := `{"id":"oid-456","displayName":"Outsider","userPrincipalName":"someone@example.com"}`
	configureEntra(t, mockIdentityServer(t, profile).URL)
	t.Setenv("ENTRA_ALLOWED_DOMAINS", "tsinglan.org")
	_, router := newTestApp(t)

	location := runEntraCallback(t, router)
	if !strings.Contains(location, "error=email_domain_not_allowed") {
		t.Fatalf("expected domain rejection, got %s", location)
	}
}

func TestEntraLinksExistingLocalAccount(t *testing.T) {
	profile := `{"id":"oid-789","displayName":"Local Person","userPrincipalName":"local@tsinglan.org"}`
	configureEntra(t, mockIdentityServer(t, profile).URL)
	app, router := newTestApp(t)
	localToken := registerTestUser(t, router, "local@tsinglan.org")

	location := runEntraCallback(t, router)
	fragment, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(fragment.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	entraToken := values.Get("token")
	if entraToken == "" {
		t.Fatalf("callback missing token, location %s", location)
	}

	// Both tokens resolve to the same account; password login still works.
	for _, token := range []string{localToken, entraToken} {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("me failed for linked account: %d %s", w.Code, w.Body.String())
		}
	}

	var linked User
	if err := app.DB.First(&linked, "email = ?", "local@tsinglan.org").Error; err != nil {
		t.Fatal(err)
	}
	if linked.EntraOID == nil || *linked.EntraOID != "oid-789" {
		t.Fatalf("local account was not linked to entra oid: %#v", linked)
	}

	body := strings.NewReader(`{"email":"local@tsinglan.org","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("local password should keep working after linking: %d %s", w.Code, w.Body.String())
	}
}
