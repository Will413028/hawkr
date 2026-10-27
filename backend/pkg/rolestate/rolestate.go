// Package rolestate is the single source of truth for OtherRoleState: the
// three-way judgment a role-owning service (seller-service, buyer-service)
// sends to account-service so it knows whether an account's *other* role is
// genuinely gone before performing an irreversible cascade delete (wiping
// auth_providers). Before this package existed, the type, its constants, and
// the three-way mapping were hand-duplicated across three independent go
// modules — the exact kind of drift that caused issue #1544 (accounts.
// account_type bitmask drifting from the real role tables) in the first
// place. Inverting Active/Absent in any one copy would make account-service
// cascade-delete accounts whose other role is still alive; this package
// makes that impossible to do inconsistently.
//
// This package deliberately does NOT import the generated proto package —
// usecase-layer callers (e.g. account-service's RemoveRoleUseCase,
// seller-service's DeleteSellerAccountUseCase) depend on rolestate for the
// type and Resolve, and must stay free of a pb dependency (see each
// service's usecase layer convention). The pb <-> Go conversions live in the
// rolestatepb sub-package, imported only by the grpc/client layers that
// already depend on pb.
package rolestate

// OtherRoleState is a caller's judgment on whether an account's other role
// (the role not currently being removed) is still active.
type OtherRoleState int

const (
	// OtherRoleUnknown means the lookup wasn't done or failed. This MUST stay
	// the zero value: a caller who forgets to set it (an unset proto field, a
	// zero-valued struct in a test, ...) fails closed instead of silently
	// triggering a cascade delete. account-service never cascades on
	// OtherRoleUnknown — an under-delete is recoverable, an over-delete
	// (wiping auth_providers) is not.
	OtherRoleUnknown OtherRoleState = iota
	// OtherRoleActive means the other role is confirmed still active.
	OtherRoleActive
	// OtherRoleAbsent means the other role is confirmed absent.
	OtherRoleAbsent
)

// Resolve maps a "does the other role's lookup show it active" boolean (plus
// the error from that lookup) to the three-way OtherRoleState. A query error
// always resolves to OtherRoleUnknown (fail-closed) — callers that want
// visibility into the failure should log err themselves alongside this
// call; this function only owns the three-way mapping, shared by every call
// site that determines "should account-service cascade" across
// seller-service and buyer-service (the primary synchronous delete path and
// the compensating retry path in each).
func Resolve(hasActive bool, err error) OtherRoleState {
	if err != nil {
		return OtherRoleUnknown
	}
	if hasActive {
		return OtherRoleActive
	}
	return OtherRoleAbsent
}
