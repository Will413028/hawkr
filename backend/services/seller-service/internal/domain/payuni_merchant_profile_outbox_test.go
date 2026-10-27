package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewPayuniMerchantProfileOutboxRow(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	row, err := NewPayuniMerchantProfileOutboxRow(101, 202, 303, "台北市信義區松高路1號", "110", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.ID != 101 || row.StoreID != 202 || row.AccountID != 303 {
		t.Errorf("ids = %d/%d/%d, want 101/202/303", row.ID, row.StoreID, row.AccountID)
	}
	if row.Status != PayuniMerchantProfileOutboxPending {
		t.Errorf("Status = %q, want pending", row.Status)
	}
	if !row.NextAttemptAt.Equal(now) || !row.CreatedAt.Equal(now) {
		t.Errorf("timestamps not set from now")
	}
	var p PayuniMerchantProfilePayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		t.Fatalf("payload not valid json: %v", err)
	}
	if p.MerAddr != "台北市信義區松高路1號" || p.MerAddrPost != "110" {
		t.Errorf("payload = %+v, want the address passed in", p)
	}
}

// 送出的是「變動後的值」而不是 diff：worker 執行時 sellers 可能已再次修改，
// 存值才能保證送出的是當時那次變動的意圖，且讓重試冪等。
func TestPayuniMerchantProfilePayload_WireKeys(t *testing.T) {
	raw, err := json.Marshal(PayuniMerchantProfilePayload{MerAddr: "A", MerAddrPost: "100"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got, want := string(raw), `{"merAddr":"A","merAddrPost":"100"}`; got != want {
		t.Errorf("wire = %s, want %s", got, want)
	}
}

func (o *PayuniMerchantProfileOutbox) mustSatisfyOutboxRow() (int64, int) {
	return o.OutboxID(), o.OutboxAttemptCount()
}

func TestPayuniMerchantProfileOutbox_SatisfiesOutboxRow(t *testing.T) {
	o := &PayuniMerchantProfileOutbox{ID: 7, AttemptCount: 3}
	id, attempt := o.mustSatisfyOutboxRow()
	if id != 7 || attempt != 3 {
		t.Errorf("OutboxID/OutboxAttemptCount = %d/%d, want 7/3", id, attempt)
	}
}
