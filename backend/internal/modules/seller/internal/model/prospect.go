package model

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// 商家名單（站外業者的 BD 名單，遷移計畫 A3b 步驟 7c 階段 4）的型別與輸入規則。聯絡方式與比對鍵的規則見 ADR
// 2026-10-04-prospect-contact-points-and-matching；比對鍵的解析器在 prospect_match.go（舊 seller-service 的 P1，加上共用平台的排除）。

// ProspectStatus 是業者的狀態。
type ProspectStatus string

const (
	// ProspectStatusActive 可操作、可被採購搜尋。
	ProspectStatusActive ProspectStatus = "ACTIVE"
	// ProspectStatusArchived 封存，或被合併進另一列（MergedInto 有值，不可還原）。
	ProspectStatusArchived ProspectStatus = "ARCHIVED"
	// ProspectStatusConverted 已轉成店家，只供檢視。
	ProspectStatusConverted ProspectStatus = "CONVERTED"
)

// ProspectStatusValues 回傳全部狀態，與 DB 的 prospects_status_check 一致。
func ProspectStatusValues() []ProspectStatus {
	return []ProspectStatus{ProspectStatusActive, ProspectStatusArchived, ProspectStatusConverted}
}

// VendorType 是業者的型態。
type VendorType string

const (
	// VendorIndividual 個人。
	VendorIndividual VendorType = "INDIVIDUAL"
	// VendorBusiness 公司或商號。
	VendorBusiness VendorType = "BUSINESS"
)

// VendorTypeValues 回傳全部業者型態，與 DB 的 prospects_vendor_type_check 一致。
func VendorTypeValues() []VendorType { return []VendorType{VendorIndividual, VendorBusiness} }

// OutreachStatus 是招商的進度。CONVERTED 只由轉成店家寫入，管理員不能手動設定。
type OutreachStatus string

const (
	// OutreachNotContacted 還沒聯絡。
	OutreachNotContacted OutreachStatus = "NOT_CONTACTED"
	// OutreachInvited 已邀請。
	OutreachInvited OutreachStatus = "INVITED"
	// OutreachReplied 對方已回覆。
	OutreachReplied OutreachStatus = "REPLIED"
	// OutreachDeclined 對方婉拒。
	OutreachDeclined OutreachStatus = "DECLINED"
	// OutreachDoNotContact 不要再聯絡。
	OutreachDoNotContact OutreachStatus = "DO_NOT_CONTACT"
	// OutreachConverted 已轉成店家。
	OutreachConverted OutreachStatus = "CONVERTED"
)

// OutreachStatusValues 回傳全部招商進度，與 DB 的 prospects_outreach_check 一致。
func OutreachStatusValues() []OutreachStatus {
	return []OutreachStatus{OutreachNotContacted, OutreachInvited, OutreachReplied, OutreachDeclined, OutreachDoNotContact, OutreachConverted}
}

// ContactKind 是聯絡方式的類型。
type ContactKind string

const (
	// ContactEmail 電子郵件。
	ContactEmail ContactKind = "EMAIL"
	// ContactPhone 電話。
	ContactPhone ContactKind = "PHONE"
	// ContactWebsite 網站。
	ContactWebsite ContactKind = "WEBSITE"
	// ContactSocial 社群帳號。
	ContactSocial ContactKind = "SOCIAL"
)

// ContactKindValues 回傳全部聯絡方式類型，與 DB 的 prospect_contacts_kind_check 一致。
func ContactKindValues() []ContactKind {
	return []ContactKind{ContactEmail, ContactPhone, ContactWebsite, ContactSocial}
}

// ContactPoint 是業者的一個聯絡方式：Value 是管理員輸入的原值（顯示用，比對鍵由解析器另外推導）；某個類型有聯絡方式時，
// 恰好一個的 Primary 是 true（vendor portal 用主要的 email，ai-service 用主要的網站）。
type ContactPoint struct {
	Kind    ContactKind
	Value   string
	Primary bool
}

// 聯絡方式的上限：值的長度與 DB 的 prospect_contacts_value_check 一致；每個業者的總數是服務端的安全上限。
const maxContacts = 50

func contactValueMax(k ContactKind) int {
	switch k {
	case ContactEmail:
		return 255
	case ContactPhone:
		return 50
	case ContactWebsite, ContactSocial:
	}
	return 500
}

// NormalizeContacts 驗證並正規化整批聯絡方式：類型要合法、值去掉前後空白後不可為空且不超過長度上限、同類型同值的重複只留一列
// （任何一列標為主要就是主要）；某個類型有多列時必須恰好一列標為主要的，只有一列時它就是主要的。結果依類型順序、值排序。
// 沒有任何聯絡方式回傳空切片。
func NormalizeContacts(in []ContactPoint) ([]ContactPoint, error) {
	if len(in) > maxContacts {
		return nil, apperr.InvalidInputOn([]string{"Contacts"}, "聯絡方式最多 %d 個", maxContacts)
	}
	type key struct {
		kind  ContactKind
		value string
	}
	merged := make(map[key]int, len(in))
	out := make([]ContactPoint, 0, len(in))
	for i, c := range in {
		field := apperr.IndexedField("Contacts", i)
		if !slices.Contains(ContactKindValues(), c.Kind) {
			return nil, apperr.InvalidInputOn([]string{field + ".Kind"}, "聯絡方式的類型不合法")
		}
		c.Value = trimtext.Trim(c.Value)
		if n := utf8.RuneCountInString(c.Value); n == 0 || n > contactValueMax(c.Kind) {
			return nil, apperr.InvalidInputOn([]string{field + ".Value"}, "聯絡方式的值必須是 1 到 %d 字", contactValueMax(c.Kind))
		}
		if at, ok := merged[key{c.Kind, c.Value}]; ok {
			out[at].Primary = out[at].Primary || c.Primary
			continue
		}
		merged[key{c.Kind, c.Value}] = len(out)
		out = append(out, c)
	}
	byKind := map[ContactKind][]int{}
	for i, c := range out {
		byKind[c.Kind] = append(byKind[c.Kind], i)
	}
	for kind, at := range byKind {
		primaries := 0
		for _, i := range at {
			if out[i].Primary {
				primaries++
			}
		}
		switch {
		case len(at) == 1:
			out[at[0]].Primary = true
		case primaries != 1:
			return nil, apperr.InvalidInputOn([]string{"Contacts"}, "%s 有多個聯絡方式時必須恰好一個是主要的", kind)
		}
	}
	slices.SortFunc(out, compareContacts)
	return out, nil
}

// compareContacts 依類型順序、再依值排序。
func compareContacts(a, b ContactPoint) int {
	order := ContactKindValues()
	if c := slices.Index(order, a.Kind) - slices.Index(order, b.Kind); c != 0 {
		return c
	}
	return strings.Compare(a.Value, b.Value)
}

// SameContacts 回報兩批（已正規化的）聯絡方式是不是同一批：不看順序，看類型、值與主要的。
func SameContacts(a, b []ContactPoint) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.SortFunc(a, compareContacts)
	slices.SortFunc(b, compareContacts)
	return slices.Equal(a, b)
}

// ContactMatchKeys 回傳聯絡方式的比對鍵（去重、排序）；不具識別力的聯絡方式沒有鍵。
func ContactMatchKeys(contacts []ContactPoint) []string {
	var keys []string
	for _, c := range contacts {
		if k := ContactMatchKey(c.Kind, c.Value); k != "" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

// Prospect 是一個業者。
type Prospect struct {
	ID               int64
	Version          int64
	DisplayName      string
	VendorType       VendorType
	Status           ProspectStatus
	OutreachStatus   OutreachStatus
	TaxID            *string
	ConvertedStore   *int64
	ConvertedAt      *time.Time
	MergedInto       *int64
	Category         *string
	Region           *string
	ProductSummary   *string
	Positioning      *string
	ResearchSummary  *string
	PriorityLetter   *string
	PriorityStars    int32
	LogoKey          *string
	LogoSourceURL    *string
	CoverKey         *string
	CoverSourceURL   *string
	CreatedByAdminID *int64
	UpdatedByAdminID *int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	// Contacts 與 CategoryIDs 只在單筆讀取時帶出，清單與搜尋結果不帶。
	Contacts    []ContactPoint
	CategoryIDs []string
}

// ProspectFields 是業者可以編輯的欄位（建立與合併後的更新共用；聯絡方式與分類是清單，不在這裡）。空字串與 nil 都是沒有值。
type ProspectFields struct {
	DisplayName     string
	VendorType      VendorType
	OutreachStatus  OutreachStatus
	TaxID           *string
	Category        *string
	Region          *string
	ProductSummary  *string
	Positioning     *string
	ResearchSummary *string
	PriorityLetter  *string
	PriorityStars   int32
}

// CreateProspectInput 是建立業者的輸入；AdminID 是操作的管理員。OutreachStatus 為空是 NOT_CONTACTED。聯絡方式的比對鍵與其他未封存
// 業者（沒有標記為不是同一家的）相同時回 PROSPECT_MATCH_FOUND（NewProspectMatchFound）；ConfirmedDistinct 是管理員確認不是同一家的業者 ID，相符的業者
// 全部都在裡面時才建立，並在同一個 transaction 記下這些配對。
type CreateProspectInput struct {
	AdminID           int64
	Fields            ProspectFields
	Contacts          []ContactPoint
	CategoryIDs       []string
	ConfirmedDistinct []int64
}

// ProspectImageUpload 是換 logo 或封面的上傳：StagedToken 是 media 的暫存 token；SourceURL 是素材的來源頁面，只作稽核、後端不會去取，
// 選填、只收 https，沒有值是空字串。
type ProspectImageUpload struct {
	StagedToken string
	SourceURL   string
}

// UpdateProspectInput 是部分更新：nil 保留原值，空字串清成沒有值；CategoryIDs 與 Contacts 為 nil 保留、空切片清空（Contacts 是整批替換）。
// Logo、Cover 非 nil 時換圖，必須在 transaction 外呼叫。ExpectedVersion 是讀到的版本（etag），必填。聯絡方式改變時的比對與
// ConfirmedDistinct 同 CreateProspectInput。
type UpdateProspectInput struct {
	ProspectID        int64
	ExpectedVersion   int64
	AdminID           int64
	DisplayName       *string
	VendorType        *VendorType
	OutreachStatus    *OutreachStatus
	TaxID             *string
	Contacts          *[]ContactPoint
	Category          *string
	Region            *string
	ProductSummary    *string
	Positioning       *string
	ResearchSummary   *string
	PriorityLetter    *string
	PriorityStars     *int32
	CategoryIDs       *[]string
	Logo              *ProspectImageUpload
	Cover             *ProspectImageUpload
	ConfirmedDistinct []int64
}

// ProspectPreviewInput 是建立之前的比對輸入。
type ProspectPreviewInput struct {
	DisplayName string
	TaxID       string
	Contacts    []ContactPoint
}

// ProspectMatchTier 是一次比對命中的信心層級。T3（名稱相同）永不自動合併：合併不可逆，代價遠高於漏合併。
type ProspectMatchTier string

const (
	// MatchTaxID 統編相同（權威識別碼）。
	MatchTaxID ProspectMatchTier = "T1_TAX_ID"
	// MatchContact 聯絡方式的比對鍵（email、電話、網域、社群帳號）相同。
	MatchContact ProspectMatchTier = "T2_CONTACT"
	// MatchName 名稱相同，只供人判斷。
	MatchName ProspectMatchTier = "T3_NAME"
)

// MatchReason 是配對命中的原因：統編、名稱、或聯絡方式的比對鍵種類（值同 prospect_contacts.match_key 的前綴）。
type MatchReason string

const (
	// ReasonTaxID 統編相同。
	ReasonTaxID MatchReason = "TAX_ID"
	// ReasonName 名稱相同。
	ReasonName MatchReason = "NAME"
	// ReasonEmail 電子郵件相同。
	ReasonEmail MatchReason = "EMAIL"
	// ReasonPhone 電話相同。
	ReasonPhone MatchReason = "PHONE"
	// ReasonDomain 網站網域相同。
	ReasonDomain MatchReason = "DOMAIN"
	// ReasonSocial 社群帳號相同。
	ReasonSocial MatchReason = "SOCIAL"
)

// MatchReasonValues 回傳全部命中原因。
func MatchReasonValues() []MatchReason {
	return []MatchReason{ReasonTaxID, ReasonName, ReasonEmail, ReasonPhone, ReasonDomain, ReasonSocial}
}

// ProspectMatch 是一筆比對結果。MatchedOn 說明為什麼認為是同一家（統編、命中的鍵種類）；一個業者只出現一次，取最高的層級。
type ProspectMatch struct {
	Prospect  Prospect
	Tier      ProspectMatchTier
	MatchedOn []MatchReason
}

// ProspectCursor 是 admin 清單的 keyset 位置（新到舊）。
type ProspectCursor struct {
	UpdatedAt  time.Time
	ProspectID int64
}

// ProspectListInput 是 admin 清單：Status 為 nil 只看 ACTIVE；Category 與 Region 是精確比對，Keyword 是名稱與三段文字的
// 模糊搜尋（ILIKE）。
type ProspectListInput struct {
	Status         *ProspectStatus
	OutreachStatus *OutreachStatus
	VendorType     *VendorType
	Category       string
	Region         string
	Keyword        string
	PageSize       int32
	After          *ProspectCursor
}

// ProspectPage 是一頁業者；Next 為 nil 表示沒有下一頁。
type ProspectPage struct {
	Items []Prospect
	Next  *ProspectCursor
}

// DuplicateCursor 是疑似重複清單的 keyset 位置。
type DuplicateCursor struct{ LeftID, RightID int64 }

// DuplicatePair 是一對疑似重複（比對鍵相同或名稱相同、沒有標記為不是同一家的 ACTIVE 業者），ID 小的在左。
type DuplicatePair struct {
	Left, Right Prospect
	// MatchedOn 是命中的原因：名稱相同、比對鍵的種類；至少一個，排序固定。
	MatchedOn []MatchReason
}

// DuplicatePage 是一頁疑似重複。
type DuplicatePage struct {
	Items []DuplicatePair
	Next  *DuplicateCursor
}

// TaxIDMatchCursor 是統編對帳清單的 keyset 位置。
type TaxIDMatchCursor struct{ ProspectID, StoreID int64 }

// TaxIDMatch 是統編對得上、但還沒連起來的業者與店家（以 KYC 申請的 store_id 為鍵）。
type TaxIDMatch struct {
	Prospect    Prospect
	StoreID     int64
	StoreName   *string
	CompanyName *string
	KYCStatus   KYCStatus
}

// TaxIDMatchPage 是一頁統編對帳。
type TaxIDMatchPage struct {
	Items []TaxIDMatch
	Next  *TaxIDMatchCursor
}

// ProspectSearchInput 是採購候選的搜尋條件（P5）：全部沒填是所有 ACTIVE 業者；填了的條件是加權 OR，不是 AND。Limit 為 0 用
// 預設，超過上限降到上限。
type ProspectSearchInput struct {
	Category    string
	Region      string
	Keywords    string
	CategoryIDs []string
	Limit       int32
}

// ProspectSearchHit 是一筆候選與它命中了哪些條件（算分與排序是呼叫端的事）；Website 是主要的網站，沒有時是 nil。
type ProspectSearchHit struct {
	Prospect       Prospect
	Website        *string
	HitCategory    bool
	HitRegion      bool
	HitKeywords    bool
	HitCategoryIDs bool
}

// ProspectContact 是業者的主要 email 與狀態（vendor portal 邀請供應商用）；沒有 email 時 Email 是 nil。Status 讓呼叫方在
// 同一次讀取裡判斷業者是不是還在進行中（讀取帶共享鎖，見 prospect.Service.Contact）。
type ProspectContact struct {
	ProspectID  int64
	DisplayName string
	Email       *string
	Status      ProspectStatus
}

// ArchiveProspectInput 是封存的輸入；ExpectedVersion 是讀到的版本（etag），必填。
type ArchiveProspectInput struct{ ProspectID, AdminID, ExpectedVersion int64 }

// RestoreProspectInput 是還原的輸入：不帶版本（還原依據的是封存這個狀態，不是內容）。聯絡方式的比對鍵被其他未封存業者
// 佔走時的處理同 CreateProspectInput 的 ConfirmedDistinct。
type RestoreProspectInput struct {
	ProspectID, AdminID int64
	ConfirmedDistinct   []int64
}

// MarkProspectsDistinctInput 是管理員直接標記一對候選不是同一家的輸入。不帶版本：決定的是身分（這兩個業者是不是同一家），
// 不是內容，內容之後怎麼改都不影響它。
type MarkProspectsDistinctInput struct{ FirstID, SecondID, AdminID int64 }

// MergeProspectsInput 是合併的輸入：輸家的聯絡方式併入贏家，輸家封存；兩邊的版本都是讀到的版本（etag），必填。
type MergeProspectsInput struct{ WinnerID, LoserID, AdminID, WinnerVersion, LoserVersion int64 }

// ConvertProspectInput 是手動轉成店家的輸入；StoreID 必須是目前的 stint，ExpectedVersion 是讀到的版本（etag），必填。
type ConvertProspectInput struct{ ProspectID, StoreID, AdminID, ExpectedVersion int64 }

// ProspectIDsMetadataKey 是 PROSPECT_MATCH_FOUND 的 ErrorInfo.metadata key：相符的業者 ID，十進位、由小到大以逗號連接
// （API 設計規範 §八）。
const ProspectIDsMetadataKey = "prospectIds"

// NewProspectMatchFound 建立「聯絡方式的比對鍵與其他未封存的業者相同、而且沒有標記為不是同一家」的錯誤：FailedPrecondition，
// reason 是 PROSPECT_MATCH_FOUND，相符的業者放在 metadata（ProspectIDsMetadataKey，由小到大）。client 讓管理員選擇合併，
// 或在請求帶上 ConfirmedDistinct 再送一次。
func NewProspectMatchFound(ids []int64) *apperr.Error {
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	labels := make([]string, len(sorted))
	for i, id := range sorted {
		labels[i] = strconv.FormatInt(id, 10)
	}
	joined := strings.Join(labels, ",")
	return apperr.FailedPrecondition("聯絡方式與業者 %s 相同，請確認是否為同一家", strings.Join(labels, "、")).
		WithReason(apperr.ReasonProspectMatchFound).WithMetadata(ProspectIDsMetadataKey, joined)
}

var taxIDPattern = regexp.MustCompile(`^[0-9]{8}$`)

// 欄位的長度上限與 DB 的 prospects_*_check 一致（文字欄位另有上限，只在這裡擋）。
const (
	prospectTextMax     = 10000
	prospectCategoryMax = 64
	prospectCategories  = 100
)

// ProspectFieldsFor 把 Prospect 現在的值還原成可編輯的欄位。
func ProspectFieldsFor(p Prospect) ProspectFields {
	return ProspectFields{
		DisplayName: p.DisplayName, VendorType: p.VendorType, OutreachStatus: p.OutreachStatus, TaxID: p.TaxID,
		Category: p.Category, Region: p.Region, ProductSummary: p.ProductSummary, Positioning: p.Positioning,
		ResearchSummary: p.ResearchSummary, PriorityLetter: p.PriorityLetter, PriorityStars: p.PriorityStars,
	}
}

// MergeProspect 把部分更新疊到目前的欄位：nil 保留原值，空字串清成沒有值。
func MergeProspect(f ProspectFields, in UpdateProspectInput) ProspectFields {
	if in.DisplayName != nil {
		f.DisplayName = *in.DisplayName
	}
	if in.VendorType != nil {
		f.VendorType = *in.VendorType
	}
	if in.OutreachStatus != nil {
		f.OutreachStatus = *in.OutreachStatus
	}
	for _, t := range []struct{ from, to **string }{
		{&in.TaxID, &f.TaxID}, {&in.Category, &f.Category}, {&in.Region, &f.Region}, {&in.ProductSummary, &f.ProductSummary}, {&in.Positioning, &f.Positioning},
		{&in.ResearchSummary, &f.ResearchSummary}, {&in.PriorityLetter, &f.PriorityLetter},
	} {
		if *t.from != nil {
			*t.to = Text(**t.from)
		}
	}
	if in.PriorityStars != nil {
		f.PriorityStars = *in.PriorityStars
	}
	return f
}

// NormalizeProspect 驗證並正規化欄位：一次檢查全部，任何一欄不合法就不寫入。與 DB 的 CHECK 是同一組規則，DB 是保底。
// manual 為 true（管理員的建立與更新）時不接受 OutreachStatus CONVERTED：那個值只由轉成店家寫入，手動設定會造成
// 「進度說已入駐、狀態卻不是」的矛盾列。
func NormalizeProspect(f *ProspectFields, manual bool) error {
	f.DisplayName = trimtext.Trim(f.DisplayName)
	if n := utf8.RuneCountInString(f.DisplayName); n == 0 || n > 255 {
		return apperr.InvalidInputOn([]string{"DisplayName"}, "業者名稱必須是 1 到 255 字")
	}
	if !slices.Contains(VendorTypeValues(), f.VendorType) {
		return apperr.InvalidInputOn([]string{"VendorType"}, "業者類型不合法")
	}
	if f.OutreachStatus == "" {
		f.OutreachStatus = OutreachNotContacted
	}
	if !slices.Contains(OutreachStatusValues(), f.OutreachStatus) {
		return apperr.InvalidInputOn([]string{"OutreachStatus"}, "招商狀態不合法")
	}
	if manual && f.OutreachStatus == OutreachConverted {
		return apperr.InvalidInputOn([]string{"OutreachStatus"}, "招商狀態不可手動設為已入駐，只能由轉成店家寫入")
	}
	for _, t := range []struct {
		value *(*string)
		limit int
		field string
	}{
		{&f.Category, 100, "Category"}, {&f.Region, 100, "Region"},
		{&f.PriorityLetter, 4, "PriorityLetter"}, {&f.ProductSummary, prospectTextMax, "ProductSummary"},
		{&f.Positioning, prospectTextMax, "Positioning"}, {&f.ResearchSummary, prospectTextMax, "ResearchSummary"},
	} {
		if *t.value = textOrNil(*t.value); *t.value != nil && utf8.RuneCountInString(**t.value) > t.limit {
			return apperr.InvalidInputOn([]string{t.field}, "%s 不可超過 %d 字", t.field, t.limit)
		}
	}
	if f.TaxID = textOrNil(f.TaxID); f.TaxID != nil && !taxIDPattern.MatchString(*f.TaxID) {
		return apperr.InvalidInputOn([]string{"TaxID"}, "統一編號必須是 8 位數字")
	}
	if f.PriorityStars < 0 {
		return apperr.InvalidInputOn([]string{"PriorityStars"}, "星等不可為負數")
	}
	return nil
}

func textOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	return Text(*s)
}

// NormalizeCategoryIDs 去掉前後空白、去重並排序；空字串或超過 64 字的 ID 不合法。
func NormalizeCategoryIDs(ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if n := utf8.RuneCountInString(id); n == 0 || n > prospectCategoryMax {
			return nil, apperr.InvalidInputOn([]string{"CategoryIDs"}, "分類 ID 必須是 1 到 %d 字", prospectCategoryMax)
		}
		out = append(out, id)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) > prospectCategories {
		return nil, apperr.InvalidInputOn([]string{"CategoryIDs"}, "分類最多 %d 個", prospectCategories)
	}
	return out, nil
}

// ProspectArchived 在業者從進行中變成封存的 transaction 內發出（重送已封存的不發；合併造成的封存由 ProspectMerged 表達，
// 不另發這個事件；還原不發事件）。
type ProspectArchived struct {
	OccurredAt time.Time
	EventID    uuid.UUID
	ProspectID int64
}

// ProspectMerged 在業者合併的 transaction 內發出：輸家（SourceProspectID）已封存並指向贏家（TargetProspectID）。
type ProspectMerged struct {
	OccurredAt                         time.Time
	EventID                            uuid.UUID
	SourceProspectID, TargetProspectID int64
}
