package domain

import (
	"encoding/json"
	"testing"
)

// TestPayuniMerStatus_AllOfficialCodes pins every MerStatus code PAYUNi's
// search_merchant_status can return to its display string. Code 8 (拒絕) was
// missing from the enum while the outcome-derivation engine
// (usecase/payuni.go) already treated it as a terminal rejection, so a real
// 8 rendered as the literal Go type name "PayuniMerStatus(8)" on the seller
// profile and the admin merchant list.
func TestPayuniMerStatus_AllOfficialCodes(t *testing.T) {
	cases := []struct {
		status PayuniMerStatus
		want   string
	}{
		{PayuniMerStatusUnregistered, "Unregistered"},
		{PayuniMerStatusActive, "Active"},
		{PayuniMerStatusClosed, "Closed"},
		{PayuniMerStatusDenied, "Denied"},
		{PayuniMerStatusUnderReview, "UnderReview"},
		{PayuniMerStatusPendingInfo, "PendingInfo"},
		{PayuniMerStatusRejected, "Rejected"},
		{PayuniMerStatusOtherClosed, "OtherClosed"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.status.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
			if !tc.status.IsAPayuniMerStatus() {
				t.Errorf("IsAPayuniMerStatus() = false, want true")
			}
			parsed, err := PayuniMerStatusString(tc.want)
			if err != nil {
				t.Fatalf("PayuniMerStatusString(%q) error: %v", tc.want, err)
			}
			if parsed != tc.status {
				t.Errorf("round-trip = %d, want %d", parsed, tc.status)
			}
		})
	}
}

// TestPayuniMerStatus_DeniedIsCode8 guards the numeric value itself — the
// display string is only correct if it is bound to the code PAYUNi actually
// sends (see commit 922c76c8f, which added `case "8"` to the derivation
// engine per https://docs.payuni.com.tw/web/#/9/138).
func TestPayuniMerStatus_DeniedIsCode8(t *testing.T) {
	if PayuniMerStatusDenied != 8 {
		t.Errorf("PayuniMerStatusDenied = %d, want 8", PayuniMerStatusDenied)
	}
}

func TestPayuniMerStatus_JSONRoundTrip(t *testing.T) {
	for _, status := range PayuniMerStatusValues() {
		raw, err := json.Marshal(status)
		if err != nil {
			t.Fatalf("Marshal(%d) error: %v", status, err)
		}
		var back PayuniMerStatus
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("Unmarshal(%s) error: %v", raw, err)
		}
		if back != status {
			t.Errorf("JSON round-trip %d -> %s -> %d", status, raw, back)
		}
	}
}

// TestPayuniMerStatus_UnknownCodeStaysUnknown keeps the fail-closed behavior
// for codes PAYUNi has not documented: they must NOT silently collapse onto a
// real status.
func TestPayuniMerStatus_UnknownCodeStaysUnknown(t *testing.T) {
	unknown := PayuniMerStatus(7)
	if unknown.IsAPayuniMerStatus() {
		t.Errorf("IsAPayuniMerStatus() = true for undocumented code 7")
	}
	if got, want := unknown.String(), "PayuniMerStatus(7)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if _, err := PayuniMerStatusString("NotAStatus"); err == nil {
		t.Errorf("PayuniMerStatusString(\"NotAStatus\") should error")
	}
}

// TestPayuniMerStatusValues_MatchesNames guards the hand-maintained index
// tables in the generated file: enumer does not yet build under Go 1.27, so
// the value list and the name list must be checked against each other.
func TestPayuniMerStatusValues_MatchesNames(t *testing.T) {
	values := PayuniMerStatusValues()
	names := PayuniMerStatusStrings()
	if len(values) != len(names) {
		t.Fatalf("values=%d names=%d, must match", len(values), len(names))
	}
	for i, v := range values {
		if v.String() != names[i] {
			t.Errorf("index %d: value %d renders %q but names[%d]=%q", i, v, v.String(), i, names[i])
		}
	}
}
