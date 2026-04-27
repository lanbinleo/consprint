package backend

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
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
	protected.GET("/dashboard", a.dashboard)
	protected.GET("/units", a.units)
	protected.GET("/concepts", a.concepts)
	protected.GET("/concepts/:id", a.concept)
	protected.PATCH("/concepts/:id/rating", a.rateConcept)
	protected.GET("/review/next", a.reviewNext)
	protected.POST("/review/events", a.reviewEvent)
	protected.GET("/import/status", a.importStatus)
	protected.POST("/import/run", a.importRun)
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
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		c.JSON(400, gin.H{"error": "tenant, email, and password are required"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "could not hash password"})
		return
	}
	tenant := Tenant{ID: NewID("ten"), Name: fallback(req.TenantName, "Personal")}
	user := User{ID: NewID("usr"), TenantID: tenant.ID, Name: fallback(req.Name, "Student"), Email: strings.ToLower(req.Email), PasswordHash: string(hash)}
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

func (a *App) me(c *gin.Context) {
	var user User
	var tenant Tenant
	a.DB.First(&user, "id = ?", c.GetString("userID"))
	a.DB.First(&tenant, "id = ?", c.GetString("tenantID"))
	c.JSON(200, gin.H{"user": user, "tenant": tenant})
}

func (a *App) dashboard(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	var total, ready, reviewed, weak int64
	a.DB.Model(&Concept{}).Count(&total)
	a.DB.Model(&Concept{}).Where("content_status <> ?", "pending").Count(&ready)
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND review_count > 0", userID).Count(&reviewed)
	a.DB.Model(&UserConceptState{}).Where("user_id = ? AND mastery < ?", userID, 3).Count(&weak)
	type Avg struct{ Avg float64 }
	var avg Avg
	a.DB.Raw("select coalesce(avg(mastery), 0) as avg from user_concept_states where user_id = ?", userID).Scan(&avg)
	var recent []ReviewEvent
	a.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(8).Find(&recent)
	c.JSON(200, gin.H{"totalConcepts": total, "readyConcepts": ready, "reviewedConcepts": reviewed, "weakConcepts": weak, "averageMastery": avg.Avg, "recent": recent})
}

func (a *App) units(c *gin.Context) {
	var units []Unit
	a.DB.Preload("Topics", func(db *gorm.DB) *gorm.DB { return db.Order("position asc") }).Order("position asc").Find(&units)
	c.JSON(200, units)
}

func (a *App) concepts(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	q := a.DB.Model(&Concept{}).Preload("Unit").Preload("Topic").Preload("Content").Order("concepts.position asc")
	if unitID := c.Query("unitId"); unitID != "" {
		q = q.Where("concepts.unit_id = ?", unitID)
	}
	if topicID := c.Query("topicId"); topicID != "" {
		q = q.Where("concepts.topic_id = ?", topicID)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		q = q.Where("lower(concepts.term) like ?", "%"+strings.ToLower(search)+"%")
	}
	var concepts []Concept
	q.Limit(1000).Find(&concepts)
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

func (a *App) reviewNext(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	var rows []struct {
		Concept
		Mastery float64 `json:"mastery"`
	}
	a.DB.Raw(`select c.*, s.mastery from concepts c join user_concept_states s on s.concept_id = c.id where s.user_id = ? order by s.short_term_review desc, s.mastery asc, random() limit 30`, userID).Scan(&rows)
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	var concepts []Concept
	a.DB.Preload("Unit").Preload("Topic").Preload("Content").Preload("Cards").Find(&concepts, "id in ?", ids)
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

func (a *App) importStatus(c *gin.Context) {
	var runs []ImportRun
	a.DB.Order("created_at desc").Limit(20).Find(&runs)
	var units, topics, concepts, ready int64
	a.DB.Model(&Unit{}).Count(&units)
	a.DB.Model(&Topic{}).Count(&topics)
	a.DB.Model(&Concept{}).Count(&concepts)
	a.DB.Model(&Concept{}).Where("content_status <> ?", "pending").Count(&ready)
	c.JSON(200, gin.H{"units": units, "topics": topics, "concepts": concepts, "readyConcepts": ready, "runs": runs})
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

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}

var errNotFound = errors.New("not found")
