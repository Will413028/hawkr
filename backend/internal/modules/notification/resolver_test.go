package notification

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

// fan-out 一頁的上限與 buyer 的追蹤者分頁上限是同一個數：兩者不一致時，夾到的頁大小會被 buyer 當成 InvalidInput。
func TestFanoutPageLimitEqualsTheBuyerPageLimit(t *testing.T) {
	t.Parallel()
	if registry.MaxFollowerPage != buyer.MaxLiveAccountsPage {
		t.Fatalf("registry.MaxFollowerPage = %d，buyer.MaxLiveAccountsPage = %d", registry.MaxFollowerPage, buyer.MaxLiveAccountsPage)
	}
}
