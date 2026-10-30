package delegationservice

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// 舊契約對外仍是小寫；模組的儲存值是 UPPER_SNAKE。轉換只收小寫，來回相等。
func TestWireCodesStayLowercaseOnTheLegacyContract(t *testing.T) {
	for want, got := range map[string]string{
		"active":          toWire(delegation.DelegationActive),
		"revoked":         toWire(delegation.DelegationRevoked),
		"expired":         toWire(delegation.DelegationExpired),
		"high":            toWire(delegation.RiskHigh),
		"interactive":     toWire(delegation.AuditInteractive),
		"platform_policy": toWire(delegation.AuthorizationPlatformPolicy),
		"seller_request":  toWire(delegation.ReasonSellerRequest),
		"storefront":      toWire(delegation.ResourceStorefront),
	} {
		if got != want {
			t.Errorf("toWire = %q，預期 %q", got, want)
		}
	}
	if v := fromWire[delegation.AuditOrigin]("scheduled"); v != delegation.AuditScheduled {
		t.Errorf("fromWire(scheduled) = %q", v)
	}
	for _, in := range []string{"", "SCHEDULED", "Scheduled"} {
		if v := fromWire[delegation.AuditOrigin](in); v != "" {
			t.Errorf("fromWire(%q) = %q，預期空字串（只收小寫）", in, v)
		}
	}
}
