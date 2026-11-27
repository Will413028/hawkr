package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
	"uuid"

	httpsocial "github.com/Will413028/hawkr/backend/internal/httpapi/social"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// socialProducts 以 catalog 模組實作 social 的商品驗證 port：商品要存在、上架中（店家可曝光）而且屬於這家店。
// 回 social 的 ErrProductNotAvailable、ErrProductNotOwned，查詢失敗回原錯誤（social 轉成 Unavailable）：
// 不把「查不到」當成「商品不存在」。使用者面的句子在 social 的目錄，這裡不寫。
type socialProducts struct{ catalog *catalog.Module }

func (a socialProducts) VerifyProduct(ctx context.Context, productID uuid.UUID, storeID int64) error {
	cards, err := a.catalog.ProductCards(ctx, time.Now(), []uuid.UUID{productID}, false)
	if err != nil {
		return err
	}
	card, ok := cards[productID]
	switch {
	case !ok:
		return social.ErrProductNotAvailable
	case card.StoreID != storeID:
		return social.ErrProductNotOwned
	}
	return nil
}

// socialCards 以 catalog 模組實作貼文卡片的商品顯示資料 port。
type socialCards struct{ catalog *catalog.Module }

func (a socialCards) Cards(ctx context.Context, ids []uuid.UUID, includeInactive bool) (map[uuid.UUID]httpsocial.ProductInfo, error) {
	items, err := a.catalog.ProductCards(ctx, time.Now(), ids, includeInactive)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]httpsocial.ProductInfo, len(items))
	for id, item := range items {
		out[id] = httpsocial.ProductInfo{
			ID: item.ID, StoreID: item.StoreID, Name: item.Title, ImageKey: item.CoverKey,
			MinPrice: item.MinPrice, MaxPrice: item.MaxPrice,
			Available: item.Status == catalog.StatusActive && item.TotalStock > 0,
		}
	}
	return out, nil
}

// noStoreBlocks 是不做買家身分寫入的組裝（social-recount）給 social 的店家封鎖依賴：被呼叫就是接線錯了，回錯誤而不是回「沒有封鎖」。
type noStoreBlocks struct{}

func (noStoreBlocks) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) {
	return false, errors.New("此組裝不做買家身分的寫入，沒有店家封鎖的來源")
}

// scheduleAuthorizers 是 social 的排程授權實作，依發起者種類登記（缺一種 social.New 就 panic）。代行（DELEGATION）的實作由 delegation 提供
// （delegation.NewScheduleAuthorizer），每一個組裝 social 的地方都傳真的實作，沒有「一律拒絕」的佔位。
func scheduleAuthorizers(delegationAuthorizer social.ScheduleAuthorizer) map[social.ScheduleKind]social.ScheduleAuthorizer {
	return map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: delegationAuthorizer}
}

// socialSellers 是 social 對 seller 的兩個依賴：讀店家與店家封鎖檢查；正式組裝傳 *seller.Module。
type socialSellers interface {
	social.Stores
	social.StoreBlocks
}

// newSocial 組裝 social 模組。serve 與 worker 都需要它：serve 要訂閱買家與店家的事件（買家的刪除事件在發布方的 transaction 內記複本並重新求值、排清除 job；店家退場只排清除 job），worker 執行
// 它的 job（圖片引用、清除、瀏覽資料清理、每日對帳；排程的是 PeriodicJobs）。
func newSocial(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers *buyer.Module, sellers socialSellers, med mediaPort, products socialProducts, delegationAuthorizer social.ScheduleAuthorizer, log *slog.Logger) *social.Module {
	m := social.New(social.Deps{
		DB: d, Events: bus, Media: med, Jobs: jc, Buyers: buyers, Stores: sellers, StoreBlocks: sellers, Products: products,
		ScheduleAuthorizers: scheduleAuthorizers(delegationAuthorizer),
		NewID:               uuid.NewV7, Now: time.Now, Logger: log,
	})
	subscribeSocial(bus, m)
	return m
}

// subscribeSocial 讓買家進入刪除中與關閉在同一個 transaction 寫 social 的複本、重新求值並排清除 job（單體規則 4 的原子狀態變更），
// 店家退場只排清除 job（訂閱者只 Enqueue），seller 建立與撤銷內容下架措施在同一個 transaction 寫複本、重新求值並排快取失效。
func subscribeSocial(bus *events.Bus, m *social.Module) {
	events.Subscribe(bus, m.OnBuyerDeletionRequested)
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	events.Subscribe(bus, m.OnMeasureCreated)
	events.Subscribe(bus, m.OnMeasureRevoked)
}

// runSocialRecount 是子命令 social-recount：對全部可重算的計數（貼文的讚、留言與分享數，留言的讚與回覆數，討論區留言的回覆數）跑重算與覆寫，
// 印出數量與漂移（手動全量重算；改了計數定義的 migration 在同一步跑一次）。只需要 DATABASE_URL。
func runSocialRecount(ctx context.Context, out io.Writer) error {
	e := config.NewEnv()
	dbc := dbConfig(e)
	if err := e.Err(); err != nil {
		return err
	}
	d, err := db.Open(ctx, dbc)
	if err != nil {
		return err
	}
	defer d.Close()
	log := newLogger()
	delegationAuthorizer := delegation.NewScheduleAuthorizer(delegation.Deps{
		DB: d, Identity: identity.New(identity.Deps{DB: d, Now: time.Now, Logger: log}), Seller: seller.NewReader(d, time.Now), Now: time.Now, Logger: log,
	})
	m := social.New(social.Deps{DB: d, StoreBlocks: noStoreBlocks{}, ScheduleAuthorizers: scheduleAuthorizers(delegationAuthorizer), NewID: uuid.NewV7, Now: time.Now, Logger: log})
	rep, err := m.RecountAll(ctx)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "重算：貼文 %d、留言 %d、討論區留言 %d；漂移 %d\n", rep.Posts, rep.Comments, rep.ForumEntries, rep.Drifted)
	for _, drift := range rep.Drifts {
		_, _ = fmt.Fprintf(out, "  %s %s：%v → %v\n", drift.Kind, drift.ID, drift.Before, drift.After)
	}
	return nil
}
