package membershipkernel_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/membershipkernel"
)

func TestBillingQuoteCanonicalJSONMatchesJavaGoldenVector(t *testing.T) {
	quote := goldenQuote("")

	canonical, err := quote.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON() error = %v", err)
	}

	want := `{"subject_type":"BUYER","subject_id":"123456789","membership_id":"membership-1","membership_change_request_id":"change-1","offer_id":"offer-1","offer_version_no":"7","unit_amount_minor":19900,"currency":"TWD","interval_unit":"MONTH","interval_count":1,"provider_account_ref":"payuni-platform-sandbox","first_period_date":"2026-09-02T00:00:00Z","expires_at":"2026-09-02T00:10:00Z"}`
	if string(canonical) != want {
		t.Fatalf("canonical JSON = %s, want %s", canonical, want)
	}
	if got := quote.SnapshotHashHex(); got != "0894ef1b9d2196b7b72a38e9c52c1a797bd7147b0c4d764d9fa53843ad846eb2" {
		t.Fatalf("snapshot hash = %s", got)
	}
}

func TestBillingQuoteValidateChecksExpiryAndSnapshotHash(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	quote := goldenQuote("")
	quote.SnapshotHash = quote.SnapshotHashHex()
	if err := quote.Validate(now); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	quote.ExpiresAt = now.Add(-time.Minute)
	if err := quote.Validate(now); !errors.Is(err, membershipkernel.ErrInvalidBillingQuote) {
		t.Fatalf("expired Validate() error = %v, want ErrInvalidBillingQuote", err)
	}
}

func TestBillingQuoteValidateRejectsTamperedHash(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	quote := goldenQuote("0000000000000000000000000000000000000000000000000000000000000000")
	if err := quote.Validate(now); !errors.Is(err, membershipkernel.ErrInvalidBillingQuote) {
		t.Fatalf("tampered Validate() error = %v, want ErrInvalidBillingQuote", err)
	}
}

func TestBillingQuoteValidateRejectsUppercaseSnapshotHash(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	quote := goldenQuote("")
	quote.SnapshotHash = strings.ToUpper(quote.SnapshotHashHex())
	if err := quote.Validate(now); !errors.Is(err, membershipkernel.ErrInvalidBillingQuote) {
		t.Fatalf("uppercase Validate() error = %v, want ErrInvalidBillingQuote", err)
	}
}

func goldenQuote(hash string) membershipkernel.BillingQuote {
	subject, err := membershipkernel.NewSubjectRef(membershipkernel.SubjectTypeBuyer, "123456789")
	if err != nil {
		panic(err)
	}
	return membershipkernel.BillingQuote{
		QuoteID:                   "q-1",
		Subject:                   subject,
		MembershipID:              "membership-1",
		MembershipChangeRequestID: "change-1",
		OfferID:                   "offer-1",
		OfferVersionNo:            "7",
		UnitAmountMinor:           19900,
		Currency:                  "TWD",
		IntervalUnit:              "MONTH",
		IntervalCount:             1,
		ProviderAccountRef:        "payuni-platform-sandbox",
		FirstPeriodDate:           time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		ExpiresAt:                 time.Date(2026, 9, 2, 0, 10, 0, 0, time.UTC),
		SnapshotHash:              hash,
	}
}
