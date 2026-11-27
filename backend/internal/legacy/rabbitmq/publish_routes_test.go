package rabbitmq

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"
)

const (
	topologyFile  = "../../../infra/mq/topology/topology.yaml"
	backendRoot   = "../../.."
	publisherSelf = "../../legacy/rabbitmq"
)

var scanRoots = []string{"../..", "../../../cmd"}

type topologyDoc struct {
	Exchanges []struct {
		Name string `yaml:"name"`
		Type string `yaml:"type"`
	} `yaml:"exchanges"`
	Queues []struct {
		Name     string `yaml:"name"`
		Bindings []struct {
			Exchange string `yaml:"exchange"`
			Key      string `yaml:"key"`
		} `yaml:"bindings"`
	} `yaml:"queues"`
}

func loadTopologyDoc(t *testing.T) topologyDoc {
	t.Helper()
	raw, err := os.ReadFile(topologyFile)
	if err != nil {
		t.Fatal(err)
	}
	var doc topologyDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Exchanges) == 0 || len(doc.Queues) == 0 {
		t.Fatalf("%s 沒有 exchanges 或 queues", topologyFile)
	}
	return doc
}

func (d topologyDoc) boundQueues(exchange, key string) []string {
	var out []string
	for _, q := range d.Queues {
		for _, b := range q.Bindings {
			if b.Exchange == exchange && topicMatch(b.Key, key) {
				out = append(out, q.Name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

type routeRow struct {
	Exchange string `json:"exchange"`
	Key      string `json:"routing_key"`
	Package  string `json:"package"`
}

type forwarder struct {
	name          string
	exIdx, keyIdx int
}

var (
	scanOnce sync.Once
	scanRows []routeRow
	scanErr  error
)

func publishRoutes(t *testing.T) []routeRow {
	t.Helper()
	scanOnce.Do(func() { scanRows, scanErr = scanAll() })
	if scanErr != nil {
		t.Fatal(scanErr)
	}
	return scanRows
}

type scanFail struct{ msg string }

func failf(format string, args ...any) { panic(scanFail{fmt.Sprintf(format, args...)}) }

func scanAll() (rows []routeRow, err error) {
	defer func() {
		if r := recover(); r != nil {
			sf, ok := r.(scanFail)
			if !ok {
				panic(r)
			}
			err = errors.New(sf.msg)
		}
	}()
	skip, _ := filepath.Abs(publisherSelf)
	for _, root := range scanRoots {
		walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
			if werr != nil {
				return werr
			}
			if !d.IsDir() {
				return nil
			}
			if abs, _ := filepath.Abs(p); abs == skip || d.Name() == "testdata" || (strings.HasPrefix(d.Name(), ".") && p != root) {
				return filepath.SkipDir
			}
			rows = append(rows, scanPackageRoutes(p)...)
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("internal/、cmd/ 底下掃不到任何 PublishArgs 發送點：掃描壞了")
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Exchange != b.Exchange {
			return a.Exchange < b.Exchange
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		return a.Package < b.Package
	})
	out := rows[:0]
	for i, r := range rows {
		if i == 0 || r != rows[i-1] {
			out = append(out, r)
		}
	}
	return out, nil
}

func isPublishArgs(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "PublishArgs"
}

func isByteSlice(e ast.Expr) bool {
	at, ok := e.(*ast.ArrayType)
	if !ok || at.Len != nil {
		return false
	}
	id, ok := at.Elt.(*ast.Ident)
	return ok && id.Name == "byte"
}

func hasByteSliceFourParams(ft *ast.FuncType) bool {
	n, hasBytes := 0, false
	for _, p := range ft.Params.List {
		c := len(p.Names)
		if c == 0 {
			c = 1
		}
		n += c
		if isByteSlice(p.Type) {
			hasBytes = true
		}
	}
	return n >= 4 && hasBytes
}

func scanPackageRoutes(dir string) []routeRow {
	entries, err := os.ReadDir(dir)
	if err != nil {
		failf("%s：%v", dir, err)
	}
	var paths []string
	relevant := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			failf("%s：%v", p, err)
		}
		paths = append(paths, p)
		if bytes.Contains(raw, []byte("PublishArgs")) || bytes.Contains(raw, []byte("Publish(")) {
			relevant = true
		}
	}
	if !relevant {
		return nil
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			failf("%s：%v", p, err)
		}
		files = append(files, f)
	}
	absDir, _ := filepath.Abs(dir)
	absRoot, _ := filepath.Abs(backendRoot)
	rel, err := filepath.Rel(absRoot, absDir)
	if err != nil {
		failf("%s：%v", dir, err)
	}
	pkg := filepath.ToSlash(rel)

	for _, f := range files {
		importsPublisher := false
		for _, im := range f.Imports {
			if strings.HasSuffix(strings.Trim(im.Path.Value, `"`), "/internal/legacy/rabbitmq") {
				importsPublisher = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.CallExpr:
				if sel, ok := v.Fun.(*ast.SelectorExpr); ok && importsPublisher && sel.Sel.Name == "Publish" && len(v.Args) == 4 {
					failf("%s：引用 legacy/rabbitmq 的檔案直接呼叫了 Publish，掃描看不到它的 exchange 與 routing key；改走 PublishArgs 字面值", pkg)
				}
			case *ast.FuncDecl:
				if v.Name.Name == "Publish" && hasByteSliceFourParams(v.Type) {
					failf("%s：宣告了帶 []byte 的 Publish 方法，可能是繞過 PublishArgs 的送出點，掃描看不到", pkg)
				}
			case *ast.InterfaceType:
				for _, m := range v.Methods.List {
					if ft, ok := m.Type.(*ast.FuncType); ok && len(m.Names) == 1 && m.Names[0].Name == "Publish" && hasByteSliceFourParams(ft) {
						failf("%s：介面宣告了帶 []byte 的 Publish，可能是繞過 PublishArgs 的送出點，掃描看不到", pkg)
					}
				}
			}
			return true
		})
	}

	consts := map[string]string{}
	declCount := map[string]int{}
	for _, f := range files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				declCount[fd.Name.Name]++
			}
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							consts[name.Name], _ = strconv.Unquote(lit.Value)
						}
					}
				}
			}
		}
	}
	resolve := func(e ast.Expr) (string, bool) {
		switch v := e.(type) {
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				s, err := strconv.Unquote(v.Value)
				return s, err == nil && s != ""
			}
		case *ast.Ident:
			s, ok := consts[v.Name]
			return s, ok && s != ""
		}
		return "", false
	}

	var rows []routeRow
	forwarders := map[string]forwarder{}
	for _, f := range files {
		total, allowed := 0, 0
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.SelectorExpr:
				if v.Sel.Name == "PublishArgs" {
					total++
				}
			case *ast.CompositeLit:
				if isPublishArgs(v.Type) {
					allowed++
				}
			case *ast.FuncType:
				if v.Params != nil {
					for _, p := range v.Params.List {
						if isPublishArgs(p.Type) {
							allowed++
						}
					}
				}
			}
			return true
		})
		if total != allowed {
			failf("%s：PublishArgs 有 %d 處不是字面值也不是函式參數型別（變數宣告、欄位、回傳值、泛型等），掃描看不到它的值", pkg, total-allowed)
		}

		for _, d := range f.Decls {
			var params []string
			fn, isFn := d.(*ast.FuncDecl)
			if isFn {
				for _, p := range fn.Type.Params.List {
					if len(p.Names) == 0 {
						params = append(params, "_")
					}
					for _, n := range p.Names {
						params = append(params, n.Name)
					}
				}
			}
			ast.Inspect(d, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok || !isPublishArgs(lit.Type) {
					return true
				}
				var exprs [2]ast.Expr
				for _, el := range lit.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						failf("%s：PublishArgs 字面值不是 key: value 寫法", pkg)
					}
					id, ok := kv.Key.(*ast.Ident)
					if !ok {
						failf("%s：PublishArgs 字面值的欄位名不是識別字", pkg)
					}
					switch id.Name {
					case "Exchange":
						exprs[0] = kv.Value
					case "RoutingKey":
						exprs[1] = kv.Value
					}
				}
				if exprs[0] == nil || exprs[1] == nil {
					failf("%s：PublishArgs 字面值缺 Exchange 或 RoutingKey", pkg)
				}
				ex, exOK := resolve(exprs[0])
				key, keyOK := resolve(exprs[1])
				if exOK && keyOK {
					rows = append(rows, routeRow{ex, key, pkg})
					return true
				}
				idx := func(e ast.Expr) int {
					if id, ok := e.(*ast.Ident); ok && isFn {
						for i, p := range params {
							if p == id.Name {
								return i
							}
						}
					}
					return -1
				}
				exIdx, keyIdx := idx(exprs[0]), idx(exprs[1])
				if exIdx < 0 || keyIdx < 0 {
					failf("%s：PublishArgs 的 Exchange 或 RoutingKey 不是字串字面值、package 層 const，也不是外層函式的參數", pkg)
				}
				name := fn.Name.Name
				if fn.Name.IsExported() {
					failf("%s：轉手函式 %s 是匯出的，別的 package 的呼叫點掃描看不到；改成未匯出", pkg, name)
				}
				if declCount[name] > 1 {
					failf("%s：轉手函式 %s 在 package 內有同名宣告（不同 receiver？），沒有型別資訊分辨不了呼叫點；改名", pkg, name)
				}
				checkNoShadow(pkg, fn, params[exIdx], params[keyIdx])
				forwarders[name] = forwarder{name, exIdx, keyIdx}
				return true
			})
		}
	}

	for _, fw := range forwarders {
		idents, calls := 0, 0
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.Ident:
					if v.Name == fw.name {
						idents++
					}
				case *ast.CallExpr:
					var name string
					switch fun := v.Fun.(type) {
					case *ast.Ident:
						name = fun.Name
					case *ast.SelectorExpr:
						name = fun.Sel.Name
					}
					if name != fw.name {
						return true
					}
					calls++
					if len(v.Args) <= max(fw.exIdx, fw.keyIdx) {
						failf("%s：%s 的呼叫參數數量不對，掃描要跟著改", pkg, fw.name)
					}
					ex, exOK := resolve(v.Args[fw.exIdx])
					key, keyOK := resolve(v.Args[fw.keyIdx])
					if !exOK || !keyOK {
						failf("%s：%s 的 exchange 或 routing key 不是字串字面值或 package 層 const，掃描看不到", pkg, fw.name)
					}
					rows = append(rows, routeRow{ex, key, pkg})
				}
				return true
			})
		}
		if idents != calls+1 {
			failf("%s：%s 有 %d 處引用不是直接呼叫（被當成值傳遞、同名欄位或區域變數？），掃描看不到那些送出點", pkg, fw.name, idents-calls-1)
		}
		if calls == 0 {
			failf("%s：%s 沒有任何呼叫點", pkg, fw.name)
		}
	}
	return rows
}

func checkNoShadow(pkg string, fn *ast.FuncDecl, names ...string) {
	if fn.Body == nil {
		return
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		var ids []*ast.Ident
		switch v := n.(type) {
		case *ast.FuncLit:
			for _, p := range v.Type.Params.List {
				ids = append(ids, p.Names...)
			}
		case *ast.AssignStmt:
			if v.Tok == token.DEFINE {
				for _, l := range v.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						ids = append(ids, id)
					}
				}
			}
		case *ast.ValueSpec:
			ids = append(ids, v.Names...)
		case *ast.RangeStmt:
			if v.Tok == token.DEFINE {
				for _, e := range []ast.Expr{v.Key, v.Value} {
					if id, ok := e.(*ast.Ident); ok {
						ids = append(ids, id)
					}
				}
			}
		}
		for _, id := range ids {
			for _, name := range names {
				if id.Name == name {
					failf("%s：轉手函式 %s 內重新宣告了參數 %s（閉包參數或區域變數遮蔽），掃描分不清它指哪一個", pkg, fn.Name.Name, name)
				}
			}
		}
		return true
	})
}

func topicMatch(pattern, key string) bool {
	var match func(p, k []string) bool
	match = func(p, k []string) bool {
		if len(p) == 0 {
			return len(k) == 0
		}
		if p[0] == "#" {
			for i := 0; i <= len(k); i++ {
				if match(p[1:], k[i:]) {
					return true
				}
			}
			return false
		}
		if len(k) == 0 || (p[0] != "*" && p[0] != k[0]) {
			return false
		}
		return match(p[1:], k[1:])
	}
	return match(strings.Split(pattern, "."), strings.Split(key, "."))
}

func TestPublishedRoutesAreBoundInTopology(t *testing.T) {
	doc := loadTopologyDoc(t)
	for _, r := range publishRoutes(t) {
		if len(doc.boundQueues(r.Exchange, r.Key)) == 0 {
			t.Errorf("%s 送出 %s/%s，topology.yaml 沒有任何 queue 綁到它：這個事件送出去沒有人收", r.Package, r.Exchange, r.Key)
		}
	}
}
