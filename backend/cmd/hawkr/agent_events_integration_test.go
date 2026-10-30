//go:build integration

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

// prospect 的封存與合併由 seller 在自己的 transaction 內發事件，agent 在同一個 transaction 內撤銷該 prospect 的邀請與 session
// （接線在 subscribeAgent；ADR 2026-10-03-vendor-portal-invite-scoped-access「撤銷」）。用真的 seller 與 agent 驗證整條路徑，
// 包含封存之後還原不會讓邀請復活、合併把輸家的邀請改掛贏家（不撤銷），以及 Ensure 與封存併發時不會留下封存業者的有效邀請。
func TestProspectArchiveAndMergeRevokeVendorPortalAccess(t *testing.T) {
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "seller", Dir: seller.Migrations()}, migrate.Module{Name: "agent", Dir: agent.Migrations()},
	))
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	accounts := identity.New(identity.Deps{DB: d, Events: bus, Now: clock, Logger: logger})
	kycKeys, kycMAC := sealtest.Keys(t)
	sellers := seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: accounts, Media: noMedia{}, Orders: noOrders{}, ProductStores: &productStoresStub{}, Registry: noRegistry{},
		KYCKeys: kycKeys, KYCMAC: kycMAC, Now: clock, Logger: logger,
	})
	agents := agent.New(agent.Deps{DB: d, Prospects: sellers, Now: clock})
	subscribeAgent(bus, agents)

	const admin = int64(9001)
	newProspect := func(name, email string) int64 {
		p, err := sellers.CreateProspect(ctx, seller.CreateProspectInput{
			AdminID: admin, Fields: seller.ProspectFields{DisplayName: name, VendorType: seller.VendorBusiness},
			Contacts: []seller.ContactPoint{{Kind: seller.ContactEmail, Value: email, Primary: true}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return p.ID
	}
	version := func(id int64) int64 {
		p, err := sellers.Prospect(ctx, id)
		if err != nil {
			t.Error(err)
		}
		return p.Version
	}
	archive := func(id int64) error {
		_, err := sellers.ArchiveProspect(ctx, seller.ArchiveProspectInput{ProspectID: id, AdminID: admin, ExpectedVersion: version(id)})
		return err
	}
	login := func(prospect, recipient int64) (string, string) {
		inv, err := agents.EnsureInvite(ctx, agent.EnsureInviteInput{ProspectID: prospect, RfqRecipientID: recipient, ExpiresAt: now.Add(24 * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		s, err := agents.ExchangeInvite(ctx, inv.Secret)
		if err != nil {
			t.Fatal(err)
		}
		return inv.Secret, s.Token
	}
	usable := func(secret, token string) bool {
		_, e1 := agents.PreviewInvite(ctx, secret)
		_, e2 := agents.ValidateSession(ctx, token)
		if (e1 == nil) != (e2 == nil) {
			t.Fatalf("邀請與 session 的狀態不一致：preview=%v validate=%v", e1, e2)
		}
		return e1 == nil
	}

	archived := newProspect("封存的業者", "a@example.com")
	loser := newProspect("合併的輸家", "l@example.com")
	winner := newProspect("合併的贏家", "w@example.com")
	aSecret, aToken := login(archived, 1)
	lSecret, lToken := login(loser, 2)
	wSecret, wToken := login(winner, 3)

	if err := archive(archived); err != nil {
		t.Fatal(err)
	}
	if usable(aSecret, aToken) || !usable(lSecret, lToken) || !usable(wSecret, wToken) {
		t.Fatal("封存只撤銷那個 prospect 的邀請")
	}
	// 還原不讓邀請復活：撤銷是不可逆的事實，要重新邀請得走新的 RFQ。
	if _, err := sellers.RestoreProspect(ctx, seller.RestoreProspectInput{ProspectID: archived, AdminID: admin}); err != nil {
		t.Fatal(err)
	}
	if usable(aSecret, aToken) {
		t.Fatal("還原不得讓撤銷的邀請復活")
	}
	if _, err := sellers.MergeProspects(ctx, seller.MergeProspectsInput{WinnerID: winner, LoserID: loser, AdminID: admin, WinnerVersion: version(winner), LoserVersion: version(loser)}); err != nil {
		t.Fatal(err)
	}
	// 合併不撤銷：輸家的邀請改掛贏家，連結與 session 照常可用。
	if !usable(lSecret, lToken) || !usable(wSecret, wToken) {
		t.Fatal("合併不得撤銷邀請")
	}
	var onWinner int64
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM agent.vendor_portal_invites WHERE prospect_id = $1 AND revoked_at IS NULL", winner).Scan(&onWinner); err != nil || onWinner != 2 {
		t.Fatalf("贏家名下的有效邀請 = %d err=%v，預期 2（自己的與改掛的）", onWinner, err)
	}
	if _, err := agents.ExchangeInvite(ctx, lSecret); err != nil {
		t.Fatal(err)
	}
	// 輸家已經封存：用輸家的 ID 重取邀請被拒絕（AG1），不會在輸家名下長出新邀請。
	_, err = agents.EnsureInvite(ctx, agent.EnsureInviteInput{ProspectID: loser, RfqRecipientID: 4, ExpiresAt: now.Add(time.Hour)})
	if !errors.Is(err, agent.ErrProspectNotActive) {
		t.Fatalf("err = %v", err)
	}
	// 贏家封存時，改掛過來的邀請一併撤銷。
	if err := archive(winner); err != nil {
		t.Fatal(err)
	}
	if usable(lSecret, lToken) || usable(wSecret, wToken) {
		t.Fatal("贏家封存要撤銷改掛過來的邀請")
	}

	// Ensure 與封存併發：不論誰先到，封存之後都不會有未撤銷的邀請（Ensure 以共享鎖讀 prospect，與封存序列化）。
	for round := range int64(25) {
		p := newProspect("併發的業者", "race@example.com")
		recipient := 1000 + round
		var ensureErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, ensureErr = agents.EnsureInvite(ctx, agent.EnsureInviteInput{ProspectID: p, RfqRecipientID: recipient, ExpiresAt: now.Add(time.Hour)})
		}()
		go func() {
			defer wg.Done()
			if err := archive(p); err != nil {
				t.Error(err)
			}
		}()
		wg.Wait()
		if ensureErr != nil && !errors.Is(ensureErr, agent.ErrProspectNotActive) {
			t.Fatalf("round %d：ensure = %v", round, ensureErr)
		}
		var live int64
		if err := d.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM agent.vendor_portal_invites WHERE prospect_id = $1 AND revoked_at IS NULL", p).Scan(&live); err != nil || live != 0 {
			t.Fatalf("round %d：封存的業者有 %d 張未撤銷的邀請（err=%v，ensure=%v）", round, live, err, ensureErr)
		}
	}
	if err := archive(archived); err != nil {
		t.Fatal(err)
	}
	_, err = agents.EnsureInvite(ctx, agent.EnsureInviteInput{ProspectID: archived, RfqRecipientID: 5, ExpiresAt: now.Add(time.Hour)})
	if !errors.Is(err, agent.ErrProspectNotActive) {
		t.Fatalf("封存之後 Ensure：err = %v", err)
	}
}
