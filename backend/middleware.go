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
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) { return a.JWTSecret, nil })
		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		claims := token.Claims.(*Claims)
		c.Set("userID", claims.UserID)
		c.Set("tenantID", claims.TenantID)
		c.Next()
	}
}

func (a *App) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var user User
		if err := a.DB.First(&user, "id = ?", c.GetString("userID")).Error; err != nil || user.Role != "admin" {
			c.JSON(403, gin.H{"error": "admin permission required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
