package session

import (
	"regexp"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

func TestRevokerDisplayNameMissingAdmin(t *testing.T) {
	id := int64(7)
	if got := revokerDisplayName(nil, &id); got != nil {
		t.Fatalf("缺少管理員時 = %q，預期 nil", *got)
	}
	if got := revokerDisplayName(map[int64]identity.Admin{7: {Email: "ops@example.test"}}, &id); got == nil || *got != "ops@example.test" {
		t.Fatalf("存在管理員時 = %v", got)
	}
}

func TestDisplayCodeIsRandomAndMatchesTheContract(t *testing.T) {
	re := regexp.MustCompile(`^OPS-[0-9A-Z]{13}$`)
	seen := map[string]bool{}
	for range 1000 {
		c := model.NewDisplayCode()
		if !re.MatchString(c) || !validDisplayCode(c) {
			t.Fatalf("%q 不符合 OPS- 加 13 碼", c)
		}
		if seen[c] {
			t.Fatalf("%q 重複", c)
		}
		seen[c] = true
	}
	for _, c := range []string{"OPS-0000000000000", "OPS-ABCDEFGHIJKLM"} {
		if !validDisplayCode(c) {
			t.Errorf("%q", c)
		}
	}
	for _, c := range []string{"", "OPS-abcdefghijklm", "OPS-000000000000", "ops-0000000000000", "OPS-00000000000000"} {
		if validDisplayCode(c) {
			t.Errorf("%q 不該通過", c)
		}
	}
}

func TestDelegationStatusFollowsTheOrderedRules(t *testing.T) {
	issued := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	s := store.DelegationDelegatedOperationSession{
		Status: "ACTIVE", IssuedAt: issued, ExpiresAt: issued.Add(30 * time.Minute), LastActiveAt: issued,
	}
	cases := []struct {
		name string
		now  time.Time
		mut  func(*store.DelegationDelegatedOperationSession)
		want model.DelegationStatus
	}{
		{"進行中", issued.Add(time.Minute), nil, model.DelegationActive},
		{"閒置逾時的前一刻", issued.Add(model.DelegationIdleTimeout - time.Second), nil, model.DelegationActive},
		{"閒置逾時", issued.Add(model.DelegationIdleTimeout), nil, model.DelegationExpired},
		{"hard 到期", issued.Add(30 * time.Minute), func(s *store.DelegationDelegatedOperationSession) { s.LastActiveAt = issued.Add(29 * time.Minute) }, model.DelegationExpired},
		{"撤銷優先於到期", issued.Add(time.Hour), func(s *store.DelegationDelegatedOperationSession) { s.Status = "REVOKED" }, model.DelegationRevoked},
	}
	for _, c := range cases {
		row := s
		if c.mut != nil {
			c.mut(&row)
		}
		if got := delegationStatus(row, c.now); got != c.want {
			t.Errorf("%s = %s，預期 %s", c.name, got, c.want)
		}
	}
}
