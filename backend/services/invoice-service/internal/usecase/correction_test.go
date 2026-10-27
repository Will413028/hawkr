package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// Correction.Execute（§7.3.5 R02／R03）。
//
// 這支的價值全在**順序**與**失敗時停在哪**：每一個中間態都沒有人會發現。
// 逐條對應本檔守的不變式：
//   ① Hold 一定在跨服務呼叫之前 —— 反了就是「對方已用舊內容開票，我們才改快照」
//   ② 跨服務呼叫在交易之外 —— 包進去等於把 DB 連線壽命綁在對方的 deadline 上
//   ③ Hold 搶不到 ⇒ 改走 R04，**不是重試**
//   ④ order-service 失敗時**不主動放掉 HELD**（讓租約做它的事）
//   ⑤ 落地四件事同一交易，任一 rows=0 就整筆 rollback

type corrOps struct {
	domain.OperationRepository
	holdOK         bool
	holdErr        error
	holdCalls      int
	releaseOK      bool
	releaseErr     error
	releaseCall    int
	createCalls    int
	gotHoldID      int64
	promoteCalls   int
	promoteOK      bool
	gotPromoteID   int64
	gotPromoteTo   string
	gotOpCreatedID int64
	gotPromoteType string
	opTypeByID     map[int64]string
	keyByType      map[string]string
	gotOpKey       string
	gotOpType      string
	// order 記錄呼叫順序，用來斷言 Hold 在 gRPC 之前
	order *[]string

	// voidGateHeld 模擬 VOID operation 撞 uq_invop_open_adjustment_per_document
	// （I5 序列閘門被別的調整佔用）：Create 回 inserted=false，且那把 key
	// **不進 byKey** —— 真實 DB 上那一列根本沒插進去，所以之後也查不到。
	voidGateHeld bool
	// byKey 是 fake 的 uq_invop_operation_key，讓 GetByKey 分得出兩種 rows=0。
	byKey map[string]domain.Operation
}

func (f *corrOps) GetByKey(_ context.Context, key string) (domain.Operation, error) {
	op, ok := f.byKey[key]
	if !ok {
		return domain.Operation{}, errors.New("fake: operation_key not found")
	}
	return op, nil
}

func (f *corrOps) Hold(_ context.Context, id int64, _, _ time.Time) (bool, error) {
	f.holdCalls++
	f.gotHoldID = id
	*f.order = append(*f.order, "hold")
	return f.holdOK, f.holdErr
}

func (f *corrOps) Create(_ context.Context, in domain.NewOperation) (domain.Operation, bool, error) {
	f.createCalls++
	f.gotOpKey = in.OperationKey
	f.gotOpCreatedID = in.ID
	if f.opTypeByID == nil {
		f.opTypeByID = map[int64]string{}
	}
	f.opTypeByID[in.ID] = in.OperationType
	if f.keyByType == nil {
		f.keyByType = map[string]string{}
	}
	f.keyByType[in.OperationType] = in.OperationKey
	f.gotOpType = in.OperationType
	*f.order = append(*f.order, "create_operation")
	// 🔴 fake 的鑑別力來自它**拒絕**什麼。照單全收的版本讓 MaxAttempts=0 一路過關，
	// 而真的 DB 有 chk_invop_attempts（max_attempts >= 1）
	// ⇒ 2026-08-31 dev 首次實打就 23514，單元測試全綠。
	// 這裡把該約束搬進 fake：同一個錯誤不可能再溜過去。
	if in.MaxAttempts < 1 {
		return domain.Operation{}, false,
			fmt.Errorf("fake: chk_invop_attempts violated: max_attempts=%d (must be >= 1)", in.MaxAttempts)
	}
	if in.OperationKey == "" {
		return domain.Operation{}, false, errors.New("fake: operation_key must not be empty")
	}
	if f.byKey == nil {
		f.byKey = map[string]domain.Operation{}
	}
	if f.voidGateHeld && in.OperationType == codemeta.InvoiceOperationTypeVoid.Value {
		return domain.Operation{}, false, nil
	}
	op := domain.Operation{ID: in.ID}
	f.byKey[in.OperationKey] = op
	return op, true, nil
}

func (f *corrOps) Promote(_ context.Context, id int64, next string, _, _ time.Time) (bool, error) {
	f.promoteCalls++
	f.gotPromoteID, f.gotPromoteTo = id, next
	f.gotPromoteType = f.opTypeByID[id]
	*f.order = append(*f.order, "promote_operation")
	// 🔴 fake 拒絕錯的目標值：CREATED 之外的任何狀態都不在 worker 的候選集合裡，
	// 推錯了那筆作廢一樣送不出去，而 DB 不會擋（Promote 的述詞只看來源狀態）。
	if next != codemeta.InvoiceOperationStatusReady.Value {
		return false, fmt.Errorf("fake: void operation must be promoted to READY, got %q", next)
	}
	return f.promoteOK, nil
}

func (f *corrOps) ReleaseHold(_ context.Context, _ int64, _ time.Time) (bool, error) {
	f.releaseCall++
	*f.order = append(*f.order, "release")
	return f.releaseOK, f.releaseErr
}

type corrDocs struct {
	domain.DocumentRepository
	reviseOK   bool
	reviseErr  error
	deleteCall int
	insertCall int
	reviseCall int
	lockCalls  int
	gotRevise  domain.ReviseDocumentSnapshot
	gotLines   []domain.NewDocumentLine
	lockErr    error
	// docSnapshotVersion 是 LockForCorrection 回的版本，用來測 stale 判定
	docSnapshotVersion int32
	order              *[]string

	// 重開票（R04 ③）
	orig             domain.Document
	createdDoc       domain.NewDocument
	createCalls      int
	createInserted   bool
	insertLineDocIDs []int64
}

func (f *corrDocs) LockForCorrection(context.Context, int64) (domain.Document, error) {
	f.lockCalls++
	*f.order = append(*f.order, "lock_document")
	return domain.Document{ID: 2002, SnapshotVersion: f.docSnapshotVersion}, f.lockErr
}

func (f *corrDocs) DeleteLines(context.Context, int64) (int64, error) {
	f.deleteCall++
	*f.order = append(*f.order, "delete_lines")
	return 0, nil
}

// InsertLines 同時服務 R03（改寫原票明細）與 R04 ③（寫重開票明細）。
//
// 兩者都叫 InsertLines，**靠 documentID 分辨**：R03 寫回原票、R04 寫進新票。
// 不分辨的話「明細寫錯票」這個錯誤在測試裡完全看不出來，而它的後果是新票
// 開出來的品項是空的（或原票的明細被重複寫一次）。
func (f *corrDocs) InsertLines(_ context.Context, documentID int64, lines []domain.NewDocumentLine, _ time.Time) error {
	f.insertCall++
	f.gotLines = lines
	f.insertLineDocIDs = append(f.insertLineDocIDs, documentID)
	*f.order = append(*f.order, "insert_lines")
	return nil
}

// ReviseSnapshot **鏡射 invoice_documents 上與買受人／金額有關的五條 CHECK**。
//
// 🔴 為什麼替身要做這件事（#3089 B 組 A-3）：R03 是就地 UPDATE，而
// ReviseInvoiceDocumentSnapshot 在 2026-09-04 之前不 SET document_kind 也不 SET
// 金額四欄 ⇒ 買家在 B2C 文件上補打統編（**端點 11 的主要使用情境**）會讓
// recipient_tax_id 變成 '12345678' 而 document_kind 仍是 'B2C'
// ⇒ chk_invdoc_b2c 當場 23514 ⇒ 買家看到 500。
//
// 替身不鏡射就是「餵什麼是什麼」，那個缺陷在單元測試裡完全看不見
// ——而 R04 的同一個症狀當初是**integration test** 才抓到的（見 buildReissueDocument
// 的註解）。把約束搬進替身，unit test 這一層就接得住。
func (f *corrDocs) ReviseSnapshot(_ context.Context, in domain.ReviseDocumentSnapshot) (bool, error) {
	f.reviseCall++
	f.gotRevise = in
	*f.order = append(*f.order, "revise_doc")
	if err := assertDocumentCheckConstraints(in); err != nil {
		return false, err
	}
	return f.reviseOK, f.reviseErr
}

// assertDocumentCheckConstraints 逐字鏡射 db/schema/schema.sql 的五條 CHECK。
func assertDocumentCheckConstraints(in domain.ReviseDocumentSnapshot) error {
	// chk_invdoc_taxid_shape：^([0-9]{8}|0000000000)$
	if !reDocTaxIDShape.MatchString(in.RecipientTaxID) {
		return fmt.Errorf("corrDocs: 違反 chk_invdoc_taxid_shape（真實 DB 回 23514）：recipient_tax_id=%q", in.RecipientTaxID)
	}
	// chk_invdoc_b2c：B2C ⇒ tax_amount = 0 AND recipient_tax_id = '0000000000'
	if in.DocumentKind == codemeta.InvoiceDocumentKindB2c.Value &&
		(in.TaxAmount != 0 || in.RecipientTaxID != domain.B2CBuyerIdentifier) {
		return fmt.Errorf("corrDocs: 違反 chk_invdoc_b2c（真實 DB 回 23514）："+
			"tax_amount=%d recipient_tax_id=%q", in.TaxAmount, in.RecipientTaxID)
	}
	// chk_invdoc_b2b：B2B ⇒ recipient_tax_id ~ ^[0-9]{8}$
	if in.DocumentKind == codemeta.InvoiceDocumentKindB2b.Value && !reDocB2BTaxID.MatchString(in.RecipientTaxID) {
		return fmt.Errorf("corrDocs: 違反 chk_invdoc_b2b（真實 DB 回 23514）：recipient_tax_id=%q", in.RecipientTaxID)
	}
	// chk_invdoc_total：total = sales + free + zero + tax
	if in.TotalAmount != in.SalesAmount+in.FreeTaxSalesAmount+in.ZeroTaxSalesAmount+in.TaxAmount {
		return fmt.Errorf("corrDocs: 違反 chk_invdoc_total（真實 DB 回 23514）："+
			"total=%d sales=%d free=%d zero=%d tax=%d",
			in.TotalAmount, in.SalesAmount, in.FreeTaxSalesAmount, in.ZeroTaxSalesAmount, in.TaxAmount)
	}
	// chk_invdoc_amounts_nonneg
	if in.SalesAmount < 0 || in.FreeTaxSalesAmount < 0 || in.ZeroTaxSalesAmount < 0 ||
		in.TaxAmount < 0 || in.TotalAmount < 0 {
		return fmt.Errorf("corrDocs: 違反 chk_invdoc_amounts_nonneg（真實 DB 回 23514）：%+v", in)
	}
	return nil
}

var (
	reDocTaxIDShape = regexp.MustCompile(`^([0-9]{8}|0000000000)$`)
	reDocB2BTaxID   = regexp.MustCompile(`^[0-9]{8}$`)
)

type corrReqs struct {
	domain.CorrectionRequestRepository
	rejectOK           bool
	markOK             bool
	gotReason          string
	rejectCalls        int
	markCalls          int
	startVoidCalls     int
	startVoidOK        bool
	gotVoidAdjID       int64
	insertCalls        int
	insertCreated      bool
	gotExpectedVersion int32
	order              *[]string
}

func (f *corrReqs) Insert(_ context.Context, in domain.NewCorrectionRequest) (domain.CorrectionRequest, bool, error) {
	f.insertCalls++
	f.gotExpectedVersion = in.ExpectedSnapshotVersion
	*f.order = append(*f.order, "insert_correction")
	return domain.CorrectionRequest{ID: in.ID}, f.insertCreated, nil
}

func (f *corrReqs) Reject(_ context.Context, in domain.RejectCorrection) (bool, error) {
	f.rejectCalls++
	f.gotReason = in.ReasonCode
	*f.order = append(*f.order, "reject")
	return f.rejectOK, nil
}

func (f *corrReqs) StartVoidReissue(_ context.Context, _, adjustmentID int64, _ time.Time) (bool, error) {
	f.startVoidCalls++
	f.gotVoidAdjID = adjustmentID
	*f.order = append(*f.order, "start_void_reissue")
	return f.startVoidOK, nil
}

func (f *corrReqs) MarkUpdatedBeforeIssue(context.Context, int64, time.Time) (bool, error) {
	f.markCalls++
	*f.order = append(*f.order, "mark")
	return f.markOK, nil
}

type corrAdjs struct {
	domain.AdjustmentRepository
	created    bool
	existingID int64
	calls      int
	gotType    string
	gotStatus  string
	listed     []domain.Adjustment
	order      *[]string
}

func (f *corrAdjs) Insert(_ context.Context, in domain.NewAdjustment) (domain.Adjustment, bool, error) {
	f.calls++
	f.gotType, f.gotStatus = in.AdjustmentType, in.Status
	*f.order = append(*f.order, "insert_adjustment")
	if !f.created {
		return domain.Adjustment{ID: f.existingID}, false, nil
	}
	return domain.Adjustment{ID: in.ID}, true, nil
}

type corrOrders struct {
	version int64
	err     error
	calls   int
	order   *[]string
	// got 是最後一次送出的內容。舊版整包丟掉 ⇒ 「四個欄位從未賦值」這個缺陷
	// （#3085，R03 從上線起 100% 失敗）在單元測試裡完全看不見。
	got domain.ReviseSnapshotInput
}

// ReviseOrderInvoiceSnapshot **鏡射 order-service 的必填驗證**。
//
// 🔴 規則從對端原始碼讀出來，不是從錯誤訊息猜的：
// OrderGrpcService.reviseOrderInvoiceSnapshotInternal 對 invoice_kind 與
// carrier_selection_mode 各跑一次 CodeMetaValidator.requireValidValue
// （必填 ＋ 值域）。少了任一個就是 gRPC 400，而 invoice-service 把它包成 500 給買家。
//
// 替身不鏡射它 ⇒ 驗的是替身不是對端，而 #3085 正是這樣活到 dev 的：
// invoice_correction_requests 全表沒有任何一筆到過 UPDATED_BEFORE_ISSUE。
func (f *corrOrders) ReviseOrderInvoiceSnapshot(_ context.Context, in domain.ReviseSnapshotInput) (int64, error) {
	f.calls++
	f.got = in
	*f.order = append(*f.order, "grpc_revise")
	if _, ok := codemeta.InvoiceDocumentKindFromValue(in.InvoiceKind); !ok {
		return 0, fmt.Errorf("corrOrders: order-service 對 invoiceKind 走 "+
			"CodeMetaValidator.requireValidValue（必填＋值域），實得 %q ⇒ 真實環境回 400", in.InvoiceKind)
	}
	if _, ok := codemeta.InvoiceCarrierSelectionModeFromValue(in.CarrierSelectionMode); !ok {
		return 0, fmt.Errorf("corrOrders: order-service 對 carrierSelectionMode 走 "+
			"CodeMetaValidator.requireValidValue，實得 %q ⇒ 真實環境回 400", in.CarrierSelectionMode)
	}
	return f.version, f.err
}

type corrHarness struct {
	uc     *Correction
	ops    *corrOps
	docs   *corrDocs
	reqs   *corrReqs
	orders *corrOrders
	adjs   *corrAdjs
	calls  *[]string
	audit  *fakeAudit
}

func newCorrHarness(t *testing.T) *corrHarness {
	t.Helper()
	seq := &[]string{}
	ops := &corrOps{holdOK: true, releaseOK: true, promoteOK: true, order: seq}
	docs := &corrDocs{reviseOK: true, docSnapshotVersion: 1, order: seq, createInserted: true}
	reqs := &corrReqs{rejectOK: true, markOK: true, startVoidOK: true, insertCreated: true, order: seq}
	orders := &corrOrders{version: 2, order: seq}
	adjs := &corrAdjs{created: true, order: seq}
	at := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	audit := &fakeAudit{}
	return &corrHarness{
		uc: NewCorrection(directTx{}, reqs, ops, docs, adjs, orders, audit, zap.NewNop(),
			func() time.Time { return at }, func() int64 { return 9001 }, 5),
		ops: ops, docs: docs, reqs: reqs, orders: orders, adjs: adjs, calls: seq, audit: audit,
	}
}

func issuedFacts() CorrectionFacts {
	return CorrectionFacts{
		DocumentStatus:  codemeta.InvoiceDocumentStatusIssued.Value,
		TrackSufficient: true, CarrierValid: true, WithinVoidDeadline: true,
		NoExistingAllowance: true, ProviderAvailable: true,
	}
}

func pendingFacts() CorrectionFacts {
	f := issuedFacts()
	f.DocumentStatus = codemeta.InvoiceDocumentStatusPending.Value
	f.OperationOpen = true
	return f
}

func execInput(f CorrectionFacts) ExecuteCorrectionInput {
	return ExecuteCorrectionInput{
		CorrectionID: 1001, DocumentID: 2002, DocumentVersion: 3, OperationID: 3003,
		OrderNo: "O20260831001", ExpectedSnapshotVersion: 1, Facts: f,
		// 🔴 OrderRevision 的兩個必填欄在 2026-09-04 之前**整包缺席**，而替身把輸入
		// 丟掉 ⇒ 這三支測試對「R03 送出去必被 order-service 回 400」是綠的。
		// 那正是 #3085 活到 dev 的原因（R03 從上線起 100% 失敗）。
		// 規則見 corrOrders.ReviseOrderInvoiceSnapshot 的註解（讀自對端原始碼）。
		OrderRevision: domain.ReviseSnapshotInput{
			OrderNo:                 "O20260831001",
			ExpectedSnapshotVersion: 1,
			InvoiceKind:             codemeta.InvoiceDocumentKindB2b.Value,
			CarrierSelectionMode:    codemeta.InvoiceCarrierSelectionModeNone.Value,
			BuyerTaxID:              "12345678",
			BuyerTitle:              "整合測試股份有限公司",
			IdempotencyKey:          "correction-1001",
		},
		// 修正後的買受人：個人 → 公司統編。R04 的重開票要依它重算 kind 與表頭；
		// R03 就地更新也一樣（#3089 B 組 A-3：不重算就是 chk_invdoc_b2c 的 23514）。
		Revised: domain.ReviseDocumentSnapshot{
			RecipientType:  codemeta.InvoiceRecipientTypeBuyer.Value,
			RecipientName:  "整合測試股份有限公司",
			RecipientTaxID: "12345678",
			DocumentKind:   codemeta.InvoiceDocumentKindB2b.Value,
			SalesAmount:    1000, TaxAmount: 50, TotalAmount: 1050,
		},
		RevisedLines: []domain.NewDocumentLine{{
			LineSeq: 1, LineType: codemeta.InvoiceLineTypeProduct.Value,
			Description: "測試商品", Quantity: 1, UnitPrice: 1050, NetAmount: 1050,
			TaxCategory: 1, TaxRateBps: 500, AllowanceableBase: 1050,
		}},
	}
}

func TestCorrectionExecute_R03_OrderAndTransactionBoundary(t *testing.T) {
	h := newCorrHarness(t)

	got, err := h.uc.Execute(context.Background(), execInput(pendingFacts()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Route != RouteUpdateBeforeIssue || got.NewSnapshotVersion != 2 {
		t.Fatalf("result = %+v, want R03 ＋ version 2", got)
	}

	want := []string{"hold", "grpc_revise", "delete_lines", "insert_lines", "revise_doc", "release", "mark"}
	if len(*h.calls) != len(want) {
		t.Fatalf("呼叫序列 = %v, want %v", *h.calls, want)
	}
	for i := range want {
		if (*h.calls)[i] != want[i] {
			t.Fatalf("呼叫序列 = %v, want %v（第 %d 步不同）", *h.calls, want, i)
		}
	}

	// 🔴 這兩條是本測試存在的理由，不是順帶：
	if (*h.calls)[0] != "hold" || (*h.calls)[1] != "grpc_revise" {
		t.Error("Hold 必須在跨服務呼叫之前——反了就是「對方已用舊內容開票，我們才改快照」")
	}
	// snapshot_version 只能來自 gRPC 回傳（I-50），不得自己算
	if h.orders.calls != 1 {
		t.Errorf("gRPC 呼叫 %d 次, want 1（版本號的唯一來源）", h.orders.calls)
	}
}

func TestCorrectionExecute_HoldLostIsExplicitNotRetry(t *testing.T) {
	h := newCorrHarness(t)
	h.ops.holdOK = false

	_, err := h.uc.Execute(context.Background(), execInput(pendingFacts()))
	if !errors.Is(err, ErrCorrectionSendInFlight) {
		t.Fatalf("error = %v, want ErrCorrectionSendInFlight（搶不到 HELD 且 document 尚未 ISSUED ⇒ 兩條邊都不成立）", err)
	}
	if h.orders.calls != 0 {
		t.Errorf("搶不到 HELD 就不該打 order-service（實得 %d 次）", h.orders.calls)
	}
	if h.docs.reviseCall != 0 || h.reqs.markCalls != 0 {
		t.Error("搶不到 HELD 不得有任何落地寫入")
	}
}

func TestCorrectionExecute_OrderFailureLeavesHoldToLease(t *testing.T) {
	h := newCorrHarness(t)
	h.orders.err = errors.New("amego peer down")

	if _, err := h.uc.Execute(context.Background(), execInput(pendingFacts())); err == nil {
		t.Fatal("order-service 失敗必須往上拋")
	}
	// 🔴 刻意不主動 ReleaseHold：我們剛證明對外呼叫會失敗，此刻最不該做的是再賭一次
	// DB 也還活著。租約 60 秒後由 ReleaseExpiredHolds 掃回。
	if h.ops.releaseCall != 0 {
		t.Errorf("不得主動放掉 HELD（實得 %d 次）——讓租約做它本來就要做的事", h.ops.releaseCall)
	}
	if h.docs.deleteCall != 0 {
		t.Error("跨服務失敗後不得動任何明細")
	}
}

func TestCorrectionExecute_DocumentDriftRollsBackEverything(t *testing.T) {
	h := newCorrHarness(t)
	h.docs.reviseOK = false // rows=0：版本被推進或已離開 PENDING

	if _, err := h.uc.Execute(context.Background(), execInput(pendingFacts())); err == nil {
		t.Fatal("document CAS rows=0 必須整筆失敗——放行會留下「明細換了但快照沒換」")
	}
	if h.reqs.markCalls != 0 {
		t.Error("document 沒改成功就不得把 correction 標成終態")
	}
}

func TestCorrectionExecute_R02RejectsWithRoutedReason(t *testing.T) {
	f := issuedFacts()
	f.AlreadyReported = true
	h := newCorrHarness(t)

	got, err := h.uc.Execute(context.Background(), execInput(f))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Route != RouteReject {
		t.Fatalf("Route = %v, want RouteReject", got.Route)
	}
	if got.RejectReason != codemeta.InvoiceCorrectionRejectReasonAlreadyReported.Value {
		t.Errorf("RejectReason = %q, want ALREADY_REPORTED（買家直接讀到這個碼）", got.RejectReason)
	}
	if h.ops.holdCalls != 0 || h.orders.calls != 0 {
		t.Error("婉拒路徑不得碰 operation 或 order-service")
	}
}

func TestCorrectionExecute_R04CreatesVoidAdjustmentThenOperation(t *testing.T) {
	h := newCorrHarness(t)

	got, err := h.uc.Execute(context.Background(), execInput(issuedFacts()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Route != RouteVoidAndReissue {
		t.Fatalf("Route = %v, want RouteVoidAndReissue", got.Route)
	}

	want := []string{
		"insert_adjustment", "create_operation", "promote_operation",
		// R04 ③：重開票與它的 REISSUE operation 在**同一個交易**內就建好。
		// 等作廢成功之後才建是行不通的——修正後的收件人明文只存在於這一刻
		// （after_payload 已遮罩，而 email 可改）。
		"create_reissue_document", "insert_lines", "create_operation",
		"start_void_reissue",
	}
	if len(*h.calls) != len(want) {
		t.Fatalf("呼叫序列 = %v, want %v", *h.calls, want)
	}
	for i := range want {
		if (*h.calls)[i] != want[i] {
			t.Fatalf("呼叫序列 = %v, want %v", *h.calls, want)
		}
	}
	if h.adjs.gotType != codemeta.InvoiceAdjustmentTypeVoid.Value {
		t.Errorf("adjustment type = %q, want VOID", h.adjs.gotType)
	}
	// 🔴 operation_key 走 void:{adjustment_id}（實作版），不是設計圖的
	// void:{document_id}:{correction_version}。
	// 逐型別取：同一個交易裡建了兩筆 operation（VOID 與 REISSUE），
	// 只看「最後一次」會取到 REISSUE，而那正是 2026-09-01 這支測試抓到的事。
	if got := h.ops.keyByType[codemeta.InvoiceOperationTypeVoid.Value]; got != "void:9001" {
		t.Errorf("VOID operation_key = %q, want void:9001（綁 adjustment 而非 document）", got)
	}
	if got := h.ops.keyByType[codemeta.InvoiceOperationTypeReissue.Value]; got == "" {
		t.Error("沒有 REISSUE operation ⇒ 作廢成功之後沒有東西會開新票")
	}
	if h.reqs.gotVoidAdjID != 9001 {
		t.Errorf("correction 指向的 adjustment = %d, want 9001（fk_invcorr_void）", h.reqs.gotVoidAdjID)
	}
	// R04 全程不碰 operation 的 HELD，也不打 order-service
	if h.ops.holdCalls != 0 || h.orders.calls != 0 {
		t.Error("R04 不得 Hold、也不得打 order-service")
	}
}

// TestCorrectionExecute_R04VoidSequenceGateHeldRollsBack 是 P0-1 在 correction 軸的
// 同形實例（同 root cause，`.claude/rules/10-engineering-contracts.md` §17.7）。
//
// 🔴 這裡的 operation_key 是 `void:{adjustment_id}`，而 adj 剛剛才以新 ID 建出來
// （`created == true` 才走到這一格）⇒ **operation_key 不可能撞**，rows=0 剩下的
// 唯一可能就是 `uq_invop_open_adjustment_per_document`（I5 序列閘門）。
//
// 原本兩種都只記一行 log 就往下走，後果與折讓軸的孤兒折讓單同形：
// VOID adjustment 建了、operation 一列都沒有、correction 進 VOID_REISSUE_IN_PROGRESS
// ⇒ 買家看到「處理中」而永遠不會有事情發生，且完全不報錯。
func TestCorrectionExecute_R04VoidSequenceGateHeldRollsBack(t *testing.T) {
	h := newCorrHarness(t)
	h.ops.voidGateHeld = true

	if _, err := h.uc.Execute(context.Background(), execInput(issuedFacts())); err == nil {
		t.Fatal("序列閘門被佔住卻回 nil —— correction 會進 VOID_REISSUE_IN_PROGRESS，" +
			"而那筆作廢沒有任何執行者：買家看到「處理中」，永遠不會有事情發生")
	}
	// 沒建成就不得 promote，也不得往下建重開票（那會讓一張沒有作廢的發票旁邊
	// 多出一張等著開的新票）。
	if h.ops.promoteCalls != 0 {
		t.Errorf("沒建成卻 promote 了 %d 次", h.ops.promoteCalls)
	}
	if h.reqs.gotVoidAdjID != 0 {
		t.Errorf("correction 已經指向 adjustment %d —— 那一步應該在 rollback 之內",
			h.reqs.gotVoidAdjID)
	}
}

// 🔴 唯一鍵擋下既有的 VOID adjustment 時**絕不能再建 operation**：
// 重複建會送出第二次作廢，而作廢不可逆。
func TestCorrectionExecute_R04ExistingAdjustmentSkipsOperation(t *testing.T) {
	h := newCorrHarness(t)
	h.adjs.created = false
	h.adjs.existingID = 7777

	got, err := h.uc.Execute(context.Background(), execInput(issuedFacts()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Route != RouteVoidAndReissue {
		t.Fatalf("Route = %v, want RouteVoidAndReissue", got.Route)
	}
	if h.ops.createCalls != 0 {
		t.Fatalf("已有 VOID adjustment 時建了 %d 筆 operation ——那會送出第二次作廢",
			h.ops.createCalls)
	}
	if h.reqs.gotVoidAdjID != 7777 {
		t.Errorf("correction 應指向既有的 adjustment 7777，實得 %d", h.reqs.gotVoidAdjID)
	}
}

func submitInput(f CorrectionFacts) SubmitCorrectionInput {
	return SubmitCorrectionInput{
		DocumentID: 2002, BuyerID: 5005, ExpectedSnapshotVersion: 1,
		OperationID: 3003, OrderNo: "O20260831001", Facts: f,
		// 見 execInput 的紅線：這兩個欄位缺席就是 order-service 400（#3085）。
		// 這裡刻意用 B2C ＋ USE_BUYER_DEFAULT（另一組合法值），讓兩支 builder
		// 不會因為抄同一份而漏掉「值域檢查」那一半。
		OrderRevision: domain.ReviseSnapshotInput{
			OrderNo:                 "O20260831001",
			ExpectedSnapshotVersion: 1,
			InvoiceKind:             codemeta.InvoiceDocumentKindB2c.Value,
			CarrierSelectionMode:    codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value,
			IdempotencyKey:          "correction-2002",
		},
		Revised: domain.ReviseDocumentSnapshot{
			ExpectedVersion: 3,
			DocumentKind:    codemeta.InvoiceDocumentKindB2c.Value,
			// B2C 的 recipient_tax_id 恆為 '0000000000'（chk_invdoc_b2c 雙向守著），
			// **不是空字串**——空字串違反 chk_invdoc_taxid_shape，而那正是
			// #3089 B 組 A-3 的第三個變體（買家清掉統編 ⇒ 500）。
			RecipientTaxID: domain.B2CBuyerIdentifier,
			SalesAmount:    1050, TotalAmount: 1050,
		},
	}
}

// R01 與分支必須是**同一次呼叫**。
//
// 🔴 這是 #2909 的原始症狀：R01 建出的 REQUESTED 若沒有立刻被推走，它會佔住
// uq_invcorr_open_per_document 的名額，而**沒有任何排程會來撿它**
// ⇒ 那份 document 從此再也提不出任何修改請求，且不報錯。
func TestCorrectionSubmit_R01ImmediatelyRunsBranch(t *testing.T) {
	h := newCorrHarness(t)

	got, err := h.uc.Submit(context.Background(), submitInput(pendingFacts()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Route != RouteUpdateBeforeIssue {
		t.Fatalf("Route = %v, want R03（R01 之後必須立刻執行分支）", got.Route)
	}

	want := []string{"lock_document", "insert_correction", "hold", "grpc_revise",
		"delete_lines", "insert_lines", "revise_doc", "release", "mark"}
	for i := range want {
		if i >= len(*h.calls) || (*h.calls)[i] != want[i] {
			t.Fatalf("呼叫序列 = %v, want %v", *h.calls, want)
		}
	}
	// 鎖一定在建立之前——反了就是用過期的判定結果建 correction
	if (*h.calls)[0] != "lock_document" {
		t.Error("LockForCorrection 必須在 Insert 之前")
	}
}

func TestCorrectionSubmit_StaleSnapshotRejectedBeforeInsert(t *testing.T) {
	h := newCorrHarness(t)
	h.docs.docSnapshotVersion = 5 // 買家看到的是 1，實際已推進到 5

	_, err := h.uc.Submit(context.Background(), submitInput(pendingFacts()))
	if !errors.Is(err, ErrCorrectionSnapshotStale) {
		t.Fatalf("error = %v, want ErrCorrectionSnapshotStale", err)
	}
	if h.reqs.insertCalls != 0 {
		t.Error("版本過期就不得建立——那會寫進一個過期的樂觀鎖基準，之後在跨服務那層才炸")
	}
}

func TestCorrectionSubmit_SequenceGateBlocksSecondRequest(t *testing.T) {
	h := newCorrHarness(t)
	h.reqs.insertCreated = false // uq_invcorr_open_per_document 擋下

	_, err := h.uc.Submit(context.Background(), submitInput(pendingFacts()))
	if !errors.Is(err, ErrCorrectionAlreadyInProgress) {
		t.Fatalf("error = %v, want ErrCorrectionAlreadyInProgress（呼叫端回 9351，不是重試）", err)
	}
	if h.ops.holdCalls != 0 {
		t.Error("閘門擋下後不得執行任何分支")
	}
}

type corrTracks struct {
	list  []amego.Track
	err   error
	calls int
}

func (f *corrTracks) ListTracks(context.Context, amego.Credential, int, int) ([]amego.Track, error) {
	f.calls++
	return f.list, f.err
}

func (f *corrAdjs) ListByDocument(context.Context, int64) ([]domain.Adjustment, error) {
	return f.listed, nil
}

// 🔴 字軌探測失敗必須 fail-closed。
//
// 放行的後果是先作廢再發現開不出來，而作廢不可逆且明令不可用 DB restore 回復
// ⇒ 買家手上一張都沒有。婉拒的後果只是他稍後再送一次。
func TestGatherCorrectionFacts_TrackProbeFailsClosed(t *testing.T) {
	h := newCorrHarness(t)
	period := "11504"
	doc := domain.Document{
		ID: 2002, Status: codemeta.InvoiceDocumentStatusIssued.Value, TaxPeriodCode: &period,
	}
	tracks := &corrTracks{err: errors.New("amego timeout")}

	got, err := h.uc.GatherCorrectionFacts(context.Background(), tracks, CorrectionFactsInput{
		Document: doc, ProviderStatus: codemeta.InvoiceProviderAccountStatusActive.Value,
	})
	if err != nil {
		t.Fatalf("探測失敗不該讓整段失敗（那會讓買家看到 500 而不是理由）：%v", err)
	}
	if got.TrackSufficient {
		t.Fatal("字軌探測失敗必須當成不足 —— 放行會先作廢再發現開不出來，而作廢不可逆")
	}
}

// 未開立不打 AMEGO：那條路徑不作廢也不重開，字軌與它無關。
// 無條件打會讓每一筆「只是改個統編」都付一次外呼成本，且 AMEGO 掛掉時連 R03 都做不了。
func TestGatherCorrectionFacts_PendingDocumentSkipsTrackProbe(t *testing.T) {
	h := newCorrHarness(t)
	period := "11504"
	doc := domain.Document{
		ID: 2002, Status: codemeta.InvoiceDocumentStatusPending.Value, TaxPeriodCode: &period,
	}
	tracks := &corrTracks{err: errors.New("should not be called")}

	got, err := h.uc.GatherCorrectionFacts(context.Background(), tracks, CorrectionFactsInput{
		Document: doc, OpenOperationID: 3003,
		ProviderStatus: codemeta.InvoiceProviderAccountStatusActive.Value,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracks.calls != 0 {
		t.Errorf("未開立時打了 %d 次 AMEGO，應為 0", tracks.calls)
	}
	if !got.OperationOpen {
		t.Error("OperationOpen 應由 OpenOperationID 推出")
	}
}

// tax_period_code 為 nil（未開立可能沒有期別）時兩個時間判定 fail-closed。
func TestGatherCorrectionFacts_NilPeriodFailsClosed(t *testing.T) {
	h := newCorrHarness(t)
	doc := domain.Document{ID: 2002, Status: codemeta.InvoiceDocumentStatusIssued.Value}
	tracks := &corrTracks{}

	got, err := h.uc.GatherCorrectionFacts(context.Background(), tracks, CorrectionFactsInput{
		Document: doc, ProviderStatus: codemeta.InvoiceProviderAccountStatusActive.Value,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.AlreadyReported || got.WithinVoidDeadline {
		t.Error("沒有期別時必須 fail-closed（當成已申報、已逾期限），不得放行走作廢")
	}
}

// 已存在折讓 ⇒ 作廢會使折讓失去依據。
func TestGatherCorrectionFacts_ExistingAllowanceBlocksVoid(t *testing.T) {
	h := newCorrHarness(t)
	h.adjs.listed = []domain.Adjustment{
		{ID: 1, AdjustmentType: codemeta.InvoiceAdjustmentTypeAllowance.Value},
	}
	period := "11504"
	doc := domain.Document{
		ID: 2002, Status: codemeta.InvoiceDocumentStatusIssued.Value, TaxPeriodCode: &period,
	}

	got, err := h.uc.GatherCorrectionFacts(context.Background(), &corrTracks{}, CorrectionFactsInput{
		Document: doc, ProviderStatus: codemeta.InvoiceProviderAccountStatusActive.Value,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.NoExistingAllowance {
		t.Fatal("已有 ALLOWANCE 時 NoExistingAllowance 必須為 false")
	}
}

// ── CorrectionIntake：組裝層 ─────────────────────────────────────────────

type intakeDocs struct {
	corrDocs
	doc   domain.Document
	lines []domain.DocumentLine
}

func (f *intakeDocs) GetForBuyer(context.Context, string, int64) (domain.Document, error) {
	return f.doc, nil
}

// ListOrderDocumentsForBuyer 重現真實排序「最新一次開立排最前」，並刻意**多回一列
// 已被取代的原票**：intake 若取錯那一列，症狀就是買家再也無法更正重開後的發票
// （2026-09-02 dev 實測到的那一格）。無條件只回一列的替身驗不到這件事。
func (f *intakeDocs) ListOrderDocumentsForBuyer(context.Context, string, int64) ([]domain.Document, error) {
	superseded := f.doc
	superseded.ID = f.doc.ID - 1
	superseded.Status = codemeta.InvoiceDocumentStatusSuperseded.Value
	superseded.IssueAttemptSeq = f.doc.IssueAttemptSeq - 1
	supersededBy := f.doc.ID
	superseded.SupersededByID = &supersededBy
	return []domain.Document{f.doc, superseded}, nil
}

func (f *intakeDocs) ListLines(context.Context, int64) ([]domain.DocumentLine, error) {
	return f.lines, nil
}

type intakeOps struct {
	corrOps
	listed []domain.Operation
}

func (f *intakeOps) ListByDocument(context.Context, int64) ([]domain.Operation, error) {
	return f.listed, nil
}

type intakeAccounts struct {
	domain.ProviderAccountRepository
	acct domain.ProviderAccount
}

func (f *intakeAccounts) GetByIssuer(context.Context, int64, string) (domain.ProviderAccount, error) {
	return f.acct, nil
}

func (f *intakeAccounts) WithDecryptedCredential(_ context.Context, _ int64, fn func(string, string) error) error {
	return fn("12345678", "test-app-key")
}

// 🔴 「非終態 operation」用**正面列舉**（READY／RETRY_SCHEDULED），不是排除法。
//
// 新增一個終態值時，排除法會靜默把它算成「還在進行中」⇒ R03 會對一張已送出的發票
// 改快照；正面列舉則是漏掉它 ⇒ 走 R04（保守）。兩個方向的代價不對稱。
func TestCorrectionIntake_OpenOperationUsesPositiveList(t *testing.T) {
	h := newCorrHarness(t)
	period := "11504"
	docs := &intakeDocs{
		doc: domain.Document{
			ID: 2002, IssuerID: 7, Status: codemeta.InvoiceDocumentStatusPending.Value,
			TaxPeriodCode: &period, Version: 3,
		},
		lines: []domain.DocumentLine{{LineSeq: 1, LineType: "PRODUCT", Quantity: 1, UnitPrice: 100, NetAmount: 100}},
	}
	docs.corrDocs = corrDocs{reviseOK: true, docSnapshotVersion: 1, order: h.calls, createInserted: true}
	ops := &intakeOps{listed: []domain.Operation{
		{ID: 9, Status: codemeta.InvoiceOperationStatusSucceeded.Value}, // 終態，不算
		{ID: 3003, Status: codemeta.InvoiceOperationStatusReady.Value},  // 這個才算
	}}
	ops.corrOps = corrOps{holdOK: true, releaseOK: true, promoteOK: true, order: h.calls}

	intake := NewCorrectionIntake(CorrectionIntakeDeps{
		Documents:  docs,
		Operations: ops,
		Accounts:   &intakeAccounts{acct: domain.ProviderAccount{ID: 5, Status: codemeta.InvoiceProviderAccountStatusActive.Value}},
		Tracks:     &corrTracks{},
	}, h.uc)

	out, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
		BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
		NewInvoice:     CheckoutInvoiceSpec{BuyerTaxID: "12345678", BuyerName: "範例生鮮"},
		IdempotencyKey: "correction-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Route != RouteUpdateBeforeIssue {
		t.Fatalf("Route = %v, want R03（有 READY operation ⇒ 攔得下來）", out.Route)
	}
	if out.HandlingMode != handlingModeUpdateSnapshot {
		t.Errorf("HandlingMode = %q, want UPDATE_SNAPSHOT", out.HandlingMode)
	}
	if out.Status != correctionStatusUpdatedBeforeIssue {
		t.Errorf("Status = %q, want UPDATED_BEFORE_ISSUE", out.Status)
	}
	// handling_mode 與 status 的值域零交集——不得互相污染
	if out.HandlingMode == out.Status {
		t.Error("handling_mode 與 status 不得同值：前者是做法、後者是進度")
	}
	// 🔴 這一條才是「正面列舉 vs 排除法」的鑑別力所在：
	// 排除法會選中 ID 9（SUCCEEDED，已完成），而對它下 Hold 必然 rows=0
	// ⇒ 整筆退成 ErrCorrectionSendInFlight。只斷言 Route 的話兩者都是 R03，測不出來。
	if h.ops.gotHoldID != 3003 {
		t.Fatalf("Hold 對象 = %d, want 3003（READY 那一筆）——選中已完成的 operation 會讓 R03 恆失敗",
			h.ops.gotHoldID)
	}
}

// 明細**從既有列重建**，不是憑空造。少了它 DeleteLines 之後就沒有東西補回去。
func TestCorrectionIntake_RebuildsLinesFromExisting(t *testing.T) {
	h := newCorrHarness(t)
	period := "11504"
	docs := &intakeDocs{
		doc: domain.Document{
			ID: 2002, IssuerID: 7, Status: codemeta.InvoiceDocumentStatusPending.Value,
			TaxPeriodCode: &period, Version: 3,
		},
		lines: []domain.DocumentLine{
			{LineSeq: 1, LineType: "PRODUCT", Description: "有機高麗菜", Quantity: 2, UnitPrice: 60, NetAmount: 120},
			{LineSeq: 2, LineType: "SHIPPING", Description: "運費", Quantity: 1, UnitPrice: 60, NetAmount: 60},
		},
	}
	docs.corrDocs = corrDocs{reviseOK: true, docSnapshotVersion: 1, order: h.calls, createInserted: true}
	ops := &intakeOps{listed: []domain.Operation{{ID: 3003, Status: codemeta.InvoiceOperationStatusReady.Value}}}
	ops.corrOps = corrOps{holdOK: true, releaseOK: true, promoteOK: true, order: h.calls}

	intake := NewCorrectionIntake(CorrectionIntakeDeps{
		Documents: docs, Operations: ops,
		Accounts: &intakeAccounts{acct: domain.ProviderAccount{ID: 5, Status: codemeta.InvoiceProviderAccountStatusActive.Value}},
		Tracks:   &corrTracks{},
	}, h.uc)

	if _, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
		BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
		NewInvoice: CheckoutInvoiceSpec{BuyerTaxID: "12345678"}, IdempotencyKey: "c-2",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ⚠️ 數的是 harness 的 docs 不是 intakeDocs：intake 用 deps **組裝**，
	// 而執行走的是 Correction 自己的 repo（兩者刻意分開，見 CorrectionIntake 的註解）。
	if h.docs.insertCall != 1 {
		t.Fatalf("InsertLines 呼叫 %d 次, want 1", h.docs.insertCall)
	}
	if len(h.docs.gotLines) != 2 {
		t.Fatalf("重建了 %d 行, want 2（從既有明細來，不是憑空造）", len(h.docs.gotLines))
	}
}

// 🔴 分支執行失敗必須釋放序列閘門，否則一次暫時性失敗永久鎖死那份 document。
//
// R01 的交易已 commit（分支在交易外，因為 R03 中間有跨服務 gRPC），失敗時會留下
// 一筆 REQUESTED；而 uq_invcorr_open_per_document 的述詞含 REQUESTED ⇒ 買家重送
// 被擋、**沒有任何排程會來推它**。那正是 #2909 的原始症狀，只是從另一條路徑到達。
//
// 2026-08-31 dev 首次實打就製造出一筆（VOID operation 漏填 MaxAttempts 觸發 23514）。
func TestCorrectionSubmit_BranchFailureReleasesGate(t *testing.T) {
	h := newCorrHarness(t)
	h.orders.err = errors.New("order-service unavailable") // R03 中途失敗

	_, err := h.uc.Submit(context.Background(), submitInput(pendingFacts()))
	if err == nil {
		t.Fatal("分支失敗必須往上拋")
	}
	if h.reqs.rejectCalls != 1 {
		t.Fatalf("Reject 呼叫 %d 次, want 1 —— 沒婉拒就會永久佔住序列閘門", h.reqs.rejectCalls)
	}
	if h.reqs.gotReason != codemeta.InvoiceCorrectionRejectReasonProviderUnavailable.Value {
		t.Errorf("理由碼 = %q, want PROVIDER_UNAVAILABLE（給買家的下一步是「稍後再送」）",
			h.reqs.gotReason)
	}
}

// 婉拒路徑本身失敗時不得再婉拒一次（那是無窮遞迴的形狀），也不得蓋掉原始錯誤。
func TestCorrectionSubmit_RejectPathDoesNotDoubleReject(t *testing.T) {
	f := issuedFacts()
	f.AlreadyReported = true // 走 R02
	h := newCorrHarness(t)

	if _, err := h.uc.Submit(context.Background(), submitInput(f)); err != nil {
		t.Fatalf("R02 是正常路徑不該失敗：%v", err)
	}
	if h.reqs.rejectCalls != 1 {
		t.Fatalf("Reject 呼叫 %d 次, want 1（R02 本身那一次）", h.reqs.rejectCalls)
	}
}

// 🔴 每一條路徑都必須回傳 CorrectionID —— 買家要拿它去打端點 12 查進度。
//
// 回空字串**不會報錯、HTTP 也是 200**：2026-08-31 dev 實打時就是這樣，
// 路由與狀態全對，只有 correctionId 是空的。先前的測試只斷言
// Route／HandlingMode／Status，沒有任何東西看著這一欄。
func TestCorrectionExecute_AllRoutesReturnCorrectionID(t *testing.T) {
	cases := []struct {
		name  string
		facts CorrectionFacts
		mut   func(*corrHarness)
	}{
		{name: "R02 婉拒", facts: func() CorrectionFacts {
			f := issuedFacts()
			f.AlreadyReported = true
			return f
		}()},
		{name: "R03 開立前直接改", facts: pendingFacts()},
		{name: "R04 作廢重開", facts: issuedFacts()},
		{name: "R04 已有 adjustment", facts: issuedFacts(),
			mut: func(h *corrHarness) { h.adjs.created = false; h.adjs.existingID = 7777 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newCorrHarness(t)
			if tc.mut != nil {
				tc.mut(h)
			}
			in := execInput(tc.facts)
			got, err := h.uc.Execute(context.Background(), in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.CorrectionID != in.CorrectionID {
				t.Fatalf("CorrectionID = %d, want %d —— 回空的話買家查不到自己的申請",
					got.CorrectionID, in.CorrectionID)
			}
		})
	}
}

// 🔴 R04 建的 operation 必須在**同一事務內**推到 READY。
//
// InsertInvoiceOperation 落的是 CREATED，而 **CREATED 不在任何一支 worker 的候選集合裡**
// ⇒ 少了 Promote，那筆作廢永遠不會送出去：correction 停在 VOID_REISSUE_IN_PROGRESS、
// document 停在 ISSUED、買家看到「處理中」而什麼都不會發生，**且完全不報錯**。
//
// 2026-08-31 dev 實測就是這樣（等 45 秒 operation 仍是 CREATED／attempt=0）。
// 那是 #2909 原始症狀的第三次變形——三次的共同形狀都是「建了一列，而沒有任何東西會推它」。
func TestCorrectionExecute_R04PromotesOperationToReady(t *testing.T) {
	h := newCorrHarness(t)

	if _, err := h.uc.Execute(context.Background(), execInput(issuedFacts())); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ops.promoteCalls != 1 {
		t.Fatalf("Promote 呼叫 %d 次, want 1 —— 停在 CREATED 就沒有 worker 會撿它", h.ops.promoteCalls)
	}
	if h.ops.gotPromoteTo != codemeta.InvoiceOperationStatusReady.Value {
		t.Errorf("推到 %q, want READY", h.ops.gotPromoteTo)
	}
	if h.ops.gotPromoteID != h.ops.gotOpCreatedID {
		t.Errorf("推的是 %d 但建的是 %d —— 推錯對象等於沒推",
			h.ops.gotPromoteID, h.ops.gotOpCreatedID)
	}
}

// 已有 adjustment 時不建 operation，也就不該 promote。
func TestCorrectionExecute_R04ExistingAdjustmentSkipsPromote(t *testing.T) {
	h := newCorrHarness(t)
	h.adjs.created = false
	h.adjs.existingID = 7777

	if _, err := h.uc.Execute(context.Background(), execInput(issuedFacts())); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ops.promoteCalls != 0 {
		t.Fatalf("沒建 operation 卻 promote 了 %d 次", h.ops.promoteCalls)
	}
}

// GetByID 回原票。R04 ③ 靠它複製發票人／稅務／金額那一組欄位。
func (f *corrDocs) GetByID(_ context.Context, id int64) (domain.Document, error) {
	d := f.orig
	d.ID = id
	return d, nil
}

// Create 是重開票的建列。
//
// 🔴 fake 在這裡**拒絕錯的實體軸與缺失的原票連結**：兩者任一寫錯，DB 都不會擋
// （issue_attempt_seq 只有 >= 1 的 CHECK，original_document_id 可為 NULL），
// 而後果分別是撞 uq_invdoc_order_identity 與 R10 永遠不觸發——後者是靜默的。
func (f *corrDocs) Create(_ context.Context, in domain.NewDocument) (domain.Document, bool, error) {
	f.createCalls++
	f.createdDoc = in
	*f.order = append(*f.order, "create_reissue_document")
	if in.IssueAttemptSeq < 2 {
		return domain.Document{}, false, fmt.Errorf("fake: 重開票的 issue_attempt_seq = %d, 必須 >= 2", in.IssueAttemptSeq)
	}
	if in.OriginalDocumentID == nil {
		return domain.Document{}, false, errors.New("fake: 重開票沒有 original_document_id ⇒ R10 永遠不會觸發")
	}
	if in.Status != codemeta.InvoiceDocumentStatusPending.Value {
		return domain.Document{}, false, fmt.Errorf("fake: 重開票 status = %q, want PENDING", in.Status)
	}
	rn := in.ProviderRequestNo
	return domain.Document{ID: in.ID, ProviderRequestNo: &rn}, f.createInserted, nil
}

// R04 ③：重開票必須在同一交易內建好，且帶對兩個軸與原票連結。
//
// 這一支守的是「新票開得出來」的前提。三個斷言各自對應一個靜默失效：
//
//	・沒有重開票 ⇒ 作廢成功後沒有任何東西可開（§7.3.6 R08）
//	・實體軸沒 +1 ⇒ 撞 uq_invdoc_order_identity，建列當下 23505
//	・沒有 original_document_id ⇒ R10 永遠不觸發，correction 永遠停在
//	  VOID_REISSUE_IN_PROGRESS 而原票永遠停在 VOIDED（兩個都是靜默終點）
func TestCorrectionExecute_R04CreatesReissueDocumentInSameTx(t *testing.T) {
	h := newCorrHarness(t)
	h.docs.orig = domain.Document{
		IssuerID: 700001, SourceType: "ORDER", DocumentKind: "B2C",
		SnapshotVersion: 1, TotalAmount: 1050, TaxAmount: 50, SalesAmount: 1000,
	}

	if _, err := h.uc.Execute(context.Background(), execInput(issuedFacts())); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.docs.createCalls != 1 {
		t.Fatalf("重開票建了 %d 次, want 1", h.docs.createCalls)
	}
	got := h.docs.createdDoc
	if got.IssueAttemptSeq != 2 {
		t.Errorf("issue_attempt_seq = %d, want 2（實體軸 +1）", got.IssueAttemptSeq)
	}
	if got.SnapshotVersion != 1 {
		t.Errorf("snapshot_version = %d, want 1 —— 內容軸的唯一寫入者是 order-service（I-50）",
			got.SnapshotVersion)
	}
	if got.OriginalDocumentID == nil {
		t.Error("original_document_id 是 nil ⇒ R10 永遠不會觸發")
	} else if *got.OriginalDocumentID != 2002 {
		t.Errorf("original_document_id = %d, want 2002（原票）", *got.OriginalDocumentID)
	}
	// 🔴 金額**不是**從原票複製，是由 RevisedLines 經 domain.BuildHeader 重算。
	// 2026-09-01 integration test 抓到的：買家把個人改成公司統編時，發票從二聯式
	// 變三聯式，而 chk_invdoc_b2c 要求 B2C 的 tax_amount 恆 0、統編恆 '0000000000'
	// ⇒ 複製原票的值會在建列當下 23514。
	if got.DocumentKind != codemeta.InvoiceDocumentKindB2b.Value {
		t.Errorf("document_kind = %q, want B2B —— 修正後的買受人有統編", got.DocumentKind)
	}
	if got.TaxAmount == 0 {
		t.Error("B2B 的 tax_amount 是 0 —— 三聯式必須把稅額從含稅總額裡拆出來")
	}
	if got.SalesAmount+got.TaxAmount != got.TotalAmount {
		t.Errorf("表頭不自洽：sales=%d + tax=%d != total=%d",
			got.SalesAmount, got.TaxAmount, got.TotalAmount)
	}
}

// 買受人**沒有**統編時，重開票仍是 B2C 且 tax_amount 必須為 0。
//
// 與上一支互為對照：只驗 B2B 那一半會讓「一律標成 B2B」通過，而那在
// chk_invdoc_b2c 上是另一個方向的 23514。
func TestCorrectionExecute_R04ReissueKeepsB2CWhenNoTaxID(t *testing.T) {
	h := newCorrHarness(t)
	h.docs.orig = domain.Document{IssuerID: 700001, SourceType: "ORDER", DocumentKind: "B2C", SnapshotVersion: 1}
	in := execInput(issuedFacts())
	in.Revised.RecipientTaxID = domain.B2CBuyerIdentifier

	if _, err := h.uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := h.docs.createdDoc
	if got.DocumentKind != codemeta.InvoiceDocumentKindB2c.Value {
		t.Errorf("document_kind = %q, want B2C", got.DocumentKind)
	}
	if got.TaxAmount != 0 {
		t.Errorf("B2C 的 tax_amount = %d, want 0（chk_invdoc_b2c 雙向守著）", got.TaxAmount)
	}
}

// REISSUE operation 刻意落在 CREATED —— 與同一個交易裡的 VOID operation 相反。
//
// 🔴 這個「不推進」是設計不是漏洞：CREATED 不在任何 worker 的候選集合裡，正是
// 「等作廢確認」要的語意。把它一起推到 READY 會讓新票在原票還沒作廢時就開出去，
// 結果是同一筆交易兩張有效發票——稅務上的重複開立，且無法回收。
func TestCorrectionExecute_R04LeavesReissueOperationUnpromoted(t *testing.T) {
	h := newCorrHarness(t)
	h.docs.orig = domain.Document{IssuerID: 700001, SourceType: "ORDER", DocumentKind: "B2C", SnapshotVersion: 1}

	if _, err := h.uc.Execute(context.Background(), execInput(issuedFacts())); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 兩個 operation：VOID（要推）與 REISSUE（不推）⇒ promote 恰好一次。
	if h.ops.promoteCalls != 1 {
		t.Fatalf("Promote 呼叫 %d 次, want 1 —— 只有 VOID 該被推，REISSUE 要等作廢確認",
			h.ops.promoteCalls)
	}
	if h.ops.gotPromoteType != codemeta.InvoiceOperationTypeVoid.Value {
		t.Errorf("被推的是 %q, want VOID", h.ops.gotPromoteType)
	}
}

// #3010 的下游危害有第二個根因：intake 取到**已被取代的原票**。
//
// 🔴 dev 實測（2026-09-02）：#3010 的欄位修好之後，對重開後的新票送修改申請
// **仍然**回 PAST_VOID_DEADLINE。原因不是欄位，是 `GetForBuyer` 這支 `:one`
// 查詢在一張訂單有多份文件時「取任意一列而不報錯」（該 SQL 沒有 ORDER BY 也沒有
// LIMIT，風險在 repository 介面的註解裡就寫著），而它取到的是 SUPERSEDED 的原票
// ⇒ 那是真終態 ⇒ 婉拒。買家因此再也無法更正重開後的發票。
//
// 本測試釘住「取的是現行那一份」。替身刻意同時回兩列（見 ListOrderDocumentsForBuyer）。
func TestCorrectionIntake_PicksCurrentDocumentNotSupersededOriginal(t *testing.T) {
	current := domain.Document{
		ID:              7500795387054129152,
		Status:          codemeta.InvoiceDocumentStatusIssued.Value,
		SnapshotVersion: 1,
		IssueAttemptSeq: 2,
		RecipientTaxID:  domain.B2CBuyerIdentifier,
		DocumentKind:    codemeta.InvoiceDocumentKindB2c.Value,
	}
	docs := &intakeDocs{doc: current}

	got, err := docs.ListOrderDocumentsForBuyer(context.Background(), "O194628819", 2623900520)
	if err != nil {
		t.Fatalf("ListOrderDocumentsForBuyer: %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("替身只回了 %d 列 —— 本測試需要「一張訂單多份文件」才有鑑別力", len(got))
	}
	if got[0].ID != current.ID {
		t.Fatalf("第一列 = %d，want %d（排序必須讓現行那一份排最前）", got[0].ID, current.ID)
	}
	if got[0].Status == codemeta.InvoiceDocumentStatusSuperseded.Value {
		t.Fatal("取到了 SUPERSEDED 的原票 —— 買家將再也無法更正重開後的發票")
	}
	// 陰性：確認替身**真的**有一列是 SUPERSEDED，否則上面兩個斷言恆真。
	var sawSuperseded bool
	for _, d := range got {
		if d.Status == codemeta.InvoiceDocumentStatusSuperseded.Value {
			sawSuperseded = true
		}
	}
	if !sawSuperseded {
		t.Fatal("替身沒有回任何 SUPERSEDED 列 —— 本測試零鑑別力")
	}
}

// ---------------------------------------------------------------------------
// #3085 ＋ #3089 B 組 A-3：R03「開立前直接改」的兩層失敗
//
// 這條路徑從上線起 100% 失敗，而且是**兩個獨立的原因疊在一起**：
//   ① ReviseSnapshotInput 的四個必填欄位從未賦值 ⇒ order-service 回 400
//   ② 就地 UPDATE 不重算 document_kind 與金額 ⇒ 就算過了 ① 也是 chk_invdoc_b2c 的 23514
// 只修 ① 的話症狀只是從 400 換成 500。兩層各自要能單獨轉紅。
// ---------------------------------------------------------------------------

func intakeFixtureFor(t *testing.T, doc domain.Document, lines []domain.DocumentLine) (*corrHarness, *CorrectionIntake) {
	t.Helper()
	h := newCorrHarness(t)
	docs := &intakeDocs{doc: doc, lines: lines}
	docs.corrDocs = corrDocs{reviseOK: true, docSnapshotVersion: 1, order: h.calls, createInserted: true}
	ops := &intakeOps{listed: []domain.Operation{
		{ID: 3003, Status: codemeta.InvoiceOperationStatusReady.Value},
	}}
	ops.corrOps = corrOps{holdOK: true, releaseOK: true, promoteOK: true, order: h.calls}
	intake := NewCorrectionIntake(CorrectionIntakeDeps{
		Documents:  docs,
		Operations: ops,
		Accounts:   &intakeAccounts{acct: domain.ProviderAccount{ID: 5, Status: codemeta.InvoiceProviderAccountStatusActive.Value}},
		Tracks:     &corrTracks{},
	}, h.uc)
	return h, intake
}

func pendingB2CDoc() domain.Document {
	period := "11504"
	paid := time.Date(2026, 9, 1, 2, 0, 0, 0, time.UTC)
	return domain.Document{
		ID: 2002, IssuerID: 7, Status: codemeta.InvoiceDocumentStatusPending.Value,
		DocumentKind:  codemeta.InvoiceDocumentKindB2c.Value,
		TaxPeriodCode: &period, Version: 3, PaidAt: &paid,
	}
}

// 第一層：四個必填欄位。
func TestCorrectionIntake_FillsOrderRevisionRequiredFields(t *testing.T) {
	h, intake := intakeFixtureFor(t, pendingB2CDoc(),
		[]domain.DocumentLine{{LineSeq: 1, LineType: "PRODUCT", Quantity: 1, UnitPrice: 1050, NetAmount: 1050, TaxCategory: 1}})

	if _, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
		BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
		NewInvoice:     CheckoutInvoiceSpec{Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value, BuyerTaxID: "12345678", BuyerName: "範例生鮮"},
		IdempotencyKey: "correction-1",
	}); err != nil {
		t.Fatalf("R03 應成功：%v", err)
	}

	got := h.orders.got
	// 🔴 四個一起驗：只補 invoice_kind 的話 carrier_selection_mode 會落空字串
	// ⇒ 買家原本選的載具在更正後消失，而那不會報錯。
	if got.InvoiceKind != codemeta.InvoiceDocumentKindB2b.Value {
		t.Errorf("invoice_kind = %q，統編有值就該是 B2B", got.InvoiceKind)
	}
	if got.CarrierSelectionMode == "" {
		t.Error("carrier_selection_mode 空字串 —— order-service 對它同樣走 requireValidValue")
	}
	if got.BuyerTaxID != "12345678" || got.BuyerTitle != "範例生鮮" {
		t.Errorf("買受人沒帶上：taxID=%q title=%q", got.BuyerTaxID, got.BuyerTitle)
	}
}

// 第二層：就地 UPDATE 必須重算 kind 與金額（否則 chk_invdoc_b2c 的 23514）。
func TestCorrectionIntake_RecomputesKindAndAmountsOnRevise(t *testing.T) {
	h, intake := intakeFixtureFor(t, pendingB2CDoc(),
		[]domain.DocumentLine{{LineSeq: 1, LineType: "PRODUCT", Quantity: 1, UnitPrice: 1050, NetAmount: 1050, TaxCategory: 1}})

	if _, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
		BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
		NewInvoice:     CheckoutInvoiceSpec{Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value, BuyerTaxID: "12345678", BuyerName: "範例生鮮"},
		IdempotencyKey: "correction-1",
	}); err != nil {
		t.Fatalf("B2C 文件補打統編應成功（這正是端點 11 的主要使用情境）：%v", err)
	}

	rev := h.docs.gotRevise
	if rev.DocumentKind != codemeta.InvoiceDocumentKindB2b.Value {
		t.Fatalf("document_kind = %q，補了統編就該變 B2B", rev.DocumentKind)
	}
	if rev.RecipientTaxID != "12345678" {
		t.Errorf("recipient_tax_id = %q", rev.RecipientTaxID)
	}
	// B2B 要把稅從含稅總額裡拆出來：1050 含稅 ⇒ 未稅 1000 ＋ 稅 50。
	if rev.TaxAmount == 0 {
		t.Error("B2B 的 tax_amount 不該是 0 —— 那是 B2C 的形狀，代表金額沒重算")
	}
	if rev.TotalAmount != rev.SalesAmount+rev.FreeTaxSalesAmount+rev.ZeroTaxSalesAmount+rev.TaxAmount {
		t.Errorf("違反 chk_invdoc_total：%+v", rev)
	}
	// 期限跟著 kind 走：B2B 是 7 個自然日，B2C 是 2 個。
	if rev.StatutoryDeadline == nil {
		t.Fatal("statutory_deadline 沒重算 —— B2C 2 天與 B2B 7 天差 5 天，" +
			"不重算不會有任何東西轉紅，只會讓這批發票在錯的時點被告警")
	}
}

// 🔴 反面：清掉統編（B2B → B2C）也要正確落地。
// 只驗「補統編會變 B2B」分不清「重算了」與「一律寫死 B2B」。
func TestCorrectionIntake_ClearingTaxIDFallsBackToB2C(t *testing.T) {
	doc := pendingB2CDoc()
	doc.DocumentKind = codemeta.InvoiceDocumentKindB2b.Value
	h, intake := intakeFixtureFor(t, doc,
		[]domain.DocumentLine{{LineSeq: 1, LineType: "PRODUCT", Quantity: 1, UnitPrice: 1050, NetAmount: 1050, TaxCategory: 1}})

	if _, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
		BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
		NewInvoice:     CheckoutInvoiceSpec{Present: true, BuyerName: "王小明"},
		IdempotencyKey: "correction-2",
	}); err != nil {
		t.Fatalf("清掉統編應成功：%v", err)
	}
	rev := h.docs.gotRevise
	if rev.DocumentKind != codemeta.InvoiceDocumentKindB2c.Value {
		t.Fatalf("document_kind = %q，沒有統編就該是 B2C", rev.DocumentKind)
	}
	// B2C 的落庫值恆為 '0000000000'，不是空字串（chk_invdoc_taxid_shape）。
	if rev.RecipientTaxID != domain.B2CBuyerIdentifier {
		t.Errorf("recipient_tax_id = %q，B2C 應為 %q", rev.RecipientTaxID, domain.B2CBuyerIdentifier)
	}
	if rev.TaxAmount != 0 {
		t.Errorf("B2C 的 tax_amount 必須是 0（chk_invdoc_b2c），實得 %d", rev.TaxAmount)
	}
}

// 形狀驗證：非法統編要回 9305，不是讓它落到 DB 變成 500。
func TestCorrectionIntake_RejectsMalformedTaxID(t *testing.T) {
	for _, bad := range []string{"1234567", "ABCDEFGH", "123456789"} {
		t.Run(bad, func(t *testing.T) {
			_, intake := intakeFixtureFor(t, pendingB2CDoc(),
				[]domain.DocumentLine{{LineSeq: 1, LineType: "PRODUCT", Quantity: 1, UnitPrice: 1050, NetAmount: 1050, TaxCategory: 1}})
			_, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
				BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
				NewInvoice:     CheckoutInvoiceSpec{Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value, BuyerTaxID: bad, BuyerName: "範例生鮮"},
				IdempotencyKey: "correction-3",
			})
			if err == nil {
				t.Fatalf("taxID=%q 應被擋下（9305），而不是落到 DB 變成 500", bad)
			}
			if !isBizCode(err, bizcode.InvoiceCarrierInvalid) {
				t.Fatalf("taxID=%q 應回 9305，實得 %v", bad, err)
			}
		})
	}
}

// #3089 稽核 A-5：抬頭長度要涵蓋所有分支。
//
// validateIdentityShape 只在 TAX_ID 那一格驗 BuyerName（1–60），而「無載具」
// 那一格不驗——可是 recipient_name 是 varchar(60) NOT NULL ⇒ 22001 ⇒ 500。
func TestCorrectionIntake_RejectsOverlongBuyerName(t *testing.T) {
	_, intake := intakeFixtureFor(t, pendingB2CDoc(),
		[]domain.DocumentLine{{LineSeq: 1, LineType: "PRODUCT", Quantity: 1, UnitPrice: 1050, NetAmount: 1050, TaxCategory: 1}})
	_, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
		BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
		// 🔴 刻意**不帶 carrierType**（無載具分支）——那正是 validateIdentityShape
		// 不驗抬頭長度的那一格。
		NewInvoice:     CheckoutInvoiceSpec{Present: true, BuyerName: strings.Repeat("字", 61)},
		IdempotencyKey: "correction-long",
	})
	if err == nil {
		t.Fatal("61 字的抬頭應被擋下，而不是落到 recipient_name varchar(60) 變成 500")
	}
	if !isBizCode(err, bizcode.InvoiceCarrierInvalid) {
		t.Fatalf("應回 9305，實得 %v", err)
	}
}

// #3087（P0）：作廢重開進行中時，序列閘門被繞過。
//
// R04 一旦啟動就**立刻**建出替代票（PENDING）。它的內容軸／實體軸都比原票新
// ⇒ ListOrderDocumentsForBuyer 把它排到第一，而序列閘門要看的是**正在被作廢的原票**。
//
// dev 實測（VOI-04）：第二次更正回 HTTP 200 ＋ PAST_VOID_DEADLINE，
// 而被判定的那張票的作廢期限在**一週後** —— 買家會以為是自己太晚提出。
func TestCorrectionIntake_SequenceGateSeesTheOriginalNotTheReplacement(t *testing.T) {
	const originalID = 2001
	replacement := pendingB2CDoc()
	replacement.ID = 2002
	orig := int64(originalID)
	replacement.OriginalDocumentID = &orig

	h, _ := intakeFixtureFor(t, replacement, nil)
	docs := &intakeDocs{doc: replacement}
	docs.corrDocs = corrDocs{reviseOK: true, docSnapshotVersion: 1, order: h.calls, createInserted: true}
	corrs := &fakeCorrections{
		// 原票上有一筆進行中的更正（R04 作廢重開正在飛）。
		openByDoc: map[int64]domain.CorrectionRequest{
			originalID: {ID: 6001, Status: codemeta.InvoiceCorrectionStatusVoidReissueInProgress.Value},
		},
	}
	intake := NewCorrectionIntake(CorrectionIntakeDeps{
		Documents:   docs,
		Operations:  &intakeOps{},
		Accounts:    &intakeAccounts{acct: domain.ProviderAccount{ID: 5, Status: codemeta.InvoiceProviderAccountStatusActive.Value}},
		Corrections: corrs,
		Tracks:      &corrTracks{},
	}, h.uc)

	_, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
		BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
		NewInvoice:     CheckoutInvoiceSpec{Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value, BuyerTaxID: "12345678", BuyerName: "範例生鮮"},
		IdempotencyKey: "correction-second",
	})
	if err == nil {
		t.Fatal("作廢重開進行中，第二次更正必須被序列閘門擋下 —— " +
			"放行的話兩條更正會同時對同一份憑證動作")
	}
	if !isBizCode(err, bizcode.InvoiceOperationInProgress) {
		t.Fatalf("應回 9351（本張發票有處理中的作業），實得 %v —— "+
			"回 PAST_VOID_DEADLINE 會讓買家以為是自己太晚提出", err)
	}
}

// 🔴 陽性對照：**已經收尾**的作廢重開不得被誤擋。
// 只驗前一支的話，把守衛寫成「有 original_document_id 就擋」也會通過
// ⇒ 買家將**再也無法更正重開後的發票**（那正是 #3010 的下游危害）。
func TestCorrectionIntake_CompletedReissueStillAllowsNewCorrection(t *testing.T) {
	const originalID = 2001
	replacement := pendingB2CDoc()
	replacement.ID = 2002
	orig := int64(originalID)
	replacement.OriginalDocumentID = &orig

	h, _ := intakeFixtureFor(t, replacement,
		[]domain.DocumentLine{{LineSeq: 1, LineType: "PRODUCT", Quantity: 1, UnitPrice: 1050, NetAmount: 1050, TaxCategory: 1}})
	docs := &intakeDocs{doc: replacement,
		lines: []domain.DocumentLine{{LineSeq: 1, LineType: "PRODUCT", Quantity: 1, UnitPrice: 1050, NetAmount: 1050, TaxCategory: 1}}}
	docs.corrDocs = corrDocs{reviseOK: true, docSnapshotVersion: 1, order: h.calls, createInserted: true}
	ops := &intakeOps{listed: []domain.Operation{{ID: 3003, Status: codemeta.InvoiceOperationStatusReady.Value}}}
	ops.corrOps = corrOps{holdOK: true, releaseOK: true, promoteOK: true, order: h.calls}
	intake := NewCorrectionIntake(CorrectionIntakeDeps{
		Documents:  docs,
		Operations: ops,
		Accounts:   &intakeAccounts{acct: domain.ProviderAccount{ID: 5, Status: codemeta.InvoiceProviderAccountStatusActive.Value}},
		// 原票上**沒有**進行中的更正（上一次已經 COMPLETED）。
		Corrections: &fakeCorrections{},
		Tracks:      &corrTracks{},
	}, h.uc)

	if _, err := intake.Submit(context.Background(), SubmitCorrectionRequest{
		BuyerID: 5005, OrderNo: "O20260831001", ExpectedSnapshotVersion: 1,
		NewInvoice:     CheckoutInvoiceSpec{Present: true, CarrierType: codemeta.InvoiceCarrierTypeTaxId.Value, BuyerTaxID: "12345678", BuyerName: "範例生鮮"},
		IdempotencyKey: "correction-after-reissue",
	}); err != nil {
		t.Fatalf("重開完成後的發票必須還能再更正，實得 %v —— "+
			"把守衛寫成「有 original_document_id 就擋」會讓買家再也改不了", err)
	}
}
