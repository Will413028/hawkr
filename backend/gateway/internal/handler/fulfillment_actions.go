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

// routeKey 是 (action, provider) —— 🔴 **provider 是不可省略的維度**。
//
// 修復前這張表只以 action 為 key，而端點是 per-provider 的：
//   - 3002 超商單拿到 dispatch → href 指向黑貓 /call_tcat
//   - 3001 黑貓單拿到 print_label → href 指向超商 /print_cvs_label
//
// 後者在 dev 上造成實際損害：賣家跟隨該 href，訂單被推進 label_print_status=4104（列印失敗），
// 而 4104 落在對帳排程的索引述詞內 ⇒ 反覆重試、每次都因缺參數而失敗。
//
// 當時的辯護是「前端契約在 href 出現的那一刻就已經 provider-neutral 了」——
// 那個論證只在「名字」這一層成立：neutral 的是名字，錯的是目的地。
type routeKey struct {
	action   string
	provider string // ShippingMethod 值域（"3001" 黑貓／"3002" 超商／…）
}

type routeTarget struct {
	method string
	path   string // {order_no} 會被替換
}

// 配送方式（ShippingMethod 值域）——與 CodeMeta 同一組值。
const (
	providerTaqbin   = "3001" // 黑貓宅配
	providerCvs      = "3002" // 7-11 交貨便 C2C
	providerCvsB2B   = "3004" // 7-11 大宗寄倉（本波未啟用）
	providerMerchant = "3005" // 商家自送（本波未啟用）
)

// actionRoute：(action, provider) → (HTTP method, 路徑樣板)。
//
// 🔴 這張表為什麼在 Gateway 而不在 shipping-service：
// 「這顆按鈕現在能不能按、誰能按」是業務判斷（微服務算，含 actor），
// 「它對應哪支 URL」是**路由知識**。Gateway 是唯一對外入口、路由表在它身上。
//
// ⚠️ 路徑樣板仍映射到現況的 provider-specific 端點。65 §6.2 的目標是 neutral 端點
// （S-10 /dispatch、S-11 /cancel、S-12 /pod），那需要 shipping-service 先有對應的 RPC。
// 在那之前，**正確性由這張表的 provider 維度承擔**：沒有登錄的組合一律丟棄，不猜路徑。
var actionRoute = map[routeKey]routeTarget{
	// ── 黑貓宅配 ──────────────────────────────────────────────
	{"dispatch", providerTaqbin}: {"POST", "/api/v1/app/shipments/{order_no}/call_tcat"},
	// 🔴 黑貓有**自己的**標籤端點（`sellerHandle(... CapOrderFulfill ...)` 掛在
	//    `/generate_home_label`）。第一版把 print_label@3001 整格拿掉，理由是
	//    「不登錄比登錯好」——那擋住了「黑貓單拿到超商端點」，卻同時造成
	//    **賣家拿不到自己該執行的列印動作**，而那正是 BLOCKER 2 明列要消滅的另一半。
	//    正解是登錄到對的端點，不是不登錄。
	{"print_label", providerTaqbin}: {"POST", "/api/v1/app/shipments/{order_no}/generate_home_label"},

	// ── 7-11 交貨便 C2C ──────────────────────────────────────
	{"handover", providerCvs}:    {"POST", "/api/v1/app/shipments/{order_no}/confirm"},
	{"print_label", providerCvs}: {"POST", "/api/v1/app/shipments/{order_no}/print_cvs_label"},
	// ⚠️ 3002 **沒有** dispatch：7-11 C2C 沒有派車 API（adapter 的 dispatch() 回 empty）。

	// ── 追蹤與兜底 ────────────────────────────────────────────
	// 🔴 這兩個動作**買賣雙方都有**，但 href 不同端點：
	//    買家走 `/app/orders/{no}/logistics`（JWTAuth），賣家走 `/app/shipments/{no}/tracking`
	//    （requireSeller + CapOrderFulfill）。第一版把它們一律標成 actor=買家，
	//    後果是**賣家端永遠沒有唯讀 fallback**——而 `view_only` 的定義逐字是
	//    「兜底值：永遠可用」。dev 實測：3001 賣家的 allowedActions 只剩一條 dispatch。
	//    ⇒ 兜底動作的 href 必須依 viewer 決定，不是把 actor 綁死。
}

// viewerScopedRoute 是「同一個 action 對不同身分指向不同端點」的那一類。
//
// 它與 actionRoute 分開，是因為兩者的失效方式不同：
//   - actionRoute 漏一格 ⇒ 動作消失（可見、可回報）
//   - 這一張漏一格 ⇒ 動作指到**另一個身分的端點** ⇒ 403（看起來像權限壞了）
var viewerScopedRoute = map[routeKey]map[string]routeTarget{
	{"track", providerTaqbin}:   trackTargets(),
	{"track", providerCvs}:      trackTargets(),
	{"track", providerCvsB2B}:   trackTargets(),
	{"track", providerMerchant}: trackTargets(),

	{"view_only", providerTaqbin}:   trackTargets(),
	{"view_only", providerCvs}:      trackTargets(),
	{"view_only", providerCvsB2B}:   trackTargets(),
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
		// ① actor eligibility —— **唯一**判定，來源是 shipping 的宣告，不是本檔的路由表。
		if len(a.ActorAccountTypes) == 0 {
			dropped = append(dropped, a.Action+"(missing_actor)")
			continue
		}
		if !slices.Contains(a.ActorAccountTypes, viewer) {
			dropped = append(dropped, a.Action+"(actor_mismatch)")
			continue
		}

		// ② 路由 —— 純轉譯。雙身分動作在 viewerScopedRoute 依 viewer 取 href，
		//    單身分動作在 actionRoute。兩張表都只回答「href 是什麼」，不回答「誰能做」。
		key := routeKey{a.Action, provider}
		target, ok := resolveTarget(key, viewer)
		if !ok {
			dropped = append(dropped, a.Action+"(no_route_for_provider)")
			continue
		}

		// ③ 不可用必須帶原因碼，否則按鈕會憑空消失。
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
			// 回應已依 viewer 過濾，這裡填 viewer 即「這條動作是給你的」。
			Actor: viewer,
		})
	}
	return actions, dropped
}

// resolveTarget 只回答 href——**純路由，不做資格判定**。
//
// 呼叫端必須已經確認 viewer 有資格（BuildAllowedActions 的 ①）。這個分工是 R4 的重點：
// 先前這兩件事綁在 viewerScopedRoute 的 key 上，於是「查不到路由」與「你沒資格」
// 是同一個判斷，而後者本該由領域決定。
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
	// 🔴 雙身分那張表也要列進來：漏了的話「路由表形狀」的契約測試看不到它們，
	//    而它們正是最容易指錯端點的一類（指到另一個身分的端點 ⇒ 403）。
	for k := range viewerScopedRoute {
		out = append(out, k.action+"@"+k.provider)
	}
	return out
}
