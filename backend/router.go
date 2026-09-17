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
	api.GET("/meta", a.appMeta)
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
	protected.GET("/practice/sets", a.practiceSets)
	protected.GET("/practice/sets/:id", a.practiceSetDetail)
	protected.POST("/practice/attempts", a.startAttempt)
	protected.GET("/practice/attempts/:id", a.attemptDetail)
	protected.POST("/practice/attempts/:id/answers", a.submitAnswer)
	protected.POST("/practice/attempts/:id/finish", a.finishAttempt)
	protected.POST("/practice/attempts/:id/answers/:qid/self-rating", a.selfRateAnswer)
	protected.GET("/practice/wrongbook", a.wrongBook)
	protected.GET("/practice/stats", a.practiceStats)
	staff := protected.Group("")
	staff.Use(a.requireRole("teacher", "admin"))
	staff.GET("/admin/questions", a.listQuestions)
	staff.POST("/admin/questions", a.createQuestion)
	staff.GET("/admin/questions/import/template", a.questionImportTemplate)
	staff.POST("/admin/questions/import/preview", a.questionImportPreview)
	staff.POST("/admin/questions/import/commit", a.questionImportCommit)
	staff.GET("/admin/tags", a.listTags)
	staff.GET("/admin/sets", a.listSets)
	staff.POST("/admin/sets", a.createSet)
	staff.GET("/admin/sets/:id", a.getSet)
	staff.PATCH("/admin/sets/:id", a.updateSet)
	staff.GET("/admin/analytics/overview", a.analyticsOverview)
	staff.GET("/admin/analytics/users/:id", a.analyticsUserDetail)
	admin := protected.Group("")
	admin.Use(a.requireAdmin())
	admin.PATCH("/concepts/:id/content", a.updateConceptContent)
	admin.GET("/import/status", a.importStatus)
	admin.POST("/import/run", a.importRun)
	admin.GET("/admin/users", a.listUsers)
	admin.PATCH("/admin/users/:id", a.updateUser)
	if _, err := os.Stat("frontend/dist/index.html"); err == nil {
		r.Static("/assets", "frontend/dist/assets")
		r.StaticFile("/favicon.svg", "frontend/dist/favicon.svg")
		r.NoRoute(func(c *gin.Context) {
			c.File("frontend/dist/index.html")
		})
	}
	return r
}
