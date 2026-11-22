package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func funcCalls(t *testing.T, path, name string) (calls map[string]bool) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls = map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch f := call.Fun.(type) {
			case *ast.Ident:
				calls[f.Name] = true
			case *ast.SelectorExpr:
				if pkg, ok := f.X.(*ast.Ident); ok {
					calls[pkg.Name+"."+f.Sel.Name] = true
				} else {
					calls["."+f.Sel.Name] = true
				}
			}
			return true
		})
		return calls
	}
	t.Fatalf("%s 沒有 %s", path, name)
	return nil
}

// serve 要訂閱匯入事件；worker 要登記推送的 worker、建立 invoice client 的 worker，並加入每日對帳的週期 job：少任何一個，
// 匯入或對帳就不會推送給 invoice-service。
func TestBizCalendarRelaySubscribesTheImportHandler(t *testing.T) {
	got := funcCalls(t, "bizcalendar_events.go", "subscribeBizCalendarRelay")
	if !got["events.Subscribe"] || !got["bizcalendarrelay.OnCalendarImported"] {
		t.Fatalf("subscribeBizCalendarRelay 的呼叫 = %v", got)
	}
}

func isSel(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == pkg && sel.Sel.Name == name
}

func isCallOf(e ast.Expr, pkg, name string) bool {
	call, ok := e.(*ast.CallExpr)
	return ok && isSel(call.Fun, pkg, name)
}

// runWorker 要把 bizcalendarrelay.NewWorker 建出的 worker 自己登記（語句本身是 AddWorkers 的呼叫，不是丟掉結果的賦值），
// 並把 bizcalendarrelay.PeriodicJobs() 的結果接進 periodic，且 periodic 在那之後傳給 jobs.NewWorker。
func TestBizCalendarRelayIsWiredIntoServeAndWorker(t *testing.T) {
	if !funcCalls(t, "main.go", "runServe")["subscribeBizCalendarRelay"] {
		t.Error("runServe 沒有呼叫 subscribeBizCalendarRelay")
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "runWorker" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("main.go 沒有 runWorker")
	}
	var registered bool
	var appendedAt, passedAt token.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.ExprStmt:
			if call, ok := s.X.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "AddWorkers" && isCallOf(sel.X, "bizcalendarrelay", "NewWorker") {
					registered = true
				}
			}
		case *ast.AssignStmt:
			if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
				return true
			}
			lhs, ok := s.Lhs[0].(*ast.Ident)
			call, isCall := s.Rhs[0].(*ast.CallExpr)
			if !ok || !isCall || lhs.Name != "periodic" || len(call.Args) != 2 || !call.Ellipsis.IsValid() {
				return true
			}
			if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "append" && isCallOf(call.Args[1], "bizcalendarrelay", "PeriodicJobs") {
				appendedAt = s.Pos()
			}
		case *ast.CallExpr:
			if isSel(s.Fun, "jobs", "NewWorker") && len(s.Args) == 4 {
				if arg, ok := s.Args[3].(*ast.Ident); ok && arg.Name == "periodic" {
					passedAt = s.Pos()
				}
			}
		}
		return true
	})
	if !registered {
		t.Error("runWorker 沒有對 bizcalendarrelay.NewWorker(...) 建出的 worker 呼叫 AddWorkers")
	}
	if !appendedAt.IsValid() {
		t.Error("runWorker 沒有 periodic = append(periodic, bizcalendarrelay.PeriodicJobs()...)")
	}
	if !passedAt.IsValid() {
		t.Error("runWorker 沒有把 periodic 當第四個參數傳給 jobs.NewWorker")
	}
	if appendedAt.IsValid() && passedAt.IsValid() && appendedAt > passedAt {
		t.Error("bizcalendarrelay.PeriodicJobs() 要在 jobs.NewWorker 之前加進 periodic")
	}
}
