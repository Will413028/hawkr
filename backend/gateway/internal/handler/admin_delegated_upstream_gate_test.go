package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 代行 handler 的「上游呼叫失敗」分支必須還原業務碼——機械閘門。
//
// 為什麼要一支掃原始碼的測試，而不是再多寫幾支個案測試：這個 bug class 已經發作兩次。
// 第一次修的是 admin_delegation.go（TestDelegationGrpcFailure_RestoresBizCode 就是那次
// 留下的），但 admin_delegated_audit.go 的三個呼叫點從未一起轉換，於是同一個形狀
// ——`if err != nil { response.BizStatus(c, bizcode.DelegationUpstreamUnavailable, …) }`
// ——又活了一次，而且活到上線：查無的 sessionId 回 424／9280「服務暫時不可用，請稍後
// 再試」，而稍後再試永遠不會成功。
//
// 個案測試守得住已經被想到的那幾支；守不住「下一支新加的 handler 又抄了同一個形狀」。
// 這支守的是形狀本身。
//
// 判準（可機械執行）：凡是把某個 gRPC client（`h.Account` / `h.Product` / …）的回傳指派
// 給 `err`，其 `if err != nil` 分支必須呼叫 delegationGrpcFailure 或 denialFromGrpcErr
// ——它們是把 EnvelopeGuardInterceptor 加在 message 前的 "[NNNN] " 前綴還原成 envelope
// code 的唯二出口。直接寫 bizcode.DelegationUpstreamUnavailable 一律視為違規。
//
// ⚠️ `h.Account == nil` 那種 nil-guard 不在掃描範圍內：它不是 `if err != nil` 分支。
// ⚠️ **但它們的碼自 #2744 起是 9289 不是 9280**，本節原本寫著「上游未接線本來就該回
// 9280」——那句話把兩件事混在一起了：上游抖動重試會好，而「這個 build 根本沒把依賴
// wire 起來」重試一萬次都一樣。兩者外部看起來完全相同（424 上升、上游查無異常），
// 差別只有碼講不講真話。九個 nil-guard 已全部改為 9289 並補上
// `Delegation.dependency.unwired` 的 error log —— 其中五個先前連一行 log 都沒有。
//
// ── 掃描範圍的兩個歷史盲區，兩者都已由本檔的推導方式關掉 ────────────────────────
//
// ① **client 白名單**：原本只認 `h.Account` 與 `h.Media`，而四類資源端點的 18 個呼叫點
// 全部走 `h.Social`／`h.Seller`／`h.Product`／`h.Order` —— 也就是說主體完全不在守備範圍
// 內。手抄的白名單漏一個 client，就對那個 client 的所有呼叫點靜默失明。現在 client 集合
// 由 holder.go 的 Holder 型別推導（見 delegationGrpcClientFields），新增 client 自動納管。
//
// ② **檔名 glob**：原本以 `admin_delegation*.go` ＋ `admin_delegated_*.go` 決定掃描範圍，
// 於是一個叫 `delegated_review.go` 的新檔寫同樣的違規形狀，閘門靜默全綠。現在範圍由
// **函式自身的代行標記**決定（見 delegationMarkedFunc），與檔名無關；舊的 glob 只留作
// tripwire——它匹配到的檔案若一個代行函式都認不出來，代表標記判定本身壞了。

// delegationUpstreamBranchExceptions 是具名例外，鍵為 handler 函式名。
// 每一條都必須寫得出「為什麼還原業務碼在這一支是錯的」。
var delegationUpstreamBranchExceptions = map[string]string{
	// EndCurrentDelegatedOperation：它的失敗情境表只有 9280 一列。把上游的業務碼原樣
	// 分流出去會讓「credential 無法解析」變成 401／410，而那正是本端點明令要回 200 的
	// 情境——回 401/404 會把工作區的結束動作永久封鎖。原始碼該處已寫明同一段理由。
	"EndCurrentDelegatedOperation": "失敗情境表只有 9280 一列；分流業務碼會永久封鎖結束動作",
}

// delegationLegacyGlobs 是本閘門早期用來圈定掃描範圍的檔名樣式。範圍判定已改為語意式
// （見檔頭盲區 ②），這兩個樣式只剩 tripwire 的作用：它們匹配到的檔案是**已知**的代行
// 檔，若其中某一個一個代行函式都認不出來，那不是「這個檔不再是代行」，而是標記判定失準。
var delegationLegacyGlobs = []string{"admin_delegation*.go", "admin_delegated_*.go"}

// delegationPackageSources 回傳本 package 內全部非測試的 .go 檔。
//
// 不做任何檔名過濾：範圍由函式自己的代行標記決定，而不是由誰記得把檔案取對名字決定。
func delegationPackageSources(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatal("掃描範圍為空——目錄讀取失效時本閘門會靜默全綠")
	}
	return out
}

// delegationGrpcClientFields 從 holder.go 推導出 Holder 上所有 gRPC client 欄位名。
//
// 為什麼不手寫清單：手寫的那一份漏掉的四個 client（Social／Seller／Product／Order）
// 承載了四類資源端點的全部 18 個上游呼叫點，而漏掉的症狀是全綠。從型別推導之後，
// 「有沒有記得回來加一行」這件事不再是正確性的前提。
//
// 判準是欄位型別為 `*grpcclient.XxxClient`：Holder 上的其他欄位（storeid／buyerid
// resolver、DelegationConfig、SSE hub、兩個字串）都不是 gRPC 呼叫，它們的 err 沒有
// "[NNNN] " 前綴可還原，掃進來只會製造假違規。
func delegationGrpcClientFields(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "holder.go", nil, 0)
	if err != nil {
		t.Fatalf("parse holder.go: %v", err)
	}
	out := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "Holder" {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, field := range st.Fields.List {
			star, ok := field.Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			sel, ok := star.X.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "grpcclient" {
				continue
			}
			for _, name := range field.Names {
				out[name.Name] = true
			}
		}
		return false
	})
	// 自我量測：推導失準最可能的症狀同樣是「一個都沒推到而全綠」。Account 與 Media 是
	// 本閘門建立時就在守的兩個，它們不在集合裡就代表解析壞了。
	if !out["Account"] || !out["Media"] {
		t.Fatalf("從 holder.go 推導出的 gRPC client 欄位不含 Account／Media（推到 %v）——解析八成失準了", delegationSortedKeys(out))
	}
	return out
}

// delegationUpstreamCallee 回傳 `h.<Client>.<Method>` 形式呼叫的 "Client.Method"，
// 其餘回空字串。clients 是 delegationGrpcClientFields 推導出的集合。
func delegationUpstreamCallee(e ast.Expr, clients map[string]bool) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	method, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	client, ok := method.X.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	holder, ok := client.X.(*ast.Ident)
	if !ok || holder.Name != "h" {
		return ""
	}
	if !clients[client.Sel.Name] {
		return ""
	}
	return client.Sel.Name + "." + method.Sel.Name
}

// delegationMarkedFunc 判斷一個函式是否在做代行的事。這取代了原本的檔名 glob：
// 一支新加的代行 handler 不論檔案叫什麼名字都會被掃到。
//
// 五個標記彼此獨立，命中任一即納入。它們不是「風格線索」而是代行路徑的必要條件：
//   - 代行的拒絕碼（bizcode.Delegation*）——每一支代行 handler 至少會用到一個
//   - resolveDelegatedRequest／denyDelegated／bindDelegatedBody——代行請求解析的共用底座
//   - middleware.DelegationSession——credential 解出來的工作階段快照，媒體那兩支只有這個
//   - h.orchestrator()——兩階段稽核的入口，所有代行 mutation 都經過
//   - 呼叫的 RPC 名稱含 "Delegation"（含 *ForDelegation 全族）
//
// ⚠️ denialFromGrpcErr／delegationGrpcFailure 也算標記，但它們是**合規**的那一側；
// 一支違規的新 handler 正是不會呼叫它們，所以判定不能只靠這兩個。
func delegationMarkedFunc(fn *ast.FuncDecl, clients map[string]bool) bool {
	marked := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if marked {
			return false
		}
		switch node := n.(type) {
		case *ast.SelectorExpr:
			pkg, ok := node.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch pkg.Name {
			case "bizcode":
				if strings.HasPrefix(node.Sel.Name, "Delegation") {
					marked = true
				}
			case "middleware":
				if node.Sel.Name == "DelegationSession" {
					marked = true
				}
			case "h":
				if node.Sel.Name == "orchestrator" {
					marked = true
				}
			}
		case *ast.Ident:
			switch node.Name {
			case "resolveDelegatedRequest", "denyDelegated", "bindDelegatedBody",
				"denialFromGrpcErr", "delegationGrpcFailure":
				marked = true
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if callee := delegationUpstreamCallee(call, clients); strings.Contains(callee, "Delegation") {
				marked = true
			}
		}
		return true
	})
	return marked
}

// delegationAssignsErr 回傳該指派是否把某個回傳值放進名為 err 的變數。
func delegationAssignsErr(a *ast.AssignStmt) bool {
	for _, lhs := range a.Lhs {
		if id, ok := lhs.(*ast.Ident); ok && id.Name == "err" {
			return true
		}
	}
	return false
}

// delegationIsErrNotNil 比對 `err != nil`。
func delegationIsErrNotNil(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	x, ok := bin.X.(*ast.Ident)
	if !ok || x.Name != "err" {
		return false
	}
	y, ok := bin.Y.(*ast.Ident)
	return ok && y.Name == "nil"
}

// delegationBranchFacts 取出一個分支內呼叫到的函式名，以及它有沒有直接寫死 9280。
func delegationBranchFacts(body *ast.BlockStmt) (calls map[string]bool, flat9280 bool) {
	calls = map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			switch fn := node.Fun.(type) {
			case *ast.Ident:
				calls[fn.Name] = true
			case *ast.SelectorExpr:
				if pkg, ok := fn.X.(*ast.Ident); ok {
					calls[pkg.Name+"."+fn.Sel.Name] = true
				} else {
					calls[fn.Sel.Name] = true
				}
			}
		case *ast.SelectorExpr:
			if pkg, ok := node.X.(*ast.Ident); ok &&
				pkg.Name == "bizcode" && node.Sel.Name == "DelegationUpstreamUnavailable" {
				flat9280 = true
			}
		}
		return true
	})
	return calls, flat9280
}

// delegationUpstreamSite 是一個「呼叫上游 → 檢查 err」的配對。
type delegationUpstreamSite struct {
	file   string
	line   int
	fn     string
	callee string
	guard  *ast.BlockStmt // nil ＝ 這個 err 根本沒有被 `if err != nil` 檢查
}

func TestDelegatedHandlers_UpstreamErrorBranchesRestoreBizCode(t *testing.T) {
	clients := delegationGrpcClientFields(t)
	fset := token.NewFileSet()
	var sites []delegationUpstreamSite
	markedFiles := map[string]bool{}

	for _, path := range delegationPackageSources(t) {
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !delegationMarkedFunc(fn, clients) {
				continue
			}
			markedFiles[path] = true
			collectDelegationUpstreamSites(fset, path, fn, clients, &sites)
		}
	}

	// 舊 glob 只剩 tripwire：它匹配到的都是已知的代行檔，其中若有一個檔連一個代行函式
	// 都認不出來，那是 delegationMarkedFunc 失準，不是那個檔不再代行了。
	for _, pattern := range delegationLegacyGlobs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, p := range matches {
			if strings.HasSuffix(p, "_test.go") || markedFiles[p] {
				continue
			}
			t.Errorf("%s 命中代行檔名樣式，卻沒有任何函式被認定為代行 —— "+
				"delegationMarkedFunc 的標記判定八成失準了，此時的綠燈是假的", p)
		}
	}

	// 這個下界是 ratchet，也是本閘門的第二個自我量測：比對器一旦失準（Holder 欄位改名、
	// 呼叫形狀改了、標記判定壞了）最可能的症狀是「一個都沒掃到而全綠」。
	//
	// 35 ＝ 本次盤點的實際命中數（Account 15／Product 7／Social 5／Seller 4／Media 2／
	// Order 2）。下界訂在 30 而不是 35：刪掉幾支代行端點是正常的維護，不該連帶把這支測試
	// 弄紅；但一次掉超過 5 個就不是「刪了幾支端點」的量級，那是比對器整組失準。
	const minSites = 30
	if len(sites) < minSites {
		t.Fatalf("只掃到 %d 個上游呼叫點（< %d）——比對器八成失準了，此時的綠燈是假的", len(sites), minSites)
	}

	usedExceptions := map[string]bool{}
	for _, s := range sites {
		if s.guard == nil {
			t.Errorf("%s:%d %s：%s 的 err 沒有任何 `if err != nil` 檢查", s.file, s.line, s.fn, s.callee)
			continue
		}
		calls, flat9280 := delegationBranchFacts(s.guard)
		if _, exempt := delegationUpstreamBranchExceptions[s.fn]; exempt {
			usedExceptions[s.fn] = true
			continue
		}
		if flat9280 {
			t.Errorf("%s:%d %s：%s 的 err 分支直接寫 bizcode.DelegationUpstreamUnavailable —— "+
				"上游的業務碼是夾在 gRPC status message 的 \"[NNNN] \" 前綴裡回來的，"+
				"壓成 9280 會把「工作階段不存在」講成「服務暫時不可用，請稍後再試」",
				s.file, s.line, s.fn, s.callee)
			continue
		}
		if !calls["delegationGrpcFailure"] && !calls["denialFromGrpcErr"] {
			t.Errorf("%s:%d %s：%s 的 err 分支沒有經過 delegationGrpcFailure 或 denialFromGrpcErr —— "+
				"這兩支是把 \"[NNNN] \" 前綴還原成 envelope code 的唯二出口（實際呼叫到：%v）",
				s.file, s.line, s.fn, s.callee, sortedCallNames(calls))
		}
	}

	// 例外清單本身也要被守：一條不再對應任何呼叫點的例外，會在下一個同名 handler
	// 出現時變成一個沒人記得為什麼存在的豁免。
	for fn := range delegationUpstreamBranchExceptions {
		if !usedExceptions[fn] {
			t.Errorf("例外 %q 已不對應任何上游呼叫點，請一併移除", fn)
		}
	}

	t.Logf("掃過 %d 個上游呼叫點（%d 個檔），具名例外 %d 個；納管的 gRPC client %d 個",
		len(sites), len(markedFiles), len(usedExceptions), len(clients))
}

// collectDelegationUpstreamSites 走一個 func 的每一個 block，配對「上游呼叫」與
// 緊接其後的 `if err != nil`，兩種寫法都收（獨立指派＋if，以及 if 的 Init 內指派）。
func collectDelegationUpstreamSites(fset *token.FileSet, path string, fn *ast.FuncDecl, clients map[string]bool, out *[]delegationUpstreamSite) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			// 形式一：`if _, err := h.Account.X(); err != nil { … }`
			if ifStmt, ok := stmt.(*ast.IfStmt); ok && ifStmt.Init != nil {
				if assign, ok := ifStmt.Init.(*ast.AssignStmt); ok && delegationAssignsErr(assign) {
					if callee := delegationUpstreamCalleeOf(assign, clients); callee != "" {
						site := delegationUpstreamSite{
							file: path, line: fset.Position(assign.Pos()).Line,
							fn: fn.Name.Name, callee: callee,
						}
						if delegationIsErrNotNil(ifStmt.Cond) {
							site.guard = ifStmt.Body
						}
						*out = append(*out, site)
					}
				}
				continue
			}
			// 形式二：`resp, err := h.Account.X(…)` 後面跟一個 `if err != nil { … }`
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok || !delegationAssignsErr(assign) {
				continue
			}
			callee := delegationUpstreamCalleeOf(assign, clients)
			if callee == "" {
				continue
			}
			site := delegationUpstreamSite{
				file: path, line: fset.Position(assign.Pos()).Line,
				fn: fn.Name.Name, callee: callee,
			}
			if i+1 < len(block.List) {
				if ifStmt, ok := block.List[i+1].(*ast.IfStmt); ok &&
					ifStmt.Init == nil && delegationIsErrNotNil(ifStmt.Cond) {
					site.guard = ifStmt.Body
				}
			}
			*out = append(*out, site)
		}
		return true
	})
}

func delegationUpstreamCalleeOf(a *ast.AssignStmt, clients map[string]bool) string {
	for _, rhs := range a.Rhs {
		if callee := delegationUpstreamCallee(rhs, clients); callee != "" {
			return callee
		}
	}
	return ""
}

func sortedCallNames(calls map[string]bool) []string {
	return delegationSortedKeys(calls)
}

func delegationSortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
