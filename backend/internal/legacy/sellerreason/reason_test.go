package sellerreason

import (
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// 核准文案（step10e-L0-copy.md）逐字的常值：期望值不經過被測的目錄查詢，改文案就必須同時改這裡。
var terminationGroundText = map[seller.TerminationGround]string{
	seller.GroundNotice:             "預告終止（第 21.2 條）",
	seller.GroundMaterialBreach:     "重大違約未改善",
	seller.GroundCeasedBusiness:     "停業",
	seller.GroundDissolution:        "解散",
	seller.GroundBankruptcy:         "破產",
	seller.GroundLicenseRevoked:     "必要許可遭撤銷",
	seller.GroundFoodSafetyIncident: "重大食安事件",
	seller.GroundFraud:              "詐欺事件",
	seller.GroundPlatformSecurity:   "危害平台安全",
	seller.GroundOtherSerious:       "其他嚴重影響履約的情形",
	seller.GroundLinkedAccount:      "關聯帳號",
}

var riskGroundText = map[seller.RiskGround]string{
	seller.RiskTransaction:   "交易風險",
	seller.RiskFoodSafety:    "食品安全",
	seller.RiskFraud:         "詐欺",
	seller.RiskPersonalData:  "個人資料",
	seller.RiskInfringement:  "侵權",
	seller.RiskSecurity:      "資安",
	seller.RiskFulfillment:   "履約",
	seller.RiskExternalOrder: "主管機關或司法機關的要求",
}

func risk(ground seller.RiskGround) seller.EffectiveMeasure {
	return seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisRisk, RiskGround: &ground}
}

func violation() seller.EffectiveMeasure {
	category := policy.Fraud
	return seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisViolation, LeadCategory: &category}
}

func TestOfMapsEveryRiskGroundToTheOtherCodeWithItsName(t *testing.T) {
	for _, ground := range seller.RiskGroundValues() {
		code, label, err := Of(seller.SanctionedBy(risk(ground)))
		if err != nil || code != "9603" || label != riskGroundText[ground] || label == "" {
			t.Errorf("%s: %q %q %v，預期 %q", ground, code, label, err, riskGroundText[ground])
		}
	}
}

func TestOfUsesTheNewestRiskMeasure(t *testing.T) {
	code, label, err := Of(seller.SanctionedBy(violation(), risk(seller.RiskFraud), risk(seller.RiskSecurity)))
	if err != nil || code != "9603" || label != "詐欺" {
		t.Errorf("%q %q %v", code, label, err)
	}
}

func TestOfIsEmptyWithoutARiskMeasure(t *testing.T) {
	for name, effective := range map[string]*seller.Sanctions{
		"未停權":     seller.NoSanctions(),
		"只有認定的停權": seller.SanctionedBy(violation()),
	} {
		if code, label, err := Of(effective); err != nil || code != "" || label != "" {
			t.Errorf("%s: %q %q %v", name, code, label, err)
		}
	}
}

func TestOfRejectsUnknownOrMissingRiskGround(t *testing.T) {
	unknown := seller.RiskGround("UNKNOWN")
	for name, effective := range map[string]*seller.Sanctions{
		"沒有風險類型":   seller.SanctionedBy(seller.EffectiveMeasure{Kind: seller.MeasureSuspension, MeasureID: uuid.NewV7(), Basis: seller.BasisRisk}),
		"不認得的風險類型": seller.SanctionedBy(risk(unknown)),
	} {
		if code, label, err := Of(effective); err == nil {
			t.Errorf("%s 不得當成有效值：%q %q", name, code, label)
		}
	}
}

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		name      string
		effective *seller.Sanctions
		want      Class
	}{
		{"未停權", seller.NoSanctions(), None},
		{"只有認定", seller.SanctionedBy(violation(), violation()), Governance},
		{"只有風險", seller.SanctionedBy(risk(seller.RiskFraud)), Admin},
		{"並存：風險優先", seller.SanctionedBy(violation(), risk(seller.RiskFraud)), Admin},
	} {
		if got, err := Classify(tc.effective); err != nil || got != tc.want {
			t.Errorf("%s: %v %v，預期 %v", tc.name, got, err, tc.want)
		}
	}
	if got, err := Classify(seller.SanctionedBy(seller.EffectiveMeasure{Kind: seller.MeasureSuspension, Basis: "WHATEVER"})); err == nil {
		t.Errorf("不認得的依據不得當成有效值：%v", got)
	}
}

// 沒有推導過的停權狀態（nil）不能當成未停權：兩個出口都回錯誤（fail-closed）。
func TestUnderivedSanctionsAreRejected(t *testing.T) {
	if got, err := Classify(nil); err == nil {
		t.Errorf("Classify(nil) 不得當成有效值：%v", got)
	}
	if code, label, err := Of(nil); err == nil {
		t.Errorf("Of(nil) 不得當成有效值：%q %q", code, label)
	}
}

func termination(basis seller.MeasureBasis, ground seller.TerminationGround) seller.EffectiveMeasure {
	m := seller.EffectiveMeasure{Kind: seller.MeasureTermination, MeasureID: uuid.NewV7(), Basis: basis, TerminationGround: &ground}
	if basis == seller.BasisViolation {
		category := policy.Fraud
		m.LeadCategory = &category
	}
	return m
}

// 終止合作在舊契約是管理員停權（9603）：不論依據是認定或契約條款（依據認定的終止不能被分成 SUSPENDED_GOVERNANCE），標籤優先用終止事由。
// mutation：Classify 拿掉 kind 判斷，依據認定的終止變成違規停權、契約依據的終止回錯誤。
func TestTerminationIsTheLegacyAdminSuspensionAndItsGroundIsTheLabel(t *testing.T) {
	for _, ground := range seller.TerminationGroundValues() {
		basis := seller.BasisContract
		if ground == seller.GroundMaterialBreach {
			basis = seller.BasisViolation
		}
		sanctions := seller.SanctionedBy(termination(basis, ground))
		if class, err := Classify(sanctions); err != nil || class != Admin {
			t.Errorf("%s：Classify=%v %v，預期 Admin", ground, class, err)
		}
		code, label, err := Of(sanctions)
		if err != nil || code != "9603" || label != terminationGroundText[ground] || label == "" {
			t.Errorf("%s：Of=%q %q %v", ground, code, label, err)
		}
	}
	// 終止事由優先於同時存在的風險措施的標籤。
	code, label, err := Of(seller.SanctionedBy(risk(seller.RiskFraud), termination(seller.BasisContract, seller.GroundBankruptcy)))
	if err != nil || code != "9603" || label != "破產" {
		t.Errorf("標籤優先用終止事由：%q %q %v", code, label, err)
	}
	// 依據認定的終止不是違規停權。
	if class, err := Classify(seller.SanctionedBy(termination(seller.BasisViolation, seller.GroundMaterialBreach), violation())); err != nil || class != Admin {
		t.Errorf("依據認定的終止要是 Admin：%v %v", class, err)
	}
}

// 空的種類、沒有事由的終止、不認得的事由都不得當成有效值傳下去。
func TestUnknownKindAndMissingGroundAreRejected(t *testing.T) {
	if got, err := Classify(seller.SanctionedBy(seller.EffectiveMeasure{MeasureID: uuid.NewV7(), Basis: seller.BasisRisk})); err == nil {
		t.Errorf("空的種類不得當成有效值：%v", got)
	}
	noGround := seller.EffectiveMeasure{Kind: seller.MeasureTermination, MeasureID: uuid.NewV7(), Basis: seller.BasisContract}
	if code, label, err := Of(seller.SanctionedBy(noGround)); err == nil {
		t.Errorf("沒有事由的終止不得當成有效值：%q %q", code, label)
	}
	unknown := seller.TerminationGround("UNKNOWN")
	if code, label, err := Of(seller.SanctionedBy(termination(seller.BasisContract, unknown))); err == nil {
		t.Errorf("不認得的事由不得當成有效值：%q %q", code, label)
	}
}
