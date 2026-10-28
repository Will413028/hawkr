package rolestatepb

import (
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
)

// TestToProto pins the rolestate.OtherRoleState → pb.OtherRoleState mapping.
//
// If this mapping is ever inverted (ACTIVE <-> ABSENT), the identity module will
// cascade-delete accounts whose other role is still alive — an irreversible
// wipe of auth_providers. This is the single copy of that mapping shared by
// the seller and buyer modules.
func TestToProto(t *testing.T) {
	tests := []struct {
		name string
		in   rolestate.OtherRoleState
		want pb.OtherRoleState
	}{
		{"ACTIVE", rolestate.OtherRoleActive, pb.OtherRoleState_OTHER_ROLE_STATE_ACTIVE},
		{"ABSENT", rolestate.OtherRoleAbsent, pb.OtherRoleState_OTHER_ROLE_STATE_ABSENT},
		{"UNKNOWN", rolestate.OtherRoleUnknown, pb.OtherRoleState_OTHER_ROLE_STATE_UNKNOWN},
		{
			// 未定義的值必須落在 UNKNOWN（fail-closed），不能落在 ABSENT。
			"未定義的值 → UNKNOWN",
			rolestate.OtherRoleState(99),
			pb.OtherRoleState_OTHER_ROLE_STATE_UNKNOWN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToProto(tt.in); got != tt.want {
				t.Errorf("ToProto(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestFromProto pins the reverse direction (pb.OtherRoleState →
// rolestate.OtherRoleState) used by the identity module's grpc handler to decode
// the incoming RemoveRoleRequest. Same inversion risk as TestToProto, just
// the other way across the wire.
func TestFromProto(t *testing.T) {
	tests := []struct {
		name string
		in   pb.OtherRoleState
		want rolestate.OtherRoleState
	}{
		{"ACTIVE", pb.OtherRoleState_OTHER_ROLE_STATE_ACTIVE, rolestate.OtherRoleActive},
		{"ABSENT", pb.OtherRoleState_OTHER_ROLE_STATE_ABSENT, rolestate.OtherRoleAbsent},
		{"UNKNOWN", pb.OtherRoleState_OTHER_ROLE_STATE_UNKNOWN, rolestate.OtherRoleUnknown},
		{
			// 未定義的值必須落在 UNKNOWN（fail-closed），不能落在 ABSENT。
			"未定義的值 → Unknown",
			pb.OtherRoleState(99),
			rolestate.OtherRoleUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromProto(tt.in); got != tt.want {
				t.Errorf("FromProto(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestRoundTrip guards against a specific inversion class where ToProto and
// FromProto are each individually "plausible" but disagree with each other
// (e.g. both hardcode ACTIVE=2/ABSENT=1 instead of matching the real pb
// values) — a bug neither table test alone would catch since they don't
// cross-check against each other.
func TestRoundTrip(t *testing.T) {
	for _, s := range []rolestate.OtherRoleState{rolestate.OtherRoleUnknown, rolestate.OtherRoleActive, rolestate.OtherRoleAbsent} {
		if got := FromProto(ToProto(s)); got != s {
			t.Errorf("FromProto(ToProto(%v)) = %v, want %v", s, got, s)
		}
	}
}
