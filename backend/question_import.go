package backend

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"sync"
	"time"

	"gorm.io/datatypes"
)

// Question import: teachers and TAs upload CSV (hand-written in Excel) or
// JSON (produced by their own local workflows) files. Import is two-step:
// preview parses and validates everything without writing to the database,
// commit persists the reviewed rows.

type QuestionChoice struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

type QuestionMaterial struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// StimulusDocument is one document inside a Stimulus (AAQ article, EBQ
// source, MCQ passage). Text is markdown and may embed uploaded images.
type StimulusDocument struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type QuestionPart struct {
	Label           string   `json:"label"`
	Prompt          string   `json:"prompt"`
	ReferenceAnswer string   `json:"referenceAnswer"`
	Rubric          []string `json:"rubric"`
	// Points is the College Board point value of this part (display only).
	Points *int `json:"points"`
}

// questionDraft is the normalized shape produced by both import channels.
type questionDraft struct {
	Type        string             `json:"type"`
	Format      string             `json:"format"`
	Stem        string             `json:"stem"`
	StimulusID  string             `json:"stimulusId"`
	Materials   []QuestionMaterial `json:"materials"`
	Choices     []QuestionChoice   `json:"choices"`
	AnswerKey   string             `json:"answerKey"`
	Explanation string             `json:"explanation"`
	Parts       []QuestionPart     `json:"parts"`
	Unit        string             `json:"unit"`
	Topic       string             `json:"topic"`
	Tags        []string           `json:"tags"`
	Concepts    []string           `json:"concepts"`
	SourceNote  string             `json:"sourceNote"`
}

type importIssue struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
}

type importPreview struct {
	Total   int             `json:"total"`
	Valid   int             `json:"valid"`
	Channel string          `json:"channel"`
	Items   []questionDraft `json:"items"`
	Errors  []importIssue   `json:"errors"`
}

// newImportPreview initializes Items/Errors as empty slices so a fully valid
// (or fully invalid) file marshals to [] instead of null.
func newImportPreview(channel string, total int) importPreview {
	return importPreview{Channel: channel, Total: total, Items: []questionDraft{}, Errors: []importIssue{}}
}

var importSessions = struct {
	sync.Mutex
	m map[string]importSession
}{m: map[string]importSession{}}

type importSession struct {
	Channel   string
	Items     []questionDraft
	ExpiresAt time.Time
}

const importSessionTTL = 30 * time.Minute

func saveImportSession(channel string, items []questionDraft) string {
	id := NewID("qim")
	importSessions.Lock()
	now := time.Now()
	for key, session := range importSessions.m {
		if now.After(session.ExpiresAt) {
			delete(importSessions.m, key)
		}
	}
	importSessions.m[id] = importSession{Channel: channel, Items: items, ExpiresAt: now.Add(importSessionTTL)}
	importSessions.Unlock()
	return id
}

func loadImportSession(id string) (importSession, bool) {
	importSessions.Lock()
	defer importSessions.Unlock()
	session, ok := importSessions.m[id]
	if !ok || time.Now().After(session.ExpiresAt) {
		return importSession{}, false
	}
	return session, true
}

// parseQuestionFile dispatches by filename extension: .json for structured
// imports, everything else is treated as CSV.
func parseQuestionFile(header *multipart.FileHeader) (importPreview, error) {
	file, err := header.Open()
	if err != nil {
		return importPreview{}, err
	}
	defer file.Close()
	if header.Size > 8<<20 {
		return importPreview{}, errors.New("file too large (max 8MB)")
	}
	if strings.HasSuffix(strings.ToLower(header.Filename), ".json") {
		return parseQuestionJSON(file)
	}
	return parseQuestionCSV(file)
}

func parseQuestionJSON(r io.Reader) (importPreview, error) {
	body, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return importPreview{}, err
	}
	var drafts []questionDraft
	if err := json.Unmarshal(body, &drafts); err != nil {
		return importPreview{}, errors.New("invalid JSON: expected an array of questions")
	}
	preview := newImportPreview("json", len(drafts))
	for i, draft := range drafts {
		if err := validateQuestionDraft(&draft); err != nil {
			preview.Errors = append(preview.Errors, importIssue{Index: i, Message: err.Error()})
			continue
		}
		preview.Items = append(preview.Items, draft)
	}
	preview.Valid = len(preview.Items)
	return preview, nil
}

func parseQuestionCSV(r io.Reader) (importPreview, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return importPreview{}, fmt.Errorf("invalid CSV: %v", err)
	}
	if len(records) == 0 {
		return importPreview{}, errors.New("empty CSV file")
	}
	header := records[0]
	// Excel "CSV UTF-8" exports prefix the first header cell with a BOM
	// (U+FEFF), which strings.TrimSpace does not strip; without this the
	// "type" column is never found and subjective rows silently parse as MCQ.
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], string(rune(0xFEFF)))
	}
	columns := map[string]int{}
	for i, name := range header {
		columns[strings.ToLower(strings.TrimSpace(name))] = i
	}
	get := func(record []string, name string) string {
		index, ok := columns[name]
		if !ok || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	preview := newImportPreview("csv", len(records)-1)
	for i, record := range records[1:] {
		draft := questionDraft{
			Type:        strings.ToLower(get(record, "type")),
			Stem:        get(record, "stem"),
			Choices:     csvChoices(record, get),
			AnswerKey:   strings.ToUpper(get(record, "answer")),
			Explanation: get(record, "explanation"),
			Tags:        splitList(get(record, "tags"), ";"),
			Unit:        get(record, "unit"),
			Topic:       get(record, "topic"),
			SourceNote:  get(record, "source_note"),
		}
		if draft.Type == "" {
			draft.Type = "mcq"
		}
		if draft.Type == "subjective" {
			draft.Parts = []QuestionPart{{
				Prompt:          draft.Stem,
				ReferenceAnswer: get(record, "reference_answer"),
				Rubric:          splitList(get(record, "rubric"), ";"),
			}}
		}
		if materials := get(record, "materials"); materials != "" {
			draft.Materials = csvMaterials(materials)
		}
		if err := validateQuestionDraft(&draft); err != nil {
			preview.Errors = append(preview.Errors, importIssue{Index: i, Message: err.Error()})
			continue
		}
		preview.Items = append(preview.Items, draft)
	}
	preview.Valid = len(preview.Items)
	return preview, nil
}

func csvChoices(record []string, get func([]string, string) string) []QuestionChoice {
	var choices []QuestionChoice
	for _, key := range []string{"a", "b", "c", "d", "e"} {
		text := get(record, "choice_"+key)
		if text != "" {
			choices = append(choices, QuestionChoice{Key: strings.ToUpper(key), Text: text})
		}
	}
	return choices
}

func csvMaterials(raw string) []QuestionMaterial {
	var materials []QuestionMaterial
	for _, part := range splitList(raw, "|") {
		if part == "" {
			continue
		}
		if title, text, ok := strings.Cut(part, "::"); ok {
			materials = append(materials, QuestionMaterial{Title: strings.TrimSpace(title), Text: strings.TrimSpace(text)})
			continue
		}
		materials = append(materials, QuestionMaterial{Text: part})
	}
	return materials
}

func splitList(raw string, separator string) []string {
	var out []string
	for _, part := range strings.Split(raw, separator) {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func validateQuestionDraft(draft *questionDraft) error {
	draft.Stem = strings.TrimSpace(draft.Stem)
	if draft.Stem == "" {
		return errors.New("stem is required")
	}
	// Format only discriminates subjective layouts; MCQ clustering comes from
	// the shared stimulus reference instead.
	draft.Format = strings.ToLower(strings.TrimSpace(draft.Format))
	if draft.Type == "mcq" {
		draft.Format = ""
	} else if draft.Format == "" {
		draft.Format = QuestionFormatFRQ
	}
	if !validQuestionFormat(draft.Format) {
		return errors.New("format must be frq, aaq, or ebq")
	}
	if (draft.Format == QuestionFormatAAQ || draft.Format == QuestionFormatEBQ) && strings.TrimSpace(draft.StimulusID) == "" {
		return errors.New(draft.Format + " questions need a shared stimulus (article / sources)")
	}
	if (draft.Format == QuestionFormatAAQ || draft.Format == QuestionFormatEBQ) && len(draft.Parts) == 0 {
		return errors.New(draft.Format + " questions need per-part prompts (parts)")
	}
	switch draft.Type {
	case "mcq":
		if len(draft.Choices) < 2 {
			return errors.New("mcq needs at least two choices (choice_a, choice_b, ...)")
		}
		found := false
		for _, choice := range draft.Choices {
			if choice.Key == draft.AnswerKey {
				found = true
				break
			}
		}
		if !found {
			return errors.New("answer must match one of the choice keys (A-E)")
		}
	case "subjective":
		hasReference := false
		for _, part := range draft.Parts {
			if strings.TrimSpace(part.ReferenceAnswer) != "" || len(part.Rubric) > 0 {
				hasReference = true
				break
			}
		}
		if !hasReference {
			return errors.New("subjective questions need a reference answer or rubric")
		}
	default:
		return errors.New("type must be mcq or subjective")
	}
	if len(draft.Tags) > 12 {
		return errors.New("too many tags (max 12)")
	}
	return nil
}

// normalizeUnitRef expands shorthand labels ("1" -> "u1", "0"/"sp" ->
// science practices) so LIKE matching finds the right unit.
func normalizeUnitRef(ref string) string {
	ref = strings.ToLower(strings.TrimSpace(ref))
	switch ref {
	case "0", "sp", "science":
		return "science-practices"
	case "1", "2", "3", "4", "5":
		return "u" + ref
	}
	return ref
}

func (a *App) findOrCreateTags(names []string) []Tag {
	var tags []Tag
	for _, raw := range names {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		var tag Tag
		if err := a.DB.First(&tag, "name = ?", name).Error; err != nil {
			tag = Tag{ID: NewID("tag"), Name: name}
			if err := a.DB.Create(&tag).Error; err != nil {
				continue
			}
		}
		tags = append(tags, tag)
	}
	return tags
}

func mustJSON(v any) datatypes.JSON {
	if v == nil {
		return datatypes.JSON([]byte("[]"))
	}
	out, err := json.Marshal(v)
	if err != nil || string(out) == "null" {
		return datatypes.JSON([]byte("[]"))
	}
	return datatypes.JSON(out)
}

// createQuestionFromDraft persists a validated draft with resolved tags and
// unit/topic/concept links. Status starts as draft so staff can review imports.
func (a *App) createQuestionFromDraft(draft questionDraft, source, createdBy string) (*Question, error) {
	unitID, topicID := a.resolveUnitTopicLinked(normalizeUnitRef(draft.Unit), draft.Topic)
	stimulusID, err := a.resolveStimulusID(draft.StimulusID)
	if err != nil {
		return nil, err
	}
	concepts, err := a.resolveConcepts(draft.Concepts)
	if err != nil {
		return nil, err
	}
	question := Question{
		ID:          NewID("q"),
		Type:        draft.Type,
		Format:      draft.Format,
		Stem:        draft.Stem,
		StimulusID:  stimulusID,
		Materials:   mustJSON(draft.Materials),
		Choices:     mustJSON(draft.Choices),
		AnswerKey:   draft.AnswerKey,
		Explanation: strings.TrimSpace(draft.Explanation),
		Parts:       mustJSON(draft.Parts),
		UnitID:      unitID,
		TopicID:     topicID,
		Status:      "draft",
		Source:      source,
		SourceNote:  strings.TrimSpace(draft.SourceNote),
		CreatedBy:   createdBy,
		Tags:        a.findOrCreateTags(draft.Tags),
	}
	if err := a.DB.Create(&question).Error; err != nil {
		return nil, err
	}
	if len(concepts) > 0 {
		if err := a.DB.Model(&question).Association("Concepts").Append(concepts); err != nil {
			return nil, err
		}
	}
	return &question, nil
}

// resolveStimulusID validates the stimulus reference exists (when set) and
// returns the nullable column value.
func (a *App) resolveStimulusID(ref string) (*string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil
	}
	var count int64
	a.DB.Model(&Stimulus{}).Where("id = ?", ref).Count(&count)
	if count == 0 {
		return nil, errors.New("unknown stimulus: " + ref)
	}
	return &ref, nil
}

// resolveConcepts loads concept rows by id; every reference must exist so a
// typo'd import row fails loudly instead of silently losing a link.
func (a *App) resolveConcepts(ids []string) ([]Concept, error) {
	trimmed := make([]string, 0, len(ids))
	for _, id := range ids {
		if v := strings.TrimSpace(id); v != "" {
			trimmed = append(trimmed, v)
		}
	}
	if len(trimmed) == 0 {
		return nil, nil
	}
	var concepts []Concept
	a.DB.Where("id in ?", trimmed).Find(&concepts)
	if len(concepts) != len(trimmed) {
		found := map[string]bool{}
		for _, concept := range concepts {
			found[concept.ID] = true
		}
		missing := make([]string, 0)
		for _, id := range trimmed {
			if !found[id] {
				missing = append(missing, id)
			}
		}
		return nil, errors.New("unknown concept: " + strings.Join(missing, ", "))
	}
	return concepts, nil
}

func (a *App) resolveUnitTopicLinked(unitRef, topicRef string) (*string, *string) {
	var unitID, topicID *string
	if unitRef != "" {
		var unit Unit
		byRef := "%" + unitRef + "%"
		if err := a.DB.Where("lower(id) like ? or lower(title) like ?", byRef, byRef).Order("position asc").First(&unit).Error; err == nil {
			id := unit.ID
			unitID = &id
		}
	}
	if unitID != nil && strings.TrimSpace(topicRef) != "" {
		var topic Topic
		byRef := "%" + strings.ToLower(strings.TrimSpace(topicRef)) + "%"
		if err := a.DB.Where("unit_id = ? and (lower(id) like ? or lower(title) like ?)", *unitID, byRef, byRef).Order("position asc").First(&topic).Error; err == nil {
			id := topic.ID
			topicID = &id
		}
	}
	return unitID, topicID
}
