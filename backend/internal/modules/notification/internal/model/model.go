// Package model 是 notification 模組的代碼值：受眾、渠道、偏好分類（D4）、呈現分類（D6）與狀態重讀提示。
// 儲存的值是 UPPER_SNAKE，與 db/schema.sql 的 CHECK 一致；偏好分類的「不可關」規則只在這裡（Disableable）。
package model

import "slices"

// Audience 是收件人的身分面：BUYER 是買家，STORE 是店家；兩者共用同一個帳號，由 App 與推播目標區分（不變量：BUYER 只推 buyer App、STORE 只推 seller App）。
type Audience string

const (
	AudienceBuyer Audience = "BUYER"
	AudienceStore Audience = "STORE"
)

// Audiences 回傳全部受眾。
func Audiences() []Audience { return []Audience{AudienceBuyer, AudienceStore} }

// Channel 是偏好的渠道：INBOX 是收件匣，PUSH 與 EMAIL 是投遞渠道（只有後兩者有投遞紀錄）。
type Channel string

const (
	ChannelInbox Channel = "INBOX"
	ChannelPush  Channel = "PUSH"
	ChannelEmail Channel = "EMAIL"
)

// Channels 回傳全部渠道。
func Channels() []Channel { return []Channel{ChannelInbox, ChannelPush, ChannelEmail} }

// DeliveryChannel 回報 c 是不是投遞渠道（有投遞紀錄的渠道）。
func DeliveryChannel(c Channel) bool { return c == ChannelPush || c == ChannelEmail }

// PrefCategory 是偏好分類（D4）：使用者設定偏好的單位，偏好覆寫列的 category。
type PrefCategory string

const (
	// 交易必要。
	CategoryOrder           PrefCategory = "ORDER"
	CategoryShipment        PrefCategory = "SHIPMENT"
	CategoryShipmentAnomaly PrefCategory = "SHIPMENT_ANOMALY"
	CategoryAftersales      PrefCategory = "AFTERSALES"
	CategoryPayout          PrefCategory = "PAYOUT"
	CategoryAccount         PrefCategory = "ACCOUNT"
	// 互動。
	CategoryChat              PrefCategory = "CHAT"
	CategoryReview            PrefCategory = "REVIEW"
	CategoryComment           PrefCategory = "COMMENT"
	CategoryLike              PrefCategory = "LIKE"
	CategoryFollow            PrefCategory = "FOLLOW"
	CategoryFollowedStorePost PrefCategory = "FOLLOWED_STORE_POST"
	// 行銷。
	CategoryCoupon  PrefCategory = "COUPON"
	CategoryRestock PrefCategory = "RESTOCK"
)

// CategoryClass 是 D4 的三大類，決定哪些渠道可以關。
type CategoryClass string

const (
	ClassTransactional CategoryClass = "TRANSACTIONAL"
	ClassInteraction   CategoryClass = "INTERACTION"
	ClassMarketing     CategoryClass = "MARKETING"
)

type categoryDef struct {
	category PrefCategory
	class    CategoryClass
	// locked 是這個分類不可關的渠道。
	locked []Channel
}

// D4：交易必要的收件匣與 email 不可關（推播可關，賣家 SHIPMENT_ANOMALY 的推播也不可關）；互動與行銷全部可關、預設開。
var transactionalLocked = []Channel{ChannelInbox, ChannelEmail}

var categoryDefs = []categoryDef{
	{CategoryOrder, ClassTransactional, transactionalLocked},
	{CategoryShipment, ClassTransactional, transactionalLocked},
	{CategoryShipmentAnomaly, ClassTransactional, []Channel{ChannelInbox, ChannelPush, ChannelEmail}},
	{CategoryAftersales, ClassTransactional, transactionalLocked},
	{CategoryPayout, ClassTransactional, transactionalLocked},
	{CategoryAccount, ClassTransactional, transactionalLocked},
	{CategoryChat, ClassInteraction, nil},
	{CategoryReview, ClassInteraction, nil},
	{CategoryComment, ClassInteraction, nil},
	{CategoryLike, ClassInteraction, nil},
	{CategoryFollow, ClassInteraction, nil},
	{CategoryFollowedStorePost, ClassInteraction, nil},
	{CategoryCoupon, ClassMarketing, nil},
	{CategoryRestock, ClassMarketing, nil},
}

// Categories 回傳 D4 的全部偏好分類；每次回傳新的切片。
func Categories() []PrefCategory {
	out := make([]PrefCategory, len(categoryDefs))
	for i, d := range categoryDefs {
		out[i] = d.category
	}
	return out
}

func lookupCategory(c PrefCategory) (categoryDef, bool) {
	for _, d := range categoryDefs {
		if d.category == c {
			return d, true
		}
	}
	return categoryDef{}, false
}

// ValidCategory 回報 c 是不是 D4 清單裡的分類。
func ValidCategory(c PrefCategory) bool {
	_, ok := lookupCategory(c)
	return ok
}

// ClassOf 回傳分類所屬的大類；不在清單內回 false。
func ClassOf(c PrefCategory) (CategoryClass, bool) {
	d, ok := lookupCategory(c)
	return d.class, ok
}

// Disableable 回報使用者能不能關掉分類 c 在渠道 ch 的通知；不在清單內的分類或渠道一律不可關（偏好 API 不接受它們）。
func Disableable(c PrefCategory, ch Channel) bool {
	d, ok := lookupCategory(c)
	if !ok || !slices.Contains(Channels(), ch) {
		return false
	}
	return !slices.Contains(d.locked, ch)
}

// DefaultEnabled 是沒有覆寫列時的預設：全部分類、全部渠道預設開（行銷也是，個資法第 20 條只要求可關）。
func DefaultEnabled(PrefCategory, Channel) bool { return true }

// Presentation 是收件匣的呈現分類（D6，10 個值；不是每個型別一個值）。
type Presentation string

const (
	PresentationOrder       Presentation = "ORDER"
	PresentationRefund      Presentation = "REFUND"
	PresentationPayout      Presentation = "PAYOUT"
	PresentationMessage     Presentation = "MESSAGE"
	PresentationSocial      Presentation = "SOCIAL"
	PresentationReview      Presentation = "REVIEW"
	PresentationPromotion   Presentation = "PROMOTION"
	PresentationProcurement Presentation = "PROCUREMENT"
	PresentationAccount     Presentation = "ACCOUNT"
	PresentationSystem      Presentation = "SYSTEM"
)

// Presentations 回傳 D6 的全部呈現分類。
func Presentations() []Presentation {
	return []Presentation{
		PresentationOrder, PresentationRefund, PresentationPayout, PresentationMessage, PresentationSocial,
		PresentationReview, PresentationPromotion, PresentationProcurement, PresentationAccount, PresentationSystem,
	}
}

// StateHint 是推播與串流帶的狀態重讀提示（D6 重議）：App 收到後重讀訂單、售後或賣家狀態；空值表示不需要重讀。
type StateHint string

const (
	StateHintNone         StateHint = ""
	StateHintOrder        StateHint = "ORDER"
	StateHintAfterSale    StateHint = "AFTER_SALE"
	StateHintSellerStatus StateHint = "SELLER_STATUS"
)

// StateHints 回傳全部非空的狀態重讀提示。
func StateHints() []StateHint {
	return []StateHint{StateHintOrder, StateHintAfterSale, StateHintSellerStatus}
}
