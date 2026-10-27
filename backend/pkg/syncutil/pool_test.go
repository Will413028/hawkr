package syncutil

import (
	"context"
	"errors"
	"testing"
)

func TestRunPool_PreservesOrderAndValues(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	results := RunPool(context.Background(), items, 2, func(_ context.Context, n int) (int, error) {
		return n * n, nil
	})
	if len(results) != len(items) {
		t.Fatalf("want %d results, got %d", len(items), len(results))
	}
	for i, r := range results {
		if r.Param != items[i] {
			t.Fatalf("index %d: want Param %d, got %d", i, items[i], r.Param)
		}
		if r.Err != nil {
			t.Fatalf("index %d: unexpected error %v", i, r.Err)
		}
		if want := items[i] * items[i]; r.Result != want {
			t.Fatalf("index %d: want Result %d, got %d", i, want, r.Result)
		}
	}
}

func TestRunPool_PerItemError(t *testing.T) {
	items := []int{1, 2, 3}
	wantErr := errors.New("odd number")
	results := RunPool(context.Background(), items, 3, func(_ context.Context, n int) (int, error) {
		if n%2 != 0 {
			return 0, wantErr
		}
		return n, nil
	})
	for i, r := range results {
		if items[i]%2 != 0 {
			if !errors.Is(r.Err, wantErr) {
				t.Fatalf("index %d: want %v, got %v", i, wantErr, r.Err)
			}
			continue
		}
		if r.Err != nil {
			t.Fatalf("index %d: unexpected error %v", i, r.Err)
		}
	}
}

func TestRunPool_PanicIsolatedPerItem(t *testing.T) {
	items := []int{1, 2, 3}
	results := RunPool(context.Background(), items, 1, func(_ context.Context, n int) (int, error) {
		if n == 2 {
			panic("kaboom")
		}
		return n, nil
	})
	if results[1].Err == nil {
		t.Fatal("expected item 2 to report a recovered-panic error")
	}
	if results[0].Err != nil || results[0].Result != 1 {
		t.Fatalf("item 1 should be unaffected by item 2's panic, got %+v", results[0])
	}
	if results[2].Err != nil || results[2].Result != 3 {
		t.Fatalf("item 3 should be unaffected by item 2's panic, got %+v", results[2])
	}
}

func TestRunPool_ZeroWorkerCountDefaultsToOne(t *testing.T) {
	items := []int{1, 2}
	results := RunPool(context.Background(), items, 0, func(_ context.Context, n int) (int, error) {
		return n, nil
	})
	for i, r := range results {
		if r.Err != nil || r.Result != items[i] {
			t.Fatalf("index %d: got %+v", i, r)
		}
	}
}

func TestRunPool_EmptyItems(t *testing.T) {
	results := RunPool[int, int](context.Background(), nil, 4, func(_ context.Context, n int) (int, error) {
		t.Fatal("handler should not be called for empty items")
		return 0, nil
	})
	if len(results) != 0 {
		t.Fatalf("want 0 results, got %d", len(results))
	}
}

func TestRunPool_ContextCancelledAbandonsUndispatchedItems(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	started := make(chan struct{})
	done := make(chan []Item[int, int], 1)

	// Single worker: it parks on item 1 until the test cancels ctx, so no
	// other item can be dispatched to it in the meantime (unbuffered jobs
	// channel) — that's what lets us assert some items were abandoned.
	go func() {
		done <- RunPool(ctx, items, 1, func(c context.Context, n int) (int, error) {
			if n == 1 {
				close(started)
				<-c.Done()
			}
			return n, nil
		})
	}()

	<-started
	cancel()
	results := <-done

	if results[0].Err != nil {
		t.Fatalf("item 1 was already in flight when ctx was cancelled and should still complete: %+v", results[0])
	}
	abandoned := false
	for _, r := range results[1:] {
		if errors.Is(r.Err, context.Canceled) {
			abandoned = true
		}
	}
	if !abandoned {
		t.Fatal("expected at least one item to be abandoned with context.Canceled after cancel")
	}
}
