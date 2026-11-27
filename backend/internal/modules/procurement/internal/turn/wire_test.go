package turn

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)


func goldenIDs() ids {
	mk := func(s string) uuid.UUID { return uuid.MustParse(s) }
	return ids{
		request: mk("0192a000-0000-7000-8000-000000000001"), conversation: mk("0192a000-0000-7000-8000-000000000002"),
		revision: mk("0192a000-0000-7000-8000-000000000003"), turn: mk("0192a000-0000-7000-8000-000000000004"), run: mk("0192a000-0000-7000-8000-000000000005"),
	}
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, got, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, pretty.Bytes()) {
		t.Errorf("%s 與產生器的輸出不同（刻意改線格式時 UPDATE_GOLDEN=1 重寫）：\n%s", name, pretty.String())
	}
}

func TestWireGolden(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(18, "小白菜", "葉菜 > 小白菜")}}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{
		Item: sp("小白菜"), Unit: sp("kg"), DeliveryCity: sp("台北"), Quantity: ip(50), BudgetPerUnit: ip(30), DeliveryDate: sp("2026-12-01"),
	})
	ev, err := turnResultEvent(d, goldenIDs(), now, uuid.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "turn_result.golden.json", ev)

	rs = &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(2, "蘋果", "加工品 > 蘋果")}}}
	d = decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("蘋果"), Quantity: ip(5), DeliveryDate: sp("2020-01-01")})
	ev, err = turnResultEvent(d, goldenIDs(), now, uuid.MustParse("0192a000-0000-7000-8000-000000000006"))
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "turn_result_interrupt.golden.json", ev)
	payload, err := json.Marshal(d.interrupt)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "interrupt_payload.golden.json", payload)

	group := buildPayload(buildCard("蘋果", spread(6, "水果", "加工品")), uitext.ZhTW)
	payload, err = json.Marshal(group)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "interrupt_payload_group.golden.json", payload)

	payload, err = json.Marshal(buildPayload(describeCard("蘋果"), uitext.ZhTW))
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "interrupt_payload_describe.golden.json", payload)
}
