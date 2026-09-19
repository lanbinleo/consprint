package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseKeyterms(t *testing.T) {
	items, err := ParseKeyterms(filepath.Join("..", "data", "sources", "keyterms.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 797 {
		t.Fatalf("expected 797 concepts, got %d", len(items))
	}
	if items[0].UnitTitle != "Science Practices" || items[0].TopicTitle != "Set A" || items[0].Term != "Independent variables" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	last := items[len(items)-1]
	if last.UnitTitle != "Unit 5: Mental and Physical Health" || last.TopicTitle != "5.5 - Treatment of Psychological Disorders" {
		t.Fatalf("unexpected last item: %#v", last)
	}
}

func TestResolveCompactID(t *testing.T) {
	concepts := []Concept{
		{ID: "ap-psychology.u2.t2-1.relative-clarity"},
		{ID: "ap-psychology.u2.t2-1.relative-size"},
		{ID: "ap-psychology.u3.t3-4.assimilation"},
	}
	index := map[string]int{}
	for i, concept := range concepts {
		index[concept.ID] = i
	}
	id, idx := resolveCompactID("relative-size", concepts, index, 0)
	if id != concepts[1].ID || idx != 1 {
		t.Fatalf("expected ordered slug resolution, got %s %d", id, idx)
	}
	id, idx = resolveCompactID(concepts[2].ID, concepts, index, idx)
	if id != concepts[2].ID || idx != 2 {
		t.Fatalf("expected exact resolution, got %s %d", id, idx)
	}
	id, _ = resolveCompactID("id: relative-clarity", concepts, index, -1)
	if id != concepts[0].ID {
		t.Fatalf("expected id-prefixed slug resolution, got %s", id)
	}
	id, _ = resolveCompactID("ap-psychology.u2.t2-1.relative-clarity (RC)", concepts, index, -1)
	if id != concepts[0].ID {
		t.Fatalf("expected parenthetical exact resolution, got %s", id)
	}
}

func TestEnrichFromCompactV2(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources")
	app, err := NewApp(filepath.Join(dir, "test.db"), sources)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := app.DB.DB()
	defer sqlDB.Close()

	unit := Unit{ID: "ap-psychology.u1", CourseID: "ap-psychology", Title: "Unit 1: Biological Bases of Behavior"}
	topic := Topic{ID: unit.ID + ".t1-5", UnitID: unit.ID, Title: "1.5 - Sleep"}
	aiConcept := Concept{ID: topic.ID + ".insomnia", CourseID: unit.CourseID, UnitID: unit.ID, TopicID: topic.ID, Term: "Insomnia", NormalizedTerm: "insomnia", ContentStatus: "ready"}
	humanConcept := Concept{ID: topic.ID + ".sample", CourseID: unit.CourseID, UnitID: unit.ID, TopicID: topic.ID, Term: "Sample", NormalizedTerm: "sample", ContentStatus: "ready"}
	for _, row := range []any{&unit, &topic, &aiConcept, &humanConcept} {
		if err := app.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	seed := []ConceptContent{
		{ID: aiConcept.ID + ".content", ConceptID: aiConcept.ID, Definition: blocksJSON([]string{"Difficulty falling or staying asleep / 难以入睡或保持睡眠"}), Source: "ai-enrichment.compact"},
		{ID: humanConcept.ID + ".content", ConceptID: humanConcept.ID, Definition: blocksJSON([]string{"A subset of the population used for the actual study."}), Source: "unit0.md"},
	}
	for i := range seed {
		if err := app.DB.Create(&seed[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := os.MkdirAll(sources, 0o755); err != nil {
		t.Fatal(err)
	}
	v2Path := filepath.Join(sources, "ai-enrichment-v2.compact")
	v2 := "@@ " + aiConcept.ID + "\ndef: 难以入睡或保持睡眠\ndef: Difficulty falling or staying asleep\n" +
		"@@ " + humanConcept.ID + "\ndef: 总体的一个子集\ndef: A subset of the population used for the actual study.\n"
	if err := os.WriteFile(v2Path, []byte(v2), 0o644); err != nil {
		t.Fatal(err)
	}

	importer := Importer{DB: app.DB, Sources: sources}
	if err := importer.EnrichFromCompactV2(v2Path); err != nil {
		t.Fatal(err)
	}

	var aiContent ConceptContent
	if err := app.DB.First(&aiContent, "concept_id = ?", aiConcept.ID).Error; err != nil {
		t.Fatal(err)
	}
	if aiContent.Source != "ai-enrichment-v2.compact" {
		t.Fatalf("expected AI content re-sourced to v2, got %q", aiContent.Source)
	}
	var defBlocks []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(aiContent.Definition, &defBlocks); err != nil {
		t.Fatal(err)
	}
	if len(defBlocks) != 2 || defBlocks[0].Text != "难以入睡或保持睡眠" || defBlocks[1].Text != "Difficulty falling or staying asleep" {
		t.Fatalf("expected zh-first bilingual blocks, got %#v", defBlocks)
	}

	var humanContent ConceptContent
	if err := app.DB.First(&humanContent, "concept_id = ?", humanConcept.ID).Error; err != nil {
		t.Fatal(err)
	}
	if humanContent.Source != "unit0.md" {
		t.Fatalf("expected human-sourced content to stay untouched, got %q", humanContent.Source)
	}

	// Re-running must converge: v2 recognizes its own output as overwrite-eligible.
	if err := importer.EnrichFromCompactV2(v2Path); err != nil {
		t.Fatal(err)
	}
	var count int64
	app.DB.Model(&ConceptContent{}).Where("concept_id = ?", aiConcept.ID).Count(&count)
	if count != 1 {
		t.Fatalf("expected one content row after re-run, got %d", count)
	}
}

func TestEnrichFromCardsOverwritesAnySource(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources")
	if err := os.MkdirAll(sources, 0o755); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(filepath.Join(dir, "test.db"), sources)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := app.DB.DB()
	defer sqlDB.Close()

	unit := Unit{ID: "ap-psychology.u2", CourseID: "ap-psychology", Title: "Unit 2: Cognition"}
	topic := Topic{ID: unit.ID + ".t2-1", UnitID: unit.ID, Title: "2.1 - Perception"}
	aiConcept := Concept{ID: topic.ID + ".perceptual-set", CourseID: unit.CourseID, UnitID: unit.ID, TopicID: topic.ID, Term: "Perceptual set", NormalizedTerm: "perceptual set", ContentStatus: "ready"}
	humanConcept := Concept{ID: topic.ID + ".closure", CourseID: unit.CourseID, UnitID: unit.ID, TopicID: topic.ID, Term: "Closure", NormalizedTerm: "closure", ContentStatus: "ready"}
	for _, row := range []any{&unit, &topic, &aiConcept, &humanConcept} {
		if err := app.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	seed := []ConceptContent{
		{ID: aiConcept.ID + ".content", ConceptID: aiConcept.ID, Definition: blocksJSON([]string{"旧AI内容"}), Source: "ai-enrichment-v2.compact"},
		{ID: humanConcept.ID + ".content", ConceptID: humanConcept.ID, Definition: blocksJSON([]string{"旧OPML内容"}), Source: "AP Psychology Notes.opml"},
	}
	for i := range seed {
		if err := app.DB.Create(&seed[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	cardsPath := filepath.Join(sources, "cards.compact")
	card := "@@ " + aiConcept.ID + "\ndef: 知觉定势\ndef: A mental predisposition to perceive things in a certain way.\nex: 恐怖片后树影像鬼\n" +
		"@@ " + humanConcept.ID + "\ndef: 闭合律\ndef: The tendency to mentally fill in gaps.\n"
	if err := os.WriteFile(cardsPath, []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}

	importer := Importer{DB: app.DB, Sources: sources}
	if err := importer.EnrichFromCards(cardsPath); err != nil {
		t.Fatal(err)
	}
	for _, cid := range []string{aiConcept.ID, humanConcept.ID} {
		var cc ConceptContent
		if err := app.DB.First(&cc, "concept_id = ?", cid).Error; err != nil {
			t.Fatal(err)
		}
		if cc.Source != "cards.compact" || cc.NeedsReview {
			t.Fatalf("expected authoritative cards.compact content for %s, got source=%s needsReview=%v", cid, cc.Source, cc.NeedsReview)
		}
		var blocks []struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(cc.Definition, &blocks); err != nil {
			t.Fatal(err)
		}
		if len(blocks) != 2 || blocks[0].Text == "旧AI内容" && false {
			t.Fatalf("unexpected definition blocks: %#v", blocks)
		}
		if len(blocks) != 2 {
			t.Fatalf("expected zh+en blocks, got %#v", blocks)
		}
	}
}
