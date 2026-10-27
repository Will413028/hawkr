package model

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateRunesNeverSplitsACharacter(t *testing.T) {
	long := strings.Repeat("漢", 150)
	got := TruncateRunes(long, PreviewMaxRunes)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != PreviewMaxRunes {
		t.Fatalf("%d 個字元、合法 = %v", utf8.RuneCountInString(got), utf8.ValidString(got))
	}
	if TruncateRunes("短", 100) != "短" || TruncateRunes("", 5) != "" || TruncateRunes("abc", 0) != "" {
		t.Fatal("短的與空的不變")
	}
	if got := TruncateRunes("a😀b😀c", 3); got != "a😀b" {
		t.Fatalf("%q", got)
	}
}

func TestCallerIdentity(t *testing.T) {
	var c Caller
	if c.OwnsStore(1) || c.IsAuthor(Actor{Kind: ActorBuyer, ID: 1}) || c.IsAuthor(Actor{Kind: ActorStore, ID: 1}) {
		t.Fatal("沒有身分")
	}
	c.BuyerID = 7
	if !c.IsAuthor(Actor{Kind: ActorBuyer, ID: 7}) || c.IsAuthor(Actor{Kind: ActorStore, ID: 7}) || c.IsAuthor(Actor{Kind: "X", ID: 7}) {
		t.Fatal("buyer_id 與 store_id 是兩個命名空間，不可只比 id")
	}
}
