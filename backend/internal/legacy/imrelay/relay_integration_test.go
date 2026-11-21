//go:build integration

package imrelay

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// imrelay 還沒有執行者（還沒接進 cmd/hawkr），所以它的正確性在這裡用真的 im 模組與真的資料庫第一次跑起來：終態、失敗路徑與重試。

type imBuyers struct{}

func (imBuyers) AccountIDByBuyerID(_ context.Context, id int64) (int64, error) { return 1000 + id, nil }
func (imBuyers) ProfileByBuyerID(context.Context, int64) (buyer.Profile, error) {
	return buyer.Profile{}, nil
}
func (imBuyers) IsBlockingStore(context.Context, int64, int64) (bool, error) { return false, nil }
func (imBuyers) LockNotClosed(context.Context, int64) error                  { return nil }

type imStores struct{}

func (imStores) Store(_ context.Context, id int64) (seller.Store, error) {
	st := seller.Store{
		StoreID: id, AccountID: 2000 + id, Status: seller.Active, Sanctions: seller.NoSanctions(),
		Conditions: seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true},
	}
	st.StoreName = "好茶舖"
	return st, nil
}
func (imStores) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) { return false, nil }
func (imStores) LockNotRetired(context.Context, int64) error                  { return nil }

type imOrders struct{}

func (imOrders) OpenOrderBetween(context.Context, int64, int64) (bool, error) { return false, nil }

const (
	buyerID = int64(11)
	storeID = int64(22)
)

type live struct {
	d      *db.DB
	module *im.Module
	bus    *events.Bus
	queue  *jobs.Client
	worker *Worker
	buyers *fakeBuyers
	stores *fakeStores
}

func setupLive(t *testing.T) *live {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "im", Dir: im.Migrations()},
	))
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	m := im.New(im.Deps{
		DB: d, Events: bus, Jobs: queue, Media: media.NewFake(), Buyers: imBuyers{}, Stores: imStores{}, Orders: imOrders{},
		NewID: uuid.NewV7, Now: time.Now,
	})
	events.Subscribe(bus, OnMessageSent(queue))
	l := &live{d: d, module: m, bus: bus, queue: queue,
		buyers: &fakeBuyers{display: []buyer.Display{{BuyerID: buyerID, DisplayName: ptr("小明")}}},
		stores: &fakeStores{store: seller.Store{StoreID: storeID, AccountID: 2000 + storeID}},
	}
	l.worker = NewWorker(m, l.buyers, l.stores, queue, nil)
	return l
}

func (l *live) count(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := l.d.Conn(context.Background()).QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return n
}

func (l *live) buyerSays(t *testing.T, text string) im.Message {
	t.Helper()
	m, err := l.module.Send(context.Background(), im.SendInput{Sender: im.Party{Kind: im.PartyBuyer, BuyerID: buyerID}, CounterpartID: storeID, Type: im.TypeText, Text: text})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func (l *live) relayJobs(t *testing.T) int64 {
	return l.count(t, "SELECT count(*) FROM river.river_job WHERE kind = 'legacy.im_message_sent'")
}

func (l *live) publishJobs(t *testing.T) int64 {
	return l.count(t, "SELECT count(*) FROM river.river_job WHERE kind = 'legacy.rabbitmq_publish'")
}

func (l *live) argsOf(m im.Message) MessageSentArgs {
	return MessageSentArgs{EventID: uuid.NewV7().String(), Occurred: time.Now(), MessageID: m.ID.String()}
}

// ID7：送出 commit 時剛好一個 relay job；transaction 回滾（發布事件之後的訂閱者失敗）時沒有 relay job、也沒有訊息。
// 把 MessageSent 改成 commit 之後才發時，回滾的情境會留下訊息，這個測試失敗。
func TestSendEnqueuesExactlyOneRelayJobAndRollsBackWithTheMessage(t *testing.T) {
	t.Parallel()
	l := setupLive(t)
	l.buyerSays(t, "hello")
	if n := l.relayJobs(t); n != 1 {
		t.Fatalf("relay job %d 個，預期 1", n)
	}

	events.Subscribe(l.bus, func(context.Context, im.MessageSent) error { return errors.New("subscriber failed") })
	_, err := l.module.Send(context.Background(), im.SendInput{Sender: im.Party{Kind: im.PartyBuyer, BuyerID: buyerID}, CounterpartID: storeID, Type: im.TypeText, Text: "rolled back"})
	if err == nil {
		t.Fatal("訂閱者失敗時送出要失敗")
	}
	if n := l.relayJobs(t); n != 1 {
		t.Fatalf("回滾的送出不能留下 relay job：%d", n)
	}
	if n := l.count(t, "SELECT count(*) FROM im.messages"); n != 1 {
		t.Fatalf("回滾的送出不能留下訊息：%d", n)
	}
	if n := l.count(t, "SELECT last_seq FROM im.conversations"); n != 1 {
		t.Fatalf("回滾的送出不能消耗 seq：%d", n)
	}
}

// 成功路徑與重試：relay job 在排入送出的 job 之後才失敗而重跑時，同一則事件只能有一個送出的 job（event_id unique）。
func TestRelayRetryEnqueuesOnePublishJobPerEvent(t *testing.T) {
	t.Parallel()
	l := setupLive(t)
	m := l.buyerSays(t, "hello")
	args := l.argsOf(m)
	for range 2 {
		if err := l.worker.messageSent(t.Context(), args); err != nil {
			t.Fatal(err)
		}
	}
	if n := l.publishJobs(t); n != 1 {
		t.Fatalf("publish job %d 個，預期 1", n)
	}
	var key, event string
	if err := l.d.Conn(t.Context()).QueryRow(t.Context(), "SELECT args->>'routing_key', args->>'event_id' FROM river.river_job WHERE kind = 'legacy.rabbitmq_publish'").Scan(&key, &event); err != nil {
		t.Fatal(err)
	}
	if key != "chat.message.sent" || event != args.EventID {
		t.Fatalf("publish = %s %s", key, event)
	}
}

// 終態：訊息在 job 執行前被抹除、隱藏，或 job 指到不存在的訊息，都不送、不報錯；失敗路徑：買家或店家查詢失敗回錯誤、不送。
func TestRelayTerminalAndFailurePathsAgainstARealDatabase(t *testing.T) {
	t.Parallel()
	t.Run("erased before the job runs", func(t *testing.T) {
		l := setupLive(t)
		m := l.buyerSays(t, "secret")
		if err := l.d.InTx(t.Context(), func(ctx context.Context) error {
			return events.Publish(ctx, l.bus, buyer.BuyerDeleted{EventID: uuid.NewV7(), OccurredAt: time.Now(), BuyerID: buyerID, AccountID: 1000 + buyerID})
		}); err != nil {
			t.Fatal(err)
		}
		if err := l.module.ErasePartyMessages(t.Context(), im.PartyBuyer, buyerID); err != nil {
			t.Fatal(err)
		}
		if err := l.worker.messageSent(t.Context(), l.argsOf(m)); err != nil {
			t.Fatal(err)
		}
		if n := l.publishJobs(t); n != 0 {
			t.Fatalf("已抹除的訊息不能送出：%d", n)
		}
	})
	t.Run("hidden before the job runs, restored afterwards", func(t *testing.T) {
		l := setupLive(t)
		m := l.buyerSays(t, "to be hidden")
		hide := im.HideInput{MessageID: m.ID, AdminID: 1, Category: string(policy.CommunityAssessable()[0])}
		if _, err := l.module.Hide(t.Context(), hide); err != nil {
			t.Fatal(err)
		}
		if err := l.worker.messageSent(t.Context(), l.argsOf(m)); err != nil {
			t.Fatal(err)
		}
		if n := l.publishJobs(t); n != 0 {
			t.Fatalf("隱藏的訊息不能送出：%d", n)
		}
		if _, err := l.module.Restore(t.Context(), im.RestoreInput{MessageID: m.ID, AdminID: 1, Reason: "誤判"}); err != nil {
			t.Fatal(err)
		}
		if err := l.worker.messageSent(t.Context(), l.argsOf(m)); err != nil {
			t.Fatal(err)
		}
		if n := l.publishJobs(t); n != 1 {
			t.Fatalf("還原後可以送出：%d", n)
		}
	})
	t.Run("message does not exist", func(t *testing.T) {
		l := setupLive(t)
		if err := l.worker.messageSent(t.Context(), MessageSentArgs{EventID: uuid.NewV7().String(), Occurred: time.Now(), MessageID: uuid.NewV7().String()}); err != nil {
			t.Fatal(err)
		}
		if n := l.publishJobs(t); n != 0 {
			t.Fatalf("不存在的訊息不能送出：%d", n)
		}
	})
	t.Run("buyer lookup fails", func(t *testing.T) {
		l := setupLive(t)
		m := l.buyerSays(t, "hello")
		l.buyers.err = errors.New("buyer down")
		if err := l.worker.messageSent(t.Context(), l.argsOf(m)); err == nil {
			t.Fatal("查詢失敗要回錯誤讓 River 重試")
		}
		if n := l.publishJobs(t); n != 0 {
			t.Fatalf("失敗時不能送出：%d", n)
		}
	})
	t.Run("store lookup fails", func(t *testing.T) {
		l := setupLive(t)
		m := l.buyerSays(t, "hello")
		l.stores.err = errors.New("seller down")
		if err := l.worker.messageSent(t.Context(), l.argsOf(m)); err == nil {
			t.Fatal("查詢失敗要回錯誤讓 River 重試")
		}
		if n := l.publishJobs(t); n != 0 {
			t.Fatalf("失敗時不能送出：%d", n)
		}
	})
}
