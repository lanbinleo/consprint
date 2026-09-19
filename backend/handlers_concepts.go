package backend

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// conceptStateLite is the compact per-user progress row shared by the concept
// cache sync (/api/concepts/states) and the flashcard deck (/api/review/next):
// conceptId plus the fields the UI reads, none of the audit columns.
type conceptStateLite struct {
	ConceptID       string `json:"conceptId"`
	Status          string `json:"status"`
	ReviewCount     int    `json:"reviewCount"`
	ShortTermReview bool   `json:"shortTermReview"`
	Starred         bool   `json:"starred"`
}

// maxUpdatedAt reports the newest updated_at in the given query as unix
// milliseconds, or 0 when there is nothing to report. It plucks the column and
// maxes in Go because SQLite's max() aggregate drops the datetime decltype, so
// the driver would hand back strings instead of time.Time.
func maxUpdatedAt(db *gorm.DB) int64 {
	times := make([]time.Time, 0)
	if err := db.Pluck("updated_at", &times).Error; err != nil {
		return 0
	}
	var newest int64
	for _, t := range times {
		if ms := t.UnixMilli(); ms > newest {
			newest = ms
		}
	}
	return newest
}

// contentVersion is the cheap change signal for the client-side concept cache:
// one tiny request tells the caller whether the cached corpus (contentVersion
// + row count) and their own progress (stateVersion) are still current, so an
// unchanged reload transfers no list data at all. Marks bump only
// stateVersion; content edits and imports bump only contentVersion.
func (a *App) contentVersion(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	var conceptCount int64
	a.DB.Model(&Concept{}).Count(&conceptCount)
	contentVersion := max(
		maxUpdatedAt(a.DB.Model(&Concept{})),
		maxUpdatedAt(a.DB.Model(&ConceptContent{})),
		maxUpdatedAt(a.DB.Model(&Unit{})),
		maxUpdatedAt(a.DB.Model(&Topic{})),
	)
	c.JSON(200, gin.H{
		"contentVersion": contentVersion,
		"conceptCount":   conceptCount,
		"stateVersion":   maxUpdatedAt(a.DB.Model(&UserConceptState{}).Where("user_id = ?", userID)),
	})
}

// conceptStates returns the caller's per-concept progress without concept
// payloads. Only rows deviating from the default (unmarked, unstarred, not
// queued) are included — absent conceptIds mean the default state — so the
// response stays tiny for fresh accounts. This is what recalibrates a locally
// cached concept list after progress made on another device.
func (a *App) conceptStates(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	rows := make([]conceptStateLite, 0)
	a.DB.Model(&UserConceptState{}).
		Where("user_id = ? and (status <> '' or starred or short_term_review)", userID).
		Order("concept_id asc").
		Find(&rows)
	c.JSON(200, rows)
}

func (a *App) units(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	unitRows := make([]Unit, 0)
	a.DB.Preload("Topics", func(db *gorm.DB) *gorm.DB { return db.Order("position asc") }).Order("position asc").Find(&unitRows)
	// Per-topic concept totals split by the user's assessment status, so the
	// flashcard setup screen can sum card counts locally.
	var countRows []struct {
		TopicID    string
		Total      int64
		Proficient int64
		Fuzzy      int64
		Unknown    int64
	}
	a.DB.Raw(`
		select c.topic_id as topic_id, count(*) as total,
		       sum(case when s.status = 'proficient' then 1 else 0 end) as proficient,
		       sum(case when s.status = 'fuzzy' then 1 else 0 end) as fuzzy,
		       sum(case when s.status = 'unknown' then 1 else 0 end) as unknown
		from concepts c
		join user_concept_states s on s.concept_id = c.id and s.user_id = ?
		group by c.topic_id
	`, userID).Scan(&countRows)
	counts := map[string]*TopicCounts{}
	for i := range countRows {
		counts[countRows[i].TopicID] = &TopicCounts{
			Total:      countRows[i].Total,
			Proficient: countRows[i].Proficient,
			Fuzzy:      countRows[i].Fuzzy,
			Unknown:    countRows[i].Unknown,
		}
	}
	// Normalize nil slices so JSON emits [] instead of null.
	for i := range unitRows {
		if unitRows[i].Topics == nil {
			unitRows[i].Topics = []Topic{}
		}
		for j := range unitRows[i].Topics {
			unitRows[i].Topics[j].Counts = counts[unitRows[i].Topics[j].ID]
		}
	}
	c.JSON(200, unitRows)
}

// concepts serves the concept corpus for the client-side cache: concept rows
// with content but WITHOUT the Unit/Topic relation objects (the client already
// has /api/units) or the merged per-user state (that lives in
// /api/concepts/states). updatedSince narrows the response to rows changed at
// or after the boundary for incremental sync.
func (a *App) concepts(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	q := a.DB.Model(&Concept{}).
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
		case "unmarked":
			q = q.Where("filter_state.status = ''")
		case "marked":
			q = q.Where("filter_state.status <> ''")
		case "proficient", "fuzzy", "unknown":
			q = q.Where("filter_state.status = ?", progress)
		}
	}
	if raw := c.Query("updatedSince"); raw != "" {
		if _, err := time.Parse(time.RFC3339, raw); err != nil {
			c.JSON(400, gin.H{"error": "updatedSince must be an RFC3339 timestamp"})
			return
		}
		// Stored datetimes carry the server's timezone offset while the
		// client's boundary may carry another; SQLite compares them as
		// strings, so normalize both sides with datetime() first.
		q = q.Where("datetime(concepts.updated_at) >= datetime(?)", raw)
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
	c.JSON(200, concepts)
}

func (a *App) concept(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	var concept Concept
	if err := a.DB.Preload("Unit").Preload("Topic").Preload("Content").First(&concept, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var state UserConceptState
	a.DB.First(&state, "user_id = ? AND concept_id = ?", userID, concept.ID)
	c.JSON(200, gin.H{"concept": concept, "state": state})
}

func (a *App) setConceptStatus(c *gin.Context) {
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !validConceptStatus(req.Status) {
		c.JSON(400, gin.H{"error": "status must be proficient, fuzzy, unknown, or empty"})
		return
	}
	userID := c.GetString("userID")
	state, err := a.stateFor(userID, c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "concept not found"})
		return
	}
	if req.Status == "" {
		// Clearing a mark is not a review: it just resets the state.
		state.Status = ""
		state.ShortTermReview = false
		a.DB.Save(&state)
		c.JSON(200, state)
		return
	}
	// Marking from the concept list is the same three-tier self-assessment as
	// a flashcard mark: record the append-only event and bump the counters so
	// dashboard stats and trends stay consistent across both entry points.
	now := time.Now()
	event := ReviewEvent{ID: NewID("rev"), UserID: userID, ConceptID: state.ConceptID, Response: req.Status, CreatedAt: now}
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&UserConceptState{}).Where("id = ?", state.ID).Updates(map[string]any{
			"status":            req.Status,
			"review_count":      gorm.Expr("review_count + 1"),
			"last_reviewed_at":  now,
			"short_term_review": req.Status != "proficient",
		}).Error; err != nil {
			return err
		}
		return tx.Create(&event).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not update status"})
		return
	}
	a.DB.First(&state, "id = ?", state.ID)
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
	a.DB.Preload("Unit").Preload("Topic").Preload("Content").First(&out, "id = ?", conceptID)
	c.JSON(200, out)
}

func (a *App) importStatus(c *gin.Context) {
	runs := make([]ImportRun, 0)
	a.DB.Order("created_at desc").Limit(20).Find(&runs)
	var units, topics, concepts, ready int64
	a.DB.Model(&Unit{}).Count(&units)
	a.DB.Model(&Topic{}).Count(&topics)
	a.DB.Model(&Concept{}).Count(&concepts)
	a.DB.Model(&Concept{}).Where("content_status <> ?", "pending").Count(&ready)
	byUnit := make([]struct {
		UnitID   string `json:"unitId"`
		Unit     string `json:"unit"`
		Concepts int    `json:"concepts"`
		Ready    int    `json:"ready"`
	}, 0)
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
