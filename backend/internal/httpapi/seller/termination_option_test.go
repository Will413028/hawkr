package seller

import (
	"testing"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 依據的需求在模組與 API 是同一組值：model 的字串改名會讓 EnumToProto 靜默回 UNSPECIFIED（client 就隱藏欄位），這裡先擋。
// toTerminationOption 對每個事由都設了非 UNSPECIFIED 的需求。
func TestTerminationViolationRequirementMatchesTheContract(t *testing.T) {
	if got := rpc.EnumMismatches[adminpb.TerminationViolationRequirement](model.TerminationViolationRequirementValues()); len(got) != 0 {
		t.Fatalf("TerminationViolationRequirement：%v", got)
	}
	for _, g := range model.TerminationGroundValues() {
		o := toTerminationOption(model.TerminationEffect{Ground: g})
		if o.GetViolationRequirement() == adminpb.TerminationViolationRequirement_TERMINATION_VIOLATION_REQUIREMENT_UNSPECIFIED {
			t.Errorf("%s 的 violation_requirement 不能是 UNSPECIFIED", g)
		}
	}
}
