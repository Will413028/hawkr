package syncutil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGroup_Go_PropagatesError(t *testing.T) {
	g := NewGroup(context.Background())
	wantErr := errors.New("boom")
	g.Go(func() error { return wantErr })
	if err := g.Wait(); !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}

func TestGroup_Go_RecoversPanic(t *testing.T) {
	g := NewGroup(context.Background())
	g.Go(func() error { panic("kaboom") })
	if err := g.Wait(); err == nil {
		t.Fatal("expected an error after a recovered panic, got nil")
	}
}

func TestGroup_Go_PanicDoesNotStopSiblings(t *testing.T) {
	g := NewGroup(context.Background())
	ran := make(chan struct{}, 1)
	g.Go(func() error { panic("kaboom") })
	g.Go(func() error {
		ran <- struct{}{}
		return nil
	})
	_ = g.Wait()
	select {
	case <-ran:
	default:
		t.Fatal("sibling goroutine did not run after a panic in another goroutine")
	}
}

func TestGroup_WithPanicCallback_Invoked(t *testing.T) {
	var got any
	g := NewGroup(context.Background(), WithPanicCallback(func(r any) { got = r }))
	g.Go(func() error { panic("kaboom") })
	_ = g.Wait()
	if got != "kaboom" {
		t.Fatalf("expected panic callback to receive %q, got %v", "kaboom", got)
	}
}

func TestGroup_TryGo_RecoversPanic(t *testing.T) {
	g := NewGroup(context.Background())
	if ok := g.TryGo(func() error { panic("kaboom") }); !ok {
		t.Fatal("expected TryGo to accept the task")
	}
	if err := g.Wait(); err == nil {
		t.Fatal("expected an error after a recovered panic, got nil")
	}
}

func TestSafeGo_RunsF(t *testing.T) {
	done := make(chan struct{})
	SafeGo(context.Background(), func() { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("f was not run within 1s")
	}
}

func TestSafeGo_PanicDoesNotCrashOrBlockCaller(t *testing.T) {
	done := make(chan struct{})
	SafeGo(context.Background(), func() {
		defer close(done)
		panic("kaboom")
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("f did not complete within 1s — panic may have escaped recovery")
	}
	// Reaching here without the test process crashing IS the assertion: a
	// panic in f must never propagate past SafeGo.
}
