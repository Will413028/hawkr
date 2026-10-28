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
	// adjustments 只為了一件事：ALLOWANCE_APPLIED 的判準**不在 status 欄上**
	// （折讓不改原件 status，只累加 allowance_total_amount）⇒ 沒有它，
	// 買家的 state 投影與文件時間軸上的「已折讓」永遠不會出現，而且不報錯（§6.1.3）。
	adjustments domain.AdjustmentRepository
	corrections domain.CorrectionRequestRepository
	// overview 服務端點 16／37 的計數。它獨立成介面是因為兩支各自橫跨 3–4 張表，
	// 歸到任何一張的 repository 都是錯的。
	overview domain.OverviewRepository
	// businessDay 只服務一件事：B07／S09／A07 的「剩餘 X 個工作天」。它是**唯讀**的
	// 鏡像（§5.1.14），與 Query 沒有 TxManager 這條規則不衝突。
	businessDay domain.BusinessDayRepository
	// cutovers 服務端點 46（自動開立切點歷史）。唯讀，寫入面在 ProviderAccount。
	cutovers domain.ProviderCutoverRepository
	// unlocks 服務端點 51 的 manualUnlock 卡。唯讀，寫入面在 AdminJob。
	unlocks domain.ManualUnlockRepository
	// cancelledOrders 只服務一件事：端點 8 在「這張訂單沒有任何 document」時，
	// 要不要說「付款完成後會立即開立發票」（#2904）。已取消的訂單不會再被付款，
	// 那句話對它永遠不成真，而本 service 的讀路徑看不到訂單狀態。
	//
	// 🔴 它是**建構子參數**而不是 setter，理由是失效方向：忘了 wire 的話，
	// setter 版本會留下一個 nil 欄位，而唯一安全的處置（nil 就跳過）正好等於
	// 「靜默退回舊的錯誤文案」——零錯誤、零 log、畫面看起來很正常。
	// 列進簽章讓那件事變成編譯錯。
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

// ---------------------------------------------------------------------------
// 稽核：ADM 揭露 P 類資料
// ---------------------------------------------------------------------------

const (
	// auditActionPIIReveal 是 §8.1.3(c) 裁定 2 指定的 action 字面值，對應告警
	// InvoicePiiRevealSpike（§8.2.4 ⑩）與指標 invoice_admin_pii_reveal_total（M13）。
	auditActionPIIReveal = "INVOICE_PII_REVEAL"

	auditResourceDocument  = "INVOICE_DOCUMENT"
	auditResourceComplaint = "INVOICE_COMPLAINT"

	// invoice_admin_audit_logs.reason_code／reason_detail 都是 NOT NULL，而讀取沒有
	// 使用者填的理由 ⇒ 用固定值標明它是「讀取造成的揭露」而不是某個 ADM 動作。
	auditReasonRead       = "READ"
	auditReasonDetailRead = "ADM 讀取含 P 類欄位的發票詳情（58 §8.1.3(c) 裁定 2）"
)

// recordPIIReveal 在 ADM 揭露 P 類資料前留一列稽核。
//
// 🔴 **fail-closed：稽核寫不進去就不揭露**。§8.1.3(c) 的裁定是「ADM 一律不遮罩，代價由
// 稽核承擔」——稽核寫失敗而照樣回明文，等於把裁定的代價那一半刪掉，而畫面上完全正常。
//
// 🔴 request_id 一律傳 nil：uq_invadmaudit_client_request(actor_account_id, action,
// request_id) WHERE request_id IS NOT NULL 會把「同一個 ADM、同一個 action、同一把
// request_id」摺成一列 ⇒ 帶了它，同一個 ADM 的第二次揭露就不留痕。correlation_id
// 沒有這個唯一索引，可以帶。
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
	// M13 在稽核列寫成功**之後**才遞增：fail-closed 的語意是「寫不進去就不揭露」，
	// 提前遞增會把沒有發生的揭露算進去。
	observeAdminPIIReveal()
	return nil
}

// ---------------------------------------------------------------------------
// 買家：B08 常用發票資料列表（端點 5）
// ---------------------------------------------------------------------------

// buyerProfileCap 是 9303 定的每位買家常用發票資料上限。
//
// 它同時是本查詢的 SQL LIMIT：ListBuyerInvoicePreferences 沒有 OFFSET，分頁在 Go 端切。
// 這在**這一支**是正確的而不是偷懶——存活列數由 9303 硬性封頂在 20，取 cap+1 就能同時
// 得到精確的 total 與「是否已越界」的訊號。
//
// 🔴 這個做法**不可複製**到 invoice_documents／invoice_operations 那些沒有上限的表：
// 那裡的 Go 端分頁會在資料長大之後靜默漏列（見交付回報的缺 query 清單）。
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

	// cap+1：拿到第 21 列代表存活列數已超過 9303 的上限，那是寫入路徑的不變式被破壞，
	// 不是分頁問題 ⇒ 記一筆讓它顯眼，回應仍以實際取到的列為準（不靜默截斷成 20）。
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
		// 越界頁回空 records ＋ 正確的 total，不是 404：前端翻到最後一頁之後刪掉一列
		// 就會落在這裡，回錯誤會讓畫面炸掉而不是回到第一頁。
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

// ---------------------------------------------------------------------------
// 申訴：B07（買家）與 A07/A08（ADM）
// ---------------------------------------------------------------------------

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
	// A08 的「賣家：{storeId}」（#3054 的稽查）：這一欄原本恆為空字串，於是管理員從
	// 列表（那一支有值）點進詳情，賣家那一段整個消失。
	// 取不到 issuer 時留 0 而不是回錯：申訴本身的資料完全健康，不該因為一個顯示欄位
	// 讓整頁打不開（與 ExpectedDocKind 同一個處置）。
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
	// 工作天倒數與 sla_due_at 是同一套曆制（domain.BusinessCalendar）：前端禁止自算
	// （§6.1.6 第 1 條），而它算不出來時**保持 nil**，由型別邊界回空字串。
	cal, err := loadBusinessCalendar(ctx, u.businessDay, minTime(now, c.SlaDueAt), maxTime(now, c.SlaDueAt))
	if err != nil {
		return ComplaintView{}, err
	}
	v := viewWith(c, events, cal, now)
	v.ExpectedDocKind = u.expectedDocKind(ctx, c.DocumentID, nil)
	return v, nil
}

// expectedDocKind 取被申訴文件的 document_kind（#3031）。
//
// memo 非 nil 時當快取用：一頁申訴常常指向同一份文件（同一張發票被重開申訴），
// 而 complaintViews 本來就是逐列查詢的形狀。
//
// 🔴 失敗回空字串、不往上拋：這是申訴卡上的一行顯示文字，讓整個佇列因為它打不開
// 是本末倒置（同 resolveStoreNames 的處置）。
func (u *Query) expectedDocKind(ctx context.Context, documentID int64, memo map[int64]string) string {
	// documents 可能是 nil（買家面的部分讀取視圖不注入它）⇒ 與 documentID==0 同樣回空字串。
	// 少了這道守衛的症狀是 panic 不是空欄位，而 panic 會讓整個端點 500。
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

// ---------------------------------------------------------------------------
// 賣家稅務資格（S02／S04）與 ADM 審核詳情（A03）
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// AMEGO 憑證帳號（S03／A04）
// ---------------------------------------------------------------------------

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
	// 沒有稅務資格列不是錯誤：pending_order_count 就不填（見上方欄位註解）。
	if profile, perr := u.profiles.GetByIssuer(ctx, issuer.ID); perr == nil {
		view.Profile, view.HasProfile = profile, true
	} else if !isBizCode(perr, bizcode.SellerTaxProfileNotFound) {
		return ProviderAccountView{}, perr
	}
	// #3031：last_request_id。取不到**不中止**——它是給客服追查用的識別欄，
	// 讓 S05 整頁因為它打不開是本末倒置。與 pending_order_count 同一個處置。
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

// providerAMEGO 是 invoice_provider_accounts.provider 的唯一值（Phase 1）。
const providerAMEGO = "AMEGO"

// sellerIssuer 由 store_id 解出賣家 issuer，並把「查無」統一成呼叫端指定的業務碼。
//
// notFoundCode 由呼叫端給，是因為同一個「這個 store 還沒有 issuer」在稅務資格頁要讀成
// 9320、在 provider 頁要讀成 9330 —— 每一頁內部一致即可，跨頁不必同碼。
func (u *Query) sellerIssuer(ctx context.Context, storeID int64, notFoundCode int32) (domain.Issuer, error) {
	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	if err == nil {
		return issuer, nil
	}
	// 只翻譯 GetByRef 對 pgx.ErrNoRows 產生的那一個碼。用 `errors.As(*DomainError)`
	// 當條件會把「連線斷了」也翻成「這個 store 還沒設定」，而那兩件事的正確處置相反
	// （重試 vs 不重試）——同一個判準見 isBizCode 的註解。
	if isBizCode(err, bizcode.InvoiceProviderAccountNotFound) {
		return domain.Issuer{}, pkgerrors.NewBusiness(notFoundCode, bizcode.Message(notFoundCode))
	}
	return domain.Issuer{}, err
}

// ---------------------------------------------------------------------------
// ADM：發票文件詳情（端點 48）與開票作業詳情（端點 51）
// ---------------------------------------------------------------------------

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

// adminAuditLogLimit 是詳情頁一次載入的 ADM 稽核列數上限。
// ListInvoiceAdminAuditLogsByResource 依 occurred_at DESC ⇒ 取最近 N 筆。
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
	// issuer 只為了填 store_id（InvoiceJobInfo.store_id／賣家名稱）：invoice_documents
	// 只有 issuer_id，而「issuer_ref_id ＝ store_id」那一步在 invoice_issuers 上。
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

	// 沒有載具也沒有 email 的文件（打統編／捐贈）不需要解密：WithDecryptedRecipient
	// 會走 KeyRing，對一列全 NULL 的密文欄呼叫它只是白花一次 DB 往返。
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

// ---------------------------------------------------------------------------
// 由狀態推導的旗標與參數（給 gRPC mapper 用；判準的 SSOT 一律是 SQL 的 allowed-from）
// ---------------------------------------------------------------------------

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
		BackoffMultiplier:      2, // submit.go backoffFor：d *= 2
		MaxIntervalSeconds:     int64(u.cfg.MaxBackoff / time.Second),
		MaxAttempts:            u.cfg.MaxAttempts,
	}
}

// 可取消的來源狀態集合。
//
// 🔴 SSOT 是 db/queries/invoice_operations.sql 的 CAS 述詞，不是「看起來像失敗的狀態」：
//   - RequestInvoiceOperationCancel ← status IN (READY, RETRY_SCHEDULED, FAILED_PERMANENT,
//     FAILED_RETRY_EXHAUSTED, SELLER_ACTION_REQUIRED)
//
// 這份清單放寬了，按鈕就會亮在一個 CAS 必定 rows=0 的狀態上——而 rows=0 是**靜默**的
// （§7.3.0 規則 4），ADM 按下去什麼事都不會發生也不會有錯誤訊息。
//
// ⚠️ 可**重試**的那一份不在這裡：它與寫端 applyRetry 共用 adminRetryKindOf
// （admin_job.go）。曾經在這裡另存一份 map，兩邊就漂移了（#3089 第 8 項）。
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

// providerFileRetention 是 AMEGO invoice_file／invoice_query 的查詢期限（§8.1.5 K2）。
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

// documentFileUnavailableReason 是「為什麼沒有檔案」的唯一判準。
//
// 抽成純函式而不是留在 FileAvailability 裡，是因為它有**第二個消費者**：
// DocumentFile（§6.1.4 的三條 /file/content）。兩份實作會漂移，而漂移的症狀是
// 「列表說可以下載、按下去說載具未中獎」——兩句話都是後端講的，使用者無從判斷。
//
// ⚠️ 呼叫端必須自己先判 pdf_object_key：已落地的自有儲存一律優先，
// 它不受 AMEGO 180 天限制（§8.1.5 K2∧K3）。
func documentFileUnavailableReason(doc domain.Document, now time.Time) string {
	switch {
	// 🔴 **NOT_APPLICABLE 要排在載具之前**（#3089 第 13 項）。
	//
	// 這格是「平台不會為這張訂單開票」——那張憑證從頭到尾不存在，
	// 談不上「載具中獎後才提供」。而載具是**買家在結帳時就選好的**，
	// 與平台後來決定不開票無關 ⇒ 排在後面的話，每一張帶載具的
	// NOT_APPLICABLE 文件都會回 CARRIER_NOT_WON。
	//
	// 買家看到的症狀（dev 實測 N=2）：對一張「平台不會開票」的單子，
	// 畫面寫「使用載具的發票在中獎後才會提供 PDF」＋一顆「查看發票」——
	// 兩句都在暗示「之後會有」，而正確答案是「不會有」。
	// #3026 修的是 RECEIPT 那一格、#3084 是 B2B 載具那一格，這是同族第三格。
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
