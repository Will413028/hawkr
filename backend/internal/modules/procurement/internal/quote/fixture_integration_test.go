//go:build integration

package quote_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfq"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
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

const buyer int64 = 1

func ptr[T any](v T) *T { return &v }

// fixture 是一位買家的一張需求（現行版本：小白菜、3 KG、台北市）、它的一次發布（指定一家店，買家預期單價 100 到 200 元每 KG）與一個可注入時間的服務。
type fixture struct {
	t           *testing.T
	d           *db.DB
	jobs        *jobs.Client
	now         time.Time
	buyer       int64
	request     uuid.UUID
	revision    uuid.UUID
	publication uuid.UUID
	hash        string
	bus         *events.Bus
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "procurement", Dir: procurement.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, d: d, jobs: jc, now: start, bus: events.NewBus()}
	f.adopt(buyer)
	return f
}

// sibling 在同一個資料庫為另一位買家建一張需求並發布；時間與 job 佇列共用。
func (f *fixture) sibling(buyerID int64) *fixture {
	g := &fixture{t: f.t, d: f.d, jobs: f.jobs, now: f.now, bus: f.bus}
	g.adopt(buyerID)
	return g
}

// adopt 為買家建一張 REQUIREMENT_READY 的需求並發布一次。
func (f *fixture) adopt(buyerID int64) {
	ctx := context.Background()
	f.buyer, f.request, f.revision = buyerID, uuid.NewV7(), uuid.NewV7()
	q := store.New(f.d.Conn(ctx))
	must(f.t, q.InsertRequest(ctx, store.InsertRequestParams{ID: f.request, BuyerID: &buyerID, Now: start}))
	must(f.t, q.InsertConversation(ctx, store.InsertConversationParams{ID: uuid.NewV7(), BuyerID: buyerID, RequestID: f.request, Now: start}))
	f.insertRevision(1, nil)
	f.exec(`UPDATE procurement.requests SET status = 'REQUIREMENT_READY' WHERE id = $1`, f.request)
	f.publication, f.hash = f.publish()
}

func (f *fixture) insertRevision(no int32, parent *uuid.UUID) {
	f.t.Helper()
	ctx := context.Background()
	owner := f.buyer
	p := store.InsertRevisionParams{
		ID: f.revision, RevisionNo: no, ParentRevisionID: parent, Item: ptr("小白菜"), Quantity: ptr(int32(3)), Unit: ptr("KG"), DeliveryCity: ptr("台北市"),
		Now: start, RequestID: f.request, BuyerID: &owner,
	}
	if n, err := store.New(f.d.Conn(ctx)).InsertRevision(ctx, p); err != nil || n != 1 {
		f.t.Fatalf("InsertRevision n=%d err=%v", n, err)
	}
}

// revise 把現行版本換成新的一版（內容不同：數量 5）；已發布的核准仍指著舊版。
func (f *fixture) revise() {
	f.t.Helper()
	ctx := context.Background()
	owner := f.buyer
	if _, err := store.New(f.d.Conn(ctx)).SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: f.request, BuyerID: &owner}); err != nil {
		f.t.Fatal(err)
	}
	parent := f.revision
	f.revision = uuid.NewV7()
	f.insertRevision(2, &parent)
	f.exec(`UPDATE procurement.request_revisions SET quantity = 5 WHERE id = $1`, f.revision)
}

// publish 預覽並執行一次發布（候選集有效的前提是需求在 MATCHING，A9-D29(4)，所以先轉成 MATCHING），邀請測試會回價的店（inviteFirstStore 到 inviteLastStore；回價列的外鍵要求有邀請列），回發布 ID 與賣家可見內容的 hash。
func (f *fixture) publish() (uuid.UUID, string) {
	f.t.Helper()
	ctx := context.Background()
	svc := rfq.Service{DB: f.d, Now: func() time.Time { return f.now }, Events: f.bus, Blocks: allStores{}, Open: allStores{}}
	q := store.New(f.d.Conn(ctx))
	set := uuid.NewV7()
	must(f.t, q.InsertCandidateSet(ctx, store.InsertCandidateSetParams{ID: set, RequestID: f.request, RequestRevisionID: f.revision, BuyerID: f.buyer, Now: f.now}))
	var cands []uuid.UUID
	for s := int64(inviteFirstStore); s <= inviteLastStore; s++ {
		cand := uuid.NewV7()
		cands = append(cands, cand)
		must(f.t, q.InsertCandidate(ctx, store.InsertCandidateParams{ID: cand, CandidateSetID: set, StoreID: s, SourceID: "p", Rank: int32(s - inviteFirstStore), Now: f.now}))
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

const inviteFirstStore, inviteLastStore = 10, 21

// allStores 是替身的 seller：沒有店封鎖買家，每家店都營業中。
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

// version 是賣家回價的一個版本（測試直接寫表）。
type version struct {
	goods      int64
	validUntil time.Time
	submitted  time.Time
	quantity   string
	unit       string // 報價數量的單位，預設 KG
	unitBasis  string
}

func (f *fixture) service() quote.Service {
	return quote.Service{DB: f.d, Now: func() time.Time { return f.now }, Jobs: f.jobs}
}

// respond 為一家店在發布上建回價，依序寫入版本（版本號從 1 起），latest_version 是最後一個。
func (f *fixture) respond(publication uuid.UUID, storeID int64, versions ...version) uuid.UUID {
	f.t.Helper()
	return f.respondAt(start, publication, storeID, versions...)
}

// respondAt 同 respond，回價身分列的建立時間是 created；店不在發布的邀請裡就補一列邀請（回價列的外鍵要求有邀請列）。
func (f *fixture) respondAt(created time.Time, publication uuid.UUID, storeID int64, versions ...version) uuid.UUID {
	f.t.Helper()
	ctx := context.Background()
	id := uuid.NewV7()
	f.exec(`INSERT INTO procurement.rfq_publication_invites (id, publication_id, position, store_id, delivery_state_at, created_at, updated_at)
		SELECT $1, $2, COALESCE(max(position), -1) + 1, $3, $4, $4, $4 FROM procurement.rfq_publication_invites WHERE publication_id = $2
		ON CONFLICT (publication_id, store_id) DO NOTHING`, uuid.NewV7(), publication, storeID, created)
	must(f.t, store.New(f.d.Conn(ctx)).InsertSellerResponse(ctx, store.InsertSellerResponseParams{ID: id, PublicationID: publication, RequestID: f.request, StoreID: storeID, Now: created}))
	for i, v := range versions {
		if v.quantity == "" {
			v.quantity = "3"
		}
		if v.unit == "" {
			v.unit = "KG"
		}
		if v.unitBasis == "" {
			v.unitBasis = "1"
		}
		if v.validUntil.IsZero() {
			v.validUntil = start.Add(72 * time.Hour)
		}
		if v.submitted.IsZero() {
			v.submitted = start.Add(time.Duration(i+1) * time.Minute)
		}
		f.exec(`INSERT INTO procurement.seller_response_versions (
				id, seller_response_id, version, response_type, spec_note, unit_basis_quantity, unit_basis_unit,
				offered_quantity, offered_quantity_unit, conversion_basis, currency, goods_amount, shipping_amount, tax_amount, tax_inclusion_basis, total_amount,
				valid_until, outside_buyer_requested_range, publication_version, client_request_id, submitted_at, created_at)
			VALUES ($1, $2, $3, 'MANUAL_QUOTE', '', $4::text::numeric, 'KG', $5::text::numeric, $11, '', 'TWD', $6::bigint, 10, 5, 'TAX_EXCLUDED', $7::bigint, $8, false, 1, $9, $10, $10)`,
			uuid.NewV7(), id, int32(i+1), v.unitBasis, v.quantity, v.goods, v.goods+15, v.validUntil, uuid.NewV7(), v.submitted, v.unit)
	}
	f.exec(`UPDATE procurement.seller_responses SET latest_version = $2 WHERE id = $1`, id, int32(len(versions)))
	return id
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

// provisioningJobs 回排入的建單 job 的接受單 ID。
func (f *fixture) provisioningJobs() []string {
	f.t.Helper()
	rows, err := f.d.Pool().Query(context.Background(), `SELECT args->>'acceptanceId' FROM river.river_job WHERE kind = $1 ORDER BY id`, quote.OrderProvisioningArgs{}.Kind())
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		must(f.t, rows.Scan(&id))
		out = append(out, id)
	}
	must(f.t, rows.Err())
	return out
}

func (f *fixture) listIn() quote.ListInput {
	return quote.ListInput{BuyerID: f.buyer, RequestID: f.request, PublicationID: f.publication}
}

func (f *fixture) acceptIn(response uuid.UUID, version int32) quote.AcceptInput {
	return quote.AcceptInput{
		BuyerID: f.buyer, RequestID: f.request, ClientRequestID: uuid.NewV7(), ResponseID: response, ExpectedVersion: version, ExpectedSellerFacingHash: f.hash,
	}
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

func wantPrecondition(t *testing.T, err error, reason apperr.Reason) *apperr.Error {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != reason {
		t.Fatalf("要 FAILED_PRECONDITION %v，得到 %v", reason, err)
	}
	return ae
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
