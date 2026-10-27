package measure

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// API 的 TerminationOption.violation_requirement 與 CreateMeasure 的無狀態驗證是同一個規則（model.TerminationGround.ViolationRequirement）：
// 每個事由 ×（沒有依據／依據只有新認定／依據引用既有認定），validateTermination 通過 ⇔ 需求允許這種輸入。
// 另外釘住每個事由的需求（獨立於實作的預期表），不然「兩邊一起改錯」不會被等價比對發現。
// mutation：ViolationRequirement 把某個 21.3 事由改成 REQUIRED → 預期表紅；validateTermination 改回自己的 switch 而規則不同 → 等價比對紅。
func TestTerminationViolationRequirementIsTheSameRuleAsCreateMeasure(t *testing.T) {
	wantRequirement := map[model.TerminationGround]model.TerminationViolationRequirement{
		model.GroundNotice:             model.ViolationForbidden,
		model.GroundLinkedAccount:      model.ViolationForbidden,
		model.GroundMaterialBreach:     model.ViolationRequired,
		model.GroundCeasedBusiness:     model.ViolationOptional,
		model.GroundDissolution:        model.ViolationOptional,
		model.GroundBankruptcy:         model.ViolationOptional,
		model.GroundLicenseRevoked:     model.ViolationOptional,
		model.GroundFoodSafetyIncident: model.ViolationOptional,
		model.GroundFraud:              model.ViolationOptional,
		model.GroundPlatformSecurity:   model.ViolationOptional,
		model.GroundOtherSerious:       model.ViolationOptional,
	}
	if len(wantRequirement) != len(model.TerminationGroundValues()) {
		t.Fatalf("預期表要涵蓋全部事由：%d 對 %d", len(wantRequirement), len(model.TerminationGroundValues()))
	}
	// 引用的認定須存在多久：需求是 REQUIRED 的事由是 MaterialBreachCure，其他沒有（錯誤句、說明句與有狀態的檢查共用 CitedFindingMinAge）。
	for _, g := range model.TerminationGroundValues() {
		want := time.Duration(0)
		if wantRequirement[g] == model.ViolationRequired {
			want = model.MaterialBreachCure
		}
		if got := g.CitedFindingMinAge(); got != want {
			t.Errorf("%s 的 CitedFindingMinAge = %s，期望 %s", g, got, want)
		}
	}
	// 錯誤句的天數取自同一個來源：沒有引用認定的 REQUIRED 事由，句子含 MaterialBreachCure 的天數。
	err := validateTermination(&model.CreateMeasureInput{Kind: model.MeasureTermination, Termination: &model.TerminationBasis{Ground: model.GroundMaterialBreach}})
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("已存在 %d 日以上", int(model.MaterialBreachCure/(24*time.Hour)))) {
		t.Errorf("錯誤句要帶 MaterialBreachCure 的天數：%v", err)
	}
	cited := []uuid.UUID{uuid.MustParse("0190a000-0000-7000-8000-000000000001")}
	newFinding := &model.NewFinding{Category: policy.Assessable()[0], Severity: model.SeverityValues()[0]}
	inputs := []struct {
		name      string
		violation *model.ViolationBasis
		allowedBy func(model.TerminationViolationRequirement) bool
	}{
		{"沒有依據", nil, func(r model.TerminationViolationRequirement) bool { return r != model.ViolationRequired }},
		{"依據只有新認定", &model.ViolationBasis{Finding: newFinding}, func(r model.TerminationViolationRequirement) bool { return r == model.ViolationOptional }},
		{"依據引用既有認定", &model.ViolationBasis{CitedFindingIDs: cited}, func(r model.TerminationViolationRequirement) bool { return r != model.ViolationForbidden }},
	}
	for _, g := range model.TerminationGroundValues() {
		if got := g.ViolationRequirement(); got != wantRequirement[g] {
			t.Errorf("%s 的依據需求 = %s，期望 %s", g, got, wantRequirement[g])
		}
		for _, in := range inputs {
			basis := &model.TerminationBasis{Ground: g, Violation: in.violation}
			if g == model.GroundLinkedAccount {
				source := uuid.MustParse("0190a000-0000-7000-8000-0000000000aa")
				basis.SourceMeasureID = &source
			}
			err := validateTermination(&model.CreateMeasureInput{Kind: model.MeasureTermination, Termination: basis})
			if want := in.allowedBy(g.ViolationRequirement()); (err == nil) != want {
				t.Errorf("%s ×「%s」：驗證 err=%v，需求 %s 允許=%v", g, in.name, err, g.ViolationRequirement(), want)
			}
		}
	}
}
