package reads_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
	"github.com/Will413028/hawkr/backend/internal/platform/querycheck"
)

// viewers 是每支 query 都要檢查的觀看者：匿名與登入的買家（封鎖加入後語句文字會不同）。
var viewers = []visibility.Viewer{visibility.Anonymous, {BuyerID: 42}}

// 單體規則 2：渲染出來的 SQL 只能參照 social 自己的表，函式只能是 social 的、內建的，或依賴表允許的 <擁有者>.pub_*。
func TestRenderedQueriesStayInsideTheModuleBoundary(t *testing.T) {
	owners := pubOwners(t)
	for _, e := range reads.Registry() {
		for _, v := range viewers {
			q, _ := e.Build(v).Sql()
			bad, err := querycheck.Check(q, "social", owners)
			if err != nil {
				t.Fatalf("%s：%v", e.Name, err)
			}
			for _, b := range bad {
				t.Errorf("%s：%s", e.Name, b)
			}
		}
	}
}

// pubOwners 是 social 可以呼叫 pub_* 的擁有者：pubsql 目錄下每個生成檔一個（cmd/jetgen 只為 internal/arch 的依賴表允許的擁有者生成）。
func pubOwners(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../pubsql/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到 pubsql 的生成檔：%v", err)
	}
	var out []string
	for _, f := range files {
		out = append(out, strings.TrimSuffix(filepath.Base(f), ".go"))
	}
	return out
}

// 規則裡的固定代碼是 SQL 常數，不是 bind 參數：`$1` 無法證明 partial index 的述詞，generic plan 會退成 Seq Scan（59 ms 對 0.7 ms）。
func TestRuleCodesAreInlineConstants(t *testing.T) {
	codes := []string{"PUBLISHED", "DRAFT", "UNPUBLISHED", "BUYER", "STORE", "VISIBLE", "DELETED", "HIDDEN"}
	for _, e := range reads.Registry() {
		for _, v := range viewers {
			_, args := e.Build(v).Sql()
			for i, a := range args {
				if s, ok := a.(string); ok && slices.Contains(codes, s) {
					t.Errorf("%s：第 %d 個參數是規則代碼 %q，要用 NewEnumValue 內聯成常數", e.Name, i+1, s)
				}
			}
		}
	}
	// 公開貼文的 query 一定含 'PUBLISHED' 常數（partial index posts_feed 的述詞）。
	for _, name := range []string{"ListFeedPosts", "ListStorePublicPosts", "ListHashtagPosts", "GetPublicPost", "TopPosts", "TrendingHashtags"} {
		e := entry(t, name)
		q, _ := e.Build(visibility.Anonymous).Sql()
		if !strings.Contains(q, "status = 'PUBLISHED'") {
			t.Errorf("%s 沒有 status = 'PUBLISHED' 的常數述詞", name)
		}
	}
}

func entry(t *testing.T, name string) reads.Entry {
	t.Helper()
	for _, e := range reads.Registry() {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("登記裡沒有 %s", name)
	return reads.Entry{}
}

// keyset 的游標條件只能是單一列比較（generic plan 下是 Index Cond）；`NOT has_cursor OR (…)` 這種布林開關是 Filter，成本隨游標深度成正比
// （step-1a-perf/README.md）。第一頁由 Go 給方向的極值游標。
func TestKeysetQueriesCompareOneRowAndHaveNoBooleanSwitch(t *testing.T) {
	rowCmp := regexp.MustCompile(`ROW\([\w.]+, [\w.]+\) [<>] ROW\(\$\d+::timestamp with time zone, \$\d+::uuid\)`)
	sw := regexp.MustCompile(`(?i)has_cursor|\$\d+::boolean\s+OR\b|\bOR\s+\$\d+::boolean`)
	n := 0
	for _, e := range reads.Registry() {
		if !e.Keyset {
			continue
		}
		n++
		for _, v := range viewers {
			q, _ := e.Build(v).Sql()
			if len(rowCmp.FindAllString(q, -1)) != 1 {
				t.Errorf("%s：游標條件要恰好一個 ROW(a, id) < ROW($, $) 的列比較", e.Name)
			}
			if sw.MatchString(q) {
				t.Errorf("%s：有布林開關的游標條件", e.Name)
			}
		}
	}
	if n < 9 {
		t.Errorf("只有 %d 支 keyset query 登記，預期 9 支", n)
	}
}

// parseDir 解析目錄裡的非測試 Go 檔。
func parseDir(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string]*ast.File{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		out[path] = f
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// social/internal 底下，原始 SQL 的逃生口（jet 的 CustomExpression、Token、Raw*、NewTable）只能出現在 pubsql 的生成檔；
// jet 自己的生成檔與 store（sqlc）不算。其他地方要寫 SQL 就走 sqlc，要用 pub_* 就改 db/pub.sql 與 make jet。
func TestRawSQLEscapesOnlyAppearInGeneratedPubWrappers(t *testing.T) {
	escapes := []string{"CustomExpression", "Token", "NewTable", "Raw", "RawBool", "RawInt", "RawFloat", "RawString", "RawTime", "RawTimez",
		"RawTimestamp", "RawTimestampz", "RawDate", "RawBytea", "RawStatement", "RawArgs"}
	skip := []string{"pubsql", "jet", "store"}
	for path, f := range parseDir(t, "..") {
		rel, _ := filepath.Rel("..", path)
		if slices.Contains(skip, strings.Split(filepath.ToSlash(rel), "/")[0]) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && slices.Contains(escapes, id.Name) {
				t.Errorf("%s：%s 是原始 SQL 的逃生口，只能出現在 pubsql 的生成檔", rel, id.Name)
			}
			return true
		})
	}
}

// 公開表（posts、comments、forum_entries）只經 visibility 的入口取得：其他套件引用 jet 的 table.Posts 等於繞過規則。
func TestPublicTablesAreOnlyReachedThroughVisibility(t *testing.T) {
	guarded := []string{"Posts", "Comments", "ForumEntries"}
	skip := []string{"visibility", "jet"}
	checked := 0
	for path, f := range parseDir(t, "..") {
		rel, _ := filepath.Rel("..", path)
		if slices.Contains(skip, strings.Split(filepath.ToSlash(rel), "/")[0]) {
			continue
		}
		alias := ""
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasSuffix(p, "/social/internal/jet/table") {
				alias = "table"
				if imp.Name != nil {
					alias = imp.Name.Name
				}
			}
		}
		if alias == "" {
			continue
		}
		checked++
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == alias && slices.Contains(guarded, sel.Sel.Name) {
					t.Errorf("%s：直接引用 table.%s 會繞過公開可讀的規則，改用 visibility 的入口", rel, sel.Sel.Name)
				}
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("沒有找到任何引用 jet table 的檔案，檢查範圍不對")
	}
}

// reads 裡每個回傳 jet 語句的匯出函式都要登記（守門測試逐支檢查登記的 query；沒登記就沒有邊界、形狀與 golden 的保護）。
func TestEveryQueryIsRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, e := range reads.Registry() {
		if registered[e.Name] {
			t.Errorf("%s 登記了兩次", e.Name)
		}
		registered[e.Name] = true
	}
	found := map[string]bool{}
	for path, f := range parseDir(t, ".") {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() || fn.Recv != nil || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				continue
			}
			id, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
			if !ok || (id.Name != "SelectStatement" && id.Name != "UpdateStatement") {
				continue
			}
			found[fn.Name.Name] = true
			if !registered[fn.Name.Name] && filepath.Base(path) != "registry.go" {
				t.Errorf("%s：%s 回傳 jet 語句，卻沒有登記在 Registry()", filepath.Base(path), fn.Name.Name)
			}
		}
	}
	for name := range registered {
		if !found[name] {
			t.Errorf("Registry() 登記了 %s，但 reads 裡沒有這個函式", name)
		}
	}
}
