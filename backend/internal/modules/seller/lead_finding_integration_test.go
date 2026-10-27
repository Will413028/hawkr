//go:build integration

package seller_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// leadOf 直接讀措施的 lead_finding_id（主要依據），不經過讀取時的排序。
func (f *fixture) leadOf(t *testing.T, measureID uuid.UUID) uuid.UUID {
	t.Helper()
	var lead uuid.UUID
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT lead_finding_id FROM seller.measures WHERE id = $1", measureID).Scan(&lead))
	return lead
}

// newFindingAt 在 f.now 往後一小時建立一筆指定嚴重度的認定（各由一個警告建立），回傳認定的 id；建立時間因此依呼叫順序遞增。
func (f *fixture) newFindingAt(t *testing.T, storeID int64, severity Severity) uuid.UUID {
	t.Helper()
	f.now = f.now.Add(time.Hour)
	return f.violate(t, storeID, severity, MeasureWarning, nil).Findings[0].ID
}

// citeOnly 只引用既有的認定（沒有新的認定）建立警告。
func (f *fixture) citeOnly(storeID int64, cited []uuid.UUID, mutate func(*CreateMeasureInput)) (CreateMeasureResult, error) {
	in := CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: MeasureWarning, StatementToSeller: "y", Note: "x",
		Violation: &ViolationBasis{CitedFindingIDs: cited},
	}
	if mutate != nil {
		mutate(&in)
	}
	return f.module.CreateMeasure(context.Background(), in)
}

func permutationsOf(in []uuid.UUID) [][]uuid.UUID {
	if len(in) <= 1 {
		return [][]uuid.UUID{slices.Clone(in)}
	}
	var out [][]uuid.UUID
	for i := range in {
		rest := slices.Concat(in[:i:i], in[i+1:])
		for _, p := range permutationsOf(rest) {
			out = append(out, append([]uuid.UUID{in[i]}, p...))
		}
	}
	return out
}

// 只引用既有認定（沒有新的認定）時，主要依據是引用的認定中最嚴重的，同嚴重度取最新建立的；引用清單怎麼排都一樣
// （CRITICAL > HIGH > MEDIUM > LOW；比較新但比較不嚴重的不會取代）。
func TestLeadFindingOfACitedOnlyMeasureIsTheMostSevereThenTheNewestWhateverTheOrder(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	low := f.newFindingAt(t, s.StoreID, SeverityLow)
	high1 := f.newFindingAt(t, s.StoreID, SeverityHigh)
	critical1 := f.newFindingAt(t, s.StoreID, SeverityCritical)
	critical2 := f.newFindingAt(t, s.StoreID, SeverityCritical)
	high2 := f.newFindingAt(t, s.StoreID, SeverityHigh)
	medium := f.newFindingAt(t, s.StoreID, SeverityMedium)

	for name, c := range map[string]struct {
		cited []uuid.UUID
		want  uuid.UUID
	}{
		"只有一筆":            {[]uuid.UUID{low}, low},
		"HIGH 比 LOW 嚴重":   {[]uuid.UUID{low, high1}, high1},
		"MEDIUM 比 LOW 嚴重": {[]uuid.UUID{low, medium}, medium},
		"同是 HIGH 取最新的（HIGH 的 high2 最晚）": {[]uuid.UUID{high1, high2}, high2},
		"CRITICAL 勝過比較新的 HIGH 與 MEDIUM": {[]uuid.UUID{critical1, high2, medium}, critical1},
		"同是 CRITICAL 取最新的":              {[]uuid.UUID{critical1, critical2, high2}, critical2},
	} {
		for _, order := range permutationsOf(c.cited) {
			res, err := f.citeOnly(s.StoreID, order, nil)
			must(t, err)
			if got := f.leadOf(t, res.Measure.ID); got != c.want {
				t.Errorf("%s：引用 %v 的主要依據是 %v，應為 %v", name, order, got, c.want)
			}
			if len(res.Measure.Findings) != len(c.cited) || res.Measure.Findings[0].ID != c.want {
				t.Errorf("%s：引用 %v 的措施讀到的依據 %+v，第一筆應為主要依據 %v", name, order, res.Measure.Findings, c.want)
			}
		}
	}
}

// 有新的認定時，主要依據永遠是新的認定，引用的認定再嚴重也不取代它，引用清單的排列也不影響。
func TestANewFindingIsAlwaysTheLead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	critical := f.newFindingAt(t, s.StoreID, SeverityCritical)
	high := f.newFindingAt(t, s.StoreID, SeverityHigh)
	for _, order := range permutationsOf([]uuid.UUID{critical, high}) {
		res, err := f.citeOnly(s.StoreID, order, func(in *CreateMeasureInput) {
			in.Violation.Finding = &NewFinding{Category: policy.Fraud, Severity: SeverityLow}
		})
		must(t, err)
		m := res.Measure
		lead := f.leadOf(t, m.ID)
		if lead == critical || lead == high {
			t.Fatalf("引用 %v：主要依據是引用的認定 %v，應為新的認定", order, lead)
		}
		created := m.Findings[0]
		if created.OriginMeasureID != m.ID || created.Severity != SeverityLow || created.ID != lead || len(m.Findings) != 3 {
			t.Fatalf("引用 %v：lead=%v findings=%+v", order, lead, m.Findings)
		}
	}
}

// 重大違約的終止合作（沒有新的認定，依據是引用的認定）：主要依據取引用的認定中最嚴重的，不取決於勾選的第一筆。
func TestMaterialBreachLeadIsTheMostSevereCitedFindingNotTheFirstOne(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	low := f.newFindingAt(t, s.StoreID, SeverityLow)
	critical := f.newFindingAt(t, s.StoreID, SeverityCritical)
	high := f.newFindingAt(t, s.StoreID, SeverityHigh)
	f.now = f.now.Add(8 * 24 * time.Hour)
	// 第一筆是最輕的 LOW、最嚴重的 CRITICAL 在中間。
	res, err := f.createTermination(s.StoreID, GroundMaterialBreach, func(in *CreateMeasureInput) {
		in.Termination.Violation = &ViolationBasis{CitedFindingIDs: []uuid.UUID{low, critical, high}}
	})
	must(t, err)
	if got := f.leadOf(t, res.Measure.ID); got != critical {
		t.Fatalf("主要依據 %v，應為最嚴重的 CRITICAL %v", got, critical)
	}
	if res.Measure.Basis != BasisViolation || len(res.Measure.Findings) != 3 || res.Measure.Findings[0].ID != critical {
		t.Fatalf("m=%+v", res.Measure)
	}
}

// 冪等：只引用既有認定時，同一個 request_id 帶同一組認定、任何排列（包含換了第一筆）都是同一個請求，回傳原本的措施（同一個主要依據）；
// 少一筆或換一筆是不同的內容。
func TestCreateMeasureRequestIDIgnoresTheOrderOfCitedFindingsWithoutANewFinding(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	low := f.newFindingAt(t, s.StoreID, SeverityLow)
	high := f.newFindingAt(t, s.StoreID, SeverityHigh)
	critical := f.newFindingAt(t, s.StoreID, SeverityCritical)
	cited := []uuid.UUID{low, high, critical}
	requestID := uuid.New()
	withID := func(in *CreateMeasureInput) { in.RequestID = &requestID }

	first, err := f.citeOnly(s.StoreID, cited, withID)
	must(t, err)
	if first.Replayed {
		t.Fatal("第一次不是重送")
	}
	lead := f.leadOf(t, first.Measure.ID)
	if lead != critical {
		t.Fatalf("主要依據 %v，應為 %v", lead, critical)
	}
	before := f.count(t, "SELECT count(*) FROM seller.measures")
	for _, order := range permutationsOf(cited) {
		again, err := f.citeOnly(s.StoreID, order, withID)
		must(t, err)
		if !again.Replayed || again.Measure.ID != first.Measure.ID {
			t.Fatalf("引用 %v：排列不同的同一組認定要回傳原本的措施：%+v", order, again)
		}
		if got := f.leadOf(t, again.Measure.ID); got != lead {
			t.Fatalf("引用 %v：主要依據 %v，應與第一次相同 %v", order, got, lead)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != before {
		t.Fatalf("measures=%d，重送不能再建一個（原本 %d）", n, before)
	}

	for name, set := range map[string][]uuid.UUID{"少一筆": {low, high}, "換一筆": {low, high, f.newFindingAt(t, s.StoreID, SeverityLow)}} {
		_, err := f.citeOnly(s.StoreID, set, withID)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "RequestID" {
			t.Fatalf("%s：同 id 不同內容要回 invalid_argument 指到 RequestID：%v", name, err)
		}
	}
}
