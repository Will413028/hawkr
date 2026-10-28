package delegation

import "github.com/Will413028/hawkr/go-pkg/errors/bizcode"

// sellers.status lifecycle values (seller module schema chk_sellers_status).
const (
	storeStatusActive              = "active"
	storeStatusSuspended           = "suspended"
	storeStatusSuspendedGovernance = "suspended_governance"
	storeStatusClosed              = "closed"
)

// StoreOperable answers "can this store be operated on behalf of right now"
// (§8.1.3 第 8 項). Pure function, no I/O — its only consumer is the Gateway
// authorization chain (gateway/internal/router/delegated_capability.go).
//
// deny-by-default whitelist: only an active, undeleted store is allowed. Any
// lifecycle value added in the future is refused until someone comes here and
// decides otherwise.
//
// deleted is checked before status because deleted_at being set always implies
// status='closed' (chk_sellers_deleted_requires_closed); testing status first
// would report a deleted store as 9272 instead of 9265.
//
// A store that cannot be found at all does not reach this function: that is the
// caller's business (an RPC NOT_FOUND) and it maps to 9265 there.
func StoreOperable(lifecycleStatus string, deleted bool) (ok bool, deniedCode int32) {
	if deleted {
		return false, bizcode.DelegationStoreNotDelegable
	}
	switch lifecycleStatus {
	case storeStatusActive:
		return true, 0
	case storeStatusSuspended, storeStatusSuspendedGovernance, storeStatusClosed:
		return false, bizcode.DelegationStoreNotOperable
	default:
		// Unknown lifecycle value. 9272 ("currently not operable") rather than
		// 9265 ("not delegable / does not exist"): the honest statement about a
		// value we do not recognise is that the store's current state does not
		// permit the operation, not that the store is absent.
		return false, bizcode.DelegationStoreNotOperable
	}
}
