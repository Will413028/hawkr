package slots

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

func protoEnumMembers(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../../../../../../api/hawkr/procurement/v1alpha1/procurement.proto")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)enum RejectionReason \{(.*?)\n\}`).FindSubmatch(b)
	if block == nil {
		t.Fatal("proto 裡找不到 enum RejectionReason")
	}
	var out []string
	for _, m := range regexp.MustCompile(`(?m)^\s*(REJECTION_REASON_\w+)\s*=`).FindAllSubmatch(block[1], -1) {
		out = append(out, string(m[1]))
	}
	return out
}

func TestReasonProtoMappingIsExhaustive(t *testing.T) {
	members := protoEnumMembers(t)
	if len(members) < 2 {
		t.Fatalf("解析到的成員太少：%v", members)
	}
	src, err := os.ReadFile("slots.go")
	if err != nil {
		t.Fatal(err)
	}
	consts := regexp.MustCompile(`Reason = "([A-Z_]+)"`).FindAllSubmatch(src, -1)
	if len(consts) != len(Reasons()) {
		t.Errorf("slots.go 宣告 %d 個 Reason，對照表有 %d 個", len(consts), len(Reasons()))
	}
	mapped := map[string]bool{}
	for _, r := range Reasons() {
		name := r.ProtoName()
		if name == "" || name == "REJECTION_REASON_UNSPECIFIED" || !slices.Contains(members, name) {
			t.Errorf("Reason %q 對到 %q，不是 proto 的有效成員", r, name)
		}
		mapped[name] = true
	}
	for _, m := range members {
		if m != "REJECTION_REASON_UNSPECIFIED" && !mapped[m] {
			t.Errorf("proto 成員 %s 沒有對應的 Reason", m)
		}
	}
	if Reason("NOPE").ProtoName() != "" {
		t.Error("未知的 Reason 要回空字串")
	}
}
