package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newModerationRetention 組裝 moderation 的保留期與檢舉人資料清除。serve 要訂閱買家的刪除事件（同 transaction 清掉檢舉人的說明與每日計數列，A7-D21），
// worker 執行它的 job（快照與說明的保留期、快照圖片的 media 引用同步）；兩邊都只需要它，不必組出完整的 moderation 模組（處置要的 social、seller、accountsuspension 在 worker 用不到）。
func newModerationRetention(d *db.DB, buyers *buyer.Module, med media.Media, jc jobs.Enqueuer, log *slog.Logger) *moderation.Retention {
	return moderation.NewRetention(moderation.RetentionDeps{DB: d, Buyer: buyers, Media: med, Jobs: jc, Now: time.Now, Logger: log})
}

// newModeration 組裝 moderation 模組，與 serve 的買家刪除事件訂閱共用同一個 Retention。
func newModeration(d *db.DB, soc *social.Module, sel *seller.Module, buyers *buyer.Module, suspension *accountsuspension.Module, med media.Media, jc jobs.Enqueuer, log *slog.Logger) (*moderation.Module, *moderation.Retention) {
	ret := newModerationRetention(d, buyers, med, jc, log)
	return moderation.New(moderation.Deps{
		DB: d, Social: soc, Seller: sel, Buyer: buyers, Suspension: suspension,
		Media: med, Jobs: jc, NewID: uuid.NewV7, Now: time.Now, Logger: log, Retention: ret,
	}), ret
}

// subscribeModeration 讓買家進入刪除中與關閉在同一個 transaction 清掉檢舉人的說明與每日計數列（單體規則 4 的原子狀態變更）。
func subscribeModeration(bus *events.Bus, r *moderation.Retention) {
	events.Subscribe(bus, r.OnBuyerDeletionRequested)
	events.Subscribe(bus, r.OnBuyerDeleted)
}
