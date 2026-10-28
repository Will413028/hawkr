package rolestate

import "testing"

// TestZeroValueIsUnknown pins OtherRoleUnknown as the zero value. This is the
// property that makes "caller forgot to set the field" fail-closed instead of
// silently triggering a cascade delete — a reordered const block would break
// it silently (the compiler gives no warning about which value is iota 0).
func TestZeroValueIsUnknown(t *testing.T) {
	var zero OtherRoleState
	if zero != OtherRoleUnknown {
		t.Fatalf("zero value = %v, want OtherRoleUnknown (%v)", zero, OtherRoleUnknown)
	}
	if OtherRoleUnknown != 0 {
		t.Fatalf("OtherRoleUnknown = %d, want 0", OtherRoleUnknown)
	}
}

// TestResolve pins the three-way mapping used by every call site that
// decides whether the identity module should cascade-delete an account. Getting
// this backwards (ACTIVE <-> ABSENT) would make the identity module wipe
// auth_providers for an account whose other role is still alive.
func TestResolve(t *testing.T) {
	sentinelErr := errTest("boom")

	tests := []struct {
		name      string
		hasActive bool
		err       error
		want      OtherRoleState
	}{
		{"error always wins → Unknown even if hasActive=true", true, sentinelErr, OtherRoleUnknown},
		{"error always wins → Unknown even if hasActive=false", false, sentinelErr, OtherRoleUnknown},
		{"no error, hasActive=true → Active", true, nil, OtherRoleActive},
		{"no error, hasActive=false → Absent", false, nil, OtherRoleAbsent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.hasActive, tt.err); got != tt.want {
				t.Errorf("Resolve(%v, %v) = %v, want %v", tt.hasActive, tt.err, got, tt.want)
			}
		})
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
