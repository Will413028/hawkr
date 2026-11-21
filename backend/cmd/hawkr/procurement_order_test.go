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

// worker 角色要登記採購 run 的 sweeper 與事件清除，並把它們的排程交給 River：漏了其中一個，逾時的 run 就沒有人標失敗（A9-D6）。
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
