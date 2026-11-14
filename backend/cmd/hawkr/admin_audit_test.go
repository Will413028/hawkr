package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
)

func TestParseAdminAudit(t *testing.T) {
	since := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 11, 15, 8, 30, 0, 0, time.UTC)
	f, err := parseAdminAudit([]string{"-admin", "7", "-action", "/svc/A", "-target-type", "payment_account", "-target-id", "11", "-since", "2026-11-01", "-until", "2026-11-15T08:30:00Z", "-limit", "5"})
	want := adminaudit.Filter{AdminID: 7, Action: "/svc/A", TargetType: "payment_account", TargetID: "11", Since: since, Until: until, Limit: 5}
	if err != nil || f != want {
		t.Fatalf("f=%+v err=%v，預期 %+v", f, err, want)
	}
	if f, err := parseAdminAudit(nil); err != nil || f != (adminaudit.Filter{}) {
		t.Fatalf("沒有旗標應該是空條件：%+v %v", f, err)
	}
	for name, args := range map[string][]string{
		"負的管理員":           {"-admin", "-1"},
		"負的筆數":            {"-limit", "-1"},
		"target-id 沒有種類":  {"-target-id", "11"},
		"時間格式不合":          {"-since", "yesterday"},
		"until 不晚於 since": {"-since", "2026-11-02", "-until", "2026-11-02"},
		"多餘的參數":           {"extra"},
		"不認得的旗標":          {"-who", "me"},
	} {
		if _, err := parseAdminAudit(args); err == nil {
			t.Errorf("%s：應該回錯誤", name)
		}
	}
}

type fakeLister struct {
	got  adminaudit.Filter
	recs []adminaudit.Record
}

func (l *fakeLister) List(_ context.Context, f adminaudit.Filter) ([]adminaudit.Record, error) {
	l.got = f
	return l.recs, nil
}

func TestPrintAdminAuditListsOneLinePerRecordAndPassesTheFilter(t *testing.T) {
	l := &fakeLister{recs: []adminaudit.Record{{ID: 3, OccurredAt: time.Date(2026, 11, 14, 4, 0, 0, 0, time.UTC), AdminID: 7, Action: "/svc/A", TargetType: "payment_account", TargetID: "11", Detail: json.RawMessage(`{"outcome":"requested"}`)}}}
	var out bytes.Buffer
	f := adminaudit.Filter{AdminID: 7}
	if err := printAdminAudit(context.Background(), l, f, &out); err != nil {
		t.Fatal(err)
	}
	if l.got != f {
		t.Fatalf("篩選條件沒有傳下去：%+v", l.got)
	}
	got := out.String()
	for _, want := range []string{"3\t2026-11-14T04:00:00Z\tadmin=7\t/svc/A\tpayment_account/11\t{\"outcome\":\"requested\"}\n", "共 1 筆\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("輸出缺 %q：%q", want, got)
		}
	}
}
