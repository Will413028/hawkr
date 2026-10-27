package delegation

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

func TestStoreOperable(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		deleted  bool
		wantOK   bool
		wantCode int32
	}{
		{name: "deleted 先判，優先於 status", status: storeStatusClosed, deleted: true, wantCode: bizcode.DelegationStoreNotDelegable},
		{name: "deleted 的 active 列（理論上不存在）同樣不可代行", status: storeStatusActive, deleted: true, wantCode: bizcode.DelegationStoreNotDelegable},
		{name: "active 且未刪除", status: storeStatusActive, wantOK: true},
		{name: "suspended", status: storeStatusSuspended, wantCode: bizcode.DelegationStoreNotOperable},
		{name: "suspended_governance", status: storeStatusSuspendedGovernance, wantCode: bizcode.DelegationStoreNotOperable},
		{name: "closed", status: storeStatusClosed, wantCode: bizcode.DelegationStoreNotOperable},
		{name: "空字串走 deny-by-default", status: "", wantCode: bizcode.DelegationStoreNotOperable},
		{name: "日後新增的生命週期值走 deny-by-default", status: "dormant", wantCode: bizcode.DelegationStoreNotOperable},
		{name: "reshape 前的舊值 deleted", status: "deleted", wantCode: bizcode.DelegationStoreNotOperable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, code := StoreOperable(tc.status, tc.deleted)
			if ok != tc.wantOK {
				t.Errorf("ok = %v，預期 %v", ok, tc.wantOK)
			}
			if code != tc.wantCode {
				t.Errorf("deniedCode = %d，預期 %d", code, tc.wantCode)
			}
			// granted 非空 ⇔ deniedCode == 0：兩個回傳值不得同時有意義。
			if ok && code != 0 {
				t.Errorf("放行時 deniedCode 必須為 0，實得 %d", code)
			}
			if !ok && code == 0 {
				t.Error("拒絕時 deniedCode 不得為 0（呼叫端會拿 0 當成功）")
			}
		})
	}
}

// TestStoreOperableIsWhitelist guards the direction of the predicate: adding a
// lifecycle value to sellers.status must not silently become "allowed".
func TestStoreOperableIsWhitelist(t *testing.T) {
	for _, s := range []string{"ACTIVE", "Active", " active", "active "} {
		if ok, _ := StoreOperable(s, false); ok {
			t.Errorf("%q 不應被視為 active（值域比對大小寫敏感、不 trim）", s)
		}
	}
}
