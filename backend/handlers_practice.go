package backend

import (
	"encoding/json"
	"errors"
	"sort"
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
// answers, rubrics) from a question for student-facing payloads. Concepts are
// passed in as lite chips so full concept content never rides along.
func stripAnswer(q Question, concepts []conceptLite) gin.H {
	data, _ := json.Marshal(q)
	var out gin.H
	if err := json.Unmarshal(data, &out); err != nil {
		return gin.H{"id": q.ID, "stem": q.Stem}
	}
	delete(out, "answerKey")
	delete(out, "explanation")
	if concepts == nil {
		concepts = []conceptLite{}
	}
	out["concepts"] = concepts
	parts := questionParts(q.Parts)
	if parts != nil {
		redacted := make([]gin.H, 0, len(parts))
		for _, part := range parts {
			row := gin.H{"label": part.Label, "prompt": part.Prompt, "points": part.Points}
			redacted = append(redacted, row)
		}
		out["parts"] = redacted
	} else if len(q.Parts) > 0 {
		// Malformed stored parts: drop the key entirely rather than risk
		// passing the raw JSON through (it would carry reference answers
		// and rubrics).
		delete(out, "parts")
	}
	if out["tags"] == nil {
		out["tags"] = []Tag{}
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
				"points":          part.Points,
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
	ensureTags(questions)
	return questions, items
}

func (a *App) practiceSets(c *gin.Context) {
	userID := c.GetString("userID")
	var sets []PracticeSet
	a.DB.Where("status = ?", "published").Order("created_at desc").Find(&sets)
	setIDs := make([]string, 0, len(sets))
	for _, set := range sets {
		setIDs = append(setIDs, set.ID)
	}
	counts := a.setQuestionCounts(setIDs)
	unitIDs, formats := a.setFacets(setIDs)
	type attemptBrief struct {
		ID         string     `json:"id"`
		FinishedAt *time.Time `json:"finishedAt"`
		Score      *int       `json:"score"`
		TotalMCQ   int        `json:"totalMcq"`
	}
	type setRow struct {
		PracticeSet
		QuestionCount int            `json:"questionCount"`
		UnitIDs       []string       `json:"unitIds"`
		Formats       []string       `json:"formats"`
		Attempts      []attemptBrief `json:"attempts"`
		BestScore     *int           `json:"bestScore"`
	}
	out := make([]setRow, 0, len(sets))
	for _, set := range sets {
		var attempts []PracticeAttempt
		a.DB.Where("user_id = ? AND set_id = ?", userID, set.ID).Order("started_at asc").Find(&attempts)
		row := setRow{
			PracticeSet:   set,
			QuestionCount: counts[set.ID],
			UnitIDs:       unitIDs[set.ID],
			Formats:       formats[set.ID],
			Attempts:      make([]attemptBrief, 0),
		}
		if row.UnitIDs == nil {
			row.UnitIDs = []string{}
		}
		if row.Formats == nil {
			row.Formats = []string{}
		}
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
	attempts := make([]PracticeAttempt, 0)
	a.DB.Where("user_id = ? AND set_id = ?", userID, set.ID).Order("started_at desc").Find(&attempts)
	ids := make([]string, 0, len(questions))
	for _, q := range questions {
		ids = append(ids, q.ID)
	}
	links := a.conceptLinks(ids)
	stripped := make([]gin.H, 0, len(questions))
	byID := map[string]Question{}
	for _, q := range questions {
		byID[q.ID] = q
	}
	for _, item := range items {
		if q, ok := byID[item.QuestionID]; ok {
			stripped = append(stripped, stripAnswer(q, conceptChips(links, q.ID)))
		}
	}
	c.JSON(200, gin.H{
		"set":       set,
		"questions": stripped,
		"attempts":  attempts,
		"stimuli":   a.stimuliFor(questions),
		"coverage":  a.computeSetCoverage(questions),
	})
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

// attemptList is the student's cross-set 做题记录: own attempts newest first,
// with the set title, progress (answered / total questions), and score.
func (a *App) attemptList(c *gin.Context) {
	userID := c.GetString("userID")
	limit := 50
	if parsed, err := strconv.Atoi(c.Query("limit")); err == nil && parsed > 0 && parsed <= 200 {
		limit = parsed
	}
	attempts := make([]PracticeAttempt, 0)
	a.DB.Where("user_id = ?", userID).Order("started_at desc").Limit(limit).Find(&attempts)
	setIDs := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		setIDs = append(setIDs, attempt.SetID)
	}
	sets := make([]PracticeSet, 0)
	if len(setIDs) > 0 {
		a.DB.Where("id in ?", setIDs).Find(&sets)
	}
	setTitle := make(map[string]string, len(sets))
	for _, set := range sets {
		setTitle[set.ID] = set.Title
	}
	questionCounts := a.setQuestionCounts(setIDs)
	answeredCounts := map[string]int{}
	if len(attempts) > 0 {
		attemptIDs := make([]string, 0, len(attempts))
		for _, attempt := range attempts {
			attemptIDs = append(attemptIDs, attempt.ID)
		}
		rows := []struct {
			AttemptID string
			Count     int
		}{}
		a.DB.Raw(`select attempt_id as attempt_id, count(*) as count from practice_answers where attempt_id in (?) group by attempt_id`, attemptIDs).Scan(&rows)
		for _, row := range rows {
			answeredCounts[row.AttemptID] = row.Count
		}
	}
	type attemptRow struct {
		PracticeAttempt
		SetTitle      string `json:"setTitle"`
		QuestionCount int    `json:"questionCount"`
		AnsweredCount int    `json:"answeredCount"`
	}
	out := make([]attemptRow, 0, len(attempts))
	for _, attempt := range attempts {
		out = append(out, attemptRow{
			PracticeAttempt: attempt,
			SetTitle:        setTitle[attempt.SetID],
			QuestionCount:   questionCounts[attempt.SetID],
			AnsweredCount:   answeredCounts[attempt.ID],
		})
	}
	c.JSON(200, out)
}

func (a *App) attemptDetail(c *gin.Context) {
	attempt, ok := a.loadOwnAttempt(c)
	if !ok {
		return
	}
	var set PracticeSet
	a.DB.First(&set, "id = ?", attempt.SetID)
	questions, items := a.loadSetQuestions(attempt.SetID)
	answers := make([]PracticeAnswer, 0)
	a.DB.Where("attempt_id = ?", attempt.ID).Find(&answers)
	answerByQuestion := map[string]PracticeAnswer{}
	for _, answer := range answers {
		answerByQuestion[answer.QuestionID] = answer
	}
	ids := make([]string, 0, len(questions))
	byID := map[string]Question{}
	for _, q := range questions {
		byID[q.ID] = q
		ids = append(ids, q.ID)
	}
	links := a.conceptLinks(ids)
	finished := attempt.FinishedAt != nil
	type questionRow = gin.H
	rows := make([]questionRow, 0, len(items))
	for _, item := range items {
		q, exists := byID[item.QuestionID]
		if !exists {
			continue
		}
		row := stripAnswer(q, conceptChips(links, q.ID))
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
	c.JSON(200, gin.H{
		"attempt":   attempt,
		"set":       set,
		"questions": rows,
		"answers":   answers,
		"stimuli":   a.stimuliFor(questions),
	})
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
		Parts      []struct {
			Label string `json:"label"`
			Text  string `json:"text"`
		} `json:"parts"`
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
	perPart := question.Format == QuestionFormatAAQ || question.Format == QuestionFormatEBQ
	// partAnswers validates the submitted per-part responses against the
	// question's own part labels; nil means this answer is not per-part.
	var partAnswers []AnswerPart
	if question.Type == "mcq" {
		if strings.TrimSpace(req.ChoiceKey) == "" {
			c.JSON(400, gin.H{"error": "choiceKey is required for mcq"})
			return
		}
	} else if perPart {
		labels := map[string]bool{}
		seen := map[string]bool{}
		for _, part := range questionParts(question.Parts) {
			labels[part.Label] = true
		}
		if len(labels) == 0 {
			c.JSON(400, gin.H{"error": "question has no parts to answer"})
			return
		}
		if len(req.Parts) == 0 {
			c.JSON(400, gin.H{"error": "parts are required for " + question.Format + " questions"})
			return
		}
		partAnswers = make([]AnswerPart, 0, len(req.Parts))
		filled := false
		for _, input := range req.Parts {
			label := strings.TrimSpace(input.Label)
			if !labels[label] {
				c.JSON(400, gin.H{"error": "unknown part label: " + label})
				return
			}
			if seen[label] {
				c.JSON(400, gin.H{"error": "duplicate part label: " + label})
				return
			}
			seen[label] = true
			text := strings.TrimSpace(input.Text)
			if text != "" {
				filled = true
			}
			partAnswers = append(partAnswers, AnswerPart{Label: label, Text: text})
		}
		if !filled {
			c.JSON(400, gin.H{"error": "at least one part needs an answer"})
			return
		}
	} else if strings.TrimSpace(req.TextAnswer) == "" {
		c.JSON(400, gin.H{"error": "textAnswer is required for subjective questions"})
		return
	}
	now := time.Now()
	var answer PracticeAnswer
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("attempt_id = ? AND question_id = ?", attempt.ID, question.ID).First(&answer).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			answer = PracticeAnswer{ID: NewID("ans"), AttemptID: attempt.ID, QuestionID: question.ID}
		}
		if question.Type == "mcq" {
			answer.ChoiceKey = strings.ToUpper(strings.TrimSpace(req.ChoiceKey))
			answer.TextAnswer = ""
			answer.Parts = mustJSON(nil)
			if attempt.Mode == "instant" {
				correct := strings.EqualFold(answer.ChoiceKey, strings.TrimSpace(question.AnswerKey))
				answer.IsCorrect = &correct
			}
		} else if perPart {
			answer.Parts = mustJSON(partAnswers)
			answer.TextAnswer = ""
			answer.ChoiceKey = ""
		} else {
			answer.TextAnswer = strings.TrimSpace(req.TextAnswer)
			answer.ChoiceKey = ""
			answer.Parts = mustJSON(nil)
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
	row := stripAnswer(question, nil)
	revealAnswer(row, question)
	c.JSON(200, gin.H{"stored": true, "answer": answer, "question": row})
}

// finalizeAttempt grades every MCQ answer of the attempt and stamps the
// score. Idempotent. Returns the transaction error so callers can surface
// grading failures instead of reporting success with unsaved state.
func (a *App) finalizeAttempt(attempt *PracticeAttempt) error {
	if attempt.FinishedAt != nil {
		return nil
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
	return a.DB.Transaction(func(tx *gorm.DB) error {
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
	if err := a.finalizeAttempt(attempt); err != nil {
		c.JSON(500, gin.H{"error": "could not grade attempt"})
		return
	}
	c.JSON(200, a.attemptSummaryPayload(attempt))
}

func (a *App) attemptSummaryPayload(attempt *PracticeAttempt) gin.H {
	questions, items := a.loadSetQuestions(attempt.SetID)
	answers := make([]PracticeAnswer, 0)
	a.DB.Where("attempt_id = ?", attempt.ID).Find(&answers)
	answerByQuestion := map[string]PracticeAnswer{}
	for _, answer := range answers {
		answerByQuestion[answer.QuestionID] = answer
	}
	ids := make([]string, 0, len(questions))
	byID := map[string]Question{}
	for _, q := range questions {
		byID[q.ID] = q
		ids = append(ids, q.ID)
	}
	links := a.conceptLinks(ids)
	rows := make([]gin.H, 0, len(items))
	correctCount := 0
	for _, item := range items {
		q, exists := byID[item.QuestionID]
		if !exists {
			continue
		}
		row := stripAnswer(q, conceptChips(links, q.ID))
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
		"stimuli":   a.stimuliFor(questions),
		"correct":   correctCount,
	}
}

func validSelfRating(rating string) bool {
	return rating == "proficient" || rating == "partial" || rating == "weak"
}

func (a *App) selfRateAnswer(c *gin.Context) {
	attempt, ok := a.loadOwnAttempt(c)
	if !ok {
		return
	}
	var req struct {
		Rating string `json:"rating"`
		Parts  []struct {
			Label  string `json:"label"`
			Rating string `json:"rating"`
		} `json:"parts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	// Self-rating travels with the answer reveal: instant mode reveals right
	// after submitting, exam mode only once the attempt is finished. Without
	// this gate a student could rate an open exam answer "weak" and read the
	// reference answer from the wrong book before submitting.
	if attempt.FinishedAt == nil && attempt.Mode == "exam" {
		c.JSON(409, gin.H{"error": "self-rating is only available after the exam is submitted"})
		return
	}
	var answer PracticeAnswer
	if err := a.DB.Where("attempt_id = ? AND question_id = ?", attempt.ID, c.Param("qid")).First(&answer).Error; err != nil {
		c.JSON(404, gin.H{"error": "answer not found"})
		return
	}
	var question Question
	if err := a.DB.First(&question, "id = ?", answer.QuestionID).Error; err != nil || question.Type != "subjective" {
		c.JSON(400, gin.H{"error": "only subjective questions can be self-rated"})
		return
	}
	if len(req.Parts) > 0 {
		labels := map[string]bool{}
		for _, part := range questionParts(question.Parts) {
			labels[part.Label] = true
		}
		ratings := make([]AnswerPartRating, 0, len(req.Parts))
		seen := map[string]bool{}
		allProficient := true
		anyWeak := false
		for _, input := range req.Parts {
			label := strings.TrimSpace(input.Label)
			if !labels[label] {
				c.JSON(400, gin.H{"error": "unknown part label: " + label})
				return
			}
			if seen[label] {
				c.JSON(400, gin.H{"error": "duplicate part label: " + label})
				return
			}
			seen[label] = true
			if !validSelfRating(input.Rating) {
				c.JSON(400, gin.H{"error": "rating must be proficient, partial, or weak"})
				return
			}
			ratings = append(ratings, AnswerPartRating{Label: label, Rating: input.Rating})
			if input.Rating == "weak" {
				anyWeak = true
			}
			if input.Rating != "proficient" {
				allProficient = false
			}
		}
		// Aggregate mirrors the wrong-book verdict rules: any weak stays in
		// the book, only all-proficient leaves it.
		aggregate := "partial"
		if anyWeak {
			aggregate = "weak"
		} else if allProficient {
			aggregate = "proficient"
		}
		answer.PartRatings = mustJSON(ratings)
		answer.SelfRating = aggregate
	} else {
		if !validSelfRating(req.Rating) {
			c.JSON(400, gin.H{"error": "rating must be proficient, partial, or weak"})
			return
		}
		answer.SelfRating = req.Rating
		answer.PartRatings = mustJSON(nil)
	}
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
		Order("practice_answers.answered_at asc, practice_answers.id asc").
		Find(&answers)
	// The latest answer per question decides. A question leaves the book only
	// on an affirmative good outcome: answered correctly (graded MCQ) or
	// self-rated proficient (subjective, after reveal). Answers without a
	// verdict yet — an exam attempt still open (MCQ ungraded, subjective
	// unrated) — leave the question's previous state untouched instead of
	// silently evicting it.
	attempts := make([]PracticeAttempt, 0)
	a.DB.Where("user_id = ?", userID).Find(&attempts)
	attemptByID := make(map[string]PracticeAttempt, len(attempts))
	for _, attempt := range attempts {
		attemptByID[attempt.ID] = attempt
	}
	type entry struct {
		QuestionID string    `json:"-"`
		Question   gin.H     `json:"question"`
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
	questions := make([]Question, 0)
	if len(questionIDs) > 0 {
		ids := make([]string, 0, len(questionIDs))
		for id := range questionIDs {
			ids = append(ids, id)
		}
		a.DB.Preload("Tags").Where("id in ?", ids).Find(&questions)
	}
	ensureTags(questions)
	links := a.conceptLinks(idsOf(questions))
	byID := map[string]Question{}
	unitByQuestion := map[string]string{}
	for _, q := range questions {
		byID[q.ID] = q
		if q.UnitID != nil {
			unitByQuestion[q.ID] = *q.UnitID
		}
	}
	for _, answer := range answers {
		q, ok := byID[answer.QuestionID]
		if !ok {
			continue
		}
		attempt, hasAttempt := attemptByID[answer.AttemptID]
		// Grading material counts as revealed only once the owning attempt
		// showed feedback: instantly in instant mode, at finish in exam mode.
		revealed := hasAttempt && (attempt.Mode != "exam" || attempt.FinishedAt != nil)
		reason := ""
		switch {
		case q.Type == "mcq":
			if answer.IsCorrect == nil {
				continue // not graded yet — no verdict, keep prior state
			}
			if !*answer.IsCorrect {
				reason = "wrong"
				counts[answer.QuestionID]++
			} else {
				delete(latest, answer.QuestionID)
				continue
			}
		case q.Type == "subjective":
			switch answer.SelfRating {
			case "weak":
				if !revealed {
					continue // exam answer not revealed yet
				}
				reason = "weak"
			case "proficient":
				delete(latest, answer.QuestionID)
				continue
			default:
				// partial or not yet rated — no verdict, keep prior state
				continue
			}
		default:
			continue
		}
		// The student has already answered these questions, so grading
		// material is revealed — but only through the explicit reveal path,
		// never by embedding the raw Question model.
		view := stripAnswer(q, conceptChips(links, q.ID))
		revealAnswer(view, q)
		latest[answer.QuestionID] = entry{QuestionID: answer.QuestionID, Question: view, Reason: reason, LastAt: answer.AnsweredAt, WrongCount: counts[answer.QuestionID]}
	}
	out := make([]entry, 0, len(latest))
	for _, value := range latest {
		out = append(out, value)
	}
	// Most recent first.
	sort.Slice(out, func(i, j int) bool { return out[i].LastAt.After(out[j].LastAt) })
	if unitFilter := c.Query("unitId"); unitFilter != "" {
		filtered := make([]entry, 0, len(out))
		for _, item := range out {
			if unitByQuestion[item.QuestionID] == unitFilter {
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
	byUnit := make([]accuracyRow, 0)
	a.DB.Raw(`
		select coalesce(u.title, 'Unlinked') as label, coalesce(u.id, '') as id,
		       count(*) as answered,
		       sum(case when pa.is_correct then 1 else 0 end) as correct
		from practice_answers pa
		join practice_attempts at on at.id = pa.attempt_id and at.user_id = ?
		join questions q on q.id = pa.question_id
		left join units u on u.id = q.unit_id
		where pa.is_correct is not null and pa.rowid = (
			select p2.rowid from practice_answers p2
			join practice_attempts a2 on a2.id = p2.attempt_id and a2.user_id = at.user_id
			where p2.question_id = pa.question_id
			order by p2.answered_at desc, p2.id desc
			limit 1
		)
		group by u.id, u.title
		order by answered desc
	`, userID).Scan(&byUnit)
	byTopic := make([]accuracyRow, 0)
	a.DB.Raw(`
		select coalesce(t.title, 'Unlinked') as label, coalesce(t.id, '') as id,
		       count(*) as answered,
		       sum(case when pa.is_correct then 1 else 0 end) as correct
		from practice_answers pa
		join practice_attempts at on at.id = pa.attempt_id and at.user_id = ?
		join questions q on q.id = pa.question_id
		left join topics t on t.id = q.topic_id
		where pa.is_correct is not null and pa.rowid = (
			select p2.rowid from practice_answers p2
			join practice_attempts a2 on a2.id = p2.attempt_id and a2.user_id = at.user_id
			where p2.question_id = pa.question_id
			order by p2.answered_at desc, p2.id desc
			limit 1
		)
		group by t.id, t.title
		order by answered desc
		limit 20
	`, userID).Scan(&byTopic)
	selfRatings := make([]struct {
		Rating string `json:"rating"`
		Count  int    `json:"count"`
	}, 0)
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
