package backend

import (
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// dummyBcryptHash is a fixed valid-format hash used only to equalize login
// response time for unknown emails.
const dummyBcryptHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (a *App) register(c *gin.Context) {
	var req struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		InviteCode string `json:"inviteCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		c.JSON(400, gin.H{"error": "email and password are required"})
		return
	}
	if requiredCode := strings.TrimSpace(os.Getenv("REGISTRATION_INVITE_CODE")); requiredCode != "" && strings.TrimSpace(req.InviteCode) != requiredCode {
		c.JSON(403, gin.H{"error": "invalid registration code"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "could not hash password"})
		return
	}
	var tenant Tenant
	if err := a.DB.First(&tenant, "id = ?", schoolTenantID).Error; err != nil {
		c.JSON(500, gin.H{"error": "school workspace missing"})
		return
	}
	role := "student"
	// In production, registration only grants admin through the invite-code
	// path (or Entra SSO): with open registration and no invite code, anyone
	// registering a listed ADMIN_EMAILS address first would pre-hijack the
	// admin account.
	inviteConfigured := strings.TrimSpace(os.Getenv("REGISTRATION_INVITE_CODE")) != ""
	if isAdminEmail(req.Email) {
		if !productionMode() || inviteConfigured {
			role = "admin"
		}
	} else if !productionMode() {
		// Fallback for local development: without ADMIN_EMAILS the very
		// first account keeps the deployment usable by becoming admin.
		var admins int64
		a.DB.Model(&User{}).Where("role = ?", "admin").Count(&admins)
		if admins == 0 {
			role = "admin"
		}
	}
	user := User{ID: NewID("usr"), TenantID: tenant.ID, Name: fallback(req.Name, "Student"), Email: strings.ToLower(req.Email), Role: role, Provider: "local", PasswordHash: string(hash)}
	if err := a.DB.Create(&user).Error; err != nil {
		c.JSON(409, gin.H{"error": "email already exists"})
		return
	}
	token, _ := a.sign(user)
	c.JSON(200, gin.H{"token": token, "user": user, "tenant": tenant})
}

func (a *App) login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	var user User
	if err := a.DB.Where("email = ?", strings.ToLower(req.Email)).First(&user).Error; err != nil {
		// Compare against a dummy hash anyway so response time does not
		// reveal whether the account exists.
		_ = bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(req.Password))
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}
	var tenant Tenant
	a.DB.First(&tenant, "id = ?", user.TenantID)
	token, _ := a.sign(user)
	c.JSON(200, gin.H{"token": token, "user": user, "tenant": tenant})
}

func (a *App) sign(user User) (string, error) {
	claims := Claims{
		UserID: user.ID, TenantID: user.TenantID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour))},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.JWTSecret)
}

func (a *App) me(c *gin.Context) {
	var user User
	var tenant Tenant
	a.DB.First(&user, "id = ?", c.GetString("userID"))
	a.DB.First(&tenant, "id = ?", c.GetString("tenantID"))
	c.JSON(200, gin.H{"user": user, "tenant": tenant})
}

func (a *App) updateMe(c *gin.Context) {
	var req struct {
		Name          string `json:"name"`
		AvatarDataURL string `json:"avatarDataUrl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	var user User
	if err := a.DB.First(&user, "id = ?", c.GetString("userID")).Error; err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		user.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.AvatarDataURL) != "" {
		if !strings.HasPrefix(req.AvatarDataURL, "data:image/") || len(req.AvatarDataURL) > 260000 {
			c.JSON(400, gin.H{"error": "avatar must be a small image data URL"})
			return
		}
		user.AvatarDataURL = req.AvatarDataURL
	}
	a.DB.Save(&user)
	var tenant Tenant
	a.DB.First(&tenant, "id = ?", user.TenantID)
	c.JSON(200, gin.H{"user": user, "tenant": tenant})
}
