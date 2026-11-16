package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"testing"
)

// subscribeSocialRelay 只訂閱發布、留言、回覆、兩種讚與措施建立：貼文建立（含草稿）、修改、排程等事件不訂閱，所以建立草稿不送 post.created。
func TestSocialRelaySubscribesOnlyTheNotificationEvents(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "social_events.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "subscribeSocialRelay" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "socialrelay" {
					got = append(got, sel.Sel.Name)
				}
			}
			return true
		})
		return false
	})
	slices.Sort(got)
	want := []string{"OnCommentCreated", "OnCommentLiked", "OnCommentReplied", "OnMeasureCreated", "OnPostLiked", "OnPostPublished"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subscribeSocialRelay 訂閱 %v，預期 %v", got, want)
	}
}
