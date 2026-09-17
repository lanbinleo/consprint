package backend

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validConceptStatus(status string) bool {
	return status == "" || status == "proficient" || status == "fuzzy" || status == "unknown"
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
		Preload("Content")
	if unitID := c.Query("unitId"); unitID != "" {
		q = q.Where("concepts.unit_id = ?", unitID)
	}
	if topicID := c.Query("topicId"); topicID != "" {
		q = q.Where("concepts.topic_id = ?", topicID)
	}
	if status := c.Query("status"); status != "" && validConceptStatus(status) {
		q = q.Where("s.status = ?", status)
	}
	if order == "outline" {
		q = q.Order("units.position asc, topics.position asc, concepts.position asc")
	} else {
		q = q.Order("s.short_term_review desc, case s.status when 'unknown' then 0 when 'fuzzy' then 1 when 'proficient' then 2 else 3 end asc, random()")
	}
	q.Limit(limit).Find(&concepts)
	c.JSON(200, concepts)
}

func (a *App) reviewEvent(c *gin.Context) {
	var req struct {
		ConceptID  string `json:"conceptId"`
		Response   string `json:"response"`
		DurationMS int    `json:"durationMs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Response != "proficient" && req.Response != "fuzzy" && req.Response != "unknown" {
		c.JSON(400, gin.H{"error": "response must be proficient, fuzzy, or unknown"})
		return
	}
	userID := c.GetString("userID")
	state, err := a.stateFor(userID, req.ConceptID)
	if err != nil {
		c.JSON(404, gin.H{"error": "concept not found"})
		return
	}
	now := time.Now()
	state.Status = req.Response
	state.ReviewCount++
	state.LastReviewedAt = &now
	state.ShortTermReview = req.Response != "proficient"
	event := ReviewEvent{ID: NewID("rev"), UserID: userID, ConceptID: req.ConceptID, Response: req.Response, DurationMS: req.DurationMS, CreatedAt: now}
	a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&state).Error; err != nil {
			return err
		}
		return tx.Create(&event).Error
	})
	c.JSON(200, gin.H{"state": state, "event": event})
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
		states = append(states, UserConceptState{ID: userID + "." + concept.ID, UserID: userID, ConceptID: concept.ID})
	}
	a.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(states, 200)
}

func (a *App) ensureState(userID, conceptID string) {
	state := UserConceptState{ID: userID + "." + conceptID, UserID: userID, ConceptID: conceptID}
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
