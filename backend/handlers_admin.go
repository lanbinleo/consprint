package backend

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Staff (teacher + admin) question bank management, bulk import, practice
// set assembly, and admin-only user management.

func (a *App) listQuestions(c *gin.Context) {
	q := a.DB.Model(&Question{}).Preload("Tags")
	if qType := c.Query("type"); qType != "" {
		q = q.Where("type = ?", qType)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	} else {
		q = q.Where("status <> ?", "archived")
	}
	if unitID := c.Query("unitId"); unitID != "" {
		q = q.Where("unit_id = ?", unitID)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		q = q.Where("lower(stem) like ?", "%"+strings.ToLower(search)+"%")
	}
	if tagsParam := c.Query("tags"); tagsParam != "" {
		names := splitList(tagsParam, ",")
		if len(names) > 0 {
			q = q.Joins("join question_tags qt on qt.question_id = questions.id").
				Joins("join tags t on t.id = qt.tag_id and lower(t.name) in ?", names)
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	var questions []Question
	q.Order("created_at desc").Limit(limit).Find(&questions)
	c.JSON(200, questions)
}

func (a *App) getQuestion(c *gin.Context) {
	var question Question
	if err := a.DB.Preload("Tags").First(&question, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, question)
}

func (a *App) createQuestion(c *gin.Context) {
	var draft questionDraft
	if err := c.ShouldBindJSON(&draft); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if err := validateQuestionDraft(&draft); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	question, err := a.createQuestionFromDraft(draft, "manual", c.GetString("userID"))
	if err != nil {
		c.JSON(500, gin.H{"error": "could not create question"})
		return
	}
	a.DB.Preload("Tags").First(question, "id = ?", question.ID)
	c.JSON(200, question)
}

func (a *App) updateQuestion(c *gin.Context) {
	var question Question
	if err := a.DB.Preload("Tags").First(&question, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Type        *string            `json:"type"`
		Stem        *string            `json:"stem"`
		Materials   []QuestionMaterial `json:"materials"`
		Choices     []QuestionChoice   `json:"choices"`
		AnswerKey   *string            `json:"answerKey"`
		Explanation *string            `json:"explanation"`
		Parts       []QuestionPart     `json:"parts"`
		Unit        *string            `json:"unit"`
		Topic       *string            `json:"topic"`
		Tags        *[]string          `json:"tags"`
		Status      *string            `json:"status"`
		SourceNote  *string            `json:"sourceNote"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Type != nil {
		question.Type = *req.Type
	}
	if req.Stem != nil {
		question.Stem = strings.TrimSpace(*req.Stem)
	}
	if req.Materials != nil {
		question.Materials = mustJSON(req.Materials)
	}
	if req.Choices != nil {
		question.Choices = mustJSON(req.Choices)
	}
	if req.AnswerKey != nil {
		question.AnswerKey = strings.ToUpper(strings.TrimSpace(*req.AnswerKey))
	}
	if req.Explanation != nil {
		question.Explanation = strings.TrimSpace(*req.Explanation)
	}
	if req.Parts != nil {
		question.Parts = mustJSON(req.Parts)
	}
	if req.Status != nil {
		if *req.Status != "draft" && *req.Status != "published" && *req.Status != "archived" {
			c.JSON(400, gin.H{"error": "invalid status"})
			return
		}
		question.Status = *req.Status
	}
	if req.SourceNote != nil {
		question.SourceNote = strings.TrimSpace(*req.SourceNote)
	}
	unitRef := ""
	if req.Unit != nil {
		unitRef = *req.Unit
	} else if question.UnitID != nil {
		unitRef = *question.UnitID
	}
	topicRef := ""
	if req.Topic != nil {
		topicRef = *req.Topic
	} else if question.TopicID != nil {
		topicRef = *question.TopicID
	}
	question.UnitID, question.TopicID = a.resolveUnitTopicLinked(normalizeUnitRef(unitRef), topicRef)
	if req.Tags != nil {
		question.Tags = a.findOrCreateTags(*req.Tags)
	}
	if err := a.DB.Session(&gorm.Session{FullSaveAssociations: true}).Save(&question).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not save question"})
		return
	}
	a.DB.Preload("Tags").First(&question, "id = ?", question.ID)
	c.JSON(200, question)
}

func (a *App) listTags(c *gin.Context) {
	var rows []struct {
		Name      string `json:"name"`
		Questions int    `json:"questions"`
	}
	a.DB.Raw(`
		select t.name as name, count(qt.question_id) as questions
		from tags t
		left join question_tags qt on qt.tag_id = t.id
		group by t.id, t.name
		having questions > 0
		order by questions desc, name asc
	`).Scan(&rows)
	c.JSON(200, rows)
}

func (a *App) questionImportTemplate(c *gin.Context) {
	template := "type,stem,choice_a,choice_b,choice_c,choice_d,choice_e,answer,explanation,unit,topic,tags,materials,reference_answer,rubric,source_note\n" +
		"mcq,\"Which research method...?\",\"Correlational study\",\"Experiment\",\"Case study\",\"Survey\",,B,\"Experiments manipulate the IV.,\",\"1\",\"1.1\",\"unit-1;research-methods\",,,,,midterm-2027\n" +
		"subjective,\"Explain how hindsight bias...\",,,,,,,,\"1\",\"1.1\",\"unit-1;frq\",,\"Reference answer text...\",\"Point 1;Point 2\",midterm-2027\n"
	c.Header("Content-Disposition", "attachment; filename=question-import-template.csv")
	c.Data(200, "text/csv; charset=utf-8", []byte(template))
}

func (a *App) questionImportPreview(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil || header == nil {
		c.JSON(400, gin.H{"error": "upload a file field named 'file'"})
		return
	}
	defer file.Close()
	preview, err := parseQuestionFile(header)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	response := gin.H{
		"total":   preview.Total,
		"valid":   preview.Valid,
		"channel": preview.Channel,
		"items":   preview.Items,
		"errors":  preview.Errors,
	}
	if preview.Valid > 0 {
		response["importId"] = saveImportSession(preview.Channel, preview.Items)
	}
	c.JSON(200, response)
}

func (a *App) questionImportCommit(c *gin.Context) {
	var req struct {
		ImportID string `json:"importId"`
		Include  []int  `json:"include"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ImportID == "" {
		c.JSON(400, gin.H{"error": "importId is required"})
		return
	}
	session, ok := loadImportSession(req.ImportID)
	if !ok {
		c.JSON(410, gin.H{"error": "import session expired, please re-upload"})
		return
	}
	include := map[int]bool{}
	for _, index := range req.Include {
		include[index] = true
	}
	created := 0
	var failures []importIssue
	for i, draft := range session.Items {
		if len(include) > 0 && !include[i] {
			continue
		}
		if _, err := a.createQuestionFromDraft(draft, session.Channel, c.GetString("userID")); err != nil {
			failures = append(failures, importIssue{Index: i, Message: err.Error()})
			continue
		}
		created++
	}
	c.JSON(200, gin.H{"created": created, "failed": failures})
}

func validSetMode(mode string) bool {
	return mode == "instant" || mode == "exam"
}

func (a *App) listSets(c *gin.Context) {
	q := a.DB.Model(&PracticeSet{})
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	var sets []PracticeSet
	q.Order("created_at desc").Find(&sets)
	type setRow struct {
		PracticeSet
		QuestionCount int  `json:"questionCount"`
		Published     bool `json:"published"`
	}
	out := make([]setRow, 0, len(sets))
	for _, set := range sets {
		var count int64
		a.DB.Model(&PracticeSetItem{}).Where("set_id = ?", set.ID).Count(&count)
		out = append(out, setRow{PracticeSet: set, QuestionCount: int(count), Published: set.Status == "published"})
	}
	c.JSON(200, out)
}

func (a *App) getSet(c *gin.Context) {
	var set PracticeSet
	if err := a.DB.First(&set, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var items []PracticeSetItem
	a.DB.Where("set_id = ?", set.ID).Order("position asc").Find(&items)
	questionIDs := make([]string, 0, len(items))
	for _, item := range items {
		questionIDs = append(questionIDs, item.QuestionID)
	}
	var questions []Question
	if len(questionIDs) > 0 {
		a.DB.Preload("Tags").Where("id in ?", questionIDs).Find(&questions)
	}
	c.JSON(200, gin.H{"set": set, "items": items, "questions": questions})
}

func (a *App) createSet(c *gin.Context) {
	var req struct {
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		Mode         string   `json:"mode"`
		TimeLimitSec *int     `json:"timeLimitSec"`
		QuestionIDs  []string `json:"questionIds"`
		Status       string   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		c.JSON(400, gin.H{"error": "title is required"})
		return
	}
	mode := req.Mode
	if mode == "" {
		mode = "instant"
	}
	if !validSetMode(mode) {
		c.JSON(400, gin.H{"error": "mode must be instant or exam"})
		return
	}
	status := req.Status
	if status == "" {
		status = "draft"
	}
	if status != "draft" && status != "published" && status != "archived" {
		c.JSON(400, gin.H{"error": "invalid status"})
		return
	}
	set := PracticeSet{ID: NewID("set"), Title: strings.TrimSpace(req.Title), Description: strings.TrimSpace(req.Description), Mode: mode, TimeLimitSec: req.TimeLimitSec, Status: status, CreatedBy: c.GetString("userID")}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&set).Error; err != nil {
			return err
		}
		return replaceSetItems(tx, set.ID, req.QuestionIDs)
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not create set"})
		return
	}
	c.JSON(200, set)
}

func replaceSetItems(tx *gorm.DB, setID string, questionIDs []string) error {
	if err := tx.Where("set_id = ?", setID).Delete(&PracticeSetItem{}).Error; err != nil {
		return err
	}
	for position, questionID := range questionIDs {
		item := PracticeSetItem{ID: NewID("psi"), SetID: setID, QuestionID: questionID, Position: position}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *App) updateSet(c *gin.Context) {
	var set PracticeSet
	if err := a.DB.First(&set, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Title        *string  `json:"title"`
		Description  *string  `json:"description"`
		Mode         *string  `json:"mode"`
		TimeLimitSec *int     `json:"timeLimitSec"`
		Status       *string  `json:"status"`
		QuestionIDs  []string `json:"questionIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			c.JSON(400, gin.H{"error": "title cannot be empty"})
			return
		}
		set.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		set.Description = strings.TrimSpace(*req.Description)
	}
	if req.Mode != nil {
		if !validSetMode(*req.Mode) {
			c.JSON(400, gin.H{"error": "mode must be instant or exam"})
			return
		}
		set.Mode = *req.Mode
	}
	if req.TimeLimitSec != nil {
		if *req.TimeLimitSec < 0 {
			c.JSON(400, gin.H{"error": "invalid time limit"})
			return
		}
		set.TimeLimitSec = req.TimeLimitSec
	}
	if req.Status != nil {
		if *req.Status != "draft" && *req.Status != "published" && *req.Status != "archived" {
			c.JSON(400, gin.H{"error": "invalid status"})
			return
		}
		set.Status = *req.Status
	}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&set).Error; err != nil {
			return err
		}
		if req.QuestionIDs != nil {
			return replaceSetItems(tx, set.ID, req.QuestionIDs)
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not save set"})
		return
	}
	c.JSON(200, set)
}

func (a *App) listUsers(c *gin.Context) {
	q := a.DB.Model(&User{})
	if role := c.Query("role"); role != "" {
		q = q.Where("role = ?", role)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		needle := strings.ToLower(search)
		q = q.Where("lower(email) like ? or lower(name) like ?", "%"+needle+"%", "%"+needle+"%")
	}
	var users []User
	q.Order("created_at asc").Limit(1000).Find(&users)
	c.JSON(200, users)
}

func (a *App) updateUser(c *gin.Context) {
	var user User
	if err := a.DB.First(&user, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	var req struct {
		Role *string `json:"role"`
		Name *string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Role != nil {
		if *req.Role != "student" && *req.Role != "teacher" && *req.Role != "admin" {
			c.JSON(400, gin.H{"error": "role must be student, teacher, or admin"})
			return
		}
		if user.ID == c.GetString("userID") && *req.Role != "admin" {
			c.JSON(400, gin.H{"error": "you cannot remove your own admin role"})
			return
		}
		user.Role = *req.Role
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		user.Name = strings.TrimSpace(*req.Name)
	}
	a.DB.Save(&user)
	c.JSON(200, user)
}
