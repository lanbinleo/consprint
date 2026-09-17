package backend

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Student-facing practice: published practice sets, attempts in instant
// (per-question feedback) or exam (feedback withheld until finish) mode,
// subjective self-assessment, a wrong-question book, and personal stats.

func questionParts(raw datatypes.JSON) []QuestionPart {
	var parts []QuestionPart
	if len(raw) == 0 {
		return parts
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil
	}
	return parts
}

func questionChoices(raw datatypes.JSON) []QuestionChoice {
	var choices []QuestionChoice
	if len(raw) == 0 {
		return choices
	}
	if err := json.Unmarshal(raw, &choices); err != nil {
		return nil
	}
	return choices
}

func questionMaterials(raw datatypes.JSON) []QuestionMaterial {
	var materials []QuestionMaterial
	if len(raw) == 0 {
		return materials
	}
	if err := json.Unmarshal(raw, &materials); err != nil {
		return nil
	}
	return materials
}

// stripAnswer removes grading material (answer key, explanation, reference
// answers, rubrics) from a question for student-facing payloads.
func stripAnswer(q Question) gin.H {
	data, _ := json.Marshal(q)
	var out gin.H
	if err := json.Unmarshal(data, &out); err != nil {
		return gin.H{"id": q.ID, "stem": q.Stem}
	}
	delete(out, "answerKey")
	delete(out, "explanation")
	parts := questionParts(q.Parts)
	if parts != nil {
		redacted := make([]gin.H, 0, len(parts))
		for _, part := range parts {
			redacted = append(redacted, gin.H{"label": part.Label, "prompt": part.Prompt})
		}
		out["parts"] = redacted
	}
	return out
}

// revealAnswer adds grading material back for a finished attempt or after
// an instant-mode submission.
func revealAnswer(out gin.H, q Question) {
	if q.Type == "mcq" {
		out["answerKey"] = q.AnswerKey
		out["explanation"] = q.Explanation
		return
	}
	parts := questionParts(q.Parts)
	if parts != nil {
		full := make([]gin.H, 0, len(parts))
		for _, part := range parts {
			full = append(full, gin.H{
				"label":           part.Label,
				"prompt":          part.Prompt,
				"referenceAnswer": part.ReferenceAnswer,
				"rubric":          part.Rubric,
			})
		}
		out["parts"] = full
	}
}

func (a *App) loadSetQuestions(setID string) ([]Question, []PracticeSetItem) {
	var items []PracticeSetItem
	a.DB.Where("set_id = ?", setID).Order("position asc").Find(&items)
	if len(items) == 0 {
		return nil, items
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.QuestionID)
	}
	var questions []Question
	a.DB.Preload("Tags").Where("id in ?", ids).Find(&questions)
	return questions, items
}

func (a *App) practiceSets(c *gin.Context) {
	userID := c.GetString("userID")
	var sets []PracticeSet
	a.DB.Where("status = ?", "published").Order("created_at desc").Find(&sets)
	type attemptBrief struct {
		ID         string     `json:"id"`
		FinishedAt *time.Time `json:"finishedAt"`
		Score      *int       `json:"score"`
		TotalMCQ   int        `json:"totalMcq"`
	}
	type setRow struct {
		PracticeSet
		QuestionCount int            `json:"questionCount"`
		Attempts      []attemptBrief `json:"attempts"`
		BestScore     *int           `json:"bestScore"`
	}
	out := make([]setRow, 0, len(sets))
	for _, set := range sets {
		var count int64
		a.DB.Model(&PracticeSetItem{}).Where("set_id = ?", set.ID).Count(&count)
		var attempts []PracticeAttempt
		a.DB.Where("user_id = ? AND set_id = ?", userID, set.ID).Order("started_at asc").Find(&attempts)
		row := setRow{PracticeSet: set, QuestionCount: int(count)}
		best := -1
		for _, attempt := range attempts {
			brief := attemptBrief{ID: attempt.ID, FinishedAt: attempt.FinishedAt, Score: attempt.Score, TotalMCQ: attempt.TotalMCQ}
			if attempt.Score != nil && *attempt.Score > best {
				best = *attempt.Score
			}
			row.Attempts = append(row.Attempts, brief)
		}
		if best >= 0 {
			value := best
			row.BestScore = &value
		}
		out = append(out, row)
	}
	c.JSON(200, out)
}

func (a *App) practiceSetDetail(c *gin.Context) {
	var set PracticeSet
	if err := a.DB.First(&set, "id = ?", c.Param("id")).Error; err != nil || set.Status != "published" {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	questions, items := a.loadSetQuestions(set.ID)
	userID := c.GetString("userID")
	var attempts []PracticeAttempt
	a.DB.Where("user_id = ? AND set_id = ?", userID, set.ID).Order("started_at desc").Find(&attempts)
	stripped := make([]gin.H, 0, len(questions))
	byID := map[string]Question{}
	for _, q := range questions {
		byID[q.ID] = q
	}
	for _, item := range items {
		if q, ok := byID[item.QuestionID]; ok {
			stripped = append(stripped, stripAnswer(q))
		}
	}
	c.JSON(200, gin.H{"set": set, "questions": stripped, "attempts": attempts})
}

func (a *App) startAttempt(c *gin.Context) {
	var req struct {
		SetID string `json:"setId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SetID == "" {
		c.JSON(400, gin.H{"error": "setId is required"})
		return
	}
	var set PracticeSet
	if err := a.DB.First(&set, "id = ?", req.SetID).Error; err != nil || set.Status != "published" {
		c.JSON(404, gin.H{"error": "set not found"})
		return
	}
	userID := c.GetString("userID")
	var unfinished []PracticeAttempt
	a.DB.Where("user_id = ? AND set_id = ? AND finished_at IS NULL", userID, set.ID).Find(&unfinished)
	for _, attempt := range unfinished {
		// Exam attempts past their deadline are finalized before resuming.
		if attempt.DeadlineAt != nil && time.Now().After(*attempt.DeadlineAt) {
			a.finalizeAttempt(&attempt)
			continue
		}
		c.JSON(200, gin.H{"attempt": attempt, "resumed": true})
		return
	}
	now := time.Now()
	attempt := PracticeAttempt{ID: NewID("att"), UserID: userID, SetID: set.ID, Mode: set.Mode, StartedAt: now}
	if set.Mode == "exam" && set.TimeLimitSec != nil && *set.TimeLimitSec > 0 {
		deadline := now.Add(time.Duration(*set.TimeLimitSec) * time.Second)
		attempt.DeadlineAt = &deadline
	}
	if err := a.DB.Create(&attempt).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not start attempt"})
		return
	}
	c.JSON(200, gin.H{"attempt": attempt, "resumed": false})
}

func (a *App) loadOwnAttempt(c *gin.Context) (*PracticeAttempt, bool) {
	var attempt PracticeAttempt
	if err := a.DB.First(&attempt, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "attempt not found"})
		return nil, false
	}
	if attempt.UserID != c.GetString("userID") {
		c.JSON(403, gin.H{"error": "not your attempt"})
		return nil, false
	}
	return &attempt, true
}

func (a *App) attemptDetail(c *gin.Context) {
	attempt, ok := a.loadOwnAttempt(c)
	if !ok {
		return
	}
	var set PracticeSet
	a.DB.First(&set, "id = ?", attempt.SetID)
	questions, items := a.loadSetQuestions(attempt.SetID)
	var answers []PracticeAnswer
	a.DB.Where("attempt_id = ?", attempt.ID).Find(&answers)
	answerByQuestion := map[string]PracticeAnswer{}
	for _, answer := range answers {
		answerByQuestion[answer.QuestionID] = answer
	}
	byID := map[string]Question{}
	for _, q := range questions {
		byID[q.ID] = q
	}
	finished := attempt.FinishedAt != nil
	type questionRow = gin.H
	rows := make([]questionRow, 0, len(items))
	for _, item := range items {
		q, exists := byID[item.QuestionID]
		if !exists {
			continue
		}
		row := stripAnswer(q)
		if finished {
			revealAnswer(row, q)
		}
		if answer, answered := answerByQuestion[q.ID]; answered {
			if q.Type == "mcq" && attempt.Mode == "instant" {
				row["myCorrect"] = answer.IsCorrect != nil && *answer.IsCorrect
			}
		}
		rows = append(rows, row)
	}
	c.JSON(200, gin.H{"attempt": attempt, "set": set, "questions": rows, "answers": answers})
}

func (a *App) submitAnswer(c *gin.Context) {
	attempt, ok := a.loadOwnAttempt(c)
	if !ok {
		return
	}
	var req struct {
		QuestionID string `json:"questionId"`
		ChoiceKey  string `json:"choiceKey"`
		TextAnswer string `json:"textAnswer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.QuestionID == "" {
		c.JSON(400, gin.H{"error": "questionId is required"})
		return
	}
	if attempt.FinishedAt != nil {
		c.JSON(409, gin.H{"error": "attempt already finished"})
		return
	}
	if attempt.DeadlineAt != nil && time.Now().After(*attempt.DeadlineAt) {
		a.finalizeAttempt(attempt)
		c.JSON(410, gin.H{"error": "time is up", "attempt": attempt})
		return
	}
	var inSet int64
	a.DB.Model(&PracticeSetItem{}).Where("set_id = ? AND question_id = ?", attempt.SetID, req.QuestionID).Count(&inSet)
	if inSet == 0 {
		c.JSON(404, gin.H{"error": "question not in this set"})
		return
	}
	var question Question
	if err := a.DB.First(&question, "id = ?", req.QuestionID).Error; err != nil {
		c.JSON(404, gin.H{"error": "question not found"})
		return
	}
	if question.Type == "mcq" && strings.TrimSpace(req.ChoiceKey) == "" {
		c.JSON(400, gin.H{"error": "choiceKey is required for mcq"})
		return
	}
	if question.Type == "subjective" && strings.TrimSpace(req.TextAnswer) == "" {
		c.JSON(400, gin.H{"error": "textAnswer is required for subjective questions"})
		return
	}
	now := time.Now()
	var answer PracticeAnswer
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("attempt_id = ? AND question_id = ?", attempt.ID, question.ID).First(&answer).Error; err != nil {
			answer = PracticeAnswer{ID: NewID("ans"), AttemptID: attempt.ID, QuestionID: question.ID}
		}
		if question.Type == "mcq" {
			answer.ChoiceKey = strings.ToUpper(strings.TrimSpace(req.ChoiceKey))
			answer.TextAnswer = ""
			if attempt.Mode == "instant" {
				correct := strings.EqualFold(answer.ChoiceKey, strings.TrimSpace(question.AnswerKey))
				answer.IsCorrect = &correct
			}
		} else {
			answer.TextAnswer = strings.TrimSpace(req.TextAnswer)
			answer.ChoiceKey = ""
		}
		answer.AnsweredAt = now
		return tx.Save(&answer).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not save answer"})
		return
	}
	if attempt.Mode == "exam" {
		c.JSON(200, gin.H{"stored": true, "answer": answer})
		return
	}
	row := stripAnswer(question)
	revealAnswer(row, question)
	c.JSON(200, gin.H{"stored": true, "answer": answer, "question": row})
}

// finalizeAttempt grades every MCQ answer of the attempt and stamps the
// score. Idempotent.
func (a *App) finalizeAttempt(attempt *PracticeAttempt) {
	if attempt.FinishedAt != nil {
		return
	}
	questions, _ := a.loadSetQuestions(attempt.SetID)
	byID := map[string]Question{}
	totalMCQ := 0
	for _, q := range questions {
		byID[q.ID] = q
		if q.Type == "mcq" {
			totalMCQ++
		}
	}
	var answers []PracticeAnswer
	a.DB.Where("attempt_id = ?", attempt.ID).Find(&answers)
	answerByQuestion := map[string]*PracticeAnswer{}
	for i := range answers {
		answerByQuestion[answers[i].QuestionID] = &answers[i]
	}
	score := 0
	_ = a.DB.Transaction(func(tx *gorm.DB) error {
		for questionID, question := range byID {
			if question.Type != "mcq" {
				continue
			}
			answer, ok := answerByQuestion[questionID]
			if !ok {
				continue
			}
			correct := strings.EqualFold(strings.TrimSpace(answer.ChoiceKey), strings.TrimSpace(question.AnswerKey))
			answer.IsCorrect = &correct
			if correct {
				score++
			}
			if err := tx.Save(answer).Error; err != nil {
				return err
			}
		}
		now := time.Now()
		attempt.FinishedAt = &now
		attempt.Score = &score
		attempt.TotalMCQ = totalMCQ
		return tx.Save(attempt).Error
	})
}

func (a *App) finishAttempt(c *gin.Context) {
	attempt, ok := a.loadOwnAttempt(c)
	if !ok {
		return
	}
	a.finalizeAttempt(attempt)
	c.JSON(200, a.attemptSummaryPayload(attempt))
}

func (a *App) attemptSummaryPayload(attempt *PracticeAttempt) gin.H {
	questions, items := a.loadSetQuestions(attempt.SetID)
	var answers []PracticeAnswer
	a.DB.Where("attempt_id = ?", attempt.ID).Find(&answers)
	answerByQuestion := map[string]PracticeAnswer{}
	for _, answer := range answers {
		answerByQuestion[answer.QuestionID] = answer
	}
	byID := map[string]Question{}
	for _, q := range questions {
		byID[q.ID] = q
	}
	rows := make([]gin.H, 0, len(items))
	correctCount := 0
	for _, item := range items {
		q, exists := byID[item.QuestionID]
		if !exists {
			continue
		}
		row := stripAnswer(q)
		revealAnswer(row, q)
		if answer, answered := answerByQuestion[q.ID]; answered && q.Type == "mcq" && answer.IsCorrect != nil && *answer.IsCorrect {
			correctCount++
		}
		rows = append(rows, row)
	}
	return gin.H{
		"attempt":   attempt,
		"questions": rows,
		"answers":   answers,
		"correct":   correctCount,
	}
}

func (a *App) selfRateAnswer(c *gin.Context) {
	attempt, ok := a.loadOwnAttempt(c)
	if !ok {
		return
	}
	var req struct {
		Rating string `json:"rating"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Rating != "proficient" && req.Rating != "partial" && req.Rating != "weak") {
		c.JSON(400, gin.H{"error": "rating must be proficient, partial, or weak"})
		return
	}
	var answer PracticeAnswer
	if err := a.DB.Where("attempt_id = ? AND question_id = ?", attempt.ID, c.Param("qid")).First(&answer).Error; err != nil {
		c.JSON(404, gin.H{"error": "answer not found"})
		return
	}
	answer.SelfRating = req.Rating
	a.DB.Save(&answer)
	c.JSON(200, answer)
}

func (a *App) wrongBook(c *gin.Context) {
	userID := c.GetString("userID")
	limit := 100
	if parsed, err := strconv.Atoi(c.Query("limit")); err == nil && parsed > 0 && parsed <= 200 {
		limit = parsed
	}
	var answers []PracticeAnswer
	a.DB.Joins("join practice_attempts at on at.id = practice_answers.attempt_id").
		Where("at.user_id = ?", userID).
		Order("practice_answers.answered_at asc").
		Find(&answers)
	// Latest answer per question decides: a question leaves the book once it
	// is answered correctly (MCQ) or self-rated proficient (subjective).
	type entry struct {
		Question   Question  `json:"question"`
		Reason     string    `json:"reason"` // wrong | weak
		LastAt     time.Time `json:"lastAt"`
		WrongCount int       `json:"wrongCount"`
	}
	latest := map[string]entry{}
	counts := map[string]int{}
	questionIDs := map[string]bool{}
	for _, answer := range answers {
		questionIDs[answer.QuestionID] = true
	}
	var questions []Question
	if len(questionIDs) > 0 {
		ids := make([]string, 0, len(questionIDs))
		for id := range questionIDs {
			ids = append(ids, id)
		}
		a.DB.Preload("Tags").Where("id in ?", ids).Find(&questions)
	}
	byID := map[string]Question{}
	for _, q := range questions {
		byID[q.ID] = q
	}
	for _, answer := range answers {
		q, ok := byID[answer.QuestionID]
		if !ok {
			continue
		}
		reason := ""
		if q.Type == "mcq" && answer.ChoiceKey != "" && answer.IsCorrect != nil && !*answer.IsCorrect {
			reason = "wrong"
			counts[answer.QuestionID]++
		} else if q.Type == "subjective" && answer.SelfRating == "weak" {
			reason = "weak"
		}
		if reason == "" {
			delete(latest, answer.QuestionID)
			continue
		}
		latest[answer.QuestionID] = entry{Question: q, Reason: reason, LastAt: answer.AnsweredAt, WrongCount: counts[answer.QuestionID]}
	}
	out := make([]entry, 0, len(latest))
	for _, value := range latest {
		out = append(out, value)
	}
	// Most recent first.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].LastAt.After(out[i].LastAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if unitFilter := c.Query("unitId"); unitFilter != "" {
		filtered := make([]entry, 0, len(out))
		for _, item := range out {
			if item.Question.UnitID != nil && *item.Question.UnitID == unitFilter {
				filtered = append(filtered, item)
			}
		}
		out = filtered
	}
	if len(out) > limit {
		out = out[:limit]
	}
	c.JSON(200, out)
}

func (a *App) practiceStats(c *gin.Context) {
	userID := c.GetString("userID")
	type accuracyRow struct {
		Label    string  `json:"label"`
		ID       string  `json:"id"`
		Answered int     `json:"answered"`
		Correct  int     `json:"correct"`
		Accuracy float64 `json:"accuracy"`
	}
	build := func(rows []accuracyRow) []accuracyRow {
		for i := range rows {
			if rows[i].Answered > 0 {
				rows[i].Accuracy = float64(rows[i].Correct) / float64(rows[i].Answered)
			}
		}
		return rows
	}
	var byUnit []accuracyRow
	a.DB.Raw(`
		select coalesce(u.title, 'Unlinked') as label, coalesce(u.id, '') as id,
		       count(*) as answered,
		       sum(case when pa.is_correct then 1 else 0 end) as correct
		from practice_answers pa
		join practice_attempts at on at.id = pa.attempt_id and at.user_id = ?
		join questions q on q.id = pa.question_id
		left join units u on u.id = q.unit_id
		where pa.is_correct is not null
		group by u.id, u.title
		order by answered desc
	`, userID).Scan(&byUnit)
	var byTopic []accuracyRow
	a.DB.Raw(`
		select coalesce(t.title, 'Unlinked') as label, coalesce(t.id, '') as id,
		       count(*) as answered,
		       sum(case when pa.is_correct then 1 else 0 end) as correct
		from practice_answers pa
		join practice_attempts at on at.id = pa.attempt_id and at.user_id = ?
		join questions q on q.id = pa.question_id
		left join topics t on t.id = q.topic_id
		where pa.is_correct is not null
		group by t.id, t.title
		order by answered desc
		limit 20
	`, userID).Scan(&byTopic)
	var selfRatings []struct {
		Rating string `json:"rating"`
		Count  int    `json:"count"`
	}
	a.DB.Raw(`
		select pa.self_rating as rating, count(*) as count
		from practice_answers pa
		join practice_attempts at on at.id = pa.attempt_id and at.user_id = ?
		where pa.self_rating <> ''
		group by pa.self_rating
	`, userID).Scan(&selfRatings)
	var totals struct {
		Attempts int `json:"attempts"`
	}
	a.DB.Raw(`select count(*) as attempts from practice_attempts where user_id = ?`, userID).Scan(&totals)
	c.JSON(200, gin.H{
		"attempts":    totals.Attempts,
		"byUnit":      build(byUnit),
		"byTopic":     build(byTopic),
		"selfRatings": selfRatings,
	})
}
