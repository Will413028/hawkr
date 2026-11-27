//go:build integration

package rabbitmq

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)


func TestPublishedRoutesReachTopologyQueues(t *testing.T) {
	doc := loadTopologyDoc(t)
	rows := publishRoutes(t)

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
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	mirror := make([]string, len(doc.Queues))
	for i, q := range doc.Queues {
		mirror[i] = fmt.Sprintf("routetest.%s.%d", runID, i)
		if _, err := ch.QueueDeclare(mirror[i], false, true, true, false, nil); err != nil {
			t.Fatalf("宣告鏡像 queue %s：%v", q.Name, err)
		}
		for _, b := range q.Bindings {
			if err := ch.QueueBind(mirror[i], b.Key, b.Exchange, false, nil); err != nil {
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
		t.Run(row.Package+"/"+row.Exchange+"/"+row.Key, func(t *testing.T) {
			want := doc.boundQueues(row.Exchange, row.Key)
			if len(want) == 0 {
				t.Fatalf("topology.yaml 沒有任何 queue 綁到 %s/%s：這個事件送出去沒有人收", row.Exchange, row.Key)
			}
			body := []byte(fmt.Sprintf(`{"event_id":"routing-%s-%s","note":"路由測試，內容不重要"}`, runID, row.Key))
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := pub.Publish(ctx, row.Exchange, row.Key, body); err != nil {
				t.Fatal(err)
			}
			var got []string
			for i, q := range doc.Queues {
				msg, ok, err := ch.Get(mirror[i], true)
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
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("%s/%s 落在的 queue 與 topology.yaml 的 binding 不符：\n got  %v\n want %v", row.Exchange, row.Key, got, want)
			}
		})
	}
}
