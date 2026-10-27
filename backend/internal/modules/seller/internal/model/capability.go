package model

import (
	"fmt"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Capability 是賣家能做的一類事，名稱沿用能力檢查 ADR（2026-09-18-seller-capability-authorization）D2 的粗粒度能力。
// 步驟 8 的每個 Connect 方法宣告它需要哪一個。
type Capability string

const (
	// CapAccountSelfRead 看自己的店家資料與停權狀態。
	CapAccountSelfRead Capability = "CAP_ACCOUNT_SELF_READ"
	// CapStorefrontWrite 店面設定：profile、logo、封面、新手引導。
	CapStorefrontWrite Capability = "CAP_STOREFRONT_WRITE"
	// CapProductManage 商品的建立、編輯、上下架、價格。
	CapProductManage Capability = "CAP_PRODUCT_MANAGE"
	// CapOrderFulfill 出貨、物流、售後、訂單查詢、PayUni 狀態。
	CapOrderFulfill Capability = "CAP_ORDER_FULFILL"
	// CapOrderContract 收縮型的訂單動作（取消出貨與退款受理）。
	CapOrderContract Capability = "CAP_ORDER_CONTRACT"
	// CapMarketing 行銷經營：優惠券、CRM、貼文發佈、AI 助理、dashboard 與報表。
	CapMarketing Capability = "CAP_MARKETING"
	// CapContentContract 收縮型的內容動作（刪除、下架自己的貼文，撤回自己的券）。
	CapContentContract Capability = "CAP_CONTENT_CONTRACT"
	// CapChatSend 送出訊息。
	CapChatSend Capability = "CAP_CHAT_SEND"
	// CapChatRead 讀取訊息。
	CapChatRead Capability = "CAP_CHAT_READ"
	// CapNotification 通知收件箱、偏好與裝置。
	CapNotification Capability = "CAP_NOTIFICATION"
	// CapAccountClose 關店。
	CapAccountClose Capability = "CAP_ACCOUNT_CLOSE"
	// CapInvoiceManage 發票與稅務憑證的查詢與設定。
	CapInvoiceManage Capability = "CAP_INVOICE_MANAGE"
	// CapInvoiceTaxRemedy 稅務身分的申請、補件與延長（救濟管道）。
	CapInvoiceTaxRemedy Capability = "CAP_INVOICE_TAX_REMEDY"
	// CapShippingSetup 物流建店、啟用與關閉配送方式；前置條件是 KYC 流程核准（含 PayUni 核准）。
	CapShippingSetup Capability = "CAP_SHIPPING_SETUP"
	// CapPublicListing 店家的狀態：買家看得到店面、可以結帳。缺的條件就是 MissingExposure。它是狀態不是動作：只出現在
	// CapabilitiesOf 的結果，不在 Capabilities()，方法不能宣告它，也不進 gateway 的能力 header。
	CapPublicListing Capability = "CAP_PUBLIC_LISTING"
)

// Capabilities 回傳賣家能做的動作類能力（方法可以宣告、gateway 的 header 可以帶），不含狀態 CapPublicListing。
func Capabilities() []Capability {
	return []Capability{
		CapAccountSelfRead, CapStorefrontWrite, CapProductManage, CapOrderFulfill, CapOrderContract, CapMarketing,
		CapContentContract, CapChatSend, CapChatRead, CapNotification, CapAccountClose,
		CapInvoiceManage, CapInvoiceTaxRemedy, CapShippingSetup,
	}
}

// AllCapabilities 回傳 CapabilitiesOf 回報的全部能力：Capabilities() 加上狀態 CapPublicListing，順序與 proto 的
// SellerCapability 數字一致。
func AllCapabilities() []Capability { return append(Capabilities(), CapPublicListing) }

// needs 是每個能力需要的條件，能力的狀態、缺項、停權白名單與是否取決於 KYC 都由這一張表推導（Stripe Connect 的做法：
// 每個 capability 對應自己的 requirements，狀態是結果）：
//   - STORE_ACTIVE：每個能力都需要，關閉的店家沒有任何能力；
//   - NOT_SUSPENDED：經營類能力需要。停權時仍授予的能力（能力檢查 ADR 的白名單）就是表裡沒有它的那些；
//   - NOT_PENDING_DELETION：經營類能力與關店需要，等待刪除時只剩履約類；
//   - KYC_APPROVED：物流建店需要（PayUni 以已成立的子商店建店）；
//   - 店家狀態 CapPublicListing 需要全部條件，缺的就是 MissingExposure。
var needs = map[Capability][]Requirement{
	CapAccountSelfRead:  {RequirementActive},
	CapOrderFulfill:     {RequirementActive},
	CapOrderContract:    {RequirementActive},
	CapContentContract:  {RequirementActive},
	CapChatSend:         {RequirementActive},
	CapChatRead:         {RequirementActive},
	CapNotification:     {RequirementActive},
	CapInvoiceTaxRemedy: {RequirementActive},
	CapAccountClose:     {RequirementActive, RequirementNotPendingDeletion},
	CapStorefrontWrite:  {RequirementActive, RequirementNotSuspended, RequirementNotPendingDeletion},
	CapProductManage:    {RequirementActive, RequirementNotSuspended, RequirementNotPendingDeletion},
	CapMarketing:        {RequirementActive, RequirementNotSuspended, RequirementNotPendingDeletion},
	CapInvoiceManage:    {RequirementActive, RequirementNotSuspended, RequirementNotPendingDeletion},
	CapShippingSetup:    {RequirementActive, RequirementNotSuspended, RequirementKYCApproved, RequirementNotPendingDeletion},
	CapPublicListing: {
		RequirementActive, RequirementNotSuspended, RequirementKYCApproved, RequirementStoreName, RequirementLogo,
		RequirementTownCode, RequirementNotPendingDeletion,
	},
}

// Needs 回傳能力需要的條件，依 Requirements() 的順序；不認得的能力回 nil。
func Needs(c Capability) []Requirement {
	var out []Requirement
	for _, r := range Requirements() {
		if slices.Contains(needs[c], r) {
			out = append(out, r)
		}
	}
	return out
}

// GrantedWhenSuspended 回傳停權時仍授予的能力（能力檢查 ADR 的白名單）：不需要 NOT_SUSPENDED 的動作類能力。
func GrantedWhenSuspended() []Capability {
	var out []Capability
	for _, c := range Capabilities() {
		if !slices.Contains(needs[c], RequirementNotSuspended) {
			out = append(out, c)
		}
	}
	return out
}

// CapabilityStatus 是一個能力此刻的狀態：Active，或沒有啟用與還缺的條件（Missing，依 Requirements() 的順序）。
type CapabilityStatus struct {
	Capability Capability
	Active     bool
	Missing    []Requirement
}

// missingFor 是一個能力缺的條件，依 Requirements() 的順序：表（needs）裡該能力需要、而店家此刻沒有滿足的條件（met：讀店家時由 SQL 讀出的
// Conditions 與 Sanctions）。
func missingFor(c Capability, s Store) []Requirement {
	var missing []Requirement
	for _, r := range Needs(c) {
		if !met(r, s) {
			missing = append(missing, r)
		}
	}
	return missing
}

// CapabilitiesOf 是店家能力唯一的推導點：由讀店家時帶上的條件（Conditions：生命週期、KYC 流程核准、公開曝光看的店名、logo、鄉鎮市區，
// 由 seller.store_requirement_flags 讀出）與停權（Sanctions，由 seller.effective_measures 讀出）決定，回傳 AllCapabilities() 的每一項
// （依 proto 的數字順序）的狀態與缺的條件；Go 只做能力對條件的對應（needs），不重算條件。關閉的店家沒有任何能力；等待刪除的店家
// 不能做經營類的事，也不能自己關店（關閉由刪除帳號的流程負責）；停權時只剩白名單；物流建店還要 KYC 流程核准。
func CapabilitiesOf(s Store) []CapabilityStatus {
	all := AllCapabilities()
	out := make([]CapabilityStatus, 0, len(all))
	for _, c := range all {
		missing := missingFor(c, s)
		out = append(out, CapabilityStatus{Capability: c, Active: len(missing) == 0, Missing: missing})
	}
	return out
}

// ActiveCapabilities 回傳 CapabilitiesOf 結果中啟用的動作類能力（不含狀態 CapPublicListing），給不需要缺項的消費者
// （gateway 的 header）。
func ActiveCapabilities(statuses []CapabilityStatus) []Capability {
	var out []Capability
	for _, st := range statuses {
		if st.Active && st.Capability != CapPublicListing {
			out = append(out, st.Capability)
		}
	}
	return out
}

// Can 回報店家目前能不能做動作類能力 c（CapabilitiesOf 的同一個推導）。c 不是動作類能力（CapPublicListing 或未知）panic：
// 呼叫方寫錯是程式錯誤（比照 seller.New 缺依賴時 panic），在第一次執行就會發現。
func (s Store) Can(c Capability) bool {
	if !slices.Contains(Capabilities(), c) {
		panic(fmt.Sprintf("seller: Store.Can 不接受 %q：它不是動作類能力", c))
	}
	return len(missingFor(c, s)) == 0
}

// Authorize 在店家沒有能力 c 時回錯誤：已關閉回 NotFound；停權造成的回 PermissionDenied，帶 ReasonSellerSuspended；
// 其他前置條件沒有滿足（等待刪除、KYC 流程未核准）回 FailedPrecondition，帶 ReasonSellerRequirementMissing，缺的條件在
// metadata 的 MissingRequirementsMetadataKey。c 不是動作類能力（CapPublicListing 或未知）、或店家的條件沒有推導過（沒有經
// stint 讀出）一律拒絕。
func Authorize(s Store, c Capability) error {
	if !slices.Contains(Capabilities(), c) {
		return fmt.Errorf("seller: %q 不是可以授權的賣家能力", c)
	}
	if !s.Conditions.Derived {
		return fmt.Errorf("seller: 店家 %d 的條件沒有推導過，不能授權", s.StoreID)
	}
	missing := missingFor(c, s)
	switch {
	case len(missing) == 0:
		return nil
	case slices.Contains(missing, RequirementActive):
		return apperr.NotFound("店家已關閉")
	case slices.Contains(missing, RequirementNotSuspended):
		return apperr.PermissionDenied("賣家帳號停權中，無法執行此操作").WithReason(apperr.ReasonSellerSuspended)
	default:
		return NewRequirementMissing(missing)
	}
}
