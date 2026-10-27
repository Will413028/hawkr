package handler

import "sync"

// maxChatWSConnsPerUser caps concurrent chat WebSocket connections per account
// to bound the WS DoS surface (§11.1). Generous headroom for legit use —
// web (a few tabs) + buyer app + seller app + reconnect overlap — while
// blocking a runaway. It's a const; bump if real usage needs more.
const maxChatWSConnsPerUser = 10

// In-memory per-account live-connection counter. Single-replica in-mem state,
// consistent with the im hub itself (design doc D4). The gateway terminates and
// proxies every browser WS, so one cap here protects both gateway and im.
// ponytail: in-mem map, fine while single-replica; a 2nd replica needs the
// Redis backplane (P2) and this would move with presence.
var (
	wsConnMu    sync.Mutex
	wsConnCount = map[int64]int{}
)

// acquireWSSlot reserves a connection slot for accountID. It returns a release
// func and ok=true when under the cap; ok=false (release nil) when the account
// is already at maxChatWSConnsPerUser. release is idempotent-safe to defer and
// deletes the key at zero so the map can't grow unbounded by accountID.
func acquireWSSlot(accountID int64) (release func(), ok bool) {
	wsConnMu.Lock()
	defer wsConnMu.Unlock()
	if wsConnCount[accountID] >= maxChatWSConnsPerUser {
		return nil, false
	}
	wsConnCount[accountID]++
	return func() {
		wsConnMu.Lock()
		defer wsConnMu.Unlock()
		if wsConnCount[accountID] <= 1 {
			delete(wsConnCount, accountID)
			return
		}
		wsConnCount[accountID]--
	}, true
}
