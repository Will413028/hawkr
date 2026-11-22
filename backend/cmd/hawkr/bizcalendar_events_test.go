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

func TestBizCalendarRelayIsWiredIntoServeAndWorker(t *testing.T) {
	if !funcCalls(t, "main.go", "runServe")["subscribeBizCalendarRelay"] {
		t.Error("runServe 沒有呼叫 subscribeBizCalendarRelay")
	}
	worker := funcCalls(t, "main.go", "runWorker")
	for _, want := range []string{"bizcalendarrelay.NewWorker", ".AddWorkers", "bizcalendarrelay.PeriodicJobs"} {
		if !worker[want] {
			t.Errorf("runWorker 沒有呼叫 %s", want)
		}
	}
}
