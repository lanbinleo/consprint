package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

// Microsoft Entra ID sign-in via the OAuth2 authorization code flow.
// The Go backend is the confidential client: the browser is redirected to
// Microsoft, then back to /api/auth/entra/callback, where we exchange the
// code, read the profile from Microsoft Graph, and issue our own JWT.

type graphProfile struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	UserPrincipalName string `json:"userPrincipalName"`
	Mail              string `json:"mail"`
}

func entraConfigured() bool {
	tenantID := env("ENTRA_TENANT_ID", "")
	clientID := env("ENTRA_CLIENT_ID", "")
	clientSecret := env("ENTRA_CLIENT_SECRET", "")
	redirectURI := env("ENTRA_REDIRECT_URI", "")
	return tenantID != "" && clientID != "" && clientSecret != "" && redirectURI != ""
}

func entraOAuthConfig() *oauth2.Config {
	tenantID := env("ENTRA_TENANT_ID", "")
	base := env("ENTRA_AUTH_BASE_URL", "https://login.microsoftonline.com/"+tenantID)
	return &oauth2.Config{
		ClientID:     env("ENTRA_CLIENT_ID", ""),
		ClientSecret: env("ENTRA_CLIENT_SECRET", ""),
		RedirectURL:  env("ENTRA_REDIRECT_URI", ""),
		Endpoint: oauth2.Endpoint{
			AuthURL:  strings.TrimRight(base, "/") + "/oauth2/v2.0/authorize",
			TokenURL: strings.TrimRight(base, "/") + "/oauth2/v2.0/token",
		},
		Scopes: []string{"openid", "profile", "email", "User.Read"},
	}
}

func entraGraphURL() string {
	return env("ENTRA_GRAPH_URL", "https://graph.microsoft.com/v1.0/me")
}

func entraAllowedDomains() []string {
	var out []string
	for _, part := range strings.Split(env("ENTRA_ALLOWED_DOMAINS", ""), ",") {
		if v := strings.ToLower(strings.TrimSpace(part)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func entraEmailAllowed(email string) bool {
	domains := entraAllowedDomains()
	if len(domains) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(email[at+1:])
	for _, allowed := range domains {
		if domain == allowed {
			return true
		}
	}
	return false
}

var entraStates = struct {
	sync.Mutex
	m map[string]time.Time
}{m: map[string]time.Time{}}

func newEntraState() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	value := hex.EncodeToString(b[:])
	entraStates.Lock()
	now := time.Now()
	for key, expires := range entraStates.m {
		if now.After(expires) {
			delete(entraStates.m, key)
		}
	}
	entraStates.m[value] = now.Add(10 * time.Minute)
	entraStates.Unlock()
	return value
}

func consumeEntraState(value string) bool {
	entraStates.Lock()
	defer entraStates.Unlock()
	expires, ok := entraStates.m[value]
	if !ok || time.Now().After(expires) {
		return false
	}
	delete(entraStates.m, value)
	return true
}

func entraRedirect(c *gin.Context, params map[string]string) {
	target := env("ENTRA_FRONTEND_REDIRECT", "/auth/callback")
	fragment := url.Values{}
	for key, value := range params {
		fragment.Set(key, value)
	}
	c.Redirect(http.StatusFound, target+"#"+fragment.Encode())
}

func (a *App) authProviders(c *gin.Context) {
	c.JSON(200, gin.H{"entra": entraConfigured()})
}

// appMeta exposes public client configuration: available login providers,
// the configurable AP exam date, and the app timezone.
func (a *App) appMeta(c *gin.Context) {
	c.JSON(200, gin.H{
		"entra":    entraConfigured(),
		"examDate": env("AP_EXAM_DATE", ""),
		"timezone": env("APP_TIMEZONE", "Asia/Shanghai"),
	})
}

func (a *App) entraLogin(c *gin.Context) {
	if !entraConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "microsoft sign-in is not configured"})
		return
	}
	c.Redirect(http.StatusFound, entraOAuthConfig().AuthCodeURL(newEntraState()))
}

func (a *App) entraCallback(c *gin.Context) {
	if !entraConfigured() {
		entraRedirect(c, map[string]string{"error": "microsoft sign-in is not configured"})
		return
	}
	if errParam := c.Query("error"); errParam != "" {
		entraRedirect(c, map[string]string{"error": fallback(errParam, "sign_in_failed")})
		return
	}
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || !consumeEntraState(state) {
		entraRedirect(c, map[string]string{"error": "invalid_state"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	token, err := entraOAuthConfig().Exchange(ctx, code)
	if err != nil {
		entraRedirect(c, map[string]string{"error": "token_exchange_failed"})
		return
	}
	profile, err := fetchGraphProfile(ctx, token)
	if err != nil {
		entraRedirect(c, map[string]string{"error": "profile_fetch_failed"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(fallback(profile.Mail, profile.UserPrincipalName)))
	if profile.ID == "" || email == "" {
		entraRedirect(c, map[string]string{"error": "profile_incomplete"})
		return
	}
	if !entraEmailAllowed(email) {
		entraRedirect(c, map[string]string{"error": "email_domain_not_allowed"})
		return
	}
	user, err := a.upsertEntraUser(profile, email)
	if err != nil {
		entraRedirect(c, map[string]string{"error": "sign_in_failed"})
		return
	}
	jwtToken, err := a.sign(user)
	if err != nil {
		entraRedirect(c, map[string]string{"error": "token_failed"})
		return
	}
	entraRedirect(c, map[string]string{"token": jwtToken})
}

func fetchGraphProfile(ctx context.Context, token *oauth2.Token) (graphProfile, error) {
	var profile graphProfile
	if token.AccessToken == "" {
		return profile, errors.New("missing access token")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, entraGraphURL(), nil)
	if err != nil {
		return profile, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return profile, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return profile, errors.New("graph request failed")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return profile, err
	}
	if err := json.Unmarshal(body, &profile); err != nil {
		return profile, err
	}
	return profile, nil
}

func (a *App) upsertEntraUser(profile graphProfile, email string) (User, error) {
	oid := profile.ID
	var user User
	if err := a.DB.First(&user, "entra_oid = ?", oid).Error; err == nil {
		if strings.TrimSpace(user.Name) == "" {
			user.Name = fallback(profile.DisplayName, email)
			a.DB.Save(&user)
		}
		return user, nil
	}
	if err := a.DB.First(&user, "email = ?", email).Error; err == nil {
		// Link the school Microsoft identity to an existing local account.
		user.EntraOID = &oid
		a.DB.Save(&user)
		return user, nil
	}
	role := "student"
	if isAdminEmail(email) {
		role = "admin"
	} else {
		var admins int64
		a.DB.Model(&User{}).Where("role = ?", "admin").Count(&admins)
		if admins == 0 {
			role = "admin"
		}
	}
	// Entra-only accounts get an unguessable password hash so local password
	// login can never succeed for them.
	var randomBytes [24]byte
	_, _ = rand.Read(randomBytes[:])
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(randomBytes[:])), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	user = User{
		ID:           NewID("usr"),
		TenantID:     schoolTenantID,
		Name:         fallback(profile.DisplayName, email),
		Email:        email,
		Role:         role,
		Provider:     "entra",
		EntraOID:     &oid,
		PasswordHash: string(hash),
	}
	if err := a.DB.Create(&user).Error; err != nil {
		return User{}, err
	}
	return user, nil
}
