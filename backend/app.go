package backend

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type App struct {
	DB        *gorm.DB
	JWTSecret []byte
	Sources   string
}

type Claims struct {
	UserID   string `json:"userId"`
	TenantID string `json:"tenantId"`
	jwt.RegisteredClaims
}

func NewApp(dbPath, sources string) (*App, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Tenant{}, &User{}, &Course{}, &Unit{}, &Topic{}, &Concept{}, &ConceptContent{}, &Card{}, &UserConceptState{}, &ReviewEvent{}, &ImportRun{}); err != nil {
		return nil, err
	}
	db.Model(&User{}).Where("role = '' OR role IS NULL").Update("role", "student")
	var admins int64
	db.Model(&User{}).Where("role = ?", "admin").Count(&admins)
	if admins == 0 {
		var first User
		if err := db.Order("created_at asc").First(&first).Error; err == nil {
			db.Model(&first).Update("role", "admin")
		}
	}
	app := &App{DB: db, JWTSecret: []byte(env("JWT_SECRET", "local-dev-secret-change-me")), Sources: sources}
	var count int64
	db.Model(&Concept{}).Count(&count)
	if count == 0 {
		_ = Importer{DB: db, Sources: sources}.RunAll()
	}
	return app, nil
}

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
	api.POST("/auth/register", a.register)
	api.POST("/auth/login", a.login)
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
	protected.PATCH("/concepts/:id/rating", a.rateConcept)
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

func (a *App) register(c *gin.Context) {
	var req struct {
		TenantName string `json:"tenantName"`
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
	var userCount int64
	a.DB.Model(&User{}).Count(&userCount)
	role := "student"
	if userCount == 0 {
		role = "admin"
	}
	tenant := Tenant{ID: NewID("ten"), Name: fallback(req.TenantName, "Personal")}
	user := User{ID: NewID("usr"), TenantID: tenant.ID, Name: fallback(req.Name, "Student"), Email: strings.ToLower(req.Email), Role: role, PasswordHash: string(hash)}
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&tenant).Error; err != nil {
			return err
		}
		return tx.Create(&user).Error
	})
	if err != nil {
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
		TenantName    string `json:"tenantName"`
		AvatarDataURL string `json:"avatarDataUrl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	var user User
	var tenant Tenant
	if err := a.DB.First(&user, "id = ?", c.GetString("userID")).Error; err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	a.DB.First(&tenant, "id = ?", c.GetString("tenantID"))
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
	if strings.TrimSpace(req.TenantName) != "" {
		tenant.Name = strings.TrimSpace(req.TenantName)
	}
	a.DB.Save(&user)
	a.DB.Save(&tenant)
	c.JSON(200, gin.H{"user": user, "tenant": tenant})
}

func (a *App) dashboard(c *gin.Context) {
	summary := a.dashboardSummaryPayload(c.GetString("userID"))
	progress := a.dashboardProgressPayload(c.GetString("userID"))
	trends := a.dashboardTrendsPayload(c.GetString("userID"))
	alerts := a.dashboardAlertsPayload(c.GetString("userID"))
	c.JSON(200, gin.H{
		"totalConcepts":    summary["totalConcepts"],
		"readyConcepts":    summary["readyConcepts"],
		"reviewedConcepts": progress["reviewedConcepts"],
		"ratedConcepts":    progress["ratedConcepts"],
		"weakConcepts":     progress["weakConcepts"],
		"averageMastery":   progress["averageMastery"],
		"todayReviews":     progress["todayReviews"],
		"todayMasteryGain": progress["todayMasteryGain"],
		"shortTermReviews": progress["shortTermReviews"],
		"streakDays":       progress["streakDays"],
		"recent":           alerts["recent"],
		"weakUnits":        alerts["weakUnits"],
		"weakTopics":       alerts["weakTopics"],
		"weakConceptsList": alerts["weakConcepts"],
		"daily":            trends["daily"],
		"hourly":           trends["hourly"],
	})
}

func (a *App) dashboardSummary(c *gin.Context) {
	c.JSON(200, a.dashboardSummaryPayload(c.GetString("userID")))
}

func (a *App) dashboardProgress(c *gin.Context) {
	c.JSON(200, a.dashboardProgressPayload(c.GetString("userID")))
}

func (a *App) dashboardTrends(c *gin.Context) {
	c.JSON(200, a.dashboardTrendsPayload(c.GetString("userID")))
}

func (a *App) dashboardAlerts(c *gin.Context) {
	c.JSON(200, a.dashboardAlertsPayload(c.GetString("userID")))
}

func (a *App) dashboardSummaryPayload(userID string) gin.H {
	a.ensureStates(userID)
	var total, ready int64
	a.DB.Model(&Concept{}).Count(&total)
	a.DB.Model(&Concept{}).Where("content_status <> ?", "pending").Count(&ready)
	return gin.H{"totalConcepts": total, "readyConcepts": ready}
}

func (a *App) dashboardProgressPayload(userID string) gin.H {
	a.ensureStates(userID)
	var reviewed, weak, rated int64
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND review_count > 0", userID).Count(&reviewed)
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND mastery > 0", userID).Count(&rated)
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND mastery > 0 AND mastery < ?", userID, 3).Count(&weak)
	var shortTerm int64
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND short_term_review = ?", userID, true).Count(&shortTerm)
	type Avg struct{ Avg float64 }
	var avg Avg
	a.DB.Raw("select coalesce(avg(mastery), 0) as avg from user_concept_states where user_id = ?", userID).Scan(&avg)
	today := todayStats(a.reviewEvents(userID))
	return gin.H{
		"reviewedConcepts": reviewed,
		"ratedConcepts":    rated,
		"weakConcepts":     weak,
		"averageMastery":   avg.Avg,
		"todayReviews":     today.Reviews,
		"todayMasteryGain": today.Gain,
		"shortTermReviews": shortTerm,
		"streakDays":       a.streakDays(userID),
	}
}

func (a *App) dashboardTrendsPayload(userID string) gin.H {
	return gin.H{"daily": a.dailyStats(userID), "hourly": a.hourlyStats(userID)}
}

func (a *App) dashboardAlertsPayload(userID string) gin.H {
	a.ensureStates(userID)
	var recent []ReviewEvent
	a.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(8).Find(&recent)
	var weak []Concept
	a.DB.Model(&Concept{}).
		Select("concepts.*").
		Joins("join user_concept_states s on s.concept_id = concepts.id and s.user_id = ?", userID).
		Where("s.mastery > 0 and s.mastery < ?", 3).
		Order("s.mastery asc, s.updated_at desc").
		Limit(6).
		Find(&weak)
	return gin.H{"recent": recent, "weakConcepts": weak, "weakUnits": a.weakUnitStats(userID), "weakTopics": a.weakTopicStats(userID)}
}

func (a *App) units(c *gin.Context) {
	var units []Unit
	a.DB.Preload("Topics", func(db *gorm.DB) *gorm.DB { return db.Order("position asc") }).Order("position asc").Find(&units)
	c.JSON(200, units)
}

func (a *App) concepts(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	q := a.DB.Model(&Concept{}).
		Preload("Unit").
		Preload("Topic").
		Preload("Content").
		Joins("join units on units.id = concepts.unit_id").
		Joins("join topics on topics.id = concepts.topic_id").
		Order("units.position asc, topics.position asc, concepts.position asc")
	if unitID := c.Query("unitId"); unitID != "" {
		q = q.Where("concepts.unit_id = ?", unitID)
	}
	if topicID := c.Query("topicId"); topicID != "" {
		q = q.Where("concepts.topic_id = ?", topicID)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		q = q.Where("lower(concepts.term) like ?", "%"+strings.ToLower(search)+"%")
	}
	if progress := c.Query("progress"); progress != "" {
		q = q.Joins("join user_concept_states filter_state on filter_state.concept_id = concepts.id and filter_state.user_id = ?", userID)
		switch progress {
		case "zero":
			q = q.Where("filter_state.mastery = 0")
		case "nonzero":
			q = q.Where("filter_state.mastery > 0")
		case "weak":
			q = q.Where("filter_state.mastery > 0 and filter_state.mastery < 3")
		}
	}
	var concepts []Concept
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "1000"))
	if limit <= 0 {
		limit = 1000
	}
	if limit > 1000 {
		limit = 1000
	}
	q.Limit(limit).Find(&concepts)
	states := map[string]UserConceptState{}
	var stateRows []UserConceptState
	a.DB.Where("user_id = ?", userID).Find(&stateRows)
	for _, s := range stateRows {
		states[s.ConceptID] = s
	}
	type row struct {
		Concept
		State UserConceptState `json:"state"`
	}
	out := make([]row, 0, len(concepts))
	for _, concept := range concepts {
		out = append(out, row{Concept: concept, State: states[concept.ID]})
	}
	c.JSON(200, out)
}

func (a *App) concept(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	var concept Concept
	if err := a.DB.Preload("Unit").Preload("Topic").Preload("Content").Preload("Cards").First(&concept, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var state UserConceptState
	a.DB.First(&state, "user_id = ? AND concept_id = ?", userID, concept.ID)
	c.JSON(200, gin.H{"concept": concept, "state": state})
}

func (a *App) rateConcept(c *gin.Context) {
	var req struct {
		Rating int `json:"rating"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Rating < 0 || req.Rating > 5 {
		c.JSON(400, gin.H{"error": "rating must be 0-5"})
		return
	}
	userID := c.GetString("userID")
	state, err := a.stateFor(userID, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "concept not found"})
		return
	}
	state.Mastery = float64(req.Rating)
	state.ManualRating = &req.Rating
	a.DB.Save(&state)
	c.JSON(200, state)
}

func (a *App) updateConceptContent(c *gin.Context) {
	var req struct {
		Definition []map[string]string `json:"definition"`
		Examples   []map[string]string `json:"examples"`
		Pitfalls   []map[string]string `json:"pitfalls"`
		Notes      []map[string]string `json:"notes"`
		Source     string              `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	conceptID := c.Param("id")
	var concept Concept
	if err := a.DB.First(&concept, "id = ?", conceptID).Error; err != nil {
		c.JSON(404, gin.H{"error": "concept not found"})
		return
	}
	content := ConceptContent{ID: conceptID + ".content", ConceptID: conceptID}
	a.DB.FirstOrCreate(&content, "concept_id = ?", conceptID)
	content.Definition = marshalBlocks(req.Definition)
	content.Examples = marshalBlocks(req.Examples)
	content.Pitfalls = marshalBlocks(req.Pitfalls)
	content.Notes = marshalBlocks(req.Notes)
	content.Source = fallback(req.Source, "manual")
	content.Confidence = 1
	content.NeedsReview = false
	if err := a.DB.Save(&content).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not save content"})
		return
	}
	a.DB.Model(&Concept{}).Where("id = ?", conceptID).Update("content_status", "ready")
	var out Concept
	a.DB.Preload("Unit").Preload("Topic").Preload("Content").Preload("Cards").First(&out, "id = ?", conceptID)
	c.JSON(200, out)
}

func (a *App) reviewNext(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	if limit <= 0 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}
	order := c.DefaultQuery("order", "random")
	var concepts []Concept
	q := a.DB.Model(&Concept{}).
		Select("concepts.*").
		Joins("join user_concept_states s on s.concept_id = concepts.id and s.user_id = ?", userID).
		Joins("join units on units.id = concepts.unit_id").
		Joins("join topics on topics.id = concepts.topic_id").
		Preload("Unit").
		Preload("Topic").
		Preload("Content").
		Preload("Cards")
	if unitID := c.Query("unitId"); unitID != "" {
		q = q.Where("concepts.unit_id = ?", unitID)
	}
	if topicID := c.Query("topicId"); topicID != "" {
		q = q.Where("concepts.topic_id = ?", topicID)
	}
	if order == "outline" {
		q = q.Order("units.position asc, topics.position asc, concepts.position asc")
	} else {
		q = q.Order("s.short_term_review desc, s.mastery asc, random()")
	}
	q.Limit(limit).Find(&concepts)
	c.JSON(200, concepts)
}

func (a *App) reviewEvent(c *gin.Context) {
	var req struct {
		ConceptID  string `json:"conceptId"`
		CardID     string `json:"cardId"`
		Response   string `json:"response"`
		DurationMS int    `json:"durationMs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Response != "know" && req.Response != "fuzzy" && req.Response != "unknown" {
		c.JSON(400, gin.H{"error": "response must be know, fuzzy, or unknown"})
		return
	}
	userID := c.GetString("userID")
	state, err := a.stateFor(userID, req.ConceptID)
	if err != nil {
		c.JSON(404, gin.H{"error": "concept not found"})
		return
	}
	before := state.Mastery
	after := nextMastery(before, req.Response)
	now := time.Now()
	state.Mastery = after
	state.ReviewCount++
	state.LastReviewedAt = &now
	state.ShortTermReview = req.Response == "unknown" || req.Response == "fuzzy"
	event := ReviewEvent{ID: NewID("rev"), UserID: userID, ConceptID: req.ConceptID, CardID: req.CardID, Response: req.Response, MasteryBefore: before, MasteryAfter: after, DurationMS: req.DurationMS, CreatedAt: now}
	a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&state).Error; err != nil {
			return err
		}
		return tx.Create(&event).Error
	})
	c.JSON(200, gin.H{"state": state, "event": event})
}

type statBucket struct {
	Label          string  `json:"label"`
	Reviews        int     `json:"reviews"`
	Learned        int     `json:"learned"`
	MasteryGain    float64 `json:"masteryGain"`
	AverageMastery float64 `json:"averageMastery"`
}

func (a *App) reviewEvents(userID string) []ReviewEvent {
	events := make([]ReviewEvent, 0)
	a.DB.Where("user_id = ?", userID).Order("created_at asc").Find(&events)
	return events
}

func appTimeLocation() *time.Location {
	if envBool("APP_USE_SYSTEM_TIMEZONE", false) {
		return time.Local
	}
	name := env("APP_TIMEZONE", "Asia/Shanghai")
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}

func appNow() time.Time {
	return time.Now().In(appTimeLocation())
}

func dayStart(t time.Time) time.Time {
	loc := appTimeLocation()
	inLoc := t.In(loc)
	return time.Date(inLoc.Year(), inLoc.Month(), inLoc.Day(), 0, 0, 0, 0, loc)
}

func todayStats(events []ReviewEvent) struct {
	Reviews int
	Gain    float64
} {
	start := dayStart(appNow())
	end := start.AddDate(0, 0, 1)
	stats := struct {
		Reviews int
		Gain    float64
	}{}
	for _, event := range events {
		at := event.CreatedAt.In(appTimeLocation())
		if at.Before(start) || !at.Before(end) {
			continue
		}
		stats.Reviews++
		if event.MasteryAfter > event.MasteryBefore {
			stats.Gain += event.MasteryAfter - event.MasteryBefore
		}
	}
	return stats
}

func (a *App) dailyStats(userID string) []statBucket {
	rows := make([]statBucket, 0, 14)
	start := dayStart(appNow()).AddDate(0, 0, -13)
	for i := 0; i < 14; i++ {
		label := start.AddDate(0, 0, i).Format("2006-01-02")
		rows = append(rows, statBucket{Label: label})
	}
	indexByLabel := make(map[string]int, len(rows))
	for i, row := range rows {
		indexByLabel[row.Label] = i
	}
	averageCounts := make([]int, len(rows))
	for _, event := range a.reviewEvents(userID) {
		at := event.CreatedAt.In(appTimeLocation())
		if at.Before(start) {
			continue
		}
		label := at.Format("2006-01-02")
		index, ok := indexByLabel[label]
		if !ok {
			continue
		}
		addEventToBucket(&rows[index], event)
		averageCounts[index]++
	}
	for i := range rows {
		if averageCounts[i] > 0 {
			rows[i].AverageMastery /= float64(averageCounts[i])
		}
	}
	return rows
}

func (a *App) hourlyStats(userID string) []statBucket {
	rows := make([]statBucket, 0, 24)
	start := appNow().Truncate(time.Hour).Add(-23 * time.Hour)
	for i := 0; i < 24; i++ {
		hour := start.Add(time.Duration(i) * time.Hour)
		rows = append(rows, statBucket{Label: hour.Format("15:00")})
	}
	indexByLabel := make(map[string]int, len(rows))
	for i := range rows {
		indexByLabel[start.Add(time.Duration(i)*time.Hour).Format("2006-01-02 15:00")] = i
	}
	averageCounts := make([]int, len(rows))
	for _, event := range a.reviewEvents(userID) {
		at := event.CreatedAt.In(appTimeLocation()).Truncate(time.Hour)
		if at.Before(start) {
			continue
		}
		index, ok := indexByLabel[at.Format("2006-01-02 15:00")]
		if !ok {
			continue
		}
		addEventToBucket(&rows[index], event)
		averageCounts[index]++
	}
	for i := range rows {
		if averageCounts[i] > 0 {
			rows[i].AverageMastery /= float64(averageCounts[i])
		}
	}
	return rows
}

func addEventToBucket(bucket *statBucket, event ReviewEvent) {
	bucket.Reviews++
	if event.MasteryBefore < 4 && event.MasteryAfter >= 4 {
		bucket.Learned++
	}
	if event.MasteryAfter > event.MasteryBefore {
		bucket.MasteryGain += event.MasteryAfter - event.MasteryBefore
	}
	bucket.AverageMastery += event.MasteryAfter
}

type weakArea struct {
	Label          string  `json:"label"`
	Weak           int     `json:"weak"`
	AverageMastery float64 `json:"averageMastery"`
}

func (a *App) weakUnitStats(userID string) []weakArea {
	rows := make([]weakArea, 0)
	a.DB.Raw(`
		select u.title as label,
		       sum(case when s.mastery > 0 and s.mastery < 3 then 1 else 0 end) as weak,
		       coalesce(avg(case when s.mastery > 0 then s.mastery end), 0) as average_mastery
		from units u
		join concepts c on c.unit_id = u.id
		join user_concept_states s on s.concept_id = c.id and s.user_id = ?
		group by u.id, u.title
		having weak > 0
		order by weak desc, average_mastery asc
		limit 5
	`, userID).Scan(&rows)
	return rows
}

func (a *App) weakTopicStats(userID string) []weakArea {
	rows := make([]weakArea, 0)
	a.DB.Raw(`
		select t.title as label,
		       sum(case when s.mastery > 0 and s.mastery < 3 then 1 else 0 end) as weak,
		       coalesce(avg(case when s.mastery > 0 then s.mastery end), 0) as average_mastery
		from topics t
		join concepts c on c.topic_id = t.id
		join user_concept_states s on s.concept_id = c.id and s.user_id = ?
		group by t.id, t.title
		having weak > 0
		order by weak desc, average_mastery asc
		limit 5
	`, userID).Scan(&rows)
	return rows
}

func (a *App) streakDays(userID string) int {
	days := make(map[string]bool)
	for _, event := range a.reviewEvents(userID) {
		days[event.CreatedAt.In(appTimeLocation()).Format("2006-01-02")] = true
	}
	streak := 0
	for day := appNow(); ; day = day.AddDate(0, 0, -1) {
		if !days[day.Format("2006-01-02")] {
			break
		}
		streak++
	}
	return streak
}

func (a *App) importStatus(c *gin.Context) {
	var runs []ImportRun
	a.DB.Order("created_at desc").Limit(20).Find(&runs)
	var units, topics, concepts, ready int64
	a.DB.Model(&Unit{}).Count(&units)
	a.DB.Model(&Topic{}).Count(&topics)
	a.DB.Model(&Concept{}).Count(&concepts)
	a.DB.Model(&Concept{}).Where("content_status <> ?", "pending").Count(&ready)
	var byUnit []struct {
		UnitID   string `json:"unitId"`
		Unit     string `json:"unit"`
		Concepts int    `json:"concepts"`
		Ready    int    `json:"ready"`
	}
	a.DB.Raw(`
		select u.id as unit_id, u.title as unit, count(c.id) as concepts,
		       sum(case when c.content_status <> 'pending' then 1 else 0 end) as ready
		from units u
		left join concepts c on c.unit_id = u.id
		group by u.id, u.title, u.position
		order by u.position asc
	`).Scan(&byUnit)
	c.JSON(200, gin.H{"units": units, "topics": topics, "concepts": concepts, "readyConcepts": ready, "byUnit": byUnit, "runs": runs})
}

func (a *App) importRun(c *gin.Context) {
	if err := (Importer{DB: a.DB, Sources: a.Sources}).RunAll(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (a *App) ensureStates(userID string) {
	var concepts []Concept
	a.DB.Select("id").Find(&concepts)
	if len(concepts) == 0 {
		return
	}
	var existing int64
	a.DB.Model(&UserConceptState{}).Where("user_id = ?", userID).Count(&existing)
	if int(existing) >= len(concepts) {
		return
	}
	states := make([]UserConceptState, 0, len(concepts))
	for _, concept := range concepts {
		states = append(states, UserConceptState{ID: userID + "." + concept.ID, UserID: userID, ConceptID: concept.ID, Mastery: 0})
	}
	a.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(states, 200)
}

func (a *App) ensureState(userID, conceptID string) {
	state := UserConceptState{ID: userID + "." + conceptID, UserID: userID, ConceptID: conceptID, Mastery: 0}
	a.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&state)
}

func (a *App) stateFor(userID, conceptID string) (UserConceptState, error) {
	var concept Concept
	if err := a.DB.First(&concept, "id = ?", conceptID).Error; err != nil {
		return UserConceptState{}, err
	}
	a.ensureState(userID, conceptID)
	var state UserConceptState
	a.DB.First(&state, "user_id = ? AND concept_id = ?", userID, conceptID)
	return state, nil
}

func nextMastery(current float64, response string) float64 {
	switch response {
	case "know":
		return Clamp(current+0.45*(1-current/5), 0, 5)
	case "fuzzy":
		return Clamp(current+0.18*(1-current/5), 0, 5)
	case "unknown":
		return Clamp(current-0.12, 0, 5)
	default:
		return current
	}
}

func env(key, fallbackValue string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return fallbackValue
}

func envBool(key string, fallbackValue bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallbackValue
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}

func marshalBlocks(blocks []map[string]string) datatypes.JSON {
	if len(blocks) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	normalized := make([]map[string]string, 0, len(blocks))
	for _, block := range blocks {
		text := strings.TrimSpace(block["text"])
		if text == "" {
			continue
		}
		kind := strings.TrimSpace(block["type"])
		if kind == "" {
			kind = "paragraph"
		}
		normalized = append(normalized, map[string]string{"type": kind, "text": text})
	}
	if len(normalized) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	out, _ := json.Marshal(normalized)
	return datatypes.JSON(out)
}

var errNotFound = errors.New("not found")
