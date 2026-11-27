package middleware

import (
	"testing"
)

func TestPickActiveRole(t *testing.T) {
	cases := []struct {
		name        string
		accountType int32
		resolved    int32
		want        int32
	}{
		{"dual resolved buyer", AccountTypeSellerAndBuyer, 1, 1},
		{"dual resolved seller", AccountTypeSellerAndBuyer, 2, 2},
		{"dual unresolved", AccountTypeSellerAndBuyer, 0, 0},
		{"buyer-only derives 1", AccountTypeBuyer, 0, 1},
		{"seller-only derives 2", AccountTypeSeller, 0, 2},
		{"buyer-only ignores seller", AccountTypeBuyer, 2, 1},
		{"seller-only ignores buyer", AccountTypeSeller, 1, 2},
		{"dual out-of-range", AccountTypeSellerAndBuyer, 7, 0},
		{"dual bitmask-3 is not a role", AccountTypeSellerAndBuyer, 3, 0},
		{"dual negative", AccountTypeSellerAndBuyer, -3, 0},
		{"buyer-only ignores poisoned", AccountTypeBuyer, 7, 1},
		{"seller-only ignores poisoned", AccountTypeSeller, 7, 2},
		{"admin login role 4", AccountTypeAdmin, 4, 0},
		{"admin claiming seller", AccountTypeAdmin, 2, 0},
		{"no roles", 0, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickActiveRole(tc.accountType, tc.resolved); got != tc.want {
				t.Fatalf("pickActiveRole(%d, %d) = %d, want %d", tc.accountType, tc.resolved, got, tc.want)
			}
		})
	}
}

func TestRoleInSet(t *testing.T) {
	if !RoleInSet(1, AccountTypeSellerAndBuyer) || !RoleInSet(2, AccountTypeSellerAndBuyer) {
		t.Fatal("both roles must be in the bitmask-3 set")
	}
	if RoleInSet(2, AccountTypeBuyer) {
		t.Fatal("seller role must NOT be in a buyer-only account")
	}
	if RoleInSet(1, AccountTypeSeller) {
		t.Fatal("buyer role must NOT be in a seller-only account")
	}
	if RoleInSet(3, AccountTypeSellerAndBuyer) {
		t.Fatal("only canonical roles 1/2 are valid active roles")
	}
}

// sanitizeActiveRole is the one rule that decides whether a resolved active role
// (單體的 X-Login-Role) may be used:
// only a role the account actually holds survives; everything else collapses to
// 0 ("no active role"), never to a guess.
func TestSanitizeActiveRole(t *testing.T) {
	for _, tc := range []struct {
		role        int32
		accountType int32
		want        int32
	}{
		{role: 1, accountType: AccountTypeSellerAndBuyer, want: 1},
		{role: 2, accountType: AccountTypeSellerAndBuyer, want: 2},
		{role: 1, accountType: AccountTypeBuyer, want: 1},
		{role: 2, accountType: AccountTypeSeller, want: 2},
		{role: 0, accountType: AccountTypeSellerAndBuyer, want: 0},
		{role: 2, accountType: AccountTypeBuyer, want: 0},
		{role: 1, accountType: AccountTypeSeller, want: 0},
		{role: 3, accountType: AccountTypeSellerAndBuyer, want: 0},
		{role: 7, accountType: AccountTypeSellerAndBuyer, want: 0},
		{role: -3, accountType: AccountTypeSellerAndBuyer, want: 0},
		{role: 1, accountType: 4, want: 0},
		{role: 2, accountType: 4, want: 0},
	} {
		if got := sanitizeActiveRole(tc.role, tc.accountType); got != tc.want {
			t.Errorf("sanitizeActiveRole(%d, accountType=%d) = %d, want %d",
				tc.role, tc.accountType, got, tc.want)
		}
	}
}
