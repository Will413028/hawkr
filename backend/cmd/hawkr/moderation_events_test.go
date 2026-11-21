package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"testing"
)

// subscribeModeration 訂閱的處理函式恰好是買家刪除中、買家刪除與店家退場三個：少訂一個，該身分的檢舉人資料就不會在同 transaction 清掉。
func TestModerationSubscribesTheBuyerAndStoreRetentionEvents(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "moderation.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "subscribeModeration" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Subscribe" && len(call.Args) == 2 {
				if h, ok := call.Args[1].(*ast.SelectorExpr); ok {
					if recv, ok := h.X.(*ast.Ident); !ok || recv.Name != "r" {
						t.Errorf("訂閱的處理函式 %s 的接收者要是參數 r", h.Sel.Name)
					}
					got = append(got, h.Sel.Name)
				}
			}
			return true
		})
		return false
	})
	slices.Sort(got)
	want := []string{"OnBuyerDeleted", "OnBuyerDeletionRequested", "OnStoreRetired"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subscribeModeration 訂閱 %v，預期 %v", got, want)
	}
}

// 每個建 events.Bus 的啟動路徑（serve 與 worker）都要呼叫 subscribeModeration：少一條路徑，該行程發出的事件就不會在同 transaction 清檢舉人資料。
func TestEveryBusConstructionPathSubscribesModeration(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	paths := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var newBus, subscribes bool
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch f := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := f.X.(*ast.Ident); ok && x.Name == "events" && f.Sel.Name == "NewBus" {
					newBus = true
				}
			case *ast.Ident:
				if f.Name == "subscribeModeration" {
					subscribes = true
				}
			}
			return true
		})
		if !newBus {
			continue
		}
		paths++
		if !subscribes {
			t.Errorf("%s 建了 events.Bus 卻沒有呼叫 subscribeModeration", fn.Name.Name)
		}
	}
	if paths < 2 {
		t.Fatalf("main.go 只找到 %d 條建 events.Bus 的路徑，預期至少 serve 與 worker 兩條", paths)
	}
}
