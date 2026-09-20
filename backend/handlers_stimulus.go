package backend

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

// Staff management of shared reading materials (stimuli): the MCQ passage of
// a question set, the AAQ article, or the EBQ's three sources. Documents are
// plain markdown and may embed uploaded images.

func stimulusDocuments(raw datatypes.JSON) []StimulusDocument {
	var docs []StimulusDocument
	if len(raw) == 0 {
		return docs
	}
	if err := json.Unmarshal(raw, &docs); err != nil {
		return nil
	}
	return docs
}

// normalizeStimulusDocuments drops empty documents and trims the rest.
func normalizeStimulusDocuments(docs []StimulusDocument) []StimulusDocument {
	out := make([]StimulusDocument, 0, len(docs))
	for _, doc := range docs {
		doc.Title = strings.TrimSpace(doc.Title)
		doc.Text = strings.TrimSpace(doc.Text)
		if doc.Title == "" && doc.Text == "" {
			continue
		}
		out = append(out, doc)
	}
	return out
}

func validStimulusKind(kind string) bool {
	return kind == "passage" || kind == "article" || kind == "sources"
}

func (a *App) listStimuli(c *gin.Context) {
	query := a.DB.Model(&Stimulus{})
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		needle := "%" + strings.ToLower(search) + "%"
		query = query.Where("lower(title) like ?", needle)
	}
	if kind := c.Query("kind"); kind != "" {
		query = query.Where("kind = ?", kind)
	}
	var stimuli []Stimulus
	query.Order("created_at desc").Limit(500).Find(&stimuli)
	// One grouped query tells the editor how many questions reference each
	// stimulus (and thus how disruptive an edit would be).
	usage := make(map[string]int, len(stimuli))
	if len(stimuli) > 0 {
		ids := make([]string, 0, len(stimuli))
		for _, stimulus := range stimuli {
			ids = append(ids, stimulus.ID)
		}
		rows := []struct {
			StimulusID string
			Count      int
		}{}
		a.DB.Model(&Question{}).Select("stimulus_id as stimulus_id, count(*) as count").
			Where("stimulus_id in ?", ids).Group("stimulus_id").Scan(&rows)
		for _, row := range rows {
			usage[row.StimulusID] = row.Count
		}
	}
	type row struct {
		Stimulus
		DocumentCount int `json:"documentCount"`
		Questions     int `json:"questions"`
	}
	out := make([]row, 0, len(stimuli))
	for _, stimulus := range stimuli {
		docs := stimulusDocuments(stimulus.Documents)
		if docs == nil {
			stimulus.Documents = datatypes.JSON([]byte("[]"))
			docs = []StimulusDocument{}
		}
		out = append(out, row{Stimulus: stimulus, DocumentCount: len(docs), Questions: usage[stimulus.ID]})
	}
	c.JSON(200, out)
}

type stimulusRequest struct {
	Title     *string            `json:"title"`
	Kind      *string            `json:"kind"`
	Documents []StimulusDocument `json:"documents"`
}

// applyStimulusRequest validates and applies title/kind/documents to a
// stimulus, returning a 400 message or "" when the row is valid.
func applyStimulusRequest(stimulus *Stimulus, req stimulusRequest) string {
	if req.Title != nil {
		stimulus.Title = strings.TrimSpace(*req.Title)
	}
	if req.Kind != nil {
		stimulus.Kind = strings.ToLower(strings.TrimSpace(*req.Kind))
	}
	if req.Documents != nil {
		stimulus.Documents = mustJSON(normalizeStimulusDocuments(req.Documents))
	}
	if stimulus.Title == "" {
		return "title is required"
	}
	if !validStimulusKind(stimulus.Kind) {
		return "kind must be passage, article, or sources"
	}
	docs := stimulusDocuments(stimulus.Documents)
	if len(docs) == 0 {
		return "at least one document with text is required"
	}
	for _, doc := range docs {
		if strings.TrimSpace(doc.Text) == "" {
			return "every document needs text"
		}
	}
	return ""
}

func (a *App) createStimulus(c *gin.Context) {
	var req stimulusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	stimulus := Stimulus{
		ID:        NewID("sti"),
		Kind:      "passage",
		CreatedBy: c.GetString("userID"),
	}
	if message := applyStimulusRequest(&stimulus, req); message != "" {
		c.JSON(400, gin.H{"error": message})
		return
	}
	if err := a.DB.Create(&stimulus).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not create stimulus"})
		return
	}
	c.JSON(200, stimulus)
}

func (a *App) updateStimulus(c *gin.Context) {
	var stimulus Stimulus
	if err := a.DB.First(&stimulus, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var req stimulusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if message := applyStimulusRequest(&stimulus, req); message != "" {
		c.JSON(400, gin.H{"error": message})
		return
	}
	if err := a.DB.Save(&stimulus).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not save stimulus"})
		return
	}
	c.JSON(200, stimulus)
}
