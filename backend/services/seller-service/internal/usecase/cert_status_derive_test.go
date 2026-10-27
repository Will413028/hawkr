package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestDeriveOverallStatus(t *testing.T) {
	cases := []struct {
		name       string
		certStatus string
		outcome    payuniOutcome
		want       string
	}{
		{"no application", "", "", domain.OverallCertNotStarted},
		{"repo COALESCE sentinel", "not_started", "", domain.OverallCertNotStarted},
		{"draft passes through", domain.CertStatusDraft.String(), "", domain.CertStatusDraft.String()},
		{"submitted passes through", domain.CertStatusSubmitted.String(), "", domain.CertStatusSubmitted.String()},
		{"forward_failed passes through", domain.CertStatusForwardFailed.String(), "", domain.CertStatusForwardFailed.String()},
		{"rejected（平台駁回）passes through", domain.CertStatusRejected.String(), "", domain.CertStatusRejected.String()},
		{"cancelled collapses to not_started", domain.CertStatusCancelled.String(), "", domain.OverallCertNotStarted},

		{"forwarded + PayUni 審核中", domain.CertStatusForwarded.String(), outcomeUnderReview, domain.CertStatusForwarded.String()},
		{"forwarded + PayUni 核准", domain.CertStatusForwarded.String(), outcomeApproved, overallStatusApproved},
		{"forwarded + PayUni 駁回", domain.CertStatusForwarded.String(), outcomeRejected, overallStatusRejected},
		{"forwarded + PayUni 要求補件", domain.CertStatusForwarded.String(), outcomePendingInfo, overallStatusPendingInfo},
		{"forwarded + 無法判定（fail closed）", domain.CertStatusForwarded.String(), "", domain.CertStatusForwarded.String()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveOverallStatus(tc.certStatus, tc.outcome); got != tc.want {
				t.Errorf("deriveOverallStatus(%q, %q) = %q, want %q", tc.certStatus, tc.outcome, got, tc.want)
			}
		})
	}
}

func TestDeriveCanResubmit(t *testing.T) {
	cases := []struct {
		name       string
		certStatus string
		outcome    payuniOutcome
		want       bool
	}{
		{"draft", domain.CertStatusDraft.String(), "", true},
		{"rejected（平台駁回）", domain.CertStatusRejected.String(), "", true},
		{"forward_failed 是 admin 重試 Forward，不是賣家重送", domain.CertStatusForwardFailed.String(), "", false},
		{"submitted 審核中不可改", domain.CertStatusSubmitted.String(), "", false},
		{"forwarded + PayUni 審核中不可改", domain.CertStatusForwarded.String(), outcomeUnderReview, false},
		{"forwarded + PayUni 核准不可改", domain.CertStatusForwarded.String(), outcomeApproved, false},
		{"forwarded + PayUni 要求補件（去 PayUni 補，不是改我方資料）", domain.CertStatusForwarded.String(), outcomePendingInfo, false},
		{"forwarded + PayUni 駁回可重送", domain.CertStatusForwarded.String(), outcomeRejected, true},
		{"cancelled", domain.CertStatusCancelled.String(), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveCanResubmit(tc.certStatus, tc.outcome); got != tc.want {
				t.Errorf("deriveCanResubmit(%q, %q) = %v, want %v", tc.certStatus, tc.outcome, got, tc.want)
			}
		})
	}
}

func TestDeriveApprovalStatus(t *testing.T) {
	cases := []struct {
		name       string
		certStatus string
		want       string
	}{
		{"no application", "", "unregistered"},
		{"repo COALESCE sentinel", "not_started", "unregistered"},
		{"draft", domain.CertStatusDraft.String(), "pending"},
		{"submitted", domain.CertStatusSubmitted.String(), "pending"},
		{"forwarded", domain.CertStatusForwarded.String(), "approved"},
		{"forward_failed", domain.CertStatusForwardFailed.String(), "approved"},
		{"rejected", domain.CertStatusRejected.String(), "rejected"},
		{"cancelled", domain.CertStatusCancelled.String(), "unregistered"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveApprovalStatus(tc.certStatus); got != tc.want {
				t.Errorf("deriveApprovalStatus(%q) = %q, want %q", tc.certStatus, got, tc.want)
			}
		})
	}
}

func TestExpandApprovalStatusFilter(t *testing.T) {
	cases := []struct {
		name           string
		approvalStatus string
		want           []string
	}{
		{"empty means no filter", "", nil},
		{"pending", "pending", []string{"draft", "submitted"}},
		{"approved", "approved", []string{"forwarded", "forward_failed"}},
		{"rejected", "rejected", []string{"rejected"}},
		{"unregistered", "unregistered", []string{"not_started", "cancelled"}},
		{"raw FSM status passes through", "forwarded", []string{"forwarded"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expandApprovalStatusFilter(tc.approvalStatus)
			if len(got) != len(tc.want) {
				t.Fatalf("expandApprovalStatusFilter(%q) = %v, want %v", tc.approvalStatus, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("expandApprovalStatusFilter(%q)[%d] = %q, want %q", tc.approvalStatus, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestExpandApprovalStatusFilterRoundTrip(t *testing.T) {
	statuses := []string{
		domain.CertStatusDraft.String(),
		domain.CertStatusSubmitted.String(),
		domain.CertStatusForwarded.String(),
		domain.CertStatusForwardFailed.String(),
		domain.CertStatusRejected.String(),
		domain.CertStatusCancelled.String(),
		domain.OverallCertNotStarted,
	}
	for _, s := range statuses {
		t.Run(s, func(t *testing.T) {
			expanded := expandApprovalStatusFilter(deriveApprovalStatus(s))
			for _, e := range expanded {
				if e == s {
					return
				}
			}
			t.Errorf("expandApprovalStatusFilter(deriveApprovalStatus(%q)) = %v, does not include %q", s, expanded, s)
		})
	}
}

func TestPayuniMerStatusToInt(t *testing.T) {
	cases := []struct {
		name      string
		merStatus string
		want      int
	}{
		{"empty", "", 0},
		{"1", "1", 1},
		{"91", "91", 91},
		{"unparsable", "not-a-number", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := payuniMerStatusToInt(tc.merStatus); got != tc.want {
				t.Errorf("payuniMerStatusToInt(%q) = %d, want %d", tc.merStatus, got, tc.want)
			}
		})
	}
}
