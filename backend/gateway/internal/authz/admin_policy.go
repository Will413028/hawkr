package authz

import "github.com/Will413028/hawkr/go-pkg/errors/bizcode"

// Session lifecycle values carried by the operation session snapshot
// (delegated_operation_sessions.status, lowercase, closed at three values).
const (
	DelegationSessionActive  = "active"
	DelegationSessionExpired = "expired"
	DelegationSessionRevoked = "revoked"
)

// AdminDelegationContext is the whole input to "can this admin do this thing to
// this store right now" (56 §8.1.3). The first three fields are the session
// snapshot; the last two are the request being judged.
//
// Both request-side fields are here on purpose. §8.1.3 requires the three verdicts
// (status → target → capability) to short-circuit IN THAT ORDER and each to fill its
// own business code; a function that only saw the snapshot could not do the target
// or capability comparison, so those two judgements would have to be re-implemented
// in the guard — and then there would be two answer points instead of the one the
// section exists to establish.
//
// There is deliberately NO ActorAdminID field. Since A31 the workspace runs on a
// different host and the request carries no second identity to compare the
// credential against; comparing a value derived from the credential with the
// credential itself is an identity, not a check. Holding the credential IS being the
// operator of that session. A field that is stored but never judged would read like
// a defence that is not there.
type AdminDelegationContext struct {
	// SessionStatus is the derived lifecycle value of the session snapshot.
	SessionStatus string
	// Capabilities is the set granted when the session was created ("the ceiling
	// issued back then", §8.1.3 第 7 項), not proof that it is still valid now.
	Capabilities []AdminCapability
	// TargetStoreID is the store the session was opened against (snapshot value —
	// never a resolver result, §8.1.3 第 5 項).
	TargetStoreID int64
	// RequestedStoreID is the :store_id of the URL being served.
	RequestedStoreID int64
	// RequiredCapability is the capability the route declared at mount time.
	RequiredCapability AdminCapability
}

// AdminSessionDenial reports the business code for a session lifecycle value, or 0
// when the session is still usable. Split out of GrantedAdminCapabilities because
// the media upload guard (requireDelegatedSessionOnly) asks only this much — it has
// no capability and no target to judge — and two copies of "which statuses are
// terminal" would eventually disagree.
//
// Unknown values fall to revoked rather than to 0: an unrecognised lifecycle value
// must never leave deniedCode empty, because a caller reading "not granted, no
// reason" has nothing to answer the client with, and deny-by-default says a status
// we do not understand is not a status we may operate under.
//
// hard 到期與閒置逾時共用 9270（`status` 三值封閉，容不下第四值），刻意不為 idle 另立
// 新業務碼。
func AdminSessionDenial(status string) int32 {
	switch status {
	case DelegationSessionActive:
		return 0
	case DelegationSessionExpired:
		return bizcode.DelegationSessionExpired
	default:
		return bizcode.DelegationSessionRevoked
	}
}

// GrantedAdminCapabilities is the single question point for delegated operations,
// the admin-side counterpart of the seller capability set the monolith sends.
//
// Verdict order is fixed at status → target → capability, each short-circuiting with
// its own code:
//
//	status      → 9270 expired / 9271 revoked（未知值同樣 9271，deny-by-default）
//	target      → 9263 target mismatch
//	capability  → 9262 capability denied
//
// granted is non-empty exactly when deniedCode is 0, so callers can branch on either
// without the two disagreeing.
//
// Items 7 (授權來源仍有效) and 8 (目標店家狀態仍可代行) of §8.1.3 are NOT decided
// here: both need I/O (delegation 模組的授權表、seller 模組的 sellers.status)
// and this function is pure, exactly like StoreOperable in pkg/delegation.
func GrantedAdminCapabilities(ctx AdminDelegationContext) (granted []AdminCapability, deniedCode int32) {
	// 1. session status.
	if code := AdminSessionDenial(ctx.SessionStatus); code != 0 {
		return nil, code
	}

	// 2. target store. The comparison is against the session snapshot, never
	// against anything derived from the request; a non-positive value on either
	// side is a mismatch rather than a wildcard.
	if ctx.TargetStoreID <= 0 || ctx.RequestedStoreID <= 0 || ctx.TargetStoreID != ctx.RequestedStoreID {
		return nil, bizcode.DelegationTargetMismatch
	}

	// 3. capability. Unknown tokens in the snapshot are dropped rather than
	// trusted: the set is a closed enumeration and the Gateway is the one issuing
	// it, so anything else in there is drift, not a permission.
	known := make([]AdminCapability, 0, len(ctx.Capabilities))
	holds := false
	for _, c := range ctx.Capabilities {
		if !IsKnownAdmin(c) {
			continue
		}
		known = append(known, c)
		if c == ctx.RequiredCapability {
			holds = true
		}
	}
	if !IsKnownAdmin(ctx.RequiredCapability) || !holds {
		return nil, bizcode.DelegationCapabilityDenied
	}

	return known, 0
}
