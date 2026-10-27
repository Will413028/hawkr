// Package sellerreason 是停權措施在舊契約的對照（CodeMeta SellerSuspensionReason 的 Value 與 Label：9601、9602、9603 的
// 管理員停權原因，與違規停權的 SUSPENDED_GOVERNANCE）。舊契約只有兩種停權：管理員停權與違規停權。新模型的措施以種類與依據區分：
// 終止合作（不論依據是認定或契約條款，舊契約沒有這個概念）與依據風險類型（RISK）的停權對應舊的管理員停權（原因碼 9603，標籤是終止事由或風險類型的名稱），
// 只依據認定（VIOLATION）的停權對應舊的違規停權；並存時以管理員停權優先（與舊契約管理員優先於違規相同），標籤優先用終止事由。舊契約有三個出口用到它：gatewayproxy 的 X-Seller-Suspended、
// legacy SellerService 的 lifecycle_status 與 suspension_reason_code（sellerserver），以及 seller.suspended 事件的 reasonCode、
// reasonLabel（sellerrelay），都經 Classify 與 Of。CodeMeta 凍結，隨舊契約退場。
package sellerreason

import (
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// legacyText 是舊契約通知的名詞來源：舊契約固定 zh-TW、預設受眾（共用文案目錄，與 api 契約的名詞同源），不隨請求語系變。
var legacyText = uitext.For(uitext.ZhTW)

// Class 是帳號停權在舊契約的種類。
type Class int

const (
	// None 沒有生效中的停權措施。
	None Class = iota
	// Admin 有依據風險類型的停權，對應舊契約的管理員停權。
	Admin
	// Governance 只有依據認定的停權，對應舊契約的違規停權。
	Governance
)

// Classify 由帳號生效中的停權措施推導舊契約的停權種類：有終止合作或 RISK 依據的是管理員停權（優先），只有 VIOLATION 依據的停權是違規停權。
// 不認得的種類或依據、沒有推導過的停權狀態（nil）都回錯誤，不當成有效值傳下去（fail-closed）。
func Classify(sanctions *seller.Sanctions) (Class, error) {
	if sanctions == nil {
		return None, errNotDerived
	}
	class := None
	for _, m := range sanctions.Measures {
		switch {
		case m.Kind == seller.MeasureTermination:
			return Admin, nil
		case m.Kind != seller.MeasureSuspension:
			return None, fmt.Errorf("sellerreason：不認得的措施種類 %q", m.Kind)
		}
		switch m.Basis {
		case seller.BasisRisk:
			return Admin, nil
		case seller.BasisViolation:
			class = Governance
		case seller.BasisContract:
			return None, fmt.Errorf("sellerreason：停權措施 %s 的依據不可能是契約條款", m.MeasureID)
		default:
			return None, fmt.Errorf("sellerreason：不認得的措施依據 %q", m.Basis)
		}
	}
	return class, nil
}

var errNotDerived = errors.New("sellerreason：店家沒有推導過停權狀態")

// adminCode 是舊契約管理員停權的「其他」原因碼：新模型的風險類型在舊契約沒有對應的原因，一律用 9603，標籤是風險類型的名稱。
const adminCode = "9603"

// Of 是帳號生效中的停權措施在舊契約的原因碼與中文標籤：有終止合作時是 9603 與最新那一個終止合作的事由名稱（優先於 RISK）；沒有終止合作而有
// RISK 依據的措施時是 9603 與最新那一個 RISK 措施的風險類型名稱；沒有（未停權、只有認定的停權）回兩個空字串、沒有錯誤：舊系統的原因碼只有管理員
// 停權才有。終止合作卻沒有事由、事由不認得，RISK 措施卻沒有風險類型、風險類型不認得，或沒有推導過停權狀態（nil），回錯誤，不當成有效值傳下去。
// 措施是新到舊（stint.Effective 的順序）。
func Of(sanctions *seller.Sanctions) (code, label string, err error) {
	if sanctions == nil {
		return "", "", errNotDerived
	}
	for _, m := range sanctions.Measures {
		if m.Kind != seller.MeasureTermination {
			continue
		}
		if m.TerminationGround == nil {
			return "", "", fmt.Errorf("sellerreason：終止合作 %s 沒有事由", m.MeasureID)
		}
		label, ok := legacyText.SellerTerminationGround(string(*m.TerminationGround))
		if !ok {
			return "", "", fmt.Errorf("sellerreason：不認得的終止事由 %q", *m.TerminationGround)
		}
		return adminCode, label, nil
	}
	for _, m := range sanctions.Measures {
		if m.Basis != seller.BasisRisk {
			continue
		}
		if m.RiskGround == nil {
			return "", "", fmt.Errorf("sellerreason：風險措施 %s 沒有風險類型", m.MeasureID)
		}
		label, ok := legacyText.SellerRiskGround(string(*m.RiskGround))
		if !ok {
			return "", "", fmt.Errorf("sellerreason：不認得的風險類型 %q", *m.RiskGround)
		}
		return adminCode, label, nil
	}
	return "", "", nil
}
