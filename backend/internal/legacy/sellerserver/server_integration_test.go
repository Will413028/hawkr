//go:build integration

package sellerserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"

	"connectrpc.com/connect"
	"github.com/riverqueue/river"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type noJobs struct{}

func (noJobs) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

func (noJobs) EnqueueReport(context.Context, river.JobArgs, *river.InsertOpts) (bool, error) {
	return true, nil
}

type noOrders struct{}

func (noOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []uuid.UUID) (map[uuid.UUID]int64, error) {
	return map[uuid.UUID]int64{}, nil
}

type fakeRatings struct {
	stats map[int64]RatingStats
	err   error
}

func (f fakeRatings) StoreRatingStats(context.Context, []int64) (map[int64]RatingStats, error) {
	return f.stats, f.err
}

type fakeFollowers struct {
	counts map[int64]int64
	err    error
}

func (f fakeFollowers) SubscriberCounts(_ context.Context, ids []int64) (map[int64]int64, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[int64]int64{}
	for _, id := range ids {
		out[id] = f.counts[id]
	}
	return out, nil
}

type slowRatings struct{ seen *ratingCall }

type ratingCall struct {
	hadDeadline atomic.Bool
	timedOut    atomic.Bool
}

func (r slowRatings) StoreRatingStats(ctx context.Context, _ []int64) (map[int64]RatingStats, error) {
	_, ok := ctx.Deadline()
	r.seen.hadDeadline.Store(ok)
	<-ctx.Done()
	r.seen.timedOut.Store(errors.Is(ctx.Err(), context.DeadlineExceeded))
	return nil, ctx.Err()
}

func (c *ratingCall) check(t *testing.T, what string) {
	t.Helper()
	if !c.hadDeadline.Load() || !c.timedOut.Load() {
		t.Fatalf("%s：評分沒有自己的期限（hadDeadline=%v timedOut=%v）", what, c.hadDeadline.Load(), c.timedOut.Load())
	}
}

func TestEveryLegacySellerRPCReadsTheModule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: inbox.Schema, Dir: inbox.Migrations()}, migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "payment", Dir: payment.Migrations()}, migrate.Module{Name: "seller", Dir: seller.Migrations()}))
	now := time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: func() time.Time { return now }, Logger: log})
	fake := media.NewFake()
	mac := sealtest.MAC(t)
	provider := payment.NewFake()
	provider.AutoActive = true
	var m *seller.Module
	clock := func() time.Time { return now }
	payments := payment.New(payment.Deps{
		DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: noJobs{}, Inbox: inbox.New(inbox.Deps{DB: d, Jobs: noJobs{}, Now: clock, Logger: log}), Provider: provider,
		Owners: ownerFunc(func(ctx context.Context, storeID int64) (bool, error) {
			s, err := m.Store(ctx, storeID)
			return err == nil && s.Status != seller.Closed, err
		}), Now: clock, Logger: log,
	})
	m = seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: noJobs{}, Identity: id, Media: fake, Orders: noOrders{}, KYCMAC: mac,
		Payment: payments, ProductStores: noProducts{}, Registry: noRegistry{},
		Now: func() time.Time { return now }, Logger: log,
	})
	events.Subscribe(bus, m.OnPayoutAccountChanged)
	account := func() int64 {
		var accountID int64
		for range 3 {
			if err := d.Conn(ctx).QueryRow(ctx, "INSERT INTO identity.accounts (self_entered_email) VALUES ('seller@example.com') RETURNING id").Scan(&accountID); err != nil {
				t.Fatal(err)
			}
		}
		return accountID
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	open := func(name string) (int64, seller.Store) {
		accountID := account()
		s, err := m.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: accountID, StoreName: name, ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路一號"})
		must(err)
		contact := func(v string) *string { return &v }
		s, err = m.UpdateProfile(ctx, seller.UpdateProfileInput{
			StoreID: s.StoreID, ExpectedVersion: s.Version,
			ContactName: contact("王小明"), ContactMobile: contact("0912345678"), SupportEmail: contact("owner@example.com"),
		})
		must(err)
		return accountID, s
	}
	exposable := func(name string) (int64, seller.Store) {
		accountID, s := open(name)
		str := func(v string) *string { return &v }
		kind := seller.KYCIndividual
		draft, err := m.SaveKYCDraft(ctx, seller.KYCDraftInput{
			AccountID: accountID, SellerType: &kind, ProductType: str("蔬果"),
		})
		must(err)
		sub, err := m.SubmitKYC(ctx, seller.SubmitKYCInput{AcceptedAgreements: seller.CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: draft.Version})
		must(err)
		_, err = m.ApproveKYC(ctx, seller.ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9})
		must(err)
		must(payments.EnsureAccount(ctx, s.StoreID, false))
		_, err = payments.SyncAccount(ctx, s.StoreID)
		must(err)
		cur, err := m.Store(ctx, s.StoreID)
		must(err)
		s, err = m.UpdateProfile(ctx, seller.UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, LogoToken: "logo", UploaderID: accountID})
		must(err)
		return accountID, s
	}

	exposedAccount, exposed := exposable("範例小農")
	_, pending := open("剛開店")
	suspendedAccount, suspended := exposable("被停權")
	review := now.Add(7 * 24 * time.Hour)
	riskSuspend := func(storeID int64, ground seller.RiskGround) seller.Measure {
		t.Helper()
		created, err := m.CreateMeasure(ctx, seller.CreateMeasureInput{
			AdminID: 7, StoreID: storeID, Kind: seller.MeasureSuspension, StatementToSeller: "風險", Note: "內部紀錄", ReviewBy: &review,
			Risk: &seller.RiskBasis{Ground: ground},
		})
		if err != nil {
			t.Fatal(err)
		}
		return created.Measure
	}
	riskSuspend(suspended.StoreID, seller.RiskFulfillment)
	_, deleting := exposable("等待刪除")
	if err := d.InTx(ctx, func(ctx context.Context) error { return m.BeginDeletion(ctx, deleting.StoreID) }); err != nil {
		t.Fatal(err)
	}
	deletedAccount, deleted := open("隨帳號刪除")
	if err := d.InTx(ctx, func(ctx context.Context) error {
		if err := m.BeginDeletion(ctx, deleted.StoreID); err != nil {
			return err
		}
		return m.CloseForDeletion(ctx, deleted.StoreID)
	}); err != nil {
		t.Fatal(err)
	}
	inheritedAccount, inheritedOld := exposable("舊店")
	inheritedMeasure := riskSuspend(inheritedOld.StoreID, seller.RiskSecurity)
	if _, err := m.CloseCurrent(ctx, inheritedAccount); err != nil {
		t.Fatal(err)
	}
	_, err := m.Apply(ctx, seller.ApplyInput{
		AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: inheritedAccount, StoreName: "新店", ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路二號",
	})
	var openingErr *apperr.Error
	if !errors.As(err, &openingErr) || openingErr.Kind() != apperr.KindFailedPrecondition || openingErr.Reason != apperr.ReasonStoreOpeningSuspended {
		t.Fatalf("apply during suspension: %v", err)
	}
	now = now.Add(time.Second)
	if _, err := m.EndMeasure(ctx, seller.EndMeasureInput{AdminID: 7, MeasureID: inheritedMeasure.ID, ExpectedVersion: inheritedMeasure.Version}); err != nil {
		t.Fatal(err)
	}
	inherited, err := m.Apply(ctx, seller.ApplyInput{
		AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: inheritedAccount, StoreName: "新店", ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路二號",
	})
	if err != nil {
		t.Fatal(err)
	}
	inheritedNew := inherited.StoreID
	_, governance := exposable("違規停權")
	ends := now.Add(30 * 24 * time.Hour)
	if _, err := m.CreateMeasure(ctx, seller.CreateMeasureInput{
		AdminID: 7, StoreID: governance.StoreID, Kind: seller.MeasureSuspension, StatementToSeller: "商品頁宣稱有機但沒有認證", Note: "內部紀錄", EndsAt: &ends,
		Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.MisleadingInfo, Severity: seller.SeverityHigh}},
	}); err != nil {
		t.Fatal(err)
	}
	closedAccount, closed := exposable("已關店")
	if _, err := m.CloseCurrent(ctx, closedAccount); err != nil {
		t.Fatal(err)
	}
	reopened, err := m.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: closedAccount, StoreName: "重新入駐", ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路二號"})
	if err != nil {
		t.Fatal(err)
	}

	s := New(m, fakeRatings{stats: map[int64]RatingStats{exposed.StoreID: {Count: 3, Average: 4.7727272727}}}, fakeFollowers{counts: map[int64]int64{exposed.StoreID: 5}}, log)
	sid := func(v int64) string { return strconv.FormatInt(v, 10) }

	t.Run("GetSeller", func(t *testing.T) {
		r, e := s.GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{StoreId: exposed.StoreID}))
		d := r.Msg.GetData()
		if e != nil || r.Msg.GetCode() != 0 || d.GetStoreId() != sid(exposed.StoreID) || d.GetAccountId() != exposedAccount || d.GetName() != "範例小農" ||
			d.GetLifecycleStatus() != "active" || d.GetRating() != 4.77 || d.GetCity() != "台北市" || d.GetDistrict() != "中正區" || d.GetPostalCode() != "100" {
			t.Fatalf("%v %v", r, e)
		}
		byAccount, e := s.GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{AccountId: exposedAccount}))
		if e != nil || byAccount.Msg.GetData().GetStoreId() != sid(exposed.StoreID) {
			t.Fatalf("%v %v", byAccount, e)
		}
		r, e = s.GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{StoreId: suspended.StoreID}))
		if e != nil || r.Msg.GetData().GetLifecycleStatus() != "suspended" || r.Msg.GetData().GetSuspensionReasonCode() != "9603" {
			t.Fatalf("%v %v", r, e)
		}
		r, e = s.GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{StoreId: closed.StoreID}))
		if e != nil || r.Msg.GetCode() != 404 || r.Msg.GetData() != nil {
			t.Fatalf("%v %v", r, e)
		}
		for _, accountID := range []int64{999999, 0, -1} {
			r, e = s.GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{AccountId: accountID}))
			if e != nil || r.Msg.GetCode() != 9233 {
				t.Fatalf("%d: %v %v", accountID, r, e)
			}
		}
		r, e = s.GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{StoreId: deleting.StoreID}))
		if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetData().GetLifecycleStatus() != "pending_deletion" {
			t.Fatalf("%v %v", r, e)
		}
		r, e = s.GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{StoreId: inheritedNew}))
		if e != nil || r.Msg.GetData().GetLifecycleStatus() != "active" || r.Msg.GetData().GetSuspensionReasonCode() != "" {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetSeller rating 是 best-effort", func(t *testing.T) {
		slowSeen := &ratingCall{}
		for name, ratings := range map[string]Ratings{"失敗": fakeRatings{err: errors.New("review down")}, "逾時": slowRatings{seen: slowSeen}} {
			r, e := New(m, ratings, fakeFollowers{}, log).GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{StoreId: exposed.StoreID}))
			if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetData().GetStoreId() != sid(exposed.StoreID) || r.Msg.GetData().GetRating() != 0 {
				t.Fatalf("%s：%v %v", name, r, e)
			}
		}
		slowSeen.check(t, "GetSeller")
	})
	t.Run("GetSellerBatch", func(t *testing.T) {
		r, e := s.GetSellerBatch(ctx, connect.NewRequest(&pb.GetSellerBatchRequest{StoreIds: []int64{closed.StoreID, exposed.StoreID, 999999}}))
		if e != nil || len(r.Msg.GetData()) != 2 || r.Msg.GetData()[0].GetStoreId() != sid(closed.StoreID) || r.Msg.GetData()[0].GetLifecycleStatus() != "closed" ||
			r.Msg.GetData()[1].GetStoreId() != sid(exposed.StoreID) || r.Msg.GetData()[1].GetStoreCode() != sid(exposed.StoreCode) {
			t.Fatalf("%v %v", r, e)
		}
		if r.Msg.GetData()[1].GetRating() != 0 {
			t.Fatalf("rating=%v", r.Msg.GetData()[1].GetRating())
		}
		failing, e := New(m, fakeRatings{err: errors.New("review down")}, fakeFollowers{}, log).GetSellerBatch(ctx, connect.NewRequest(&pb.GetSellerBatchRequest{StoreIds: []int64{exposed.StoreID}}))
		if e != nil || len(failing.Msg.GetData()) != 1 {
			t.Fatalf("%v %v", failing, e)
		}
		if r, e := s.GetSellerBatch(ctx, connect.NewRequest(&pb.GetSellerBatchRequest{StoreIds: []int64{0, -5, exposed.StoreID}})); e != nil || r.Msg.GetCode() != 0 || len(r.Msg.GetData()) != 1 {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("BatchGetSellerMeta", func(t *testing.T) {
		r, e := s.BatchGetSellerMeta(ctx, connect.NewRequest(&pb.BatchGetSellerMetaRequest{StoreIds: []int64{exposed.StoreID, pending.StoreID, suspended.StoreID, closed.StoreID}}))
		if e != nil || len(r.Msg.GetSellers()) != 4 {
			t.Fatalf("%v %v", r, e)
		}
		type truth struct{ manage, accept, card bool }
		for storeID, want := range map[int64]truth{
			exposed.StoreID: {true, true, true}, pending.StoreID: {true, false, false}, suspended.StoreID: {false, false, true}, closed.StoreID: {false, false, true},
		} {
			got := r.Msg.GetSellers()[storeID]
			if got.GetCanManageProducts() != want.manage || got.GetCanAcceptOrders() != want.accept || got.GetHasPublicCard() != want.card {
				t.Errorf("store %d: manage=%v accept=%v card=%v, want %+v", storeID, got.GetCanManageProducts(), got.GetCanAcceptOrders(), got.GetHasPublicCard(), want)
			}
		}
		e1 := r.Msg.GetSellers()[exposed.StoreID]
		if e1.GetPayuniMerId() != "" || e1.GetZipCode() != "100" || e1.GetAccountId() != exposedAccount || e1.GetSellerName() != "範例小農" {
			t.Fatalf("%v", e1)
		}
	})
	t.Run("ResolvePrincipals", func(t *testing.T) {
		r, e := s.ResolvePrincipals(ctx, connect.NewRequest(&pb.ResolvePrincipalsRequest{Ids: []int64{exposed.StoreID, closed.StoreID, 999999}}))
		if e != nil || r.Msg.GetPrincipals()[exposed.StoreID] != exposedAccount || len(r.Msg.GetPrincipals()) != 1 || !slices.Equal(r.Msg.GetUnresolvedIds(), []int64{closed.StoreID, 999999}) {
			t.Fatalf("%v %v", r, e)
		}
		for _, id := range []int64{closed.StoreID, 999999} {
			if v, ok := r.Msg.GetPrincipals()[id]; ok {
				t.Fatalf("unresolved id %d 出現在 principals：%d", id, v)
			}
		}
		r, e = s.ResolvePrincipals(ctx, connect.NewRequest(&pb.ResolvePrincipalsRequest{Ids: []int64{deleted.StoreID, 0}}))
		if e != nil || !slices.Equal(r.Msg.GetUnresolvedIds(), []int64{deleted.StoreID, 0}) || len(r.Msg.GetPrincipals()) != 0 {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetSellerShippingProfile", func(t *testing.T) {
		r, e := s.GetSellerShippingProfile(ctx, connect.NewRequest(&pb.GetSellerShippingProfileRequest{StoreId: exposed.StoreID}))
		d := r.Msg.GetData()
		if e != nil || d.GetContactEmail() != "owner@example.com" || d.GetZipCode() != "100" || d.GetMerId() != "" || d.GetSenderPhone() != "0912345678" {
			t.Fatalf("%v %v", r, e)
		}
		r, e = s.GetSellerShippingProfile(ctx, connect.NewRequest(&pb.GetSellerShippingProfileRequest{StoreId: closed.StoreID}))
		if e != nil || r.Msg.GetCode() != 404 || r.Msg.GetData() != nil {
			t.Fatalf("closed: %v %v", r, e)
		}
		r, e = s.GetSellerShippingProfile(ctx, connect.NewRequest(&pb.GetSellerShippingProfileRequest{StoreId: pending.StoreID}))
		if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetData().GetContactEmail() != "owner@example.com" {
			t.Fatalf("%v %v", r, e)
		}
		noPhone, err := m.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: account(), StoreName: "沒填電話", TownCode: "63000050", AddressDetail: "測試路三號"})
		must(err)
		email := "nophone@example.com"
		_, err = m.UpdateProfile(ctx, seller.UpdateProfileInput{StoreID: noPhone.StoreID, ExpectedVersion: noPhone.Version, SupportEmail: &email})
		must(err)
		r, e = s.GetSellerShippingProfile(ctx, connect.NewRequest(&pb.GetSellerShippingProfileRequest{StoreId: noPhone.StoreID}))
		if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetData().GetContactEmail() != email || r.Msg.GetData().GetSenderPhone() != "" || r.Msg.GetData().GetZipCode() != "100" {
			t.Fatalf("no phone: %v %v", r, e)
		}
	})
	t.Run("MatchCertificateHolder", func(t *testing.T) {
		companyName, taxID, companyKind := "範例小農有限公司", "33330333", seller.KYCCompany
		companyAccount, company := open("公司店")
		companyDraft, err := m.SaveKYCDraft(ctx, seller.KYCDraftInput{AccountID: companyAccount, SellerType: &companyKind, CompanyName: &companyName, TaxID: &taxID})
		must(err)
		companySub, err := m.SubmitKYC(ctx, seller.SubmitKYCInput{AcceptedAgreements: seller.CurrentKYCAcceptances(), AccountID: companyAccount, ExpectedVersion: companyDraft.Version})
		must(err)
		_, err = m.ApproveKYC(ctx, seller.ApproveKYCInput{ApplicationID: companySub.ID, ExpectedVersion: companySub.Version, AdminID: 9})
		must(err)
		r, e := s.MatchCertificateHolder(ctx, connect.NewRequest(&pb.MatchCertificateHolderRequest{StoreId: company.StoreID, HolderName: " 範例 小農有限公司 "}))
		if e != nil || !r.Msg.GetMatched() || r.Msg.GetMatchedField() != "COMPANY_NAME" || !r.Msg.GetHasReference() {
			t.Fatalf("%v %v", r, e)
		}
		r, e = s.MatchCertificateHolder(ctx, connect.NewRequest(&pb.MatchCertificateHolderRequest{StoreId: company.StoreID, HolderName: "別人有限公司"}))
		if e != nil || r.Msg.GetMatched() || r.Msg.GetMatchedField() != "NONE" || !r.Msg.GetHasReference() {
			t.Fatalf("%v %v", r, e)
		}
		for name, id := range map[string]int64{"個人賣家": exposed.StoreID, "沒有申請": pending.StoreID} {
			r, e = s.MatchCertificateHolder(ctx, connect.NewRequest(&pb.MatchCertificateHolderRequest{StoreId: id, HolderName: "王小明"}))
			if e != nil || r.Msg.GetMatched() || r.Msg.GetHasReference() {
				t.Fatalf("%s：%v %v", name, r, e)
			}
		}
	})
	t.Run("ListPublicStoreIDsByCity", func(t *testing.T) {
		for _, input := range []string{"台北市", "臺北市", "tw-tpe"} {
			r, e := s.ListPublicStoreIDsByCity(ctx, connect.NewRequest(&pb.ListPublicStoreIDsByCityRequest{City: input}))
			if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetCity() != "台北市" || r.Msg.GetCityCode() != "TW-TPE" || !slices.Equal(r.Msg.GetStoreIds(), []string{sid(exposed.StoreID)}) {
				t.Fatalf("%q: %v %v", input, r, e)
			}
		}
		if r, e := s.ListPublicStoreIDsByCity(ctx, connect.NewRequest(&pb.ListPublicStoreIDsByCityRequest{City: "火星"})); e != nil || r.Msg.GetCode() != 400 {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("ListPublicStoresByCity", func(t *testing.T) {
		r, e := s.ListPublicStoresByCity(ctx, connect.NewRequest(&pb.ListPublicStoresByCityRequest{}))
		if e != nil || len(r.Msg.GetCities()) != 22 {
			t.Fatalf("%v %v", r, e)
		}
		for _, c := range r.Msg.GetCities() {
			want := 0
			if c.GetCityCode() == "TW-TPE" {
				want = 1
			}
			if len(c.GetStoreIds()) != want || len(c.GetPreviewStoreIds()) != want {
				t.Errorf("%s: %v", c.GetCityCode(), c)
			}
		}
	})
	t.Run("GetSeller 認定停權", func(t *testing.T) {
		r, e := s.GetSeller(ctx, connect.NewRequest(&pb.GetSellerRequest{StoreId: governance.StoreID}))
		if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetData().GetLifecycleStatus() != "suspended_governance" || r.Msg.GetData().GetSuspensionReasonCode() != "" {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("GetSellerStats", func(t *testing.T) {
		if r, e := s.GetSellerStats(ctx, connect.NewRequest(&pb.GetSellerStatsRequest{StoreId: closed.StoreID})); e != nil || r.Msg.GetCode() != 404 || r.Msg.GetData() != nil {
			t.Fatalf("closed: %v %v", r, e)
		}
		if r, e := s.GetSellerStats(ctx, connect.NewRequest(&pb.GetSellerStatsRequest{AccountId: 0})); e != nil || r.Msg.GetCode() != 404 {
			t.Fatalf("account 0: %v %v", r, e)
		}
		r, e := s.GetSellerStats(ctx, connect.NewRequest(&pb.GetSellerStatsRequest{StoreId: exposed.StoreID}))
		d := r.Msg.GetData()
		if e != nil || d.GetStoreId() != sid(exposed.StoreID) || d.GetRating() != 4.77 || d.GetReviewCount() != 3 || d.GetFollowers() != 5 {
			t.Fatalf("%v %v", r, e)
		}
		noRatings := New(m, fakeRatings{err: errors.New("review down")}, fakeFollowers{counts: map[int64]int64{exposed.StoreID: 5}}, log)
		r, e = noRatings.GetSellerStats(ctx, connect.NewRequest(&pb.GetSellerStatsRequest{StoreId: exposed.StoreID}))
		if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetData().GetFollowers() != 5 || r.Msg.GetData().GetRating() != 0 || r.Msg.GetData().GetReviewCount() != 0 {
			t.Fatalf("評價失敗：%v %v", r, e)
		}
		slowSeen := &ratingCall{}
		slow := New(m, slowRatings{seen: slowSeen}, fakeFollowers{counts: map[int64]int64{exposed.StoreID: 5}}, log)
		r, e = slow.GetSellerStats(ctx, connect.NewRequest(&pb.GetSellerStatsRequest{StoreId: exposed.StoreID}))
		if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetData().GetFollowers() != 5 || r.Msg.GetData().GetRating() != 0 || r.Msg.GetData().GetReviewCount() != 0 {
			t.Fatalf("評價逾時：%v %v", r, e)
		}
		slowSeen.check(t, "GetSellerStats")
		noFollowers := New(m, fakeRatings{stats: map[int64]RatingStats{exposed.StoreID: {Count: 3, Average: 4.5}}}, fakeFollowers{err: errors.New("buyer down")}, log)
		r, e = noFollowers.GetSellerStats(ctx, connect.NewRequest(&pb.GetSellerStatsRequest{StoreId: exposed.StoreID}))
		if e != nil || r.Msg.GetCode() != 0 || r.Msg.GetData().GetFollowers() != 0 || r.Msg.GetData().GetRating() != 4.5 || r.Msg.GetData().GetReviewCount() != 3 {
			t.Fatalf("粉絲數失敗：%v %v", r, e)
		}
	})
	t.Run("SearchSellerProspects", func(t *testing.T) {
		p, err := m.CreateProspect(ctx, seller.CreateProspectInput{AdminID: 1, Fields: seller.ProspectFields{DisplayName: "王記農場", VendorType: seller.VendorBusiness, Category: new("蔬果")}})
		if err != nil {
			t.Fatal(err)
		}
		r, e := s.SearchSellerProspects(ctx, connect.NewRequest(&pb.SearchSellerProspectsRequest{Category: "蔬果"}))
		if e != nil || len(r.Msg.GetData()) != 1 || r.Msg.GetData()[0].GetProspect().GetId() != sid(p.ID) || r.Msg.GetData()[0].GetProspect().GetVendorType() != "business" || !r.Msg.GetData()[0].GetHitCategory() {
			t.Fatalf("%v %v", r, e)
		}
	})
	t.Run("ListStoreIDsByAccount", func(t *testing.T) {
		r, e := s.ListStoreIDsByAccount(ctx, connect.NewRequest(&pb.ListStoreIDsByAccountRequest{AccountId: closedAccount}))
		if e != nil || !slices.Equal(r.Msg.GetStoreIds(), []int64{closed.StoreID, reopened.StoreID}) {
			t.Fatalf("%v %v", r, e)
		}
		for _, accountID := range []int64{999999, 0} {
			r, e = s.ListStoreIDsByAccount(ctx, connect.NewRequest(&pb.ListStoreIDsByAccountRequest{AccountId: accountID}))
			if e != nil || r.Msg.GetCode() != 0 || len(r.Msg.GetStoreIds()) != 0 {
				t.Fatalf("%d: %v %v", accountID, r, e)
			}
		}
	})
	t.Run("ResolveSellerAccountID", func(t *testing.T) {
		r, e := s.ResolveSellerAccountID(ctx, connect.NewRequest(&pb.ResolveSellerAccountIDRequest{StoreId: suspended.StoreID}))
		if e != nil || r.Msg.GetAccountId() != suspendedAccount {
			t.Fatalf("%v %v", r, e)
		}
		if r, e := s.ResolveSellerAccountID(ctx, connect.NewRequest(&pb.ResolveSellerAccountIDRequest{StoreId: deleted.StoreID})); e != nil || r.Msg.GetAccountId() != deletedAccount {
			t.Fatalf("%v %v", r, e)
		}
		for _, id := range []int64{999999, 0} {
			if _, e := s.ResolveSellerAccountID(ctx, connect.NewRequest(&pb.ResolveSellerAccountIDRequest{StoreId: id})); connect.CodeOf(e) != connect.CodeNotFound {
				t.Fatalf("%d: %v", id, e)
			}
		}
	})
	t.Run("GetCurrentStoreID", func(t *testing.T) {
		r, e := s.GetCurrentStoreID(ctx, connect.NewRequest(&pb.GetCurrentStoreIDRequest{AccountId: closedAccount}))
		if e != nil || r.Msg.GetStoreId() != reopened.StoreID {
			t.Fatalf("%v %v", r, e)
		}
		for _, accountID := range []int64{999999, 0} {
			if _, e := s.GetCurrentStoreID(ctx, connect.NewRequest(&pb.GetCurrentStoreIDRequest{AccountId: accountID})); connect.CodeOf(e) != connect.CodeNotFound {
				t.Fatalf("%d: %v", accountID, e)
			}
		}
	})
}

type ownerFunc func(ctx context.Context, storeID int64) (bool, error)

func (f ownerFunc) OwnerActive(ctx context.Context, storeID int64) (bool, error) {
	return f(ctx, storeID)
}

func (ownerFunc) OwnerContactEmail(context.Context, int64) (string, bool, error) {
	return "owner@example.com", true, nil
}
