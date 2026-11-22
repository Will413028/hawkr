//go:build integration

package procurement

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

func newDB(t *testing.T) *db.DB {
	t.Helper()
	return pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "procurement", Dir: Migrations()}))
}

var epoch = time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)

func id() uuid.UUID {
	return uuid.NewV7()
}

func ptr[T any](v T) *T { return &v }

// chain 是一位買家從需求到接受報價的整條資料，每一列都由 store 的查詢寫入，所以這個測試同時是每支寫入查詢的第一個執行者。
type chain struct {
	buyer                                               int64
	request, revision, conversation, turn, candidateSet uuid.UUID
	candidate, preview, snapshot, approval, publication uuid.UUID
	invite, sellerResponse, acceptance, run             uuid.UUID
	acceptanceClientRequest                             uuid.UUID
}

const storeID = int64(900)

// seed 為買家寫入一整條資料。published 為 true 時走到發布、回價、接受；否則停在候選與預覽之前。
func seed(ctx context.Context, t *testing.T, q *store.Queries, buyerID int64, published bool) chain {
	t.Helper()
	c := chain{buyer: buyerID, request: id(), revision: id(), conversation: id(), turn: id(), candidateSet: id(), candidate: id(),
		run: id(), acceptanceClientRequest: id()}
	must := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s：%v", what, err)
		}
	}
	rows := func(what string, n int64, err error) {
		t.Helper()
		must(what, err)
		if n != 1 {
			t.Fatalf("%s：影響 %d 列，要 1", what, n)
		}
	}
	must("InsertRequest", q.InsertRequest(ctx, store.InsertRequestParams{ID: c.request, BuyerID: &buyerID, Now: epoch}))
	n0, err0 := q.InsertRevision(ctx, store.InsertRevisionParams{
		ID: c.revision, RequestID: c.request, BuyerID: &buyerID, RevisionNo: 1, Item: ptr("外套"), Quantity: ptr(int32(3)), Unit: ptr("件"),
		BudgetPerUnit: ptr(int64(500)), DeliveryCity: ptr("台北市"), DeliveryDate: ptr(epoch), ExactMatchRequired: ptr(false), Now: epoch,
	})
	rows("InsertRevision", n0, err0)
	must("InsertConversation", q.InsertConversation(ctx, store.InsertConversationParams{ID: c.conversation, BuyerID: buyerID, RequestID: c.request, Now: epoch}))
	n, err := q.InsertTurn(ctx, store.InsertTurnParams{
		ID: c.turn, RequestID: c.request, ConversationID: c.conversation, BuyerID: buyerID, ResultRevisionID: c.revision, Source: "LLM_EXTRACT",
		BuyerMessage: "我要三件外套", AssistantReply: "好的", Steps: []byte("[]"), RecordedThisTurn: []byte("[]"), OptionalStreak: 2, SkippedSlots: []string{"PACKAGING"}, ClientRequestID: id(), Now: epoch,
	})
	rows("InsertTurn", n, err)
	n, err = q.InsertCandidateSet(ctx, store.InsertCandidateSetParams{ID: c.candidateSet, RequestID: c.request, RequestRevisionID: c.revision, BuyerID: buyerID, ClientRequestID: id(), Now: epoch})
	rows("InsertCandidateSet", n, err)
	must("InsertCandidate", q.InsertCandidate(ctx, store.InsertCandidateParams{
		ID: c.candidate, CandidateSetID: c.candidateSet, StoreID: storeID, SourceID: "p1", Score: 80, Rank: 1,
		MatchedFacts: []byte(`[{"slotKey":"ITEM","label":"外套"}]`), UnverifiableFacts: []byte("[]"), FactsAsOf: epoch, Now: epoch,
	}))
	must("InsertRun", func() error {
		n, err := q.InsertRun(ctx, store.InsertRunParams{ID: c.run, BuyerID: buyerID, RequestID: c.request, ConversationID: &c.conversation, RunType: "TURN", ClientRequestID: id(), QuotaPeriodStart: &epoch, Now: epoch})
		if err == nil && n != 1 {
			err = errors.New("InsertRun 沒有寫入")
		}
		return err
	}())
	must("InsertRunInput", q.InsertRunInput(ctx, store.InsertRunInputParams{RunID: c.run, BuyerMessage: "我要三件外套", Locale: "ZH_TW", Now: epoch}))
	if _, err := q.ConsumeUsage(ctx, store.ConsumeUsageParams{BuyerID: buyerID, PeriodStart: epoch, Now: epoch, QuotaLimit: 5}); err != nil {
		t.Fatalf("ConsumeUsage：%v", err)
	}
	if !published {
		return c
	}
	c.preview, c.snapshot, c.approval, c.publication, c.invite, c.sellerResponse, c.acceptance = id(), id(), id(), id(), id(), id(), id()
	n, err = q.InsertPricePreview(ctx, store.InsertPricePreviewParams{
		ID: c.preview, BuyerID: &buyerID, RequestID: c.request, RequestRevisionID: c.revision, RangeLowerAmount: 400, RangeUpperAmount: 600,
		BasisQuantity: 1, BasisUnit: "件", TotalLowerAmount: ptr(int64(1200)), TotalUpperAmount: ptr(int64(1800)), TotalQuantity: ptr(int32(3)), TotalUnit: ptr("件"),
		ClientRequestID: id(), ExpiresAt: epoch.Add(time.Hour), Now: epoch,
	})
	rows("InsertPricePreview", n, err)
	n, err = q.InsertConfirmedSnapshot(ctx, store.InsertConfirmedSnapshotParams{
		ID: c.snapshot, BuyerID: &buyerID, RequestID: c.request, RequestRevisionID: c.revision, PricePreviewID: c.preview, BuyerNote: "請盡快", ClientRequestID: id(),
		ExpiresAt: epoch.Add(time.Hour), Now: epoch,
	})
	rows("InsertConfirmedSnapshot", n, err)
	n, err = q.InsertApproval(ctx, store.InsertApprovalParams{
		ID: c.approval, BuyerID: &buyerID, RequestID: c.request, RequestRevisionID: c.revision, ConfirmedRfqSnapshotID: c.snapshot, AudienceMode: "DIRECT_RECIPIENTS",
		SchemaVersion: "v1", ClientRequestID: id(), Now: epoch, ExpiresAt: epoch.Add(time.Hour),
	})
	rows("InsertApproval", n, err)
	must("InsertApprovalRecipient", q.InsertApprovalRecipient(ctx, store.InsertApprovalRecipientParams{ApprovalID: c.approval, Position: 0, StoreID: storeID}))
	claimed, err := q.ClaimApprovalForExecute(ctx, store.ClaimApprovalForExecuteParams{ID: c.approval, Version: 1, BuyerID: &buyerID, Now: epoch})
	must("ClaimApprovalForExecute", err)
	n, err = q.MarkApprovalSucceeded(ctx, store.MarkApprovalSucceededParams{ID: c.approval, Version: claimed.Version, Now: epoch})
	rows("MarkApprovalSucceeded", n, err)
	n, err = q.InsertPublication(ctx, store.InsertPublicationParams{ID: c.publication, BuyerID: &buyerID, RequestID: c.request, RequestRevisionID: c.revision, ApprovalID: c.approval, SchemaVersion: "v1", ClientRequestID: id(), Now: epoch})
	rows("InsertPublication", n, err)
	must("InsertInvite", q.InsertInvite(ctx, store.InsertInviteParams{ID: c.invite, PublicationID: c.publication, Position: 0, StoreID: storeID, Now: epoch}))
	must("InsertSellerResponse", q.InsertSellerResponse(ctx, store.InsertSellerResponseParams{ID: c.sellerResponse, PublicationID: c.publication, RequestID: c.request, StoreID: storeID, Now: epoch}))
	n, err = q.InsertAcceptance(ctx, acceptanceParams(c, buyerID, c.acceptance, c.acceptanceClientRequest))
	rows("InsertAcceptance", n, err)
	return c
}

func acceptanceParams(c chain, buyerID int64, acceptanceID, clientRequest uuid.UUID) store.InsertAcceptanceParams {
	return store.InsertAcceptanceParams{
		ID: acceptanceID, BuyerID: &buyerID, RequestID: c.request, PublicationID: c.publication, SellerResponseID: c.sellerResponse, SellerResponseVersion: 1,
		SellerResponsePayloadHash: "h1", ConfirmedRfqPayloadHash: "h2", StoreID: storeID, Currency: "TWD", GoodsAmount: 1000, ShippingAmount: 100, TaxAmount: 50,
		AcceptedDealPrice: 1150, ClientRequestID: clientRequest, Now: epoch,
	}
}

func count(ctx context.Context, t *testing.T, d *db.DB, table, where string, args ...any) int {
	t.Helper()
	var n int
	if err := d.Pool().QueryRow(ctx, "SELECT count(*) FROM procurement."+table+" WHERE "+where, args...).Scan(&n); err != nil {
		t.Fatalf("count %s：%v", table, err)
	}
	return n
}

// 讀取查詢與狀態轉移：每一支都至少被呼叫一次，而且驗證回來的內容與冪等行為。
func TestStoreQueriesRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	q := store.New(d.Pool())
	c := seed(ctx, t, q, 11, true)

	req, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: c.request, BuyerID: &c.buyer})
	if err != nil || req.Status != "CLARIFYING" {
		t.Fatalf("GetRequestForBuyer = %+v, %v", req, err)
	}
	other := int64(12)
	if _, err := q.GetRequestForBuyer(ctx, store.GetRequestForBuyerParams{ID: c.request, BuyerID: &other}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("別的買家讀不到：%v", err)
	}
	if n, err := q.SetRequestStatus(ctx, store.SetRequestStatusParams{ID: c.request, BuyerID: &c.buyer, FromStatus: "CLARIFYING", Status: "REQUIREMENT_READY", Now: epoch}); err != nil || n != 1 {
		t.Fatalf("SetRequestStatus n=%d err=%v", n, err)
	}
	if n, _ := q.SetRequestStatus(ctx, store.SetRequestStatusParams{ID: c.request, BuyerID: &c.buyer, FromStatus: "CLARIFYING", Status: "MATCHING", Now: epoch}); n != 0 {
		t.Fatalf("目前狀態不是 from_status 時不得轉移（條件式 UPDATE）：n=%d", n)
	}

	next, err := q.NextRevisionNo(ctx, store.NextRevisionNoParams{RequestID: c.request, BuyerID: &c.buyer})
	if err != nil || next != 2 {
		t.Fatalf("NextRevisionNo = %d, %v", next, err)
	}
	rev2 := id()
	if n, err := q.SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: c.request, BuyerID: &c.buyer}); err != nil || n != 1 {
		t.Fatalf("SupersedeActiveRevision n=%d err=%v", n, err)
	}
	if n, err := q.InsertRevision(ctx, store.InsertRevisionParams{ID: rev2, RequestID: c.request, BuyerID: &c.buyer, RevisionNo: next, ParentRevisionID: &c.revision, Item: ptr("夾克"), Now: epoch}); err != nil || n != 1 {
		t.Fatalf("InsertRevision n=%d err=%v", n, err)
	}
	active, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: c.request, BuyerID: &c.buyer})
	if err != nil || active.ID != rev2 || active.Item == nil || *active.Item != "夾克" {
		t.Fatalf("GetActiveRevision = %+v, %v", active, err)
	}
	if got, err := q.GetRevision(ctx, store.GetRevisionParams{ID: c.revision, RequestID: c.request, BuyerID: &c.buyer}); err != nil || got.Status != "SUPERSEDED" {
		t.Fatalf("GetRevision = %+v, %v", got, err)
	}
	if revs, err := q.ListRevisions(ctx, store.ListRevisionsParams{RequestID: c.request, BuyerID: &c.buyer}); err != nil || len(revs) != 2 || revs[0].RevisionNo != 1 {
		t.Fatalf("ListRevisions = %d, %v", len(revs), err)
	}
	// 一張需求同時只有一個現行版本：直接再寫一個 ACTIVE 會被 partial unique 擋下。
	if _, err := q.InsertRevision(ctx, store.InsertRevisionParams{ID: id(), RequestID: c.request, BuyerID: &c.buyer, RevisionNo: 3, Now: epoch}); err == nil {
		t.Fatal("同一張需求寫入第二個 ACTIVE 版本要失敗")
	}

	conv, err := q.GetConversationForBuyer(ctx, store.GetConversationForBuyerParams{ID: c.conversation, BuyerID: c.buyer})
	if err != nil || conv.TitleSource != "AUTO" {
		t.Fatalf("GetConversationForBuyer = %+v, %v", conv, err)
	}
	if err := q.SetConversationPreview(ctx, store.SetConversationPreviewParams{ID: c.conversation, BuyerID: c.buyer, Preview: "我要三件外套", Now: epoch}); err != nil {
		t.Fatal(err)
	}
	if n, err := q.RenameConversation(ctx, store.RenameConversationParams{ID: c.conversation, BuyerID: c.buyer, Title: "外套詢價", Now: epoch}); err != nil || n != 1 {
		t.Fatalf("RenameConversation n=%d err=%v", n, err)
	}
	list, err := q.ListConversations(ctx, store.ListConversationsParams{BuyerID: c.buyer, PageSize: 10})
	if err != nil || len(list) != 1 || list[0].Title != "外套詢價" || list[0].TitleSource != "USER" {
		t.Fatalf("ListConversations = %+v, %v", list, err)
	}
	if n, err := q.SoftDeleteConversation(ctx, store.SoftDeleteConversationParams{ID: c.conversation, BuyerID: c.buyer, Now: &epoch}); err != nil || n != 1 {
		t.Fatalf("SoftDeleteConversation n=%d err=%v", n, err)
	}
	if n, _ := q.SoftDeleteConversation(ctx, store.SoftDeleteConversationParams{ID: c.conversation, BuyerID: c.buyer, Now: &epoch}); n != 0 {
		t.Fatal("已刪除的對話再刪是目標狀態：不再更新")
	}
	if list, _ := q.ListConversations(ctx, store.ListConversationsParams{BuyerID: c.buyer, PageSize: 10}); len(list) != 0 {
		t.Fatal("軟刪除的對話不在清單裡")
	}
	if got := count(ctx, t, d, "requests", "id = $1", c.request); got != 1 {
		t.Fatal("刪對話不刪需求")
	}

	// 輪次的冪等鍵：同一個 client_request_id 重送回 0 列，改讀既有的。
	turns, err := q.ListTurnsByRequest(ctx, store.ListTurnsByRequestParams{RequestID: c.request, BuyerID: c.buyer})
	if err != nil || len(turns) != 1 {
		t.Fatalf("ListTurnsByRequest = %d, %v", len(turns), err)
	}
	if turns[0].OptionalStreak != 2 || len(turns[0].SkippedSlots) != 1 || turns[0].SkippedSlots[0] != "PACKAGING" {
		t.Fatalf("optional progress = %d %v, want 2 [PACKAGING]", turns[0].OptionalStreak, turns[0].SkippedSlots)
	}
	dup, err := q.InsertTurn(ctx, store.InsertTurnParams{
		ID: id(), RequestID: c.request, ConversationID: c.conversation, BuyerID: c.buyer, ResultRevisionID: rev2, Source: "ANSWER",
		Steps: []byte("[]"), RecordedThisTurn: []byte("[]"), SkippedSlots: []string{}, ClientRequestID: turns[0].ClientRequestID, Now: epoch,
	})
	if err != nil || dup != 0 {
		t.Fatalf("重送的 InsertTurn n=%d err=%v", dup, err)
	}
	nilSkipID := id()
	if n, err := q.InsertTurn(ctx, store.InsertTurnParams{
		ID: id(), RequestID: c.request, ConversationID: c.conversation, BuyerID: c.buyer, ResultRevisionID: rev2, Source: "ANSWER",
		Steps: []byte("[]"), RecordedThisTurn: []byte("[]"), SkippedSlots: nil, ClientRequestID: nilSkipID, Now: epoch,
	}); err != nil || n != 1 {
		t.Fatalf("nil SkippedSlots 的 InsertTurn 要存成空陣列，n=%d err=%v", n, err)
	}
	if got, err := q.GetTurnByClientRequest(ctx, store.GetTurnByClientRequestParams{BuyerID: c.buyer, ClientRequestID: nilSkipID}); err != nil || got.SkippedSlots == nil || len(got.SkippedSlots) != 0 {
		t.Fatalf("nil SkippedSlots 讀回 = %v, %v, want 空陣列", got.SkippedSlots, err)
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.turns SET skipped_slots = ARRAY['QUANTITY'] WHERE id = $1`, c.turn); err == nil {
		t.Fatal("skipped_slots 只允許 PACKAGING、BUDGET_PER_UNIT：QUANTITY 要被 CHECK 擋下")
	}
	if got, err := q.GetTurnByClientRequest(ctx, store.GetTurnByClientRequestParams{BuyerID: c.buyer, ClientRequestID: turns[0].ClientRequestID}); err != nil || got.ID != c.turn {
		t.Fatalf("GetTurnByClientRequest = %+v, %v", got, err)
	}

	set, err := q.GetCandidateSetForBuyer(ctx, store.GetCandidateSetForBuyerParams{ID: c.candidateSet, RequestID: c.request, BuyerID: c.buyer})
	if err != nil {
		t.Fatal(err)
	}
	if dup, err := q.InsertCandidateSet(ctx, store.InsertCandidateSetParams{ID: id(), RequestID: c.request, RequestRevisionID: c.revision, BuyerID: c.buyer, ClientRequestID: set.ClientRequestID, Now: epoch}); err != nil || dup != 0 {
		t.Fatalf("重送的 InsertCandidateSet n=%d err=%v", dup, err)
	}
	if got, err := q.GetCandidateSetByClientRequest(ctx, store.GetCandidateSetByClientRequestParams{RequestRevisionID: c.revision, ClientRequestID: set.ClientRequestID, BuyerID: c.buyer}); err != nil || got.ID != c.candidateSet {
		t.Fatalf("GetCandidateSetByClientRequest = %+v, %v", got, err)
	}
	if n, err := q.SetCandidateSelected(ctx, store.SetCandidateSelectedParams{Selected: true, Now: epoch, ID: c.candidate, CandidateSetID: c.candidateSet, RequestID: c.request, BuyerID: c.buyer}); err != nil || n != 1 {
		t.Fatalf("SetCandidateSelected n=%d err=%v", n, err)
	}
	cands, err := q.ListCandidates(ctx, store.ListCandidatesParams{CandidateSetID: c.candidateSet, BuyerID: c.buyer})
	if err != nil || len(cands) != 1 || !cands[0].Selected || cands[0].SelectedAt == nil {
		t.Fatalf("ListCandidates = %+v, %v", cands, err)
	}
	if n, _ := q.SetCandidateSelected(ctx, store.SetCandidateSelectedParams{Selected: false, ID: c.candidate, CandidateSetID: c.candidateSet, RequestID: c.request, BuyerID: 999}); n != 0 {
		t.Fatal("別的買家不能切換選取")
	}
	if n, err := q.SetCandidateSelected(ctx, store.SetCandidateSelectedParams{Selected: false, ID: c.candidate, CandidateSetID: c.candidateSet, RequestID: c.request, BuyerID: c.buyer}); err != nil || n != 1 {
		t.Fatalf("取消選取 n=%d err=%v", n, err)
	}
	if cands, _ := q.ListCandidates(ctx, store.ListCandidatesParams{CandidateSetID: c.candidateSet, BuyerID: c.buyer}); cands[0].Selected || cands[0].SelectedAt != nil {
		t.Fatal("取消選取後 selected_at 要清掉")
	}

	pv, err := q.GetPricePreview(ctx, store.GetPricePreviewParams{ID: c.preview, RequestID: c.request, BuyerID: &c.buyer})
	if err != nil || pv.TotalQuantity == nil || *pv.TotalQuantity != 3 {
		t.Fatalf("GetPricePreview = %+v, %v", pv, err)
	}
	if dup, err := q.InsertPricePreview(ctx, store.InsertPricePreviewParams{ID: id(), BuyerID: &c.buyer, RequestID: c.request, RequestRevisionID: c.revision, RangeLowerAmount: 1, RangeUpperAmount: 2, BasisQuantity: 1, BasisUnit: "件", ClientRequestID: pv.ClientRequestID, ExpiresAt: epoch, Now: epoch}); err != nil || dup != 0 {
		t.Fatalf("重送的 InsertPricePreview n=%d err=%v", dup, err)
	}
	if got, err := q.GetPricePreviewByClientRequest(ctx, store.GetPricePreviewByClientRequestParams{BuyerID: &c.buyer, ClientRequestID: pv.ClientRequestID}); err != nil || got.ID != c.preview {
		t.Fatalf("GetPricePreviewByClientRequest = %+v, %v", got, err)
	}
	snap, err := q.GetConfirmedSnapshot(ctx, store.GetConfirmedSnapshotParams{ID: c.snapshot, RequestID: c.request, BuyerID: &c.buyer})
	if err != nil || snap.BuyerNote != "請盡快" {
		t.Fatalf("GetConfirmedSnapshot = %+v, %v", snap, err)
	}
	if got, err := q.GetConfirmedSnapshotByClientRequest(ctx, store.GetConfirmedSnapshotByClientRequestParams{BuyerID: &c.buyer, ClientRequestID: snap.ClientRequestID}); err != nil || got.ID != c.snapshot {
		t.Fatalf("GetConfirmedSnapshotByClientRequest = %+v, %v", got, err)
	}
	appr, err := q.GetApproval(ctx, store.GetApprovalParams{ID: c.approval, RequestID: c.request, BuyerID: &c.buyer})
	if err != nil || appr.State != "SUCCEEDED" || appr.Version != 3 {
		t.Fatalf("GetApproval = %+v, %v", appr, err)
	}
	if got, err := q.GetApprovalByClientRequest(ctx, store.GetApprovalByClientRequestParams{BuyerID: &c.buyer, ClientRequestID: appr.ClientRequestID}); err != nil || got.ID != c.approval {
		t.Fatalf("GetApprovalByClientRequest = %+v, %v", got, err)
	}
	// 過期的 version 或已不是 PENDING 的核准，ClaimApprovalForExecute 沒命中任何列。
	if _, err := q.ClaimApprovalForExecute(ctx, store.ClaimApprovalForExecuteParams{ID: c.approval, Version: 1, BuyerID: &c.buyer, Now: epoch}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("已執行過的核准不能再 claim：%v", err)
	}
	if rs, err := q.ListApprovalRecipients(ctx, store.ListApprovalRecipientsParams{ApprovalID: c.approval, BuyerID: &c.buyer}); err != nil || len(rs) != 1 || rs[0].StoreID != storeID {
		t.Fatalf("ListApprovalRecipients = %+v, %v", rs, err)
	}
	pub, err := q.GetPublication(ctx, store.GetPublicationParams{ID: c.publication, BuyerID: &c.buyer})
	if err != nil || pub.State != "PUBLISHED" {
		t.Fatalf("GetPublication = %+v, %v", pub, err)
	}
	if got, err := q.GetPublicationByClientRequest(ctx, store.GetPublicationByClientRequestParams{BuyerID: &c.buyer, ClientRequestID: pub.ClientRequestID}); err != nil || got.ID != c.publication {
		t.Fatalf("GetPublicationByClientRequest = %+v, %v", got, err)
	}
	if dup, err := q.InsertPublication(ctx, store.InsertPublicationParams{ID: id(), BuyerID: &c.buyer, RequestID: c.request, RequestRevisionID: c.revision, ApprovalID: c.approval, SchemaVersion: "v1", ClientRequestID: id(), Now: epoch}); err != nil || dup != 0 {
		t.Fatalf("同一個核准只能發布一次：n=%d err=%v", dup, err)
	}
	otherBuyer := int64(999)
	if _, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: c.request, BuyerID: &otherBuyer}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("別的買家不能讀現行版本：%v", err)
	}
	if _, err := q.GetRevision(ctx, store.GetRevisionParams{ID: c.revision, RequestID: c.request, BuyerID: &otherBuyer}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("別的買家不能讀版本：%v", err)
	}
	if revs, err := q.ListRevisions(ctx, store.ListRevisionsParams{RequestID: c.request, BuyerID: &otherBuyer}); err != nil || len(revs) != 0 {
		t.Fatalf("別的買家列不到版本：%d, %v", len(revs), err)
	}
	if n, _ := q.SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: c.request, BuyerID: &otherBuyer}); n != 0 {
		t.Fatal("別的買家不能把版本轉成 SUPERSEDED")
	}
	if cs, err := q.ListCandidates(ctx, store.ListCandidatesParams{CandidateSetID: c.candidateSet, BuyerID: otherBuyer}); err != nil || len(cs) != 0 {
		t.Fatalf("別的買家列不到候選：%d, %v", len(cs), err)
	}
	if rs, err := q.ListApprovalRecipients(ctx, store.ListApprovalRecipientsParams{ApprovalID: c.approval, BuyerID: &otherBuyer}); err != nil || len(rs) != 0 {
		t.Fatalf("別的買家列不到收件店：%d, %v", len(rs), err)
	}
	if _, err := q.GetPublication(ctx, store.GetPublicationParams{ID: c.publication, BuyerID: &otherBuyer}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("別的買家不能讀發布：%v", err)
	}
	if is, err := q.ListInvites(ctx, store.ListInvitesParams{PublicationID: c.publication, BuyerID: &otherBuyer}); err != nil || len(is) != 0 {
		t.Fatalf("別的買家列不到邀請：%d, %v", len(is), err)
	}
	if _, err := q.GetLiveAcceptance(ctx, store.GetLiveAcceptanceParams{RequestID: c.request, BuyerID: &otherBuyer}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("別的買家不能讀 live 接受單：%v", err)
	}
	if _, err := q.GetRunInput(ctx, store.GetRunInputParams{RunID: c.run, BuyerID: otherBuyer}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("別的買家不能讀 run 輸入：%v", err)
	}
	if _, err := q.GetCandidateSetByClientRequest(ctx, store.GetCandidateSetByClientRequestParams{RequestRevisionID: c.revision, ClientRequestID: set.ClientRequestID, BuyerID: otherBuyer}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("別的買家不能用 client_request_id 讀候選集：%v", err)
	}
	if _, err := q.NextRevisionNo(ctx, store.NextRevisionNoParams{RequestID: c.request, BuyerID: &otherBuyer}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("別的買家取不到下一個版本號（要 ErrNoRows，不是聚合補出的 1）：%v", err)
	}
	if n, err := q.InsertRevision(ctx, store.InsertRevisionParams{ID: id(), RequestID: c.request, BuyerID: &otherBuyer, RevisionNo: 99, Now: epoch}); err != nil || n != 0 {
		t.Fatalf("別的買家不能對別人的需求寫版本：n=%d err=%v", n, err)
	}
	expiredApproval := id()
	if n, err := q.InsertApproval(ctx, store.InsertApprovalParams{
		ID: expiredApproval, BuyerID: &c.buyer, RequestID: c.request, RequestRevisionID: c.revision, ConfirmedRfqSnapshotID: c.snapshot, AudienceMode: "DIRECT_RECIPIENTS",
		SchemaVersion: "v1", ClientRequestID: id(), Now: epoch, ExpiresAt: epoch.Add(time.Hour),
	}); err != nil || n != 1 {
		t.Fatalf("InsertApproval（過期測試）n=%d err=%v", n, err)
	}
	if _, err := q.ClaimApprovalForExecute(ctx, store.ClaimApprovalForExecuteParams{ID: expiredApproval, Version: 1, BuyerID: &c.buyer, Now: epoch.Add(2 * time.Hour)}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("過期的核准不能 claim：%v", err)
	}
	invites, err := q.ListInvites(ctx, store.ListInvitesParams{PublicationID: c.publication, BuyerID: &c.buyer})
	if err != nil || len(invites) != 1 || invites[0].DeliveryState != "QUEUED" {
		t.Fatalf("ListInvites = %+v, %v", invites, err)
	}
	if n, err := q.MarkInviteSent(ctx, store.MarkInviteSentParams{ID: c.invite, Now: epoch}); err != nil || n != 1 {
		t.Fatalf("MarkInviteSent n=%d err=%v", n, err)
	}
	if n, _ := q.MarkInviteFailed(ctx, store.MarkInviteFailedParams{ID: c.invite, Now: epoch}); n != 0 {
		t.Fatal("已 SENT 的邀請不能再標 FAILED")
	}
	if got, err := q.GetSellerResponse(ctx, store.GetSellerResponseParams{PublicationID: c.publication, StoreID: storeID}); err != nil || got.ID != c.sellerResponse || got.LatestVersion != 0 {
		t.Fatalf("GetSellerResponse = %+v, %v", got, err)
	}

	// 接受單：冪等鍵相同回 0 列並可讀回；usage 讀取。
	if dup, err := q.InsertAcceptance(ctx, acceptanceParams(c, c.buyer, id(), c.acceptanceClientRequest)); err != nil || dup != 0 {
		t.Fatalf("重送的 InsertAcceptance n=%d err=%v", dup, err)
	}
	if got, err := q.GetAcceptanceByClientRequest(ctx, store.GetAcceptanceByClientRequestParams{BuyerID: &c.buyer, ClientRequestID: c.acceptanceClientRequest}); err != nil || got.ID != c.acceptance {
		t.Fatalf("GetAcceptanceByClientRequest = %+v, %v", got, err)
	}
	if got, err := q.GetLiveAcceptance(ctx, store.GetLiveAcceptanceParams{RequestID: c.request, BuyerID: &c.buyer}); err != nil || got.ID != c.acceptance {
		t.Fatalf("GetLiveAcceptance = %+v, %v", got, err)
	}
	if n, err := q.MarkOrderCreated(ctx, store.MarkOrderCreatedParams{ID: c.acceptance, OrderNo: ptr("O1"), PaymentDeadline: &epoch, Now: epoch}); err != nil || n != 1 {
		t.Fatalf("MarkOrderCreated n=%d err=%v", n, err)
	}
	if n, _ := q.MarkOrderFailed(ctx, store.MarkOrderFailedParams{ID: c.acceptance, Reason: ptr("X"), Now: epoch}); n != 0 {
		t.Fatal("已建單的接受單不能再標建單失敗")
	}
	run, err := q.GetRunForBuyer(ctx, store.GetRunForBuyerParams{ID: c.run, BuyerID: c.buyer})
	if err != nil || run.Status != "QUEUED" || run.RunType != "TURN" {
		t.Fatalf("GetRunForBuyer = %+v, %v", run, err)
	}
	if dup, err := q.InsertRun(ctx, store.InsertRunParams{ID: id(), BuyerID: c.buyer, RequestID: c.request, RunType: "TURN", ClientRequestID: run.ClientRequestID, QuotaPeriodStart: &epoch, Now: epoch}); err != nil || dup != 0 {
		t.Fatalf("run 的冪等鍵：n=%d err=%v", dup, err)
	}
	if in, err := q.GetRunInput(ctx, store.GetRunInputParams{RunID: c.run, BuyerID: c.buyer}); err != nil || in.BuyerMessage != "我要三件外套" {
		t.Fatalf("GetRunInput = %+v, %v", in, err)
	}
	if used, err := q.GetUsage(ctx, store.GetUsageParams{BuyerID: c.buyer, PeriodStart: epoch}); err != nil || used != 1 {
		t.Fatalf("GetUsage = %d, %v", used, err)
	}
}

// S6（A9-D19 修訂）：live 鎖只擋 ACTIVE。接受單建單失敗後轉成 ORDER_FAILED 並解除鎖，買家可以再接受；沒失敗時同一張需求不能接受第二份。
func TestLiveLockOnlyBlocksActiveAcceptances(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	q := store.New(d.Pool())
	c := seed(ctx, t, q, 21, true)

	second := id()
	if n, err := q.InsertAcceptance(ctx, acceptanceParams(c, c.buyer, second, id())); err != nil || n != 0 {
		t.Fatalf("已有 ACTIVE 接受單時第二份要被擋：n=%d err=%v", n, err)
	}
	if n, err := q.MarkOrderFailed(ctx, store.MarkOrderFailedParams{ID: c.acceptance, Reason: ptr("RETRY_EXHAUSTED"), Now: epoch}); err != nil || n != 1 {
		t.Fatalf("MarkOrderFailed n=%d err=%v", n, err)
	}
	var state, provisioning string
	if err := d.Pool().QueryRow(ctx, `SELECT state, order_provisioning_state FROM procurement.buyer_quote_acceptances WHERE id = $1`, c.acceptance).Scan(&state, &provisioning); err != nil {
		t.Fatal(err)
	}
	if state != "ORDER_FAILED" || provisioning != "ORDER_FAILED" {
		t.Fatalf("state=%s provisioning=%s", state, provisioning)
	}
	if _, err := q.GetLiveAcceptance(ctx, store.GetLiveAcceptanceParams{RequestID: c.request, BuyerID: &c.buyer}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("建單失敗後沒有 live 接受單：%v", err)
	}
	if n, err := q.InsertAcceptance(ctx, acceptanceParams(c, c.buyer, second, id())); err != nil || n != 1 {
		t.Fatalf("解除鎖後買家可以再接受：n=%d err=%v", n, err)
	}
	// 沒有寫入者的值不在 CHECK 內（S6）：直接寫會被擋下。
	for _, bad := range []string{"CANCELLED", "EXPIRED", "PAYMENT_FINALIZED"} {
		if _, err := d.Pool().Exec(ctx, `UPDATE procurement.buyer_quote_acceptances SET state = $1 WHERE id = $2`, bad, second); err == nil {
			t.Errorf("state %s 沒有寫入者，CHECK 應該擋下", bad)
		}
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.rfq_publications SET state = 'CANCELLED' WHERE id = $1`, c.publication); err == nil {
		t.Error("publication state CANCELLED 沒有寫入者，CHECK 應該擋下")
	}
}

// S9：候選沒有店名快照欄位，也沒有 lane、prospect。
func TestCandidatesHaveNoStoreNameColumns(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	rows, err := d.Pool().Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = 'procurement' AND table_name = 'candidates' ORDER BY ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	cols, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"id", "candidate_set_id", "store_id", "source_id", "score", "rank", "matched_facts", "unverifiable_facts", "facts_as_of", "selected", "selected_at", "created_at"}
	if !slices.Equal(cols, want) {
		t.Fatalf("candidates 的欄位 = %v，要 %v（沒有 display_name、seller_name、logo_key、store_code、lane、prospect_id）", cols, want)
	}
}

// 額度（A9-D9）：一列計數器加條件 UPDATE，達上限即拒絕，退回後可再用，不會變負。
func TestUsageCounterIsConditional(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	q := store.New(d.Pool())
	consume := func(limit int32) (int32, error) {
		return q.ConsumeUsage(ctx, store.ConsumeUsageParams{BuyerID: 31, PeriodStart: epoch, Now: epoch, QuotaLimit: limit})
	}
	for want := int32(1); want <= 2; want++ {
		if got, err := consume(2); err != nil || got != want {
			t.Fatalf("第 %d 次 = %d, %v", want, got, err)
		}
	}
	if _, err := consume(2); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("達上限要拒絕（0 列）：%v", err)
	}
	if n, err := q.RefundUsage(ctx, store.RefundUsageParams{BuyerID: 31, PeriodStart: epoch, Now: epoch}); err != nil || n != 1 {
		t.Fatalf("RefundUsage n=%d err=%v", n, err)
	}
	if got, err := consume(2); err != nil || got != 2 {
		t.Fatalf("退回後可再用：%d, %v", got, err)
	}
	if _, err := consume(0); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("上限 0 不得扣：%v", err)
	}
	if n, _ := q.RefundUsage(ctx, store.RefundUsageParams{BuyerID: 32, PeriodStart: epoch, Now: epoch}); n != 0 {
		t.Fatal("沒有這一列時退回不動")
	}
}

type snapshotOfBuyer struct {
	requests, conversations, turns, candidateSets, runs, usage int
	publishedRequestBuyerNull                                  bool
	anonymized                                                 map[string]int
}

var anonymizedTables = []string{"price_previews", "confirmed_rfq_snapshots", "rfq_publication_approvals", "rfq_publications", "buyer_quote_acceptances"}

func snap(ctx context.Context, t *testing.T, d *db.DB, c chain) snapshotOfBuyer {
	t.Helper()
	s := snapshotOfBuyer{anonymized: map[string]int{}}
	s.requests = count(ctx, t, d, "requests", "id = $1", c.request)
	s.conversations = count(ctx, t, d, "conversations", "id = $1", c.conversation)
	s.turns = count(ctx, t, d, "turns", "id = $1", c.turn)
	s.candidateSets = count(ctx, t, d, "candidate_sets", "id = $1", c.candidateSet)
	s.runs = count(ctx, t, d, "agent_runs", "id = $1", c.run)
	s.usage = count(ctx, t, d, "usage", "buyer_id = $1", c.buyer)
	s.publishedRequestBuyerNull = count(ctx, t, d, "requests", "id = $1 AND buyer_id IS NULL", c.request) == 1
	for _, tbl := range anonymizedTables {
		s.anonymized[tbl] = count(ctx, t, d, tbl, "request_id = $1 AND buyer_id IS NULL", c.request)
	}
	return s
}

// S10：BuyerDeleted 後，買家資料刪除；已發布的需求與賣家看得到的紀錄保留、buyer_id 設 NULL；別的買家不受影響；重送冪等。
func TestBuyerDeletedRemovesBuyerDataAndAnonymizesPublishedRecords(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	q := store.New(d.Pool())
	published := seed(ctx, t, q, 41, true)
	draft := seed(ctx, t, q, 41, false)
	bystander := seed(ctx, t, q, 42, true)
	m := New(Deps{DB: d, Now: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})

	deleteBuyer := func() error {
		return d.InTx(ctx, func(ctx context.Context) error { return m.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: 41}) })
	}
	if err := deleteBuyer(); err != nil {
		t.Fatal(err)
	}

	if s := snap(ctx, t, d, draft); s.requests != 0 || s.conversations != 0 || s.turns != 0 || s.candidateSets != 0 || s.runs != 0 {
		t.Fatalf("沒發布過的需求與買家資料要全部刪除：%+v", s)
	}
	s := snap(ctx, t, d, published)
	if s.requests != 1 || !s.publishedRequestBuyerNull {
		t.Fatalf("已發布的需求保留且 buyer_id 為 NULL：%+v", s)
	}
	for _, tbl := range anonymizedTables {
		if s.anonymized[tbl] != 1 {
			t.Errorf("%s 保留且 buyer_id 為 NULL，實際 %d 列", tbl, s.anonymized[tbl])
		}
	}
	if s.conversations != 0 || s.turns != 0 || s.candidateSets != 0 || s.runs != 0 || s.usage != 0 {
		t.Fatalf("已發布的需求的買家專屬資料（對話、輪次、候選、run、額度）要刪除：%+v", s)
	}
	for _, kept := range []struct{ table, where string }{
		{"rfq_publication_invites", "id = $1"}, {"seller_responses", "id = $1"}, {"rfq_publication_approval_recipients", "approval_id = $1"},
	} {
		arg := map[string]uuid.UUID{"rfq_publication_invites": published.invite, "seller_responses": published.sellerResponse, "rfq_publication_approval_recipients": published.approval}[kept.table]
		if count(ctx, t, d, kept.table, kept.where, arg) != 1 {
			t.Errorf("賣家看得到的 %s 要保留", kept.table)
		}
	}
	if b := snap(ctx, t, d, bystander); b.requests != 1 || b.conversations != 1 || b.turns != 1 || b.runs != 1 || b.usage != 1 || b.publishedRequestBuyerNull {
		t.Fatalf("別的買家不受影響：%+v", b)
	}
	if err := deleteBuyer(); err != nil {
		t.Fatalf("重送要冪等：%v", err)
	}
}

// S10 的原子性：其中一步失敗時，其餘步驟也不得生效（斷言「全部成立」鑑別力為零，所以注入第 N 步的失敗）。
// 這裡讓最後面的額度刪除失敗，確認前面已刪的對話、已匿名的需求都跟著回滾。
func TestBuyerDeletedIsAtomicWhenALaterStepFails(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	q := store.New(d.Pool())
	c := seed(ctx, t, q, 51, true)
	if _, err := d.Pool().Exec(ctx, `
		CREATE FUNCTION procurement.fail_usage_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected'; END $$;
		CREATE TRIGGER fail_usage_delete BEFORE DELETE ON procurement.usage FOR EACH ROW EXECUTE FUNCTION procurement.fail_usage_delete();`); err != nil {
		t.Fatal(err)
	}
	m := New(Deps{DB: d, Now: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	err := d.InTx(ctx, func(ctx context.Context) error { return m.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: 51}) })
	if err == nil {
		t.Fatal("注入的失敗要讓 OnBuyerDeleted 回錯")
	}
	s := snap(ctx, t, d, c)
	if s.conversations != 1 || s.turns != 1 || s.runs != 1 || s.candidateSets != 1 || s.usage != 1 || s.publishedRequestBuyerNull {
		t.Fatalf("一步失敗時其餘步驟必須回滾：%+v", s)
	}
	for _, tbl := range anonymizedTables {
		if s.anonymized[tbl] != 0 {
			t.Errorf("%s 不該已被匿名", tbl)
		}
	}
}

// R2-1：conversion_basis 在契約不是必填，空字串要能寫入；超過 500 個字元仍被擋。
func TestSellerResponseVersionAllowsEmptyConversionBasis(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	q := store.New(d.Pool())
	c := seed(ctx, t, q, 61, true)
	insert := func(version int, basis string) error {
		_, err := d.Pool().Exec(ctx, `
			INSERT INTO procurement.seller_response_versions (
				id, seller_response_id, version, response_type, offered_quality_grade, offered_quality_inspection_notes,
				unit_basis_quantity, unit_basis_unit, offered_quantity, offered_quantity_unit, conversion_basis, currency,
				goods_amount, shipping_amount, tax_amount, tax_inclusion_basis, total_amount, valid_until,
				outside_buyer_requested_range, publication_version, client_request_id, submitted_at, created_at)
			VALUES ($1, $2, $3, 'MANUAL_QUOTE', 'A', '', 1, '件', 1, '件', $4, 'TWD', 100, 0, 0, 'TAX_INCLUDED', 100, $5, false, 1, $6, $5, $5)`,
			id(), c.sellerResponse, version, basis, epoch, id())
		return err
	}
	if err := insert(1, ""); err != nil {
		t.Fatalf("空的 conversion_basis 要能寫入：%v", err)
	}
	if err := insert(2, strings.Repeat("a", 501)); err == nil {
		t.Fatal("超過 500 個字元的 conversion_basis 要被擋")
	}
}

// R2-5：核准被 claim 之後買家被刪除（buyer_id 設 NULL），MarkApprovalSucceeded 仍要能收尾，不能卡在 EXECUTING。
func TestMarkApprovalSucceededSurvivesBuyerDeletion(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	q := store.New(d.Pool())
	c := seed(ctx, t, q, 71, false)
	snapshot, preview, approval := id(), id(), id()
	policyID, policyVersion := "wish-pool", int32(1)
	if n, err := q.InsertPricePreview(ctx, store.InsertPricePreviewParams{
		ID: preview, BuyerID: &c.buyer, RequestID: c.request, RequestRevisionID: c.revision, RangeLowerAmount: 1, RangeUpperAmount: 2,
		BasisQuantity: 1, BasisUnit: "件", ClientRequestID: id(), ExpiresAt: epoch.Add(time.Hour), Now: epoch,
	}); err != nil || n != 1 {
		t.Fatalf("InsertPricePreview n=%d err=%v", n, err)
	}
	if n, err := q.InsertConfirmedSnapshot(ctx, store.InsertConfirmedSnapshotParams{
		ID: snapshot, BuyerID: &c.buyer, RequestID: c.request, RequestRevisionID: c.revision, PricePreviewID: preview, ClientRequestID: id(),
		ExpiresAt: epoch.Add(time.Hour), Now: epoch,
	}); err != nil || n != 1 {
		t.Fatalf("InsertConfirmedSnapshot n=%d err=%v", n, err)
	}
	if n, err := q.InsertApproval(ctx, store.InsertApprovalParams{
		ID: approval, BuyerID: &c.buyer, RequestID: c.request, RequestRevisionID: c.revision, ConfirmedRfqSnapshotID: snapshot, AudienceMode: "WISH_POOL",
		AudiencePolicyID: &policyID, AudiencePolicyVersion: &policyVersion, SchemaVersion: "v1", ClientRequestID: id(), Now: epoch, ExpiresAt: epoch.Add(time.Hour),
	}); err != nil || n != 1 {
		t.Fatalf("InsertApproval n=%d err=%v", n, err)
	}
	claimed, err := q.ClaimApprovalForExecute(ctx, store.ClaimApprovalForExecuteParams{ID: approval, Version: 1, BuyerID: &c.buyer, Now: epoch})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.rfq_publication_approvals SET buyer_id = NULL WHERE id = $1`, approval); err != nil {
		t.Fatal(err)
	}
	if n, err := q.MarkApprovalSucceeded(ctx, store.MarkApprovalSucceededParams{ID: approval, Version: claimed.Version, Now: epoch}); err != nil || n != 1 {
		t.Fatalf("buyer_id 為 NULL 後 MarkApprovalSucceeded n=%d err=%v", n, err)
	}
	if n, _ := q.MarkApprovalSucceeded(ctx, store.MarkApprovalSucceededParams{ID: approval, Version: claimed.Version, Now: epoch}); n != 0 {
		t.Fatal("已 SUCCEEDED 的核准不能再被標一次（CAS）")
	}
}

// A9-D20 R4：run 帳本的 publication_id 是 ON DELETE SET NULL，BuyerDeleted 先刪 run（子表隨 CASCADE），
// 之後匿名保留的發布不再被 run 牽連；agent_runs.buyer_id 維持 NOT NULL，所以 run 只能刪、不能匿名。
func TestBuyerDeletedDeletesRunsBeforeAnonymizingPublications(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	q := store.New(d.Pool())
	c := seed(ctx, t, q, 51, true)
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET publication_id = $1 WHERE id = $2`, c.publication, c.run); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(ctx, `INSERT INTO procurement.agent_run_events (run_id, seq, event_type, occurred_at, created_at) VALUES ($1, 1, 'text', $2, $2)`, c.run, epoch); err != nil {
		t.Fatal(err)
	}
	m := New(Deps{DB: d, Now: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err := d.InTx(ctx, func(ctx context.Context) error { return m.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: 51}) }); err != nil {
		t.Fatal(err)
	}
	for _, tbl := range []string{"agent_runs", "agent_run_inputs", "agent_run_events"} {
		if n := count(ctx, t, d, tbl, "true"); n != 0 {
			t.Errorf("%s 還剩 %d 列", tbl, n)
		}
	}
	if n := count(ctx, t, d, "rfq_publications", "id = $1 AND buyer_id IS NULL", c.publication); n != 1 {
		t.Fatalf("發布要保留並匿名，實際 %d 列", n)
	}
}

func TestDeletingAPublicationClearsTheRunReference(t *testing.T) {
	ctx := context.Background()
	d := newDB(t)
	c := seed(ctx, t, store.New(d.Pool()), 52, true)
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET publication_id = $1 WHERE id = $2`, c.publication, c.run); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool().Exec(ctx, `DELETE FROM procurement.rfq_publications WHERE id = $1`, c.publication); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, d, "agent_runs", "id = $1 AND publication_id IS NULL", c.run); n != 1 {
		t.Fatalf("run 要保留、publication_id 設 NULL，實際 %d 列", n)
	}
}
