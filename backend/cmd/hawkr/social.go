package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// socialProducts 以還在舊服務的 product-service 實作 social 的商品驗證 port（B4 的 catalog 搬進單體後換成 in-process）：商品要存在、上架中
// （includeInactive 為假時只回上架中的）而且屬於這家店。回 social 的 ErrProductNotAvailable、ErrProductNotOwned，查詢失敗回原錯誤（social 轉成
// Unavailable）：不把「查不到」當成「商品不存在」。使用者面的句子在 social 的目錄，這裡不寫。
type socialProducts struct{ client *productservice.Client }

func (a socialProducts) VerifyProduct(ctx context.Context, productID, storeID int64) error {
	cards, err := a.client.Cards(ctx, []int64{productID}, false)
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

// noStoreBlocks 是不做買家身分寫入的組裝（social-recount）給 social 的店家封鎖依賴：被呼叫就是接線錯了，回錯誤而不是回「沒有封鎖」。
type noStoreBlocks struct{}

func (noStoreBlocks) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) {
	return false, errors.New("此組裝不做買家身分的寫入，沒有店家封鎖的來源")
}

// socialSellers 是 social 對 seller 的兩個依賴：讀店家與店家封鎖檢查；正式組裝傳 *seller.Module。
type socialSellers interface {
	social.Stores
	social.StoreBlocks
}

// newSocial 組裝 social 模組。serve 與 worker 都需要它：serve 要訂閱買家與店家的事件（買家的刪除事件在發布方的 transaction 內記複本並重新求值、排清除 job；店家退場只排清除 job），worker 執行
// 它的 job（圖片引用、清除、瀏覽資料清理、每日對帳；排程的是 PeriodicJobs）。
func newSocial(d *db.DB, bus *events.Bus, jc jobs.Enqueuer, buyers *buyer.Module, sellers socialSellers, med media.Media, products socialProducts, log *slog.Logger) *social.Module {
	m := social.New(social.Deps{
		DB: d, Events: bus, Media: med, Jobs: jc, Buyers: buyers, Stores: sellers, StoreBlocks: sellers, Products: products,
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
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
	m := social.New(social.Deps{DB: d, StoreBlocks: noStoreBlocks{}, NewID: uuid.NewV7, Now: time.Now, Logger: newLogger()})
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
