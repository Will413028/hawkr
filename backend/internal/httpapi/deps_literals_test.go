package httpapi

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/template"
)

const httpapiImportPath = "github.com/Will413028/hawkr/backend/internal/httpapi"

func requiredDepsFields(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "server.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "NewHandler" || fn.Recv != nil {
			continue
		}
		for _, stmt := range fn.Body.List {
			ifs, ok := stmt.(*ast.IfStmt)
			if !ok {
				continue
			}
			cmp, ok := ifs.Cond.(*ast.BinaryExpr)
			if !ok || cmp.Op != token.EQL {
				continue
			}
			sel, ok := cmp.X.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "deps" {
				continue
			}
			if id, ok := cmp.Y.(*ast.Ident); !ok || id.Name != "nil" {
				continue
			}
			if len(ifs.Body.List) != 1 {
				continue
			}
			ret, ok := ifs.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 2 {
				continue
			}
			if id, ok := ret.Results[0].(*ast.Ident); !ok || id.Name != "nil" {
				continue
			}
			fields = append(fields, sel.Sel.Name)
		}
	}
	return fields
}

func depsLiteralProblems(t *testing.T, name string, src any, required []string) (problems []string, found int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("解析 %s 失敗：%v", name, err)
	}
	alias := ""
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p != httpapiImportPath {
			continue
		}
		alias = "httpapi"
		if imp.Name != nil {
			alias = imp.Name.Name
		}
	}
	if alias == "" {
		return nil, 0
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Deps" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != alias {
			return true
		}
		found++
		set := map[string]bool{}
		for _, e := range lit.Elts {
			if kv, ok := e.(*ast.KeyValueExpr); ok {
				if k, ok := kv.Key.(*ast.Ident); ok {
					set[k.Name] = true
				}
			}
		}
		var missing []string
		for _, r := range required {
			if !set[r] {
				missing = append(missing, r)
			}
		}
		if len(missing) > 0 {
			pos := fset.Position(lit.Pos())
			problems = append(problems, pos.Filename+":"+strconv.Itoa(pos.Line)+" 的 httpapi.Deps 缺必填欄位 "+strings.Join(missing, ", "))
		}
		return true
	})
	return problems, found
}

// NewHandler 對必填依賴在執行期才報錯；這支測試讓漏設在 unit 層就失敗，不必等 CI 的 integration。
func TestHTTPAPIDepsLiteralsSetEveryRequiredField(t *testing.T) {
	required := requiredDepsFields(t)
	sort.Strings(required)
	has := map[string]bool{}
	for _, r := range required {
		has[r] = true
	}
	if len(required) == 0 || !has["Review"] {
		t.Fatalf("解析 NewHandler 的必填欄位失敗：%v", required)
	}
	if has["Now"] || has["Logger"] {
		t.Fatalf("Now、Logger 是補預設，不該算必填：%v", required)
	}

	var problems []string
	total := 0
	sawBuyerIntegration := false
	for _, root := range []string{"../../cmd", "../../internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			p, n := depsLiteralProblems(t, path, nil, required)
			problems = append(problems, p...)
			total += n
			if n > 0 && filepath.ToSlash(path) == "../../internal/httpapi/buyer/buyer_integration_test.go" {
				sawBuyerIntegration = true
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if !sawBuyerIntegration {
		t.Fatal("沒掃到 buyer_integration_test.go 的 httpapi.Deps：掃描漏了帶 build tag 的檔")
	}

	tmplFound := 0
	err := filepath.WalkDir("../../cmd/new-module/templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(raw, []byte("httpapi.Deps{")) {
			return nil
		}
		tpl, err := template.New(filepath.Base(path)).Parse(string(raw))
		if err != nil {
			t.Fatalf("解析範本 %s 失敗：%v", path, err)
		}
		var out bytes.Buffer
		if err := tpl.Execute(&out, map[string]string{"Name": "sample", "Module": "github.com/Will413028/hawkr/backend"}); err != nil {
			t.Fatalf("渲染範本 %s 失敗：%v", path, err)
		}
		p, n := depsLiteralProblems(t, path, out.Bytes(), required)
		problems = append(problems, p...)
		tmplFound += n
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if tmplFound == 0 {
		t.Fatal("範本裡沒找到 httpapi.Deps：範本檢查失效")
	}
	if total < 8 {
		t.Fatalf("只找到 %d 個 httpapi.Deps 字面值，掃描可能失效", total)
	}
	for _, p := range problems {
		t.Error(p)
	}
}
