package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Will413028/hawkr/backend/internal/legacy/mediaservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// sellerCommand 是子命令共用的組裝：資料庫、事件匯流排、identity 與 seller 模組，依賴與 serve、worker 同一組設定與同一個 newSeller，
// 事件的 relay 訂閱也是同一個 subscribeSellerRelay（命令發出的事件在自己的 transaction 內排入 relay job，由 worker 送出）。
type sellerCommand struct {
	DB       *db.DB
	Bus      *events.Bus
	Identity *identity.Module
	Seller   *seller.Module
}

// openSeller 組裝 sellerCommand，需要完整 seller 模組的子命令（reannounce-exposures、dev-seed）走這裡。子命令不呼叫 media、order、product 的服務
// （clients 是惰性連線），但模組要求這些依賴，位址、KYC 金鑰與 serve 一樣必填。只讀店家的子命令（delegation-eligibility、admin-role）
// 不走這裡，用 seller.NewReader，只要 DATABASE_URL。關閉用 Close。
func openSeller(ctx context.Context, prepare func(*config.Env) error) (*sellerCommand, error) {
	e := config.NewEnv()
	if prepare != nil {
		if err := prepare(e); err != nil {
			return nil, err
		}
	}
	dbc := dbConfig(e)
	internalSecret := e.StringOr("INTERNAL_SERVICE_HMAC_SECRET", "")
	mediaCfg := mediaservice.Config{Addr: e.String("MEDIA_SERVICE_ADDR"), Secret: internalSecret}
	orderAddr := e.String("ORDER_SERVICE_ADDR")
	productAddr := e.String("PRODUCT_SERVICE_ADDR")
	log := newLogger()
	kyc := readKYCKeys(e)
	pay := readPayUni(e, log)
	registry := readGCIS(e)
	if err := e.Err(); err != nil {
		return nil, err
	}
	d, err := db.Open(ctx, dbc)
	if err != nil {
		return nil, err
	}
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		d.Close()
		return nil, err
	}
	med, err := mediaservice.New(mediaCfg)
	if err != nil {
		d.Close()
		return nil, err
	}
	orders, err := orderservice.New(orderAddr, internalSecret)
	if err != nil {
		d.Close()
		return nil, err
	}
	products, err := productservice.New(productAddr, internalSecret)
	if err != nil {
		d.Close()
		return nil, err
	}
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	module := newSeller(d, bus, jc, id, med, orders, sellerProducts{client: products}, kyc, pay, registry, log)
	subscribeSellerRelay(bus, jc)
	return &sellerCommand{DB: d, Bus: bus, Identity: id, Seller: module}, nil
}

// Close 關閉資料庫連線。
func (c *sellerCommand) Close() { c.DB.Close() }

// openSellerCommand 是只要 seller 模組的子命令（reannounce-exposures、dev-seed）用的簡寫。
func openSellerCommand(ctx context.Context, prepare func(*config.Env) error) (*seller.Module, func(), error) {
	c, err := openSeller(ctx, prepare)
	if err != nil {
		return nil, nil, err
	}
	return c.Seller, c.Close, nil
}

const reannounceUsage = "用法：reannounce-exposures（不帶參數）"

// runReannounceExposures 由 seller 全量重新公告可曝光：每一家店（含已關閉的）都不比對差異地公告一次，版本照常加一。種子重寫之後，以及 Java 的
// store_exposure 重建或死信處理之後使用。事件經 relay job 排入資料庫，由 worker（make worker）送到 RabbitMQ。
func runReannounceExposures(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New(reannounceUsage)
	}
	module, closeFn, err := openSellerCommand(ctx, nil)
	if err != nil {
		return err
	}
	defer closeFn()
	n, err := module.ReannounceExposures(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "已重新公告 %d 家店的可曝光（事件由 worker 送出）\n", n)
	return err
}

const devSeedUsage = "用法：dev-seed [-check]（只給本機開發；-check 只檢查前提、不寫資料，在 01-account.sql 之前執行）"

// runDevSeed 把本機開發的種子店家還原成 fixture（devSeedStores，明文 fixture；upsert，不碰種子以外的店家）：與 dev-token 同一個保護
// （requireDevEnvironment），前提是 migrate 已套用。寫完不公告：接著跑 reannounce-exposures（infra/seed-test-data.sh 依序呼叫兩者）。
// -check 在 01-account.sql 刪除重建帳號之前確認前提（見 seller.CheckDevSeed），不成立就失敗、不寫任何資料。
func runDevSeed(ctx context.Context, args []string, out io.Writer) error {
	checkOnly := false
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "-check":
		checkOnly = true
	default:
		return errors.New(devSeedUsage)
	}
	c, err := openSeller(ctx, requireDevEnvironment)
	if err != nil {
		return err
	}
	defer c.Close()
	module := c.Seller
	stores := devSeedStores()
	// 刪帳申請由 accountdeletion 擁有：進行中的申請會把還原的店家再關掉、或在每小時的對帳刪掉 01-account.sql 重建的帳號（重建把 identity 的
	// PENDING_DELETION 悄悄重設，申請列還在），種子不繞過它（沒有取消申請的 API）。範圍是 01 重建的全部帳號，不只種子店家的擁有者；與店家的前提一起檢查、一起回報。
	checkAll := func() error {
		err := module.CheckDevSeed(ctx, stores, devSeedRebuiltAccounts)
		var accounts []int64
		for _, r := range devSeedRebuiltAccounts {
			for id := r.From; id <= r.To; id++ {
				accounts = append(accounts, id)
			}
		}
		open, derr := accountdeletion.NewReader(c.DB).AccountsWithOpenRequest(ctx, accounts)
		if derr != nil {
			return errors.Join(err, derr)
		}
		if len(open) > 0 {
			derr = fmt.Errorf("01-account.sql 重建的帳號 %v 有還沒完成的刪帳申請（accountdeletion）：種子不繞過它去還原或重建帳號，也沒有取消申請的 API；請重建本機資料庫", open)
		}
		return errors.Join(err, derr)
	}
	if checkOnly {
		if err := checkAll(); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, "種子的前提成立：重建的帳號區間沒有種子以外的未關閉店家，種子帳號沒有進行中的停權、關閉、等待刪除，重建的帳號沒有刪帳申請")
		return err
	}
	if err := checkAll(); err != nil {
		return err
	}
	if err := module.SeedDevStores(ctx, stores, devSeedRebuiltAccounts); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "已寫入 %d 家種子店家（接著執行 reannounce-exposures）\n", len(stores))
	return err
}
