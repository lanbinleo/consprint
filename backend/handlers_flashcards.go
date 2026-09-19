package backend

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validConceptStatus(status string) bool {
	return status == "" || status == "proficient" || status == "fuzzy" || status == "unknown"
}

// splitCSV parses a comma-separated query value, dropping empty segments.
func splitCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// reviewScope builds the concept query shared by reviewNext and reviewCount
// (topic/unit/status filters). It writes a 400 response and returns nil on an
// invalid status filter.
func (a *App) reviewScope(c *gin.Context, userID string) *gorm.DB {
	q := a.DB.Model(&Concept{}).
		Joins("join user_concept_states s on s.concept_id = concepts.id and s.user_id = ?", userID).
		Joins("join units on units.id = concepts.unit_id").
		Joins("join topics on topics.id = concepts.topic_id")
	// topicId and topicIds (comma-separated) are unioned; when any topic id is
	// present it takes precedence over unitId (a topic belongs to one unit).
	topicIDs := splitCSV(c.Query("topicIds"))
	if topicID := c.Query("topicId"); topicID != "" {
		topicIDs = append(topicIDs, topicID)
	}
	if len(topicIDs) > 0 {
		q = q.Where("concepts.topic_id in ?", topicIDs)
	} else if unitID := c.Query("unitId"); unitID != "" {
		q = q.Where("concepts.unit_id = ?", unitID)
	}
	if raw := c.Query("status"); raw != "" {
		parts := splitCSV(raw)
		statuses := make([]string, 0, len(parts))
		for _, part := range parts {
			if part == "unmarked" {
				part = "" // unmarked states store an empty status
			} else if !validConceptStatus(part) {
				c.JSON(400, gin.H{"error": "status must be proficient, fuzzy, unknown, or unmarked"})
				return nil
			}
			statuses = append(statuses, part)
		}
		if len(statuses) == 1 {
			q = q.Where("s.status = ?", statuses[0])
		} else if len(statuses) > 1 {
			q = q.Where("s.status in ?", statuses)
		}
	}
	return q
}

// conceptWithState is the review payload row: the concept plus the caller's
// per-concept state (status, star) in one flat object.
type conceptWithState struct {
	Concept
	State UserConceptState `json:"state"`
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
	scope := a.reviewScope(c, userID)
	if scope == nil {
		return
	}
	concepts := make([]Concept, 0)
	q := scope.
		Select("concepts.*").
		Preload("Unit").
		Preload("Topic").
		Preload("Content")
	if order == "outline" {
		q = q.Order("units.position asc, topics.position asc, concepts.position asc")
	} else {
		q = q.Order("s.short_term_review desc, case s.status when 'unknown' then 0 when 'fuzzy' then 1 when 'proficient' then 2 else 3 end asc, random()")
	}
	q.Limit(limit).Find(&concepts)
	states := map[string]UserConceptState{}
	if len(concepts) > 0 {
		ids := make([]string, 0, len(concepts))
		for _, concept := range concepts {
			ids = append(ids, concept.ID)
		}
		var stateRows []UserConceptState
		a.DB.Where("user_id = ? and concept_id in ?", userID, ids).Find(&stateRows)
		for _, state := range stateRows {
			states[state.ConceptID] = state
		}
	}
	out := make([]conceptWithState, 0, len(concepts))
	for _, concept := range concepts {
		out = append(out, conceptWithState{Concept: concept, State: states[concept.ID]})
	}
	c.JSON(200, out)
}

// reviewCount reports how many concepts match the same filters as reviewNext,
// capped reporting only — used by the flashcard setup screen to show totals.
func (a *App) reviewCount(c *gin.Context) {
	userID := c.GetString("userID")
	a.ensureStates(userID)
	scope := a.reviewScope(c, userID)
	if scope == nil {
		return
	}
	var total int64
	if err := scope.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"error": "count failed"})
		return
	}
	c.JSON(200, gin.H{"total": total})
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
	event := ReviewEvent{ID: NewID("rev"), UserID: userID, ConceptID: req.ConceptID, Response: req.Response, DurationMS: req.DurationMS, CreatedAt: now}
	// Update counters inside the transaction (review_count = review_count + 1)
	// so concurrent reviews cannot lose increments via read-modify-write.
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&UserConceptState{}).Where("id = ?", state.ID).Updates(map[string]any{
			"status":            req.Response,
			"review_count":      gorm.Expr("review_count + 1"),
			"last_reviewed_at":  now,
			"short_term_review": req.Response != "proficient",
		}).Error; err != nil {
			return err
		}
		return tx.Create(&event).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not record review"})
		return
	}
	a.DB.First(&state, "id = ?", state.ID)
	c.JSON(200, gin.H{"state": state, "event": event})
}

// reviewEventBatch records multiple review events in one transaction. The
// client queues marks locally for instant card flow and flushes here in
// batches (also on page exit via keepalive fetch).
func (a *App) reviewEventBatch(c *gin.Context) {
	var req struct {
		Events []struct {
			ConceptID  string `json:"conceptId"`
			Response   string `json:"response"`
			DurationMS int    `json:"durationMs"`
		} `json:"events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Events) == 0 {
		c.JSON(400, gin.H{"error": "events must be a non-empty array"})
		return
	}
	if len(req.Events) > 200 {
		c.JSON(400, gin.H{"error": "too many events in one batch (max 200)"})
		return
	}
	userID := c.GetString("userID")
	type pending struct {
		stateID  string
		response string
		event    ReviewEvent
	}
	items := make([]pending, 0, len(req.Events))
	seen := map[string]bool{}
	for i := range req.Events {
		e := req.Events[i]
		if e.Response != "proficient" && e.Response != "fuzzy" && e.Response != "unknown" {
			c.JSON(400, gin.H{"error": "response must be proficient, fuzzy, or unknown"})
			return
		}
		// Duplicate concept ids in one batch keep only the last response.
		if seen[e.ConceptID] {
			for j := range items {
				if items[j].event.ConceptID == e.ConceptID {
					items = append(items[:j], items[j+1:]...)
					break
				}
			}
		}
		seen[e.ConceptID] = true
		state, err := a.stateFor(userID, e.ConceptID)
		if err != nil {
			c.JSON(404, gin.H{"error": "concept not found: " + e.ConceptID})
			return
		}
		// Per-event timestamps (not one shared "now") keep intra-batch
		// ordering meaningful for latest-event-per-concept consumers.
		items = append(items, pending{
			stateID:  state.ID,
			response: e.Response,
			event:    ReviewEvent{ID: NewID("rev"), UserID: userID, ConceptID: e.ConceptID, Response: e.Response, DurationMS: e.DurationMS, CreatedAt: time.Now()},
		})
	}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			if err := tx.Model(&UserConceptState{}).Where("id = ?", item.stateID).Updates(map[string]any{
				"status":            item.response,
				"review_count":      gorm.Expr("review_count + 1"),
				"last_reviewed_at":  item.event.CreatedAt,
				"short_term_review": item.response != "proficient",
			}).Error; err != nil {
				return err
			}
			if err := tx.Create(&item.event).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not record review"})
		return
	}
	c.JSON(200, gin.H{"recorded": len(items)})
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
