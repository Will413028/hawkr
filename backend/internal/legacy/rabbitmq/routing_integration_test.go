//go:build integration

package rabbitmq

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.yaml.in/yaml/v3"
)

// 兩個 relay 送出的每個 (exchange, routing key) 經真的 Publisher 送進 broker 後，要落在 infra/mq/topology/topology.yaml
// 宣告的 queue（exchange、queue、binding 都是讀檔宣告，不手抄），而且 body 一字不差。測試放在這裡而不是 relay 的 package：
// relay import 本 package，relay 的測試反過來 import 這裡的 startBroker 會成環。
//
// key 清單由 relay 的原始碼導出（enqueuePublish 呼叫的字面值），不手抄：relay 新增一個 key 就多一列，沒有人需要記得改這裡。
// 欄位正確性不在這裡：sellerrelay 的 wire_test 對照舊 publisher 的原始碼。終點：最後一個 relay 刪除時刪掉本檔。

const topologyFile = "../../../infra/mq/topology/topology.yaml"

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

type routeRow struct{ relay, exchange, key string }

// relayRoutes 讀 relay 原始碼，列出每個 enqueuePublish(ctx, _, exchange, key, payload) 呼叫的 exchange 與 routing key。
// 任何一個參數不是字面值（或 const），或 PublishArgs 在 enqueuePublish 以外被組出來，就失敗：那代表這個掃描看不到全部的 key。
func relayRoutes(t *testing.T, relay string) []routeRow {
	t.Helper()
	path := filepath.Join("..", relay, "relay.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
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
	var rows []routeRow
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
				rows = append(rows, routeRow{relay, ex, key})
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
	if len(rows) == 0 {
		t.Fatalf("%s：找不到任何 enqueuePublish", relay)
	}
	return rows
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

func TestRelayRoutingKeysReachTopologyQueues(t *testing.T) {
	doc := loadTopologyDoc(t)
	var rows []routeRow
	for _, relay := range []string{"sellerrelay", "buyerrelay"} {
		rows = append(rows, relayRoutes(t, relay)...)
	}

	url := startBroker(t)
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range doc.Exchanges {
		if err := ch.ExchangeDeclare(e.Name, e.Type, true, false, false, false, nil); err != nil {
			t.Fatalf("宣告 exchange %s：%v", e.Name, err)
		}
	}
	for _, q := range doc.Queues {
		if _, err := ch.QueueDeclare(q.Name, true, false, false, false, nil); err != nil {
			t.Fatalf("宣告 queue %s：%v", q.Name, err)
		}
		for _, b := range q.Bindings {
			if err := ch.QueueBind(q.Name, b.Key, b.Exchange, false, nil); err != nil {
				t.Fatalf("綁定 %s <- %s/%s：%v", q.Name, b.Exchange, b.Key, err)
			}
		}
	}

	pub, err := New(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pub.Close() })

	for _, row := range rows {
		t.Run(row.relay+"/"+row.exchange+"/"+row.key, func(t *testing.T) {
			var want []string
			for _, q := range doc.Queues {
				for _, b := range q.Bindings {
					if b.Exchange == row.exchange && topicMatch(b.Key, row.key) {
						want = append(want, q.Name)
						break
					}
				}
			}
			sort.Strings(want)
			if len(want) == 0 {
				t.Fatalf("topology.yaml 沒有任何 queue 綁到 %s/%s：這個事件送出去沒有人收", row.exchange, row.key)
			}
			body := []byte(fmt.Sprintf(`{"event_id":"routing-%s-%s","note":"路由測試，內容不重要"}`, row.relay, row.key))
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := pub.Publish(ctx, row.exchange, row.key, body); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, q := range doc.Queues {
				msg, ok, err := ch.Get(q.Name, true)
				if err != nil {
					t.Fatal(err)
				}
				if !ok {
					continue
				}
				got = append(got, q.Name)
				if string(msg.Body) != string(body) {
					t.Errorf("queue %s 收到的 body 不同：\n got  %s\n want %s", q.Name, msg.Body, body)
				}
			}
			sort.Strings(got)
			if !slices.Equal(got, want) {
				t.Errorf("%s/%s 落在的 queue 與 topology.yaml 的 binding 不符：\n got  %v\n want %v", row.exchange, row.key, got, want)
			}
		})
	}
}
