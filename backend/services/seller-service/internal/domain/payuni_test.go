package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFormatMerID_Format(t *testing.T) {
	id := FormatMerID(2612345678)
	if id != "HAWK2612345678" {
		t.Errorf("FormatMerID(2612345678) = %q, want %q", id, "HAWK2612345678")
	}
}

func TestFormatMerID_Prefix(t *testing.T) {
	id := FormatMerID(2600000001)
	if !strings.HasPrefix(id, "HAWK") {
		t.Errorf("FormatMerID() = %q, want HAWK prefix", id)
	}
}

func TestPayuniRegistration_SetRegistered(t *testing.T) {
	reg := &PayuniRegistration{}
	merID := "HAWK2612345678"
	now := time.Now()

	reg.SetRegistered(merID, now)

	if reg.PayuniMerID != merID {
		t.Errorf("PayuniMerID = %v, want %v", reg.PayuniMerID, merID)
	}
	if reg.Status != PayuniStatusPending {
		t.Errorf("Status = %v, want %v", reg.Status, PayuniStatusPending)
	}
	if reg.RegisteredAt == nil || !reg.RegisteredAt.Equal(now) {
		t.Errorf("RegisteredAt = %v, want %v", reg.RegisteredAt, now)
	}
	if !reg.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", reg.UpdatedAt, now)
	}
}

func TestPayuniRegistration_SetActive(t *testing.T) {
	reg := &PayuniRegistration{
		Status: PayuniStatusPending,
	}
	now := time.Now()

	reg.SetActive(now)

	if reg.Status != PayuniStatusActive {
		t.Errorf("Status = %v, want %v", reg.Status, PayuniStatusActive)
	}
	if !reg.PaymentEnabled {
		t.Errorf("PaymentEnabled = %v, want true", reg.PaymentEnabled)
	}
	if reg.ApprovedAt == nil || !reg.ApprovedAt.Equal(now) {
		t.Errorf("ApprovedAt = %v, want %v", reg.ApprovedAt, now)
	}
	if !reg.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", reg.UpdatedAt, now)
	}
}

func TestPayuniRegistration_SetFailed(t *testing.T) {
	reg := &PayuniRegistration{
		Status: PayuniStatusPending,
	}
	errorMsg := "Registration failed"
	now := time.Now()

	reg.SetFailed("", errorMsg, now)

	if reg.Status != PayuniStatusFailed {
		t.Errorf("Status = %v, want %v", reg.Status, PayuniStatusFailed)
	}
	if reg.ErrorMessage != errorMsg {
		t.Errorf("ErrorMessage = %v, want %v", reg.ErrorMessage, errorMsg)
	}
	if !reg.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", reg.UpdatedAt, now)
	}
}

func TestPayuniRegistration_Apply(t *testing.T) {
	reg := &PayuniRegistration{}
	now := time.Now()

	reg.Apply(PayuniStatusObservation{
		ObservedAt: now, Source: "sync",
		MerStatus: "1", MemberStatus: "1", CreditStatus: "0",
		ApplePayStatus: "1", GooglePayStatus: "0", ATMStatus: "1",
		Reason: "documents need re-upload",
	})

	if reg.MerStatus != "1" {
		t.Errorf("MerStatus = %v, want 1", reg.MerStatus)
	}
	if reg.MemberStatus != "1" {
		t.Errorf("MemberStatus = %v, want 1", reg.MemberStatus)
	}
	if reg.CreditStatus != "0" {
		t.Errorf("CreditStatus = %v, want 0", reg.CreditStatus)
	}
	if reg.ApplePayStatus != "1" {
		t.Errorf("ApplePayStatus = %v, want 1", reg.ApplePayStatus)
	}
	if reg.GooglePayStatus != "0" {
		t.Errorf("GooglePayStatus = %v, want 0", reg.GooglePayStatus)
	}
	if reg.ATMStatus != "1" {
		t.Errorf("ATMStatus = %v, want 1", reg.ATMStatus)
	}
	if reg.Reason != "documents need re-upload" {
		t.Errorf("Reason = %v, want documents need re-upload", reg.Reason)
	}
	if reg.LastSyncedAt == nil || !reg.LastSyncedAt.Equal(now) {
		t.Errorf("LastSyncedAt = %v, want %v", reg.LastSyncedAt, now)
	}
	if !reg.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", reg.UpdatedAt, now)
	}
}

func TestPayuniRegistration_KYCChangedBy(t *testing.T) {
	cases := []struct {
		name         string
		merStatus    string
		memberStatus string
		want         bool
	}{
		{"both same, no change", "1", "1", false},
		{"MerStatus differs", "9", "1", true},
		{"MemberStatus differs", "1", "5", true},
		{"both differ", "9", "5", true},
	}
	for _, tc := range cases {
		reg := &PayuniRegistration{MerStatus: "1", MemberStatus: "1"}
		obs := PayuniStatusObservation{MerStatus: tc.merStatus, MemberStatus: tc.memberStatus}
		if got := reg.KYCChangedBy(obs); got != tc.want {
			t.Errorf("%s: KYCChangedBy(%q, %q) = %v, want %v",
				tc.name, tc.merStatus, tc.memberStatus, got, tc.want)
		}
	}
}

func TestPayuniRegistration_Lifecycle(t *testing.T) {
	reg := &PayuniRegistration{}

	// 1. Initial State
	if reg.Status != PayuniStatusUnregistered {
		t.Errorf("Initial Status = %v, want Unregistered", reg.Status)
	}

	// 2. Register
	merID := "HAWK2612345678"
	t1 := time.Now()
	reg.SetRegistered(merID, t1)

	if reg.Status != PayuniStatusPending {
		t.Errorf("After SetRegistered: Status = %v, want Pending", reg.Status)
	}
	if reg.PayuniMerID != merID {
		t.Errorf("After SetRegistered: MerID = %v, want %v", reg.PayuniMerID, merID)
	}
	if reg.RegisteredAt == nil || !reg.RegisteredAt.Equal(t1) {
		t.Errorf("After SetRegistered: RegisteredAt mismatch")
	}

	// 3. Update Status (e.g. callback or sync)
	t2 := t1.Add(time.Minute)
	reg.Apply(PayuniStatusObservation{
		ObservedAt: t2, Source: "sync",
		MerStatus: "1", MemberStatus: "1", CreditStatus: "1",
		ApplePayStatus: "1", GooglePayStatus: "1", ATMStatus: "1",
	})

	if reg.MerStatus != "1" {
		t.Errorf("After Apply: MerStatus mismatch")
	}
	if reg.LastSyncedAt == nil || !reg.LastSyncedAt.Equal(t2) {
		t.Errorf("After Apply: LastSyncedAt mismatch")
	}
	if !reg.UpdatedAt.Equal(t2) {
		t.Errorf("After Apply: UpdatedAt mismatch")
	}

	// 4. Activate
	t3 := t2.Add(time.Minute)
	reg.SetActive(t3)

	if reg.Status != PayuniStatusActive {
		t.Errorf("After SetActive: Status = %v, want Active", reg.Status)
	}
	if !reg.PaymentEnabled {
		t.Errorf("After SetActive: PaymentEnabled should be true")
	}
	if reg.ApprovedAt == nil || !reg.ApprovedAt.Equal(t3) {
		t.Errorf("After SetActive: ApprovedAt mismatch")
	}

	// 5. Fail (e.g. later if something goes wrong, though typically it goes from Pending -> Failed)
	// Let's reset and test failure path
	regFail := &PayuniRegistration{}
	regFail.SetRegistered(merID, t1)

	tFail := t1.Add(time.Hour)
	failMsg := "Documents rejected"
	regFail.SetFailed("", failMsg, tFail)

	if regFail.Status != PayuniStatusFailed {
		t.Errorf("After SetFailed: Status = %v, want Failed", regFail.Status)
	}
	if regFail.ErrorMessage != failMsg {
		t.Errorf("After SetFailed: ErrorMessage mismatch")
	}
	if !regFail.UpdatedAt.Equal(tFail) {
		t.Errorf("After SetFailed: UpdatedAt mismatch")
	}
}

// Apply must leave RawResponse alone. The column holds the last PAYUNi *API*
// response and is written by the pull paths only; an observation's own raw
// input travels with the observation and reaches the audit trail through
// NewStatusHistory. Letting Apply write it would put both back in one slot,
// which is what made webhook audit rows inherit an unrelated payload in the
// first place.
func TestPayuniRegistration_Apply_LeavesRawResponseAlone(t *testing.T) {
	apiResponse := json.RawMessage(`{"MerStatus":"9","MemberStatus":"9"}`)
	reg := &PayuniRegistration{RawResponse: apiResponse}

	reg.Apply(PayuniStatusObservation{
		ObservedAt: time.Now(), Source: "webhook",
		MerStatus: "1", MemberStatus: "1",
		Raw: json.RawMessage(`{"MerStatus":"1","MemberStatus":"1"}`),
	})

	if string(reg.RawResponse) != string(apiResponse) {
		t.Errorf("RawResponse = %s, want it untouched (%s)", reg.RawResponse, apiResponse)
	}
}

// An audit row takes its identity from the registration and everything else
// from the observation — never the registration's own status fields, which
// are whatever the last observation to touch them left behind.
func TestPayuniRegistration_NewStatusHistory_TakesStatusFromObservation(t *testing.T) {
	now := time.Now()
	reg := &PayuniRegistration{
		AccountID: 100, StoreID: 7300000000000000201, PayuniMerID: "HAWK100",
		// Stale state from an earlier observation, plus an unrelated raw body.
		MerStatus: "9", MemberStatus: "9", Reason: "舊的原因",
		RawResponse: json.RawMessage(`{"Status":"SUCCESS","Message":"合作商店資料建立成功"}`),
	}
	obs := PayuniStatusObservation{
		ObservedAt: now, Source: "webhook",
		MerStatus: "1", MemberStatus: "1", Reason: "",
		Raw: json.RawMessage(`{"MerStatus":"1","MemberStatus":"1"}`),
	}

	h := reg.NewStatusHistory(42, obs)

	if h.AccountID != 100 || h.StoreID != 7300000000000000201 || h.PayuniMerID != "HAWK100" {
		t.Errorf("identity fields must come from the registration, got %+v", h)
	}
	if h.MerStatus != "1" || h.MemberStatus != "1" {
		t.Errorf("status = %q/%q, want 1/1 from the observation", h.MerStatus, h.MemberStatus)
	}
	if h.Reason != "" {
		t.Errorf("Reason = %q, want the observation's empty one, not the stale %q", h.Reason, reg.Reason)
	}
	if string(h.RawResponse) != string(obs.Raw) {
		t.Errorf("RawResponse = %s, want the observation's own payload %s", h.RawResponse, obs.Raw)
	}
	if !h.RecordedAt.Equal(now) {
		t.Errorf("RecordedAt = %v, want the observation instant %v", h.RecordedAt, now)
	}
}

func TestPayuniRegistration_SetFailed_CarriesCode(t *testing.T) {
	now := time.Now().UTC()
	r := &PayuniRegistration{}
	r.SetFailed("MER_92", "審核未通過", now)
	if r.ErrorCode != "MER_92" {
		t.Errorf("ErrorCode = %q, want MER_92", r.ErrorCode)
	}
	if r.ErrorMessage != "審核未通過" {
		t.Errorf("ErrorMessage = %q, want 審核未通過", r.ErrorMessage)
	}
}

func TestPayuniRegistration_SetActive_ClearsCode(t *testing.T) {
	now := time.Now().UTC()
	r := &PayuniRegistration{ErrorCode: "MER_92", ErrorMessage: "審核未通過"}
	r.SetActive(now)
	if r.ErrorCode != "" {
		t.Errorf("ErrorCode = %q, want empty after SetActive", r.ErrorCode)
	}
}
