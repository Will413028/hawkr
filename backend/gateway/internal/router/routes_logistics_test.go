package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// 物流財務（線 B）路由的掛載證明。
//
// 為什麼需要這一支：`TestSellerCapabilityRegistryMatchesMatrix` 只涵蓋**賣家面**
// （它比對的是 authz registry，而 admin 路由不進 registry）。七條 Admin 路由若忘了在
// registerLogisticsFinanceRoutes 裡掛上，沒有任何東西會轉紅——症狀是 ADM 打 404，
// 而後端的 handler、DTO、RPC 全部寫好了、測試也全綠。
//
// 🔴 期望值硬寫字面值（不從 constants.go 反推）：拿常數去比對用同一組常數註冊的路由，
// 是自己跟自己對——常數改了兩邊一起改，測試永遠綠。這裡寫的是**前端會打的那個字串**。
func TestLogisticsFinanceRoutesAreMounted(t *testing.T) {
	engine := buildFullRouter(t)

	mounted := map[string]bool{}
	for _, r := range engine.Routes() {
		mounted[r.Method+" "+r.Path] = true
	}

	want := []string{
		// 賣家面（W6-3 Logistics Finance）
		"GET /api/v1/app/logistics_finance",
		"GET /api/v1/app/logistics_finance/receivables",
		"GET /api/v1/app/logistics_finance/ledger",
		// Admin 面（W6-4 Financial Operations ＋ Provider Reconciliation）
		"GET /api/v1/admin/logistics/receivables",
		"GET /api/v1/admin/logistics/receivables/:receivable_no",
		"GET /api/v1/admin/logistics/collections",
		"GET /api/v1/admin/logistics/adjustments",
		"GET /api/v1/admin/logistics/provider_statements",
		"GET /api/v1/admin/logistics/provider_statements/:statement_no",
		"GET /api/v1/admin/logistics/exception_cases",
		// 責任判定 maker-checker（WP2B，65 §6.1.1 A-2／A-4／A-5）。少任何一支，責任
		// 判定案件就永遠沒有人推得走：訂單卡在 SHIPPING_ABNORMAL(35)、買家的錢無限期
		// 卡住，而全鏈路沒有任何錯誤訊息。
		"GET /api/v1/admin/logistics/exception_cases/:case_no",
		"POST /api/v1/admin/logistics/exception_cases/:case_no/resolutions",
		"POST /api/v1/admin/logistics/exception_cases/:case_no/resolutions/:resolution_id/approvals",
	}
	for _, w := range want {
		if !mounted[w] {
			t.Errorf("物流財務路由未掛載：%s —— 後端做完了但沒有人打得到", w)
		}
	}

	// 🔴 **應收與收款軸維持唯讀**：那條軸的金額與狀態只能由送達事件與 scheduler 推進，
	// 開一條 HTTP 寫入入口＝繞過 idempotency key 與 lease 的第二路徑。
	//
	// ⚠️ 例外只有責任判定 maker-checker 的兩條 POST：它們不碰 logistics_receivables 一個
	// 欄位，寫的是 fulfillment_exception_cases 與 fulfillment_case_resolutions，而那兩張
	// 表的推進本來就只有人做得到（誰該負責這次履約失敗，沒有任何事件算得出來）。
	//
	// 🔴 **逐條列白名單而不是把這段斷言拿掉**：拿掉之後，任何人都能在這個 prefix 底下
	// 新增寫入端點而沒有任何東西會轉紅——那正是本段存在的理由。
	writeAllowed := map[string]bool{
		"POST /api/v1/admin/logistics/exception_cases/:case_no/resolutions":                          true,
		"POST /api/v1/admin/logistics/exception_cases/:case_no/resolutions/:resolution_id/approvals": true,
	}
	for _, r := range engine.Routes() {
		isLogistics := strings.HasPrefix(r.Path, "/api/v1/admin/logistics") ||
			strings.HasPrefix(r.Path, "/api/v1/app/logistics_finance")
		if isLogistics && r.Method != "GET" && !writeAllowed[r.Method+" "+r.Path] {
			t.Errorf("物流財務不得有寫入端點，卻掛了 %s %s", r.Method, r.Path)
		}
	}
	// 白名單自己也要對帳：列了卻沒掛 ⇒ 白名單腐化成一句沒有人查的話，而且會靜默
	// 放行一條「名字對得上、實際不存在」的路徑。
	for k := range writeAllowed {
		if !mounted[k] {
			t.Errorf("寫入白名單列了 %s，但它沒有掛載——白名單與實況已漂移", k)
		}
	}
}

// ==================== 寫入路由的冪等鍵覆蓋 ====================
//
// 65 §6.1.4 對 A-1～A-16 的所有寫入路由逐字要求 `Idempotency-Key`。少掛一條的後果不是
// 「重送會失敗」而是**重送會成功第二次**：maker 重送多開一張提案、checker 重送多做一次
// 覆核，而兩者都是不可回復的稽核事實——事後看起來就像真的有人做了兩次。
//
// 🔴 這道 gate 的觀測窗與它擋不住的東西（`06-no-small-tails.md` §3.10）：
//
//	① 觀測窗 ＝ `routes_logistics.go` 這一個檔的 AST。把物流寫入路由註冊到別的檔就在
//	   窗外 —— 所以掃出 0 條寫入路由時是 FAIL 而不是 SKIP（見下）。
//	② 它驗的是「有沒有掛上」，不是「掛上之後有沒有效」。後者由
//	   internal/middleware/idempotency_test.go 的行為測試守。
//	③ 它驗不到「middleware 被別名 import 之後改名呼叫」——識別字比對就是這個代價。
//	   同 invoice_idempotency_coverage_test.go 的既有選擇，不在這裡自創第二套。

// logisticsWriteRoutesMissingIdempotency 掃一段 routes 原始碼，回傳「是寫入方法卻沒帶
// RequireIdempotencyKey」的註冊點，以及掃到的寫入路由總數。
//
// 吃 source 字串而不是檔名，是為了讓鑑別力測試能餵一段合成的反例進來——否則這支
// 判準自己有沒有效，沒有任何東西驗得到。
func logisticsWriteRoutesMissingIdempotency(t *testing.T, src string) (missing []string, total int) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes_logistics.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 routes_logistics.go 失敗（觀測窗不存在 ⇒ FAIL，不 SKIP）：%v", err)
	}

	writeMethods := map[string]bool{"POST": true, "PUT": true, "PATCH": true, "DELETE": true}

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			// group.POST(Path, ...) / group.PUT(...) …
			if !writeMethods[fn.Sel.Name] {
				return true
			}
			total++
			if !callArgsMentionIdempotency(call.Args) {
				missing = append(missing, fn.Sel.Name+" "+exprName(call.Args[0]))
			}
		case *ast.Ident:
			// sellerHandle(group, "POST", Path, cap, handler) 這一類 helper：method 是
			// 第二個參數的字串字面值。不處理的話，換成 helper 註冊就整條溜出觀測窗。
			if len(call.Args) < 3 || !strings.HasPrefix(fn.Name, "sellerHandle") {
				return true
			}
			if !writeMethods[stringLit(call.Args[1])] {
				return true
			}
			total++
			if !callArgsMentionIdempotency(call.Args) {
				missing = append(missing, stringLit(call.Args[1])+" "+exprName(call.Args[2]))
			}
		}
		return true
	})
	return missing, total
}

func TestLogisticsWriteRoutesAllRequireIdempotencyKey(t *testing.T) {
	src, err := os.ReadFile("routes_logistics.go")
	if err != nil {
		t.Fatalf("讀 routes_logistics.go 失敗（觀測窗不存在 ⇒ FAIL，不 SKIP）：%v", err)
	}
	missing, total := logisticsWriteRoutesMissingIdempotency(t, string(src))

	// 🔴 掃出 0 條 ⇒ FAIL。「目標不存在就靜默轉綠」是這類 gate 最常見的死法：責任判定
	// 那兩條 POST 被搬走或刪掉時，這支測試必須出聲而不是變成一個永遠通過的空迴圈。
	if total < 2 {
		t.Fatalf("routes_logistics.go 只掃到 %d 條寫入路由，預期至少 2 條（A-4 提案／A-5 覆核）"+
			"——它們被搬走或刪掉了，這道 gate 已失去觀測對象", total)
	}
	for _, m := range missing {
		t.Errorf("物流寫入路由未掛 %s：%s —— 重送會多開一張提案／多做一次覆核，"+
			"而那是不可回復的稽核事實", idempotencyMiddlewareIdent, m)
	}
}

// TestLogisticsIdempotencyGateHasDiscrimination 證明上面那支判準抓得到它該抓的東西。
// 少了這支，「全部都掛了」與「判準壞掉永遠回空」在輸出上完全一樣。
func TestLogisticsIdempotencyGateHasDiscrimination(t *testing.T) {
	const withoutGuard = `package router

func registerX(admin *gin.RouterGroup, h *handler.Holder) {
	admin.GET(AdminLogisticsExceptionCasesPath, h.List)
	admin.POST(AdminLogisticsCaseResolutionsPath, h.Propose)
	admin.POST(AdminLogisticsCaseResolutionApprovals, middleware.RequireIdempotencyKey(), h.Review)
}
`
	missing, total := logisticsWriteRoutesMissingIdempotency(t, withoutGuard)
	if total != 2 {
		t.Fatalf("合成案例應掃到 2 條寫入路由，實得 %d —— 判準連數都數不對", total)
	}
	if len(missing) != 1 || !strings.Contains(missing[0], "AdminLogisticsCaseResolutionsPath") {
		t.Fatalf("判準沒抓到那條沒掛守衛的 POST；missing = %v", missing)
	}
}
