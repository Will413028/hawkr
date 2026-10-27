package grpc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
)

// TestGraphControllerDepsAllFieldsWired 守的是「struct 加了欄位，組裝它的
// composite literal 沒跟著加」這一類漏接。
//
// 為什麼需要它：`GraphControllerDeps.Narrator` 由 2026-09-03 的 `77b8ffd9e`
// 加入，欄位自己的註解寫明它的用途是「讓消歧暫停那條分支也能敘述
// UNDERSTAND_INPUT／PICK_NEXT_QUESTION 兩碼」，但 di.go 的 fx provider 從那天
// 起就沒有設定它 —— 那條分支的 narrator 一直是 nil，兩碼的 Step.Text 一直是空
// 字串。**單元測試抓不到**：它們各自直接建構 deps 並帶入 narrator，只有真的
// 走過 fx 的 live 流量才會暴露（2026-09-08 dev 實測：走 terminal node 的正常
// 輪次兩碼都有敘述，走消歧暫停的兩輪都是空的）。
//
// 用 AST 比對而不是呼叫 provider：這些欄位全是介面，要在測試裡逐一造 stub 才
// 呼叫得動，而那份 stub 本身又會在加欄位時忘了更新 —— 用「宣告的欄位集合」對
// 「literal 實際指名的欄位集合」是同一個問題的無 stub 版本，且新增欄位自動納入。
func TestGraphControllerDepsAllFieldsWired(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "di.go", nil, 0)
	if err != nil {
		t.Fatalf("parse di.go: %v", err)
	}

	assigned := map[string]bool{}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "GraphControllerDeps" {
			return true
		}
		found = true
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok {
				assigned[key.Name] = true
			}
		}
		return true
	})
	if !found {
		t.Fatal("di.go 裡找不到 usecase.GraphControllerDeps 的 composite literal —— " +
			"組裝位置若已搬走，這個測試要跟著搬，不是刪掉")
	}

	declared := reflect.TypeOf(usecase.GraphControllerDeps{})
	for i := range declared.NumField() {
		name := declared.Field(i).Name
		if !assigned[name] {
			t.Errorf("GraphControllerDeps.%s 有宣告但 di.go 的 fx provider 沒有設定 —— "+
				"漏接的欄位在執行期是 nil，而消費端多半只是靜默跳過那段行為", name)
		}
	}
}
