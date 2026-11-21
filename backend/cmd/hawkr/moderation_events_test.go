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
