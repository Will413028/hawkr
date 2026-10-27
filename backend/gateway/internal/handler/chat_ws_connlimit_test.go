package handler

import (
	"sync"
	"testing"
)

// liveWSConns reads the live connection count for an account under the lock
// (white-box; the map is package-private).
func liveWSConns(accountID int64) int {
	wsConnMu.Lock()
	defer wsConnMu.Unlock()
	return wsConnCount[accountID]
}

func TestAcquireWSSlot_CapEnforced(t *testing.T) {
	const acct int64 = 9001 // distinct id so the global map stays test-isolated

	releases := make([]func(), 0, maxChatWSConnsPerUser)
	for i := 0; i < maxChatWSConnsPerUser; i++ {
		release, ok := acquireWSSlot(acct)
		if !ok {
			t.Fatalf("acquire %d/%d should succeed under the cap", i+1, maxChatWSConnsPerUser)
		}
		releases = append(releases, release)
	}

	// One past the cap must be rejected, with a nil release.
	if release, ok := acquireWSSlot(acct); ok || release != nil {
		t.Fatalf("acquire past cap must fail with nil release; got ok=%v release!=nil=%v", ok, release != nil)
	}

	// Freeing one slot lets exactly one more in.
	releases[0]()
	releases = releases[1:]
	release, ok := acquireWSSlot(acct)
	if !ok {
		t.Fatal("acquire after releasing a slot should succeed")
	}
	releases = append(releases, release)

	// Releasing everything must drop the account from the map (no leak).
	for _, r := range releases {
		r()
	}
	if n := liveWSConns(acct); n != 0 {
		t.Fatalf("after releasing all, count should be 0 (key deleted); got %d", n)
	}
	wsConnMu.Lock()
	_, present := wsConnCount[acct]
	wsConnMu.Unlock()
	if present {
		t.Fatal("zero-count account must be deleted from the map, not left at 0")
	}
}

// TestAcquireWSSlot_ConcurrentNoLeak hammers acquire/release from many
// goroutines; run under -race to catch lock mistakes. The count must never be
// observably exceeded and must settle back to absent.
func TestAcquireWSSlot_ConcurrentNoLeak(t *testing.T) {
	const acct int64 = 9002
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if release, ok := acquireWSSlot(acct); ok {
				if n := liveWSConns(acct); n > maxChatWSConnsPerUser {
					t.Errorf("live count %d exceeded cap %d", n, maxChatWSConnsPerUser)
				}
				release()
			}
		}()
	}
	wg.Wait()

	wsConnMu.Lock()
	_, present := wsConnCount[acct]
	wsConnMu.Unlock()
	if present {
		t.Fatal("after concurrent acquire/release, account must be absent from the map")
	}
}
