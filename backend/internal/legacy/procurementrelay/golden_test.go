package procurementrelay

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"slices"
	"testing"
	"uuid"
)

const (
	goldenFile  = "../../../tests/contract/golden/event_payload_keys.json"
	fixtureFile = "../../../docs/plans/2026-11-19-a9-agent/fixtures/events/invite_created.json"
)

type fixture struct {
	Exchange   string            `json:"exchange"`
	RoutingKey string            `json:"routing_key"`
	Payload    map[string]string `json:"payload"`
}

func loadFixture(t *testing.T) fixture {
	t.Helper()
	b, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func produced(t *testing.T) map[string]any {
	t.Helper()
	body, err := payload(args())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// 產生的 payload 與 fixture 逐欄位比對：鍵名、值的 JSON 型別（全是字串）、沒有多餘的鍵；值的格式另測（TSID 變 UUID，A9-D45(6)）。
func TestPayloadMatchesTheGoldenFixtureFieldByField(t *testing.T) {
	fx, got := loadFixture(t), produced(t)
	if fx.Exchange != exchangeProcurement || fx.RoutingKey != routingKeyInviteMade {
		t.Fatalf("fixture 的 exchange/routing key = %s/%s", fx.Exchange, fx.RoutingKey)
	}
	if len(got) != len(fx.Payload) {
		t.Fatalf("keys: got %v, fixture %v", got, fx.Payload)
	}
	for k := range fx.Payload {
		v, ok := got[k]
		if !ok {
			t.Fatalf("payload 缺 fixture 的鍵 %q：%v", k, got)
		}
		if _, isString := v.(string); !isString {
			t.Fatalf("payload[%q] 型別 %T，fixture 是字串", k, v)
		}
	}
}

func TestValueFormatsAreUUIDsForInvitesAndDecimalForTheStore(t *testing.T) {
	got := produced(t)
	for _, k := range []string{"inviteId", "publicationId"} {
		if _, err := uuid.Parse(got[k].(string)); err != nil {
			t.Fatalf("%s = %v 不是 UUID", k, got[k])
		}
	}
	if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(got["storeId"].(string)) {
		t.Fatalf("storeId = %v 不是十進位字串", got["storeId"])
	}
}

func TestPayloadKeysMatchTheGoldenEmittedKeys(t *testing.T) {
	b, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]struct {
		RoutingKey  string   `json:"routing_key"`
		EmittedKeys []string `json:"emitted_keys"`
	}
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	entry := golden["procurement.rfq_publication_invite.created.seller"]
	if entry.RoutingKey != routingKeyInviteMade {
		t.Fatalf("golden routing key = %q", entry.RoutingKey)
	}
	var keys []string
	for k := range produced(t) {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	want := slices.Clone(entry.EmittedKeys)
	slices.Sort(want)
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("payload keys %v, golden emitted_keys %v", keys, want)
	}
}
