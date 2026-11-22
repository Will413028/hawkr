package database

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeTx struct {
	pgx.Tx
	commits, rollbacks int
	commitErr          error
}

func (f *fakeTx) Commit(context.Context) error   { f.commits++; return f.commitErr }
func (f *fakeTx) Rollback(context.Context) error { f.rollbacks++; return nil }

type fakePool struct{ tx *fakeTx }

func (p fakePool) Begin(context.Context) (pgx.Tx, error) { return p.tx, nil }

func newFake() (TxManager, *fakeTx) {
	tx := &fakeTx{}
	return &pgxTxManager{pool: fakePool{tx}}, tx
}

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	m, tx := newFake()
	if err := m.WithTx(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("commits=%d rollbacks=%d", tx.commits, tx.rollbacks)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	m, tx := newFake()
	want := errors.New("boom")
	if err := m.WithTx(context.Background(), func(context.Context) error { return want }); err != want {
		t.Fatalf("err=%v", err)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("commits=%d rollbacks=%d", tx.commits, tx.rollbacks)
	}
}

func TestWithTx_RollsBackOnPanicAndRepanics(t *testing.T) {
	m, tx := newFake()
	func() {
		defer func() {
			if r := recover(); r != "kaboom" {
				t.Fatalf("recovered %v, want the original panic", r)
			}
		}()
		_ = m.WithTx(context.Background(), func(context.Context) error { panic("kaboom") })
	}()
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("commits=%d rollbacks=%d", tx.commits, tx.rollbacks)
	}
}

func TestWithTx_RollsBackOnGoexit(t *testing.T) {
	m, tx := newFake()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.WithTx(context.Background(), func(context.Context) error {
			runtime.Goexit()
			return nil
		})
	}()
	wg.Wait()
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("commits=%d rollbacks=%d", tx.commits, tx.rollbacks)
	}
}

func TestWithTx_NestedReusesOuterTx(t *testing.T) {
	m, tx := newFake()
	err := m.WithTx(context.Background(), func(ctx context.Context) error {
		return m.WithTx(ctx, func(context.Context) error { return nil })
	})
	if err != nil || tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("err=%v commits=%d rollbacks=%d", err, tx.commits, tx.rollbacks)
	}
}
