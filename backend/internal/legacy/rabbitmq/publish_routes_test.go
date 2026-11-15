package rabbitmq

import (
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

	"go.yaml.in/yaml/v3"
)

// 單體裡每個把事件交給本 package 的 Publisher 的地方（PublishArgs 字面值），送出的每個 (exchange, routing key)，
// 都要落在 infra/mq/topology/topology.yaml 宣告的 queue。清單由 internal/ 底下的原始碼導出，不手抄，也不寫死 package 名單：
// 新增一個發送者就多一列。這裡讀原始碼而不是讓 relay 匯出路由表，因為消費端 notification-service 是另一個 Go module，
// 不能 import 單體，兩邊都得讀原始碼；單體這側的發送清單由這份掃描統一產生。
// notification-service 的 relay_routes_test.go 有同一份掃描（兩個 module 不能共用），改這裡要一併改那裡。
//
// 掃不全就失敗，不靜默略過：PublishArgs 只允許出現在「字面值」與「函式參數型別」兩種位置；字面值的 Exchange、RoutingKey
// 必須是字串字面值或同 package 的 package 層 const，或是轉手函式（以函式參數填入）的參數，而轉手函式的每個呼叫點都要是
// 可解析的呼叫（不得被當成值傳遞）。欄位正確性不在這裡（那是各 package 的 wire_test）。終點：最後一個舊 Publisher 使用者刪除時刪掉。

const (
	topologyFile  = "../../../infra/mq/topology/topology.yaml"
	internalRoot  = "../.."
	publisherSelf = "../../legacy/rabbitmq"
)

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

// boundQueues 回傳 topology.yaml 裡綁到 (exchange, key) 的 queue 名稱（排序）。
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

type routeRow struct{ pkg, exchange, key string }

type forwarder struct {
	name          string
	exIdx, keyIdx int
}

// publishRoutes 掃描 root 底下每個 package（略過 skipDir 與 _test.go）。
func publishRoutes(t *testing.T, root, skipDir string) []routeRow {
	t.Helper()
	skip, _ := filepath.Abs(skipDir)
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if abs, _ := filepath.Abs(p); abs == skip {
				return filepath.SkipDir
			}
			dirs = append(dirs, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var rows []routeRow
	for _, dir := range dirs {
		rows = append(rows, scanPackageRoutes(t, dir)...)
	}
	if len(rows) == 0 {
		t.Fatalf("%s 底下掃不到任何 PublishArgs 發送點：掃描壞了", root)
	}
	return rows
}

func isPublishArgs(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "PublishArgs"
}

func scanPackageRoutes(t *testing.T, dir string) []routeRow {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return nil
	}
	pkg := filepath.ToSlash(dir)

	// 只解析 package 層的 const：函式內的同名 const 不算。
	consts := map[string]string{}
	for _, f := range files {
		for _, d := range f.Decls {
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
		// PublishArgs 只能出現在字面值的型別與函式參數型別。
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
			t.Fatalf("%s：PublishArgs 有 %d 處不是字面值也不是函式參數型別（變數宣告、欄位、泛型等），掃描看不到它的值", pkg, total-allowed)
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
						t.Fatalf("%s：PublishArgs 字面值不是 key: value 寫法", pkg)
					}
					switch kv.Key.(*ast.Ident).Name {
					case "Exchange":
						exprs[0] = kv.Value
					case "RoutingKey":
						exprs[1] = kv.Value
					}
				}
				if exprs[0] == nil || exprs[1] == nil {
					t.Fatalf("%s：PublishArgs 字面值缺 Exchange 或 RoutingKey", pkg)
				}
				ex, exOK := resolve(exprs[0])
				key, keyOK := resolve(exprs[1])
				if exOK && keyOK {
					rows = append(rows, routeRow{pkg, ex, key})
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
					t.Fatalf("%s：PublishArgs 的 Exchange 或 RoutingKey 不是字串字面值、package 層 const，也不是外層函式的參數", pkg)
				}
				forwarders[fn.Name.Name] = forwarder{fn.Name.Name, exIdx, keyIdx}
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
						t.Fatalf("%s：%s 的呼叫參數數量不對，掃描要跟著改", pkg, fw.name)
					}
					ex, exOK := resolve(v.Args[fw.exIdx])
					key, keyOK := resolve(v.Args[fw.keyIdx])
					if !exOK || !keyOK {
						t.Fatalf("%s：%s 的 exchange 或 routing key 不是字串字面值或 package 層 const，掃描看不到", pkg, fw.name)
					}
					rows = append(rows, routeRow{pkg, ex, key})
				}
				return true
			})
		}
		// 識別字出現次數 = 一個宣告 + 每個呼叫；多出來的是被當成值傳遞（method value）或被其他方式引用。
		if idents != calls+1 {
			t.Fatalf("%s：%s 有 %d 處引用不是直接呼叫（被當成值傳遞？），掃描看不到那些送出點", pkg, fw.name, idents-calls-1)
		}
		if calls == 0 {
			t.Fatalf("%s：%s 沒有任何呼叫點", pkg, fw.name)
		}
	}
	return rows
}

// topicMatch 是 AMQP topic exchange 的比對：* 一個字，# 零個以上的字（notification-service 的 relay_routes_test.go 有同一份）。
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
	for _, r := range publishRoutes(t, internalRoot, publisherSelf) {
		if len(doc.boundQueues(r.exchange, r.key)) == 0 {
			t.Errorf("%s 送出 %s/%s，topology.yaml 沒有任何 queue 綁到它：這個事件送出去沒有人收", r.pkg, r.exchange, r.key)
		}
	}
}
