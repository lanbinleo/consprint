package backend

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Importer struct {
	DB      *gorm.DB
	Sources string
}

type parsedConcept struct {
	UnitTitle  string
	TopicTitle string
	Term       string
	Position   int
}

func (i Importer) RunAll() error {
	if err := i.ImportKeyterms(filepath.Join(i.Sources, "keyterms.md")); err != nil {
		return err
	}
	_ = i.EnrichFromBullets(filepath.Join(i.Sources, "unit0.md"), "unit0.md")
	_ = i.EnrichFromBullets(filepath.Join(i.Sources, "unit1.md"), "unit1.md")
	_ = i.EnrichFromOPML(filepath.Join(i.Sources, "AP-Psychology-Notes.opml"))
	_ = i.EnrichFromCompact(filepath.Join(i.Sources, "ai-enrichment.compact"))
	return nil
}

func (i Importer) ImportKeyterms(path string) error {
	items, err := ParseKeyterms(path)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errors.New("no concepts parsed from keyterms")
	}

	err = i.DB.Transaction(func(tx *gorm.DB) error {
		course := Course{ID: "ap-psychology", Title: "AP Psychology"}
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&course).Error; err != nil {
			return err
		}
		unitOrder := map[string]int{}
		topicOrder := map[string]int{}
		for _, item := range items {
			if _, ok := unitOrder[item.UnitTitle]; !ok {
				unitOrder[item.UnitTitle] = len(unitOrder)
			}
			topicKey := item.UnitTitle + "\x00" + item.TopicTitle
			if _, ok := topicOrder[topicKey]; !ok {
				topicOrder[topicKey] = len(topicOrder)
			}
			unitID := "ap-psychology." + UnitSlug(item.UnitTitle, unitOrder[item.UnitTitle])
			topicID := unitID + "." + TopicSlug(item.TopicTitle, topicOrder[topicKey])
			conceptID := topicID + "." + Slugify(item.Term)

			unit := Unit{ID: unitID, CourseID: course.ID, Title: item.UnitTitle, Position: unitOrder[item.UnitTitle]}
			topic := Topic{ID: topicID, UnitID: unitID, Title: item.TopicTitle, Position: topicOrder[topicKey]}
			concept := Concept{
				ID:             conceptID,
				CourseID:       course.ID,
				UnitID:         unitID,
				TopicID:        topicID,
				Term:           item.Term,
				NormalizedTerm: NormalizeTerm(item.Term),
				Position:       item.Position,
				ContentStatus:  "pending",
			}
			card := Card{ID: conceptID + ".recognition", ConceptID: conceptID, Type: "recognition", Prompt: item.Term, Back: ""}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&unit).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&topic).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{"term", "normalized_term", "unit_id", "topic_id", "position"}),
			}).Create(&concept).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&card).Error; err != nil {
				return err
			}
		}
		run := ImportRun{ID: NewID("imp"), Source: "keyterms.md", Status: "ok", Message: "Imported canonical concepts", Counts: fmt.Sprintf("concepts=%d", len(items))}
		return tx.Create(&run).Error
	})
	return err
}

func ParseKeyterms(path string) ([]parsedConcept, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	inBody := false
	currentUnit := ""
	currentTopic := ""
	pos := 0
	var out []parsedConcept
	unitRE := regexp.MustCompile(`^Unit\s+\d+:`)
	topicRE := regexp.MustCompile(`^\d+\.\d+\s+-\s+`)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if t == "" {
			continue
		}
		if strings.Contains(t, "Quizlet review for that set") {
			inBody = true
			currentUnit = "Science Practices"
			currentTopic = ""
			continue
		}
		if strings.Contains(t, "Quizlet review for that topic") {
			inBody = true
			continue
		}
		if !inBody {
			continue
		}
		switch {
		case strings.HasPrefix(t, "Science Practices"):
			currentUnit = "Science Practices"
			currentTopic = ""
		case unitRE.MatchString(t):
			currentUnit = t
			currentTopic = ""
		case strings.HasPrefix(t, "Set ") || topicRE.MatchString(t):
			currentTopic = t
			pos = 0
		case strings.HasPrefix(t, "●"):
			term := strings.TrimSpace(strings.TrimPrefix(t, "●"))
			if currentUnit != "" && currentTopic != "" && term != "" {
				out = append(out, parsedConcept{UnitTitle: currentUnit, TopicTitle: currentTopic, Term: term, Position: pos})
				pos++
			}
		}
	}
	return out, scanner.Err()
}

func (i Importer) EnrichFromBullets(path, source string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	entries := map[string][]string{}
	var current string
	for _, raw := range lines {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "") || strings.HasPrefix(t, "●") {
			current = strings.TrimSpace(strings.TrimLeft(t, "●"))
			entries[NormalizeTerm(current)] = nil
			continue
		}
		if strings.HasPrefix(t, "o") && len(t) > 1 && current != "" {
			note := strings.TrimSpace(strings.TrimPrefix(t, "o"))
			if note != "" {
				entries[NormalizeTerm(current)] = append(entries[NormalizeTerm(current)], note)
			}
		}
	}
	return i.applyNotes(entries, source, 0.78)
}

func (i Importer) EnrichFromOPML(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)
	var concepts []Concept
	if err := i.DB.Find(&concepts).Error; err != nil {
		return err
	}
	entries := map[string][]string{}
	for _, c := range concepts {
		re := regexp.MustCompile(`(?is)<outline[^>]+text="` + regexp.QuoteMeta(escapeXMLAttr(c.Term)) + `"[^>]*>(.*?)</outline>`)
		m := re.FindStringSubmatch(text)
		if len(m) < 2 {
			continue
		}
		childRE := regexp.MustCompile(`(?is)<outline[^>]+text="([^"]+)"`)
		for _, child := range childRE.FindAllStringSubmatch(m[1], 5) {
			n := htmlUnescape(child[1])
			if n != c.Term && len([]rune(n)) > 4 {
				entries[NormalizeTerm(c.Term)] = append(entries[NormalizeTerm(c.Term)], n)
			}
		}
	}
	return i.applyNotes(entries, "AP Psychology Notes.opml", 0.58)
}

func (i Importer) EnrichFromCompact(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	type entry struct {
		def   []string
		ex    []string
		pit   []string
		notes []string
	}
	entries := map[string]*entry{}
	currentID := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "@@") {
			currentID = strings.TrimSpace(strings.TrimPrefix(t, "@@"))
			if currentID != "" {
				entries[currentID] = &entry{}
			}
			continue
		}
		if currentID == "" {
			continue
		}
		key, value, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch key {
		case "def", "definition":
			entries[currentID].def = append(entries[currentID].def, value)
		case "ex", "example":
			entries[currentID].ex = append(entries[currentID].ex, value)
		case "pit", "pitfall":
			entries[currentID].pit = append(entries[currentID].pit, value)
		case "note":
			entries[currentID].notes = append(entries[currentID].notes, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	updated := 0
	for conceptID, e := range entries {
		var concept Concept
		if err := i.DB.First(&concept, "id = ?", conceptID).Error; err != nil {
			continue
		}
		if concept.ContentStatus == "ready" {
			continue
		}
		payload := ConceptContent{
			ID:          concept.ID + ".content",
			ConceptID:   concept.ID,
			Definition:  blocksJSON(e.def),
			Examples:    blocksJSON(e.ex),
			Pitfalls:    blocksJSON(e.pit),
			Notes:       blocksJSON(e.notes),
			Source:      "ai-enrichment.compact",
			Confidence:  0.66,
			NeedsReview: true,
		}
		if err := i.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&payload).Error; err != nil {
			return err
		}
		status := "partial"
		if len(e.def) > 0 {
			status = "ready"
		}
		if err := i.DB.Model(&Concept{}).Where("id = ?", concept.ID).Update("content_status", status).Error; err != nil {
			return err
		}
		updated++
	}
	run := ImportRun{ID: NewID("imp"), Source: "ai-enrichment.compact", Status: "ok", Message: "Imported compact AI enrichment", Counts: fmt.Sprintf("concepts=%d", updated)}
	return i.DB.Create(&run).Error
}

func (i Importer) applyNotes(entries map[string][]string, source string, confidence float64) error {
	if len(entries) == 0 {
		return nil
	}
	var concepts []Concept
	if err := i.DB.Find(&concepts).Error; err != nil {
		return err
	}
	byTerm := map[string]Concept{}
	for _, c := range concepts {
		byTerm[c.NormalizedTerm] = c
	}
	updated := 0
	for term, notes := range entries {
		concept, ok := byTerm[term]
		if !ok || len(notes) == 0 {
			continue
		}
		def, examples, pitfalls, extra := classifyNotes(notes)
		payload := ConceptContent{
			ID:          concept.ID + ".content",
			ConceptID:   concept.ID,
			Definition:  blocksJSON(def),
			Examples:    blocksJSON(examples),
			Pitfalls:    blocksJSON(pitfalls),
			Notes:       blocksJSON(extra),
			Source:      source,
			Confidence:  confidence,
			NeedsReview: confidence < 0.7,
		}
		if err := i.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&payload).Error; err != nil {
			return err
		}
		status := "partial"
		if len(def) > 0 {
			status = "ready"
		}
		if err := i.DB.Model(&Concept{}).Where("id = ?", concept.ID).Update("content_status", status).Error; err != nil {
			return err
		}
		updated++
	}
	run := ImportRun{ID: NewID("imp"), Source: source, Status: "ok", Message: "Enriched concepts from notes", Counts: fmt.Sprintf("concepts=%d", updated)}
	return i.DB.Create(&run).Error
}

func classifyNotes(notes []string) (def, examples, pitfalls, extra []string) {
	for _, n := range dedupe(notes) {
		l := strings.ToLower(n)
		switch {
		case strings.Contains(l, " vs.") || strings.Contains(l, "区别") || strings.Contains(l, "不要") || strings.Contains(l, "not ") || strings.Contains(l, "confus"):
			pitfalls = append(pitfalls, n)
		case strings.Contains(n, "比如") || strings.Contains(n, "例如") || strings.Contains(l, "example") || strings.Contains(n, "研究") || strings.Contains(n, "实验"):
			examples = append(examples, n)
		case len(def) < 2:
			def = append(def, n)
		default:
			extra = append(extra, n)
		}
	}
	return
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		k := NormalizeTerm(v)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func blocksJSON(lines []string) datatypes.JSON {
	if len(lines) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	type block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	var blocks []block
	for _, line := range lines {
		blocks = append(blocks, block{Type: "paragraph", Text: line})
	}
	b, _ := json.Marshal(blocks)
	return datatypes.JSON(b)
}

func htmlUnescape(s string) string {
	replacer := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'")
	return replacer.Replace(s)
}

func escapeXMLAttr(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(s)
}
