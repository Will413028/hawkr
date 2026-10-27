package main

import "testing"

func TestParseDelegationEligibility(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		enabled bool
		valid   bool
	}{
		{[]string{"enable", "-admin-id", "7", "-operator-admin-id", "9"}, true, true},
		{[]string{"disable", "-admin-id", "7", "-operator-admin-id", "9"}, false, true},
		{[]string{"disable", "-admin-id", "7"}, false, false},
		{[]string{"enable", "-admin-id", "0", "-operator-admin-id", "9"}, false, false},
		{[]string{"enable", "-admin-id", "7", "-operator-admin-id", "9", "extra"}, false, false},
		{[]string{"unknown", "-admin-id", "7", "-operator-admin-id", "9"}, false, false},
	} {
		got, err := parseDelegationEligibility(tc.args)
		if (err == nil) != tc.valid {
			t.Errorf("%q：err = %v，valid = %v", tc.args, err, tc.valid)
		}
		if tc.valid && (got.AdminID != 7 || got.OperatorAdminID != 9 || got.Enabled != tc.enabled) {
			t.Errorf("%q：%+v", tc.args, got)
		}
	}
}
