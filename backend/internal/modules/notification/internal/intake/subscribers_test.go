package intake

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// tokenPurgeSubscribers 是 subscribers.go 裡允許做排意圖以外事情的訂閱者：裝置 token 的同步刪除（隨停權、帳號刪除在同一個
// transaction 刪 token）。加進來要寫理由，並且只能刪 token（TestTokenPurgeSubscribersOnlyDeleteTokens）。
var tokenPurgeSubscribers = []string{"PurgeTokensOnSuspended", "PurgeTokensOnAccountDeleted"}

var majorVersion = regexp.MustCompile(`^v[0-9]+$`)

// importNames 回傳檔案裡每個 import 在檔內使用的名稱與它的完整路徑。
func importNames(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if majorVersion.MatchString(name) {
			name = path.Base(path.Dir(p))
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		out[name] = p
	}
	return out
}

// calls 回傳函式本體裡每個呼叫，解析成「完整套件路徑.名稱」：套件限定的呼叫（store.New）取 import 的完整路徑，
// 值上的方法呼叫（s.d.Registry.Lookup）寫成「.名稱」（不看型別，只看名稱），同套件的函式寫成「名稱」。
// 所以允許 store.New 不會連帶允許別的套件的 New。
func calls(fn *ast.FuncDecl, imports map[string]string) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			if x, ok := f.X.(*ast.Ident); ok {
				if p, ok := imports[x.Name]; ok {
					out = append(out, p+"."+f.Sel.Name)
					return true
				}
			}
			out = append(out, "."+f.Sel.Name)
		case *ast.Ident:
			out = append(out, f.Name)
		}
		return true
	})
	return out
}

func funcs(t *testing.T, file string) ([]*ast.FuncDecl, map[string]string) {
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
	return out, importNames(f)
}

// 同步訂閱者在來源的 transaction 內執行，失敗會讓來源 rollback：除了白名單的 token 清除，只准呼叫 Accept。
func TestSynchronousSubscribersOnlyAccept(t *testing.T) {
	files, err := filepath.Glob("subscribers*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到 subscribers*.go（%v）", err)
	}
	total := 0
	for _, file := range slices.DeleteFunc(files, func(f string) bool { return strings.HasSuffix(f, "_test.go") }) {
		fns, imports := funcs(t, file)
		for _, fn := range fns {
			if slices.Contains(tokenPurgeSubscribers, fn.Name.Name) || slices.Contains(pureSubscriberHelpers, fn.Name.Name) {
				continue
			}
			total++
			got := slices.DeleteFunc(calls(fn, imports), func(c string) bool { return slices.Contains(pureSubscriberHelpers, c) })
			if len(got) != 1 || got[0] != ".Accept" {
				t.Errorf("%s 的 %s 呼叫了 %v；同步訂閱者只准呼叫 Accept", file, fn.Name.Name, got)
			}
		}
	}
	if total == 0 {
		t.Fatal("subscribers*.go 沒有訂閱者")
	}
}

// pureSubscriberHelpers 是訂閱者可以呼叫的純函式（只把事件欄位換成參數欄位，不碰資料）；本身也不算訂閱者。
var pureSubscriberHelpers = []string{"partyOf"}

// Accept 是同步路徑上唯一碰資料的函式：只登記意圖列、排 job，不讀偏好、不寫收件匣、不鎖帳號。
func TestAcceptOnlyInsertsTheIntentAndEnqueues(t *testing.T) {
	const store = "github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	allowed := []string{
		".Lookup", ".IdempotencyKey", ".OccurredAt", ".NewID", ".InsertIntent", ".Conn", ".Now", ".Enqueue",
		"fmt.Errorf", "encoding/json.Marshal", "errors.Is", store + ".New",
	}
	fns, imports := funcs(t, "intake.go")
	for _, fn := range fns {
		if fn.Name.Name != "Accept" {
			continue
		}
		for _, c := range calls(fn, imports) {
			if !slices.Contains(allowed, c) {
				t.Errorf("Accept 呼叫了 %s，不在同步路徑的允許清單 %v", c, allowed)
			}
		}
		return
	}
	t.Fatal("找不到 Accept")
}

// 允許清單比對的是「完整套件路徑加名稱」：同名的別的套件函式（例如 seller.New）不會因為 store.New 被允許而通過。
func TestAcceptAllowListResolvesPackages(t *testing.T) {
	src := "package x\n\nimport (\n\t\"fmt\"\n\t\"github.com/Will413028/hawkr/backend/internal/modules/seller\"\n\tpgx \"github.com/jackc/pgx/v5\"\n)\n\n" +
		"func Accept() { _ = seller.New(); _ = fmt.Errorf(\"x\"); _ = pgx.Connect(); s.d.Lookup() }\n"
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	fn := f.Decls[1].(*ast.FuncDecl)
	got := calls(fn, importNames(f))
	want := []string{"github.com/Will413028/hawkr/backend/internal/modules/seller.New", "fmt.Errorf", "github.com/jackc/pgx/v5.Connect", ".Lookup"}
	if !slices.Equal(got, want) {
		t.Fatalf("calls = %v，預期 %v", got, want)
	}
}

// 白名單內的訂閱者只准呼叫刪 token 的 helper，helper 只准刪 token（不排 job、不讀別的模組、不碰別的表）。
func TestTokenPurgeSubscribersOnlyDeleteTokens(t *testing.T) {
	fns, imports := funcs(t, "subscribers.go")
	seen := 0
	for _, fn := range fns {
		if !slices.Contains(tokenPurgeSubscribers, fn.Name.Name) {
			continue
		}
		seen++
		if got := calls(fn, imports); len(got) != 1 || got[0] != ".deleteAccountTokens" {
			t.Errorf("%s 呼叫了 %v；token 清除訂閱者只准呼叫 deleteAccountTokens", fn.Name.Name, got)
		}
	}
	if seen != len(tokenPurgeSubscribers) {
		t.Fatalf("subscribers.go 只有 %d 個白名單訂閱者，預期 %d", seen, len(tokenPurgeSubscribers))
	}
	const store = "github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	allowed := []string{".Conn", store + ".New", ".DeleteDeviceTokensByAccount", "fmt.Errorf"}
	helpers, himports := funcs(t, "tokens.go")
	for _, fn := range helpers {
		for _, c := range calls(fn, himports) {
			if !slices.Contains(allowed, c) {
				t.Errorf("%s 呼叫了 %s，不在 token 清除的允許清單 %v", fn.Name.Name, c, allowed)
			}
		}
	}
}
