package authz

import (
	"os"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

// The golden file is one of the four places the closed enumeration lives (Go
// constants / golden / two delegation-module CHECK constraints). CI compares all four
// with scripts/check-delegation-capability-closure.py; this test keeps the two
// in-repo Go-side copies honest without needing the workflow to run.
func TestAdminCapabilitiesMatchGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/admin_capabilities.golden")
	if err != nil {
		t.Fatalf("讀不到 golden 檔：%v", err)
	}
	var golden []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			golden = append(golden, line)
		}
	}
	if len(golden) != 9 {
		t.Fatalf("golden 檔有 %d 行，capability 是封閉九值", len(golden))
	}
	if len(AllAdminCapabilities) != len(golden) {
		t.Fatalf("Go 常數 %d 個、golden %d 行", len(AllAdminCapabilities), len(golden))
	}
	for i, want := range golden {
		if string(AllAdminCapabilities[i]) != want {
			t.Errorf("第 %d 項：Go 常數 %q、golden %q（順序也必須一致）", i+1, AllAdminCapabilities[i], want)
		}
	}
}

func TestIsKnownAdminRejectsNonMembers(t *testing.T) {
	for _, c := range AllAdminCapabilities {
		if !IsKnownAdmin(c) {
			t.Errorf("%q 應為已知 capability", c)
		}
	}
	// 空字串必須是未知，否則「漏宣告 capability」會靜默掛上一條無防護的代行路由。
	// 履約禁區機器名與賣家 token 同樣不得被誤認。
	for _, c := range []AdminCapability{"", "fulfillment", "shipment.write", "orders.write", "CAP_ORDER_FULFILL", "products.bulk", "audit.read"} {
		if IsKnownAdmin(c) {
			t.Errorf("%q 不在封閉九值內，不得被認為已知", c)
		}
	}
}

func TestRegisterAdminPanicsOnUnknownToken(t *testing.T) {
	for _, c := range []AdminCapability{"", "products.bulk"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("RegisterAdmin(%q) 必須 panic", c)
				}
			}()
			RegisterAdmin("GET", "/api/v1/admin/__test__", c)
		}()
	}
}

func TestGrantedAdminCapabilities(t *testing.T) {
	base := AdminDelegationContext{
		SessionStatus:      DelegationSessionActive,
		Capabilities:       []AdminCapability{AdminCapProductsRead, AdminCapProductsWrite},
		TargetStoreID:      2685914682,
		RequestedStoreID:   2685914682,
		RequiredCapability: AdminCapProductsWrite,
	}

	cases := []struct {
		name     string
		mutate   func(*AdminDelegationContext)
		wantCode int32
	}{
		{name: "active + 同店 + 持有 capability", wantCode: 0},
		{name: "expired 短路在 status", mutate: func(c *AdminDelegationContext) {
			c.SessionStatus = DelegationSessionExpired
			c.RequestedStoreID = 1 // 後兩項也違規，仍必須回 status 的碼
			c.RequiredCapability = AdminCapOrdersRead
		}, wantCode: bizcode.DelegationSessionExpired},
		{name: "revoked", mutate: func(c *AdminDelegationContext) { c.SessionStatus = DelegationSessionRevoked }, wantCode: bizcode.DelegationSessionRevoked},
		{name: "未知 status 走 deny-by-default，不得留 0", mutate: func(c *AdminDelegationContext) { c.SessionStatus = "ended" }, wantCode: bizcode.DelegationSessionRevoked},
		{name: "空 status", mutate: func(c *AdminDelegationContext) { c.SessionStatus = "" }, wantCode: bizcode.DelegationSessionRevoked},
		{name: "target 不符短路在 target", mutate: func(c *AdminDelegationContext) {
			c.RequestedStoreID = 2632049831
			c.RequiredCapability = AdminCapOrdersRead // capability 也不足，仍必須回 target 的碼
		}, wantCode: bizcode.DelegationTargetMismatch},
		{name: "snapshot 無 target", mutate: func(c *AdminDelegationContext) { c.TargetStoreID = 0 }, wantCode: bizcode.DelegationTargetMismatch},
		{name: "URL 無 store", mutate: func(c *AdminDelegationContext) { c.RequestedStoreID = 0 }, wantCode: bizcode.DelegationTargetMismatch},
		{name: "capability 不足", mutate: func(c *AdminDelegationContext) { c.RequiredCapability = AdminCapOrdersRead }, wantCode: bizcode.DelegationCapabilityDenied},
		{name: "route 宣告未知 token", mutate: func(c *AdminDelegationContext) { c.RequiredCapability = "products.bulk" }, wantCode: bizcode.DelegationCapabilityDenied},
		{name: "snapshot 帶未知 token 不得被當成持有", mutate: func(c *AdminDelegationContext) {
			c.Capabilities = []AdminCapability{"fulfillment"}
			c.RequiredCapability = "fulfillment"
		}, wantCode: bizcode.DelegationCapabilityDenied},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := base
			ctx.Capabilities = append([]AdminCapability(nil), base.Capabilities...)
			if tc.mutate != nil {
				tc.mutate(&ctx)
			}
			granted, code := GrantedAdminCapabilities(ctx)
			if code != tc.wantCode {
				t.Fatalf("deniedCode = %d，預期 %d", code, tc.wantCode)
			}
			// granted 非空 ⇔ deniedCode == 0。
			if (len(granted) > 0) != (code == 0) {
				t.Fatalf("granted=%v 與 deniedCode=%d 不一致", granted, code)
			}
		})
	}
}

func TestGrantedAdminCapabilitiesDropsUnknownFromSnapshot(t *testing.T) {
	granted, code := GrantedAdminCapabilities(AdminDelegationContext{
		SessionStatus:      DelegationSessionActive,
		Capabilities:       []AdminCapability{AdminCapPostsRead, "audit.read", AdminCapPostsWrite},
		TargetStoreID:      7,
		RequestedStoreID:   7,
		RequiredCapability: AdminCapPostsRead,
	})
	if code != 0 {
		t.Fatalf("deniedCode = %d，預期 0", code)
	}
	if len(granted) != 2 {
		t.Fatalf("granted = %v，未知 token 應被丟棄後剩 2 個", granted)
	}
}
