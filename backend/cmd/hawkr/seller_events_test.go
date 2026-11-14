package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"testing"
)

// subscribeSellerRelay 訂閱的 relay 事件只有店家生命週期、停權、資料與曝光五種：平台審核（核准、退回）與收款開通的結果
// 都不轉成舊的 merchant.verification.*（舊通知服務沒有部署，結果通知由通知模組承接）。
func TestSellerRelaySubscribesOnlyTheStoreEvents(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "seller_events.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "subscribeSellerRelay" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "sellerrelay" {
					got = append(got, sel.Sel.Name)
				}
			}
			return true
		})
		return false
	})
	slices.Sort(got)
	want := []string{"OnStoreExposureChanged", "OnStoreProfileChanged", "OnStoreRetired", "OnStoreSuspended", "OnStoreUnsuspended"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subscribeSellerRelay 訂閱 %v，預期 %v", got, want)
	}
}
