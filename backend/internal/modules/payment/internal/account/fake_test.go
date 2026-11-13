package account

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func deadlineCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

// 重試換了信箱：同一把鍵得到同一個外部帳戶，每次呼叫的信箱都記下來（D55 買到的性質）。
// mutation：Fake 的建立不以鍵回同一個帳戶 → 兩個 ID 不同，失敗。
func TestFakeEnsureWithTheSameKeyReturnsTheSameAccountWhateverTheEmail(t *testing.T) {
	f := NewFake()
	ctx := deadlineCtx(t)
	a, err := f.EnsureAccount(ctx, EnsureInput{StoreID: 1, IdempotencyKey: "k", ContactEmail: "old@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.EnsureAccount(ctx, EnsureInput{StoreID: 1, IdempotencyKey: "k", ContactEmail: "new@example.com"})
	if err != nil || a != b || f.Accounts() != 1 {
		t.Fatalf("a=%q b=%q err=%v 帳戶數=%d", a, b, err, f.Accounts())
	}
	if got := f.EnsureEmails(); !slices.Equal(got, []string{"old@example.com", "new@example.com"}) {
		t.Fatalf("信箱 %v", got)
	}
}

func TestFakeEnsureThatLosesItsResponseStillCreatesTheAccountForTheKey(t *testing.T) {
	f := NewFake()
	ctx := deadlineCtx(t)
	f.FailEnsureAfterCreate(errors.New("timeout"))
	if _, err := f.EnsureAccount(ctx, EnsureInput{StoreID: 1, IdempotencyKey: "k", ContactEmail: "old@example.com"}); err == nil {
		t.Fatal("應回錯誤")
	}
	id, err := f.EnsureAccount(ctx, EnsureInput{StoreID: 1, IdempotencyKey: "k", ContactEmail: "new@example.com"})
	if err != nil || id == "" || f.Accounts() != 1 {
		t.Fatalf("id=%q err=%v 帳戶數=%d", id, err, f.Accounts())
	}
}

func TestFakeResolveFindsOrCreatesTheAccountOfTheKeyAndEnsureReusesIt(t *testing.T) {
	f := NewFake()
	ctx := deadlineCtx(t)
	r, err := f.ResolveAccount(ctx, ResolveInput{StoreID: 1, IdempotencyKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	e, err := f.EnsureAccount(ctx, EnsureInput{StoreID: 1, IdempotencyKey: "k", ContactEmail: "a@example.com"})
	if err != nil || e != r || f.Accounts() != 1 || len(f.Resolves()) != 1 {
		t.Fatalf("r=%q e=%q err=%v 帳戶數=%d", r, e, err, f.Accounts())
	}
}
