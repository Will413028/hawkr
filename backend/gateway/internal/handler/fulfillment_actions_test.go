package handler

import (
	"strings"
	"testing"
)

// 🔴 本檔在 2026-09-10（BLOCKER 2）整支重寫。
//
// 舊版把「print_label 恆等於超商端點」釘成契約：
//
//	if got.Href != "/api/v1/app/shipments/O-2026-0001/print_cvs_label" { … }
//
// 全檔 108 行**沒有任何 provider 或 actor 輸入**，於是它在
// 「黑貓單拿到超商標籤 href」這個實際造成資料污染的失效上是全綠的。
// 修 production 規則之後保留那支測試，就等於保留那個錯誤契約。

// in 建構一筆 shipping-service 的可用性投影。
//
// actors 是可變長度，因為 actor eligibility 現在是**集合**：track／view_only 由領域
// 宣告 {買家, 賣家}，seller 動作宣告 {賣家}。空集合＝領域沒宣告（fail-closed 丟棄）。
func in(action string, enabled bool, reason string, actors ...string) ActionInput {
	return ActionInput{Action: action, Enabled: enabled, DisabledReasonCode: reason, ActorAccountTypes: actors}
}

// ==================== provider 維度 ====================

// 這是 QA 在 dev 上實際量到損害的那一格：3001 黑貓單拿到指向 /print_cvs_label 的 href，
// 賣家跟隨之後 label_print_status 由 4101 → 4104（列印失敗），並進入對帳排程的無限重試。
func TestBuildAllowedActions_TaqbinNeverGetsTheCvsLabelEndpoint(t *testing.T) {
	// 🔴 要釘的不變式是「**不得指向超商端點**」，不是「不得有 print_label」。
	//    第一版釘成後者，於是修好跨 provider 誤導的同時，也讓黑貓賣家拿不到列印動作
	//    ——那是 BLOCKER 2 明列要消滅的另一半。
	actions, _ := BuildAllowedActions("O290960523", providerTaqbin, ViewerSeller, []ActionInput{
		in("print_label", true, "", ViewerSeller),
	})
	for _, a := range actions {
		if strings.Contains(a.Href, "print_cvs_label") {
			t.Fatalf("3001 拿到超商標籤端點——那條 href 對黑貓單 100%% 失敗並污染列印狀態機：%s", a.Href)
		}
	}
	if len(actions) != 1 || actions[0].Href != "/api/v1/app/shipments/O290960523/generate_home_label" {
		t.Fatalf("3001 應拿到宅配取號端點，實得 %+v", actions)
	}
}

// 反方向：3002 超商單不得拿到黑貓的 /call_tcat。
func TestBuildAllowedActions_CvsNeverGetsTheTaqbinDispatchEndpoint(t *testing.T) {
	actions, dropped := BuildAllowedActions("O874657066", providerCvs, ViewerSeller, []ActionInput{
		in("dispatch", true, "", ViewerSeller),
	})
	if len(actions) != 0 {
		t.Fatalf("3002 不得拿到 dispatch（黑貓端點），實得 %+v", actions)
	}
	if len(dropped) != 1 || dropped[0] != "dispatch(no_route_for_provider)" {
		t.Fatalf("丟棄理由要指得出是 provider 不匹配，實得 %v", dropped)
	}
}

// 陽性對照：正確的組合必須真的給得出 href——證明上面兩支不是「什麼都丟棄」造成的假綠。
func TestBuildAllowedActions_CorrectProviderPairingsAreServed(t *testing.T) {
	cases := []struct {
		provider string
		action   string
		wantHref string
	}{
		{providerTaqbin, "dispatch", "/api/v1/app/shipments/O1/call_tcat"},
		{providerCvs, "print_label", "/api/v1/app/shipments/O1/print_cvs_label"},
		{providerCvs, "handover", "/api/v1/app/shipments/O1/confirm"},
	}
	for _, tc := range cases {
		actions, dropped := BuildAllowedActions("O1", tc.provider, ViewerSeller, []ActionInput{
			in(tc.action, true, "", ViewerSeller),
		})
		if len(actions) != 1 {
			t.Fatalf("%s@%s 應被服務，實得 %+v（dropped=%v）", tc.action, tc.provider, actions, dropped)
		}
		if actions[0].Href != tc.wantHref {
			t.Fatalf("%s@%s href 應為 %s，實得 %s", tc.action, tc.provider, tc.wantHref, actions[0].Href)
		}
	}
}

// ==================== actor 維度 ====================

// 買家端不得出現賣家動作：QA 實測買家跟隨那些 href 得 403 seller access required。
func TestBuildAllowedActions_BuyerNeverReceivesSellerActions(t *testing.T) {
	actions, dropped := BuildAllowedActions("O1", providerCvs, ViewerBuyer, []ActionInput{
		in("handover", true, "", ViewerSeller),
		in("print_label", true, "", ViewerSeller),
		in("track", true, "", ViewerBuyer, ViewerSeller),
	})
	if len(actions) != 1 || actions[0].Action != "track" {
		t.Fatalf("買家只該拿到 track，實得 %+v", actions)
	}
	if len(dropped) != 2 {
		t.Fatalf("兩條賣家動作應被丟棄，實得 %v", dropped)
	}
}

// 賣家端拿得到自己的動作（同一份輸入、不同 viewer ⇒ 不同結果）。
// 🔴 這一支與上一支是同一組輸入：兩者一起才證明過濾是**依 actor**而不是依「丟掉大部分」。
func TestBuildAllowedActions_SellerReceivesSellerActions(t *testing.T) {
	actions, _ := BuildAllowedActions("O1", providerCvs, ViewerSeller, []ActionInput{
		in("handover", true, "", ViewerSeller),
		in("print_label", true, "", ViewerSeller),
		in("track", true, "", ViewerBuyer, ViewerSeller),
	})
	// 賣家拿到自己的兩個 mutation 動作 ＋ 唯讀 fallback（track 對賣家指向賣家端點）。
	if len(actions) != 3 {
		t.Fatalf("賣家應拿到 handover ＋ print_label ＋ track(賣家端點)，實得 %+v", actions)
	}
	for _, a := range actions {
		if a.Action == "track" && a.Href != "/api/v1/app/shipments/O1/tracking" {
			t.Fatalf("賣家的 track 必須指向賣家端點，實得 %s", a.Href)
		}
		if strings.HasPrefix(a.Href, "/api/v1/app/orders/") {
			t.Fatalf("賣家端不該出現買家端點：%+v", a)
		}
	}
}

// actor 缺失一律丟棄——不得預設成某一方。
func TestBuildAllowedActions_MissingActorIsDropped(t *testing.T) {
	actions, dropped := BuildAllowedActions("O1", providerCvs, ViewerSeller, []ActionInput{
		in("handover", true, ""),
	})
	if len(actions) != 0 || len(dropped) != 1 || dropped[0] != "handover(missing_actor)" {
		t.Fatalf("缺 actor 應被丟棄且指得出原因，實得 actions=%+v dropped=%v", actions, dropped)
	}
}

// ==================== 既有的兩條 fail-closed（保留） ====================

func TestBuildAllowedActions_UnknownActionIsDroppedNotGuessed(t *testing.T) {
	actions, dropped := BuildAllowedActions("O123", providerCvs, ViewerBuyer, []ActionInput{
		in("track", true, "", ViewerBuyer),
		in("some_future_action", true, "", ViewerBuyer),
	})
	if len(actions) != 1 || actions[0].Action != "track" {
		t.Fatalf("未登錄的 action 應被丟棄，實得 %+v", actions)
	}
	if len(dropped) != 1 || dropped[0] != "some_future_action(no_route_for_provider)" {
		t.Fatalf("丟棄理由不符，實得 %v", dropped)
	}
}

// 不可用卻沒帶原因碼 ⇒ 丟棄：前端無法說明原因，按鈕會憑空消失。
func TestBuildAllowedActions_DisabledWithoutReasonIsDropped(t *testing.T) {
	actions, dropped := BuildAllowedActions("O1", providerCvs, ViewerSeller, []ActionInput{
		in("handover", false, "", ViewerSeller),
	})
	if len(actions) != 0 || len(dropped) != 1 || dropped[0] != "handover(missing_reason)" {
		t.Fatalf("不可用且無原因碼應被丟棄，實得 actions=%+v dropped=%v", actions, dropped)
	}
}

// 帶了原因碼的不可用動作**必須出現**（enabled=false ＋ reason），前端才能顯示灰按鈕＋原因。
func TestBuildAllowedActions_DisabledWithReasonIsServed(t *testing.T) {
	actions, _ := BuildAllowedActions("O1", providerCvs, ViewerSeller, []ActionInput{
		in("handover", false, "9527", ViewerSeller),
	})
	if len(actions) != 1 || actions[0].Enabled || actions[0].DisabledReasonCode != "9527" {
		t.Fatalf("不可用但有原因碼者應保留，實得 %+v", actions)
	}
	if actions[0].Href == "" || actions[0].Method == "" {
		t.Fatalf("不可用的動作一樣要帶 href/method（前端可能要顯示但禁點），實得 %+v", actions[0])
	}
}

// actor 必須出現在對外契約上，而不只是內部的過濾條件。
// 🔴 沒有這一格，「後端有正確區分 actor」對前端與 QA 都只能相信、無法查。
func TestBuildAllowedActions_ActorIsPartOfTheOutwardContract(t *testing.T) {
	actions, _ := BuildAllowedActions("O1", providerCvs, ViewerSeller, []ActionInput{
		in("handover", true, "", ViewerSeller),
	})
	if len(actions) != 1 {
		t.Fatalf("預期 1 條，實得 %+v", actions)
	}
	if actions[0].Actor != ViewerSeller {
		t.Fatalf("actor 應出現在對外契約且為 %q，實得 %q", ViewerSeller, actions[0].Actor)
	}
}

// ==================== 雙身分動作（track / view_only） ====================

// 🔴 賣家必須拿得到唯讀 fallback，而且 href 要指向**賣家自己的**端點。
// 第一版把 track/view_only 一律標成 actor=買家，dev 實測 3001 賣家的 allowedActions
// 只剩一條 dispatch —— 而 view_only 的定義逐字是「兜底值：永遠可用」。
func TestBuildAllowedActions_SellerGetsReadOnlyFallbackOnItsOwnEndpoint(t *testing.T) {
	actions, dropped := BuildAllowedActions("O1", providerTaqbin, ViewerSeller, []ActionInput{
		// 領域宣告雙身分 {買家, 賣家}；gateway 只做「viewer 在不在集合內」＋依 viewer 選 href。
		in("view_only", true, "", ViewerBuyer, ViewerSeller),
	})
	if len(actions) != 1 {
		t.Fatalf("賣家應拿到 view_only，實得 %+v（dropped=%v）", actions, dropped)
	}
	if actions[0].Href != "/api/v1/app/shipments/O1/tracking" {
		t.Fatalf("賣家的 view_only 必須指向賣家端點，實得 %s", actions[0].Href)
	}
	if actions[0].Actor != ViewerSeller {
		t.Fatalf("actor 應反映實際 viewer，實得 %q", actions[0].Actor)
	}
}

// 陰性對照：同一個 action 對買家指向買家端點——證明上一支不是「一律給賣家端點」。
func TestBuildAllowedActions_BuyerGetsTheBuyerTrackingEndpoint(t *testing.T) {
	actions, _ := BuildAllowedActions("O1", providerTaqbin, ViewerBuyer, []ActionInput{
		in("track", true, "", ViewerBuyer, ViewerSeller),
	})
	if len(actions) != 1 || actions[0].Href != "/api/v1/app/orders/O1/logistics" {
		t.Fatalf("買家的 track 必須指向買家端點，實得 %+v", actions)
	}
}

// 🔴 黑貓有自己的標籤端點；不登錄 ⇒ 賣家拿不到自己該執行的動作（BLOCKER 2 的另一半）。
func TestBuildAllowedActions_TaqbinSellerGetsItsOwnLabelEndpoint(t *testing.T) {
	actions, dropped := BuildAllowedActions("O1", providerTaqbin, ViewerSeller, []ActionInput{
		in("print_label", true, "", ViewerSeller),
	})
	if len(actions) != 1 {
		t.Fatalf("3001 賣家應拿到 print_label，實得 %+v（dropped=%v）", actions, dropped)
	}
	if actions[0].Href != "/api/v1/app/shipments/O1/generate_home_label" {
		t.Fatalf("3001 的 print_label 必須指向宅配取號端點（不是超商的），實得 %s", actions[0].Href)
	}
}

// 兩個 provider 的同一個 action 必須指向不同端點——這一對才是「有 provider 維度」的證明。
func TestBuildAllowedActions_PrintLabelDiffersByProvider(t *testing.T) {
	taqbin, _ := BuildAllowedActions("O1", providerTaqbin, ViewerSeller, []ActionInput{
		in("print_label", true, "", ViewerSeller)})
	cvs, _ := BuildAllowedActions("O1", providerCvs, ViewerSeller, []ActionInput{
		in("print_label", true, "", ViewerSeller)})
	if len(taqbin) != 1 || len(cvs) != 1 {
		t.Fatalf("兩者都該有 print_label，實得 taqbin=%+v cvs=%+v", taqbin, cvs)
	}
	if taqbin[0].Href == cvs[0].Href {
		t.Fatalf("3001 與 3002 的 print_label 指到同一個端點 ⇒ provider 維度不成立：%s", taqbin[0].Href)
	}
}

// ==================== 路由表自身的形狀 ====================

// 🔴 這一支守的是「表上每一格都同時帶 action 與 provider」——
// 有人把 provider 維度拿掉（退回 per-action 常數）時，這裡會編譯失敗或斷言失敗。
func TestActionRouteTableCarriesProviderDimension(t *testing.T) {
	routes := KnownFulfillmentActionRoutes()
	if len(routes) == 0 {
		t.Fatal("路由表為空")
	}
	for _, r := range routes {
		if len(r) < 3 || r[len(r)-5:len(r)-4] != "@" {
			t.Fatalf("路由 key 應為 action@provider 形式，實得 %q", r)
		}
	}
	// 具體反例：3002 不得有 dispatch（7-11 C2C 沒有派車 API）。
	// ⚠️ 這裡**不再**斷言「3001 沒有 print_label」——黑貓有自己的標籤端點，
	//    不登錄會讓賣家拿不到動作。要防的是「指到別人的端點」，
	//    由 TestBuildAllowedActions_PrintLabelDiffersByProvider 守。
	for _, r := range routes {
		if r == "dispatch@"+providerCvs {
			t.Fatal("3002 又出現 dispatch 路由——7-11 C2C 沒有派車 API")
		}
	}
}
