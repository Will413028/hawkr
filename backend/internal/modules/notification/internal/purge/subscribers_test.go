package purge

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// 同步訂閱者在來源的 transaction 內執行，失敗會讓來源 rollback：只准呼叫 enqueue（排清除 job）與 string 轉型（受眾常數），
// 不碰資料表、不讀別的模組。名稱以呼叫的寫法比對：值上的方法呼叫寫成「.名稱」，同套件的函式與轉型寫成「名稱」。
func TestSynchronousSubscribersOnlyEnqueue(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "subscribers.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	allowed := []string{".enqueue", "string"}
	found := 0
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name == "enqueue" {
			continue
		}
		found++
		enqueues := 0
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch f := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = "." + f.Sel.Name
			case *ast.Ident:
				name = f.Name
			default:
				name = "?"
			}
			if !slices.Contains(allowed, name) {
				t.Errorf("%s 呼叫了 %s，同步訂閱者只准呼叫 %v", fn.Name.Name, name, allowed)
			}
			if name == ".enqueue" {
				enqueues++
			}
			return true
		})
		if enqueues != 1 {
			t.Errorf("%s 呼叫 enqueue %d 次，預期 1 次", fn.Name.Name, enqueues)
		}
	}
	if found != 3 {
		t.Fatalf("subscribers.go 有 %d 個訂閱者，預期 3 個（AccountDeleted、BuyerDeleted、StoreRetired）", found)
	}
}

// enqueue 是同步路徑上唯一碰 job 的函式：只排 job，不碰資料庫。
func TestEnqueueOnlyEnqueuesTheJob(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "subscribers.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "enqueue" {
			continue
		}
		var got []string
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					got = append(got, sel.Sel.Name)
				}
			}
			return true
		})
		if !slices.Equal(got, []string{"InsertOpts", "Enqueue"}) {
			t.Fatalf("enqueue 呼叫了 %v，預期只有 InsertOpts 與 Enqueue", got)
		}
		return
	}
	t.Fatal("找不到 enqueue")
}

func TestRecipientArgsValidation(t *testing.T) {
	ok := []RecipientArgs{{AccountID: 1}, {AccountID: 1, Audience: "BUYER"}, {AccountID: 1, Audience: "STORE"}, {AccountID: 1, Audience: "STORE", StoreID: 5}}
	for _, a := range ok {
		if err := a.validate(); err != nil {
			t.Errorf("%+v 應合法：%v", a, err)
		}
	}
	bad := []RecipientArgs{{}, {AccountID: -1}, {AccountID: 1, Audience: "X"}, {AccountID: 1, StoreID: 5}, {AccountID: 1, Audience: "BUYER", StoreID: 5}, {AccountID: 1, Audience: "STORE", StoreID: -1}}
	for _, a := range bad {
		if err := a.validate(); err == nil {
			t.Errorf("%+v 應不合法", a)
		}
	}
}

func TestScopeMapsZeroValuesToNull(t *testing.T) {
	aud, st := RecipientArgs{AccountID: 1}.scope()
	if aud != nil || st != nil {
		t.Fatalf("帳號範圍不限受眾與店：%v %v", aud, st)
	}
	aud, st = RecipientArgs{AccountID: 1, Audience: "STORE", StoreID: 7}.scope()
	if aud == nil || *aud != "STORE" || st == nil || *st != 7 {
		t.Fatalf("店範圍要帶受眾與店：%v %v", aud, st)
	}
}
