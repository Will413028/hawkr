package rpc

import (
	"strings"
	"testing"
	"uuid"
)

func TestParseIDAcceptsOnlyPositiveIntegersAndNamesTheField(t *testing.T) {
	if id, err := ParseID("store_id", "42"); err != nil || id != 42 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	for _, raw := range []string{"", "0", "-1", "abc", "1.5", " 1", "99999999999999999999"} {
		_, err := ParseID("store_id", raw)
		if err == nil || !strings.Contains(err.Error(), "store_id") {
			t.Fatalf("%q：err=%v，要指出 store_id", raw, err)
		}
	}
}

func TestParseUUID(t *testing.T) {
	want := uuid.NewV7()
	if got, err := ParseUUID("id", want.String()); err != nil || got != want {
		t.Fatalf("got=%v err=%v", got, err)
	}
	for _, raw := range []string{"", "x", "123"} {
		if _, err := ParseUUID("id", raw); err == nil {
			t.Fatalf("%q 被接受", raw)
		}
	}
}
