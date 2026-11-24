package notification

import (
	"slices"
	"strings"
	"testing"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
)

// 投遞失敗原因的契約列舉與模組的值域一一對應（模組存小寫、契約是大寫），且可重送的原因都是已知的原因。
func TestDeliveryFailureReasonEnumMatchesTheModule(t *testing.T) {
	all := notification.DeliveryFailureReasons()
	upper := make([]string, len(all))
	for i, r := range all {
		upper[i] = strings.ToUpper(r)
	}
	if bad := rpc.EnumMismatches[adminpb.DeliveryFailureReason](upper); len(bad) != 0 {
		t.Fatal(bad)
	}
	for _, r := range notification.ResendableReasons() {
		if !slices.Contains(all, r) {
			t.Errorf("可重送的原因 %q 不在全部原因裡", r)
		}
	}
}
