package idempotency_test

import (
	"encoding/hex"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/idempotency"
)

func TestHashEncodingIsStable(t *testing.T) {
	got := hex.EncodeToString(idempotency.Hash("comment", "abc", "-", "內文"))
	const want = "e8c314f18a010e9ffc650b8adea57ad309442fdd900b6b93d182ddc7c449a1c6"
	if got != want {
		t.Fatalf("hash = %s, want %s", got, want)
	}
}

func TestHasherMatchesHash(t *testing.T) {
	b := idempotency.New()
	b.Put("a")
	b.Put("b", "c")
	if hex.EncodeToString(b.Sum()) != hex.EncodeToString(idempotency.Hash("a", "b", "c")) {
		t.Fatal("incremental Put differs from Hash")
	}
}

func TestLengthPrefixPreventsConcatenationCollision(t *testing.T) {
	if hex.EncodeToString(idempotency.Hash("ab", "c")) == hex.EncodeToString(idempotency.Hash("a", "bc")) {
		t.Fatal("collision")
	}
}
