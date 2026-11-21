//go:build integration

package crm_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// lockWaiters 數目前等在鎖上的連線（每個整合測試有自己的資料庫，不受平行測試影響）。
func (f *fixture) lockWaiters() int64 {
	f.t.Helper()
	return f.scalar("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'")
}

// 進行中的標籤寫入與店家退場交錯：寫入已經通過停用檢查、卡在標籤的唯一鍵上（另一個未提交的 transaction 先插了同名標籤）時，店家退場提交、
// 抹除跑完；之後寫入才繼續。店家退場後不得留下標籤。
//
// 現況的缺口：標籤寫入只讀 inactive_parties，不取身分鎖（LockStoreTags 鎖的是標籤命名空間，與退場不互斥），所以寫入在退場提交之後仍然成功，
// 資料留在已退場店家名下、抹除 job 不會再回頭刪。擁有者守門之後，退場要等寫入提交（寫入先取店家列的 FOR KEY SHARE），抹除在寫入之後才跑。
func TestTagWriteInFlightAcrossStoreRetirementLeavesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()

	blocker, err := f.db.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx) //nolint:errcheck
	if _, err := blocker.Exec(ctx, "INSERT INTO crm.tags (id, store_id, name, created_at, updated_at) VALUES ($1, $2, '卡住', $3, $3)", uuid.New(), storeB, epoch); err != nil {
		t.Fatal(err)
	}

	written := make(chan error, 1)
	go func() {
		_, err := f.m.CreateTag(ctx, storeB, "卡住")
		written <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	waitUntil := func(what string, ok func() bool) {
		for !ok() {
			select {
			case err := <-written:
				t.Fatalf("%s：寫入已提前結束（%v）", what, err)
			default:
			}
			if time.Now().After(deadline) {
				t.Fatal(what)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitUntil("寫入沒有卡在標籤的唯一鍵上", func() bool { return f.lockWaiters() >= 1 })

	retired := make(chan error, 1)
	go func() { retired <- f.retireStoreErr(storeB) }()
	erased := false
	erase := func() {
		if err := f.m.ErasePartyData(ctx, "STORE", storeB); err != nil {
			t.Error(err)
		}
		erased = true
	}
	// 退場要不是立刻提交（現況：寫入不擋它），就是等在寫入的守門鎖上（兩個連線在等）。前者在寫入繼續之前先跑抹除。
	for !erased {
		select {
		case err := <-retired:
			if err != nil {
				t.Fatal(err)
			}
			erase()
			retired <- nil
		default:
			if f.lockWaiters() >= 2 {
				goto release
			}
			if time.Now().After(deadline) {
				t.Fatal("退場既沒有提交也沒有等在守門鎖上")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
release:
	if err := blocker.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil && apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("寫入：%v", err)
	}
	if err := <-retired; err != nil {
		t.Fatal(err)
	}
	if !erased {
		erase()
	}
	if n := f.scalar("SELECT count(*) FROM crm.tags WHERE store_id = $1", storeB); n != 0 {
		t.Fatalf("店家退場並抹除之後還留下 %d 個標籤", n)
	}
}
