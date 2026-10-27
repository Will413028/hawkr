//go:build integration

package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

// ---------------------------------------------------------------------------
// 作廢→重開全鏈路（#2909）的整合驗證。
//
// 🔴 **為什麼一定要有這一層**：本鏈路的失效幾乎全是「DB 層說不」——
// chk_invdoc_versions、uq_invdoc_order_identity、uq_invadj_void_once、
// uq_invcorr_open_per_document、各狀態轉移的 allowed-from 述詞。
// 這些在 unit test 的 fake 上**全部不存在**，而它們正是本 session 反覆踩到的東西
// （MaxAttempts 未設 → 23514、operation 停 CREATED、分支失敗鎖住閘門）。
//
// dev 上跑不了同樣的路徑：AMEGO 共用測試帳號在 2026 年 9-10 月期別
// **沒有 category=1（自動配號）字軌**（實測 track_all：13 條全是 category=2）
// ⇒ 任何 f0401 都回 3040111 字軌不足，開不出當期發票就走不到更正。
// ---------------------------------------------------------------------------

// voidAmego 是可切換回應的 AMEGO 替身：開票、作廢、查詢三支。
type voidAmego struct {
	srv *httptest.Server
	// voidCode 非 0 時 f0501 回該錯誤碼（測 R05）。
	voidCode atomic.Value // string
	// queryType 是 invoice_query 回的 invoice_type，用來模擬 C0401 → C0501 的時間差。
	queryType atomic.Value // string
	issueSeq  atomic.Int64
	voidCalls atomic.Int64
	queryCall atomic.Int64
}

func newVoidAmego(t *testing.T) *voidAmego {
	t.Helper()
	f := &voidAmego{}
	f.voidCode.Store("")
	f.queryType.Store("C0401")
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		switch r.URL.Path {
		case amego.EndpointIssue:
			n := f.issueSeq.Add(1)
			_ = enc.Encode(map[string]any{
				"code": 0, "msg": "",
				"invoice_number": []string{"AB12345678", "AB12345679", "AB12345680"}[n%3],
				"invoice_time":   time.Now().Unix(), "random_number": "4321",
			})
		case amego.EndpointVoid:
			f.voidCalls.Add(1)
			if code, _ := f.voidCode.Load().(string); code != "" {
				_ = enc.Encode(map[string]any{"code": code, "msg": "模擬拒絕"})
				return
			}
			_ = enc.Encode(map[string]any{"code": 0, "msg": ""})
		case "/json/invoice_query":
			f.queryCall.Add(1)
			it, _ := f.queryType.Load().(string)
			_ = enc.Encode(map[string]any{
				"code": 0, "msg": "",
				"data": map[string]any{"invoice_number": "AB12345678", "invoice_type": it},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// voidEnv 是本檔專用的環境：在 setupE2E 的基礎上多接三個 usecase。
type voidEnv struct {
	*e2eEnv
	fake  *voidAmego
	corr  *usecase.Correction
	void  *usecase.Voiding
	docs  domain.DocumentRepository
	ops   domain.OperationRepository
	corrs domain.CorrectionRequestRepository
}

// setupVoidEnv 先跑一次完整的 order.paid → ISSUED，再把作廢軸接上去。
//
// 直接沿用 setupE2E 的種子（issuer／稅務檔／provider account／金鑰環）——
// 那些都是本鏈路的真實前置，另起一套只會讓兩份 seed 各自漂移。
func setupVoidEnv(t *testing.T) *voidEnv {
	t.Helper()
	env := setupE2E(t)
	ctx := context.Background()

	// 換成可切換回應的替身：setupE2E 那個只認 f0401。
	fake := newVoidAmego(t)
	client, err := amego.New(amego.Options{
		BaseURL: fake.srv.URL, HTTPClient: fake.srv.Client(),
		AllowRealInvoice: false, TestInvoice: e2eTaxID,
		RateLimitQPS: 50, Deadlines: amego.DefaultDeadlines(),
	})
	if err != nil {
		t.Fatalf("amego.New: %v", err)
	}

	// 金鑰環必須與 setupE2E 用同一組，否則讀不回它剛加密落庫的收件人資料。
	cur, err := crypto.New("integration-master-key")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	ring := &postgres.KeyRing{Current: cur, CurrentVersion: 1, LookupHMACKey: "integration-lookup-key"}
	docs := postgres.NewDocumentRepository(env.pool, ring)
	ops := postgres.NewOperationRepository(env.pool)
	corrs := postgres.NewCorrectionRequestRepository(env.pool)
	adjs := postgres.NewAdjustmentRepository(env.pool)
	accounts := postgres.NewProviderAccountRepository(env.pool, ring)
	audit := postgres.NewAuditRepository(env.pool)
	tx := database.NewTxManager(env.pool)

	var idSeq int64
	newID := func() int64 { idSeq++; return time.Now().UnixNano()%1_000_000_000_000 + idSeq*97 }
	cfg := usecase.IssuanceConfig{
		MaxAttempts: 13, BaseBackoff: time.Second, MaxBackoff: time.Minute,
		IssueLease: 120 * time.Second, RecheckBase: time.Second,
	}

	// outbox／issuers 是 P3（invoice.voided）的依賴（#3055 的橫向稽查）：
	// 作廢確認成功時要在同一交易內寫事件。
	void := usecase.NewVoiding(tx, docs, ops, corrs, accounts, audit, client,
		postgres.NewOutboxRepository(env.pool), postgres.NewIssuerRepository(env.pool),
		// adjustments：cancelReasonFor 讀它決定送給 AMEGO 的 CancelReason（#3073）。
		// 本檔走 correction 來源（op.AdjustmentID 為 nil）⇒ 真實 repository 傳進去
		// 也不會被查，但傳真的比傳 nil 誠實：日後有人在本檔加一條退款作廢的案例時，
		// 它會直接運作而不是 panic。
		adjs, cfg, nil)
	// audit 傳 nil：本檔驗的是作廢重開的狀態流轉，不驗 I7 留痕
	// （recordCorrectionTransition 對 nil audit 靜默跳過）。
	corr := usecase.NewCorrection(tx, corrs, ops, docs, adjs, noopReviser{}, nil, nil,
		func() time.Time { return time.Now().UTC() }, newID, 13)

	// 🔴 **必須自己建一支 IssueWorker**：setupE2E 那支綁的是 strictVoider（被呼叫到就
	// 判測試失敗），因為開票軸的既有測試不該走到作廢路徑。本檔正是要走那條路。
	// 順帶：它也要用上面那個可切換回應的 AMEGO 替身，setupE2E 的只認 f0401。
	issuance := usecase.NewIssuance(tx,
		postgres.NewIssuerRepository(env.pool),
		postgres.NewSellerTaxProfileRepository(env.pool),
		accounts, docs, ops, corrs,
		postgres.NewOutboxRepository(env.pool), audit, client,
		stubReceiptRenderer{}, stubReceiptUploader{},
		newID, cfg, nil)
	env.issue = NewIssueWorker(ops, docs, issuance, void, strictAllowancer(t), outbox.Config{
		MaxAttempts: 13, BaseBackoff: time.Second, MaxBackoff: time.Minute,
		FetchBatch: 20, ClaimLease: 120 * time.Second,
	}, testMetrics, testObsA, nil)

	_ = ctx
	return &voidEnv{e2eEnv: env, fake: fake, corr: corr, void: void, docs: docs, ops: ops, corrs: corrs}
}

// noopReviser 讓 R03 分支可編譯；本檔只走 R04，不會呼叫到它。
type noopReviser struct{}

func (noopReviser) ReviseOrderInvoiceSnapshot(context.Context, domain.ReviseSnapshotInput) (int64, error) {
	panic("void_reissue_integration_test: R04 不該呼叫 ReviseOrderInvoiceSnapshot")
}

// issueFirstInvoice 走完整的 order.paid → ISSUED，回傳那張 document。
func (e *voidEnv) issueFirstInvoice(t *testing.T) domain.Document {
	t.Helper()
	ctx := context.Background()
	if err := e.paid.Handle(ctx, "order.paid", e2ePaidBody(t, "evt-void-1", e.e2eEnv)); err != nil {
		t.Fatalf("order.paid: %v", err)
	}
	if err := e.issue.RunOnce(ctx); err != nil {
		t.Fatalf("issue worker: %v", err)
	}
	doc, err := e.docs.GetBySourceRef(ctx, e.orderNo)
	if err != nil {
		t.Fatalf("load document: %v", err)
	}
	if doc.Status != codemeta.InvoiceDocumentStatusIssued.Value {
		t.Fatalf("前提不成立：document status=%s, want ISSUED", doc.Status)
	}
	return doc
}

// mustClaimForRecheck 走 production 在 INV.rc 上的**兩步**，測試少了它就對不上真實呼叫順序。
//
// usecase/recheck.go 的迴圈是：ClaimRecheck 先把 DB 那一列蓋成 PROVIDER_STATUS_RECHECK，
// 接著把記憶體裡那份 op.Status 同步跟上（該檔明文標紅的那一行），最後才呼叫 ConfirmVoid。
// 而 MarkInvoiceOperationWaitingConfirmation 的述詞是
// `status IN ('SENDING', 'PROVIDER_STATUS_RECHECK')`——直接拿受理後的 WAITING_CONFIRMATION
// 進去，UPDATE 就是命中 0 列。長紅四次 promote 的成因是這裡，不是 production 壞掉。
func (e *voidEnv) mustClaimForRecheck(t *testing.T, op domain.Operation) domain.Operation {
	t.Helper()
	// claim 的候選述詞要求 `next_attempt_at <= now`；受理時那一列已被排到未來的重試點，
	// 所以要用一個夠晚的 now，否則 claim 會靜默命中 0 列（回 false 而不是 error）。
	now := time.Now().Add(24 * time.Hour)
	ok, err := e.ops.ClaimRecheck(context.Background(), op.ID, now, now.Add(60*time.Second))
	if err != nil {
		t.Fatalf("ClaimRecheck(op=%d): %v", op.ID, err)
	}
	if !ok {
		t.Fatalf("ClaimRecheck(op=%d) 命中 0 列 —— 候選述詞或 next_attempt_at 與預期不符", op.ID)
	}
	op.Status = codemeta.InvoiceOperationStatusProviderStatusRecheck.Value
	return op
}

// TestIntegration_VoidReissue_HappyPath 走完 R04 → f0501 → 確認 → 重開 → R10。
//
// 每一步都對**真的** DB 斷言，因為本鏈路的失效幾乎全是 DB 層說不。
func TestIntegration_VoidReissue_HappyPath(t *testing.T) {
	e := setupVoidEnv(t)
	ctx := context.Background()
	doc := e.issueFirstInvoice(t)

	// ── R04：建 VOID adjustment ＋ VOID operation(READY) ＋ 重開票 ＋ REISSUE op(CREATED)
	res, err := e.corr.Submit(ctx, submitCorrectionFor(doc, e.orderNo))
	if err != nil {
		t.Fatalf("R04 Submit: %v", err)
	}
	if res.Route != usecase.RouteVoidAndReissue {
		t.Fatalf("route=%v, want VOID_AND_REISSUE（前提：document 已 ISSUED 且在可作廢窗口內）", res.Route)
	}

	ops, err := e.ops.ListByDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("list ops: %v", err)
	}
	voidOp := findOpByType(ops, codemeta.InvoiceOperationTypeVoid.Value)
	if voidOp == nil {
		t.Fatal("R04 沒有建 VOID operation")
	}
	if voidOp.Status != codemeta.InvoiceOperationStatusReady.Value {
		t.Fatalf("VOID operation status=%s, want READY —— CREATED 不在任何 worker 的候選集合裡",
			voidOp.Status)
	}

	reissue := e.mustLoadReissue(t, doc)
	if reissue.IssueAttemptSeq != doc.IssueAttemptSeq+1 {
		t.Errorf("重開票 issue_attempt_seq=%d, want %d", reissue.IssueAttemptSeq, doc.IssueAttemptSeq+1)
	}
	if reissue.SnapshotVersion != doc.SnapshotVersion {
		t.Errorf("重開票動了內容軸（snapshot_version %d → %d）—— 那一欄的唯一寫入者是 order-service",
			doc.SnapshotVersion, reissue.SnapshotVersion)
	}
	rOps, _ := e.ops.ListByDocument(ctx, reissue.ID)
	rOp := findOpByType(rOps, codemeta.InvoiceOperationTypeReissue.Value)
	if rOp == nil {
		t.Fatal("沒有 REISSUE operation ⇒ 作廢成功後沒有東西會開新票")
	}
	if rOp.Status != codemeta.InvoiceOperationStatusCreated.Value {
		t.Fatalf("REISSUE operation status=%s, want CREATED —— 它必須等作廢確認", rOp.Status)
	}

	// ── f0501：worker 撿 VOID operation 送出
	if err := e.issue.RunOnce(ctx); err != nil {
		t.Fatalf("issue worker (void): %v", err)
	}
	if n := e.fake.voidCalls.Load(); n != 1 {
		t.Fatalf("f0501 被打了 %d 次, want 1", n)
	}
	voidOp = e.mustGetOp(t, voidOp.ID)
	if voidOp.Status != codemeta.InvoiceOperationStatusWaitingConfirmation.Value {
		t.Fatalf("受理後 VOID operation status=%s, want WAITING_CONFIRMATION", voidOp.Status)
	}
	if got := e.mustGetDoc(t, doc.ID); got.Status != codemeta.InvoiceDocumentStatusVoiding.Value {
		t.Fatalf("document status=%s, want VOIDING（受理 ≠ 已作廢）", got.Status)
	}

	// ── 確認第一輪：AMEGO 還回 C0401 ⇒ 不得提早寫 VOIDED
	if err := e.void.ConfirmVoid(ctx, e.mustClaimForRecheck(t, *voidOp)); err != nil {
		t.Fatalf("ConfirmVoid(C0401): %v", err)
	}
	if got := e.mustGetDoc(t, doc.ID); got.Status != codemeta.InvoiceDocumentStatusVoiding.Value {
		t.Fatalf("C0401 就把 document 寫成 %s —— 那會讓 R10 在原票還沒作廢時開出新票", got.Status)
	}
	if got := e.mustGetOp(t, rOp.ID); got.Status != codemeta.InvoiceOperationStatusCreated.Value {
		t.Fatalf("C0401 就把 REISSUE 推成 %s", got.Status)
	}

	// ── 確認第二輪：翻成 C0501 ⇒ VOIDED ＋ REISSUE → READY
	e.fake.queryType.Store("C0501")
	voidOp = e.mustGetOp(t, voidOp.ID)
	if err := e.void.ConfirmVoid(ctx, e.mustClaimForRecheck(t, *voidOp)); err != nil {
		t.Fatalf("ConfirmVoid(C0501): %v", err)
	}
	if got := e.mustGetDoc(t, doc.ID); got.Status != codemeta.InvoiceDocumentStatusVoided.Value {
		t.Fatalf("確認 C0501 後 document status=%s, want VOIDED", got.Status)
	}
	if got := e.mustGetOp(t, rOp.ID); got.Status != codemeta.InvoiceOperationStatusReady.Value {
		t.Fatalf("REISSUE operation status=%s, want READY —— 沒推它就永遠不會開新票", got.Status)
	}

	// ── 重開：worker 撿 REISSUE operation 開票 ⇒ R10
	if err := e.issue.RunOnce(ctx); err != nil {
		t.Fatalf("issue worker (reissue): %v", err)
	}
	if got := e.mustGetDoc(t, reissue.ID); got.Status != codemeta.InvoiceDocumentStatusIssued.Value {
		t.Fatalf("重開票 status=%s, want ISSUED", got.Status)
	}
	if got := e.mustGetDoc(t, doc.ID); got.Status != codemeta.InvoiceDocumentStatusSuperseded.Value {
		t.Fatalf("R10：原票 status=%s, want SUPERSEDED", got.Status)
	}
	// 🔴 重開票必須沿用**原文件的收件人身分**。
	//
	// 2026-09-01 dev 實測的 P0：`recipient_ref_id` 沒被帶過去 ⇒ 新文件的收件人是 NULL
	// ⇒ 買家在 B05／B09 **看不到取代自己那張的新發票**，只看得到已 SUPERSEDED 的舊的。
	// 完全靜默：欄位可為 NULL、DB 不擋、狀態機一路正確。
	// 端點 9／10 的述詞是 `recipient_type='BUYER' AND recipient_ref_id = $2`，
	// 少了它就是「這張發票不屬於任何人」。
	newDoc := e.mustGetDoc(t, reissue.ID)
	if newDoc.RecipientRefID == nil || doc.RecipientRefID == nil ||
		*newDoc.RecipientRefID != *doc.RecipientRefID {
		t.Fatalf("重開票 recipient_ref_id=%v, want %v（與原票相同）——買家會看不到自己的新發票",
			newDoc.RecipientRefID, doc.RecipientRefID)
	}
	c, err := e.corrs.GetByID(ctx, res.CorrectionID)
	if err != nil {
		t.Fatalf("load correction: %v", err)
	}
	if c.Status != codemeta.InvoiceCorrectionStatusCompleted.Value {
		t.Fatalf("R10：correction status=%s, want COMPLETED", c.Status)
	}
}

// TestIntegration_VoidReissue_ProviderRejects 走 R05：作廢被拒。
//
// 三個斷言對應三個各自獨立的靜默失效：document 卡在 VOIDING、買家看不到理由、
// 重開票變成沒人會撿的 PENDING。
func TestIntegration_VoidReissue_ProviderRejects(t *testing.T) {
	e := setupVoidEnv(t)
	ctx := context.Background()
	doc := e.issueFirstInvoice(t)

	res, err := e.corr.Submit(ctx, submitCorrectionFor(doc, e.orderNo))
	if err != nil {
		t.Fatalf("R04 Submit: %v", err)
	}
	reissue := e.mustLoadReissue(t, doc)

	e.fake.voidCode.Store("3050141") // 已存在折讓單 ⇒ ALLOWANCE_EXISTS
	if err := e.issue.RunOnce(ctx); err != nil {
		t.Fatalf("issue worker (void): %v", err)
	}

	if got := e.mustGetDoc(t, doc.ID); got.Status != codemeta.InvoiceDocumentStatusIssued.Value {
		t.Fatalf("R05：原票 status=%s, want ISSUED —— 留在 VOIDING 就沒有掃描器會撿它", got.Status)
	}
	c, err := e.corrs.GetByID(ctx, res.CorrectionID)
	if err != nil {
		t.Fatalf("load correction: %v", err)
	}
	if c.Status != codemeta.InvoiceCorrectionStatusRejected.Value {
		t.Fatalf("R05：correction status=%s, want REJECTED", c.Status)
	}
	if c.RejectReasonCode == nil || *c.RejectReasonCode != codemeta.InvoiceCorrectionRejectReasonAllowanceExists.Value {
		t.Fatalf("R05：reject_reason_code=%v, want ALLOWANCE_EXISTS（3050141）", c.RejectReasonCode)
	}
	if got := e.mustGetDoc(t, reissue.ID); got.Status != codemeta.InvoiceDocumentStatusCancelledBeforeIssue.Value {
		t.Fatalf("R05：重開票 status=%s, want CANCELLED_BEFORE_ISSUE —— 留著就是一張沒人會撿的 PENDING", got.Status)
	}
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

// submitCorrectionFor 組出 R04 的輸入：把 B2C 改成公司統編。
//
// Facts 直接給定而不是走 GatherCorrectionFacts：那一支要打 AMEGO 查字軌餘量，
// 而本檔的替身沒有 track_status。決策路由本身有自己的 unit test（correction_decide_test），
// 這裡要驗的是**路由之後**的落地。
func submitCorrectionFor(doc domain.Document, orderNo string) usecase.SubmitCorrectionInput {
	name := "整合測試股份有限公司"
	taxID := "12345678"
	return usecase.SubmitCorrectionInput{
		DocumentID:              doc.ID,
		BuyerID:                 2623900520,
		ExpectedSnapshotVersion: doc.SnapshotVersion,
		BeforePayload:           []byte(`{"buyerName":"消費者","buyerTaxId":"0000000000"}`),
		AfterPayload:            []byte(`{"buyerName":"整合測試股份有限公司","buyerTaxId":"12345678"}`),
		IssueAttemptSeq:         doc.IssueAttemptSeq,
		OrderNo:                 orderNo,
		Facts: usecase.CorrectionFacts{
			DocumentStatus:      codemeta.InvoiceDocumentStatusIssued.Value,
			TrackSufficient:     true,
			CarrierValid:        true,
			WithinVoidDeadline:  true,
			NoExistingAllowance: true,
			ProviderAvailable:   true,
			AlreadyReported:     false,
		},
		Revised: domain.ReviseDocumentSnapshot{
			RecipientType: codemeta.InvoiceRecipientTypeBuyer.Value,
			// 🔴 **鏡射 correction_intake.go 的建構**：它從原文件沿用收件人身分。
			// 這裡若也省略，測試就會餵一個 nil 進去、然後斷言 nil ——
			// 自己跟自己對，2026-09-01 那個 P0 正是這樣溜過去的。
			RecipientRefID: doc.RecipientRefID,
			RecipientName:  name,
			RecipientTaxID: taxID,
		},
		RevisedLines: []domain.NewDocumentLine{{
			LineSeq: 1, LineType: codemeta.InvoiceLineTypeProduct.Value,
			Description: "整合測試商品", Quantity: 1, UnitPrice: 100, NetAmount: 100,
			TaxCategory: 1, TaxRateBps: 500, AllowanceableBase: 100,
		}},
	}
}

func findOpByType(ops []domain.Operation, t string) *domain.Operation {
	for i := range ops {
		if ops[i].OperationType == t {
			return &ops[i]
		}
	}
	return nil
}

func (e *voidEnv) mustGetDoc(t *testing.T, id int64) domain.Document {
	t.Helper()
	d, err := e.docs.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID(%d): %v", id, err)
	}
	return d
}

func (e *voidEnv) mustGetOp(t *testing.T, id int64) *domain.Operation {
	t.Helper()
	o, err := e.ops.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetOpByID(%d): %v", id, err)
	}
	return &o
}

// mustLoadReissue 用**決定性的 provider_request_no** 反查重開票 —— 與正式碼同一條路徑，
// 而不是掃 original_document_id（那沒有索引，且會讓測試與實作用不同的查法）。
func (e *voidEnv) mustLoadReissue(t *testing.T, orig domain.Document) domain.Document {
	t.Helper()
	no := domain.ProviderRequestNoForOrder(e.orderNo, orig.SnapshotVersion, orig.IssueAttemptSeq+1)
	d, err := e.docs.GetByProviderRequestNo(context.Background(), no)
	if err != nil {
		t.Fatalf("重開票查不到（provider_request_no=%s）：%v —— R04 沒有建它", no, err)
	}
	return d
}

// ---------------------------------------------------------------------------
// 待審接單額度的累加（#2900 的真正前置）
// ---------------------------------------------------------------------------

// 待審中的賣家接單時，pending_order_count／amount 必須在**建 document 的同交易內**累加。
//
// 🔴 在 2026-09-01 之前 BumpSellerTaxProfilePendingOrderCounters 的非生成呼叫端是 **0**
// ——SQL、repo 方法與欄位註解都在，而沒有任何執行者。後果是待審接單雙上限
// （§6.1.2 C5、業務碼 9326）**結構上永遠不會觸發**：計數器恆 0、Blocked 恆 false。
// 那使得「把 ValidateOrderInvoiceInputInternal 接進結帳」（#2900）變成裝一個永遠
// 不會拒絕的檢查。
//
// 用 integration test 而不是 unit test：要驗的是「同一交易內」與「恰好一次」，
// 兩者都只有在真的 DB 上才有意義。
func TestIntegration_PendingIntakeCountersBumpOnDocumentCreate(t *testing.T) {
	e := setupVoidEnv(t)
	ctx := context.Background()

	// 把賣家改成待審（SELF_ISSUE_PENDING）＋ 給一個足夠大的上限，避免撞到別的分支。
	if _, err := e.pool.Exec(ctx, `
		UPDATE seller_tax_profiles
		   SET review_status = 'SELF_ISSUE_PENDING',
		       pending_order_limit_count = 999, pending_order_limit_amount = 9999999,
		       pending_order_count = 0, pending_order_amount = 0,
		       pending_valid_until = NOW() + interval '30 days',
		       -- chk_stp_attested_when_committed：SELF_ISSUE_PENDING 必須有這兩欄
		       attestation_version = 'v1', attested_at = NOW(),
		       -- chk_stp_pending_window：待審狀態必須有 submitted_at ＋ pending_valid_until
		       submitted_at = NOW() - interval '1 day'
		 WHERE issuer_id = $1`, e.issuerID); err != nil {
		t.Fatalf("seed pending profile: %v", err)
	}

	if err := e.paid.Handle(ctx, "order.paid", e2ePaidBody(t, "evt-intake-1", e.e2eEnv)); err != nil {
		t.Fatalf("order.paid: %v", err)
	}

	var cnt int32
	var amt int64
	row := e.pool.QueryRow(ctx,
		`SELECT pending_order_count, pending_order_amount FROM seller_tax_profiles WHERE issuer_id = $1`, e.issuerID)
	if err := row.Scan(&cnt, &amt); err != nil {
		t.Fatalf("read counters: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("pending_order_count = %d, want 1 —— 沒累加的話 9326 永遠不會觸發", cnt)
	}
	if amt <= 0 {
		t.Fatalf("pending_order_amount = %d, want > 0", amt)
	}

	// 冪等重入：同一筆 order.paid 再來一次，計數**不得**再加。
	if err := e.paid.Handle(ctx, "order.paid", e2ePaidBody(t, "evt-intake-1", e.e2eEnv)); err != nil {
		t.Fatalf("order.paid (retry): %v", err)
	}
	var cnt2 int32
	if err := e.pool.QueryRow(ctx,
		`SELECT pending_order_count FROM seller_tax_profiles WHERE issuer_id = $1`, e.issuerID).Scan(&cnt2); err != nil {
		t.Fatalf("read counters again: %v", err)
	}
	if cnt2 != 1 {
		t.Fatalf("重投之後 pending_order_count = %d, want 1 —— 冪等重入重複計數會讓賣家被誤擋", cnt2)
	}
}

// 非待審的賣家**不得**累加。
//
// 🔴 這兩個計數器沒有任何遞減或重置路徑（遞減需要有人在窗口滑出時掃描，那是
// 《後端服務責任邊界規範》禁止的 reconciliation job）⇒ 無條件累加會讓一個歷史悠久的
// 賣家哪天進入待審時被累積總數瞬間擋死，而症狀是結帳回 9326、查不出原因。
func TestIntegration_ApprovedSellerDoesNotBumpIntakeCounters(t *testing.T) {
	e := setupVoidEnv(t)
	ctx := context.Background()

	if err := e.paid.Handle(ctx, "order.paid", e2ePaidBody(t, "evt-intake-2", e.e2eEnv)); err != nil {
		t.Fatalf("order.paid: %v", err)
	}
	var cnt int32
	if err := e.pool.QueryRow(ctx,
		`SELECT pending_order_count FROM seller_tax_profiles WHERE issuer_id = $1`, e.issuerID).Scan(&cnt); err != nil {
		t.Fatalf("read counters: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("已核准的賣家 pending_order_count = %d, want 0", cnt)
	}
}
