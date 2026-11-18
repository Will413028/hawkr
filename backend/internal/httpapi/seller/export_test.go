package seller

import (
	"testing"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// DeclaredCapability 讓外部的測試套件讀宣告表：方法宣告需要的能力（豁免的方法回 false）。
func DeclaredCapability(procedure string) (model.Capability, bool) {
	d, ok := declarations[procedure]
	return d.capability, ok && d.capability != ""
}

// CapabilityStatusOf 依能力找出 GetMyStore 回報的那一項（單元與 integration 測試共用）；列表的位置不等於列舉的數字
// （刪除的數字保留不重用），所以不能用索引。
func CapabilityStatusOf(t *testing.T, caps []*pb.CapabilityStatus, c pb.SellerCapability) *pb.CapabilityStatus {
	t.Helper()
	for _, st := range caps {
		if st.GetCapability() == c {
			return st
		}
	}
	t.Fatalf("GetMyStore 沒有回報能力 %v：%v", c, caps)
	return nil
}

// NewTestGovernance 建立測試用的 Governance：Images 是零值，ContentSnapshots 回報每則內容都是 12 號店寫的現存內容；override 改寫個別相依。
func NewTestGovernance(t *testing.T, m GovernanceModule, override ...func(*GovernanceDeps)) *Governance {
	t.Helper()
	d := GovernanceDeps{Module: m, ContentSnapshots: ownsEverything{storeID: 12}}
	for _, o := range override {
		o(&d)
	}
	g, err := NewGovernance(d)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
