package worker

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"go.uber.org/zap"
)

// 單體（internal/legacy 下的 relay 與各 legacy 服務）送出的每個 (exchange, routing key)，消費者這一側要成立：
// topology.yaml 有 queue 綁到它，而且 notification-service 不是接不到就是明白宣告不處理（notifyAllowlist）。
// 單體是另一個 Go module，這裡不能 import 它的發送清單，所以讀它的原始碼取得；掃描規則與失敗條件見單體
// internal/legacy/rabbitmq/publish_routes_test.go（同一份掃描，兩個 module 不能共用，改一邊要改另一邊）。
// 每個 route 有沒有 template 要查 DB，那部分在 template_routes_integration_test.go。
// 欄位正確性不在這裡（那是各發送者的 wire_test）。終點：最後一個舊 Publisher 使用者刪除時刪掉。

const (
	monolithInternal = "../../../../internal"
	monolithSelf     = "../../../../internal/legacy/rabbitmq"
)

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

// topicMatch 是 AMQP topic exchange 的比對：* 一個字，# 零個以上的字（單體 internal/legacy/rabbitmq/publish_routes_test.go 有同一份，兩個 module 不能共用）。
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

// notifyAllowlist 是「單體送得出來、notification-service 卻刻意不做通知」的 route。其餘每個 route 要有 enabled 的 template
// 或是靜態訂閱（staticSubscriptions）。每筆寫由哪個 queue 消費，過期的條目（不再送出、已有靜態訂閱、已有 template）會讓測試失敗。
type notifyAllowlistEntry struct{ exchange, key, consumerQueue, reason string }

var notifyAllowlist = []notifyAllowlistEntry{
	{"hawkr.seller", "seller.profile.changed", "cart.seller.profile.changed", "購物車依店家資料變更刷新，不通知任何人"},
	{"hawkr.seller", "seller.exposure.changed", "product.seller.exposure.changed", "商品依店家曝光狀態更新，不通知任何人"},
	{"hawkr.buyer", "buyer.favorite.removed", "product.buyer.favorite.removed", "商品收藏數扣減，取消收藏不通知賣家"},
}

// staticSubscriptions 與 di.go 的 startConsumer 裡 append 的靜態訂閱同一組（不是由 template 驅動）。
func staticSubscriptions() []mq.Subscription {
	return []mq.Subscription{
		buyerDeletedSubscription(nil, zap.NewNop()),
		accountDeletedSubscription(nil, zap.NewNop()),
		sellerDeletedSubscription(nil, zap.NewNop()),
	}
}

func staticHandles(exchange, key string) (mq.Subscription, bool) {
	for _, s := range staticSubscriptions() {
		if s.Exchange != exchange {
			continue
		}
		for _, k := range s.RoutingKeys {
			if topicMatch(k, key) {
				return s, true
			}
		}
	}
	return mq.Subscription{}, false
}

func TestPublishedRoutesAreBound(t *testing.T) {
	topo := loadTopology(t)
	for _, r := range publishRoutes(t, monolithInternal, monolithSelf) {
		if len(boundQueues(topo, r.exchange, r.key)) == 0 {
			t.Errorf("%s 送出 %s/%s，topology.yaml 沒有任何 queue 綁到它：這個事件送出去沒有人收", r.pkg, r.exchange, r.key)
		}
	}
}

func boundQueues(topo mqTopology, exchange, key string) []string {
	var out []string
	for _, q := range topo.Queues {
		for _, b := range q.Bindings {
			if b.Exchange == exchange && topicMatch(b.Key, key) {
				out = append(out, q.Name)
				break
			}
		}
	}
	return out
}

func TestStaticSubscriptionsAreBound(t *testing.T) {
	topo := loadTopology(t)
	for _, s := range staticSubscriptions() {
		for _, k := range s.RoutingKeys {
			if !slices.Contains(boundQueues(topo, s.Exchange, k), s.Queue) {
				t.Errorf("靜態訂閱 %s 沒有被 topology.yaml 綁到 %s/%s", s.Queue, s.Exchange, k)
			}
		}
	}
}

func TestNotifyAllowlistIsCurrent(t *testing.T) {
	topo := loadTopology(t)
	emitted := map[string]bool{}
	for _, r := range publishRoutes(t, monolithInternal, monolithSelf) {
		emitted[r.exchange+"/"+r.key] = true
	}
	seen := map[string]bool{}
	for _, e := range notifyAllowlist {
		id := e.exchange + "/" + e.key
		if seen[id] {
			t.Errorf("allowlist 重複：%s", id)
		}
		seen[id] = true
		if e.reason == "" || e.consumerQueue == "" {
			t.Errorf("allowlist %s 缺原因或消費的 queue", id)
		}
		if !emitted[id] {
			t.Errorf("allowlist %s 已不是單體送出的 route：刪掉這筆", id)
		}
		if _, ok := staticHandles(e.exchange, e.key); ok {
			t.Errorf("allowlist %s 已有靜態訂閱處理：刪掉這筆", id)
		}
		if strings.HasPrefix(e.consumerQueue, "notification.") {
			t.Errorf("allowlist %s 的消費者不該是 notification 自己的 queue", id)
		}
		if !slices.Contains(boundQueues(topo, e.exchange, e.key), e.consumerQueue) {
			t.Errorf("allowlist %s 宣稱由 %s 消費，但 topology.yaml 沒有綁", id, e.consumerQueue)
		}
	}
}
