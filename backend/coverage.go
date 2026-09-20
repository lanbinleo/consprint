package backend

import (
	"encoding/json"
	"sort"

	"gorm.io/datatypes"
)

// Set coverage and question linkage enrichment: which units, topics, and
// concepts a practice set touches, derived live from its questions so the
// per-question links stay the single source of truth.

type conceptLite struct {
	ID   string `json:"id"`
	Term string `json:"term"`
}

// idsOf collects question ids in sequence order.
func idsOf(questions []Question) []string {
	ids := make([]string, 0, len(questions))
	for _, q := range questions {
		ids = append(ids, q.ID)
	}
	return ids
}

// conceptLinks loads {id, term} chips for many questions in one grouped
// query; full concept content never rides along with question payloads.
func (a *App) conceptLinks(questionIDs []string) map[string][]conceptLite {
	out := map[string][]conceptLite{}
	if len(questionIDs) == 0 {
		return out
	}
	rows := []struct {
		QuestionID string
		ID         string
		Term       string
	}{}
	a.DB.Raw(`
		select qc.question_id as question_id, c.id as id, c.term as term
		from question_concepts qc
		join concepts c on c.id = qc.concept_id
		where qc.question_id in (?)
		order by c.unit_id, c.position
	`, questionIDs).Scan(&rows)
	for _, row := range rows {
		out[row.QuestionID] = append(out[row.QuestionID], conceptLite{ID: row.ID, Term: row.Term})
	}
	return out
}

// conceptChips returns the lite chips of one question (never nil).
func conceptChips(links map[string][]conceptLite, questionID string) []conceptLite {
	if chips, ok := links[questionID]; ok && chips != nil {
		return chips
	}
	return []conceptLite{}
}

type coverageBucket struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Count int    `json:"count"`
}

type coverageConceptBucket struct {
	ID    string `json:"id"`
	Term  string `json:"term"`
	Count int    `json:"count"`
}

type setCoverage struct {
	Units    []coverageBucket        `json:"units"`
	Topics   []coverageBucket        `json:"topics"`
	Concepts []coverageConceptBucket `json:"concepts"`
	Unlinked int                     `json:"unlinked"`
}

// computeSetCoverage rolls a set's questions up into unit/topic/concept
// buckets. A question with no unit, no topic, and no concepts counts as
// unlinked so the assembly editor can flag coverage gaps.
func (a *App) computeSetCoverage(questions []Question) setCoverage {
	coverage := setCoverage{
		Units:    []coverageBucket{},
		Topics:   []coverageBucket{},
		Concepts: []coverageConceptBucket{},
	}
	ids := make([]string, 0, len(questions))
	for _, q := range questions {
		ids = append(ids, q.ID)
	}
	links := a.conceptLinks(ids)

	unitTitles := map[string]string{}
	unitPosition := map[string]int{}
	var units []Unit
	a.DB.Select("id, title, position").Order("position asc").Find(&units)
	for _, unit := range units {
		unitTitles[unit.ID] = unit.Title
		unitPosition[unit.ID] = unit.Position
	}
	topicTitles := map[string]string{}
	var topics []Topic
	a.DB.Select("id, title").Find(&topics)
	for _, topic := range topics {
		topicTitles[topic.ID] = topic.Title
	}

	unitsByID := map[string]*coverageBucket{}
	topicsByID := map[string]*coverageBucket{}
	conceptsByID := map[string]*coverageConceptBucket{}
	for _, q := range questions {
		chips := links[q.ID]
		if q.UnitID == nil && q.TopicID == nil && len(chips) == 0 {
			coverage.Unlinked++
			continue
		}
		if q.UnitID != nil {
			id := *q.UnitID
			if unitsByID[id] == nil {
				unitsByID[id] = &coverageBucket{ID: id, Title: unitTitles[id]}
			}
			unitsByID[id].Count++
		}
		if q.TopicID != nil {
			id := *q.TopicID
			if topicsByID[id] == nil {
				topicsByID[id] = &coverageBucket{ID: id, Title: topicTitles[id]}
			}
			topicsByID[id].Count++
		}
		for _, chip := range chips {
			if conceptsByID[chip.ID] == nil {
				conceptsByID[chip.ID] = &coverageConceptBucket{ID: chip.ID, Term: chip.Term}
			}
			conceptsByID[chip.ID].Count++
		}
	}
	for _, bucket := range unitsByID {
		coverage.Units = append(coverage.Units, *bucket)
	}
	sort.Slice(coverage.Units, func(i, j int) bool {
		if coverage.Units[i].Count != coverage.Units[j].Count {
			return coverage.Units[i].Count > coverage.Units[j].Count
		}
		return unitPosition[coverage.Units[i].ID] < unitPosition[coverage.Units[j].ID]
	})
	for _, bucket := range topicsByID {
		coverage.Topics = append(coverage.Topics, *bucket)
	}
	sort.Slice(coverage.Topics, func(i, j int) bool {
		if coverage.Topics[i].Count != coverage.Topics[j].Count {
			return coverage.Topics[i].Count > coverage.Topics[j].Count
		}
		return coverage.Topics[i].Title < coverage.Topics[j].Title
	})
	for _, bucket := range conceptsByID {
		coverage.Concepts = append(coverage.Concepts, *bucket)
	}
	sort.Slice(coverage.Concepts, func(i, j int) bool {
		if coverage.Concepts[i].Count != coverage.Concepts[j].Count {
			return coverage.Concepts[i].Count > coverage.Concepts[j].Count
		}
		return coverage.Concepts[i].Term < coverage.Concepts[j].Term
	})
	return coverage
}

// stimulusLite is the student/admin view of a shared stimulus. Documents are
// reading material, never grading content, so they are always safe to send.
type stimulusLite struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Kind      string             `json:"kind"`
	Documents []StimulusDocument `json:"documents"`
}

// stimuliFor loads the distinct stimuli referenced by the given questions.
func (a *App) stimuliFor(questions []Question) []stimulusLite {
	seen := map[string]bool{}
	ids := make([]string, 0)
	for _, q := range questions {
		if q.StimulusID != nil && !seen[*q.StimulusID] {
			seen[*q.StimulusID] = true
			ids = append(ids, *q.StimulusID)
		}
	}
	out := make([]stimulusLite, 0, len(ids))
	if len(ids) == 0 {
		return out
	}
	var rows []Stimulus
	a.DB.Where("id in (?)", ids).Find(&rows)
	byID := make(map[string]Stimulus, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	// Keep the order the stimuli first appear in the question sequence.
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			continue
		}
		docs := stimulusDocuments(row.Documents)
		if docs == nil {
			docs = []StimulusDocument{}
		}
		out = append(out, stimulusLite{ID: row.ID, Title: row.Title, Kind: row.Kind, Documents: docs})
	}
	return out
}

// setFacets computes, for many sets at once, the covered unit ids (ordered
// by unit position) and the question formats present (mcq | frq | aaq | ebq)
// — one grouped query instead of per-set counts.
func (a *App) setFacets(setIDs []string) (map[string][]string, map[string][]string) {
	unitIDs := map[string][]string{}
	formatList := map[string][]string{}
	if len(setIDs) == 0 {
		return unitIDs, formatList
	}
	rows := []struct {
		SetID  string
		UnitID *string
		Format string
	}{}
	a.DB.Raw(`
		select psi.set_id as set_id, q.unit_id as unit_id,
			case when q.type = 'mcq' then 'mcq' when q.format <> '' then q.format else 'frq' end as format
		from practice_set_items psi
		join questions q on q.id = psi.question_id
		where psi.set_id in (?)
		group by psi.set_id, q.unit_id, format
	`, setIDs).Scan(&rows)
	unitSeen := map[string]map[string]bool{}
	formatSeen := map[string]map[string]bool{}
	for _, row := range rows {
		if unitSeen[row.SetID] == nil {
			unitSeen[row.SetID] = map[string]bool{}
		}
		if row.UnitID != nil {
			unitSeen[row.SetID][*row.UnitID] = true
		}
		if formatSeen[row.SetID] == nil {
			formatSeen[row.SetID] = map[string]bool{}
		}
		formatSeen[row.SetID][row.Format] = true
	}
	unitPosition := map[string]int{}
	var units []Unit
	a.DB.Select("id, position").Order("position asc").Find(&units)
	for _, unit := range units {
		unitPosition[unit.ID] = unit.Position
	}
	for setID, seen := range unitSeen {
		ids := make([]string, 0, len(seen))
		for id := range seen {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return unitPosition[ids[i]] < unitPosition[ids[j]] })
		if ids == nil {
			ids = []string{}
		}
		unitIDs[setID] = ids
	}
	canonical := []string{"mcq", "frq", "aaq", "ebq"}
	rank := map[string]int{}
	for i, format := range canonical {
		rank[format] = i
	}
	for setID, seen := range formatSeen {
		formats := make([]string, 0, len(seen))
		for format := range seen {
			formats = append(formats, format)
		}
		sort.Slice(formats, func(i, j int) bool { return rank[formats[i]] < rank[formats[j]] })
		formatList[setID] = formats
	}
	return unitIDs, formatList
}

// setQuestionCounts counts items per set in one grouped query.
func (a *App) setQuestionCounts(setIDs []string) map[string]int {
	counts := map[string]int{}
	if len(setIDs) == 0 {
		return counts
	}
	rows := []struct {
		SetID string
		Count int
	}{}
	a.DB.Raw(`select set_id as set_id, count(*) as count from practice_set_items where set_id in (?) group by set_id`, setIDs).Scan(&rows)
	for _, row := range rows {
		counts[row.SetID] = row.Count
	}
	return counts
}

// answerPart / answerPartRating decode helpers for PracticeAnswer JSON
// columns.
func answerParts(raw datatypes.JSON) []AnswerPart {
	var parts []AnswerPart
	if len(raw) == 0 {
		return parts
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil
	}
	return parts
}

func answerPartRatings(raw datatypes.JSON) []AnswerPartRating {
	var ratings []AnswerPartRating
	if len(raw) == 0 {
		return ratings
	}
	if err := json.Unmarshal(raw, &ratings); err != nil {
		return nil
	}
	return ratings
}
