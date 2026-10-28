// Package codemeta mirrors the subset of CodeMeta enum string values that the
// gateway needs to inject into downstream gRPC requests.
//
// Source of truth: docs/代碼管理（CodeMeta）/業務類型碼.md (ClientType, AppType).
// Account type ints come from gateway/internal/middleware/account_type.go.
//
// We do not import the generated internal/platform/codemeta/code_meta.go: it
// belongs to the monolith module, which gateway cannot import; keeping a thin local mirror
// avoids cross-module coupling. Keep these values in sync with the markdown.
package codemeta

import "github.com/Will413028/hawkr/gateway/internal/middleware"

// ClientType values (CodeMeta TechnicalCode "ClientType").
const (
	ClientTypeMobileApp  = "MOBILE_APP"
	ClientTypeWebBrowser = "WEB_BROWSER"
)

// AppType values (CodeMeta TechnicalCode "AppType").
const (
	AppTypeBuyer  = "BUYER"
	AppTypeSeller = "SELLER"
)

// AfterSaleAppealState values (CodeMeta BusinessStatusCode "AfterSaleAppealState",
// docs/代碼管理（CodeMeta）/業務狀態碼.md). These are the enum *Value* column —
// aftersale_cases.appeal_state stores the Value and every API surface (request
// filter and response field alike) carries the same Value, never the Name.
const (
	AfterSaleAppealStateNone          = "0"
	AfterSaleAppealStateAppealed      = "10"
	AfterSaleAppealStateInvestigating = "20"
	AfterSaleAppealStateResolved      = "30"
)

// AppTypeFromAccountType derives the AppType string from gateway's int-valued
// JWT account_type. SELLER_AND_BUYER and ADMIN both fall back to BUYER because
// the only callers (payment / refund) are buyer-flow endpoints.
func AppTypeFromAccountType(accountType int) string {
	switch accountType {
	case middleware.AccountTypeSeller:
		return AppTypeSeller
	default:
		return AppTypeBuyer
	}
}

// WeightUnit values (CodeMeta BusinessTypeCode "WeightUnit",
// docs/代碼管理（CodeMeta）/業務類型碼.md). product_sku.unit stores the *Value*,
// not the Code and not the Chinese label.
//
// 這裡鏡像的是**值域**不是 Label：Label 一律走生成的 pkg/codemeta（手寫第二份中文
// 對照表的失敗形狀是 drift 而非錯誤）。值域鏡像在本 package 早有先例
// （AfterSaleAppealState），動機相同——Gateway 要能在寫入前擋掉髒值，而髒值落庫
// 會讓商品上架成功、到結帳鏈才由 WeightUnit.fromValue() 拋例外。
const (
	WeightUnitKG    = "3301"
	WeightUnitCatty = "3302"
	WeightUnitGram  = "3303"
)

// IsValidWeightUnit reports whether value is one of the three WeightUnit values.
func IsValidWeightUnit(value string) bool {
	switch value {
	case WeightUnitKG, WeightUnitCatty, WeightUnitGram:
		return true
	default:
		return false
	}
}

// DelegationAuditActions is the closed nineteen-value set of
// DelegationAuditAction (56 §6.5.3). Gateway needs the SET (to reject an
// unknown audit-logs filter with 9260 instead of silently returning the full
// result) — the Chinese labels stay in the generated pkg/codemeta.
//
// 靜默忽略一個不合法的篩選值，會讓前端以為篩選生效而實際拿到全集；那種錯誤在畫面上
// 看起來完全正常。
var DelegationAuditActions = map[string]struct{}{
	"STOREFRONT_UPDATE": {}, "STOREFRONT_LOGO_UPDATE": {}, "STOREFRONT_COVER_UPDATE": {},
	"PRODUCT_CREATE": {}, "PRODUCT_UPDATE": {}, "PRODUCT_PUBLISH": {},
	"PRODUCT_UNPUBLISH": {}, "PRODUCT_BATCH_UNPUBLISH": {},
	"POST_CREATE": {}, "POST_UPDATE": {}, "POST_PUBLISH": {},
	"POST_UNPUBLISH": {}, "POST_SCHEDULE": {},
	"ORDER_PII_REVEAL": {},
	"STOREFRONT_READ":  {}, "PRODUCT_READ": {}, "POST_READ": {}, "ORDER_READ": {},
	"AUDIT_CORRECTION": {},
}

// IsKnownDelegationAuditAction reports whether action is one of the nineteen.
func IsKnownDelegationAuditAction(action string) bool {
	_, ok := DelegationAuditActions[action]
	return ok
}

// DelegationResourceTypes is the closed four-value set backing chk_aoal_resource_type.
var DelegationResourceTypes = map[string]struct{}{
	"storefront": {}, "product": {}, "post": {}, "order": {},
}

// DelegationAuditResults is the closed three-value set backing chk_aoal_result.
// PENDING is a legitimate terminal state, not a transient one — a confirm that never
// arrived leaves the row there for good.
var DelegationAuditResults = map[string]struct{}{
	"PENDING": {}, "SUCCEEDED": {}, "FAILED": {},
}
