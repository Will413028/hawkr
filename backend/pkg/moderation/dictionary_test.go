package moderation_test

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/moderation"
)

func TestCheckAgainst_EmptyTextNoHit(t *testing.T) {
	hit, words := moderation.CheckAgainst("", []string{"bad"})
	if hit {
		t.Fatalf("expected no hit for empty text, got hit=%v words=%v", hit, words)
	}
}

func TestCheckAgainst_HitsExactWord(t *testing.T) {
	hit, words := moderation.CheckAgainst("you are bad", []string{"bad"})
	if !hit {
		t.Fatalf("expected hit, got hit=false")
	}
	if len(words) != 1 || words[0] != "bad" {
		t.Fatalf("expected words=[bad], got %v", words)
	}
}

func TestCheckAgainst_HitsSubstring(t *testing.T) {
	// dict has "fuck", text "fucking" should hit
	hit, words := moderation.CheckAgainst("fucking awful", []string{"fuck"})
	if !hit || len(words) != 1 || words[0] != "fuck" {
		t.Fatalf("expected hit=true words=[fuck], got hit=%v words=%v", hit, words)
	}
}

func TestCheckAgainst_CaseInsensitive(t *testing.T) {
	hit, _ := moderation.CheckAgainst("You Are BAD", []string{"bad"})
	if !hit {
		t.Fatal("expected hit for BAD when dict has bad (case-insensitive)")
	}
}

func TestCheckAgainst_ChineseHit(t *testing.T) {
	hit, words := moderation.CheckAgainst("你他媽的去死", []string{"他媽的"})
	if !hit || len(words) != 1 || words[0] != "他媽的" {
		t.Fatalf("expected Chinese substring hit, got hit=%v words=%v", hit, words)
	}
}

func TestCheckAgainst_MultipleHits(t *testing.T) {
	hit, words := moderation.CheckAgainst("fuck this shit", []string{"fuck", "shit", "damn"})
	if !hit {
		t.Fatal("expected hit")
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 hit words, got %d: %v", len(words), words)
	}
}

func TestCheckAgainst_EmptyWordList(t *testing.T) {
	hit, _ := moderation.CheckAgainst("anything goes", nil)
	if hit {
		t.Fatal("expected no hit when word list is nil")
	}
}

func TestCheckAgainst_SkipsEmptyWord(t *testing.T) {
	// empty string is a substring of anything; must be skipped to avoid false positives
	hit, _ := moderation.CheckAgainst("clean text", []string{""})
	if hit {
		t.Fatal("expected empty word in dict to be skipped, not a universal match")
	}
}

func TestCheck_HitsBaselineWord(t *testing.T) {
	hit, words := moderation.Check("你好白痴")
	if !hit {
		t.Fatal("expected Check(\"你好白痴\") to hit the baseline dictionary")
	}
	found := false
	for _, w := range words {
		if w == "白痴" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected hit words to contain \"白痴\", got %v", words)
	}
}

func TestCheck_NoHitOnCleanText(t *testing.T) {
	hit, words := moderation.Check("正常商品描述")
	if hit {
		t.Errorf("expected no hit for clean text, got hit=true words=%v", words)
	}
}
