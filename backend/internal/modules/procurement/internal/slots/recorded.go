package slots

import (
	"strconv"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
)

// Recorded 是這一輪新記下的一個欄位。Value 是 Apply 後的正典值的顯示字串；ExactMatchRequired 是 "true" 或 "false"，
// 由 reply 依語系轉成買家看的字。
type Recorded struct {
	Key   model.SlotKey
	Value string
}

var order = []model.SlotKey{
	Item, ItemResolution, Quantity, Unit, Packaging,
	BudgetPerUnit, DeliveryCity, DeliveryDate, ExactMatchRequired,
}

// RecordedThisTurn 算出這一輪要顯示的欄位：patch 有帶、而且 Apply 後真的跟 current 不同的欄位，值讀 next。
// 用「前後是否不同」而不是看 rejected 清單：截斷但有採用的欄位也是這輪記下的，完全丟棄的不是。
// ExactMatchRequired 只有買家明確設定的值才算記下；nil 是系統預設，不算。這輪只看 patch 是否帶它。
func RecordedThisTurn(patch Patch, current, next Set) []Recorded {
	out := []Recorded{}
	for _, k := range order {
		if v, ok := changed(k, patch, current, next); ok {
			out = append(out, Recorded{Key: k, Value: v})
		}
	}
	return out
}

func changed(key model.SlotKey, p Patch, cur, next Set) (string, bool) {
	switch key {
	case Item:
		if p.Item == nil || equalPtr(cur.Item, next.Item) {
			return "", false
		}
		return deref(next.Item), true
	case ItemResolution:
		if p.Resolution == nil || equalPtr(cur.Resolution, next.Resolution) || next.Resolution == nil {
			return "", false
		}
		return next.Resolution.DisplayValue(), true
	case Packaging:
		if p.Packaging == nil || equalPtr(cur.Packaging, next.Packaging) {
			return "", false
		}
		return deref(next.Packaging), true
	case DeliveryCity:
		if p.DeliveryCity == nil || equalPtr(cur.DeliveryCity, next.DeliveryCity) {
			return "", false
		}
		return deref(next.DeliveryCity), true
	case DeliveryDate:
		if p.DeliveryDate == nil || equalPtr(cur.DeliveryDate, next.DeliveryDate) || next.DeliveryDate == nil {
			return "", false
		}
		return next.DeliveryDate.Format(DateLayout), true
	case Unit:
		if p.Unit == nil || equalPtr(cur.Unit, next.Unit) {
			return "", false
		}
		return deref(next.Unit), true
	case Quantity:
		if p.Quantity == nil || equalPtr(cur.Quantity, next.Quantity) || next.Quantity == nil {
			return "", false
		}
		return strconv.FormatInt(int64(*next.Quantity), 10), true
	case BudgetPerUnit:
		if p.BudgetPerUnit == nil || equalPtr(cur.BudgetPerUnit, next.BudgetPerUnit) || next.BudgetPerUnit == nil {
			return "", false
		}
		return strconv.FormatInt(*next.BudgetPerUnit, 10), true
	case ExactMatchRequired:
		if p.ExactMatchRequired == nil || equalPtr(cur.ExactMatchRequired, next.ExactMatchRequired) || next.ExactMatchRequired == nil {
			return "", false
		}
		return strconv.FormatBool(*next.ExactMatchRequired), true
	}
	return "", false
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
