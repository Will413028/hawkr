package usecase

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// Query 是唯讀查詢面（買家 B08／B07、賣家 S02–S05、ADM A02/A04/A05/A06/A07）。
//
// 它刻意**沒有** TxManager、ProviderClient、Outbox：三者任一在圖上出現，就是一條
// 「有人以為可以在這裡開票／發事件」的路。唯一的寫入是 ADM 揭露 P 類資料時的稽核列
// （§8.1.3(c) 裁定 2），那是讀取的**代價**不是業務寫入，走 audit repository 單列 append。
type Query struct {
	issuers    domain.IssuerRepository
	profiles   domain.SellerTaxProfileRepository
	accounts   domain.ProviderAccountRepository
	prefs      domain.BuyerPreferenceRepository
	documents  domain.DocumentRepository
	operations domain.OperationRepository
	complaints domain.ComplaintRepository
	audit      domain.AuditRepository
	adjustments domain.AdjustmentRepository
	corrections domain.CorrectionRequestRepository
	overview domain.OverviewRepository
	businessDay domain.BusinessDayRepository
	cutovers domain.ProviderCutoverRepository
	unlocks domain.ManualUnlockRepository
	cancelledOrders domain.CancelledOrderRepository
	cfg             IssuanceConfig
	now             func() time.Time
	log             *zap.Logger
}

// NewQuery 組出唯讀查詢 usecase。
func NewQuery(
	issuers domain.IssuerRepository,
	profiles domain.SellerTaxProfileRepository,
	accounts domain.ProviderAccountRepository,
	prefs domain.BuyerPreferenceRepository,
	documents domain.DocumentRepository,
	operations domain.OperationRepository,
	complaints domain.ComplaintRepository,
	audit domain.AuditRepository,
	adjustments domain.AdjustmentRepository,
	corrections domain.CorrectionRequestRepository,
	overview domain.OverviewRepository,
	businessDay domain.BusinessDayRepository,
	cutovers domain.ProviderCutoverRepository,
	unlocks domain.ManualUnlockRepository,
	cancelledOrders domain.CancelledOrderRepository,
	cfg IssuanceConfig,
	log *zap.Logger,
) *Query {
	return &Query{
		issuers:         issuers,
		profiles:        profiles,
		accounts:        accounts,
		prefs:           prefs,
		documents:       documents,
		operations:      operations,
		complaints:      complaints,
		audit:           audit,
		adjustments:     adjustments,
		corrections:     corrections,
		overview:        overview,
		businessDay:     businessDay,
		cutovers:        cutovers,
		unlocks:         unlocks,
		cancelledOrders: cancelledOrders,
		cfg:             cfg,
		now:             func() time.Time { return time.Now().UTC() },
		log:             log,
	}
}

// WithClock 注入時間來源（測試用）。
func (u *Query) WithClock(now func() time.Time) *Query {
	if now != nil {
		u.now = now
	}
	return u
}


const (
	auditActionPIIReveal = "INVOICE_PII_REVEAL"

	auditResourceDocument  = "INVOICE_DOCUMENT"
	auditResourceComplaint = "INVOICE_COMPLAINT"

	auditReasonRead       = "READ"
	auditReasonDetailRead = "ADM 讀取含 P 類欄位的發票詳情（58 §8.1.3(c) 裁定 2）"
)

func (u *Query) recordPIIReveal(ctx context.Context, actorAccountID int64, resourceType string, resourceID int64) error {
	corr := interceptor.CorrelationIDFromContext(ctx)
	var corrPtr *string
	if corr != "" {
		corrPtr = &corr
	}
	if _, err := u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
		ActorAccountID: actorAccountID,
		Action:         auditActionPIIReveal,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		ReasonCode:     auditReasonRead,
		ReasonDetail:   auditReasonDetailRead,
		CorrelationID:  corrPtr,
		Now:            u.now(),
	}); err != nil {
		u.logger().Error("invoice.query.pii_reveal_audit.failed",
			zap.String("resource_type", resourceType), zap.Int64("resource_id", resourceID), zap.Error(err))
		return err
	}
	observeAdminPIIReveal()
	return nil
}


const buyerProfileCap = 20

const (
	buyerProfileDefaultPageSize = 10
	buyerProfileMaxPageSize     = buyerProfileCap
)

// BuyerProfilePage 是 B08 列表的一頁。
type BuyerProfilePage struct {
	Records  []CheckoutProfile
	Total    int32
	Page     int32
	PageSize int32
}

// ListBuyerProfiles 回這位買家的常用發票資料（B08）。
//
// Tier A：buyerID 進 SQL 的 WHERE（BuyerPreferenceRepository.ListByBuyer）。別人的
// buyer_id 只會得到空列表，不會有任何可分辨的錯誤。
//
// 🔴 明碼不出服務邊界：DisplayValue 取已落庫的 carrier_value_mask，本路徑**不解密**。
func (u *Query) ListBuyerProfiles(ctx context.Context, buyerID int64, page, pageSize int32) (BuyerProfilePage, error) {
	normPage, normSize, offset := pagination.NormalizePage(page, pageSize, buyerProfileDefaultPageSize, buyerProfileMaxPageSize)

	rows, err := u.prefs.ListByBuyer(ctx, buyerID, buyerProfileCap+1)
	if err != nil {
		return BuyerProfilePage{}, err
	}
	if len(rows) > buyerProfileCap {
		u.logger().Error("invoice.query.buyer_profiles.cap_exceeded",
			zap.Int64("buyer_id", buyerID), zap.Int("alive_rows", len(rows)), zap.Int("cap", buyerProfileCap))
	}

	total := int32(len(rows)) //nolint:gosec // 上限 cap+1，不可能溢位
	out := BuyerProfilePage{Records: []CheckoutProfile{}, Total: total, Page: normPage, PageSize: normSize}
	if offset >= total {
		return out, nil
	}
	end := offset + normSize
	if end > total {
		end = total
	}
	for _, row := range rows[offset:end] {
		out.Records = append(out.Records, toCheckoutProfile(row))
	}
	return out, nil
}


// ComplaintView 是一件申訴案的完整投影。
//
// ⚠️ RemainingBusinessDays／OverdueBusinessDays 是 **nil**（未計算），不是 0：
// 3 工作天 SLA 的算式要 pkg/bizcalendar（§5.1.14／設計書 :170 逐字「禁止複製第二份
// 假日邏輯」），而該 package 目前**不存在**。回 0 會被前端讀成「今天到期」，
// 而算成自然日會偏早 ⇒ 賣家被誤判逾期 ⇒ 違規計數 +1（§5.1.14 表格逐字）。
// 缺口登記在交付回報。
type ComplaintView struct {
	Complaint             domain.Complaint
	Events                []domain.ComplaintEvent
	RemainingBusinessDays *int32
	OverdueBusinessDays   *int32
	// ExpectedDocKind ＝ 被申訴的那份文件的 document_kind（B2C／B2B／RECEIPT，#3031）。
	// 58A §S09 的申訴卡有一行「買家要的是：發票／收據」，來源就是它；恆為空字串時那一行是空的。
	// 取不到（文件被刪、或讀取失敗）時留空字串——申訴本身的資料完全健康，
	// 不該因為一個顯示欄位讓整頁打不開。
	ExpectedDocKind string
	// StoreID ＝ invoice_issuers.issuer_ref_id（**不是** complaint.IssuerID，那是 PK）。
	//
	// 🔴 只有 ADM 詳情（端點 59）填它：A08 的「賣家：{storeId}」讀這一欄，而
	// ADM 列表（端點 58）早就從 SQL join 帶回同一個值。買家面（B07）與賣家面（S10）
	// 刻意留 0 —— 兩者的畫面都不顯示它，多查一次 issuer 只是白花一次往返。
	// 0 ⇒ 型別邊界回空字串（fmtID 的既有行為）。
	StoreID int64
}

// GetBuyerComplaint 回買家自己的一件申訴（端點 15）。
//
// Tier A：complainant_buyer_id 進 SQL 的 WHERE。order_no 不匹配時走**同一個**
// bizcode.InvoiceComplaintNotFound —— 不得為它另開一個碼，那等於在回應裡放一個
// 可以區分「不是你的」與「不存在」的旗標（§8.1.4(c)）。
func (u *Query) GetBuyerComplaint(ctx context.Context, buyerID int64, orderNo string, complaintID int64) (ComplaintView, error) {
	c, err := u.complaints.GetForBuyer(ctx, complaintID, buyerID)
	if err != nil {
		return ComplaintView{}, err
	}
	if orderNo != "" && derefString(c.OrderNo) != orderNo {
		return ComplaintView{}, complaintNotFound()
	}
	return u.complaintView(ctx, c)
}

// GetComplaintForAdmin 回一件申訴的 ADM 視角（端點 59）。
//
// 沒有擁有者述詞是刻意的：ADM 的授權在 Gateway 的 requireAccountType(ADMIN)。代價是
// 每次讀取都要留 INVOICE_PII_REVEAL 稽核，而那一列寫不進去就不回資料（fail-closed）。
func (u *Query) GetComplaintForAdmin(ctx context.Context, actorAccountID, complaintID int64) (ComplaintView, error) {
	c, err := u.complaints.GetByID(ctx, complaintID)
	if err != nil {
		return ComplaintView{}, err
	}
	if err := u.recordPIIReveal(ctx, actorAccountID, auditResourceComplaint, c.ID); err != nil {
		return ComplaintView{}, err
	}
	v, err := u.complaintView(ctx, c)
	if err != nil {
		return ComplaintView{}, err
	}
	if issuer, ierr := u.issuers.GetByID(ctx, c.IssuerID); ierr == nil {
		v.StoreID = issuer.IssuerRefID
	}
	return v, nil
}

func (u *Query) complaintView(ctx context.Context, c domain.Complaint) (ComplaintView, error) {
	events, err := u.complaints.ListEvents(ctx, c.ID)
	if err != nil {
		return ComplaintView{}, err
	}
	now := u.now()
	cal, err := loadBusinessCalendar(ctx, u.businessDay, minTime(now, c.SlaDueAt), maxTime(now, c.SlaDueAt))
	if err != nil {
		return ComplaintView{}, err
	}
	v := viewWith(c, events, cal, now)
	v.ExpectedDocKind = u.expectedDocKind(ctx, c.DocumentID, nil)
	return v, nil
}

func (u *Query) expectedDocKind(ctx context.Context, documentID int64, memo map[int64]string) string {
	if documentID == 0 || u.documents == nil {
		return ""
	}
	if memo != nil {
		if v, ok := memo[documentID]; ok {
			return v
		}
	}
	kind := ""
	if doc, err := u.documents.GetByID(ctx, documentID); err == nil {
		kind = doc.DocumentKind
	}
	if memo != nil {
		memo[documentID] = kind
	}
	return kind
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func complaintNotFound() error {
	return pkgerrors.NewBusiness(bizcode.InvoiceComplaintNotFound, bizcode.Message(bizcode.InvoiceComplaintNotFound))
}


// SellerTaxProfileView 是稅務資格的投影。賣家面與 ADM 面讀的是同一份資料
// （proto 的 GetTaxReviewForAdminResponse 直接回 SellerTaxProfileData），差別只在
// 授權方式，故共用同一個 view。
type SellerTaxProfileView struct {
	StoreID int64
	Issuer  domain.Issuer
	Profile domain.SellerTaxProfile
}

// GetSellerTaxProfile 回賣家自己的稅務資格（端點 20）。
//
// Tier A 兩段都在 SQL：invoice_issuers.issuer_ref_id = store_id AND issuer_type='SELLER'
// （GetByRef），再以 issuer_id 取 profile。別的 store_id 解出別的 issuer ⇒ 拿不到這一列。
//
// 🔴 issuer 查無與 profile 查無**必須回同一個碼**（9320）：GetByRef 內部把 pgx.ErrNoRows
// 映成 9330（provider 語意），在稅務資格這條路徑上直接回它，會讓「這個 store 還沒有
// issuer」與「有 issuer 但沒送過審」對外可分辨，而兩者對買賣家都只是「還沒設定」。
func (u *Query) GetSellerTaxProfile(ctx context.Context, storeID int64) (SellerTaxProfileView, error) {
	issuer, err := u.sellerIssuer(ctx, storeID, bizcode.SellerTaxProfileNotFound)
	if err != nil {
		return SellerTaxProfileView{}, err
	}
	profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
	if err != nil {
		return SellerTaxProfileView{}, err
	}
	return SellerTaxProfileView{StoreID: storeID, Issuer: issuer, Profile: profile}, nil
}

// GetTaxReviewForAdmin 是同一份資料的 ADM 視角（端點 39）。
//
// 不留 INVOICE_PII_REVEAL：稅務資格的欄位（統編、法人名稱、負責人、營業地址）是 O 類
// ——公開營業登記資訊，§8.1.3(b) 三端皆完整、不列入 P 類。
func (u *Query) GetTaxReviewForAdmin(ctx context.Context, storeID int64) (SellerTaxProfileView, error) {
	return u.GetSellerTaxProfile(ctx, storeID)
}


// ProviderAccountView 是 provider 帳號的投影。
//
// 🔴 這個 struct 裡**沒有、也永遠不會有** app_key 或 fingerprint 欄位（§8.1.1(f) U2）：
// C 類憑證單獨持有即可對外開票，任何端、任何情況都不回顯。賣家自我確認走
// Account.CredentialLast4 ＋ Account.UpdatedAt。
type ProviderAccountView struct {
	StoreID int64
	Account domain.ProviderAccount
	// PendingOrderCount 來自 seller_tax_profiles（A09 顯示「切點前尚未付款的訂單數」）。
	// HasProfile=false ⇒ 這位賣家有 provider 帳號但沒有稅務資格列，數字不可信 ⇒ 不填。
	Profile    domain.SellerTaxProfile
	HasProfile bool
	// LastRequestID 是最近一次 AMEGO 往返在我方 log／trace 上的識別碼（#3031）。
	// 還沒打過 AMEGO 時為空字串——那是合法狀態（剛設定憑證、還沒驗證）。
	LastRequestID string
}

// GetSellerProviderAccount 回賣家自己的 AMEGO 帳號現況（端點 24）。
//
// Tier A：issuer 由 store_id 解出（SQL 述詞），provider account 掛在 issuer_id 上。
// issuer 查無與 account 查無共用 9330——兩者對賣家都只是「尚未設定電子發票服務商連線」。
func (u *Query) GetSellerProviderAccount(ctx context.Context, storeID int64) (ProviderAccountView, error) {
	issuer, err := u.sellerIssuer(ctx, storeID, bizcode.InvoiceProviderAccountNotFound)
	if err != nil {
		return ProviderAccountView{}, err
	}
	account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAMEGO)
	if err != nil {
		return ProviderAccountView{}, err
	}
	view := ProviderAccountView{StoreID: storeID, Account: account}
	if profile, perr := u.profiles.GetByIssuer(ctx, issuer.ID); perr == nil {
		view.Profile, view.HasProfile = profile, true
	} else if !isBizCode(perr, bizcode.SellerTaxProfileNotFound) {
		return ProviderAccountView{}, perr
	}
	if rid, rerr := u.audit.LatestProviderCorrelationID(ctx, issuer.ID); rerr == nil {
		view.LastRequestID = rid
	}
	return view, nil
}

// GetProviderAccountForAdmin 是同一份資料的 ADM 視角（端點 42）。
// C 類憑證在 ADM 端一樣不回顯（§8.1.3(b) 第一列「ADM：永不回顯；同賣家端」）。
func (u *Query) GetProviderAccountForAdmin(ctx context.Context, storeID int64) (ProviderAccountView, error) {
	return u.GetSellerProviderAccount(ctx, storeID)
}

const providerAMEGO = "AMEGO"

func (u *Query) sellerIssuer(ctx context.Context, storeID int64, notFoundCode int32) (domain.Issuer, error) {
	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	if err == nil {
		return issuer, nil
	}
	if isBizCode(err, bizcode.InvoiceProviderAccountNotFound) {
		return domain.Issuer{}, pkgerrors.NewBusiness(notFoundCode, bizcode.Message(notFoundCode))
	}
	return domain.Issuer{}, err
}


// AdminDocumentView 是 ADM 的發票詳情（A06 的四層）。
//
// RevealedCarrierID／RevealedEmail 是**明碼**：§8.1.3(c) 裁定 2「ADM 端一律不遮罩，但
// 每次揭露 P 類資料都要留稽核」。它們只在 recordPIIReveal 成功之後才被填。
type AdminDocumentView struct {
	Document          domain.Document
	Issuer            domain.Issuer
	Lines             []domain.DocumentLine
	Timeline          []domain.StatusHistoryEntry
	Operations        []domain.Operation
	Exchanges         []domain.ProviderAuditLogEntry
	AuditLogs         []domain.AdminAuditLogEntry
	RevealedCarrierID string
	RevealedEmail     string
}

const adminAuditLogLimit = 50

// GetDocumentForAdmin 回一份發票的 ADM 詳情（端點 48）。
//
// 順序是刻意的：**先寫稽核、後解密**。反過來寫，稽核失敗時明碼已經解出來、而且很可能
// 已經在某個 log 欄位裡了。
func (u *Query) GetDocumentForAdmin(ctx context.Context, actorAccountID, documentID int64) (AdminDocumentView, error) {
	doc, err := u.documents.GetByID(ctx, documentID)
	if err != nil {
		return AdminDocumentView{}, err
	}
	if err := u.recordPIIReveal(ctx, actorAccountID, auditResourceDocument, doc.ID); err != nil {
		return AdminDocumentView{}, err
	}

	view := AdminDocumentView{Document: doc}
	if view.Issuer, err = u.issuers.GetByID(ctx, doc.IssuerID); err != nil {
		return AdminDocumentView{}, err
	}
	if view.Lines, err = u.documents.ListLines(ctx, doc.ID); err != nil {
		return AdminDocumentView{}, err
	}
	if view.Timeline, err = u.audit.ListStatusHistory(ctx, subjectDocument, doc.ID); err != nil {
		return AdminDocumentView{}, err
	}
	if view.Operations, err = u.operations.ListByDocument(ctx, doc.ID); err != nil {
		return AdminDocumentView{}, err
	}
	for _, op := range view.Operations {
		ex, exErr := u.audit.ListProviderAuditLogsByOperation(ctx, op.ID)
		if exErr != nil {
			return AdminDocumentView{}, exErr
		}
		view.Exchanges = append(view.Exchanges, ex...)
	}
	if view.AuditLogs, err = u.audit.ListAdminAuditLogsByResource(ctx, auditResourceDocument, doc.ID, adminAuditLogLimit); err != nil {
		return AdminDocumentView{}, err
	}

	if doc.CarrierId1Ciphertext != nil || doc.RecipientEmailCiphertext != nil {
		if err := u.documents.WithDecryptedRecipient(ctx, doc.ID, func(s domain.RecipientSecrets) error {
			view.RevealedCarrierID = s.CarrierID1
			view.RevealedEmail = s.Email
			return nil
		}); err != nil {
			return AdminDocumentView{}, err
		}
	}
	return view, nil
}

// AdminJobView 是 ADM 的開票作業詳情（端點 51）。
type AdminJobView struct {
	Operation domain.Operation
	Document  domain.Document
	Issuer    domain.Issuer
	Exchanges []domain.ProviderAuditLogEntry
	AuditLogs []domain.AdminAuditLogEntry
	// ManualUnlock 是這件 job 最近一次的人工出口解鎖請求（§5.1.16）。
	//
	// nil ＝ **從未發起過**（proto 逐字「null = 尚無解鎖請求」）。不要回一個空物件：
	// 58A §A06 用 `manualUnlock === null` 決定「申請解鎖」按鈕的可見性，空物件會讓
	// 那顆按鈕消失，而 A4 就沒有任何入口。
	//
	// Status 已投影：DB 上仍掛著但已過 24 小時的那一列在這裡是 EXPIRED
	// （逾期不由排程器收斂，§5.1.16 紅字）。
	ManualUnlock *ManualUnlockView
}

// GetJobForAdmin 回一件開票作業的 ADM 詳情（端點 51）。
//
// 一樣留 INVOICE_PII_REVEAL：回應帶 provider_exchanges，而 §8.1.3(b) 最後一列把
// 「provider 稽核 request／response 原文」判為 P 類、ADM「可見（預設收合）＋留稽核」。
// 裁定 2 的內文只列了端點 48／59，但矩陣那一列沒有限定端點——以矩陣為準（見交付回報）。
//
// 🔴 job 的擁有者判定一律經 invoice_documents（§8.1.4 末段）：invoice_operations
// 完全沒有訂單／store 欄，不得自行反正規化一個 order_no。本支是 ADM 面所以沒有擁有者
// 述詞，但賣家版（端點 33）落地時必須走這條 join，不能改成先查 operation 再比對。
func (u *Query) GetJobForAdmin(ctx context.Context, actorAccountID, jobID int64) (AdminJobView, error) {
	op, err := u.operations.GetByID(ctx, jobID)
	if err != nil {
		return AdminJobView{}, err
	}
	doc, err := u.documents.GetByID(ctx, op.DocumentID)
	if err != nil {
		return AdminJobView{}, err
	}
	if err := u.recordPIIReveal(ctx, actorAccountID, auditResourceDocument, doc.ID); err != nil {
		return AdminJobView{}, err
	}
	view := AdminJobView{Operation: op, Document: doc}
	if view.Issuer, err = u.issuers.GetByID(ctx, doc.IssuerID); err != nil {
		return AdminJobView{}, err
	}
	if view.Exchanges, err = u.audit.ListProviderAuditLogsByOperation(ctx, op.ID); err != nil {
		return AdminJobView{}, err
	}
	if view.AuditLogs, err = u.audit.ListAdminAuditLogsByResource(ctx, auditResourceDocument, doc.ID, adminAuditLogLimit); err != nil {
		return AdminJobView{}, err
	}
	unlock, found, err := u.unlocks.GetLatest(ctx, op.ID)
	if err != nil {
		return AdminJobView{}, err
	}
	if found {
		v := toManualUnlockView(unlock, u.now())
		view.ManualUnlock = &v
	}
	return view, nil
}

func (u *Query) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}


// RetryPolicyView 是 T-I12 要求進 job DTO 的退避參數（前端不得自行猜測）。
type RetryPolicyView struct {
	InitialIntervalSeconds int64
	BackoffMultiplier      int64
	MaxIntervalSeconds     int64
	MaxAttempts            int32
}

// RetryPolicy 回本服務實際使用的退避參數。
//
// 🔴 三個值全部取自同一份 IssuanceConfig，而 backoffFor 的乘數是寫死的 2（submit.go）。
// 在這裡另打一份數字，就會出現「畫面上寫 30s×2、實際跑 60s×3」而沒有任何測試會紅。
//
// ⚠️ MaxAttempts 對外顯示用 config 值；worker 判斷「次數用完了沒」讀的是
// invoice_operations.max_attempts 那一列（建列當下寫入）。舊列可能是舊值 ⇒ 逐列的
// max_attempts 由 InvoiceJobInfo.max_attempts 帶出，本結構只描述**現在的政策**。
func (u *Query) RetryPolicy() RetryPolicyView {
	return RetryPolicyView{
		InitialIntervalSeconds: int64(u.cfg.BaseBackoff / time.Second),
		BackoffMultiplier:      2,
		MaxIntervalSeconds:     int64(u.cfg.MaxBackoff / time.Second),
		MaxAttempts:            u.cfg.MaxAttempts,
	}
}

var cancellableOperationStatuses = map[string]struct{}{
	"READY":                  {},
	"RETRY_SCHEDULED":        {},
	"FAILED_PERMANENT":       {},
	"FAILED_RETRY_EXHAUSTED": {},
	"SELLER_ACTION_REQUIRED": {},
}

// CanRetryOperation 回報這件作業現在能不能重試（proto 的 can_retry）。
//
// 🔴 **直接走寫端的判準**，不另存一份清單——見 adminRetryKindOf 的註解。
//
// WAITING_CONFIRMATION／FAILED_UNKNOWN 恆為 false 是 proto 的硬要求，而它們本來就不在
// allowed-from 裡 ⇒ 這裡不需要第二個特例，加了反而會讓兩份清單日後漂移。
func CanRetryOperation(status string) bool {
	return adminRetryKindOf(status) != adminRetryNone
}

// CanCancelOperation 回報這件作業現在能不能取消。
func CanCancelOperation(status string) bool {
	_, ok := cancellableOperationStatuses[status]
	return ok
}

// 發票檔案不可下載的三個原因（proto InvoiceDocumentSummary.file_unavailable_reason）。
const (
	FileUnavailableCarrierNotWon = "CARRIER_NOT_WON"
	FileUnavailableExpired       = "EXPIRED"
	FileUnavailableNotApplicable = "NOT_APPLICABLE"
)

const providerFileRetention = 180 * 24 * time.Hour

// FileAvailability 判斷這份文件有沒有可下載的 PDF，沒有的話為什麼。
//
// 判準與 9312／9313 對齊：載具發票中獎前不提供（9312）、超過查詢期限（9313）。
// 已落地的 pdf_object_key 一律優先——那是我方自有儲存，不受 AMEGO 180 天限制（K2∧K3）。
func (u *Query) FileAvailability(doc domain.Document) (available bool, reason string) {
	if doc.PdfObjectKey != nil && *doc.PdfObjectKey != "" {
		return true, ""
	}
	return false, documentFileUnavailableReason(doc, u.now())
}

func documentFileUnavailableReason(doc domain.Document, now time.Time) string {
	switch {
	case doc.Status == codemeta.InvoiceDocumentStatusNotApplicable.Value:
		return FileUnavailableNotApplicable
	case doc.CarrierType != nil:
		return FileUnavailableCarrierNotWon
	case doc.ProviderIssuedAt != nil && now.Sub(*doc.ProviderIssuedAt) > providerFileRetention:
		return FileUnavailableExpired
	default:
		return FileUnavailableNotApplicable
	}
}
