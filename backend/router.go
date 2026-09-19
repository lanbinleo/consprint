package backend

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func (a *App) Router() *gin.Engine {
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", env("CORS_ORIGIN", "http://localhost:5173"))
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	api := r.Group("/api")
	// Compress API JSON on the wire: the concept corpus is ~1.4MB raw and
	// compresses ~10x. Scoped to /api so static files and SPA assets are
	// untouched; fetch() decodes gzip transparently.
	api.Use(gzip.Gzip(gzip.DefaultCompression))
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
	protected.GET("/dashboard/starred", a.dashboardStarred)
	protected.GET("/dashboard/recent", a.dashboardRecent)
	protected.GET("/announcements", a.announcements)
	protected.GET("/calendar", a.calendarEvents)
	protected.GET("/units", a.units)
	protected.GET("/note-resources", a.noteResources)
	protected.GET("/concepts", a.concepts)
	protected.GET("/concepts/states", a.conceptStates)
	protected.GET("/content/version", a.contentVersion)
	protected.GET("/concepts/:id", a.concept)
	protected.PATCH("/concepts/:id/status", a.setConceptStatus)
	protected.PATCH("/concepts/:id/star", a.starConcept)
	protected.GET("/review/next", a.reviewNext)
	protected.GET("/review/count", a.reviewCount)
	protected.POST("/review/events", a.reviewEvent)
	protected.POST("/review/events/batch", a.reviewEventBatch)
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
	staff.GET("/admin/questions/:id", a.getQuestion)
	staff.PATCH("/admin/questions/:id", a.updateQuestion)
	staff.GET("/admin/questions/import/template", a.questionImportTemplate)
	staff.POST("/admin/questions/import/preview", a.questionImportPreview)
	staff.POST("/admin/questions/import/commit", a.questionImportCommit)
	staff.GET("/admin/tags", a.listTags)
	staff.GET("/admin/note-resources", a.listNoteResources)
	staff.POST("/admin/note-resources", a.createNoteResource)
	staff.PATCH("/admin/note-resources/:id", a.updateNoteResource)
	staff.DELETE("/admin/note-resources/:id", a.deleteNoteResource)
	staff.POST("/admin/note-resources/upload", a.uploadNoteResourceFile)
	staff.GET("/admin/announcements", a.listAnnouncements)
	staff.POST("/admin/announcements", a.createAnnouncement)
	staff.PATCH("/admin/announcements/:id", a.updateAnnouncement)
	staff.DELETE("/admin/announcements/:id", a.deleteAnnouncement)
	staff.GET("/admin/calendar", a.listCalendarEvents)
	staff.POST("/admin/calendar", a.createCalendarEvent)
	staff.PATCH("/admin/calendar/:id", a.updateCalendarEvent)
	staff.DELETE("/admin/calendar/:id", a.deleteCalendarEvent)
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
	// /files serves uploaded note materials (PDFs, images) from the data dir.
	// It must stay public: <iframe>/<img> subresources cannot carry the
	// Authorization header. /avatars and /fonts fix the production binary
	// missing frontend/public assets that Vite only serves in dev.
	filesDir := a.PublicDir
	if filesDir == "" {
		filesDir = filepath.Join("data", "public")
	}
	r.Static("/files", filesDir)
	if _, err := os.Stat("frontend/public/avatars"); err == nil {
		r.Static("/avatars", "frontend/public/avatars")
	}
	if _, err := os.Stat("frontend/public/fonts"); err == nil {
		r.Static("/fonts", "frontend/public/fonts")
	}
	if _, err := os.Stat("frontend/dist/index.html"); err == nil {
		r.Static("/assets", "frontend/dist/assets")
		r.StaticFile("/favicon.svg", "frontend/dist/favicon.svg")
		r.NoRoute(func(c *gin.Context) {
			// API paths must never fall through to the SPA shell: a 200 HTML
			// response breaks frontend JSON parsing.
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			c.File("frontend/dist/index.html")
		})
	}
	return r
}
