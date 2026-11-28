//go:build integration

package media_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const holdSource = "test.evidence"

func holdInTx(f *fixture, assetID uuid.UUID, sourceID uuid.UUID, until time.Time) error {
	return f.d.InTx(f.t.Context(), func(ctx context.Context) error {
		return f.m.Hold(ctx, []uuid.UUID{assetID}, holdSource, sourceID, until)
	})
}

func releaseInTx(f *fixture, sourceID uuid.UUID) error {
	return f.d.InTx(f.t.Context(), func(ctx context.Context) error {
		return f.m.ReleaseHold(ctx, holdSource, sourceID)
	})
}

func rawHold(t *testing.T, f *fixture, assetID uuid.UUID) {
	t.Helper()
	_, err := f.d.Pool().Exec(t.Context(),
		`INSERT INTO media.asset_holds (asset_id, source, source_id, until, created_at) VALUES ($1, 'test.raw', $2, $3, $4)`,
		assetID, uuid.New(), f.clock.Now().Add(24*time.Hour), f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
}

func secureConfirmed(t *testing.T, f *fixture) (media.ConfirmedAsset, []string) {
	t.Helper()
	res := uploadSecure(t, f, owner, pdfOf(3000))
	a := confirmSecure(t, f, owner, res.Token)
	keys := chunkKeys(t, f, a.ID)
	if len(keys) == 0 {
		t.Fatal("機敏資產沒有任何段")
	}
	return a, keys
}

func requestApplied(t *testing.T, f *fixture, assetID uuid.UUID) bool {
	t.Helper()
	return count(t, f, `SELECT count(*) FROM media.asset_deletion_requests WHERE asset_id = $1 AND applied_at IS NOT NULL`, assetID) == 1
}

func hasAll(f *fixture, keys []string) bool {
	for _, k := range keys {
		if !f.store.has(k) {
			return false
		}
	}
	return true
}

func holdsOf(t *testing.T, f *fixture, assetID uuid.UUID) int {
	t.Helper()
	return count(t, f, `SELECT count(*) FROM media.asset_holds WHERE asset_id = $1`, assetID)
}

func deleteAll(t *testing.T, f *fixture) {
	t.Helper()
	applyDeletions(t, f)
	if _, err := f.m.DeleteAssets(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestApplyDeletionsSkipsHeldAsset(t *testing.T) {
	f := setup(t)
	a, keys := secureConfirmed(t, f)
	if err := holdInTx(f, a.ID, uuid.New(), f.clock.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	deleteAll(t, f)
	if got := physicalAssetState(t, f, a.ID); got != "STAGED" {
		t.Fatalf("被保全的資產物理狀態 = %s，預期 STAGED", got)
	}
	if !hasAll(f, keys) {
		t.Fatal("被保全的資產物件被刪了")
	}
	if requestApplied(t, f, a.ID) {
		t.Fatal("被保全資產的刪除請求不該被標為已套用")
	}
	assertNoOrphanDeleting(t, f)
}

func TestExpiredHoldAllowsDeletion(t *testing.T) {
	f := setup(t)
	a, keys := secureConfirmed(t, f)
	if err := holdInTx(f, a.ID, uuid.New(), f.clock.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	deleteAll(t, f)
	if physicalAssetState(t, f, a.ID) != "STAGED" || !hasAll(f, keys) {
		t.Fatal("保全期間內資產要維持原狀")
	}
	f.clock.Advance(2 * time.Hour)
	deleteAll(t, f)
	if got := physicalAssetState(t, f, a.ID); got != "DELETED" {
		t.Fatalf("保全到期後資產物理狀態 = %s，預期 DELETED", got)
	}
	for _, k := range keys {
		if f.store.has(k) {
			t.Fatalf("保全到期後物件 %s 還在", k)
		}
	}
}

func TestClaimAssetsOfUploadsSkipsHeldAsset(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, pdfOf(3000))
	_, assetID := secureRow(t, f, res.Token)
	rawHold(t, f, assetID)
	n, err := store.New(f.d.Pool()).ClaimAssetsOfUploads(t.Context(), store.ClaimAssetsOfUploadsParams{Ids: []uuid.UUID{assetID}, Now: f.clock.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("ClaimAssetsOfUploads 認領了 %d 個被保全的資產", n)
	}
	if got := physicalAssetState(t, f, assetID); got != "STAGED" {
		t.Fatalf("被保全的資產物理狀態 = %s，預期 STAGED", got)
	}
	second := uploadSecure(t, f, owner, pdfOf(3000))
	_, freeID := secureRow(t, f, second.Token)
	n, err = store.New(f.d.Pool()).ClaimAssetsOfUploads(t.Context(), store.ClaimAssetsOfUploadsParams{Ids: []uuid.UUID{freeID}, Now: f.clock.Now()})
	if err != nil || n != 1 {
		t.Fatalf("沒有保全的資產要被認領：n=%d err=%v", n, err)
	}
}

func TestReclaimUploadWithHeldAssetKeepsUploadStateAndRollsBackClaim(t *testing.T) {
	f := setup(t)
	res := uploadSecure(t, f, owner, pdfOf(3000))
	uploadID, assetID := secureRow(t, f, res.Token)
	keys := chunkKeys(t, f, assetID)
	rawHold(t, f, assetID)
	f.clock.Advance(media.UploadTTL + time.Minute)
	gc, err := f.m.CollectGarbage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if gc.Claimed != 0 || gc.Deleted != 0 {
		t.Fatalf("GC 結果 = %+v，被保全的暫存不該被認領或刪除", gc)
	}
	if got := uploadState(t, f, uploadID); got != "READY" {
		t.Fatalf("暫存列狀態 = %s，預期維持 READY", got)
	}
	if kind := claimKind(t, f, uploadID); kind != "" {
		t.Fatalf("回滾後不該留下認領列，得到 %s", kind)
	}
	if got := physicalAssetState(t, f, assetID); got != "STAGED" || !hasAll(f, keys) {
		t.Fatalf("資產狀態 = %s，物件在 = %v，預期 STAGED 且物件都在", got, hasAll(f, keys))
	}
	if n := count(t, f, `SELECT count(*) FROM media.staged_uploads WHERE state = 'DELETING'`); n != 0 {
		t.Fatalf("有 %d 筆暫存列留在 DELETING", n)
	}
}

func TestDeleteUploadRechecksHoldBeforeDeletingObjects(t *testing.T) {
	setDeleting := func(t *testing.T, f *fixture, uploadID, assetID uuid.UUID) {
		t.Helper()
		if _, err := f.d.Pool().Exec(t.Context(),
			`INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at) VALUES ($1, 'RECLAIMED', $2, $3)`, uploadID, assetID, f.clock.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := f.d.Pool().Exec(t.Context(), `UPDATE media.staged_uploads SET state = 'DELETING' WHERE id = $1`, uploadID); err != nil {
			t.Fatal(err)
		}
		rawHold(t, f, assetID)
	}
	check := func(t *testing.T, f *fixture, uploadID, assetID uuid.UUID, keys []string) {
		t.Helper()
		if _, err := f.m.DeleteAssets(t.Context()); err != nil {
			t.Fatal(err)
		}
		if !hasAll(f, keys) {
			t.Fatal("被保全的暫存對應的物件被刪了")
		}
		if got := uploadState(t, f, uploadID); got != "READY" {
			t.Fatalf("暫存列狀態 = %s，預期退回 READY", got)
		}
		if kind := claimKind(t, f, uploadID); kind != "" {
			t.Fatalf("退回後認領列要被刪，得到 %s", kind)
		}
		if got := physicalAssetState(t, f, assetID); got != "STAGED" {
			t.Fatalf("資產狀態 = %s，預期 STAGED", got)
		}
	}
	t.Run("公開 final_key", func(t *testing.T) {
		f := setup(t)
		res := ready(t, f, owner)
		_, final, assetID := uploadKeys(t, f, res.ID)
		setDeleting(t, f, res.ID, *assetID)
		check(t, f, res.ID, *assetID, []string{*final})
	})
	t.Run("SECURE planned_asset_id", func(t *testing.T) {
		f := setup(t)
		tok := uploadSecure(t, f, owner, pdfOf(3000))
		uploadID, assetID := secureRow(t, f, tok.Token)
		keys := chunkKeys(t, f, assetID)
		setDeleting(t, f, uploadID, assetID)
		check(t, f, uploadID, assetID, keys)
	})
}

func TestDeleteAssetRechecksHoldBeforeDeletingObject(t *testing.T) {
	run := func(t *testing.T, f *fixture, a media.ConfirmedAsset, keys []string) {
		t.Helper()
		if err := markDeletedInTx(f, a.ID); err != nil {
			t.Fatal(err)
		}
		applyDeletions(t, f)
		if got := physicalAssetState(t, f, a.ID); got != "DELETING" {
			t.Fatalf("Apply 之後資產物理狀態 = %s，預期 DELETING", got)
		}
		rawHold(t, f, a.ID)
		if _, err := f.m.DeleteAssets(t.Context()); err != nil {
			t.Fatal(err)
		}
		if !hasAll(f, keys) {
			t.Fatal("Apply 之後才登記保全，物件仍被刪了")
		}
		if got := physicalAssetState(t, f, a.ID); got != "STAGED" {
			t.Fatalf("資產物理狀態 = %s，預期退回 STAGED", got)
		}
		if requestApplied(t, f, a.ID) {
			t.Fatal("退回後刪除請求的 applied_at 要清回 NULL")
		}
		assertNoOrphanDeleting(t, f)
	}
	t.Run("公開", func(t *testing.T) {
		f := setup(t)
		a, final := confirmed(t, f)
		run(t, f, a, []string{final})
	})
	t.Run("SECURE", func(t *testing.T) {
		f := setup(t)
		a, keys := secureConfirmed(t, f)
		run(t, f, a, keys)
	})
}

func TestHoldOnDeletingAssetReturnsAssetGone(t *testing.T) {
	f := setup(t)
	a, _ := secureConfirmed(t, f)
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	applyDeletions(t, f)
	err := holdInTx(f, a.ID, uuid.New(), f.clock.Now().Add(time.Hour))
	if !errors.Is(err, media.ErrAssetGone) {
		t.Fatalf("Hold 對刪除中的資產 = %v，預期 ErrAssetGone", err)
	}
	if n := holdsOf(t, f, a.ID); n != 0 {
		t.Fatalf("ErrAssetGone 之後不該留下保全列，有 %d 筆", n)
	}
	missing := holdInTx(f, uuid.New(), uuid.New(), f.clock.Now().Add(time.Hour))
	if !errors.Is(missing, media.ErrAssetNotFound) {
		t.Fatalf("Hold 對不存在的資產 = %v，預期 ErrAssetNotFound", missing)
	}
}

func TestTwoHoldSourcesStackAndReleasingOneKeepsAsset(t *testing.T) {
	f := setup(t)
	a, keys := secureConfirmed(t, f)
	first, second := uuid.New(), uuid.New()
	until := f.clock.Now().Add(24 * time.Hour)
	if err := holdInTx(f, a.ID, first, until); err != nil {
		t.Fatal(err)
	}
	if err := holdInTx(f, a.ID, second, until); err != nil {
		t.Fatal(err)
	}
	if err := markDeletedInTx(f, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := releaseInTx(f, first); err != nil {
		t.Fatal(err)
	}
	deleteAll(t, f)
	if physicalAssetState(t, f, a.ID) != "STAGED" || !hasAll(f, keys) {
		t.Fatal("還剩一個來源的保全，資產不該被刪")
	}
	if err := releaseInTx(f, second); err != nil {
		t.Fatal(err)
	}
	if err := releaseInTx(f, second); err != nil {
		t.Fatalf("ReleaseHold 要冪等：%v", err)
	}
	deleteAll(t, f)
	if got := physicalAssetState(t, f, a.ID); got != "DELETED" {
		t.Fatalf("兩個來源都解除後資產物理狀態 = %s，預期 DELETED", got)
	}
}

func TestExtendHoldOnlyExtendsAndExpireHoldsRemovesOnlyExpiredRows(t *testing.T) {
	f := setup(t)
	a, _ := secureConfirmed(t, f)
	b, _ := secureConfirmed(t, f)
	src := uuid.New()
	start := f.clock.Now()
	if err := holdInTx(f, a.ID, src, start.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := holdInTx(f, b.ID, uuid.New(), start.Add(10*time.Hour)); err != nil {
		t.Fatal(err)
	}
	extend := func(until time.Time) error {
		return f.d.InTx(t.Context(), func(ctx context.Context) error { return f.m.ExtendHold(ctx, holdSource, src, until) })
	}
	if err := extend(start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(90 * time.Minute)
	if n, err := f.m.ExpireHolds(t.Context()); err != nil || n != 0 {
		t.Fatalf("ExtendHold 不得縮短期限：刪了 %d 筆，err=%v", n, err)
	}
	if err := extend(start.Add(150 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(70 * time.Minute)
	if n, err := f.m.ExpireHolds(t.Context()); err != nil || n != 1 {
		t.Fatalf("ExpireHolds 刪了 %d 筆（預期 1），err=%v", n, err)
	}
	if holdsOf(t, f, a.ID) != 0 || holdsOf(t, f, b.ID) != 1 {
		t.Fatal("只該刪掉到期的那一筆")
	}
	missing := f.d.InTx(t.Context(), func(ctx context.Context) error {
		return f.m.ExtendHold(ctx, holdSource, uuid.New(), f.clock.Now().Add(time.Hour))
	})
	if !errors.Is(missing, media.ErrHoldNotFound) {
		t.Fatalf("ExtendHold 找不到來源 = %v，預期 ErrHoldNotFound", missing)
	}
}

func TestHoldEntryPointsRequireATransaction(t *testing.T) {
	f := setup(t)
	a, _ := secureConfirmed(t, f)
	until := f.clock.Now().Add(time.Hour)
	if err := f.m.Hold(t.Context(), []uuid.UUID{a.ID}, holdSource, uuid.New(), until); !errors.Is(err, media.ErrNoTransaction) {
		t.Fatalf("Hold 在 transaction 外 = %v", err)
	}
	if err := f.m.ExtendHold(t.Context(), holdSource, uuid.New(), until); !errors.Is(err, media.ErrNoTransaction) {
		t.Fatalf("ExtendHold 在 transaction 外 = %v", err)
	}
	if err := f.m.ReleaseHold(t.Context(), holdSource, uuid.New()); !errors.Is(err, media.ErrNoTransaction) {
		t.Fatalf("ReleaseHold 在 transaction 外 = %v", err)
	}
	if _, err := f.m.CopyToSecure(t.Context(), owner, a.ID); !errors.Is(err, media.ErrNoTransaction) {
		t.Fatalf("CopyToSecure 在 transaction 外 = %v", err)
	}
}

func publicSource(t *testing.T, f *fixture) (media.ConfirmedAsset, []byte) {
	t.Helper()
	res, err := f.m.PutPublic(t.Context(), owner, bytes.NewReader(pngBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := confirmInTx(f, owner, res.Token)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := f.store.Get(t.Context(), src.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	return src, readAll(t, rc)
}

func requestCopy(t *testing.T, f *fixture, sourceID uuid.UUID, holdUntil time.Time) uuid.UUID {
	t.Helper()
	var copyID uuid.UUID
	err := f.d.InTx(t.Context(), func(ctx context.Context) error {
		var err error
		if copyID, err = f.m.CopyToSecure(ctx, owner, sourceID); err != nil {
			return err
		}
		return f.m.Hold(ctx, []uuid.UUID{copyID}, holdSource, uuid.New(), holdUntil)
	})
	if err != nil {
		t.Fatalf("CopyToSecure：%v", err)
	}
	return copyID
}

func TestCopyToSecureProducesIndependentSecureCopy(t *testing.T) {
	f := setup(t)
	src, want := publicSource(t, f)
	copyID := requestCopy(t, f, src.ID, f.clock.Now().Add(24*time.Hour))
	if _, _, err := f.m.ReadSecure(t.Context(), copyID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("複製完成前讀副本 = %v，預期 NotFound", err)
	}
	if n, err := f.m.ProcessCopies(t.Context()); err != nil || n != 1 {
		t.Fatalf("ProcessCopies = %d, %v", n, err)
	}
	rc, ct, err := f.m.ReadSecure(t.Context(), copyID)
	if err != nil {
		t.Fatalf("ReadSecure 副本：%v", err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, want) || ct != src.ContentType {
		t.Fatalf("副本內容或型別與來源不同：%d 位元組、型別 %s", len(got), ct)
	}
	if physicalAssetState(t, f, copyID) != "STAGED" || assetState(t, f, copyID) != "ACTIVE" {
		t.Fatalf("副本狀態 = %s／%s", physicalAssetState(t, f, copyID), assetState(t, f, copyID))
	}
	var visibility string
	if err := f.d.Pool().QueryRow(t.Context(), `SELECT visibility FROM media.assets WHERE id = $1`, copyID).Scan(&visibility); err != nil || visibility != "SECURE" {
		t.Fatalf("副本可見性 = %q，err=%v", visibility, err)
	}
	if err := markDeletedInTx(f, src.ID); err != nil {
		t.Fatal(err)
	}
	deleteAll(t, f)
	if f.store.has(src.ObjectKey) {
		t.Fatal("來源原圖照一般流程刪除，物件不該還在")
	}
	rc, _, err = f.m.ReadSecure(t.Context(), copyID)
	if err != nil {
		t.Fatalf("原圖刪除後讀副本：%v", err)
	}
	if got := readAll(t, rc); !bytes.Equal(got, want) {
		t.Fatal("原圖刪除後副本內容改變")
	}
	if holdsOf(t, f, copyID) != 1 {
		t.Fatal("擁有者登記在副本上的保全要還在")
	}
	if calls := f.store.callsInTx(); len(calls) != 0 {
		t.Fatalf("這些儲存呼叫的 ctx 帶著 transaction：%v", calls)
	}
}

func TestCopyToSecureKeepsOriginalUntilCopyCompletes(t *testing.T) {
	f := setup(t)
	src, _ := publicSource(t, f)
	copyID := requestCopy(t, f, src.ID, f.clock.Now().Add(24*time.Hour))
	if err := markDeletedInTx(f, src.ID); err != nil {
		t.Fatal(err)
	}
	deleteAll(t, f)
	if got := physicalAssetState(t, f, src.ID); got != "STAGED" || !f.store.has(src.ObjectKey) {
		t.Fatalf("複製完成前原圖狀態 = %s、物件在 = %v，預期 STAGED 且物件在", got, f.store.has(src.ObjectKey))
	}
	if n, err := f.m.ProcessCopies(t.Context()); err != nil || n != 1 {
		t.Fatalf("ProcessCopies = %d, %v", n, err)
	}
	if n := holdsOf(t, f, src.ID); n != 0 {
		t.Fatalf("複製完成後原圖上的待複製保全要釋放，還有 %d 筆", n)
	}
	deleteAll(t, f)
	if got := physicalAssetState(t, f, src.ID); got != "DELETED" || f.store.has(src.ObjectKey) {
		t.Fatalf("複製完成後原圖狀態 = %s、物件在 = %v，預期 DELETED 且物件已刪", got, f.store.has(src.ObjectKey))
	}
	if _, _, err := f.m.ReadSecure(t.Context(), copyID); err != nil {
		t.Fatalf("副本讀不到：%v", err)
	}
}

func TestCopyToSecureAbandonsTheRequestWhenTheOriginalIsGone(t *testing.T) {
	f := setup(t)
	src, _ := publicSource(t, f)
	copyID := requestCopy(t, f, src.ID, f.clock.Now().Add(24*time.Hour))
	f.clock.Advance(media.CopyHoldTTL + time.Minute)
	if err := markDeletedInTx(f, src.ID); err != nil {
		t.Fatal(err)
	}
	deleteAll(t, f)
	if physicalAssetState(t, f, src.ID) != "DELETED" {
		t.Fatal("待複製保全過期後原圖照一般流程刪除")
	}
	if n, err := f.m.ProcessCopies(t.Context()); err != nil || n != 1 {
		t.Fatalf("ProcessCopies = %d, %v", n, err)
	}
	if _, _, err := f.m.ReadSecure(t.Context(), copyID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("放棄的副本讀取 = %v，預期 NotFound", err)
	}
	if n := count(t, f, `SELECT count(*) FROM media.asset_copy_requests WHERE asset_id = $1 AND failed_at IS NOT NULL`, copyID); n != 1 {
		t.Fatal("複製請求要標為失敗")
	}
}

func TestCopyToSecureRefusesNonPublicOrUnconfirmedSources(t *testing.T) {
	f := setup(t)
	secureAsset, _ := secureConfirmed(t, f)
	res := ready(t, f, owner)
	_, _, unconfirmed := uploadKeys(t, f, res.ID)
	for name, id := range map[string]uuid.UUID{"機敏": secureAsset.ID, "未確認": *unconfirmed, "不存在": uuid.New()} {
		err := f.d.InTx(t.Context(), func(ctx context.Context) error {
			_, err := f.m.CopyToSecure(ctx, owner, id)
			return err
		})
		if apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s來源 = %v，預期 NotFound", name, err)
		}
	}
	if n := count(t, f, `SELECT count(*) FROM media.asset_copy_requests`); n != 0 {
		t.Fatalf("被拒的請求留下 %d 筆複製請求", n)
	}
}
