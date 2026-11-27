package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

type sellerCommand struct {
	DB       *db.DB
	Bus      *events.Bus
	Jobs     jobs.Enqueuer
	Identity *identity.Module
	Seller   *seller.Module
	Owners   *sellerOwners
	Logger   *slog.Logger
}

func openSeller(ctx context.Context, prepare func(*config.Env) error) (*sellerCommand, error) {
	e := config.NewEnv()
	if prepare != nil {
		if err := prepare(e); err != nil {
			return nil, err
		}
	}
	dbc := dbConfig(e)
	internalSecret := e.StringOr("INTERNAL_SERVICE_HMAC_SECRET", "")
	orderAddr := e.String("ORDER_SERVICE_ADDR")
	log := newLogger()
	kycMAC := readKYCHashKeys(e)
	paymentProvider := readPayment(e, log)
	registry := readGCIS(e)
	mediaCfg := readMedia(e)
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
	med, err := newMedia(mediaCfg, d, jc, log)
	if err != nil {
		d.Close()
		return nil, err
	}
	orders, err := orderservice.New(orderAddr, internalSecret)
	if err != nil {
		d.Close()
		return nil, err
	}
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	catalogModule := newCatalog(d, bus, med, buyer.New(buyer.Deps{DB: d, Events: bus, Now: time.Now, Logger: log}))
	payments, owners := newPayment(d, bus, jc, newInbox(d, jc, log), paymentProvider, nil, nil, log)
	module := newSeller(d, bus, jc, id, med, orders, sellerProducts{catalog: catalogModule}, kycMAC, payments, registry, log)
	owners.bind(module)
	subscribeSellerPayout(bus, module)
	subscribeSellerRelay(bus, jc)
	return &sellerCommand{DB: d, Bus: bus, Jobs: jc, Identity: id, Seller: module, Owners: owners, Logger: log}, nil
}

// Close 關閉資料庫連線。
func (c *sellerCommand) Close() { c.DB.Close() }

func openSellerCommand(ctx context.Context, prepare func(*config.Env) error) (*seller.Module, func(), error) {
	c, err := openSeller(ctx, prepare)
	if err != nil {
		return nil, nil, err
	}
	return c.Seller, c.Close, nil
}

const reannounceUsage = "用法：reannounce-exposures（不帶參數）"

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
	checkAll := func() error {
		err := module.CheckDevSeed(ctx, stores, devSeedAccountIDs)
		open, derr := accountdeletion.NewReader(c.DB).AccountsWithOpenRequest(ctx, devSeedAccountIDs)
		if derr != nil {
			return errors.Join(err, derr)
		}
		if len(open) > 0 {
			derr = fmt.Errorf("01-account.sql 的種子帳號 %v 有還沒完成的刪帳申請（accountdeletion）：種子不繞過它去還原帳號，也沒有取消申請的 API；請重建本機資料庫", open)
		}
		return errors.Join(err, derr)
	}
	if checkOnly {
		if err := checkAll(); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, "種子的前提成立：種子帳號名下沒有種子以外的未關閉店家，種子帳號沒有進行中的停權、關閉、等待刪除，也沒有刪帳申請")
		return err
	}
	if err := checkAll(); err != nil {
		return err
	}
	if err := module.SeedDevStores(ctx, stores, devSeedAccountIDs); err != nil {
		return err
	}
	ids := make([]int64, 0, len(stores))
	for _, st := range stores {
		ids = append(ids, st.StoreID)
	}
	seedFake := payment.NewFake()
	seedFake.AutoActive = true
	if err := seedPaymentAccounts(ctx, c, seedFake, ids, out); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "已寫入 %d 家種子店家（接著執行 reannounce-exposures）\n", len(stores))
	return err
}
