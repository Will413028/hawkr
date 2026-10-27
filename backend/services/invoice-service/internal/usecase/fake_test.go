package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔的 fake 一律**嵌入介面**而不是逐方法實作空殼。
//
// 差別是行為：嵌入之後沒被覆寫的方法是 nil，呼叫時 panic 並指名方法。空殼則會回零值，
// 而「回零值」與「這條路徑本來就不該被走到」在測試裡看起來一模一樣——那正是本波要防的
// 那種靜默。

// directTx 直接執行 callback：單元測試不驗交易語意（那是 integration_test.go 的職責，
// 它有真的 PostgreSQL 可以 rollback）。
type directTx struct{}

func (directTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type fakeIssuerRepo struct {
	domain.IssuerRepository
	issuer domain.Issuer
	err    error
}

func (f *fakeIssuerRepo) GetByRef(context.Context, string, int64) (domain.Issuer, error) {
	return f.issuer, f.err
}

type fakeProfileRepo struct {
	domain.SellerTaxProfileRepository
	profile domain.SellerTaxProfile
	err     error
}

func (f *fakeProfileRepo) GetByIssuer(context.Context, int64) (domain.SellerTaxProfile, error) {
	return f.profile, f.err
}

type fakeAccountRepo struct {
	domain.ProviderAccountRepository
	account domain.ProviderAccount
	err     error
	taxID   string
	appKey  string
}

func (f *fakeAccountRepo) GetByIssuer(context.Context, int64, string) (domain.ProviderAccount, error) {
	return f.account, f.err
}

func (f *fakeAccountRepo) WithDecryptedCredential(_ context.Context, _ int64, fn func(string, string) error) error {
	return fn(f.taxID, f.appKey)
}

type fakeDocRepo struct {
	domain.DocumentRepository

	created  []domain.NewDocument
	lines    []domain.NewDocumentLine
	receipts []string

	// notInserted 讓測試模擬冪等命中（uq_invdoc_order_identity 已有那一列）。
	notInserted bool

	// byID 覆寫 GetByID 的回傳（見該方法的註解）。未登記的 id 退回 stored。
	byID map[int64]domain.Document

	// superseded 記 MarkSuperseded 的 (原票 id, 新票 id)；supersedeFails 讓它回 rows=0。
	superseded     [][2]int64
	supersedeFails bool

	awaitingManual     []string
	issuing            int
	issued             []domain.MarkDocumentIssued
	receiptIssued      []domain.MarkReceiptIssued
	issueFailedReasons []string
	issueFailed        int
	pendingRetry       int

	// §5.2.1 逾期分類（#3261 延伸）的替身面。
	deadlineRows      []domain.Document
	deadlineCalls     int
	deadlineThreshold time.Time
	deadlineErr       error

	stored  domain.Document
	secrets domain.RecipientSecrets
	docLine []domain.DocumentLine

	// ADM 寫入面
	resumed            [][2]int64
	resumeFails        bool
	resolvedExternally []domain.MarkDocumentResolvedExternally
	resolveDocFails    bool
	byRequestNo        map[string]domain.Document

	// ownerStoreID 是 GetForSeller 的歸屬述詞右運算元（SQL 裡是 invoice_issuers 的 join）。
	ownerStoreID int64
}

func (f *fakeDocRepo) Create(_ context.Context, in domain.NewDocument) (domain.Document, bool, error) {
	f.created = append(f.created, in)
	if f.notInserted {
		return domain.Document{}, false, nil
	}
	d := domain.Document{
		ID: in.ID, IssuerID: in.IssuerID, SourceRef: strPtrOrNil(in.SourceRef),
		DocumentKind: in.DocumentKind, FulfillmentMode: in.FulfillmentMode, Status: in.Status,
		NotApplicableReason: strPtrOrNil(in.NotApplicableReason),
		TotalAmount:         in.TotalAmount, RecipientTaxID: in.RecipientTaxID,
		CarrierType: strPtrOrNil(in.CarrierType), InvoiceDateTaipei: in.InvoiceDateTaipei,
		TaxPeriodCode: strPtrOrNil(in.TaxPeriodCode), ProviderRequestNo: strPtrOrNil(in.ProviderRequestNo),
	}
	f.stored = d
	return d, true, nil
}

func (f *fakeDocRepo) InsertLines(_ context.Context, _ int64, lines []domain.NewDocumentLine, _ time.Time) error {
	f.lines = append(f.lines, lines...)
	return nil
}

func (f *fakeDocRepo) NextReceiptNo(context.Context) (string, error) {
	no := fmt.Sprintf("R%012d", len(f.receipts)+1)
	f.receipts = append(f.receipts, no)
	return no, nil
}

// MarkAwaitingManual 鏡射 MarkInvoiceDocumentAwaitingManual 的 allowed-from
// （`status IN ('PENDING','ISSUE_FAILED')`）並真的落 status ＋ manual_reason_code
// （2026-09-08 補）。
//
// 🔴 原本無條件回 true 且不動 stored：於是「document 有沒有真的轉到 AWAITING_MANUAL」
// 在 unit 層驗不到，測試只能斷言「這支被呼叫過」——而 ISSUING 直接跳 AWAITING_MANUAL
// 在真實 SQL 上是 rows=0（要先經 PENDING），那正是呼叫端最容易漏掉的一步。
func (f *fakeDocRepo) MarkAwaitingManual(_ context.Context, _ int64, code string, _ time.Time) (bool, error) {
	f.awaitingManual = append(f.awaitingManual, code)
	if f.stored.Status != "PENDING" && f.stored.Status != "ISSUE_FAILED" {
		return false, nil
	}
	f.stored.Status = "AWAITING_MANUAL"
	f.stored.ManualReasonCode = &code
	return true, nil
}

// MarkSuperseded 是 R10 的第一步（原票 VOIDED → SUPERSEDED）。
//
// 🔴 本 fake 原本**沒有這個方法**，於是 R10 的成功路徑一走到就 panic（嵌入的
// domain.DocumentRepository 是 nil 介面）——換句話說那條路在 unit 層從未被覆蓋，
// 既有的 R10 測試走的是**失敗**分支（#3055 補測試時才發現）。
//
// 回傳 false 的形態刻意保留：呼叫端把 rows=0 當成「我對狀態的理解與 DB 不符」
// 並整筆 rollback，那條分支也需要測得到。
// MarkSuperseded 鏡射 MarkInvoiceDocumentSuperseded 的 allowed-from
// （`status IN ('VOIDED','ISSUED')`，2026-09-08 補）。
//
// 🔴 少了這一格，「原票該不該離開 VOIDED」這條不變式在測試裡完全失明：無條件回 true 的
// 替身讓「呼叫了 MarkSuperseded」與「原票真的轉了 SUPERSEDED」變成同一件事，而 dev 上
// 實際發生的是**根本沒呼叫**（resolveExternally 漏了整段鏈路收尾），兩筆原票停在 VOIDED
// 長達一週並持續點亮 I8。同 MarkResolvedExternally 那一格的紅字。
//
// 目標是**原票**不是 stored（stored 是替代票）⇒ 先查 byID，這也是它與其他 fake 的差別。
func (f *fakeDocRepo) MarkSuperseded(_ context.Context, id, supersededBy int64, _ time.Time) (bool, error) {
	f.superseded = append(f.superseded, [2]int64{id, supersededBy})
	if f.supersedeFails {
		return false, nil
	}
	target, ok := f.byID[id]
	if !ok {
		target = f.stored
	}
	if target.Status != "VOIDED" && target.Status != "ISSUED" {
		return false, nil
	}
	target.Status = "SUPERSEDED"
	target.SupersededByID = &supersededBy
	if _, exists := f.byID[id]; exists {
		f.byID[id] = target
	}
	if f.stored.ID == id {
		f.stored = target
	}
	return true, nil
}

// GetByID 先查 byID 再退回 stored。
//
// 🔴 byID 是給「同一輪要讀到**不只一份** document」的案子用的（R10 的原票 vs 新票）：
// 無條件回 stored 的 fake 會讓「原票號取自原 document」與「取自新 document」兩種寫法
// **都通過**，而那正是 P5 最容易寫錯的一欄（症狀是通知寫成「原發票 X 已作廢並重新
// 開立，新發票號碼 X」，同一個號碼且不報錯）。
func (f *fakeDocRepo) GetByID(_ context.Context, id int64) (domain.Document, error) {
	if d, ok := f.byID[id]; ok {
		return d, nil
	}
	return f.stored, nil
}

func (f *fakeDocRepo) ListLines(context.Context, int64) ([]domain.DocumentLine, error) {
	return f.docLine, nil
}

// FetchApproachingStatutoryDeadlines 回預先擺好的列，並記下 threshold。
//
// **記 threshold 不只記次數**：呼叫端傳錯時間（例如傳 now+6h 而不是 now）會讓
// 還沒逾期的文件被提前標成 DEADLINE_MISSED，而那只有 threshold 抓得到。
func (f *fakeDocRepo) FetchApproachingStatutoryDeadlines(_ context.Context, threshold time.Time, _ int32) ([]domain.Document, error) {
	f.deadlineCalls++
	f.deadlineThreshold = threshold
	return f.deadlineRows, f.deadlineErr
}

func (f *fakeDocRepo) WithDecryptedRecipient(_ context.Context, _ int64, fn func(domain.RecipientSecrets) error) error {
	return fn(f.secrets)
}

func (f *fakeDocRepo) MarkIssuing(context.Context, int64, time.Time) (bool, error) {
	f.issuing++
	return true, nil
}

func (f *fakeDocRepo) MarkIssued(_ context.Context, in domain.MarkDocumentIssued) (bool, error) {
	f.issued = append(f.issued, in)
	return true, nil
}

func (f *fakeDocRepo) MarkReceiptIssued(_ context.Context, in domain.MarkReceiptIssued) (bool, error) {
	f.receiptIssued = append(f.receiptIssued, in)
	return true, nil
}

// MarkIssueFailed 記下**理由碼**而不只是次數：它是賣家在賣家中心看得到的唯一
// 「為什麼」，只數次數的替身對「理由被丟掉」這個缺陷零鑑別力（2026-09-05）。
func (f *fakeDocRepo) MarkIssueFailed(_ context.Context, _ int64, reasonCode string, _ time.Time) (bool, error) {
	f.issueFailed++
	f.issueFailedReasons = append(f.issueFailedReasons, reasonCode)
	return true, nil
}

// MarkPendingRetry 鏡射 MarkInvoiceDocumentPendingRetry 的 allowed-from
// （`status = 'ISSUING'` 單值）並真的落 PENDING。理由同 MarkAwaitingManual。
func (f *fakeDocRepo) MarkPendingRetry(context.Context, int64, time.Time) (bool, error) {
	f.pendingRetry++
	if f.stored.Status != "ISSUING" {
		return false, nil
	}
	f.stored.Status = "PENDING"
	return true, nil
}

// GetForSeller／GetForBuyerByID 重現 SQL 的**歸屬述詞**：不是自己的就是查無（9301）。
// 無條件回 stored 會讓兩支端點的 Tier A 陰性全綠而什麼都沒驗到。
func (f *fakeDocRepo) GetForSeller(_ context.Context, id, storeID int64) (domain.SellerDocumentItem, error) {
	if f.stored.ID != id || f.ownerStoreID != storeID {
		return domain.SellerDocumentItem{}, notFoundErr(bizcode.InvoiceDocumentNotFound)
	}
	return domain.SellerDocumentItem{Document: f.stored}, nil
}

func (f *fakeDocRepo) GetForBuyerByID(_ context.Context, id, buyerID int64) (domain.Document, error) {
	if f.stored.ID != id || f.stored.RecipientRefID == nil || *f.stored.RecipientRefID != buyerID {
		return domain.Document{}, notFoundErr(bizcode.InvoiceDocumentNotFound)
	}
	return f.stored, nil
}

// ClaimNotificationResend 重現 10 分鐘窗口那一句的 WHERE。
//
// 🔴 無條件回 claimed=true 會讓 9352 那一支測試全綠而什麼都沒驗到——而 9352 正是
// 「ADM 幫忙點一次、賣家再點一次，買家十分鐘內收到兩封」唯一的防線。
func (f *fakeDocRepo) ClaimNotificationResend(
	_ context.Context, id int64, windowStart, now time.Time,
) (int32, bool, error) {
	if f.stored.ID != id {
		return 0, false, nil
	}
	if f.stored.NotificationResentAt != nil && f.stored.NotificationResentAt.After(windowStart) {
		return 0, false, nil
	}
	at := now
	f.stored.NotificationResentAt = &at
	f.stored.NotificationResendCount++
	return f.stored.NotificationResendCount, true, nil
}

type fakeOpRepo struct {
	domain.OperationRepository

	created  []domain.NewOperation
	promoted []string

	stored domain.Operation

	claimed  int
	done     int
	failed   int
	unknown  []domain.MarkOperationUnknown
	attempts []int32
	exhaust  int
	seller   int

	// ADM 寫入面。casFails 讓測試模擬 rows=0（守衛不成立）。
	casFails bool
	// M4 P14／P16 的集合式副作用與端點 62 的 Tier A 讀取。
	failOpen           []domain.FailOpenOperationsForIssuer
	openCount          int64
	sellerViews        [][2]int64
	sellerViewErr      error
	retryAfterFix      []adminRetryCall
	resetAttempts      []adminRetryCall
	cancels            []domain.RequestOperationCancel
	resolvedExternally []adminRetryCall
	allowManual        []adminRetryCall
	confirmCancel      []confirmCancelCall
	// gotProvider* 是 #3082 的斷言材料。
	gotProviderCode    string
	gotProviderMessage string

	// byDocument 餵 ListByDocument（端點 73 的「零 operation」判準）。
	byDocument map[int64][]domain.Operation
}

func (f *fakeOpRepo) Create(_ context.Context, in domain.NewOperation) (domain.Operation, bool, error) {
	f.created = append(f.created, in)
	op := domain.Operation{ID: in.ID, DocumentID: in.DocumentID, OperationKey: in.OperationKey,
		Status: "CREATED", MaxAttempts: in.MaxAttempts}
	f.stored = op
	return op, true, nil
}

func (f *fakeOpRepo) Promote(_ context.Context, _ int64, next string, _, _ time.Time) (bool, error) {
	f.promoted = append(f.promoted, next)
	f.stored.Status = next
	return true, nil
}

func (f *fakeOpRepo) GetByID(context.Context, int64) (domain.Operation, error) { return f.stored, nil }

// ListByDocument 回這份文件底下已存在的 operation。
//
// 端點 73 用它判「零 operation」——而那是 CanResumeDocumentFromManual 的兩個條件之一。
// 預設空切片（fixture 的常態是「還沒有任何 operation」）；要測「已經有 job」的分支就
// 塞 byDocument。
func (f *fakeOpRepo) ListByDocument(_ context.Context, documentID int64) ([]domain.Operation, error) {
	return f.byDocument[documentID], nil
}

func (f *fakeOpRepo) Claim(context.Context, int64, time.Time, time.Time) (bool, error) {
	f.claimed++
	f.stored.Status = "SENDING"
	return true, nil
}

func (f *fakeOpRepo) MarkDone(context.Context, int64, time.Time) (bool, error) {
	f.done++
	return true, nil
}

func (f *fakeOpRepo) MarkFailed(context.Context, int64, string, time.Time) (bool, error) {
	f.failed++
	return true, nil
}

// MarkFailedWithProviderResult **記下 provider 兩欄**（#3082）。
//
// 🔴 只計數的替身對「provider_code 有沒有真的被寫入」完全沒有反應 ——
// 而 2026-09-04 的 dev 實機驗證正是這樣抓到開票軸的遺漏：
// reason_code 對了、兩個 provider 欄仍是空的，unit test 全綠。
func (f *fakeOpRepo) MarkFailedWithProviderResult(_ context.Context, in domain.MarkOperationProviderResult) (bool, error) {
	f.failed++
	f.gotProviderCode = in.ProviderCode
	f.gotProviderMessage = in.ProviderMessage
	return true, nil
}

func (f *fakeOpRepo) MarkWaitingConfirmation(_ context.Context, in domain.MarkOperationUnknown) (bool, error) {
	f.unknown = append(f.unknown, in)
	return true, nil
}

func (f *fakeOpRepo) UpdateAttempt(_ context.Context, _ int64, n int32, _ time.Time, _ *string, _ time.Time) error {
	f.attempts = append(f.attempts, n)
	return nil
}

func (f *fakeOpRepo) MarkRetryExhausted(context.Context, int64, string, time.Time) (bool, error) {
	f.exhaust++
	return true, nil
}

func (f *fakeOpRepo) MarkSellerActionRequired(context.Context, domain.MarkOperationProviderResult) (bool, error) {
	f.seller++
	return true, nil
}

type fakeOutbox struct {
	domain.OutboxRepository
	inserted []domain.NewOutboxEvent
	// err 讓 Insert 失敗，用來驗「事件寫不進去時整個交易要 rollback」。
	err error
}

func (f *fakeOutbox) Insert(_ context.Context, in domain.NewOutboxEvent) (domain.OutboxEvent, error) {
	if f.err != nil {
		return domain.OutboxEvent{}, f.err
	}
	f.inserted = append(f.inserted, in)
	return domain.OutboxEvent{}, nil
}

type fakeAudit struct {
	domain.AuditRepository
	entries []domain.NewStatusHistoryEntry
	admin   []domain.NewAdminAuditLogEntry
}

// AppendStatusHistory **鏡射 invoice_status_history 的 varchar 長度約束**。
//
// 🔴 原本無條件回 nil，於是「把人看的錯誤訊息塞進 correlation_id」這個真實缺陷在
// 測試裡完全看不見。dev 實測（第 3 輪 INV58R3-ALW06）：AMEGO 回「查無此發票」，
// 折讓的失敗處置以 `SQLSTATE 22001 value too long for type character varying(64)`
// 整個 rollback ⇒ **失敗處置本身失敗**，而真正的 provider 錯誤被那句 22001 取代。
//
// 長度取自 dev schema 實查：correlation_id varchar(64)、reason_code varchar(48)、
// reason_detail 是 text（無上限）。替身不鏡射約束 ⇒ 驗的是替身不是資料庫。
func (f *fakeAudit) AppendStatusHistory(_ context.Context, in domain.NewStatusHistoryEntry) error {
	if in.CorrelationID != nil && len(*in.CorrelationID) > 64 {
		return fmt.Errorf("fakeAudit: correlation_id 長度 %d 超過 varchar(64) —— "+
			"真實 DB 會回 SQLSTATE 22001 並讓整個交易 rollback；"+
			"人看的說明要走 reason_detail（text），不要借位 correlation_id",
			len(*in.CorrelationID))
	}
	if in.ReasonCode != nil && len(*in.ReasonCode) > 48 {
		return fmt.Errorf("fakeAudit: reason_code 長度 %d 超過 varchar(48)", len(*in.ReasonCode))
	}
	f.entries = append(f.entries, in)
	return nil
}

// fakeProvider 記下每一次外呼。requests 的長度就是「我方到底送出去幾次」——
// 「沒有任何請求送出去」這種宣稱只有數它才算數。
type fakeProvider struct {
	// allowanceCalls 數的是「真的打出去幾次」——折讓單號送出前先落庫，
	// 重送會讓 AMEGO 端出現第二張同號憑證，所以次數本身就是不變式。
	allowanceCalls   int
	allowanceFn      func(amego.AllowanceRequest) (*amego.AllowanceResult, error)
	allowanceQueryFn func(string) (*amego.AllowanceDetail, error)
	requests         []amego.IssueRequest
	result           *amego.IssueResult
	err              error
	verify           fakeProviderVerify
	// voidFn 讓作廢軸的測試複用同一個 fake；不設就是拒絕（見 VoidInvoice）。
	voidFn          func(amego.VoidRequest) (*amego.VoidAccepted, error)
	queryByNumberFn func(string) (*amego.InvoiceDetail, error)

	// barcodeErr／barcodeCalls 是送出前載具驗證的替身面（§P12）。
	//
	// 🔴 預設 nil ＝ 驗證通過 ＝ 照常送 f0401：讓既有的每一支開票測試維持原行為，
	// 否則「新增一道前置檢查」會把所有測試的意義悄悄換掉。要驗擋下來的路徑就把
	// barcodeErr 設成帶 Code 的 *amego.APIError。
	barcodeErr   error
	barcodeCalls []string
}

// VerifyMobileBarcode 記下被驗的載具值。**記值不只記次數**：只記次數的替身對
// 「驗錯了東西」（例如把 CarrierID2 或空字串送去驗）零鑑別力。
func (f *fakeProvider) VerifyMobileBarcode(_ context.Context, _ amego.Credential, barcode string) error {
	f.barcodeCalls = append(f.barcodeCalls, barcode)
	return f.barcodeErr
}

func (f *fakeProvider) IssueInvoice(_ context.Context, _ amego.Credential, req amego.IssueRequest) (*amego.IssueResult, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

// 兩支驗證類唯讀端點（§7.3.4 P03 的 ① 與 ③）。
//
// banErr／tracks 分成兩組旗標而不是共用 err：P03 的三項各自對應不同的落點
// （P04／P05／P06 vs 「沒有字軌」），共用一個錯誤就分不出測試在驗哪一格。
type fakeProviderVerify struct {
	banCalls  [][]string
	banErr    error
	trackErr  error
	tracks    []amego.Track
	trackArgs [][2]int
}

func (f *fakeProvider) QueryCompanyNames(_ context.Context, _ amego.Credential, bans []string) (map[string]string, error) {
	f.verify.banCalls = append(f.verify.banCalls, bans)
	if f.verify.banErr != nil {
		return nil, f.verify.banErr
	}
	out := map[string]string{}
	for _, b := range bans {
		out[b] = "測試股份有限公司"
	}
	return out, nil
}

func (f *fakeProvider) ListTracks(_ context.Context, _ amego.Credential, year, period int) ([]amego.Track, error) {
	f.verify.trackArgs = append(f.verify.trackArgs, [2]int{year, period})
	if f.verify.trackErr != nil {
		return nil, f.verify.trackErr
	}
	return f.verify.tracks, nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func notFoundErr(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}

// amegoResult 是 f0401 成功回應的樣本。
func amegoResult() amego.IssueResult {
	return amego.IssueResult{
		InvoiceNumber: "AB12345678",
		InvoiceTime:   1756512000,
		RandomNumber:  "4321",
	}
}

var _ = errors.New
var _ = notFoundErr

// ---------------------------------------------------------------------------
// ADM 寫入面（端點 52／53／56／57／60）的 fake 補充
//
// 每一支 CAS 都有一個 ok 旗標可以關掉：**rows=0 是本波最重要的陰性情境**，
// 而它在真的 DB 上是靜默的（不拋例外、不告警）——測不到它就等於沒有測守衛。
// ---------------------------------------------------------------------------

func (f *fakeIssuerRepo) GetByID(context.Context, int64) (domain.Issuer, error) {
	return f.issuer, f.err
}

// fakeDocRepo 的 ADM 補充。
// ResumeFromManual 鏡射 ResumeInvoiceDocumentFromManual 的兩道述詞
// （`status = 'AWAITING_MANUAL' AND version = @expected_version`，2026-09-05 補）。
//
// 🔴 在此之前它**無條件回 true**，於是「usecase 傳錯 expectedVersion」或
// 「document 根本不在 AWAITING_MANUAL」兩種真實缺陷在測試裡都是綠的——
// fake 的鑑別力來自它拒絕什麼（同 MarkResolvedExternally 那一格的紅字）。
func (f *fakeDocRepo) ResumeFromManual(_ context.Context, id, expectedVersion int64, _ time.Time) (bool, error) {
	f.resumed = append(f.resumed, [2]int64{id, expectedVersion})
	if f.resumeFails {
		return false, nil
	}
	if f.stored.Status != "AWAITING_MANUAL" || f.stored.Version != expectedVersion {
		return false, nil
	}
	f.stored.Status = "PENDING"
	f.stored.Version++
	return true, nil
}

// MarkResolvedExternally 鏡射 MarkInvoiceDocumentResolvedExternally 的 allowed-from
// （`status IN ('ISSUE_FAILED','AWAITING_MANUAL')`，2026-09-04 補）。
//
// 🔴 少了這一格，端點 56 的測試對 **#3112** 完全失明：VOID 類型的 job 走完 A4 之後
// document 停在 `VOIDED`，真實 SQL 會 rows=0 而呼叫端回 500，而無條件回 true 的替身
// 讓那條分支永遠是綠的。那個缺陷是 dev 端到端才發現的——**單元測試本來就該抓到**。
func (f *fakeDocRepo) MarkResolvedExternally(_ context.Context, in domain.MarkDocumentResolvedExternally) (bool, error) {
	f.resolvedExternally = append(f.resolvedExternally, in)
	if f.resolveDocFails {
		return false, nil
	}
	if f.stored.Status != "ISSUE_FAILED" && f.stored.Status != "AWAITING_MANUAL" {
		return false, nil
	}
	f.stored.Status = "RESOLVED_EXTERNALLY"
	return true, nil
}

func (f *fakeDocRepo) GetByProviderRequestNo(_ context.Context, no string) (domain.Document, error) {
	if f.byRequestNo == nil {
		return domain.Document{}, notFoundErr(bizcode.InvoiceDocumentNotFound)
	}
	d, ok := f.byRequestNo[no]
	if !ok {
		return domain.Document{}, notFoundErr(bizcode.InvoiceDocumentNotFound)
	}
	return d, nil
}

// fakeOpRepo 的 ADM 補充。
func (f *fakeOpRepo) AdminRetryAfterFix(_ context.Context, id, expectedVersion int64, hash string, _ time.Time) (bool, error) {
	f.retryAfterFix = append(f.retryAfterFix, adminRetryCall{ID: id, Version: expectedVersion, Hash: hash})
	if f.casFails {
		return false, nil
	}
	f.stored.Status = "READY"
	f.stored.Version++
	return true, nil
}

func (f *fakeOpRepo) AdminResetAttempts(_ context.Context, id, expectedVersion int64, _ time.Time) (bool, error) {
	f.resetAttempts = append(f.resetAttempts, adminRetryCall{ID: id, Version: expectedVersion})
	if f.casFails {
		return false, nil
	}
	f.stored.Status = "READY"
	f.stored.Version++
	return true, nil
}

func (f *fakeOpRepo) RequestCancel(_ context.Context, in domain.RequestOperationCancel) (bool, error) {
	f.cancels = append(f.cancels, in)
	if f.casFails {
		return false, nil
	}
	f.stored.Status = "CANCEL_REQUESTED"
	f.stored.Version++
	return true, nil
}

// MarkResolvedExternally 鏡射 MarkInvoiceOperationResolvedExternally 的兩個述詞
// （`status = 'MANUAL_PROCESS_ALLOWED'` ＋ version CAS，2026-09-04 補）。
//
// version 那一格不是裝飾：SQL 的註解逐字說「『必填』只有寫進述詞才是保證，
// 少了它應用層那句 if 是純 TOCTOU」——替身不驗它，那句保證在測試裡就不存在。
func (f *fakeOpRepo) MarkResolvedExternally(_ context.Context, id, expectedVersion int64, _ time.Time) (bool, error) {
	f.resolvedExternally = append(f.resolvedExternally, adminRetryCall{ID: id, Version: expectedVersion})
	if f.casFails {
		return false, nil
	}
	if f.stored.Status != "MANUAL_PROCESS_ALLOWED" || f.stored.Version != expectedVersion {
		return false, nil
	}
	f.stored.Status = "RESOLVED_EXTERNALLY"
	f.stored.Version++
	return true, nil
}

// AllowManualProcess 重現 J29 的兩個 WHERE 述詞：來源狀態必須是 CANCELLED_BY_ADMIN，
// 且 actor 不得是當初提出取消的那一位。無條件回 true 會讓「覆核人 ≠ 取消發起人」
// 那支測試全綠而什麼都沒驗到。
func (f *fakeOpRepo) AllowManualProcess(_ context.Context, id, expectedVersion, actorAccountID int64, _ time.Time) (bool, error) {
	f.allowManual = append(f.allowManual, adminRetryCall{ID: id, Version: expectedVersion})
	if f.casFails {
		return false, nil
	}
	if f.stored.Status != "CANCELLED_BY_ADMIN" {
		return false, nil
	}
	if f.stored.CancelRequestedBy == nil || *f.stored.CancelRequestedBy == actorAccountID {
		return false, nil
	}
	if f.stored.Version != expectedVersion {
		return false, nil
	}
	f.stored.Status = "MANUAL_PROCESS_ALLOWED"
	f.stored.Version++
	return true, nil
}

// ConfirmCancel 重現 J28 的四個 WHERE 述詞（來源狀態／cancel_requested_at 非空／
// cancel_confirmed_at 為空／version），外加 chk_invop_two_person 這條 **CHECK**——
// CHECK 不在 WHERE 裡，違反時 DB 回 23514 而不是 rows=0，兩者在應用層是不同的路徑
// （前者是 500、後者是 9341）。fake 少了它，「同一人送兩次」那支測試會綠得毫無意義。
//
// 🔴 `f.stored.Version++` 不是裝飾：J28 的 SQL 有 version = version + 1，而端點 55
// 緊接著要用 **+1 後**的版本去做 J29。fake 不遞增的話，傳錯版本的實作照樣全綠。
func (f *fakeOpRepo) ConfirmCancel(_ context.Context, in domain.ConfirmOperationCancel) (bool, error) {
	f.confirmCancel = append(f.confirmCancel, confirmCancelCall{
		ID: in.ID, Version: in.ExpectedVersion, ConfirmedBy: in.ConfirmedBy,
		Kind: in.EvidenceKind, ObjectKey: in.EvidenceObjectKey,
	})
	if f.casFails {
		return false, nil
	}
	if f.stored.Status != "CANCEL_BLOCKED_UNKNOWN" {
		return false, nil
	}
	if f.stored.CancelRequestedBy == nil {
		return false, nil // cancel_requested_at IS NOT NULL（chk_invop_cancel_pair 綁定兩者）
	}
	if f.stored.CancelConfirmedAt != nil {
		return false, nil // 重送同一個確認 ⇒ 冪等的 rows=0
	}
	if f.stored.Version != in.ExpectedVersion {
		return false, nil
	}
	if *f.stored.CancelRequestedBy == in.ConfirmedBy {
		// chk_invop_two_person：DB 回 23514，不是 rows=0。
		return false, errFakeTwoPersonViolation
	}
	f.stored.Status = "CANCELLED_BY_ADMIN"
	f.stored.CancelConfirmedBy = &in.ConfirmedBy
	confirmedAt := in.Now
	f.stored.CancelConfirmedAt = &confirmedAt
	f.stored.Version++
	return true, nil
}

// errFakeTwoPersonViolation 代表 DB 的 chk_invop_two_person 被違反（SQLSTATE 23514）。
var errFakeTwoPersonViolation = errors.New("fake: chk_invop_two_person violated")

type confirmCancelCall struct {
	ID          int64
	Version     int64
	ConfirmedBy int64
	Kind        string
	ObjectKey   *string
}

func (f *fakeOpRepo) GetByKey(context.Context, string) (domain.Operation, error) {
	return f.stored, nil
}

type adminRetryCall struct {
	ID      int64
	Version int64
	Hash    string
}

// fakeAudit 的 ADM 補充。稽核列是 5 年保存義務，測試要看得到 before/after 的內容。
func (f *fakeAudit) AppendAdminAuditLog(_ context.Context, in domain.NewAdminAuditLogEntry) (int64, error) {
	f.admin = append(f.admin, in)
	return int64(len(f.admin)), nil
}

type fakeComplaintRepo struct {
	domain.ComplaintRepository

	stored domain.Complaint
	err    error

	casFails bool

	decided  []domain.DecideComplaint
	returned []adminRetryCall
	events   []domain.NewComplaintEvent

	// S10（端點 36）。
	sellerErr   error
	sellerReads [][2]int64
	responded   []complaintSellerCall
	escalated   []complaintSellerCall

	violationCount int64
}

func (f *fakeComplaintRepo) GetByID(context.Context, int64) (domain.Complaint, error) {
	return f.stored, f.err
}

func (f *fakeComplaintRepo) DecideEscalated(_ context.Context, in domain.DecideComplaint) (bool, error) {
	f.decided = append(f.decided, in)
	if f.casFails {
		return false, nil
	}
	f.stored.Status = in.NextStatus
	f.stored.Version++
	return true, nil
}

func (f *fakeComplaintRepo) ReturnToSeller(_ context.Context, id, expectedVersion int64, _, _ time.Time) (bool, error) {
	f.returned = append(f.returned, adminRetryCall{ID: id, Version: expectedVersion})
	if f.casFails {
		return false, nil
	}
	f.stored.Status = "OPEN"
	f.stored.Version++
	return true, nil
}

func (f *fakeComplaintRepo) AppendEvent(_ context.Context, in domain.NewComplaintEvent) (domain.ComplaintEvent, error) {
	f.events = append(f.events, in)
	return domain.ComplaintEvent{}, nil
}

func (f *fakeComplaintRepo) CountConfirmedViolationOrders(context.Context, int64, time.Time) (int64, error) {
	return f.violationCount, nil
}

// fakeBizDays 是一份空的工作天曆（沒有假日）。K10 只需要它算得出 sla_due_at。
type fakeBizDays struct {
	domain.BusinessDayRepository
	synced *time.Time
}

func (f *fakeBizDays) ListHolidays(context.Context, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}

func (f *fakeBizDays) SyncedAt(context.Context) (*time.Time, error) { return f.synced, nil }

// ---------------------------------------------------------------------------
// M4 provider 帳號寫入面（端點 24／25／26／27／44／45／66／67）的 fake 補充
//
// 每一支 CAS 都有一個獨立的 ok 旗標可以關掉：**rows=0 是本波最重要的陰性情境**，
// 而它在真的 DB 上是靜默的（不拋例外、不告警）——測不到它就等於沒有測守衛。
// 旗標刻意不共用一個 casFails：allowed-from 與樂觀鎖是兩個不同的守衛，共用一個旗標
// 就分不出測試在驗哪一個。
// ---------------------------------------------------------------------------

type providerCall struct {
	ID      int64
	Version int64
	Status  string
	Reason  string
	Extra   string
}

type fakeProviderAccountRepo struct {
	domain.ProviderAccountRepository

	account domain.ProviderAccount
	getErr  error

	// 解密後餵給 verifyCredential 的憑證。
	taxID   string
	appKey  string
	openErr error

	// 明文旁路偵測：任何一次寫入路徑收到的 AppKey 都記在這裡，
	// 測試用它斷言「明文有到 repository、而且沒有到別的地方」。
	seenAppKeys []string

	created    []domain.NewProviderAccount
	createErr  error
	resubmit   []providerCall
	resubmitOK bool
	taken      bool

	startVerify   []providerCall
	startVerifyOK bool
	landed        []domain.LandProviderVerification
	landOK        bool
	failedActive  []domain.LandProviderVerification
	failActiveOK  bool
	touched       []providerCall
	touchOK       bool

	rotationStarted []providerCall
	rotationErr     error
	promoted        []domain.PromoteCredential
	promoteErr      error
	aborted         []providerCall
	abortOK         bool
	matches         bool

	cutovers  []providerCall
	cutoverOK bool
	suspended []providerCall
	suspendOK bool
	revoked   []providerCall
	revokeOK  bool
}

func newFakeProviderAccountRepo(account domain.ProviderAccount) *fakeProviderAccountRepo {
	return &fakeProviderAccountRepo{
		account: account, taxID: "12345678", appKey: "stored-app-key",
		resubmitOK: true, startVerifyOK: true, landOK: true, failActiveOK: true,
		touchOK: true, abortOK: true, cutoverOK: true, suspendOK: true, revokeOK: true,
	}
}

func (f *fakeProviderAccountRepo) GetByIssuer(context.Context, int64, string) (domain.ProviderAccount, error) {
	if f.getErr != nil {
		return domain.ProviderAccount{}, f.getErr
	}
	return f.account, nil
}

func (f *fakeProviderAccountRepo) WithDecryptedCredential(_ context.Context, _ int64, fn func(string, string) error) error {
	if f.openErr != nil {
		return f.openErr
	}
	return fn(f.taxID, f.appKey)
}

func (f *fakeProviderAccountRepo) Create(_ context.Context, in domain.NewProviderAccount) error {
	f.seenAppKeys = append(f.seenAppKeys, in.AppKey)
	f.created = append(f.created, in)
	if f.createErr != nil {
		return f.createErr
	}
	f.account = domain.ProviderAccount{
		ID: in.ID, IssuerID: in.IssuerID, Provider: in.Provider,
		ProviderTaxID: in.ProviderTaxID, Status: in.Status, Version: 0,
	}
	// 建列成功之後那一列就存在了 ⇒ 後續的重讀（viewOf）不能再回 9330。
	// 少了這一行，測試會在「已經建好卻讀不到」這個真實世界不存在的狀態上失敗。
	f.getErr = nil
	return nil
}

func (f *fakeProviderAccountRepo) ResubmitCredential(_ context.Context, in domain.ResubmitProviderCredential) (bool, bool, error) {
	f.seenAppKeys = append(f.seenAppKeys, in.AppKey)
	f.resubmit = append(f.resubmit, providerCall{ID: in.ID, Version: in.ExpectedVersion, Extra: in.ProviderTaxID})
	if f.taken {
		return false, true, nil
	}
	if !f.resubmitOK {
		return false, false, nil
	}
	f.account.Status = "CREDENTIAL_VERIFYING"
	f.account.ProviderTaxID = in.ProviderTaxID
	f.account.Version++
	return true, false, nil
}

func (f *fakeProviderAccountRepo) StartVerification(_ context.Context, id, expectedVersion int64, _ time.Time) (bool, error) {
	f.startVerify = append(f.startVerify, providerCall{ID: id, Version: expectedVersion})
	if !f.startVerifyOK {
		return false, nil
	}
	f.account.Status = "CREDENTIAL_VERIFYING"
	f.account.Version++
	return true, nil
}

func (f *fakeProviderAccountRepo) LandVerification(_ context.Context, in domain.LandProviderVerification) (bool, error) {
	f.landed = append(f.landed, in)
	if !f.landOK {
		return false, nil
	}
	f.account.Status = in.NextStatus
	f.account.VerifiedAt = in.VerifiedAt
	f.account.Version++
	return true, nil
}

func (f *fakeProviderAccountRepo) FailVerificationFromActive(_ context.Context, in domain.LandProviderVerification) (bool, error) {
	f.failedActive = append(f.failedActive, in)
	if !f.failActiveOK {
		return false, nil
	}
	f.account.Status = in.NextStatus
	f.account.Version++
	return true, nil
}

func (f *fakeProviderAccountRepo) TouchHealthCheck(_ context.Context, id, expectedVersion int64, _, _ time.Time) (bool, error) {
	f.touched = append(f.touched, providerCall{ID: id, Version: expectedVersion})
	if !f.touchOK {
		return false, nil
	}
	f.account.Version++
	return true, nil
}

func (f *fakeProviderAccountRepo) StartCredentialRotation(_ context.Context, in domain.StartCredentialRotation) error {
	f.seenAppKeys = append(f.seenAppKeys, in.AppKey)
	f.rotationStarted = append(f.rotationStarted, providerCall{ID: in.ID, Version: in.ExpectedVersion})
	if f.rotationErr != nil {
		return f.rotationErr
	}
	f.account.Status = "ROTATION_PENDING"
	f.account.Version++
	return nil
}

func (f *fakeProviderAccountRepo) PromoteCredential(_ context.Context, in domain.PromoteCredential) error {
	f.seenAppKeys = append(f.seenAppKeys, in.AppKey)
	f.promoted = append(f.promoted, in)
	if f.promoteErr != nil {
		return f.promoteErr
	}
	f.account.Status = in.NextStatus
	f.account.Version++
	return nil
}

func (f *fakeProviderAccountRepo) AbortCredentialRotation(_ context.Context, id, expectedVersion int64, reason string, _ time.Time) (bool, error) {
	f.aborted = append(f.aborted, providerCall{ID: id, Version: expectedVersion, Reason: reason})
	if !f.abortOK {
		return false, nil
	}
	f.account.Status = "ACTIVE"
	f.account.Version++
	return true, nil
}

func (f *fakeProviderAccountRepo) MatchesCurrentCredential(context.Context, int64, string, string) (bool, error) {
	return f.matches, nil
}

func (f *fakeProviderAccountRepo) SetAutomationCutover(_ context.Context, id, expectedVersion int64, cutoverAt, _ time.Time) (bool, error) {
	f.cutovers = append(f.cutovers, providerCall{ID: id, Version: expectedVersion, Extra: cutoverAt.Format(time.RFC3339)})
	if !f.cutoverOK {
		return false, nil
	}
	f.account.AutomationCutoverAt = &cutoverAt
	f.account.Version++
	return true, nil
}

func (f *fakeProviderAccountRepo) Suspend(_ context.Context, id, expectedVersion int64, reason string, _ time.Time) (bool, error) {
	f.suspended = append(f.suspended, providerCall{ID: id, Version: expectedVersion, Reason: reason})
	if !f.suspendOK {
		return false, nil
	}
	f.account.Status = "SUSPENDED"
	f.account.Version++
	return true, nil
}

func (f *fakeProviderAccountRepo) Revoke(_ context.Context, id, expectedVersion int64, reason string, _ time.Time) (bool, error) {
	f.revoked = append(f.revoked, providerCall{ID: id, Version: expectedVersion, Reason: reason})
	if !f.revokeOK {
		return false, nil
	}
	f.account.Status = "REVOKED"
	f.account.Version++
	return true, nil
}

// fakeOpRepo 的 M4 補充。failOpen 記下每一次集合式轉終態，openCount 是 pendingOrderCount。
func (f *fakeOpRepo) FailOpenForIssuer(_ context.Context, in domain.FailOpenOperationsForIssuer) (int64, error) {
	f.failOpen = append(f.failOpen, in)
	return f.openCount, nil
}

func (f *fakeOpRepo) CountOpenForIssuer(context.Context, int64) (int64, error) {
	return f.openCount, nil
}

func (f *fakeOpRepo) GetViewForSeller(_ context.Context, id, storeID int64) (domain.OperationView, error) {
	if f.sellerViewErr != nil {
		return domain.OperationView{}, f.sellerViewErr
	}
	f.sellerViews = append(f.sellerViews, [2]int64{id, storeID})
	return domain.OperationView{Operation: f.stored, IssuerRefID: storeID}, nil
}

// fakeComplaintRepo 的 S10 補充。
func (f *fakeComplaintRepo) GetForSeller(_ context.Context, id, storeID int64) (domain.Complaint, error) {
	if f.sellerErr != nil {
		return domain.Complaint{}, f.sellerErr
	}
	f.sellerReads = append(f.sellerReads, [2]int64{id, storeID})
	return f.stored, nil
}

func (f *fakeComplaintRepo) MarkSellerResponded(_ context.Context, id, storeID, expectedVersion int64, dueAt, _ time.Time) (bool, error) {
	f.responded = append(f.responded, complaintSellerCall{ID: id, StoreID: storeID, Version: expectedVersion, DueAt: dueAt})
	if f.casFails {
		return false, nil
	}
	f.stored.Status = "SELLER_RESPONDED"
	f.stored.BuyerReviewDueAt = &dueAt
	f.stored.Version++
	return true, nil
}

func (f *fakeComplaintRepo) EscalateBySeller(_ context.Context, id, storeID, expectedVersion int64, _ time.Time) (bool, error) {
	f.escalated = append(f.escalated, complaintSellerCall{ID: id, StoreID: storeID, Version: expectedVersion})
	if f.casFails {
		return false, nil
	}
	f.stored.Status = "ESCALATED_TO_ADM"
	f.stored.Version++
	return true, nil
}

func (f *fakeComplaintRepo) ListEvents(context.Context, int64) ([]domain.ComplaintEvent, error) {
	return nil, nil
}

type complaintSellerCall struct {
	ID      int64
	StoreID int64
	Version int64
	DueAt   time.Time
}

// fakeUnlockRepo 是 invoice_manual_unlocks 的替身（端點 54／55）。
//
// 它刻意**在記憶體裡重現兩條 SQL 守衛**（雙人與 24 小時窗口），因為那兩條正是本流程的
// 全部價值：替身若無條件回 true，「不得同一人」與「逾期不可核准」的測試就會全綠而
// 什麼都沒驗到。真 SQL 的鑑別力另由 integration 測試守。
type fakeUnlockRepo struct {
	domain.ManualUnlockRepository

	rows []domain.ManualUnlock

	expired  int
	created  []domain.NewManualUnlock
	decided  []domain.DecideManualUnlock
	takenNow bool // 下一次 Create 模擬撞 uq_invmu_open_per_operation
}

func (f *fakeUnlockRepo) ExpireStale(_ context.Context, operationID int64, now time.Time) (int64, error) {
	n := int64(0)
	for i := range f.rows {
		r := &f.rows[i]
		if r.OperationID == operationID && r.Status == domain.ManualUnlockAwaitingSecondApproval && !now.Before(r.ExpiresAt) {
			r.Status = domain.ManualUnlockExpired
			n++
		}
	}
	f.expired += int(n)
	return n, nil
}

func (f *fakeUnlockRepo) Create(_ context.Context, in domain.NewManualUnlock) (domain.ManualUnlock, bool, error) {
	f.created = append(f.created, in)
	if f.takenNow {
		return domain.ManualUnlock{}, true, nil
	}
	row := domain.ManualUnlock{
		ID:                    in.ID,
		OperationID:           in.OperationID,
		Status:                domain.ManualUnlockAwaitingSecondApproval,
		RequestedByAccountID:  in.RequestedByAccountID,
		RequestReason:         in.RequestReason,
		ProviderCheckEvidence: in.ProviderCheckEvidence,
		ProviderCheckedAt:     in.ProviderCheckedAt,
		RequestedAt:           in.Now,
		ExpiresAt:             in.ExpiresAt,
		// 🔴 這兩欄非帶不可：覆核（J28）從**這一列**讀證據種類。fake 不保存的話，
		// 覆核端永遠讀到 nil ⇒「證據不合法」那條分支恆成立，J28 那支測試會用一個
		// 假的理由失敗，而真正的連動一行都沒被驗到。
		CancelEvidenceKind:      in.CancelEvidenceKind,
		CancelEvidenceObjectKey: in.CancelEvidenceObjectKey,
	}
	f.rows = append(f.rows, row)
	return row, false, nil
}

func (f *fakeUnlockRepo) GetByID(_ context.Context, id int64) (domain.ManualUnlock, error) {
	for _, r := range f.rows {
		if r.ID == id {
			return r, nil
		}
	}
	return domain.ManualUnlock{}, notFoundErr(bizcode.InvoiceJobNotFound)
}

func (f *fakeUnlockRepo) GetOpen(_ context.Context, operationID int64, now time.Time) (domain.ManualUnlock, bool, error) {
	for _, r := range f.rows {
		if r.OperationID == operationID && r.Status == domain.ManualUnlockAwaitingSecondApproval && now.Before(r.ExpiresAt) {
			return r, true, nil
		}
	}
	return domain.ManualUnlock{}, false, nil
}

func (f *fakeUnlockRepo) GetLatest(_ context.Context, operationID int64) (domain.ManualUnlock, bool, error) {
	for i := len(f.rows) - 1; i >= 0; i-- {
		if f.rows[i].OperationID == operationID {
			return f.rows[i], true, nil
		}
	}
	return domain.ManualUnlock{}, false, nil
}

// Decide 重現 DecideManualUnlock 的四個 WHERE 述詞。
func (f *fakeUnlockRepo) Decide(_ context.Context, in domain.DecideManualUnlock) (bool, error) {
	f.decided = append(f.decided, in)
	for i := range f.rows {
		r := &f.rows[i]
		switch {
		case r.ID != in.ID, r.OperationID != in.OperationID:
			continue
		case r.Status != domain.ManualUnlockAwaitingSecondApproval:
			return false, nil
		case !in.Now.Before(r.ExpiresAt):
			return false, nil
		case r.RequestedByAccountID == in.ApprovedByAccountID:
			return false, nil
		case r.Version != in.ExpectedVersion:
			return false, nil
		}
		approver := in.ApprovedByAccountID
		decidedAt := in.Now
		reason := in.DecisionReason
		r.Status = in.NextStatus
		r.ApprovedByAccountID = &approver
		r.DecidedAt = &decidedAt
		r.DecisionReason = &reason
		r.Version++
		return true, nil
	}
	return false, nil
}

// fakeCutoverRepo 是 invoice_provider_cutovers 的替身（端點 26／46／67）。
type fakeCutoverRepo struct {
	domain.ProviderCutoverRepository

	rows   []domain.ProviderCutover
	closed int
}

func (f *fakeCutoverRepo) CloseOpen(_ context.Context, providerAccountID int64, now time.Time) (int64, error) {
	n := int64(0)
	for i := range f.rows {
		r := &f.rows[i]
		if r.ProviderAccountID == providerAccountID && r.DeactivatedAt == nil {
			t := now
			r.DeactivatedAt = &t
			n++
		}
	}
	f.closed += int(n)
	return n, nil
}

func (f *fakeCutoverRepo) Create(_ context.Context, in domain.NewProviderCutover) (domain.ProviderCutover, error) {
	row := domain.ProviderCutover{
		ID:                 int64(len(f.rows) + 1),
		ProviderAccountID:  in.ProviderAccountID,
		IssuerID:           in.IssuerID,
		AccountVersion:     in.AccountVersion,
		CutoverAt:          in.CutoverAt,
		Mode:               "AMEGO_AUTO",
		CreatedByAccountID: in.CreatedByAccountID,
		CreatedAt:          in.Now,
	}
	f.rows = append(f.rows, row)
	return row, nil
}

func (f *fakeCutoverRepo) ListForAdmin(_ context.Context, providerAccountID int64, _ domain.Page) ([]domain.ProviderCutover, error) {
	out := make([]domain.ProviderCutover, 0, len(f.rows))
	for i := len(f.rows) - 1; i >= 0; i-- {
		if f.rows[i].ProviderAccountID == providerAccountID {
			out = append(out, f.rows[i])
		}
	}
	return out, nil
}

func (f *fakeCutoverRepo) CountForAdmin(_ context.Context, providerAccountID int64) (int64, error) {
	n := int64(0)
	for _, r := range f.rows {
		if r.ProviderAccountID == providerAccountID {
			n++
		}
	}
	return n, nil
}

// VoidInvoice 讓 fakeProvider 滿足加寬後的 ProviderClient。
//
// 🔴 預設**拒絕**而不是回成功：這個 fake 服務的是開票軸的測試，那些測試沒有一個
// 應該打到 f0501。回 nil 會讓「作廢單被拿去走開票路徑」這類分派錯誤在測試裡看不出來
// ——fake 的鑑別力來自它拒絕什麼。作廢軸的測試用自己的 fake（fakeVoidProvider）。
// IssueAllowance／QueryAllowance 讓 fakeProvider 滿足加寬後的 ProviderClient（#3073）。
//
// 預設 fail-loud：開票軸與作廢軸的測試都不該打折讓端點，真的打到代表分派錯了
// —— 那正是 isVoidOperation／isAllowanceOperation 要防的事。折讓軸自己的測試
// 用 allowanceFn 注入。
func (f *fakeProvider) IssueAllowance(_ context.Context, _ amego.Credential, req amego.AllowanceRequest) (*amego.AllowanceResult, error) {
	f.allowanceCalls++
	if f.allowanceFn != nil {
		return f.allowanceFn(req)
	}
	return nil, fmt.Errorf("fakeProvider: unexpected IssueAllowance(%s)", req.AllowanceNumber)
}

func (f *fakeProvider) QueryAllowance(_ context.Context, _ amego.Credential, no string) (*amego.AllowanceDetail, error) {
	if f.allowanceQueryFn != nil {
		return f.allowanceQueryFn(no)
	}
	return nil, fmt.Errorf("fakeProvider: unexpected QueryAllowance(%s)", no)
}

func (f *fakeProvider) VoidInvoice(_ context.Context, _ amego.Credential, req amego.VoidRequest) (*amego.VoidAccepted, error) {
	if f.voidFn != nil {
		return f.voidFn(req)
	}
	return nil, fmt.Errorf("fakeProvider: unexpected VoidInvoice(%s) — 開票軸的測試不該打 f0501", req.CancelInvoiceNumber)
}

// QueryInvoiceByNumber 讓 fakeProvider 滿足加寬後的 ProviderClient。
func (f *fakeProvider) QueryInvoiceByNumber(_ context.Context, _ amego.Credential, number string) (*amego.InvoiceDetail, error) {
	if f.queryByNumberFn != nil {
		return f.queryByNumberFn(number)
	}
	return nil, fmt.Errorf("fakeProvider: unexpected QueryInvoiceByNumber(%s)", number)
}

// fakeCorrections 是 domain.CorrectionRequestRepository 的測試替身。
//
// 🔴 預設對每一個查詢都回「沒有 open correction」（業務碼），對每一個寫入都**失敗**：
// 開票軸的既有測試沒有一支涉及更正請求，回成功會讓 R10 的漏接看不出來。
// 要測 R10 的測試自己把 openByDoc 設進去。
type fakeCorrections struct {
	openByDoc   map[int64]domain.CorrectionRequest
	failedByDoc map[int64]domain.CorrectionRequest
	// latestByOrder 是第三份 map，理由同 failedByDoc 的說明：真實 SQL 是三支述詞
	// 不同的查詢（進行中／REISSUE_FAILED／不分狀態取最新），共用一份 map 的替身
	// 會讓「呼叫端查錯那一支」測不出來 —— 而端點 8 要的正是**被婉拒的那一筆**，
	// 它不在前兩支的述詞裡。
	//
	// 🔴 鍵是 **order_no** 不是 document_id：更正請求掛在被作廢的那張 document 上，
	// 作廢重開完成後訂單的最新 document 是重開的新那張、身上沒有 correction 列
	// ⇒ 以 document 為鍵的替身複製不出那個缺陷（2026-09-05 修）。
	latestByOrder map[string]domain.CorrectionRequest
	completed     []int64
	reissueFail   []int64
	retried       []int64
	completeOK    bool
	failErr       error
}

func (f *fakeCorrections) GetByID(context.Context, int64) (domain.CorrectionRequest, error) {
	return domain.CorrectionRequest{}, pkgerrors.NewBusiness(bizcode.InvoiceCorrectionNotAllowed, "no correction")
}

func (f *fakeCorrections) GetOpenByDocument(_ context.Context, documentID int64) (domain.CorrectionRequest, error) {
	if c, ok := f.openByDoc[documentID]; ok {
		return c, nil
	}
	return domain.CorrectionRequest{}, pkgerrors.NewBusiness(bizcode.InvoiceCorrectionNotAllowed, "no open correction")
}

// GetReissueFailedByDocument 與 GetOpenByDocument **分開兩份 map**，這不是重複：
// 真實 SQL 就是兩支述詞不相交的查詢（REQUESTED／VOID_REISSUE_IN_PROGRESS
// vs REISSUE_FAILED）。用同一份 map 服務兩者的替身，會讓「呼叫端查錯那一支」
// 這個真實缺陷（#3089 A 組第 1 項的第二層）永遠測不出來。
func (f *fakeCorrections) GetReissueFailedByDocument(_ context.Context, documentID int64) (domain.CorrectionRequest, error) {
	if c, ok := f.failedByDoc[documentID]; ok {
		return c, nil
	}
	return domain.CorrectionRequest{}, pkgerrors.NewBusiness(bizcode.InvoiceCorrectionNotAllowed, "no reissue-failed correction")
}

func (f *fakeCorrections) GetLatestByOrder(_ context.Context, orderNo string, _ int64) (domain.CorrectionRequest, error) {
	if c, ok := f.latestByOrder[orderNo]; ok {
		return c, nil
	}
	return domain.CorrectionRequest{}, pkgerrors.NewBusiness(bizcode.InvoiceCorrectionNotAllowed, "no correction")
}

func (f *fakeCorrections) RetryReissue(_ context.Context, id, _ int64, _ time.Time) (bool, error) {
	f.retried = append(f.retried, id)
	return true, nil
}

func (f *fakeCorrections) GetForBuyer(context.Context, int64, int64, string) (domain.CorrectionRequest, error) {
	return domain.CorrectionRequest{}, pkgerrors.NewBusiness(bizcode.InvoiceCorrectionNotAllowed, "no correction")
}

func (f *fakeCorrections) Insert(context.Context, domain.NewCorrectionRequest) (domain.CorrectionRequest, bool, error) {
	return domain.CorrectionRequest{}, false, errors.New("fakeCorrections: unexpected Insert")
}

func (f *fakeCorrections) Reject(context.Context, domain.RejectCorrection) (bool, error) {
	return false, errors.New("fakeCorrections: unexpected Reject")
}

func (f *fakeCorrections) MarkUpdatedBeforeIssue(context.Context, int64, time.Time) (bool, error) {
	return false, errors.New("fakeCorrections: unexpected MarkUpdatedBeforeIssue")
}

func (f *fakeCorrections) StartVoidReissue(context.Context, int64, int64, time.Time) (bool, error) {
	return false, errors.New("fakeCorrections: unexpected StartVoidReissue")
}

func (f *fakeCorrections) MarkReissueFailed(_ context.Context, id int64, _ time.Time) (bool, error) {
	f.reissueFail = append(f.reissueFail, id)
	return true, f.failErr
}

func (f *fakeCorrections) Complete(_ context.Context, id, _ int64, _ time.Time) (bool, error) {
	f.completed = append(f.completed, id)
	return f.completeOK, f.failErr
}

// ---------------------------------------------------------------------------
// 平台代製收據的兩個替身（#3026）
// ---------------------------------------------------------------------------

// fakeReceiptRenderer 產出一份**看起來像 PDF** 的 bytes 並記下收到的資料。
//
// 🔴 它的鑑別力來自「記下 in」而不是「回了 bytes」：settleReceipt 的正確性
// 有一半在於**餵給渲染器的資料對不對**（出據人是誰、買受人有沒有個資、金額對不對），
// 而那件事只能在這裡看到。回一個固定 blob 只驗得到「有呼叫」。
type fakeReceiptRenderer struct {
	in    domain.ReceiptData
	calls int
	err   error
}

func (f *fakeReceiptRenderer) Render(d domain.ReceiptData) ([]byte, error) {
	f.calls++
	f.in = d
	if f.err != nil {
		return nil, f.err
	}
	return []byte("%PDF-1.3\nfake\n%%EOF"), nil
}

// fakeReceiptUploader 記下上傳內容並回一個固定 objectKey。
type fakeReceiptUploader struct {
	data        []byte
	filename    string
	contentType string
	refID       string
	calls       int
	key         string
	err         error
}

func (f *fakeReceiptUploader) UploadSecure(
	_ context.Context, data []byte, filename, contentType, refID string,
) (string, error) {
	f.calls++
	f.data, f.filename, f.contentType, f.refID = data, filename, contentType, refID
	if f.err != nil {
		return "", f.err
	}
	if f.key == "" {
		return "secure/receipt/fake.pdf", nil
	}
	return f.key, nil
}
