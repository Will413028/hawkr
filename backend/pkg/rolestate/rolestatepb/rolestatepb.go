// Package rolestatepb converts between rolestate.OtherRoleState and the
// generated pb.OtherRoleState enum (pkg/proto/account).
//
// Split out from pkg/rolestate itself so that usecase-layer callers can
// import rolestate for the type and Resolve without dragging the generated
// proto package into their import graph. Only the grpc/client layers (which
// already depend on pb to speak the account-service RPC) import this
// sub-package.
package rolestatepb

import (
	pb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
)

// ToProto converts rolestate.OtherRoleState to the pb enum account-service's
// RemoveRole expects. Defaults to UNKNOWN (fail-closed) for any value that
// isn't explicitly Active or Absent.
func ToProto(s rolestate.OtherRoleState) pb.OtherRoleState {
	switch s {
	case rolestate.OtherRoleActive:
		return pb.OtherRoleState_OTHER_ROLE_STATE_ACTIVE
	case rolestate.OtherRoleAbsent:
		return pb.OtherRoleState_OTHER_ROLE_STATE_ABSENT
	default:
		return pb.OtherRoleState_OTHER_ROLE_STATE_UNKNOWN
	}
}

// FromProto converts the pb enum to rolestate.OtherRoleState. Defaults to
// rolestate.OtherRoleUnknown (fail-closed) for any value that isn't
// explicitly ACTIVE or ABSENT.
func FromProto(s pb.OtherRoleState) rolestate.OtherRoleState {
	switch s {
	case pb.OtherRoleState_OTHER_ROLE_STATE_ACTIVE:
		return rolestate.OtherRoleActive
	case pb.OtherRoleState_OTHER_ROLE_STATE_ABSENT:
		return rolestate.OtherRoleAbsent
	default:
		return rolestate.OtherRoleUnknown
	}
}
