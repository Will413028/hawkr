package notification

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/announce"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 公告一頁的收件人數上限不得超過名單來源（buyer、seller）一頁的上限，否則 ListLive* 回 InvalidInput、派送永遠失敗。
func TestAnnounceChunkFitsTheDirectorySources(t *testing.T) {
	if announce.MaxChunk > buyer.MaxLiveAccountsPage || announce.MaxChunk > seller.MaxLiveStoresPage {
		t.Fatalf("announce.MaxChunk = %d，buyer 上限 %d、seller 上限 %d", announce.MaxChunk, buyer.MaxLiveAccountsPage, seller.MaxLiveStoresPage)
	}
}
