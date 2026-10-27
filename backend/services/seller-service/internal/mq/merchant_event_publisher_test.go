package mq

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestRoutingKeyForStatus(t *testing.T) {
	cases := []struct {
		status string
		want   string
	}{
		{"approved", routingKeyVerificationApproved},
		{"rejected", routingKeyVerificationRejected},
		{"pending_info", routingKeyVerificationPendingInfo},
		{"forwarded_to_payuni", routingKeyVerificationForwarded},
		// 未知 status 必須回空字串，不得 fallback 到真實 key（approved 文案最強）。
		{"forwarded", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := routingKeyForStatus(tc.status); got != tc.want {
			t.Errorf("routingKeyForStatus(%q) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

// 未知 status 必須完全跳過 publish：channel 為 nil，若 guard 被移除會 panic
// （guard 先於任何 channel 存取回傳 error）。
func TestPublishVerificationCompletedUnknownStatusSkips(t *testing.T) {
	p := &MerchantEventPublisher{logger: zap.NewNop()}
	err := p.PublishVerificationCompleted(context.Background(), domain.MerchantVerificationCompletedEvent{
		MerchantID: 1, Status: "no_such_status",
	})
	if err == nil {
		t.Error("expected an error for an unknown verification status")
	}
}

func TestRoutingKeyForStatus_UnderReview(t *testing.T) {
	if got := routingKeyForStatus("under_review"); got != routingKeyVerificationUnderReview {
		t.Errorf("routingKeyForStatus(%q) = %q, want %q", "under_review", got, routingKeyVerificationUnderReview)
	}
}

func TestRoutingKeyForStatus_UnknownStillEmpty(t *testing.T) {
	if got := routingKeyForStatus("some_future_status"); got != "" {
		t.Errorf("routingKeyForStatus(unknown) = %q, want empty string (caller must skip publish)", got)
	}
}
