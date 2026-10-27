package syncutil

import (
	"context"
	"sync"
)

// Item pairs one input Param with the outcome of running it through RunPool's
// handler. Err is non-nil (and Result the zero value) if handler returned an
// error, panicked, or was never dispatched because ctx was cancelled first
// (in which case Err is ctx.Err()).
type Item[Param, Result any] struct {
	Param  Param
	Result Result
	Err    error
}

// RunPool runs handler over items using up to workerCount concurrent
// goroutines and returns one Item per input, in the same order as items.
// A panic inside handler for one item is recovered and reported as that
// item's Err (see recoverPanic) — it does not lose or block the other items.
//
// workerCount <= 0 defaults to 1. If ctx is cancelled, items not yet
// dispatched to a worker are abandoned with Err set to ctx.Err(); items
// already in flight run to completion.
func RunPool[Param, Result any](
	ctx context.Context,
	items []Param,
	workerCount int,
	handler func(context.Context, Param) (Result, error),
) []Item[Param, Result] {
	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > len(items) {
		workerCount = len(items)
	}

	results := make([]Item[Param, Result], len(items))
	for i, p := range items {
		results[i].Param = p
	}
	if workerCount == 0 {
		return results
	}

	dispatched := make([]bool, len(items))
	jobs := make(chan int)
	go func() {
		defer close(jobs)
		for i := range items {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
				dispatched[i] = true
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for w := 0; w < workerCount; w++ {
		go func() {
			defer wg.Done()
			for idx := range jobs {
				idx := idx
				var res Result
				err := recoverPanic(ctx, func() error {
					r, err := handler(ctx, items[idx])
					res = r
					return err
				}, nil)
				results[idx].Result = res
				results[idx].Err = err
			}
		}()
	}
	wg.Wait()

	for i, ok := range dispatched {
		if !ok {
			results[i].Err = ctx.Err()
		}
	}
	return results
}
