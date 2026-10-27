package arch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// CheckApperrLiterals 回報 src 裡 apperr.* 呼叫的參數出現字串字面值的位置（"%s" 除外）：使用者面的句子一律來自模組的文案目錄
// （internal/text），呼叫 apperr 時只許 `"%s", text.X(locale)`。複合字面值（欄位名稱的 []string{"ProductID"}）不算句子，略過。
// 包含巢狀的呼叫，例如 apperr.WithCause(apperr.Unavailable("…"), err)。
func CheckApperrLiterals(filename string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "apperr" {
			return true
		}
		for _, arg := range call.Args {
			ast.Inspect(arg, func(m ast.Node) bool {
				switch v := m.(type) {
				case *ast.CompositeLit:
					return false
				case *ast.CallExpr:
					// 巢狀的 apperr 呼叫由外層的走訪自己檢查，這裡不重複回報。
					if inner, ok := v.Fun.(*ast.SelectorExpr); ok {
						if pkg, ok := inner.X.(*ast.Ident); ok && pkg.Name == "apperr" {
							return false
						}
					}
				case *ast.BasicLit:
					if v.Kind == token.STRING && v.Value != `"%s"` {
						out = append(out, fmt.Sprintf("%s：apperr.%s 的參數有字串字面值 %s（句子要來自 internal/text 的目錄）", fset.Position(v.Pos()), sel.Sel.Name, v.Value))
					}
				}
				return true
			})
		}
		return true
	})
	return out, nil
}

// generatedFile 回報檔案是不是生成檔（頭幾行有 Go 的 "Code generated … DO NOT EDIT." 標記）。
func generatedFile(src []byte) bool {
	head := string(src)
	if len(head) > 600 {
		head = head[:600]
	}
	return strings.Contains(head, "Code generated") && strings.Contains(head, "DO NOT EDIT")
}
