package backend

import (
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

func TestNextMastery(t *testing.T) {
	if got := nextMastery(0, "know"); got <= 0 || got >= 1 {
		t.Fatalf("know at 0 should increase modestly, got %f", got)
	}
	if got := nextMastery(4.9, "know"); got <= 4.9 || got > 5 {
		t.Fatalf("know at high mastery should still grow toward cap, got %f", got)
	}
	if got := nextMastery(0.05, "unknown"); got != 0 {
		t.Fatalf("unknown should clamp at 0, got %f", got)
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
