package sellerrelay

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// payuniNotices 的文案逐字等於 CodeMeta 的 PayuniSellerFacingError（pkg/codemeta/labels.go，單體不能 import 那個 module，
// 所以讀它的原始碼），而且涵蓋 labels.go 裡全部的 MER_x、MEMBER_x（錯誤目錄登記的狀態碼）與通用文案。
func TestNoticesMatchCodeMeta(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "../../../pkg/codemeta/labels.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "payuniSellerFacingErrorLabels" {
			return true
		}
		for _, el := range spec.Values[0].(*ast.CompositeLit).Elts {
			kv := el.(*ast.KeyValueExpr)
			key, _ := strconv.Unquote(kv.Key.(*ast.BasicLit).Value)
			value, _ := strconv.Unquote(kv.Value.(*ast.BasicLit).Value)
			labels[key] = value
		}
		return false
	})
	if len(labels) == 0 {
		t.Fatal("payuniSellerFacingErrorLabels not found in labels.go")
	}
	for code, notice := range payuniNotices {
		if got, ok := labels[code]; !ok || got != notice {
			t.Errorf("%s: relay %q, CodeMeta %q (present=%v)", code, notice, got, ok)
		}
	}
	for code := range labels {
		if (strings.HasPrefix(code, "MER_") || strings.HasPrefix(code, "MEMBER_") || code == payuniUnknown) && payuniNotices[code] == "" {
			t.Errorf("%s is in CodeMeta but not in the relay", code)
		}
	}
}
