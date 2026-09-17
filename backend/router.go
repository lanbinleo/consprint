package backend

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func (a *App) Router() *gin.Engine {
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", env("CORS_ORIGIN", "http://localhost:5173"))
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	api.GET("/auth/providers", a.authProviders)
	api.POST("/auth/register", a.register)
	api.POST("/auth/login", a.login)
	api.GET("/auth/entra/login", a.entraLogin)
	api.GET("/auth/entra/callback", a.entraCallback)
	protected := api.Group("")
	protected.Use(a.auth())
	protected.GET("/me", a.me)
	protected.PATCH("/me", a.updateMe)
	protected.GET("/dashboard", a.dashboard)
	protected.GET("/dashboard/summary", a.dashboardSummary)
	protected.GET("/dashboard/progress", a.dashboardProgress)
	protected.GET("/dashboard/trends", a.dashboardTrends)
	protected.GET("/dashboard/alerts", a.dashboardAlerts)
	protected.GET("/units", a.units)
	protected.GET("/concepts", a.concepts)
	protected.GET("/concepts/:id", a.concept)
	protected.PATCH("/concepts/:id/status", a.setConceptStatus)
	protected.GET("/review/next", a.reviewNext)
	protected.POST("/review/events", a.reviewEvent)
	admin := protected.Group("")
	admin.Use(a.requireAdmin())
	admin.PATCH("/concepts/:id/content", a.updateConceptContent)
	admin.GET("/import/status", a.importStatus)
	admin.POST("/import/run", a.importRun)
	if _, err := os.Stat("frontend/dist/index.html"); err == nil {
		r.Static("/assets", "frontend/dist/assets")
		r.StaticFile("/favicon.svg", "frontend/dist/favicon.svg")
		r.NoRoute(func(c *gin.Context) {
			c.File("frontend/dist/index.html")
		})
	}
	return r
}
