package handler

import (
	"testing"

	socialpb "github.com/Will413028/hawkr/go-pkg/proto/social"
)

// The delegated detail carries the platform-hidden state like the delegated
// list does, so the workspace deep link can tell a hidden post from a live one.
func TestPostResource_CarriesHiddenState(t *testing.T) {
	hidden := postResource(&socialpb.PostDetailInfo{
		Id:           "1",
		HiddenAt:     "2026-10-01T08:00:00Z",
		HiddenReason: "違規",
	}).Payload.(DelegatedPostDetail)
	if hidden.HiddenAt == nil || *hidden.HiddenAt != "2026-10-01T08:00:00Z" {
		t.Fatalf("hiddenAt = %v, want 2026-10-01T08:00:00Z", hidden.HiddenAt)
	}
	if hidden.HiddenReason != "違規" {
		t.Fatalf("hiddenReason = %q, want 違規", hidden.HiddenReason)
	}

	live := postResource(&socialpb.PostDetailInfo{Id: "2"}).Payload.(DelegatedPostDetail)
	if live.HiddenAt != nil || live.HiddenReason != "" {
		t.Fatalf("a live post must have null hiddenAt and empty hiddenReason, got %v %q", live.HiddenAt, live.HiddenReason)
	}
}
