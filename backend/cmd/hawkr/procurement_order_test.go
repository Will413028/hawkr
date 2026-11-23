package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// BuyerDeleted 的訂閱者各自鎖表，訂閱（也就是執行）順序必須服從 lockorder.Modules：procurement 在 moderation 之前（單體規則 4）。
func TestProcurementSubscribesBeforeModeration(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runServe", "runWorker"} {
		procurement, moderation := -1, -1
		found := false
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			found = true
			i := 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := call.Fun.(*ast.Ident); ok {
					switch id.Name {
					case "subscribeProcurement":
						procurement = i
					case "subscribeModeration":
						moderation = i
					}
				}
				i++
				return true
			})
		}
		if !found || procurement < 0 || moderation < 0 {
			t.Fatalf("%s：找不到 subscribeProcurement 或 subscribeModeration 的呼叫", name)
		}
		if procurement > moderation {
			t.Errorf("%s：subscribeProcurement 在 subscribeModeration 之後，違反 lockorder.Modules", name)
		}
	}
}

// worker 角色要登記採購 run 的 reconciler 與事件清除，並把它們的排程交給 River：漏了其中一個，job 死了的 run 就沒有人標失敗（A9-D6、A9-D22）。
func TestWorkerRoleRegistersProcurementJobs(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var registers, schedules bool
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "runWorker" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch f := call.Fun.(type) {
			case *ast.Ident:
				registers = registers || f.Name == "addProcurementWorkers"
			case *ast.SelectorExpr:
				if x, ok := f.X.(*ast.Ident); ok && x.Name == "procurementModule" && f.Sel.Name == "PeriodicJobs" {
					schedules = true
				}
			}
			return true
		})
	}
	if !registers || !schedules {
		t.Fatalf("runWorker：addProcurementWorkers = %v，procurementModule.PeriodicJobs = %v", registers, schedules)
	}
}

func calls(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == name {
				found = true
			}
		}
		return true
	})
	return found
}

// WatchRun 的串流在 serve 角色：沒啟動共用的 LISTEN，串流永遠收不到喚醒；worker 角色不需要它。
func TestServeRoleStartsTheProcurementHubAndWorkerRoleDoesNot(t *testing.T) {
	if !calls(funcBody(t, "runServe"), "startProcurementHub") {
		t.Error("runServe 沒有呼叫 startProcurementHub")
	}
	if calls(funcBody(t, "runWorker"), "startProcurementHub") {
		t.Error("runWorker 不該啟動 Hub")
	}
}

// 送出與取消要排入、取消 River job：serve 與 worker 組模組時都得給 job client，不能是 nil。
func TestProcurementModuleIsBuiltWithTheJobClientInBothRoles(t *testing.T) {
	for _, name := range []string{"runServe", "runWorker"} {
		withJobs := false
		ast.Inspect(funcBody(t, name), func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "newProcurement" || id.Name == "newProcurementWorker") && len(call.Args) >= 3 {
				if a, ok := call.Args[1].(*ast.Ident); ok && a.Name == "jc" {
					withJobs = true
				}
			}
			return true
		})
		if !withJobs {
			t.Errorf("%s：newProcurement 沒有帶 jc", name)
		}
	}
}

// SubmitTurn 要讀買家額度：serve 與 worker 組採購模組時都得給買家模組當 Entitlements，不能是 nil（nil 會讓每次送出都回內部錯誤）。
func TestProcurementModuleIsBuiltWithTheBuyerModuleAsEntitlementsInBothRoles(t *testing.T) {
	for _, name := range []string{"runServe", "runWorker"} {
		wired := false
		ast.Inspect(funcBody(t, name), func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "newProcurement" || id.Name == "newProcurementWorker") && len(call.Args) >= 3 {
				if a, ok := call.Args[2].(*ast.Ident); ok && a.Name == "buyerModule" {
					wired = true
				}
			}
			return true
		})
		if !wired {
			t.Errorf("%s：newProcurement 沒有帶 buyerModule 當 Entitlements", name)
		}
	}
}

// StoreRetired 的訂閱者要在 serve 與 worker 都訂，且 imageai 在 moderation 之前（單體規則 4 的 lockorder）。
func TestBothRolesSubscribeImageAIBeforeModeration(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runServe", "runWorker"} {
		imageAI, moderation := -1, -1
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			i := 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok {
						switch id.Name {
						case "subscribeImageAI":
							imageAI = i
						case "subscribeModeration":
							moderation = i
						}
					}
					i++
				}
				return true
			})
		}
		if imageAI < 0 || moderation < 0 {
			t.Fatalf("%s：找不到 subscribeImageAI 或 subscribeModeration 的呼叫", name)
		}
		if imageAI > moderation {
			t.Errorf("%s：subscribeImageAI 在 subscribeModeration 之後，違反 lockorder.Modules", name)
		}
	}
}
