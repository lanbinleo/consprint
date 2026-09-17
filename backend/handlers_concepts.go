package backend

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
		case "unmarked":
			q = q.Where("filter_state.status = ''")
		case "marked":
			q = q.Where("filter_state.status <> ''")
		case "proficient", "fuzzy", "unknown":
			q = q.Where("filter_state.status = ?", progress)
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
	state.Status = req.Status
	state.ShortTermReview = req.Status == "fuzzy" || req.Status == "unknown"
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
	a.DB.Preload("Unit").Preload("Topic").Preload("Content").First(&out, "id = ?", conceptID)
	c.JSON(200, out)
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
