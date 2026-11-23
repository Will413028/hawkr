//go:build integration

package identity_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestShareLockAccountReportsLiveDeletedAndMissing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	live := f.account(t, "user_live")
	gone := f.account(t, "user_gone")
	if err := f.m.DeleteAccount(ctx, gone); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}

	if _, err := f.m.ShareLockAccount(ctx, live); err == nil {
		t.Fatal("在 transaction 外呼叫應該失敗")
	}
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		if deleted, err := f.m.ShareLockAccount(ctx, live); err != nil || deleted {
			t.Errorf("存活的帳號 = %v, %v", deleted, err)
		}
		if deleted, err := f.m.ShareLockAccount(ctx, gone); err != nil || !deleted {
			t.Errorf("已刪除的帳號 = %v, %v", deleted, err)
		}
		if _, err := f.m.ShareLockAccount(ctx, 987654321); apperr.KindOf(err) != apperr.KindNotFound {
			t.Errorf("不存在的帳號 = %v，應該是 NotFound", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestShareLockAccountAllowsOtherReadersButBlocksDeletion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")

	holding := make(chan struct{})
	release := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			if _, err := f.m.ShareLockAccount(ctx, id); err != nil {
				return err
			}
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding

	otherCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := f.db.InTx(otherCtx, func(ctx context.Context) error {
		_, err := f.m.ShareLockAccount(ctx, id)
		return err
	}); err != nil {
		t.Fatalf("第二個共享鎖不該等第一個：%v", err)
	}

	deleteDone := make(chan error, 1)
	go func() { deleteDone <- f.m.DeleteAccount(ctx, id) }()
	select {
	case err := <-deleteDone:
		t.Fatalf("共享鎖還在時刪帳號不該完成：%v", err)
	case <-time.After(500 * time.Millisecond):
	}
	close(release)
	if err := <-holderDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-deleteDone:
		if err != nil {
			t.Fatalf("DeleteAccount: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("共享鎖釋放後刪帳號沒有完成")
	}
}
