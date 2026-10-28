package seller

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// 商家名單（站外業者的 BD 名單，遷移計畫 A3b 步驟 7c 階段 4）的公開型別與方法。方法由呼叫方（admin 的 Connect 方法、ai-service
// 的 legacy 轉接、7d 的 vendor portal）先確認呼叫者的身分；操作者是管理員（AdminID）。比對、合併與唯一性的規則見
// `internal/prospect` 的套件說明、ADR `2026-10-04-prospect-contact-points-and-matching` 與 docs/modules/seller.md §15。

// ProspectStatus 是業者的狀態（進行中、封存或被合併、已轉成店家）。
type ProspectStatus = model.ProspectStatus

// VendorType 是業者的型態（個人或公司）。
type VendorType = model.VendorType

// OutreachStatus 是招商的進度。
type OutreachStatus = model.OutreachStatus

// ContactKind 是聯絡方式的類型。
type ContactKind = model.ContactKind

// ContactPoint 是業者的一個聯絡方式（原值與是否為該類型的主要聯絡方式）。
type ContactPoint = model.ContactPoint

// Prospect 是一個業者。
type Prospect = model.Prospect

// ProspectFields 是業者可以編輯的欄位。
type ProspectFields = model.ProspectFields

// CreateProspectInput 是建立業者的輸入。
type CreateProspectInput = model.CreateProspectInput

// UpdateProspectInput 是業者的部分更新。
type UpdateProspectInput = model.UpdateProspectInput

// ProspectPreviewInput 是建立之前的比對輸入。
type ProspectPreviewInput = model.ProspectPreviewInput

// ProspectMatchTier 是一次比對命中的信心層級。
type ProspectMatchTier = model.ProspectMatchTier

// ProspectMatch 是一筆比對結果。
type ProspectMatch = model.ProspectMatch

// ProspectCursor 是業者清單的 keyset 位置。
type ProspectCursor = model.ProspectCursor

// ProspectListInput 是業者清單的篩選與分頁輸入。
type ProspectListInput = model.ProspectListInput

// ProspectPage 是一頁業者。
type ProspectPage = model.ProspectPage

// DuplicateCursor 是疑似重複清單的 keyset 位置。
type DuplicateCursor = model.DuplicateCursor

// DuplicatePair 是一對疑似重複。
type DuplicatePair = model.DuplicatePair

// DuplicatePage 是一頁疑似重複。
type DuplicatePage = model.DuplicatePage

// TaxIDMatchCursor 是統編對帳清單的 keyset 位置。
type TaxIDMatchCursor = model.TaxIDMatchCursor

// TaxIDMatch 是統編對得上、但還沒連起來的業者與店家。
type TaxIDMatch = model.TaxIDMatch

// TaxIDMatchPage 是一頁統編對帳。
type TaxIDMatchPage = model.TaxIDMatchPage

// ProspectSearchInput 是採購候選的搜尋條件。
type ProspectSearchInput = model.ProspectSearchInput

// ProspectSearchHit 是一筆候選與它命中的條件。
type ProspectSearchHit = model.ProspectSearchHit

// ProspectContact 是業者的主要 email 與狀態。
type ProspectContact = model.ProspectContact

// ArchiveProspectInput 是封存的輸入。
type ArchiveProspectInput = model.ArchiveProspectInput

// RestoreProspectInput 是還原的輸入。
type RestoreProspectInput = model.RestoreProspectInput

// MarkProspectsDistinctInput 是標記一對候選不是同一家的輸入。
type MarkProspectsDistinctInput = model.MarkProspectsDistinctInput

// MergeProspectsInput 是合併的輸入。
type MergeProspectsInput = model.MergeProspectsInput

// ConvertProspectInput 是手動轉成店家的輸入。
type ConvertProspectInput = model.ConvertProspectInput

// ProspectImageUpload 是換 logo 或封面的上傳（暫存 token 與選填的 https 來源網址）。
type ProspectImageUpload = model.ProspectImageUpload

const (
	// ProspectStatusActive 可操作、可被採購搜尋。
	ProspectStatusActive = model.ProspectStatusActive
	// ProspectStatusArchived 封存，或被合併進另一列（不可還原）。
	ProspectStatusArchived = model.ProspectStatusArchived
	// ProspectStatusConverted 已轉成店家。
	ProspectStatusConverted = model.ProspectStatusConverted
	// VendorIndividual 個人。
	VendorIndividual = model.VendorIndividual
	// VendorBusiness 公司或商號。
	VendorBusiness = model.VendorBusiness
	// OutreachNotContacted 還沒聯絡。
	OutreachNotContacted = model.OutreachNotContacted
	// OutreachInvited 已邀請。
	OutreachInvited = model.OutreachInvited
	// OutreachReplied 對方已回覆。
	OutreachReplied = model.OutreachReplied
	// OutreachDeclined 對方婉拒。
	OutreachDeclined = model.OutreachDeclined
	// OutreachDoNotContact 不要再聯絡。
	OutreachDoNotContact = model.OutreachDoNotContact
	// OutreachConverted 已轉成店家（只由轉成店家寫入）。
	OutreachConverted = model.OutreachConverted
	// ContactEmail 電子郵件。
	ContactEmail = model.ContactEmail
	// ContactPhone 電話。
	ContactPhone = model.ContactPhone
	// ContactWebsite 網站。
	ContactWebsite = model.ContactWebsite
	// ContactSocial 社群帳號。
	ContactSocial = model.ContactSocial
	// MatchTaxID 統編相同。
	MatchTaxID = model.MatchTaxID
	// MatchContact 聯絡方式的比對鍵相同。
	MatchContact = model.MatchContact
	// MatchName 名稱相同（只供人判斷）。
	MatchName = model.MatchName
	// ReasonTaxID 統編相同。
	ReasonTaxID = model.ReasonTaxID
	// ReasonName 名稱相同。
	ReasonName = model.ReasonName
	// ReasonEmail 電子郵件相同。
	ReasonEmail = model.ReasonEmail
	// ReasonPhone 電話相同。
	ReasonPhone = model.ReasonPhone
	// ReasonDomain 網站網域相同。
	ReasonDomain = model.ReasonDomain
	// ReasonSocial 社群帳號相同。
	ReasonSocial = model.ReasonSocial
)

// MatchReason 是配對命中的原因。
type MatchReason = model.MatchReason

// MatchReasonValues 回傳全部命中原因。
func MatchReasonValues() []MatchReason { return model.MatchReasonValues() }

// ProspectStatusValues 回傳全部業者狀態。
func ProspectStatusValues() []ProspectStatus { return model.ProspectStatusValues() }

// VendorTypeValues 回傳全部業者型態。
func VendorTypeValues() []VendorType { return model.VendorTypeValues() }

// OutreachStatusValues 回傳全部招商進度。
func OutreachStatusValues() []OutreachStatus { return model.OutreachStatusValues() }

// ContactKindValues 回傳全部聯絡方式類型。
func ContactKindValues() []ContactKind { return model.ContactKindValues() }

// CreateProspect 建立業者；統編已被別的未封存業者持有時回 AlreadyExists（reason PROSPECT_TAX_ID_HELD，metadata `prospectIds` 是持有者的 ID）；聯絡方式的比對鍵與其他未封存
// 業者相同、而且沒有標記為不是同一家時回 FailedPrecondition（reason PROSPECT_MATCH_FOUND，metadata `prospectIds` 是相符的業者），`ConfirmedDistinct` 涵蓋全部相符的業者時才建立並記下配對。
func (m *Module) CreateProspect(ctx context.Context, in CreateProspectInput) (Prospect, error) {
	return m.prospect.Create(ctx, in)
}

// PreviewProspect 建立之前的比對：回傳可能是同一家的業者（統編相同的會讓建立被拒絕、聯絡方式的比對鍵相同的需要確認不是同一家；
// 名稱相同只供人判斷）。
func (m *Module) PreviewProspect(ctx context.Context, in ProspectPreviewInput) ([]ProspectMatch, error) {
	return m.prospect.Preview(ctx, in)
}

// Prospect 讀一個業者（任何狀態），帶聯絡方式與分類；沒有時回 NotFound。
func (m *Module) Prospect(ctx context.Context, id int64) (Prospect, error) {
	return m.prospect.Get(ctx, id)
}

// Prospects 是 admin 的業者清單（預設只看進行中，新到舊的 keyset）。
func (m *Module) Prospects(ctx context.Context, in ProspectListInput) (ProspectPage, error) {
	return m.prospect.List(ctx, in)
}

// UpdateProspect 部分更新（需要 ExpectedVersion，不符回 VersionConflict；只有進行中的業者可以改）；沒有任何改變時回傳目前的
// 業者、不寫入。`Contacts` 非 nil 時整批替換聯絡方式（比對同 CreateProspect；統編被別的未封存業者持有回 FailedPrecondition，reason PROSPECT_TAX_ID_HELD）；
// `Logo`、`Cover` 非 nil 時換圖，必須在 transaction 外呼叫（圖片依 media 的順序轉正，被拒絕的請求不消耗 token）。
func (m *Module) UpdateProspect(ctx context.Context, in UpdateProspectInput) (Prospect, error) {
	return m.prospect.Update(ctx, in)
}

// ArchiveProspect 封存業者（需要 ExpectedVersion，不符回 VersionConflict；已封存而且版本相符的重送回傳目前的業者；已轉成店家的回
// FailedPrecondition）。
func (m *Module) ArchiveProspect(ctx context.Context, in ArchiveProspectInput) (Prospect, error) {
	return m.prospect.Archive(ctx, in)
}

// RestoreProspect 還原封存的業者（不帶版本）；被合併的不可還原，封存期間統編被別人拿走時回 FailedPrecondition，聯絡方式的比對鍵
// 被其他未封存業者佔走、而且沒有標記為不是同一家時回 FailedPrecondition（reason PROSPECT_MATCH_FOUND，metadata `prospectIds` 是相符的業者）（`ConfirmedDistinct` 涵蓋全部相符的業者時才還原）。
func (m *Module) RestoreProspect(ctx context.Context, in RestoreProspectInput) (Prospect, error) {
	return m.prospect.Restore(ctx, in)
}

// MarkProspectsDistinct 標記一對進行中的業者不是同一家（疑似重複清單的處理方式之一，另一個是合併）：不帶版本，已標記的重送回傳成功，
// 兩個 ID 相同回 InvalidInput，不存在回 NotFound，不是進行中的回 FailedPrecondition。
func (m *Module) MarkProspectsDistinct(ctx context.Context, in MarkProspectsDistinctInput) error {
	return m.prospect.MarkDistinct(ctx, in)
}

// MergeProspects 把輸家併進贏家（兩邊都需要版本，不符回 VersionConflict；聯絡方式併入贏家、「不是同一家」配對與分類改掛贏家，
// 統編在贏家沒有時搬過去，輸家封存並釋放圖片引用），回傳贏家。
func (m *Module) MergeProspects(ctx context.Context, in MergeProspectsInput) (Prospect, error) {
	return m.prospect.Merge(ctx, in)
}

// ConvertProspect 把業者手動轉成目前的店家（store_id，需要 ExpectedVersion，不符回 VersionConflict）；轉過的回
// FailedPrecondition，店家被別的業者認領回 AlreadyExists。
func (m *Module) ConvertProspect(ctx context.Context, in ConvertProspectInput) (Prospect, error) {
	return m.prospect.Convert(ctx, in)
}

// DuplicateProspects 是疑似重複的清單：尚未處理的候選配對（進行中的業者之間比對鍵相同或名稱相同，排除已標記為不是同一家的），keyset。
func (m *Module) DuplicateProspects(ctx context.Context, pageSize int32, after *DuplicateCursor) (DuplicatePage, error) {
	return m.prospect.Duplicates(ctx, pageSize, after)
}

// ProspectTaxIDMatches 是統編對帳：業者的統編對得上目前店家的 KYC 申請（以 store_id 為鍵）、但還沒有連起來（已被其他業者認領的店家
// 不列），keyset。
func (m *Module) ProspectTaxIDMatches(ctx context.Context, pageSize int32, after *TaxIDMatchCursor) (TaxIDMatchPage, error) {
	return m.prospect.TaxIDMatches(ctx, pageSize, after)
}

// SearchProspects 是採購候選的搜尋（給 ai-service）：只看進行中的業者，填了的條件是加權 OR，結果不排序（算分是呼叫端的事）；每筆帶
// 主要的網站（`Website`）。
func (m *Module) SearchProspects(ctx context.Context, in ProspectSearchInput) ([]ProspectSearchHit, error) {
	return m.prospect.Search(ctx, in)
}

// ProspectContact 回傳業者的主要 email 與狀態（vendor portal 邀請供應商用，任何狀態）；沒有 email 時 Email 是 nil。在呼叫方
// 的 transaction 內呼叫時，那一列帶共享鎖，與封存、合併序列化：讀到 ProspectStatusActive 之後，到呼叫方提交之前業者不會被封存。
func (m *Module) ProspectContact(ctx context.Context, id int64) (ProspectContact, error) {
	return m.prospect.Contact(ctx, id)
}
