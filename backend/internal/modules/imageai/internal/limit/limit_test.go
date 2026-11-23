package limit

import "testing"

func TestTryAcquireStopsAtTheLimitAndReleaseFreesASlot(t *testing.T) {
	l := New(2)
	r1, ok1 := l.TryAcquire()
	_, ok2 := l.TryAcquire()
	if !ok1 || !ok2 {
		t.Fatal("前兩個名額要拿得到")
	}
	if _, ok := l.TryAcquire(); ok {
		t.Fatal("超過上限卻拿到名額")
	}
	r1()
	if _, ok := l.TryAcquire(); !ok {
		t.Fatal("釋放後要能再拿")
	}
}

func TestNonPositiveLimitFallsBackToOne(t *testing.T) {
	l := New(0)
	if _, ok := l.TryAcquire(); !ok {
		t.Fatal("上限 0 時至少要有 1 個名額")
	}
	if _, ok := l.TryAcquire(); ok {
		t.Fatal("上限 0 應取 1")
	}
}
