package adminperm

import (
	"regexp"
	"slices"
	"testing"
)

var nameFormat = regexp.MustCompile(`^[a-z]+\.[a-z0-9]+(_[a-z0-9]+)*\.[a-z]+(_[a-z]+)*$`)

func TestCatalogNamesFollowModuleResourceVerb(t *testing.T) {
	seen := map[Permission]bool{}
	for _, p := range Catalog() {
		if !nameFormat.MatchString(string(p)) {
			t.Errorf("%q 不是 <模組>.<資源>.<動詞>", p)
		}
		if seen[p] {
			t.Errorf("%q 在目錄裡重複", p)
		}
		seen[p] = true
	}
}

func TestOwnerAllowsEveryPermissionInTheCatalog(t *testing.T) {
	for _, p := range Catalog() {
		if !Allows([]Role{RoleOwner}, p) {
			t.Errorf("OWNER 不允許 %s", p)
		}
	}
}

func TestNoRolesAllowNothing(t *testing.T) {
	for _, roles := range [][]Role{nil, {}} {
		for _, p := range Catalog() {
			if Allows(roles, p) {
				t.Errorf("沒有角色卻允許 %s", p)
			}
		}
	}
}

func TestPermissionOutsideTheCatalogIsNeverAllowed(t *testing.T) {
	for _, p := range []Permission{"", "seller.no_such.get"} {
		if Allows([]Role{RoleOwner}, p) {
			t.Errorf("OWNER 允許了不在目錄的 %q", p)
		}
	}
}

func TestUnknownRoleAllowsNothing(t *testing.T) {
	if Allows([]Role{"VIEWER"}, SellerStoresList) {
		t.Fatal("未知的角色不得被允許")
	}
}

func TestRolesListsOwner(t *testing.T) {
	if !slices.Equal(Roles(), []Role{RoleOwner}) {
		t.Fatalf("Roles() = %v", Roles())
	}
}
