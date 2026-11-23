package intake

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// tokenPurgeSubscribers 是 subscribers.go 裡允許做排意圖以外事情的訂閱者：A8-3 的裝置 token 同步刪除（隨停權、帳號刪除在同一個
// transaction 刪 token）。現在是空的；加進來要寫理由，並且只能刪 token。
var tokenPurgeSubscribers = []string{}

// calls 回傳函式本體裡每個呼叫的名稱（selector 取最後一段，一般函式取名稱）。
func calls(fn *ast.FuncDecl) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			out = append(out, f.Sel.Name)
		case *ast.Ident:
			out = append(out, f.Name)
		}
		return true
	})
	return out
}

func funcs(t *testing.T, file string) []*ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []*ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			out = append(out, fn)
		}
	}
	return out
}

// 同步訂閱者在來源的 transaction 內執行，失敗會讓來源 rollback：除了白名單的 token 清除，只准呼叫 Accept。
func TestSynchronousSubscribersOnlyAccept(t *testing.T) {
	fns := funcs(t, "subscribers.go")
	if len(fns) == 0 {
		t.Fatal("subscribers.go 沒有訂閱者")
	}
	for _, fn := range fns {
		if slices.Contains(tokenPurgeSubscribers, fn.Name.Name) {
			continue
		}
		got := calls(fn)
		if len(got) != 1 || got[0] != "Accept" {
			t.Errorf("%s 呼叫了 %v；同步訂閱者只准呼叫 Accept", fn.Name.Name, got)
		}
	}
}

// Accept 是同步路徑上唯一碰資料的函式：只登記意圖列、排 job，不讀偏好、不寫收件匣、不鎖帳號。
func TestAcceptOnlyInsertsTheIntentAndEnqueues(t *testing.T) {
	allowed := []string{"Lookup", "Errorf", "Marshal", "IdempotencyKey", "NewID", "InsertIntent", "New", "Conn", "Now", "Is", "Enqueue"}
	for _, fn := range funcs(t, "intake.go") {
		if fn.Name.Name != "Accept" {
			continue
		}
		for _, c := range calls(fn) {
			if !slices.Contains(allowed, c) {
				t.Errorf("Accept 呼叫了 %s，不在同步路徑的允許清單 %v", c, allowed)
			}
		}
		return
	}
	t.Fatal("找不到 Accept")
}
