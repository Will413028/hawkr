package arch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strings"
)

// RestrictedCalls 規定一組方法名只能出現在哪些目錄的非測試程式。
type RestrictedCalls struct {
	// Names 是受限的方法名（任何 X.Name 的選取，呼叫或方法值都算）。
	Names []string
	// AllowedDirs 是可以使用它們的目錄，單體根目錄起算的斜線路徑；目錄底下的子目錄也算。
	AllowedDirs []string
	// Why 說明規則，放進違規訊息。
	Why string
}

// SuspensionEntries 守住 identity 的停權、解除入口：「有店的帳號不能停權」的檢查在 accountsuspension，繞過它直接呼叫 identity
// 的入口就會停權有店的帳號、略過鎖順序與事件。identity 自己的實作（模組內的轉接與 sqlc 的查詢）不算呼叫者。
var SuspensionEntries = RestrictedCalls{
	Names:       []string{"SuspendAccount", "UnsuspendAccount"},
	AllowedDirs: []string{"internal/modules/accountsuspension", "internal/modules/identity"},
	Why:         "停權與解除只能經 accountsuspension（A7-D15）",
}

// RestrictedCallsResult 是一次掃描的結果。
type RestrictedCallsResult struct {
	Violations []string
	// Allowed 是在允許的目錄裡看到的受限名稱的次數，供測試確認掃描不是空轉。
	Allowed int
}

// CheckRestrictedCalls 檢查 src 裡受限的名稱。filename 是單體根目錄起算的斜線路徑；呼叫方負責略過測試檔與生成檔。
func CheckRestrictedCalls(filename string, src []byte, rule RestrictedCalls) (RestrictedCallsResult, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return RestrictedCallsResult{}, err
	}
	allowed := false
	for _, dir := range rule.AllowedDirs {
		if strings.HasPrefix(filename, dir+"/") {
			allowed = true
		}
	}
	var res RestrictedCallsResult
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		for _, name := range rule.Names {
			if sel.Sel.Name != name {
				continue
			}
			if allowed {
				res.Allowed++
			} else {
				res.Violations = append(res.Violations, fmt.Sprintf("%s：不得使用 %s：%s", fset.Position(sel.Sel.Pos()), name, rule.Why))
			}
		}
		return true
	})
	return res, nil
}

// RestrictedPackageCalls 規定某個套件的一組函式只能在哪些目錄呼叫；與 RestrictedCalls 不同，它依 import 解析套件名（含別名），
// 同名的其他套件函式不算，測試檔也要檢查。
type RestrictedPackageCalls struct {
	// ImportPath 是受限函式所屬的套件。
	ImportPath string
	// Names 是受限的函式名。
	Names []string
	// AllowedDirs 是可以呼叫的目錄，單體根目錄起算的斜線路徑；子目錄也算。
	AllowedDirs []string
	// Why 說明規則，放進違規訊息。
	Why string
}

// LLMClientCalls 守住單一 LLM client 建構點的呼叫面：genai 的型別各處可用，但建構 client 只有 internal/platform/llm。
var LLMClientCalls = RestrictedPackageCalls{
	ImportPath:  "google.golang.org/genai",
	Names:       []string{"NewClient"},
	AllowedDirs: []string{"internal/platform/llm"},
	Why:         "LLM client 只能由 internal/platform/llm 建構（A9-D10、AD5）",
}

// CheckRestrictedPackageCalls 檢查 src 裡對受限套件函式的選取（呼叫或函式值）。filename 是單體根目錄起算的斜線路徑；
// 呼叫方負責略過生成檔。
func CheckRestrictedPackageCalls(filename string, src []byte, rule RestrictedPackageCalls) (RestrictedCallsResult, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return RestrictedCallsResult{}, err
	}
	allowed := false
	for _, dir := range rule.AllowedDirs {
		if strings.HasPrefix(filename, dir+"/") {
			allowed = true
		}
	}
	names := map[string]bool{}
	dot := false
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, "\"`") != rule.ImportPath {
			continue
		}
		switch {
		case imp.Name == nil:
			names[path.Base(rule.ImportPath)] = true
		case imp.Name.Name == ".":
			dot = true
		case imp.Name.Name != "_":
			names[imp.Name.Name] = true
		}
	}
	var res RestrictedCallsResult
	hit := func(pos token.Pos, name string) {
		if allowed {
			res.Allowed++
			return
		}
		res.Violations = append(res.Violations, fmt.Sprintf("%s：不得使用 %s.%s：%s", fset.Position(pos), path.Base(rule.ImportPath), name, rule.Why))
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			if ok && names[id.Name] && slices.Contains(rule.Names, x.Sel.Name) {
				hit(x.Sel.Pos(), x.Sel.Name)
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && dot && slices.Contains(rule.Names, id.Name) {
				hit(id.Pos(), id.Name)
			}
		}
		return true
	})
	return res, nil
}
