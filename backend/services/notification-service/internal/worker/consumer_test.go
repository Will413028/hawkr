package worker

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// topologyPath locates infra/mq/topology/topology.yaml relative to this package
// (services/notification-service/internal/worker → repo root is 4 levels up).
//
// 2026-07-26: this used to point at topology.json, the checked-in flattened
// artifact. That file was removed when infra/mq moved to topology.yaml as the
// single declarative source (P1 of the mq-topology-yaml migration) — see
// infra/mq/README.md and docs/基礎建設/MQ Topology 管理（Terraform）.md.
const topologyPath = "../../../../infra/mq/topology/topology.yaml"

// mqTopology mirrors the subset of topology.yaml's schema this test needs: which
// exchanges exist and with what type, and which queues exist with what retry
// policy and bindings. It intentionally does not model `arguments`, `dlq`, or
// anything else expand.py derives — see the comment on
// TestNotificationQueuesExistInTerraformTopology for why.
type mqTopology struct {
	Exchanges []struct {
		Name string `yaml:"name"`
		Type string `yaml:"type"`
	} `yaml:"exchanges"`
	Queues []struct {
		Name  string `yaml:"name"`
		Retry *struct {
			MaxAttempts int `yaml:"max_attempts"`
		} `yaml:"retry"`
		Bindings []struct {
			Exchange string `yaml:"exchange"`
			Key      string `yaml:"key"`
		} `yaml:"bindings"`
	} `yaml:"queues"`
}

func loadTopology(t *testing.T) mqTopology {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(topologyPath))
	if err != nil {
		t.Fatalf("read %s: %v", topologyPath, err)
	}
	var topo mqTopology
	if err := yaml.Unmarshal(raw, &topo); err != nil {
		t.Fatalf("parse %s: %v", topologyPath, err)
	}
	return topo
}

// TestNotificationQueuesExistInTerraformTopology enforces the one invariant that
// keeps this service working under passive declare: every queue this service
// subscribes to must actually be provisioned by infra/mq Terraform.
//
// Passive declare gives no runtime signal when it is violated — QueueDeclarePassive
// 404s, that subscription is created but never receives anything, and the
// notification silently never sends (it does not even reach a DLQ, because the
// queue does not exist). That is exactly how #1795 stayed invisible. Asserting it
// here turns a silent production failure into a failed build.
//
// This checks declarative *intent* in topology.yaml, not the resources that
// intent expands into. Before 2026-07-26 it additionally asserted the derived
// `.dlq` queue, the `.retry` fanout exchange, the queue's x-dead-letter-exchange
// argument, and both the retry- and DLQ-routing bindings. All five of those are
// now *caused by* the presence of a `retry: {max_attempts: 3}` policy on the
// queue entry — asserting the policy is present is enough, because that the
// policy expands into exactly those five resources is guaranteed elsewhere:
// the 8 policy-expansion unit tests in infra/mq/tools/test_expand.py, the
// 423-resource round-trip identity check against the real topology, and the
// container-verified `terraform plan` no-op on both classic and quorum queue
// types (infra/mq's P1 migration). Coverage was not dropped here — it moved up
// a layer. Re-deriving it in Go instead would be a third implementation of the
// same expansion rules that already exist in expand.py and main.tf's HCL
// locals; that duplication is deliberately not taken on a third time.
func TestNotificationQueuesExistInTerraformTopology(t *testing.T) {
	topo := loadTopology(t)

	type queueInfo struct {
		hasRetry    bool
		maxAttempts int
		bindings    map[[2]string]bool // [exchange, routing key]
	}

	queues := make(map[string]queueInfo, len(topo.Queues))
	for _, q := range topo.Queues {
		qi := queueInfo{bindings: make(map[[2]string]bool, len(q.Bindings))}
		if q.Retry != nil {
			qi.hasRetry = true
			qi.maxAttempts = q.Retry.MaxAttempts
		}
		for _, b := range q.Bindings {
			qi.bindings[[2]string{b.Exchange, b.Key}] = true
		}
		queues[q.Name] = qi
	}
	exchanges := make(map[string]string, len(topo.Exchanges))
	for _, e := range topo.Exchanges {
		exchanges[e.Name] = e.Type
	}

	for exchange, queue := range notificationQueueByExchange {
		// Source exchange must exist and be topic — "#" only means "match everything"
		// on a topic exchange.
		if got, ok := exchanges[exchange]; !ok {
			t.Errorf("exchange %q 不在 topology.yaml", exchange)
			continue
		} else if got != "topic" {
			t.Errorf("exchange %q 型別為 %q，非 topic —— # 萬用綁定不成立", exchange, got)
		}

		qi, ok := queues[queue]
		if !ok {
			t.Errorf("queue %q（exchange %q）不在 topology.yaml 的 queues: 清單 —— passive declare 下這條訂閱會靜默失效",
				queue, exchange)
			continue
		}

		// buildDynamicSubscriptions always sets MaxRetries: 3 (pinned by
		// TestBuildDynamicSubscriptionsIsDerivedFromMap below), so the queue must
		// carry a matching retry policy — that policy is what expand.py turns into
		// the .retry exchange, the .dlq queue, the dead-letter argument, and both
		// mirroring bindings.
		if !qi.hasRetry || qi.maxAttempts != 3 {
			t.Errorf("queue %q 缺 retry.max_attempts=3 policy —— .retry exchange / .dlq / "+
				"dead-letter 參數 / 對應 binding 皆由此展開，見 expand.py 測試", queue)
		}

		// Wildcard binding is what decouples the template table from infra: without
		// it, a template using a not-yet-bound routing key would silently never fire.
		if !qi.bindings[[2]string{exchange, "#"}] {
			t.Errorf("缺 binding {exchange: %q, key: \"#\"} --> %q", exchange, queue)
		}
	}
}

// TestBuildDynamicSubscriptionsIsDerivedFromMap pins the property that makes the
// subscription set stable: it comes from notificationQueueByExchange, not from
// whatever templates happen to be loaded, so an empty/stale cache cannot silently
// shrink what this service consumes.
func TestBuildDynamicSubscriptionsIsDerivedFromMap(t *testing.T) {
	subs := buildDynamicSubscriptions(
		NewTemplateCache(nil), nil, nil, "https://cdn.example", zap.NewNop(), vendorInviteDeps{})

	if len(subs) != len(notificationQueueByExchange) {
		t.Fatalf("subscription 數 = %d，預期 %d（應與 map 等長，與 cache 內容無關）",
			len(subs), len(notificationQueueByExchange))
	}
	for _, s := range subs {
		if want := notificationQueueByExchange[s.Exchange]; s.Queue != want {
			t.Errorf("exchange %q 對到 queue %q，預期 %q", s.Exchange, s.Queue, want)
		}
		if len(s.RoutingKeys) != 1 || s.RoutingKeys[0] != "#" {
			t.Errorf("exchange %q 的 routing keys = %v，預期 [#]", s.Exchange, s.RoutingKeys)
		}
		if s.MaxRetries != 3 {
			t.Errorf("exchange %q 的 MaxRetries = %d，預期 3", s.Exchange, s.MaxRetries)
		}
	}
}
