package backend

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func (a *App) auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		tokenString := strings.TrimPrefix(h, "Bearer ")
		if tokenString == "" {
			c.JSON(401, gin.H{"error": "missing token"})
			c.Abort()
			return
		}
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) { return a.JWTSecret, nil }, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		claims := token.Claims.(*Claims)
		// Re-check the user so deleted or demoted accounts lose access
		// immediately instead of riding the token until expiry.
		var user User
		if err := a.DB.First(&user, "id = ?", claims.UserID).Error; err != nil {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("tenantID", claims.TenantID)
		c.Set("role", user.Role)
		c.Next()
	}
}

func (a *App) requireAdmin() gin.HandlerFunc {
	return a.requireRole("admin")
}

// requireRole gates a route to the given roles and stores the user in the
// request context for handlers.
func (a *App) requireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(c *gin.Context) {
		var user User
		if err := a.DB.First(&user, "id = ?", c.GetString("userID")).Error; err != nil {
			c.JSON(403, gin.H{"error": "permission required"})
			c.Abort()
			return
		}
		if !allowed[user.Role] {
			c.JSON(403, gin.H{"error": "permission required"})
			c.Abort()
			return
		}
		c.Set("role", user.Role)
		c.Next()
	}
}
