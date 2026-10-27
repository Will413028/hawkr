package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// TestDealTemperatureConstantsMatchCodeMeta 把成交溫度計 v1 的九個字面值釘到
// CodeMeta 生成常數上。
//
// domain 層（deal_temperature.go）刻意不 import codemeta，以維持
// ComputeDealTemperatureCoverage 的純函數性質；代價是 domain 的字面值常數與
// CodeMeta 登記值可能各自漂移，而且不會有編譯錯誤提醒——兩邊都是普通字串。
// 這支測試把那個代價收斂成一個會紅的斷言，放在 usecase 層是因為這層本來就
// 同時 import 兩者（domain 不能、也不該 import codemeta）。
//
// 前八組是 domain 的 status／reason／band 常數對 CodeMeta 的 .Value；
// 第九組「MAPPING_MISSING」是 price_preview.go 沿用的裸字面值——domain 沒有
// 對應常數，因為 domain.ComputeDealTemperatureCoverage 從不產生這個 reason，
// 它只在 BUYER_ENTERED_NO_REFERENCE 分支由 usecase 直接指派（見
// price_preview.go dealTemperatureFor 與其呼叫端）。這裡直接釘住該字面值本身：
// 若未來 CodeMeta 重新產生時這個 reason 的 Value 被改名，這支測試會先紅，而
// 不是等到前端或其他 repo 的拷貝悄悄對不上。
func TestDealTemperatureConstantsMatchCodeMeta(t *testing.T) {
	cases := []struct {
		name        string
		domainValue string
		codeMeta    string
	}{
		{"status/AVAILABLE", domain.DealTemperatureStatusAvailable, codemeta.ProcurementDealTemperatureStatusAvailable.Value},
		{"status/UNKNOWN", domain.DealTemperatureStatusUnknown, codemeta.ProcurementDealTemperatureStatusUnknown.Value},
		{"reason/BUYER_RANGE_NOT_SET", domain.DealTemperatureReasonBuyerRangeNotSet, codemeta.ProcurementDealTemperatureReasonBuyerRangeNotSet.Value},
		{"reason/RANGE_OUTSIDE_REFERENCE", domain.DealTemperatureReasonRangeOutsideReference, codemeta.ProcurementDealTemperatureReasonRangeOutsideReference.Value},
		{"reason/QUANTILES_NOT_MONOTONIC", domain.DealTemperatureReasonQuantilesNotMonotonic, codemeta.ProcurementDealTemperatureReasonQuantilesNotMonotonic.Value},
		{"band/COLD", domain.DealTemperatureBandCold, codemeta.ProcurementDealTemperatureBandCold.Value},
		{"band/WARM", domain.DealTemperatureBandWarm, codemeta.ProcurementDealTemperatureBandWarm.Value},
		{"band/HOT", domain.DealTemperatureBandHot, codemeta.ProcurementDealTemperatureBandHot.Value},
		// 沒有 domain.DealTemperatureReasonMappingMissing 可比——見上方函式註解。
		{"reason/MAPPING_MISSING (usecase 裸字面值)", "MAPPING_MISSING", codemeta.ProcurementDealTemperatureReasonMappingMissing.Value},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.domainValue != tc.codeMeta {
				t.Errorf("字面值 %q 與 CodeMeta %q 不一致", tc.domainValue, tc.codeMeta)
			}
		})
	}
}
