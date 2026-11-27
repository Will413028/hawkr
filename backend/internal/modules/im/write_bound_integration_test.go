//go:build integration

package im_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
)

// 寫入 transaction 有時間上限：別的 transaction 持有該路徑要等的鎖時，每一條頂層寫入路徑都在鎖上等到上限，
// 以逾時錯誤失敗，而不是無限等。
func TestWriteTxBeyondTheBoundFails(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	sent := f.say(t, f.buyerParty(b1), s1, "hello")
	conv, msg := sent.ConversationID, sent.ID
	store := f.storeParty(t, s1)
	buyer := f.buyerParty(b1)
	lockConversation := `SELECT 1 FROM im.conversations WHERE id = $1 FOR UPDATE`
	lockMessage := `SELECT 1 FROM im.messages WHERE id = $1 FOR UPDATE`
	lockBuyer := `SELECT 1 FROM buyer.buyers WHERE buyer_id = $1 FOR UPDATE`
	cases := []struct {
		name string
		lock string
		arg  any
		call func() error
	}{
		{"Send", lockBuyer, b1, func() error {
			_, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: buyer, CounterpartID: s1, Type: im.TypeText, Text: "blocked"})
			return err
		}},
		{"Hide", lockMessage, msg, func() error { _, err := f.m.Hide(ctx, f.hideInput(msg)); return err }},
		{"Restore", lockMessage, msg, func() error {
			_, err := f.m.Restore(ctx, im.RestoreInput{MessageID: msg, AdminID: 1, Reason: "restored"})
			return err
		}},
		{"MarkRead", lockConversation, conv, func() error { return f.m.MarkRead(ctx, conv, store, 1) }},
		{"MarkAllRead", lockConversation, conv, func() error { return f.m.MarkAllRead(ctx, store) }},
	}
	for _, c := range cases {
		locked := make(chan struct{})
		release := make(chan struct{})
		holder := make(chan error, 1)
		go func() {
			holder <- f.db.InTx(ctx, func(ctx context.Context) error {
				if _, err := f.db.Conn(ctx).Exec(ctx, c.lock, c.arg); err != nil {
					return err
				}
				close(locked)
				<-release
				return nil
			})
		}()
		select {
		case <-locked:
		case err := <-holder:
			t.Fatalf("%s：持鎖失敗：%v", c.name, err)
		}
		done := make(chan error, 1)
		go func() { done <- c.call() }()
		var err error
		select {
		case err = <-done:
		case <-time.After(guard):
			close(release)
			if herr := <-holder; herr != nil {
				t.Error(herr)
			}
			t.Fatalf("%s 在鎖上等，沒有被時間上限中止", c.name)
		}
		close(release)
		if herr := <-holder; herr != nil {
			t.Fatal(herr)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "57014" && pgErr.Code != "25P04") {
			t.Fatalf("%s：預期 statement_timeout（57014）或 transaction_timeout（25P04），得到 %v", c.name, err)
		}
	}
}

// 開始早於 lastOK、提交晚於斷線的送出：其 last_message_at 小於重連的 lastOK，補洞起點往回放 RecoveryMargin 才補得到。
func TestSendStraddlingLastOKIsResyncedOnReconnect(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	gate := make(chan struct{})
	slept := make(chan struct{}, 16)
	cfg := fastBackoff()
	cfg.Sleep = func(ctx context.Context, _ time.Duration) error {
		slept <- struct{}{}
		select {
		case <-gate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	sent := make(chan im.Message, 1)
	commit := make(chan struct{})
	holder := make(chan error, 1)
	go func() {
		holder <- f.db.InTx(ctx, func(ctx context.Context) error {
			m, err := f.m.Send(ctx, im.SendInput{RequestID: uuid.NewV4(), Sender: f.buyerParty(b1), CounterpartID: s1, Type: im.TypeText, Text: "straddles"})
			if err != nil {
				return err
			}
			sent <- m
			<-commit
			return nil
		})
	}()
	var msg im.Message
	select {
	case msg = <-sent:
	case <-time.After(guard):
		t.Fatal("送出沒有完成")
	}
	m := f.newModule(cfg)
	f.run(t, m)
	sub := f.subscribe(t, m, f.storeParty(t, s1), nil)
	untilReady(t, sub)
	f.exec(t, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid() AND query LIKE 'SELECT now()%'`)
	<-slept
	close(commit)
	if err := <-holder; err != nil {
		t.Fatal(err)
	}
	close(gate)
	evs := collectUntil(t, sub, func(e im.Event) bool { return e.Kind == im.EventResync })
	last := evs[len(evs)-1]
	if last.All || last.ConversationID != msg.ConversationID {
		t.Fatalf("預期 Resync{%v}，得到 %+v", msg.ConversationID, last)
	}
}
