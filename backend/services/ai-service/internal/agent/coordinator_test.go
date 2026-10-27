package agent

import (
	"strings"
	"testing"
)

// coordinator instruction 必須含委派規則與退款轉真人守則，防被意外簡化。
func TestCoordinatorInstruction_KeyPhrases(t *testing.T) {
	cases := []struct {
		name    string
		instr   string
		phrases []string
	}{
		{"seller", sellerCoordinatorInstruction, []string{
			"order_specialist", "inventory_specialist", "payment_specialist", "price_specialist",
			"依序呼叫", "退款", "禁止自行編造",
		}},
		{"buyer", buyerCoordinatorInstruction, []string{
			"buyer_order_specialist", "buyer_tracking_specialist", "buyer_seasonal_specialist",
			"依序呼叫", "禁止自行編造", "轉接真人",
		}},
	}
	for _, c := range cases {
		for _, p := range c.phrases {
			if !strings.Contains(c.instr, p) {
				t.Errorf("%s coordinator instruction 缺關鍵片段 %q", c.name, p)
			}
		}
	}
}
