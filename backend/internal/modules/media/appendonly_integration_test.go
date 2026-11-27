//go:build integration

package media_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/lockorder"
)

type sqlRecorder struct {
	mu  sync.Mutex
	sql []string
}

func (r *sqlRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sql = append(r.sql, data.SQL)
	return ctx
}

func (r *sqlRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (r *sqlRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sql = nil
}

func (r *sqlRecorder) statements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.sql...)
}

func recordedModule(t *testing.T, f *fixture) (*media.Module, *db.DB, *sqlRecorder) {
	t.Helper()
	rec := &sqlRecorder{}
	d, err := db.Open(t.Context(), db.Config{URL: f.d.Pool().Config().ConnString(), MaxConns: 4, ConnectRetryBudget: 10 * time.Second, Tracer: rec})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	m := media.New(media.Deps{
		DB: d, Jobs: f.jobs, Store: f.store, NewID: uuid.NewV7, Now: f.clock.Now, Keyring: f.keyring,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return m, d, rec
}

func assertAppendOnly(t *testing.T, rec *sqlRecorder, what string) {
	t.Helper()
	stmts := rec.statements()
	inserted := false
	for _, q := range stmts {
		locks, err := lockorder.Classify(q)
		if err != nil {
			t.Fatalf("%s：無法解析 SQL：%v\n%s", what, err, q)
		}
		if len(locks) != 0 {
			t.Errorf("%s：擁有者 transaction 內的 SQL 鎖了既有的列 %+v\n%s", what, locks, q)
		}
		if strings.Contains(q, "river") {
			t.Errorf("%s：擁有者 transaction 內碰了 river schema\n%s", what, q)
		}
		if strings.Contains(strings.ToUpper(q), "INSERT INTO MEDIA.") {
			inserted = true
		}
	}
	if !inserted {
		t.Fatalf("%s：沒有錄到任何 media INSERT，守門沒有走到該走的路徑（共 %d 句）", what, len(stmts))
	}
}

func TestOwnerTransactionIssuesOnlyAppendsAndReads(t *testing.T) {
	f := setup(t)
	m, d, rec := recordedModule(t, f)
	a, b, c, e := ready(t, f, owner), ready(t, f, owner), ready(t, f, owner), ready(t, f, owner)

	t.Run("Confirm 新確認、同 transaction 重入、MarkDeleted、重複與空清單", func(t *testing.T) {
		rec.reset()
		err := d.InTx(t.Context(), func(ctx context.Context) error {
			asset, err := m.Confirm(ctx, owner, a.Token, media.VisibilityPublic)
			if err != nil {
				return err
			}
			if _, err = m.Confirm(ctx, owner, a.Token, media.VisibilityPublic); err != nil {
				return err
			}
			if err := m.MarkDeleted(ctx, []uuid.UUID{asset.ID, asset.ID, uuid.New()}); err != nil {
				return err
			}
			if err := m.MarkDeleted(ctx, []uuid.UUID{asset.ID}); err != nil {
				return err
			}
			return m.MarkDeleted(ctx, nil)
		})
		if err != nil {
			t.Fatal(err)
		}
		assertAppendOnly(t, rec, "新確認與刪除")
	})

	t.Run("MarkDeleted 之後同 transaction 重入", func(t *testing.T) {
		rec.reset()
		var reentry error
		err := d.InTx(t.Context(), func(ctx context.Context) error {
			asset, err := m.Confirm(ctx, owner, b.Token, media.VisibilityPublic)
			if err != nil {
				return err
			}
			if err := m.MarkDeleted(ctx, []uuid.UUID{asset.ID}); err != nil {
				return err
			}
			_, reentry = m.Confirm(ctx, owner, b.Token, media.VisibilityPublic)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		wantKind(t, reentry, apperr.KindFailedPrecondition)
		assertAppendOnly(t, rec, "刪除後重入")
	})

	t.Run("跨 transaction 確認被拒", func(t *testing.T) {
		rec.reset()
		var elsewhere error
		err := d.InTx(t.Context(), func(ctx context.Context) error {
			_, elsewhere = m.Confirm(ctx, owner, a.Token, media.VisibilityPublic)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if !errors.Is(elsewhere, media.ErrUploadAlreadyConfirmed) {
			t.Fatalf("跨 transaction 確認 = %v，預期 ErrUploadAlreadyConfirmed", elsewhere)
		}
		for _, q := range rec.statements() {
			if locks, _ := lockorder.Classify(q); len(locks) != 0 || strings.Contains(q, "river") {
				t.Errorf("被拒的確認也不得加列鎖或碰 river：%s", q)
			}
		}
	})

	t.Run("ConfirmFields", func(t *testing.T) {
		rec.reset()
		err := d.InTx(t.Context(), func(ctx context.Context) error {
			_, err := media.ConfirmFields(ctx, m, owner, []media.ConfirmItem{{Field: "c", Token: c.Token}, {Field: "e", Token: e.Token}},
				media.VisibilityPublic)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		assertAppendOnly(t, rec, "ConfirmFields")
	})
}

func TestConfirmThenMarkDeletedSameTxDeletionWins(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	_, final, _ := uploadKeys(t, f, res.ID)
	var asset media.ConfirmedAsset
	var reentry error
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		var err error
		if asset, err = f.m.Confirm(ctx, owner, res.Token, media.VisibilityPublic); err != nil {
			return err
		}
		if err := f.m.MarkDeleted(ctx, []uuid.UUID{asset.ID}); err != nil {
			return err
		}
		_, reentry = f.m.Confirm(ctx, owner, res.Token, media.VisibilityPublic)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wantKind(t, reentry, apperr.KindFailedPrecondition)
	if got := assetState(t, f, asset.ID); got != "DELETING" || physicalAssetState(t, f, asset.ID) != "STAGED" {
		t.Fatalf("commit 後推導 = %s、物理 = %s，預期推導 DELETING、物理 STAGED", got, physicalAssetState(t, f, asset.ID))
	}
	applyDeletions(t, f)
	assertNoOrphanDeleting(t, f)
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if f.store.has(*final) || assetState(t, f, asset.ID) != "DELETED" {
		t.Fatal("物件應該已刪、資產應該是 DELETED")
	}

	res2 := ready(t, f, owner)
	errRollback := errors.New("rollback")
	err = f.d.InTx(t.Context(), func(ctx context.Context) error {
		a, err := f.m.Confirm(ctx, owner, res2.Token, media.VisibilityPublic)
		if err != nil {
			return err
		}
		if err := f.m.MarkDeleted(ctx, []uuid.UUID{a.ID}); err != nil {
			return err
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	if claimKind(t, f, res2.ID) != "" || count(t, f, `SELECT count(*) FROM media.asset_deletion_requests`) != 0 || assetState(t, f, assetIDOf(t, f, res2.ID)) != "STAGED" {
		t.Fatal("rollback 後不該留下認領或請求")
	}
	f.clock.Advance(media.UploadTTL + time.Minute)
	if got, err := f.m.CollectGarbage(t.Context()); err != nil || got.Claimed != 1 {
		t.Fatalf("rollback 之後 GC = %+v, %v，預期認領 1 筆", got, err)
	}
}

func TestConfirmBlocksOnUncommittedClaim(t *testing.T) {
	for _, commit := range []bool{true, false} {
		t.Run(map[bool]string{true: "A commits", false: "A rolls back"}[commit], func(t *testing.T) {
			testConfirmBlocksOnUncommittedClaim(t, commit)
		})
	}
}

func testConfirmBlocksOnUncommittedClaim(t *testing.T, commit bool) {
	{
		f := setup(t)
		res := ready(t, f, owner)
		aIn, aOut := make(chan struct{}), make(chan struct{})
		var aErr, bErr error
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			aErr = f.d.InTx(t.Context(), func(ctx context.Context) error {
				if _, err := f.m.Confirm(ctx, owner, res.Token, media.VisibilityPublic); err != nil {
					return err
				}
				close(aIn)
				<-aOut
				if !commit {
					return errors.New("rollback")
				}
				return nil
			})
		}()
		<-aIn
		bDone := make(chan struct{})
		go func() {
			defer close(bDone)
			_, bErr = confirmInTx(f, owner, res.Token)
		}()
		select {
		case <-bDone:
			t.Fatal("B 應該等 A 的認領結束，卻先返回了")
		case <-time.After(500 * time.Millisecond):
		}
		close(aOut)
		wg.Wait()
		<-bDone
		if commit {
			if aErr != nil || !errors.Is(bErr, media.ErrUploadAlreadyConfirmed) {
				t.Fatalf("A commit：A=%v B=%v，預期 B 得 ErrUploadAlreadyConfirmed", aErr, bErr)
			}
		} else if aErr == nil || bErr != nil {
			t.Fatalf("A rollback：A=%v B=%v，預期 B 成功", aErr, bErr)
		}
	}
}

func TestConfirmFieldsAndGCNeverDeadlockOrStall(t *testing.T) {
	f := setup(t)
	before := newClock()
	before.Advance(10 * time.Minute)
	after := newClock()
	after.Advance(media.UploadTTL + time.Minute)
	confirmer, collector := f.module(before), f.module(after)
	tokens := make([]media.BeginResult, 3)
	for i := range tokens {
		tokens[i] = ready(t, f, owner)
	}
	var items []media.ConfirmItem
	for i, tk := range tokens {
		items = append(items, media.ConfirmItem{Field: string(rune('a' + i)), Token: tk.Token})
	}
	gcDone := make(chan error, 1)
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		if _, err := media.ConfirmFields(ctx, confirmer, owner, items, media.VisibilityPublic); err != nil {
			return err
		}
		start := time.Now()
		got, err := collector.CollectGarbage(t.Context())
		gcDone <- err
		if err == nil && got.Claimed != 0 {
			t.Errorf("GC 認領了 %d 列，擁有者還沒提交的認領不能被搶", got.Claimed)
		}
		if elapsed := time.Since(start); elapsed > 10*time.Second {
			t.Errorf("GC 被拖住 %v", elapsed)
		}
		return err
	})
	if err != nil || <-gcDone != nil {
		t.Fatalf("擁有者 transaction = %v", err)
	}
	for _, tk := range tokens {
		if claimKind(t, f, tk.ID) != "CONFIRMED" {
			t.Fatalf("token %s 應該是 CONFIRMED 認領", tk.ID)
		}
	}
	assertNoDanglingAssets(t, f)
}

func TestApplyDeletionsIdempotent(t *testing.T) {
	f := setup(t)
	a, _ := confirmed(t, f)
	b, _ := confirmed(t, f)
	if err := markDeletedInTx(f, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	counts := make([]int, 2)
	for i := range counts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := f.module(f.clock).ApplyDeletions(t.Context())
			if err != nil {
				t.Error(err)
			}
			counts[i] = n
		}()
	}
	wg.Wait()
	if counts[0]+counts[1] != 2 {
		t.Fatalf("兩個 worker 共套用 %d 筆，預期 2", counts[0]+counts[1])
	}
	var applied1 time.Time
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT applied_at FROM media.asset_deletion_requests WHERE asset_id = $1`, a.ID).Scan(&applied1); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Hour)
	if n, err := f.m.ApplyDeletions(t.Context()); err != nil || n != 0 {
		t.Fatalf("重跑 = %d, %v，預期 0", n, err)
	}
	var applied2 time.Time
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT applied_at FROM media.asset_deletion_requests WHERE asset_id = $1`, a.ID).Scan(&applied2); err != nil {
		t.Fatal(err)
	}
	if !applied1.Equal(applied2) {
		t.Fatal("applied_at 只寫一次")
	}
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	applyDeletions(t, f)
	if got := assetState(t, f, a.ID); got != "DELETED" {
		t.Fatalf("已 DELETED 的資產 = %s", got)
	}
}

func TestOrphanDeletingInvariantHoldsAndTheCheckIsNotVacuous(t *testing.T) {
	f := setup(t)
	a, _ := confirmed(t, f)
	staged := ready(t, f, owner)
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	applyDeletions(t, f)
	f.clock.Advance(100 * media.UploadTTL)
	if _, err := f.m.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertNoOrphanDeleting(t, f)
	_ = staged
	b, _ := confirmed(t, f)
	if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.assets SET state = 'DELETING' WHERE id = $1`, b.ID); err != nil {
		t.Fatal(err)
	}
	if n := orphanDeleting(t, f); n != 1 {
		t.Fatalf("orphanDeleting = %d，預期抓到 1 個", n)
	}
}

func TestReadsAfterMarkDeletedCommit(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	a, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if dup, err := f.m.CopyPublic(t.Context(), owner, a.ID); err != nil || dup.Token == "" {
		t.Fatalf("確認後 CopyPublic = %+v, %v", dup, err)
	}
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.m.CopyPublic(t.Context(), owner, a.ID)
	wantKind(t, err, apperr.KindNotFound)
	if got := physicalAssetState(t, f, a.ID); got != "STAGED" {
		t.Fatalf("物理 state = %s，job 還沒跑應該仍是 STAGED", got)
	}
}

func TestMarkDeleted_IgnoresAssetsOfReclaimedUploads(t *testing.T) {
	f := setup(t)
	res := ready(t, f, owner)
	assetID := assetIDOf(t, f, res.ID)
	_, final, _ := uploadKeys(t, f, res.ID)
	f.store.setDeleteFailure(*final, errors.New("r2 unavailable"))
	f.clock.Advance(media.UploadTTL + time.Minute)
	if _, err := f.m.CollectGarbage(t.Context()); err == nil {
		t.Fatal("刪物件失敗應該回錯誤")
	}
	if claimKind(t, f, res.ID) != "RECLAIMED" {
		t.Fatal("GC 應該以 RECLAIMED 認領這筆暫存")
	}
	if err := markDeletedInTx(f, assetID); err != nil {
		t.Fatal(err)
	}
	assertDeletionRequests(t, f, 0)
}
