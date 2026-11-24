package deliver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

func quietPlan(t *testing.T, name string, expiresAt time.Time) plan {
	t.Helper()
	e, ok := registry.Lookup(name)
	if !ok {
		t.Fatalf("沒有型別 %s", name)
	}
	raw, _, err := e.ExampleParams()
	if err != nil {
		t.Fatal(err)
	}
	if name == "voucher_expiring" {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		m["expires_at"] = expiresAt.UTC().Format(time.RFC3339)
		if raw, err = json.Marshal(m); err != nil {
			t.Fatal(err)
		}
	}
	return plan{entry: e, params: raw}
}

func TestQuietWait(t *testing.T) {
	at := func(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, twtime.Taipei) }
	cases := []struct {
		name    string
		typ     string
		now     time.Time
		expires time.Time
		want    time.Duration
	}{
		{"行銷類 23:00 等到 09:00", "voucher_expiring", at(24, 23, 0), at(26, 0, 0), 10 * time.Hour},
		{"行銷類 02:00 等到 09:00", "voucher_expiring", at(25, 2, 0), at(26, 0, 0), 7 * time.Hour},
		{"行銷類 10:00 立即", "voucher_expiring", at(24, 10, 0), at(26, 0, 0), 0},
		{"行銷類 21:59 立即", "voucher_expiring", at(24, 21, 59), at(26, 0, 0), 0},
		{"靜音結束前到期不延後", "voucher_expiring", at(24, 23, 30), at(25, 6, 0), 0},
		{"剛好 09:00 到期不延後", "voucher_expiring", at(24, 23, 30), at(25, 9, 0), 0},
		{"09:01 到期仍延後", "voucher_expiring", at(24, 23, 30), at(25, 9, 1), 9*time.Hour + 30*time.Minute},
		{"非行銷類不延後", "account_suspended", at(24, 23, 0), time.Time{}, 0},
	}
	for _, c := range cases {
		s := New(Deps{Now: func() time.Time { return c.now }})
		got, err := s.quietWait(quietPlan(t, c.typ, c.expires))
		if err != nil || got != c.want {
			t.Errorf("%s：quietWait = %v（%v），預期 %v", c.name, got, err, c.want)
		}
	}
}
