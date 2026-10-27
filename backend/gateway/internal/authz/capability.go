// Package authz is the single question-point for "can this seller do this thing".
//
// It holds the Gateway-internal seller capability model (46 號設計書): a small set
// of coarse-grained capabilities (aligned to the 06 功能矩陣 終判 categories), the
// policy that maps a seller's context (status, tier) to the set of capabilities it
// currently holds, and a registry of which route declared which capability.
//
// Capability tokens are a Gateway-internal authorization concept — they do NOT
// cross service boundaries and are NOT surfaced to the front-end, so they live here
// as Go constants and deliberately stay OUT of CodeMeta (46 §8.1). The suspension
// business codes (9240/9241) and reason codes DO go through CodeMeta; capabilities
// do not.
//
// 規範：`IsSeller(` / 停權（suspended）判斷只准出現在本 package。禁止在
// handler / usecase 內自行組裝賣家身分或停權 if（唯一例外＝IM/social 的訂單關聯
// 細分，由下游 usecase 讀 injectSellerSuspension 注入的 header，見 46 §4.4）。
package authz

// Capability is a coarse-grained Gateway-internal authorization token marking one
// class of thing a seller may do. Value is the SHOUTY token name so obs/logs and the
// route-coverage test read cleanly; the Go identifier stays PascalCase per §13.4.
type Capability string

// The 15 capabilities (46 §3.4；CapInvoiceManage、CapInvoiceTaxRemedy 由 58 號 §8.1.6 新增，CapShippingSetup 由前置條件能力新增). Which capabilities a seller holds is derived only by the
// monolith's seller module (model.CapabilitiesOf) and sent in X-Seller-Capabilities; the gateway only checks
// membership. "白名單" below means the seller module keeps the capability while suspended.
const (
	// CapAccountSelfRead — 看自己 profile / 停權狀態（A GET me、看自己帳號）。白名單。
	CapAccountSelfRead Capability = "CAP_ACCOUNT_SELF_READ"
	// CapStorefrontWrite — 店面設定寫入（profile/avatar/cover/onboarding；A PUT/PATCH me）。BLOCK。
	CapStorefrontWrite Capability = "CAP_STOREFRONT_WRITE"
	// CapProductManage — 商品建立/編輯/上下架/匯入/分類/運費試算/認證/hashtag（C、D、部分 I 寫）。BLOCK。
	CapProductManage Capability = "CAP_PRODUCT_MANAGE"
	// CapOrderFulfill — 出貨/物流/售後/訂單查詢/PayUni 狀態（I、J、K orders view=seller、A payuni）。白名單。
	CapOrderFulfill Capability = "CAP_ORDER_FULFILL"
	// CapOrderContract — 收縮型：cancelShipment（取消出貨+退款受理；J cancelShipment）。白名單。
	CapOrderContract Capability = "CAP_ORDER_CONTRACT"
	// CapMarketing — 行銷經營：優惠券建立/CRM 寫入+查詢/貼文發佈/AI 助理/dashboard/報表（E、F建、G、H、A dashboard）。BLOCK。
	CapMarketing Capability = "CAP_MARKETING"
	// CapContentContract — 收縮型：刪自己貼文、自行下架貼文、撤自己券（E DeletePost／UnpublishMyPost、F Revoke）。白名單。
	CapContentContract Capability = "CAP_CONTENT_CONTRACT"
	// CapChatSend — IM 送訊（停權時 im-service 再做訂單關聯細分）。白名單（受限）。
	CapChatSend Capability = "CAP_CHAT_SEND"
	// CapChatRead — IM 讀取（history/conversations/unread/已讀）。白名單。
	CapChatRead Capability = "CAP_CHAT_READ"
	// CapNotification — 通知收件箱/偏好/device-token。白名單。
	CapNotification Capability = "CAP_NOTIFICATION"
	// CapAccountClose — 收縮型：關閉店家（刪帳號是 identity 的事，不需要這個能力）。白名單。
	CapAccountClose Capability = "CAP_ACCOUNT_CLOSE"
	// CapInvoiceManage — 發票／稅務憑證的「查詢與設定」：S01 總覽、S03 AMEGO 憑證、
	// S05 收據代製授權、S06/S07 發票紀錄與明細（含 PDF 反代子路由）、S08 異常詳情、
	// S12 進項發票。BLOCK：停權時不授予。唯讀查詢也 BLOCK 的依據是 46 號
	// 2026-07-10 裁決「純唯讀查看也鎖」（規範 §五 第 2 點）。58 號 §8.1.6(a)。
	CapInvoiceManage Capability = "CAP_INVOICE_MANAGE"
	// CapInvoiceTaxRemedy — 稅務身分申請／補件／延長申請／自行開票責任條款同意
	// （S02、S04）＋ 自行開立回報（端點 62／62R）。白名單：停權賣家仍必須能補稅務件
	// ——鎖住即死結（補不了件 ⇒ 審核不完成 ⇒ 恢復後依然接不了單）。58 號 §8.1.6(a)。
	//
	// 為什麼一定要兩個 token 而不是一個：白名單成員資格由集合成員關係編碼
	// （seller 模組的停權白名單）⇒ 同一個 token 不可能既 BLOCK 又放行，
	// 而發票模組確實同時需要兩類。這是機械上的必然，不是設計偏好。
	CapInvoiceTaxRemedy Capability = "CAP_INVOICE_TAX_REMEDY"
	// CapShippingSetup — 物流建店、啟用與關閉配送方式（shipping_onboardings 的建立與 toggle）。BLOCK，
	// 而且有前置條件：KYC 流程核准（含 PayUni 核准）才授予。前置條件由 seller 模組推導，gateway 一樣
	// 只檢查集合成員。讀取（列出與查詢建店設定、shipping_methods）留在 CapProductManage。
	CapShippingSetup Capability = "CAP_SHIPPING_SETUP"
)

// AllCapabilities lists every capability, used to build the active set and to
// validate that a declared capability is a known token.
var AllCapabilities = []Capability{
	CapAccountSelfRead, CapStorefrontWrite, CapProductManage, CapOrderFulfill,
	CapOrderContract, CapMarketing, CapContentContract,
	CapChatSend, CapChatRead, CapNotification, CapAccountClose,
	CapInvoiceManage, CapInvoiceTaxRemedy, CapShippingSetup,
}

// IsKnown reports whether cap is one of the defined capability tokens.
func IsKnown(cap Capability) bool {
	for _, c := range AllCapabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// IsSellerView reports whether a `?view=` query value selects the seller-facing
// view of a buyer/seller-shared route（46 §3.3；"seller" 與 "selling" 同義，其餘值
// 含空字串一律買家視角）。sellerViewGuard 與 handler 的 view 分流都必須用本判定，
// 禁止散落字面值比對。
func IsSellerView(view string) bool {
	return view == "seller" || view == "selling"
}
