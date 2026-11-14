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

// 取得 ID 以冪等鍵得到同一個外部帳戶，完成設定可以換信箱重送，每次呼叫的信箱都記下來（D55 買到的性質）。
// mutation：Fake 的取得不以鍵回同一個帳戶 → 兩個 ID 不同，失敗。
func TestFakeAcquireWithTheSameKeyReturnsTheSameAccountAndConfigureIsRepeatable(t *testing.T) {
	f := NewFake()
	ctx := deadlineCtx(t)
	a, err := f.AcquireAccount(ctx, AcquireInput{StoreID: 1, IdempotencyKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.AcquireAccount(ctx, AcquireInput{StoreID: 1, IdempotencyKey: "k"})
	if err != nil || a != b || f.Accounts() != 1 {
		t.Fatalf("a=%q b=%q err=%v 帳戶數=%d", a, b, err, f.Accounts())
	}
	for _, email := range []string{"old@example.com", "new@example.com"} {
		if err := f.ConfigureAccount(ctx, ConfigureInput{ExternalID: a, ContactEmail: email}); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.ConfigureEmails(); !slices.Equal(got, []string{"old@example.com", "new@example.com"}) {
		t.Fatalf("信箱 %v", got)
	}
	if err := f.ConfigureAccount(ctx, ConfigureInput{ExternalID: "nope", ContactEmail: "x@example.com"}); !errors.Is(err, ErrPermanent) {
		t.Fatalf("不存在的帳戶應是永久失敗：%v", err)
	}
}

func TestFakeAcquireThatLosesItsResponseStillCreatesTheAccountForTheKey(t *testing.T) {
	f := NewFake()
	ctx := deadlineCtx(t)
	f.FailAcquireAfterCreate(errors.New("timeout"))
	if _, err := f.AcquireAccount(ctx, AcquireInput{StoreID: 1, IdempotencyKey: "k"}); err == nil {
		t.Fatal("應回錯誤")
	}
	id, err := f.AcquireAccount(ctx, AcquireInput{StoreID: 1, IdempotencyKey: "k"})
	if err != nil || id == "" || f.Accounts() != 1 {
		t.Fatalf("id=%q err=%v 帳戶數=%d", id, err, f.Accounts())
	}
}
