// Package model 定義店家的型別、儲存的代碼、事件與輸入規則。
package model

import (
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Status 是 stint 的生命週期。停權是另一個維度（Store.Suspended），不在這裡。
type Status string

const (
	// Active 營業中的 stint。
	Active Status = "ACTIVE"
	// PendingDeletion 隨帳號刪除等待未結訂單結束的 stint；仍是帳號目前的店家。
	PendingDeletion Status = "PENDING_DELETION"
	// Closed 已關閉的歷史 stint。
	Closed Status = "CLOSED"
)

// StatusValues 回傳全部儲存的生命週期狀態，與 DB 的 stores_status_check 一致。
func StatusValues() []Status { return []Status{Active, PendingDeletion, Closed} }

// Profile 是店家可以編輯的欄位。沒有值的欄位是 nil，不存空字串。
type Profile struct {
	StoreName                                string
	Description, ContactPhone                *string
	ContactName, ContactMobile, SupportEmail *string
	TownCode, AddressDetail                  *string
	Prize, WebsiteURL                        *string
	OnboardingCompleted                      bool
}

// Sanctions 是帳號在讀取當下生效中的停權措施（新到舊），是停權狀態唯一的來源：有任何一筆就是停權。nil（沒有經過推導的
// Store）視為停權（fail-closed），只有推導出沒有措施（NoSanctions）才放行。
type Sanctions struct {
	Measures []EffectiveMeasure
}

// NoSanctions 是推導的結果：帳號沒有生效中的停權措施。
func NoSanctions() *Sanctions { return &Sanctions{Measures: []EffectiveMeasure{}} }

// SanctionedBy 是推導的結果：帳號有這些生效中的停權措施（至少一筆才是停權；沒有措施等於 NoSanctions）。
func SanctionedBy(measures ...EffectiveMeasure) *Sanctions {
	return &Sanctions{Measures: append([]EffectiveMeasure{}, measures...)}
}

// Suspended 回報帳號是否停權：沒有推導過（nil）也算停權。
func (s *Sanctions) Suspended() bool { return s == nil || len(s.Measures) > 0 }

// Terminated 回報帳號生效中的措施有沒有終止合作（沒有推導過的 nil 不算）：解除停權不會讓這樣的帳號回到可以開店，要撤銷終止。
func (s *Sanctions) Terminated() bool {
	if s == nil {
		return false
	}
	for _, m := range s.Measures {
		if m.Kind == MeasureTermination {
			return true
		}
	}
	return false
}

// OpeningState 是判斷帳號此刻能不能申請開店的全部輸入：帳號有沒有目前的店家、帳號是不是存活的（不在等待刪除）、帳號生效中的措施。
type OpeningState struct {
	HasStore      bool
	AccountActive bool
	Sanctions     *Sanctions
}

// OpeningBlocker 是帳號不能申請開店的一個原因。
type OpeningBlocker string

const (
	// OpeningTerminated 帳號有生效中的終止合作（條款第 21.3 條）：終止撤銷之前，解除停權也開不了店。
	OpeningTerminated OpeningBlocker = "TERMINATED"
	// OpeningSuspended 帳號停權中（沒有推導過的 nil 也算，fail-closed）。
	OpeningSuspended OpeningBlocker = "SUSPENDED"
	// OpeningAccountDeleting 帳號正在刪除（等待未結訂單結束），不能再授予角色。
	OpeningAccountDeleting OpeningBlocker = "ACCOUNT_DELETING"
	// OpeningStoreExists 帳號已經有目前的店家。
	OpeningStoreExists OpeningBlocker = "STORE_EXISTS"
)

// OpeningBlockerValues 回傳全部原因，依優先序（結構性的在前）。
func OpeningBlockerValues() []OpeningBlocker {
	return []OpeningBlocker{OpeningTerminated, OpeningSuspended, OpeningAccountDeleting, OpeningStoreExists}
}

// OpeningBlockers 回答帳號此刻不能申請開店的全部原因，依 OpeningBlockerValues 的優先序；空表示可以申請。ApplyStore 的前置條件與開店入口的可用性
// （GetMyStoreOpening）都只看這一份。停權的判斷只算非終止的措施：終止合作本身已是更重的原因，不再同時標成停權。
func OpeningBlockers(st OpeningState) []OpeningBlocker {
	var out []OpeningBlocker
	if st.Sanctions.Terminated() {
		out = append(out, OpeningTerminated)
	}
	if st.Sanctions == nil || slices.ContainsFunc(st.Sanctions.Measures, func(m EffectiveMeasure) bool { return m.Kind != MeasureTermination }) {
		out = append(out, OpeningSuspended)
	}
	if !st.AccountActive {
		out = append(out, OpeningAccountDeleting)
	}
	if st.HasStore {
		out = append(out, OpeningStoreExists)
	}
	return out
}

// Text 是原因給使用者看的一句話（語系 l）；開店入口的可用性訊息與 ApplyStore 被擋下時錯誤的訊息都由它產生。不認得的原因回零值（Text 是空字串）。
func (b OpeningBlocker) Text(l uitext.Locale) uitext.Message {
	switch b {
	case OpeningTerminated:
		return text.OpeningUnavailableTerminated(l)
	case OpeningSuspended:
		return text.OpeningUnavailableSuspended(l)
	case OpeningAccountDeleting:
		return text.OpeningUnavailableAccountDeleting(l)
	case OpeningStoreExists:
		return text.OpeningUnavailableStoreExists(l)
	}
	return uitext.Message{}
}

// Err 是 ApplyStore 被這個原因擋下時回的錯誤（TERMINATED 與 SUSPENDED 帶 ErrorReason），訊息是 Text(l)。
func (b OpeningBlocker) Err(l uitext.Locale) error {
	switch b {
	case OpeningTerminated:
		return apperr.FailedPrecondition("%s", b.Text(l).Text).WithReason(apperr.ReasonStoreOpeningTerminated)
	case OpeningSuspended:
		return apperr.FailedPrecondition("%s", b.Text(l).Text).WithReason(apperr.ReasonStoreOpeningSuspended)
	case OpeningAccountDeleting:
		return apperr.FailedPrecondition("%s", b.Text(l).Text)
	case OpeningStoreExists:
		return apperr.AlreadyExists("%s", b.Text(l).Text)
	default:
		return fmt.Errorf("seller：不認得的開店原因 %q", string(b))
	}
}

// OpeningUnavailabilities 把原因換成帶句子的清單（語系 l），順序不變。
func OpeningUnavailabilities(l uitext.Locale, blockers []OpeningBlocker) []OpeningUnavailability {
	var out []OpeningUnavailability
	for _, b := range blockers {
		out = append(out, OpeningUnavailability{Blocker: b, Message: b.Text(l)})
	}
	return out
}

// StoreOpening 是帳號此刻能不能申請開店：CanApply 為 true 時 Unavailable 是空的，否則依優先序列出原因與句子。
type StoreOpening struct {
	CanApply    bool
	Unavailable []OpeningUnavailability
}

// OpeningBlock 是 ApplyStore 的前置條件：第一個原因的錯誤（語系 l），沒有原因回 nil。
func OpeningBlock(l uitext.Locale, st OpeningState) error {
	if blockers := OpeningBlockers(st); len(blockers) > 0 {
		return blockers[0].Err(l)
	}
	return nil
}

// IDs 是生效中措施的 ID。
func (s *Sanctions) IDs() []uuid.UUID {
	if s == nil {
		return []uuid.UUID{}
	}
	ids := make([]uuid.UUID, len(s.Measures))
	for i, m := range s.Measures {
		ids[i] = m.MeasureID
	}
	return ids
}

// Conditions 是店家與時間無關的條件此刻是否滿足（曝光與能力共用，一欄對一個 Requirement）。它不在 Go 推導：讀店家時由
// seller.store_requirement_flags（db/objects.sql，曝光規則唯一的家）讀出（stint.WithConditions），Go 只把布林對應到 Requirement；
// 與時間有關的停權由 Sanctions 表示（同樣讀 SQL 的 seller.effective_measures）。Derived 為 false（沒有讀過）時每一項都不滿足
// （fail-closed，比照 Sanctions 的 nil）。
type Conditions struct {
	Derived            bool
	Active             bool
	NotPendingDeletion bool
	KYCApproved        bool
	StoreName          bool
	Logo               bool
	TownCode           bool
}

// Store 是一個 stint。Sanctions 是讀取當下由生效中的措施推導的帳號停權狀態（帳號任何一個 stint 的措施都算），不是 stint 的
// 欄位；用 Suspended 判斷。Conditions 是讀取當下由 SQL 讀出的其他條件；能力與曝光只看這兩者（不看 Status、店名等欄位）。
type Store struct {
	Profile
	StoreID, StoreCode, AccountID int64
	Version                       int64
	Status                        Status
	Sanctions                     *Sanctions
	Conditions                    Conditions
	LogoKey, CoverKey             *string
	CreatedAt, UpdatedAt          time.Time
	// Closure 是 stint 目前這一次的關店紀錄（何時、為什麼）；已關閉的 stint 讀取時一定帶上（store_closures 目前的那一列），沒有關閉時是 nil。
	Closure *Closure
	// ApprovedFoodRegistration 是目前核准的食品業者登錄（公開字號的唯一來源），沒有時是 nil；OpenFoodRegistration 是賣家看得到的最近
	// 一筆未結束的登錄（待審或被退回）；RevokedFoodRegistration 是最近一次被平台撤銷的登錄，只在目前沒有核准、撤銷之後也沒有更新的待審或核准
	// （更新的核准即使之後又結束也算）時有值（賣家看到它就知道要重新送審）。三者只在讀取的路徑上載入（stint.Load、stint.AttachFood），ToStore 不帶。
	ApprovedFoodRegistration *FoodRegistration
	OpenFoodRegistration     *FoodRegistration
	RevokedFoodRegistration  *FoodRegistration
}

// Suspended 回報帳號是否停權：沒有推導過（Sanctions 是 nil）也算停權。
func (s Store) Suspended() bool { return s.Sanctions.Suspended() }

// EffectiveMeasures 是帳號此刻生效中的停權措施（沒有推導過時是空）。
func (s Store) EffectiveMeasures() []EffectiveMeasure {
	if s.Sanctions == nil {
		return nil
	}
	return s.Sanctions.Measures
}

// ClosedBy 是關閉 stint 的人。
type ClosedBy string

const (
	// ClosedByOwner 賣家自己關店。
	ClosedByOwner ClosedBy = "ACCOUNT_OWNER"
	// ClosedByTermination 終止合作生效且沒有未結訂單後由系統關店；記造成關店的終止合作措施，理由與作者在措施裡。
	ClosedByTermination ClosedBy = "TERMINATION"
	// ClosedByDeletion 隨帳號刪除關閉；申請者與理由在 accountdeletion 的申請紀錄。
	ClosedByDeletion ClosedBy = "ACCOUNT_DELETION"
)

// ClosedByValues 回傳全部關閉者，與 DB 的 store_closures_closed_by_check 一致。
func ClosedByValues() []ClosedBy {
	return []ClosedBy{ClosedByOwner, ClosedByTermination, ClosedByDeletion}
}

// Closure 是 stint 目前這一次關閉的紀錄（store_closures 目前的那一列）：ClosedAt 是關店時間；By 是 ClosedByTermination 時有 MeasureID（造成關店的終止合作措施），
// 其他沒有。恢復營業之後這一次的紀錄結束（歷史留在 store_closures），Store.Closure 回到 nil。
type Closure struct {
	ClosedAt  time.Time
	By        ClosedBy
	MeasureID *uuid.UUID
}

// StoreCursor 是後台店家清單的 keyset 位置（新到舊）。
type StoreCursor struct {
	CreatedAt time.Time
	StoreID   int64
}

// Valid 回報位置是不是完整的。
func (c StoreCursor) Valid() bool { return c.StoreID > 0 && !c.CreatedAt.IsZero() }

// StoreListInput 是後台店家清單的輸入。Statuses 為空是目前的店家（營運中與等待刪除），已關閉的要明列；Query 是店名或電話
// 的模糊搜尋，或完全相同的 store_code（空白不篩選）；KYCStatus 是目前那一筆還沒取消的 KYC 申請的狀態（草稿是賣家自己的，
// 不能篩選）；PageSize 為 0 用預設值。
type StoreListInput struct {
	Statuses []Status
	Query    string
	// KYCStatus 只接受 APPROVED、REJECTED（儲存的事實）；「待審核」用 KYCAwaitingReview。
	KYCStatus *KYCStatus
	// KYCAwaitingReview 為 true 只列商家認證申請在後台待辦裡的店家（seller.review_kyc_application 的成員：審核中、店家營運中、帳號沒有停權）。
	KYCAwaitingReview bool
	// AccountID 只列這個帳號的店家（完全相符；含已關閉的要在 Statuses 明列）。
	AccountID *int64
	// PayUniBindStatus 是 PayUni 註冊的綁定狀態（儲存的事實）。
	PayUniBindStatus *PayUniBindStatus
	// Suspended 為 true 只列帳號此刻停權中的店家，false 只列沒有停權的，nil 不篩選；「停權中」就是 stint.Derive 的那個定義。
	Suspended *bool
	PageSize  int32
	After     *StoreCursor
}

// StoreListItem 是清單的一列：店家（帶推導的停權狀態）與它的 KYC 流程狀態（沒有申請是 NOT_STARTED）。
type StoreListItem struct {
	Store   Store
	KYCFlow KYCFlowStatus
}

// StorePage 是一頁店家；Next 為 nil 表示沒有下一頁。
type StorePage struct {
	Items []StoreListItem
	Next  *StoreCursor
}

// StoreStats 是後台的店家統計：全部都只算目前的店家（營運中與等待刪除）。KYC 的家數依目前那一筆還沒取消的申請的狀態，
// 沒有申請與草稿的店家不在任何一格。
type StoreStats struct {
	// KYCAwaitingReview 是商家認證申請在後台待辦裡的店家數，與 StoreListInput.KYCAwaitingReview 同一個成員。
	Total, KYCAwaitingReview, KYCApproved, KYCRejected, NewThisMonth int32
}

// ApplyInput 是申請開店的輸入；空字串表示沒有提供。
type ApplyInput struct {
	AccountID                                   int64
	StoreName, Description, ContactPhone, Prize string
	// TownCode 是內政部的 8 碼鄉鎮市區代碼（platform/twaddr）。
	TownCode, AddressDetail string
	// AcceptedAgreements 是賣家同意的文件，剛好是 ApplyAgreements 的種類、版本是現行版本。ClientIP 與 UserAgent 是同意當下的
	// 來源，取不到是空字串。
	AcceptedAgreements  []AgreementAcceptance
	ClientIP, UserAgent string
}

// UpdateProfileInput 是賣家改店家資料的部分更新：nil 保留原值；非 nil 的空字串清成沒有值（StoreName 不能清）。
// ExpectedVersion 必填，是讀取時拿到的版本（ADR 2026-09-25-optimistic-concurrency-etag）。
type UpdateProfileInput struct {
	StoreID, ExpectedVersion  int64
	StoreName                 *string
	Description, ContactPhone *string
	// ContactName、ContactMobile、SupportEmail 是送給 PayUni 的聯絡資料。
	ContactName, ContactMobile, SupportEmail *string
	TownCode, AddressDetail                  *string
	Prize, WebsiteURL                        *string
	// LogoToken、CoverToken 是新上傳的圖的 staged token，空字串表示沿用現在的圖。UploaderID 是上傳暫存圖片的人
	// （換圖時必填）。
	LogoToken, CoverToken string
	UploaderID            int64
}

// StoreOpened 在開店的 transaction 內發出。
type StoreOpened struct {
	OccurredAt                    time.Time
	EventID                       uuid.UUID
	StoreID, StoreCode, AccountID int64
}

// StoreProfileChanged 在 profile、logo 或封面寫入的 transaction 內發出。
type StoreProfileChanged struct {
	OccurredAt time.Time
	EventID    uuid.UUID
	StoreID    int64
}

// StoreSuspended 在帳號從未停權變成停權的 transaction 內發出。PeriodID 是這段停權期間的身分，與結束時的
// StoreUnsuspended 相同。不帶措施的內容：持續停權中換措施（新增、結束、撤銷其中一個）不發事件，帶了會過期；需要措施的
// 消費者讀 ListMeasures。
type StoreSuspended struct {
	OccurredAt                   time.Time
	EventID                      uuid.UUID
	StoreID, AccountID, PeriodID int64
}

// StoreUnsuspended 在店家從停權恢復的 transaction 內發出；PeriodID 是結束的那段停權期間。
type StoreUnsuspended struct {
	OccurredAt                   time.Time
	EventID                      uuid.UUID
	StoreID, AccountID, PeriodID int64
}

// StoreExposureChanged 在店家的「最後公告的可曝光」改變的 transaction 內發出（ADR 2026-10-05-store-exposure-to-legacy
// 決策 1）：寫入造成可曝光翻轉時由那個寫入的 transaction 發出，時間造成的變化（處分到期）由對帳 job 補發。Exposable 是
// 公告的值（規則只有 seller.pub_exposable_stores，經 exposure.Classify），Version 是這家店的公告序號（從 1 開始、每次公告加一），消費者以它排序、
// 丟棄舊版本。
type StoreExposureChanged struct {
	OccurredAt time.Time
	EventID    uuid.UUID
	StoreID    int64
	Exposable  bool
	Version    int64
}

// StorePendingDeletion 在 stint 隨帳號刪除進入等待（ACTIVE → PENDING_DELETION）的 transaction 內發出：店家不再做經營類
// 的事（新訂單、上架），下游據此下架；只在真的轉移時發一次。
type StorePendingDeletion struct {
	OccurredAt                    time.Time
	EventID                       uuid.UUID
	StoreID, StoreCode, AccountID int64
}

// StoreClosed 在 stint 關閉的 transaction 內發出。關店可逆（終止合作的申覆期間內可以恢復），所以它不是下游刪除資料的訊號；不可逆的
// 下游動作由退場（第一段清理）時的 StoreRetired 觸發。
type StoreClosed struct {
	OccurredAt                    time.Time
	EventID                       uuid.UUID
	StoreID, StoreCode, AccountID int64
}

// StoreReopened 在已關閉的 stint 恢復營業（終止合作撤銷或申覆成立）的 transaction 內發出。
type StoreReopened struct {
	OccurredAt                    time.Time
	EventID                       uuid.UUID
	StoreID, StoreCode, AccountID int64
}

// StoreRetired 在 stint 退場（第一段清理）的 transaction 內發出（個資在第二段才清）：此後恢復營業不可能，logo 與封面已釋放，店家的所有下游資料可以刪除
// （relay 轉成 seller.deleted）。
type StoreRetired struct {
	OccurredAt                    time.Time
	EventID                       uuid.UUID
	StoreID, StoreCode, AccountID int64
}

// Error 把查無資料轉成 NotFound。
func Error(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到店家")
	}
	return err
}

// Positive 驗證每個 ID 都是正數。
func Positive(ids ...int64) error {
	for _, id := range ids {
		if id <= 0 {
			return apperr.InvalidInput("ID 必須為正數")
		}
	}
	return nil
}
