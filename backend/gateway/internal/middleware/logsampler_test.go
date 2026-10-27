package middleware

import (
	"testing"
	"time"
)

// Each call site owns its budget. A shared one would let the noisiest site spend
// the window and silence the others — the exact failure sampling exists to stop.
func TestLogSampler_CallSitesDoNotShareABudget(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	noisy := newLogSampler(time.Minute, 1)
	noisy.now = clk.Now
	quiet := newLogSampler(time.Minute, 1)
	quiet.now = clk.Now

	if ok, _ := noisy.admit("noisy-1"); !ok {
		t.Fatal("first key must be admitted")
	}
	if ok, _ := noisy.admit("noisy-2"); ok {
		t.Fatal("precondition: the noisy site's budget must be spent")
	}
	if ok, _ := quiet.admit("quiet-1"); !ok {
		t.Fatal("a second call site must keep its own budget — one flooding site must never silence another")
	}

	clk.Advance(time.Minute + time.Second)
	if ok, dropped := noisy.admit("noisy-1"); !ok || dropped != 1 {
		t.Fatalf("noisy site after roll: admitted=%v dropped=%d, want true/1", ok, dropped)
	}
	if _, dropped := quiet.admit("quiet-2"); dropped != 0 {
		t.Fatalf("quiet site reported dropped=%d, want 0 — one site's suppressions must not be attributed to another", dropped)
	}
}

// The count must cover repeats of an already-logged key, not just keys turned
// away by the cap: a sustained flood is one key, and reporting 0 for it would
// make a 100x incident read as a one-off.
func TestLogSampler_CountsRepeatsNotOnlyCapOverflow(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1700000000, 0)}
	s := newLogSampler(time.Minute, 10) // cap never reached: one key only
	s.now = clk.Now

	const occurrences = 100
	for i := 0; i < occurrences; i++ {
		s.admit("same-key")
	}
	clk.Advance(time.Minute + time.Second)
	admitted, dropped := s.admit("same-key")
	if !admitted {
		t.Fatal("the key must be admitted again in the new window")
	}
	if dropped != occurrences-1 {
		t.Fatalf("dropped = %d, want %d — repeats are how a single-source flood presents, so leaving them uncounted understates it entirely",
			dropped, occurrences-1)
	}
}
