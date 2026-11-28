package handler

import (
	"slices"
	"strings"
)

// AllowedAction 是對外契約的動作描述。
//
// 🔴 為什麼一定要有 Href ＋ Method（只給 Action 代號不夠）：
// 前端不從 status 推導「可以做什麼」是一半；另一半是它同樣不該推導「要打哪支 URL」——
// 而後者需要的正是同一份 provider 知識。後端直接給 href ⇒ **新增 provider 時前端零改動**。
//
// 🔴 為什麼 DisabledReasonCode 在 Enabled=false 時必填：
// 沒有它，前端只能讓按鈕憑空消失。「不可用要說明原因、不得憑空消失」是前端規格的硬要求，
// 而使用者看到按鈕消失時會以為是自己的問題。
type AllowedAction struct {
	Action             string `json:"action"`                       // FulfillmentAction 值域（snake_case）
	Href               string `json:"href"`                         // 完整路徑，前端只跟隨
	Method             string `json:"method"`                       // GET | POST
	Enabled            bool   `json:"enabled"`                      //
	DisabledReasonCode string `json:"disabledReasonCode,omitempty"` // 業務碼；Enabled=false 時必填
	DisabledReasonText string `json:"disabledReasonText,omitempty"` //
	// Actor 是「誰有資格執行這個動作」——AccountType 值域（"1" 買家 / "2" 賣家 / "4" 管理員）。
	//
	// 🔴 這一格是**對外可驗證的**，不只是內部過濾條件：
	// 沒有它，「後端有正確區分 actor」這件事對前端與 QA 都只能相信、無法查。
	// 回應本身已依 viewer 過濾過，所以這裡的值恆等於呼叫者的身分——
	// 那正是重點：**它讓「這條動作屬於誰」變成契約的一部分而不是實作細節**。
	Actor string `json:"actor"`
}

// ActionInput 是 shipping-service 算出的可用性（proto ActionAvailability 的 gateway 側投影）。
type ActionInput struct {
	Action             string
	Enabled            bool
	DisabledReasonCode string
	// ActorAccountTypes 是 shipping-service 宣告的 actor eligibility ——
	// 「哪些身分可以做這個動作」的**唯一** SSOT（FulfillmentActionPolicy.ACTORS）。
	// AccountType 值域："1" 買家 / "2" 賣家 / "4" 管理員；集合而非單值，因為
	// 同一動作買賣家都能做、只是端點不同（track／view_only）是常態不是例外。
	//
	// 🔴 Gateway 不得自行判斷「這個 action 屬於誰」。先前 viewerScopedRoute 用
	// 「這張路由表有沒有這個 viewer 的 key」當資格判定（drop 理由逐字是 actor_mismatch），
	// 於是領域宣告的 actor 對 track／view_only 完全是死的 ⇒ 兩份業務矩陣。
	ActorAccountTypes []string
}

type routeKey struct {
	action   string
	provider string
}

type routeTarget struct {
	method string
	path   string
}

const (
	providerTaqbin   = "3001"
	providerCvs      = "3002"
	providerMerchant = "3005"
)

var actionRoute = map[routeKey]routeTarget{
	{"dispatch", providerTaqbin}:    {"POST", "/api/v1/app/shipments/{order_no}/call_tcat"},
	{"print_label", providerTaqbin}: {"POST", "/api/v1/app/shipments/{order_no}/generate_home_label"},

	{"handover", providerCvs}:    {"POST", "/api/v1/app/shipments/{order_no}/confirm"},
	{"print_label", providerCvs}: {"POST", "/api/v1/app/shipments/{order_no}/print_cvs_label"},
}

var viewerScopedRoute = map[routeKey]map[string]routeTarget{
	{"track", providerTaqbin}:   trackTargets(),
	{"track", providerCvs}:      trackTargets(),
	{"track", providerMerchant}: trackTargets(),

	{"view_only", providerTaqbin}:   trackTargets(),
	{"view_only", providerCvs}:      trackTargets(),
	{"view_only", providerMerchant}: trackTargets(),
}

func trackTargets() map[string]routeTarget {
	return map[string]routeTarget{
		ViewerBuyer:  {"GET", "/api/v1/app/orders/{order_no}/logistics"},
		ViewerSeller: {"GET", "/api/v1/app/shipments/{order_no}/tracking"},
	}
}

// Viewer 是「誰在看這個回應」——AccountType 值域。
const (
	ViewerBuyer  = "1"
	ViewerSeller = "2"
	ViewerAdmin  = "4"
)

// BuildAllowedActions 把 shipping-service 算出的可用性補上路由，成為對外契約。
//
// 🔴 **三道 fail-closed，任一不成立就丟棄，不猜。**
//  1. actor 不是本次的 viewer ⇒ 丟棄。買家端不得出現賣家動作
//     （QA 實測：買家跟隨那些 href 得 403 seller access required）。
//  2. (action, provider) 不在表上 ⇒ 丟棄。猜錯的症狀出現在使用者面前而不是這裡。
//  3. 不可用卻沒帶原因碼 ⇒ 丟棄。前端無法說明原因，按鈕會憑空消失。
//
// 丟棄的項目回在 dropped 供呼叫端決定要不要告警——靜默丟棄與「本來就不該出現」長得一樣。
func BuildAllowedActions(orderNo, provider, viewer string, availability []ActionInput) (actions []AllowedAction, dropped []string) {
	for _, a := range availability {
		if len(a.ActorAccountTypes) == 0 {
			dropped = append(dropped, a.Action+"(missing_actor)")
			continue
		}
		if !slices.Contains(a.ActorAccountTypes, viewer) {
			dropped = append(dropped, a.Action+"(actor_mismatch)")
			continue
		}

		key := routeKey{a.Action, provider}
		target, ok := resolveTarget(key, viewer)
		if !ok {
			dropped = append(dropped, a.Action+"(no_route_for_provider)")
			continue
		}

		if !a.Enabled && a.DisabledReasonCode == "" {
			dropped = append(dropped, a.Action+"(missing_reason)")
			continue
		}

		actions = append(actions, AllowedAction{
			Action:             a.Action,
			Href:               strings.ReplaceAll(target.path, "{order_no}", orderNo),
			Method:             target.method,
			Enabled:            a.Enabled,
			DisabledReasonCode: a.DisabledReasonCode,
			Actor:              viewer,
		})
	}
	return actions, dropped
}

func resolveTarget(key routeKey, viewer string) (routeTarget, bool) {
	if byViewer, dual := viewerScopedRoute[key]; dual {
		target, ok := byViewer[viewer]
		return target, ok
	}
	target, ok := actionRoute[key]
	return target, ok
}

// KnownFulfillmentActionRoutes 供契約測試斷言本表的涵蓋範圍。
// 回傳 "action@provider" 形式，讓測試能同時看到兩個維度。
func KnownFulfillmentActionRoutes() []string {
	out := make([]string, 0, len(actionRoute)+len(viewerScopedRoute))
	for k := range actionRoute {
		out = append(out, k.action+"@"+k.provider)
	}
	for k := range viewerScopedRoute {
		out = append(out, k.action+"@"+k.provider)
	}
	return out
}
