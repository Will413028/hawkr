package measure

import (
	"bytes"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// ids 回傳 n 個依位元組遞增排序的固定 UUID（a < b < c ...），測試用它們組出各種順序。
func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.MustParse(fmt.Sprintf("0190a000-0000-7000-8000-%012x", i+1))
	}
	return out
}

// violationInput 是依據認定的警告輸入；finding 為 nil 時沒有新的認定（只引用既有的）。
func violationInput(finding *model.NewFinding, cited ...uuid.UUID) model.CreateMeasureInput {
	return model.CreateMeasureInput{
		AdminID: 7, StoreID: 100, Kind: model.MeasureWarning, StatementToSeller: "商品頁宣稱有機", Note: "客服轉來兩筆檢舉",
		Violation: &model.ViolationBasis{Finding: finding, CitedFindingIDs: cited},
	}
}

func newFinding(sources ...uuid.UUID) *model.NewFinding {
	return &model.NewFinding{Category: policy.Fraud, Severity: model.SeverityHigh, SourceReportIDs: sources}
}

// terminationInput 是重大違約的終止合作輸入：沒有新的認定，依據是引用的認定。
func terminationInput(cited ...uuid.UUID) model.CreateMeasureInput {
	return model.CreateMeasureInput{
		AdminID: 7, StoreID: 100, Kind: model.MeasureTermination, StatementToSeller: "未改善", Note: "內部",
		Termination: &model.TerminationBasis{
			Ground:    model.GroundMaterialBreach,
			Violation: &model.ViolationBasis{CitedFindingIDs: cited},
		},
	}
}

func permutations(in []uuid.UUID) [][]uuid.UUID {
	if len(in) <= 1 {
		return [][]uuid.UUID{slices.Clone(in)}
	}
	var out [][]uuid.UUID
	for i := range in {
		rest := slices.Concat(in[:i:i], in[i+1:])
		for _, p := range permutations(rest) {
			out = append(out, append([]uuid.UUID{in[i]}, p...))
		}
	}
	return out
}

// 來源通報是集合：同一組通報不論順序，request hash 都相同；不同的組合（少一筆、換一筆、多一筆）不同。
func TestRequestHashIgnoresTheOrderOfSourceReports(t *testing.T) {
	r := ids(4)
	want := requestHash(violationInput(newFinding(r[0], r[1], r[2])))
	perms := permutations(r[:3])
	if len(perms) != 6 {
		t.Fatalf("permutations=%d", len(perms))
	}
	for _, p := range perms {
		if got := requestHash(violationInput(newFinding(p...))); !bytes.Equal(got, want) {
			t.Errorf("來源通報 %v 的 hash 應與升序相同", p)
		}
	}
	for name, other := range map[string][]uuid.UUID{
		"少一筆":  {r[0], r[1]},
		"換一筆":  {r[0], r[1], r[3]},
		"多一筆":  {r[0], r[1], r[2], r[3]},
		"沒有來源": nil,
	} {
		if bytes.Equal(requestHash(violationInput(newFinding(other...))), want) {
			t.Errorf("%s：不同的來源通報組合不能有相同的 hash", name)
		}
	}
}

// 引用的既有認定是集合：有新的認定時，順序不同的同一組 hash 相同，不同的組合不同。
func TestRequestHashIgnoresTheOrderOfCitedFindingsWithANewFinding(t *testing.T) {
	f := ids(4)
	source := ids(1)
	want := requestHash(violationInput(newFinding(source...), f[0], f[1], f[2]))
	for _, p := range permutations(f[:3]) {
		if got := requestHash(violationInput(newFinding(source...), p...)); !bytes.Equal(got, want) {
			t.Errorf("引用的認定 %v 的 hash 應與升序相同", p)
		}
	}
	for name, other := range map[string][]uuid.UUID{
		"少一筆":  {f[0], f[1]},
		"換一筆":  {f[0], f[1], f[3]},
		"沒有引用": nil,
	} {
		if bytes.Equal(requestHash(violationInput(newFinding(source...), other...)), want) {
			t.Errorf("%s：不同的引用組合不能有相同的 hash", name)
		}
	}
	// 來源通報與引用的認定是兩個欄位：同一個 id 放在不同欄位不是同一個請求。
	if bytes.Equal(requestHash(violationInput(newFinding(f[0]))), requestHash(violationInput(newFinding(), f[0]))) {
		t.Error("同一個 id 在來源通報與引用的認定，hash 不能相同")
	}
}

// 沒有新的認定（只引用既有的：重大違約的終止合作，或只引用既有認定的警告）時也一樣，沒有「第一筆」的例外：主要依據由規則決定
// （最嚴重、同嚴重度取最新建立的，見 findings 與 model.LeadFindingOf），不編碼在位置裡，所以任何排列都是同一個請求。
func TestRequestHashIgnoresTheOrderOfCitedFindingsWithoutANewFinding(t *testing.T) {
	f := ids(4)
	inputs := map[string]func(cited ...uuid.UUID) model.CreateMeasureInput{
		"重大違約的終止合作":  terminationInput,
		"只引用既有認定的警告": func(cited ...uuid.UUID) model.CreateMeasureInput { return violationInput(nil, cited...) },
	}
	for name, build := range inputs {
		want := requestHash(build(f[0], f[1], f[2]))
		for _, p := range permutations(f[:3]) {
			if got := requestHash(build(p...)); !bytes.Equal(got, want) {
				t.Errorf("%s：引用的認定 %v 的 hash 應與升序相同（包含換了第一筆的排列）", name, p)
			}
		}
		for other, set := range map[string][]uuid.UUID{
			"少一筆": {f[0], f[1]},
			"換一筆": {f[0], f[1], f[3]},
			"多一筆": {f[0], f[1], f[2], f[3]},
		} {
			if bytes.Equal(requestHash(build(set...)), want) {
				t.Errorf("%s：%s，不同的引用組合不能有相同的 hash", name, other)
			}
		}
	}
	// 沒有新的認定與有新的認定是不同的請求（主要依據的來源不同）。
	if bytes.Equal(requestHash(violationInput(nil, f[0])), requestHash(violationInput(newFinding(), f[0]))) {
		t.Error("有沒有新的認定要反映在 hash")
	}
}

// 排序不改動呼叫端的輸入：欄位錯誤要指到輸入的原始位置（SourceReportIDs[i]、CitedFindingIDs[i]）。
func TestRequestHashDoesNotReorderTheCallersLists(t *testing.T) {
	r := ids(3)
	reversed := []uuid.UUID{r[2], r[1], r[0]}
	sources := slices.Clone(reversed)
	cited := slices.Clone(reversed)
	requestHash(violationInput(newFinding(sources...), cited...))
	requestHash(terminationInput(cited...))
	if !slices.Equal(sources, reversed) || !slices.Equal(cited, reversed) {
		t.Fatalf("輸入被改動：sources=%v cited=%v", sources, cited)
	}
}

// 集合以外的欄位仍然各自區分請求（排序只影響 id 清單）。
func TestRequestHashStillSeparatesEveryOtherField(t *testing.T) {
	r := ids(2)
	base := violationInput(newFinding(r[0]))
	want := requestHash(base)
	if !bytes.Equal(requestHash(base), want) {
		t.Fatal("同一份輸入的 hash 要穩定")
	}
	end := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	changes := map[string]func(in *model.CreateMeasureInput){
		"店家":     func(in *model.CreateMeasureInput) { in.StoreID = 101 },
		"種類":     func(in *model.CreateMeasureInput) { in.Kind = model.MeasureSuspension },
		"給賣家的理由": func(in *model.CreateMeasureInput) { in.StatementToSeller = "別的理由" },
		"內部註記":   func(in *model.CreateMeasureInput) { in.Note = "別的註記" },
		"結束時間":   func(in *model.CreateMeasureInput) { in.EndsAt = &end },
		"類別":     func(in *model.CreateMeasureInput) { in.Violation.Finding.Category = policy.Counterfeit },
		"嚴重度":    func(in *model.CreateMeasureInput) { in.Violation.Finding.Severity = model.SeverityLow },
	}
	for name, change := range changes {
		in := violationInput(newFinding(r[0]))
		change(&in)
		if bytes.Equal(requestHash(in), want) {
			t.Errorf("%s：改了之後 hash 必須不同", name)
		}
	}
}
