package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func sellersParamIndex(t *testing.T, fn string) int {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "procurement.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn {
			continue
		}
		i := 0
		for _, field := range fd.Type.Params.List {
			for _, name := range field.Names {
				if name.Name == "sellers" {
					return i
				}
				i++
			}
		}
	}
	t.Fatalf("procurement.go 的 %s 沒有 sellers 參數", fn)
	return -1
}

// 發布詢價的封鎖與營業中檢查失敗時一律回組裝錯誤，所以 serve 與 worker 的採購模組都要拿到 seller 模組，傳 nil 會讓每次發布都失敗。
func TestProcurementIsAssembledWithTheSellerModule(t *testing.T) {
	for _, c := range []struct{ role, build string }{{"runServe", "newProcurement"}, {"runWorker", "newProcurementWorker"}} {
		idx := sellersParamIndex(t, c.build)
		found := false
		ast.Inspect(funcBody(t, c.role), func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != c.build {
				return true
			}
			found = true
			if idx >= len(call.Args) {
				t.Errorf("%s 呼叫 %s 的參數不夠", c.role, c.build)
				return true
			}
			if arg, ok := call.Args[idx].(*ast.Ident); !ok || arg.Name != "sellerModule" {
				t.Errorf("%s 的 %s 第 %d 個參數要是 sellerModule", c.role, c.build, idx+1)
			}
			return true
		})
		if !found {
			t.Errorf("%s 沒有呼叫 %s", c.role, c.build)
		}
	}
}
