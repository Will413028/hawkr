package domain

import "testing"

func TestGlobalForbiddenClaimCodes_NonEmpty(t *testing.T) {
	if len(GlobalForbiddenClaimCodes) == 0 {
		t.Fatal("GlobalForbiddenClaimCodes must not be empty")
	}
}

func TestClaimCode_KnownValuesAreDistinct(t *testing.T) {
	seen := map[ClaimCode]bool{}
	all := []ClaimCode{
		ClaimRequestFactRecorded, ClaimNextQuestionRequired, ClaimNextQuestionOptional,
		ClaimCorrectionApplied, ClaimInputRejected, ClaimExtractionDegraded,
		ClaimBasicsComplete, ClaimPriceReferenceAvailable, ClaimReconfirmationRequired,
		ClaimRfqPendingApproval,
		ClaimSearchStarted, ClaimSearchCompleted, ClaimRfqPublished,
		ClaimSellerNotified, ClaimDealCreated, ClaimOrderCreated, ClaimPaymentProcessed,
	}
	for _, c := range all {
		if seen[c] {
			t.Fatalf("duplicate ClaimCode value: %s", c)
		}
		seen[c] = true
	}
}
