package tsid

import (
	"sort"
	"sync"
	"testing"
	"time"
)

// TestGenerator_SequenceOverflowRollsToNextMillisecond is a white-box test:
// it constructs a *generator directly (rather than going through
// NewGenerator()) with seq primed two calls away from the 2^22 overflow
// boundary, so the busy-wait/rollover branch at the bottom of next()
// (tsid.go) is exercised deterministically. Reaching that branch by driving
// NewGenerator()'s public function alone would require ~4.19M calls landing
// in a single millisecond — three orders of magnitude above this package's
// measured throughput (~1-1.5M ops/s with the mutex), so it's not something
// the black-box tests above can trigger reliably.
//
// Runs on a timeout-guarded goroutine: if the rollover logic ever regresses
// into an infinite spin, the test fails fast instead of hanging the whole
// suite.
//
// lastMs for the primed generator is NOT taken from an independent
// time.Now() read in the test — that raced against next()'s own internal
// clock read across a millisecond boundary (flaky under -race, where
// instrumentation overhead widens the window between the two reads; caught
// by `go test -race -run TestGenerator_SequenceOverflowRollsToNextMillisecond
// -count=300`, reproduced roughly once per run). Instead, a real seed call
// through next() records the generator's own view of "now" as lastMs, so
// there is only ever one clock read establishing that value.
func TestGenerator_SequenceOverflowRollsToNextMillisecond(t *testing.T) {
	done := make(chan struct{})
	var (
		seedID, firstID, secondID, thirdID int64
		lastMsAtStart                      int64
	)

	go func() {
		defer close(done)

		g := &generator{}
		// Seed call: takes the normal path (zero-value lastMs/seq), so the
		// resulting id's timestamp portion is exactly what g itself
		// recorded as lastMs — no separately-read wall clock involved.
		seedID = g.next()
		lastMsAtStart = seedID >> 22

		// Prime seq to two calls away from overflow, in-place on the same
		// instance that just made the seed call, so lastMs is still that
		// self-recorded value.
		g.seq = (1 << 22) - 2

		// seq: (1<<22)-2 -> (1<<22)-1, same millisecond, no overflow yet.
		firstID = g.next()
		// seq: (1<<22)-1 -> (1<<22), hits the overflow branch: busy-waits
		// for the next millisecond, then resets seq to 0.
		secondID = g.next()
		// first call after rollover: should be seq=1 in the new millisecond.
		thirdID = g.next()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for sequence overflow rollover — busy-wait loop may be stuck")
	}

	firstTs := firstID >> 22
	secondTs := secondID >> 22
	thirdTs := thirdID >> 22

	if firstTs != lastMsAtStart {
		t.Fatalf("pre-overflow call: expected timestamp %d, got %d", lastMsAtStart, firstTs)
	}
	// The overflow call must have genuinely waited for wall-clock time to
	// advance past lastMs, not fabricated a value while still in the same
	// millisecond.
	if secondTs <= lastMsAtStart {
		t.Fatalf("overflow call did not roll to a later millisecond: lastMsAtStart=%d secondTs=%d", lastMsAtStart, secondTs)
	}
	secondSeq := secondID & ((1 << 22) - 1)
	if secondSeq != 0 {
		t.Fatalf("expected seq to reset to 0 right after overflow, got %d", secondSeq)
	}

	// Subsequent calls stay in (or move past) that same later millisecond,
	// with seq continuing to increment from 0.
	if thirdTs < secondTs {
		t.Fatalf("timestamp regressed after overflow: secondTs=%d thirdTs=%d", secondTs, thirdTs)
	}
	if thirdID <= secondID {
		t.Fatalf("id not strictly increasing across overflow boundary: secondID=%d thirdID=%d", secondID, thirdID)
	}
	if thirdTs == secondTs {
		thirdSeq := thirdID & ((1 << 22) - 1)
		if thirdSeq != 1 {
			t.Fatalf("expected seq=1 for the call right after rollover (same ms as secondTs), got %d", thirdSeq)
		}
	}

	if firstID == secondID || secondID == thirdID || firstID == thirdID {
		t.Fatalf("ids not unique across overflow boundary: first=%d second=%d third=%d", firstID, secondID, thirdID)
	}
}

// TestNewGenerator_SingleGeneratorRapidCallsUnique reproduces the original
// pricebook-service batch-import collision scenario: a single generator
// called tightly in a loop with no delay, so most calls land in the same
// millisecond (or a handful of adjacent ones). With the old pure-random
// design this had a near-100% collision probability at this volume
// (birthday paradox over a 2^22 space); the per-millisecond sequence
// counter must make every value unique.
func TestNewGenerator_SingleGeneratorRapidCallsUnique(t *testing.T) {
	gen := NewGenerator()

	const n = 100_000
	ids := make(map[int64]struct{}, n)
	for i := 0; i < n; i++ {
		id := gen()
		if _, dup := ids[id]; dup {
			t.Fatalf("duplicate id %d at call %d", id, i)
		}
		ids[id] = struct{}{}
	}

	if len(ids) != n {
		t.Fatalf("expected %d unique ids, got %d", n, len(ids))
	}
}

// TestNewGenerator_NonDecreasing asserts that IDs produced by a single
// generator instance never go backwards, preserving the time-sortable
// property the package promises.
func TestNewGenerator_NonDecreasing(t *testing.T) {
	gen := NewGenerator()

	const n = 100_000
	prev := gen()
	for i := 1; i < n; i++ {
		cur := gen()
		if cur <= prev {
			t.Fatalf("id not strictly increasing at call %d: prev=%d cur=%d", i, prev, cur)
		}
		prev = cur
	}
}

// TestNewGenerator_ConcurrentUnique drives many goroutines against the same
// generator instance concurrently (simulating concurrent HTTP/gRPC request
// handlers each minting an ID) and asserts global uniqueness across all of
// them. Must also pass under `go test -race`.
func TestNewGenerator_ConcurrentUnique(t *testing.T) {
	gen := NewGenerator()

	const goroutines = 50
	const perGoroutine = 2000

	results := make([][]int64, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(idx int) {
			defer wg.Done()
			batch := make([]int64, perGoroutine)
			for i := 0; i < perGoroutine; i++ {
				batch[i] = gen()
			}
			results[idx] = batch
		}(g)
	}
	wg.Wait()

	seen := make(map[int64]struct{}, goroutines*perGoroutine)
	for _, batch := range results {
		for _, id := range batch {
			if _, dup := seen[id]; dup {
				t.Fatalf("duplicate id %d across goroutines", id)
			}
			seen[id] = struct{}{}
		}
	}

	total := goroutines * perGoroutine
	if len(seen) != total {
		t.Fatalf("expected %d unique ids across goroutines, got %d", total, len(seen))
	}
}

// TestNewGenerator_TimestampSanity asserts the timestamp portion of a
// generated ID (id >> 22) matches wall-clock time at the moment of the
// call within a generous tolerance, proving the sequence-counter rework
// didn't break time-sortability. This is a sanity bound (loose window),
// not a performance/latency assertion.
func TestNewGenerator_TimestampSanity(t *testing.T) {
	gen := NewGenerator()

	before := time.Now().UnixMilli()
	id := gen()
	after := time.Now().UnixMilli()

	ts := id >> 22
	const toleranceMs = 50
	if ts < before-toleranceMs || ts > after+toleranceMs {
		t.Fatalf("timestamp portion %d out of expected range [%d, %d] (+/- %dms)", ts, before, after, toleranceMs)
	}
}

// TestNewGenerator_SortedOrderMatchesCallOrder is a redundant but cheap
// cross-check on top of TestNewGenerator_NonDecreasing: sorting all ids
// from a single generator should not reorder them relative to call order.
func TestNewGenerator_SortedOrderMatchesCallOrder(t *testing.T) {
	gen := NewGenerator()

	const n = 10_000
	ids := make([]int64, n)
	for i := 0; i < n; i++ {
		ids[i] = gen()
	}

	sorted := make([]int64, n)
	copy(sorted, ids)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	for i := range ids {
		if ids[i] != sorted[i] {
			t.Fatalf("call-order sequence diverges from sorted order at index %d: ids[i]=%d sorted[i]=%d", i, ids[i], sorted[i])
		}
	}
}
