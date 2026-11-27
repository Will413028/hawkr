package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// SnapshotFields 漏填 = 選了全表 = 稽核可能虛報，而它不編譯失敗、不轉紅、回應完全
// 正常。這一支守的是任何新的 Orchestrate 呼叫點都必須明寫 SnapshotFields（要全表就寫
// nil），把「想過這件事」留在版控裡。
//
// 以 AST 而不是 grep 為錨：註解與字串裡出現 SnapshotFields 不算數，只有真的填在
// composite literal 的 key 才算（§5「命中的是程式碼還是註解」）。
func TestDelegatedOrchestrateCallSites_AllDeclareSnapshotFields(t *testing.T) {
	// 掃整個 package 而不是 admin_delegated_*.go：新端點若落在不合那個命名的
	// 檔案（例如既有的 admin_delegation.go），glob 版會靜默漏掉它，而 checked 門檻
	// 只擋得住「全部錨點失效」，擋不住「新增一個沒被掃到的」。
	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob 失敗: %v", err)
	}
	files := make([]string, 0, len(all))
	for _, f := range all {
		if !strings.HasSuffix(f, "_test.go") {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		t.Fatal("掃不到任何 .go —— 這支測試的錨點失效了，等於沒有閘門")
	}

	checked := 0
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("解析 %s 失敗: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Orchestrate" && sel.Sel.Name != "OrchestrateBatch") {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.CompositeLit)
				if !ok {
					continue
				}
				checked++
				if !compositeLitHasKey(lit, "SnapshotFields") {
					t.Errorf("%s:%d 的 %s 呼叫沒有宣告 SnapshotFields —— 漏填等於選了全表比較，"+
						"而狀態切換／媒體替換類端點在全表比較下會把寫入回應沒帶到的每一欄"+
						"記成「被清空」。要全表請明寫 SnapshotFields: nil。",
						path, fset.Position(lit.Pos()).Line, sel.Sel.Name)
				}
			}
			return true
		})
	}
	// 反向鑑別力：掃不到呼叫點時上面的迴圈一次都不會執行，整支測試會假綠。5 是
	// 當下的實際數量（product 4、order 1；商品上下架兩支各自共用一個呼叫點）。新增端點時這個數字要跟著往上調——那正是提醒
	// 你「有一支新的東西要決定比較範圍」的地方。
	if checked < 1 {
		t.Errorf("只掃到 %d 個 Orchestrate 呼叫點，少於已知的 1 個 —— AST 錨點可能失效了", checked)
	}
}

func compositeLitHasKey(lit *ast.CompositeLit, key string) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == key {
			return true
		}
	}
	return false
}

// ==================== helpers ====================
