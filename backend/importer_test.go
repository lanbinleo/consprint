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

