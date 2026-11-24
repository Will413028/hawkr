//go:build integration

package rfq_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var start = time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)

const buyer int64 = 1

func ptr[T any](v T) *T { return &v }

// fixture 是一位買家的一張需求（REQUIREMENT_READY；現行版本：小白菜、3 KG、台北市）與一個可注入時間、可注入訂閱者失敗的服務。
type fixture struct {
	t             *testing.T
	d             *db.DB
	buyer         int64
	request       uuid.UUID
	revision      uuid.UUID
	revisionNo    int32
	bus           *events.Bus
	mu            sync.Mutex
	now           time.Time
	published     []model.RfqPublished
	subscriberErr error
	lastSet       uuid.UUID
	lastCands     []uuid.UUID
	lastSetRev    uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "procurement", Dir: procurement.Migrations()}))
	f := &fixture{t: t, d: d, bus: events.NewBus(), now: start}
	events.Subscribe(f.bus, func(_ context.Context, e model.RfqPublished) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.subscriberErr != nil {
			return f.subscriberErr
		}
		f.published = append(f.published, e)
		return nil
	})
	f.adopt(buyer)
	return f
}

// sibling 在同一個資料庫為另一位買家（或同一位買家的另一張需求）建一張需求；時間與事件匯流排共用。
func (f *fixture) sibling(buyerID int64) *fixture {
	g := &fixture{t: f.t, d: f.d, bus: f.bus, now: f.now}
	g.adopt(buyerID)
	return g
}

func (f *fixture) adopt(buyerID int64) {
	ctx := context.Background()
	f.buyer, f.request, f.revision, f.revisionNo = buyerID, uuid.NewV7(), uuid.NewV7(), 0
	q := store.New(f.d.Conn(ctx))
	must(f.t, q.InsertRequest(ctx, store.InsertRequestParams{ID: f.request, BuyerID: &buyerID, Now: start}))
	must(f.t, q.InsertConversation(ctx, store.InsertConversationParams{ID: uuid.NewV7(), BuyerID: buyerID, RequestID: f.request, Now: start}))
	f.revise(func(p *store.InsertRevisionParams) {})
	f.setStatus("REQUIREMENT_READY")
}

// setStatus 直接改需求狀態：狀態改變不產生新版本，發布鏈要在預覽與執行時各查一次。
func (f *fixture) setStatus(status string) {
	f.t.Helper()
	f.exec(`UPDATE procurement.requests SET status = $2 WHERE id = $1`, f.request, status)
}

func (f *fixture) service() rfq.Service {
	return rfq.Service{DB: f.d, Now: func() time.Time { return f.now }, Events: f.bus}
}

// revise 把現行版本取代成新的一版（預設內容：小白菜、3 KG、台北市），f.revision 指向新版。
func (f *fixture) revise(edit func(*store.InsertRevisionParams)) {
	f.t.Helper()
	ctx := context.Background()
	q := store.New(f.d.Conn(ctx))
	owner := f.buyer
	var parent *uuid.UUID
	if f.revisionNo > 0 {
		if _, err := q.SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: f.request, BuyerID: &owner}); err != nil {
			f.t.Fatal(err)
		}
		p := f.revision
		parent = &p
	}
	f.revisionNo++
	p := store.InsertRevisionParams{
		ID: uuid.NewV7(), RevisionNo: f.revisionNo, ParentRevisionID: parent, Item: ptr("小白菜"), Quantity: ptr(int32(3)), Unit: ptr("KG"),
		DeliveryCity: ptr("台北市"), Now: start, RequestID: f.request, BuyerID: &owner,
	}
	edit(&p)
	if n, err := q.InsertRevision(ctx, p); err != nil || n != 1 {
		f.t.Fatalf("InsertRevision n=%d err=%v", n, err)
	}
	f.revision = p.ID
}

// searched 建一次成功的候選搜尋（候選依序是 stores），需求轉 MATCHING，回候選集與每個候選的 ID（依 stores 的順序）。
func (f *fixture) searched(stores ...int64) (uuid.UUID, []uuid.UUID) {
	f.t.Helper()
	set, ids := f.seedSearch(stores...)
	f.exec(`UPDATE procurement.requests SET status = 'MATCHING' WHERE id = $1`, f.request)
	return set, ids
}

// seedSearch 建一次成功的候選搜尋但不動需求狀態，並記住它是現行版本最後一次搜尋（publicationIn 用）。
func (f *fixture) seedSearch(stores ...int64) (uuid.UUID, []uuid.UUID) {
	f.t.Helper()
	ctx := context.Background()
	q := store.New(f.d.Conn(ctx))
	set := uuid.NewV7()
	must(f.t, q.InsertCandidateSet(ctx, store.InsertCandidateSetParams{ID: set, RequestID: f.request, RequestRevisionID: f.revision, BuyerID: f.buyer, Now: f.now}))
	ids := make([]uuid.UUID, len(stores))
	for i, s := range stores {
		ids[i] = uuid.NewV7()
		must(f.t, q.InsertCandidate(ctx, store.InsertCandidateParams{ID: ids[i], CandidateSetID: set, StoreID: s, SourceID: "p", Rank: int32(i), Now: f.now}))
	}
	f.exec(`INSERT INTO procurement.agent_runs (id, buyer_id, request_id, candidate_set_id, run_type, status, client_request_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'CANDIDATE_SEARCH', 'SUCCEEDED', $5, $6, $6)`, uuid.NewV7(), f.buyer, f.request, set, uuid.NewV7(), f.now.Add(time.Duration(f.count("procurement.agent_runs WHERE request_id = $1", f.request))*time.Second))
	f.lastSet, f.lastCands, f.lastSetRev = set, ids, f.revision
	return set, ids
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

func (f *fixture) approvalState(id uuid.UUID) (string, int32) {
	f.t.Helper()
	var state string
	var version int32
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT state, version FROM procurement.rfq_publication_approvals WHERE id = $1`, id).Scan(&state, &version); err != nil {
		f.t.Fatal(err)
	}
	return state, version
}

func (f *fixture) events() []model.RfqPublished {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]model.RfqPublished(nil), f.published...)
}

func (f *fixture) priceIn() rfq.PriceInput {
	return rfq.PriceInput{BuyerID: f.buyer, RequestID: f.request, ExpectedRevisionID: f.revision, LowerBoundAmount: 100, UpperBoundAmount: 200}
}

// publicationIn 是指定對象的預設輸入：選現行版本最後一次搜尋的第一個候選；現行版本還沒有搜尋就先搜尋一次（店 10、20），不改需求狀態。
func (f *fixture) publicationIn() rfq.PreviewPublicationInput {
	f.t.Helper()
	if f.lastSetRev != f.revision {
		f.seedSearch(10, 20)
	}
	return f.directIn(f.lastSet, f.lastCands[0])
}

func (f *fixture) directIn(set uuid.UUID, candidates ...uuid.UUID) rfq.PreviewPublicationInput {
	in := rfq.PreviewPublicationInput{
		BuyerID: f.buyer, RequestID: f.request, ClientRequestID: uuid.NewV7(), ExpectedRevisionID: f.revision, LowerBoundAmount: 100, UpperBoundAmount: 200,
		AudienceMode: model.AudienceDirectRecipients, CandidateSetID: &set, SelectedCandidateIDs: candidates,
	}
	return in
}

func (f *fixture) approved(in rfq.PreviewPublicationInput) model.PublicationPreview {
	f.t.Helper()
	p, err := f.service().PreviewPublication(context.Background(), in)
	must(f.t, err)
	return p
}

func (f *fixture) executeIn(p model.PublicationPreview) rfq.ExecuteInput {
	return rfq.ExecuteInput{
		BuyerID: f.buyer, RequestID: f.request, ClientRequestID: uuid.NewV7(), ApprovalID: p.ApprovalID, ExpectedRevisionID: f.revision,
		ApprovalVersion: p.ApprovalVersion, PayloadHash: p.SellerFacingHash,
	}
}

func (f *fixture) execute(in rfq.ExecuteInput) (model.Publication, error) {
	return f.service().ExecutePublication(context.Background(), in)
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

func wantInvalid(t *testing.T, err error, field string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("要 INVALID_ARGUMENT（%s），得到 %v", field, err)
	}
	for _, v := range ae.FieldViolations() {
		if v.Field == field {
			return
		}
	}
	t.Fatalf("INVALID_ARGUMENT 要指向 %s，得到 %v", field, ae.FieldViolations())
}

// wantReason 檢查錯誤帶（或不帶）指定的 reason。
func wantReason(t *testing.T, err error, reason apperr.Reason, present bool) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("要 apperr，得到 %v", err)
	}
	if (ae.Reason == reason) != present {
		t.Fatalf("reason %v 應該%s，得到 %v", reason, map[bool]string{true: "有", false: "沒有"}[present], ae.Reason)
	}
}
