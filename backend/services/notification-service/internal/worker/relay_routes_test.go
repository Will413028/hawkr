package worker

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// 單體的 relay（internal/legacy/sellerrelay、buyerrelay）送出的每個 (exchange, routing key)，消費者這一側要成立：
// topology.yaml 有 queue 綁到它，而且 notification-service 有訂這個 exchange 的 handler（newTemplateHandler 依 routing key
// 找 template）。relay 在另一個 Go module，這裡讀它的原始碼取得 key 清單（同 sellerrelay 的 wire_test 讀舊 publisher 的做法），
// 不 import。欄位正確性不在這裡（那是 relay 的 wire_test）。終點：最後一個 relay 刪除時刪掉。

const relayRoot = "../../../../internal/legacy"

type relayRoute struct{ relay, exchange, key string }

// relayRoutes 列出 <relay>/relay.go 裡每個 enqueuePublish(ctx, _, exchange, key, payload) 的 exchange 與 routing key；
// 參數不是字面值或 const、或 PublishArgs 在 enqueuePublish 以外被組出來，就失敗（掃描看不到全部的 key）。
func relayRoutes(t *testing.T, relay string) []relayRoute {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(relayRoot, relay, "relay.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	consts := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		if vs, ok := n.(*ast.ValueSpec); ok {
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					consts[name.Name], _ = strconv.Unquote(lit.Value)
				}
			}
		}
		return true
	})
	str := func(e ast.Expr) string {
		switch v := e.(type) {
		case *ast.BasicLit:
			s, _ := strconv.Unquote(v.Value)
			return s
		case *ast.Ident:
			return consts[v.Name]
		}
		return ""
	}
	var out []relayRoute
	publishLits := 0
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "enqueuePublish" {
				if len(v.Args) != 5 {
					t.Fatalf("%s：enqueuePublish 的參數不是 5 個，掃描要跟著改", relay)
				}
				ex, key := str(v.Args[2]), str(v.Args[3])
				if ex == "" || key == "" {
					t.Fatalf("%s：enqueuePublish 的 exchange 或 routing key 不是字面值，掃描看不到", relay)
				}
				out = append(out, relayRoute{relay, ex, key})
			}
		case *ast.CompositeLit:
			if sel, ok := v.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "PublishArgs" {
				publishLits++
			}
		}
		return true
	})
	if publishLits != 1 {
		t.Fatalf("%s：PublishArgs 在 %d 處被組出來，預期只有 enqueuePublish 一處", relay, publishLits)
	}
	if len(out) == 0 {
		t.Fatalf("%s：找不到任何 enqueuePublish", relay)
	}
	return out
}

// topicMatch 是 AMQP topic exchange 的比對：* 一個字，# 零個以上的字。
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

func TestRelayRoutesAreBoundAndHandled(t *testing.T) {
	topo := loadTopology(t)
	var routes []relayRoute
	for _, relay := range []string{"sellerrelay", "buyerrelay"} {
		routes = append(routes, relayRoutes(t, relay)...)
	}

	subs := buildDynamicSubscriptions(NewTemplateCache(nil), nil, nil, "https://cdn.example", zap.NewNop(), vendorInviteDeps{})
	handledQueue := map[string]string{} // exchange → notification-service 消費它的 queue
	for _, s := range subs {
		if s.Handler != nil {
			handledQueue[s.Exchange] = s.Queue
		}
	}

	for _, r := range routes {
		t.Run(r.relay+"/"+r.exchange+"/"+r.key, func(t *testing.T) {
			var bound []string
			for _, q := range topo.Queues {
				for _, b := range q.Bindings {
					if b.Exchange == r.exchange && topicMatch(b.Key, r.key) {
						bound = append(bound, q.Name)
						break
					}
				}
			}
			sort.Strings(bound)
			if len(bound) == 0 {
				t.Fatalf("topology.yaml 沒有任何 queue 綁到 %s/%s", r.exchange, r.key)
			}
			queue, ok := handledQueue[r.exchange]
			if !ok {
				t.Fatalf("notification-service 沒有訂 exchange %s 的 handler（notificationQueueByExchange）", r.exchange)
			}
			found := false
			for _, q := range bound {
				found = found || q == queue
			}
			if !found {
				t.Errorf("%s/%s 沒有被 notification 的 queue %s 綁到（綁到的 queue：%v）", r.exchange, r.key, queue, bound)
			}
		})
	}
}
