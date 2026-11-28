//go:build integration

package sellerside_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/sellerside"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var start = time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)

const (
	buyerID           int64 = 1
	invitedStore      int64 = 10
	otherInvitedStore int64 = 11
	outsider          int64 = 99
)

func ptr[T any](v T) *T { return &v }

type fakeStores struct {
	mu        sync.Mutex
	suspended bool
	retired   bool
}

func (f *fakeStores) set(suspended, retired bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.suspended, f.retired = suspended, retired
}

func (f *fakeStores) LockNotRetired(_ context.Context, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.retired {
		return apperr.NotFound("店家已退場")
	}
	return nil
}

func (f *fakeStores) Store(_ context.Context, id int64) (seller.Store, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := seller.Store{StoreID: id, Status: seller.Active, Conditions: seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true}}
	if !f.suspended {
		st.Sanctions = seller.NoSanctions()
	}
	return st, nil
}

type fixture struct {
	t           *testing.T
	d           *db.DB
	jobs        *jobs.Client
	bus         *events.Bus
	stores      *fakeStores
	now         time.Time
	buyer       int64
	request     uuid.UUID
	revision    uuid.UUID
	publication uuid.UUID
	hash        string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "procurement", Dir: procurement.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, d: d, jobs: jc, bus: events.NewBus(), stores: &fakeStores{}, now: start}
	f.adopt(buyerID)
	return f
}

func (f *fixture) sibling(buyer int64, at time.Time) *fixture {
	g := &fixture{t: f.t, d: f.d, jobs: f.jobs, bus: f.bus, stores: f.stores, now: at}
	g.adopt(buyer)
	return g
}

func (f *fixture) adopt(buyer int64) {
	ctx := context.Background()
	f.buyer, f.request, f.revision = buyer, uuid.NewV7(), uuid.NewV7()
	q := store.New(f.d.Conn(ctx))
	must(f.t, q.InsertRequest(ctx, store.InsertRequestParams{ID: f.request, BuyerID: &buyer, Now: f.now}))
	must(f.t, q.InsertConversation(ctx, store.InsertConversationParams{ID: uuid.NewV7(), BuyerID: buyer, RequestID: f.request, Now: f.now}))
	if n, err := q.InsertRevision(ctx, store.InsertRevisionParams{
		ID: f.revision, RevisionNo: 1, Item: ptr("小白菜"), Quantity: ptr(int32(3)), Unit: ptr("KG"), DeliveryCity: ptr("台北市"),
		Now: f.now, RequestID: f.request, BuyerID: &buyer,
	}); err != nil || n != 1 {
		f.t.Fatalf("InsertRevision n=%d err=%v", n, err)
	}
	f.exec(`UPDATE procurement.requests SET status = 'REQUIREMENT_READY' WHERE id = $1`, f.request)
	f.publication, f.hash = f.publish()
}

func (f *fixture) publish() (uuid.UUID, string) {
	f.t.Helper()
	ctx := context.Background()
	svc := rfq.Service{DB: f.d, Now: func() time.Time { return f.now }, Events: f.bus, Blocks: allStores{}, Open: allStores{}}
	q := store.New(f.d.Conn(ctx))
	set := uuid.NewV7()
	must(f.t, q.InsertCandidateSet(ctx, store.InsertCandidateSetParams{ID: set, RequestID: f.request, RequestRevisionID: f.revision, BuyerID: f.buyer, Now: f.now}))
	var cands []uuid.UUID
	for i, s := range []int64{invitedStore, otherInvitedStore} {
		cand := uuid.NewV7()
		cands = append(cands, cand)
		must(f.t, q.InsertCandidate(ctx, store.InsertCandidateParams{ID: cand, CandidateSetID: set, StoreID: s, SourceID: "p", Rank: int32(i), Now: f.now}))
	}
	f.exec(`INSERT INTO procurement.agent_runs (id, buyer_id, request_id, candidate_set_id, run_type, status, client_request_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'CANDIDATE_SEARCH', 'SUCCEEDED', $5, $6, $6)`, uuid.NewV7(), f.buyer, f.request, set, uuid.NewV7(), f.now)
	f.exec(`UPDATE procurement.requests SET status = 'MATCHING' WHERE id = $1`, f.request)
	p, err := svc.PreviewPublication(ctx, rfq.PreviewPublicationInput{
		BuyerID: f.buyer, RequestID: f.request, ClientRequestID: uuid.NewV7(), ExpectedRevisionID: f.revision, LowerBoundAmount: 100, UpperBoundAmount: 200,
		CandidateSetID: set, SelectedCandidateIDs: cands,
	})
	must(f.t, err)
	pub, err := svc.ExecutePublication(ctx, rfq.ExecuteInput{
		BuyerID: f.buyer, RequestID: f.request, ClientRequestID: uuid.NewV7(), ApprovalID: p.ApprovalID, ExpectedRevisionID: f.revision, ApprovalVersion: p.ApprovalVersion,
		PayloadHash: p.SellerFacingHash,
	})
	must(f.t, err)
	return pub.ID, pub.SellerFacingHash
}

func (f *fixture) service() sellerside.Service {
	return sellerside.Service{DB: f.d, Now: func() time.Time { return f.now }, Stores: f.stores}
}

func (f *fixture) buyerQuotes() quote.Service {
	return quote.Service{DB: f.d, Now: func() time.Time { return f.now }, Jobs: f.jobs}
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.d.Pool().Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.d.Pool().QueryRow(context.Background(), "SELECT count(*) FROM "+sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *fixture) validQuote() model.QuoteInput {
	until := f.now.Add(72 * time.Hour)
	return model.QuoteInput{
		ResponseType: model.ResponseTypeManualQuote, SpecNote: "規格如圖", UnitBasisQuantity: "1", UnitBasisUnit: "KG",
		OfferedQuantity: "12.5", OfferedQuantityUnit: "KG", GoodsAmount: 1000, ShippingAmount: 50, TaxAmount: 20,
		TaxInclusionBasis: model.TaxExcluded, TotalAmount: 1070, ValidUntil: &until,
	}
}

func (f *fixture) submitIn(store int64, cid uuid.UUID, expectedResponse string, q model.QuoteInput) sellerside.SubmitInput {
	return sellerside.SubmitInput{
		StoreID: store, PublicationID: f.publication, ClientRequestID: cid, ExpectedPublicationVersion: "1", ExpectedSellerFacingHash: f.hash,
		ExpectedResponseVersion: expectedResponse, Quote: q,
	}
}

func (f *fixture) submit(store int64, expectedResponse string, q model.QuoteInput) model.SellerResponseVersion {
	f.t.Helper()
	v, err := f.service().Submit(context.Background(), f.submitIn(store, uuid.NewV7(), expectedResponse, q))
	must(f.t, err)
	return v
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantKind(t *testing.T, err error, kind apperr.Kind) {
	t.Helper()
	if apperr.KindOf(err) != kind {
		t.Fatalf("要 %v，得到 %v", kind, err)
	}
}

func wantConflict(t *testing.T, err error, target string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindVersionConflict || ae.Reason != apperr.ReasonVersionConflict || ae.Metadata["conflictTarget"] != target {
		t.Fatalf("要 VERSION_CONFLICT conflictTarget=%s，得到 %v", target, err)
	}
}

func wantClosed(t *testing.T, err error) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonRfqRecipientClosed {
		t.Fatalf("要 FAILED_PRECONDITION／RFQ_RECIPIENT_CLOSED，得到 %v", err)
	}
}

type allStores struct{}

func (allStores) StoreIDsBlockingBuyer(context.Context, int64, []int64) (map[int64]struct{}, error) {
	return map[int64]struct{}{}, nil
}

func (allStores) OpenStoreIDs(_ context.Context, ids []int64) (map[int64]struct{}, error) {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}
