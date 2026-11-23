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

func TestParseUUIDRejectsNonCanonicalForms(t *testing.T) {
	id := uuid.NewV7()
	h := id.String()
	for _, raw := range []string{"urn:uuid:" + h, "{" + h + "}", strings.ReplaceAll(h, "-", ""), " " + h, ""} {
		_, err := ParseUUID("request_id", raw)
		if err == nil || !strings.Contains(err.Error(), "request_id") {
			t.Fatalf("%q：err=%v，要拒絕並指出 request_id", raw, err)
		}
	}
	if got, err := ParseUUID("request_id", strings.ToUpper(h)); err != nil || got != id {
		t.Fatalf("大寫的標準寫法 got=%v err=%v", got, err)
	}
}
