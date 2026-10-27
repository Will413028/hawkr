package accounttype

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// CodeMeta〈業務類型碼〉的 AccountType 是這裡的投影：gateway 與舊服務以它解讀單體帶去的值。
func TestLegacyAccountTypeMatchesCodeMeta(t *testing.T) {
	md, err := os.ReadFile("../../../docs/代碼管理（CodeMeta）/業務類型碼.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(md), "## AccountType\n")
	if !ok {
		t.Fatal("CodeMeta〈業務類型碼〉沒有 AccountType 那一段")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	codemeta := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([A-Z_]+) \| [A-Z_]+ \| (\d+) \|`).FindAllStringSubmatch(section, -1) {
		codemeta[m[1]], _ = strconv.Atoi(m[2])
	}
	want := map[string]int{
		"BUYER":            OfLiveAccount(nil),
		"SELLER":           OfLoginRole(identity.Principal{Kind: identity.PrincipalAccount, Roles: []identity.Role{identity.RoleSeller}}, "SELLER"),
		"SELLER_AND_BUYER": OfLiveAccount([]identity.Role{identity.RoleSeller}),
		"ADMIN":            Admin,
	}
	if len(codemeta) != len(want) {
		t.Fatalf("CodeMeta 的 AccountType = %v，預期剛好 %d 個值", codemeta, len(want))
	}
	for name, v := range want {
		if got, ok := codemeta[name]; !ok || got != v {
			t.Errorf("%s：這裡是 %d，CodeMeta 是 %d", name, v, got)
		}
	}
}

// 登入身分只是提示加上帳號實際有的角色：只有帳號有賣家角色而且提示是 SELLER 才是賣家，每個帳號都是買家；管理員沒有。
func TestLoginRoleFollowsTheHintWithinTheAccountsRoles(t *testing.T) {
	seller := identity.Principal{Kind: identity.PrincipalAccount, ID: 7, Roles: []identity.Role{identity.RoleSeller}}
	buyer := identity.Principal{Kind: identity.PrincipalAccount, ID: 7}
	for _, c := range []struct {
		name string
		p    identity.Principal
		hint string
		want int
	}{
		{"賣家，提示賣家", seller, "SELLER", Seller},
		{"賣家，沒有提示是買家", seller, "", Buyer},
		{"賣家，提示看不懂是買家", seller, "2", Buyer},
		{"賣家，提示買家", seller, "BUYER", Buyer},
		{"不是賣家，提示賣家仍是買家", buyer, "SELLER", Buyer},
		{"不是賣家，沒有提示", buyer, "", Buyer},
		{"管理員沒有", identity.Principal{Kind: identity.PrincipalAdmin, ID: 7}, "SELLER", 0},
	} {
		if got := OfLoginRole(c.p, c.hint); got != c.want {
			t.Errorf("%s：= %d，預期 %d", c.name, got, c.want)
		}
	}
}
