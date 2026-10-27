package seller

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/kyc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// ErrFileNotSynced 表示機敏檔（證件、佐證）在 media 還讀不到，而且它的 media 引用同步還沒結算：稍後重試（ReadKYCDocument、
// ReadEvidence）。
var ErrFileNotSynced = model.ErrFileNotSynced

// ErrFileMissing 表示我們的資料庫有這份機敏檔、引用也同步過了，media 卻說沒有（ReadKYCDocument、ReadEvidence）：資料不一致，
// 要人處理。
var ErrFileMissing = model.ErrFileMissing

// KYC 認證（ADR 2026-10-19-seller-kyc-payuni-consolidated）的公開型別與方法。賣家的方法以帳號為身分，授權由模組做
// （申請以帳號目前的店家為準）；管理員的方法由呼叫方（admin 的 Connect 方法）先確認是管理員。

// KYCStatus 是申請的狀態：只記人工決定。
type KYCStatus = model.KYCStatus

// KYCSellerType 是賣家型態（個人或企業）。
type KYCSellerType = model.KYCSellerType

// KYCDocumentSlot 是證件與文件圖片的位置。
type KYCDocumentSlot = model.KYCDocumentSlot

// KYCDocument 是申請上的一張證件圖片。
type KYCDocument = model.KYCDocument

// KYCDocumentRef 是草稿裡一張證件圖片的指定（保留現有的或換上新上傳的）。
type KYCDocumentRef = model.KYCDocumentRef

// KYCApplication 是申請的遮蔽檢視：身分證號與銀行帳號只有遮蔽後的值。
type KYCApplication = model.KYCApplication

// KYCRejection 是最近一次退回的內容。
type KYCRejection = model.KYCRejection

// KYCIssue 是退回時指出的一個問題（欄位、文件類欄位指到的那一張、原因碼、給賣家看的說明）。
type KYCIssue = model.KYCIssue

// KYCIssueCode 是退回時一個問題的原因類別。
type KYCIssueCode = model.KYCIssueCode

// KYCAdminApplication 是後台的申請檢視（審核紀錄、顯示全文的稽核紀錄、銀行帳號的比對標記）。
type KYCAdminApplication = model.KYCAdminApplication

// KYCReview 是一筆審核紀錄（不含欄位值）。
type KYCReview = model.KYCReview

// ReviewAction 是審核紀錄的動作。
type ReviewAction = model.ReviewAction

// KYCReveal 是一筆顯示全文的稽核紀錄（不含值）。
type KYCReveal = model.KYCReveal

// KYCAuditCursor 是稽核紀錄的 keyset 位置。
type KYCAuditCursor = model.KYCAuditCursor

// KYCAuditListInput 是稽核紀錄的分頁輸入。
type KYCAuditListInput = model.KYCAuditListInput

// KYCRevealPage 是一頁顯示全文的紀錄。
type KYCRevealPage = model.KYCRevealPage

// KYCDocumentAccessPage 是一頁證件讀取紀錄。
type KYCDocumentAccessPage = model.KYCDocumentAccessPage

// KYCDocumentAccess 是一筆證件圖片的讀取紀錄（不含圖片的 ID 與內容）。
type KYCDocumentAccess = model.KYCDocumentAccess

// KYCDocumentLocator 是證件圖片在申請裡的位置。
type KYCDocumentLocator = model.KYCDocumentLocator

// ReadKYCDocumentInput 是讀一張證件圖片的輸入。
type ReadKYCDocumentInput = model.ReadKYCDocumentInput

// RevealGrantTTL 是顯示全文的授權的有效時間。
const RevealGrantTTL = model.RevealGrantTTL

// KYCSensitive 是顯示全文的結果。
type KYCSensitive = model.KYCSensitive

// KYCDraftInput 是草稿的部分更新。
type KYCDraftInput = model.KYCDraftInput

// SubmitKYCInput 是送出申請的輸入。
type SubmitKYCInput = model.SubmitKYCInput

// CancelKYCInput 是取消申請的輸入。
type CancelKYCInput = model.CancelKYCInput

// Registry 是登記資料來源的 port（真正的實作是 platform/gcis 的 Client；回來源的原始資料，轉成模組的型別只有 model.NewRegistration
// 一處）：公司型申請送件時查詢並存快照，也給互動的統編查詢（LookupBusinessRegistration）。
type Registry = kyc.Registry

// Registration 是查詢取得的登記資料，RegistrationCheck 是一次查詢的結果與比對（後台檢視的 RegistrationCheck）。
type (
	Registration                = model.Registration
	RegistrationCheck           = model.RegistrationCheck
	RegistrationOutcome         = model.RegistrationOutcome
	RegistrationStatus          = model.RegistrationStatus
	RequeryKYCRegistrationInput = model.RequeryKYCRegistrationInput
)

// 登記資料查詢的結果（RegistrationCheck.Outcome）。
const (
	RegistrationPending     = model.RegistrationPending
	RegistrationFound       = model.RegistrationFound
	RegistrationNotFound    = model.RegistrationNotFound
	RegistrationUnavailable = model.RegistrationUnavailable
	RegistrationCancelled   = model.RegistrationCancelled
)

// 登記現況的分類（Registration.Status）：來源的現況文字不在對照表內時是 RegistrationStatusUnknown，原文在 StatusText。
const (
	RegistrationStatusActive            = model.RegistrationStatusActive
	RegistrationStatusOrderedToDissolve = model.RegistrationStatusOrderedToDissolve
	RegistrationStatusTerminated        = model.RegistrationStatusTerminated
	RegistrationStatusUnknown           = model.RegistrationStatusUnknown
)

// RegistrationOutcomeValues 與 RegistrationStatusValues 回傳登記資料查詢的全部結果與現況分類（與 DB 的 CHECK 一致）。
func RegistrationOutcomeValues() []RegistrationOutcome { return model.RegistrationOutcomeValues() }

// RegistrationStatusValues 見 RegistrationOutcomeValues。
func RegistrationStatusValues() []RegistrationStatus { return model.RegistrationStatusValues() }

// ApproveKYCInput 是核准的輸入。
type ApproveKYCInput = model.ApproveKYCInput

// RejectKYCInput 是退回的輸入。
type RejectKYCInput = model.RejectKYCInput

// RevealKYCInput 是顯示全文的輸入。
type RevealKYCInput = model.RevealKYCInput

// KYCCursor 是 admin 清單的 keyset 位置。
type KYCCursor = model.KYCCursor

// KYCListInput 是 admin 清單的輸入。
type KYCListInput = model.KYCListInput

// KYCPage 是一頁申請。
type KYCPage = model.KYCPage

const (
	KYCStatusDraft      = model.KYCStatusDraft
	KYCStatusSubmitted  = model.KYCStatusSubmitted
	KYCStatusApproved   = model.KYCStatusApproved
	KYCStatusRejected   = model.KYCStatusRejected
	KYCStatusCancelled  = model.KYCStatusCancelled
	KYCIndividual       = model.KYCIndividual
	KYCCompany          = model.KYCCompany
	SlotIDCardFront     = model.SlotIDCardFront
	SlotIDCardBack      = model.SlotIDCardBack
	SlotBankBook        = model.SlotBankBook
	SlotCompanyDocument = model.SlotCompanyDocument

	IssueUnreadable = model.IssueUnreadable
	IssueMismatch   = model.IssueMismatch
	IssueExpired    = model.IssueExpired
	IssueMissing    = model.IssueMissing
	IssueInvalid    = model.IssueInvalid
	IssueOther      = model.IssueOther

	ReviewSubmit  = model.ReviewSubmit
	ReviewApprove = model.ReviewApprove
	ReviewReject  = model.ReviewReject
	ReviewCancel  = model.ReviewCancel
	ReviewRebind  = model.ReviewRebind
)

// KYCIssueCodeValues 回傳全部退回問題的原因碼。
func KYCIssueCodeValues() []KYCIssueCode { return model.KYCIssueCodeValues() }

// ReviewActionValues 回傳全部審核動作。
func ReviewActionValues() []ReviewAction { return model.ReviewActionValues() }

// NormalizeTaxID 去掉空白並檢查統編（8 碼數字加 2023 年起的檢查碼）。統編的規則只有這一處：申請草稿的欄位與登記資料查詢
// （httpapi）都用它。
func NormalizeTaxID(raw string) (string, bool) { return model.NormalizeTaxID(raw) }

// KYCStatusValues 回傳全部申請狀態。
func KYCStatusValues() []KYCStatus { return model.KYCStatusValues() }

// KYCSellerTypeValues 回傳全部賣家型態。
func KYCSellerTypeValues() []KYCSellerType { return model.KYCSellerTypeValues() }

// KYCDocumentSlotValues 回傳全部證件位置。
func KYCDocumentSlotValues() []KYCDocumentSlot { return model.KYCDocumentSlotValues() }

// KYCDocumentFieldNames 回傳文件類的欄位路徑（退回的問題可以指到某一張文件的欄位）。
func KYCDocumentFieldNames() []string { return model.KYCDocumentFieldNames() }

// KYCFieldNames 回傳申請上可以指出「有問題」的欄位路徑（與對外 KYCApplication 的欄位路徑一對一）：退回的問題、PayUni 問題、缺項共用。
func KYCFieldNames() []string { return model.KYCFieldNames() }

// KYCSlotField 回傳證件位置所屬的欄位路徑（後台契約用欄位表示文件）；不認得的位置回 false。
func KYCSlotField(slot KYCDocumentSlot) (string, bool) { return model.KYCSlotField(slot) }

// IssuableFields 回傳申請可以在退回時指出問題的欄位路徑（RejectKYC 的驗證與後台退回表單共用的同一份規則）。
func IssuableFields(t KYCSellerType) []string { return model.IssuableFields(t) }

// StoreFieldNames 回傳店家上可以指出「有問題」的欄位路徑（與對外 Store 的欄位一對一）：送出前缺的店家欄位與 PayUni 問題共用。
func StoreFieldNames() []string { return model.StoreFieldNames() }

// SaveKYCDraft 部分更新帳號目前店家的 KYC 草稿（還沒有申請時建立，ExpectedVersion 為 0；被退回的申請在這裡回到 DRAFT）。
// 需要 CapStorefrontWrite；必須在 transaction 外呼叫（證件圖片依 media 的順序轉正）。版本不符回 VersionConflict。
func (m *Module) SaveKYCDraft(ctx context.Context, in KYCDraftInput) (KYCApplication, error) {
	return m.kyc.SaveDraft(ctx, in)
}

// SubmitKYC 送出草稿：必填欄位與證件齊全、身分沒有命中別的帳號生效中的處分。
func (m *Module) SubmitKYC(ctx context.Context, in SubmitKYCInput) (KYCApplication, error) {
	return m.kyc.Submit(ctx, in)
}

// CancelKYC 取消草稿或審核中的申請。
func (m *Module) CancelKYC(ctx context.Context, in CancelKYCInput) (KYCApplication, error) {
	return m.kyc.Cancel(ctx, in)
}

// MyKYC 回傳帳號目前店家的申請（遮蔽）；沒有申請時回 NotFound。
func (m *Module) MyKYC(ctx context.Context, accountID int64) (KYCApplication, error) {
	return m.kyc.Mine(ctx, accountID)
}

// AdminKYCApplications 是 admin 的申請清單（不含草稿；狀態與姓名 ILIKE 篩選；keyset）。
func (m *Module) AdminKYCApplications(ctx context.Context, in KYCListInput) (KYCPage, error) {
	return m.kyc.AdminList(ctx, in)
}

// StoreKYC 以店家讀它目前那一筆還沒取消的申請（遮蔽的檢視，帶流程狀態與證件）。草稿是賣家自己的，後台讀不到：沒有申請、
// 已取消或只有草稿時 ok 為 false；店家不存在回 NotFound。
func (m *Module) StoreKYC(ctx context.Context, storeID int64) (application KYCApplication, ok bool, err error) {
	return m.kyc.ForStore(ctx, storeID)
}

// CertHolderMatch 是業者名稱歸屬比對的結果。
type CertHolderMatch = model.CertHolderMatch

// MatchCertificateHolder 只為 legacy 的 SellerService.MatchCertificateHolder 存在（product-service 的有機與產銷履歷證書自動核准，
// 退場：product 搬進單體、直接呼叫本方法，主計畫 B4）：holderName 是政府證書查驗回的業者名稱，是否屬於店家目前已人工核准的 KYC 的
// 姓名或公司名稱（只回判定與相符的欄位，名字不出模組；不要求 PayUni 核准）。沒有基準（店家不存在、已關閉、沒有核准的申請、
// 名稱都是空的）時 HasReference 為 false；讀取失敗回錯誤。
func (m *Module) MatchCertificateHolder(ctx context.Context, storeID int64, holderName string) (CertHolderMatch, error) {
	return m.kyc.MatchCertHolder(ctx, storeID, holderName)
}

// AdminKYCApplication 讀一筆申請的後台檢視（遮蔽）。
func (m *Module) AdminKYCApplication(ctx context.Context, applicationID uuid.UUID) (KYCAdminApplication, error) {
	return m.kyc.AdminGet(ctx, applicationID)
}

// RevealKYCSensitive 回傳身分證號、負責人身分證號與銀行帳號的全文，同時寫入 append-only 的稽核紀錄（不含值）。
func (m *Module) RevealKYCSensitive(ctx context.Context, in RevealKYCInput) (KYCSensitive, error) {
	return m.kyc.RevealSensitive(ctx, in)
}

// ReadKYCDocument 讀一張證件圖片（管理員）：要有這位管理員對這筆申請未到期的授權（RevealKYCSensitive 建立，15 分鐘），否則
// PermissionDenied（不區分申請存在與否）；個資已清除或位置沒有圖片回 NotFound。在讀取圖片之前先寫一列 append-only 的讀取紀錄
// （寫不進去就不讀、不回傳）；圖片經 media 的機敏讀取取得。
func (m *Module) ReadKYCDocument(ctx context.Context, in ReadKYCDocumentInput) (FileContent, error) {
	return m.kyc.ReadDocument(ctx, in)
}

// ListKYCReveals 列出一筆申請的顯示全文紀錄（新到舊的 keyset）；申請不存在或是草稿回 NotFound。
func (m *Module) ListKYCReveals(ctx context.Context, in KYCAuditListInput) (KYCRevealPage, error) {
	return m.kyc.ListReveals(ctx, in)
}

// ListKYCDocumentAccesses 列出一筆申請的證件讀取紀錄（新到舊的 keyset）；申請不存在或是草稿回 NotFound。
func (m *Module) ListKYCDocumentAccesses(ctx context.Context, in KYCAuditListInput) (KYCDocumentAccessPage, error) {
	return m.kyc.ListDocumentAccesses(ctx, in)
}

// ApproveKYC 核准審核中的申請：在鎖內重新檢查（版本、狀態、申請人的帳號沒有停權、身分沒有命中別的帳號生效中的處分）。回傳
// 的後台檢視在同一個 transaction 內讀（含剛寫入的審核紀錄）。
func (m *Module) ApproveKYC(ctx context.Context, in ApproveKYCInput) (KYCAdminApplication, error) {
	return m.kyc.Approve(ctx, in)
}

// RequeryKYCRegistration 對審核中的公司型申請重新查詢登記資料（排入背景工作，不等結果）：這一輪還沒查完的查詢被取代，回傳新增的這筆
// 進行中的查詢（帶期限，伺服器保證在期限前到終態）。申請不是 SUBMITTED 或不是公司型回 FailedPrecondition，不存在或是草稿回 NotFound。
func (m *Module) RequeryKYCRegistration(ctx context.Context, in RequeryKYCRegistrationInput) (RegistrationCheck, error) {
	return m.kyc.RequeryRegistration(ctx, in)
}

// SweepKYCRegistryChecks 是 seller.kyc_registry_sweep job 的內容（給 worker 與測試呼叫）：把超過期限還在 PENDING 的登記資料查詢收成終態，
// 並替閒置的 PENDING 補排 job。回傳收掉的與補排的查詢數。
func (m *Module) SweepKYCRegistryChecks(ctx context.Context) (expired int64, resynced int, err error) {
	return m.kyc.SweepRegistryChecks(ctx)
}

// LookupBusinessRegistration 是互動的統編查詢（賣家填表時帶入公司資料）：呼叫登記資料來源並轉成模組的型別（現況分類、地址拆分與送件的
// 查詢快照同一個入口）。外部呼叫，不可在 transaction 內；查無是 found 為 false，來源沒有給出可用的答案回 Unavailable。統編的格式與
// 檢查碼由呼叫方先驗證（NormalizeTaxID），限流與能力檢查在 httpapi。
func (m *Module) LookupBusinessRegistration(ctx context.Context, taxID string) (Registration, bool, error) {
	return m.kyc.LookupRegistration(ctx, taxID)
}

// KYCRegistryCheckArgs 是 seller.kyc_registry_check job 的參數（一次登記資料查詢；沒有 River 唯一鍵，每列只排一次，冪等由「仍是 PENDING 才寫」保證）。
type KYCRegistryCheckArgs = kyc.RegistryCheckArgs

// RunKYCRegistryCheck 是 seller.kyc_registry_check job 的內容：認領查詢、transaction 外呼叫登記資料來源、再寫入結果與比對。
// 來源暫時不可用時回錯誤（River 重試）；finalAttempt 為 true（最後一次嘗試）時改為把查詢記為 UNAVAILABLE。給 worker 與測試呼叫。
func (m *Module) RunKYCRegistryCheck(ctx context.Context, checkID uuid.UUID, finalAttempt bool) error {
	return m.kyc.RunRegistryCheck(ctx, checkID, finalAttempt)
}

// RejectKYC 退回審核中的申請；回傳的後台檢視在同一個 transaction 內讀。
func (m *Module) RejectKYC(ctx context.Context, in RejectKYCInput) (KYCAdminApplication, error) {
	return m.kyc.Reject(ctx, in)
}

// RehashKYCIdentities 補算身分比對用的 hash：金鑰輪替（金鑰環新增金鑰）之後，既有的 KYC 申請還沒有新金鑰算的 blind index，關聯帳號的比對
// 才找得到輪替後寫入的快照；只解密缺列的申請，沒有輪替時什麼都不做。serve 與 worker 啟動時在 CheckKYCKeys 之後呼叫，可重複執行；回傳補算的申請數。
func (m *Module) RehashKYCIdentities(ctx context.Context) (int, error) {
	return m.kyc.RehashIdentities(ctx)
}

// CheckKYCKeys 是啟動檢查：資料庫裡被參照的每一把金鑰（KYC 密文開頭的金鑰 ID、blind index 與處分身分快照的金鑰 ID）
// 都要在設定的金鑰環裡，缺了就回錯誤（serve 與 worker 啟動時呼叫，失敗就不啟動）。否則金鑰被誤拿掉時，要到有人讀到那一列
// 才發現解不開、比對漏掉。
func (m *Module) CheckKYCKeys(ctx context.Context) error {
	return m.kyc.CheckKeys(ctx)
}
