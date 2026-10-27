package usecase

import (
	"strconv"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

const (
	overallStatusApproved    = "approved"
	overallStatusRejected    = "rejected"
	overallStatusPendingInfo = "pending_info"
)

func deriveOverallStatus(certStatus string, outcome payuniOutcome) string {
	if certStatus == "" || certStatus == domain.OverallCertNotStarted ||
		certStatus == domain.CertStatusCancelled.String() {
		return domain.OverallCertNotStarted
	}
	if certStatus != domain.CertStatusForwarded.String() {
		return certStatus
	}
	switch outcome {
	case outcomeApproved:
		return overallStatusApproved
	case outcomeRejected:
		return overallStatusRejected
	case outcomePendingInfo:
		return overallStatusPendingInfo
	default:
		return domain.CertStatusForwarded.String()
	}
}

func deriveCanResubmit(certStatus string, outcome payuniOutcome) bool {
	switch certStatus {
	case domain.CertStatusDraft.String(),
		domain.CertStatusRejected.String():
		return true
	case domain.CertStatusForwarded.String():
		return outcome == outcomeRejected
	default:
		return false
	}
}

func deriveApprovalStatus(certStatus string) string {
	switch certStatus {
	case domain.CertStatusRejected.String():
		return "rejected"
	case domain.CertStatusForwarded.String(), domain.CertStatusForwardFailed.String():
		return "approved" // admin gate passed
	case domain.CertStatusDraft.String(), domain.CertStatusSubmitted.String():
		return "pending"
	default:
		return "unregistered" // "", "not_started", "cancelled"
	}
}

func expandApprovalStatusFilter(approvalStatus string) []string {
	switch approvalStatus {
	case "":
		return nil
	case "pending":
		return []string{domain.CertStatusDraft.String(), domain.CertStatusSubmitted.String()}
	case "approved":
		return []string{domain.CertStatusForwarded.String(), domain.CertStatusForwardFailed.String()}
	case "rejected":
		return []string{domain.CertStatusRejected.String()}
	case "unregistered":
		return []string{domain.OverallCertNotStarted, domain.CertStatusCancelled.String()}
	default:
		return []string{approvalStatus}
	}
}

func payuniMerStatusToInt(merStatus string) int {
	if merStatus == "" {
		return 0
	}
	v, err := strconv.Atoi(merStatus)
	if err != nil {
		return 0
	}
	return v
}
