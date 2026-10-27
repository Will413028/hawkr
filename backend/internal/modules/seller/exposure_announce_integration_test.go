//go:build integration

package seller_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
)

// announced 讀這家店最後公告的值；沒有列等同 version 0 的列。
func (f *fixture) announced(t *testing.T, storeID int64) (exposable bool, version int64) {
	t.Helper()
	rows, err := f.db.Conn(context.Background()).Query(context.Background(), "SELECT exposable, version FROM seller.store_exposures WHERE store_id=$1", storeID)
	must(t, err)
	defer rows.Close()
	if rows.Next() {
		must(t, rows.Scan(&exposable, &version))
	}
	must(t, rows.Err())
	return exposable, version
}

// wantAnnouncements 比對這一輪收到的 StoreExposureChanged（依序）：店家、公告的值與版本，並且最後公告的列等於最後一個事件。
func (f *fixture) wantAnnouncements(t *testing.T, got *recorded[StoreExposureChanged], from int, storeID int64, want ...bool) {
	t.Helper()
	all := got.all()
	if len(all) != from+len(want) {
		t.Fatalf("announcements=%+v, want %d new after %d: %v", all, len(want), from, want)
	}
	for i, exposable := range want {
		e := all[from+i]
		if e.StoreID != storeID || e.Exposable != exposable || e.EventID[6]>>4 != 7 || e.OccurredAt.IsZero() {
			t.Errorf("announcement %d=%+v, want store %d exposable=%v with a UUIDv7 event id", from+i, e, storeID, exposable)
		}
	}
	last := all[len(all)-1]
	if exposable, version := f.announced(t, storeID); exposable != last.Exposable || version != last.Version {
		t.Errorf("announced row=(%v,%d), last event=%+v", exposable, version, last)
	}
}

// 可曝光翻轉的寫入在它自己的 transaction 內公告，版本每次加一（ADR 2026-10-05-store-exposure-to-legacy 決策 1）。四類寫入路徑：
// profile（logo、鄉鎮）、停權、KYC 與 PayUni 的結果、狀態（等待刪除）。
func TestExposureFlipsAreAnnouncedByEveryKindOfWrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	accountID, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	if got := announcements.all(); len(got) != 0 {
		t.Fatalf("PayUni 核准但還沒有 logo，不可曝光：%+v", got)
	}

	f.logoed(t, accountID, s)
	f.wantAnnouncements(t, announcements, 0, s.StoreID, true)
	if _, version := f.announced(t, s.StoreID); version != 1 {
		t.Fatalf("第一次公告的版本是 %d，want 1", version)
	}

	n := len(announcements.all())
	rec := f.suspend(t, s.StoreID)
	f.wantAnnouncements(t, announcements, n, s.StoreID, false)
	n = len(announcements.all())
	_, err := f.lift(ctx, rec)
	must(t, err)
	f.wantAnnouncements(t, announcements, n, s.StoreID, true)

	n = len(announcements.all())
	f.now = f.now.Add(time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourceRefresh))
	f.wantAnnouncements(t, announcements, n, s.StoreID, false)
	n = len(announcements.all())
	f.now = f.now.Add(time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourceRefresh))
	f.wantAnnouncements(t, announcements, n, s.StoreID, true)

	n = len(announcements.all())
	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	// 有 PayUni 註冊的店家不能經 UpdateProfile 清掉地址：直接清（代表地址不合法的歷史資料），再以別的欄位的寫入觸發公告。
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET town_code = NULL, address_detail = NULL WHERE store_id = $1", s.StoreID)
	must(t, err)
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, Prize: ptr("優良農民")})
	must(t, err)
	f.wantAnnouncements(t, announcements, n, s.StoreID, false)
	n = len(announcements.all())
	cur, err = f.module.Store(ctx, s.StoreID)
	must(t, err)
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, TownCode: ptr("63000050"), AddressDetail: ptr("測試路一號")})
	must(t, err)
	f.wantAnnouncements(t, announcements, n, s.StoreID, true)

	n = len(announcements.all())
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, s.StoreID) }))
	f.wantAnnouncements(t, announcements, n, s.StoreID, false)
	versions := make([]int64, 0)
	for _, e := range announcements.all() {
		versions = append(versions, e.Version)
	}
	for i, v := range versions {
		if v != int64(i+1) {
			t.Fatalf("versions=%v, want 1..n in order", versions)
		}
	}
}

func TestClosingAnExposableStoreAnnouncesNotExposable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	accountID, s := f.exposable(t)
	n := len(announcements.all())
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	f.wantAnnouncements(t, announcements, n, s.StoreID, false)
}

func TestMeasureAnnouncesWhenItStartsAndWhenItIsRevoked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	_, s := f.exposable(t)
	n := len(announcements.all())
	ends := f.now.Add(24 * time.Hour)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	f.wantAnnouncements(t, announcements, n, s.StoreID, false)
	n = len(announcements.all())
	_, err := f.module.RevokeMeasure(ctx, RevokeMeasureInput{MeasureID: v.ID, ExpectedVersion: v.Version, AdminID: 7, Reason: RevokeAdminCorrection})
	must(t, err)
	f.wantAnnouncements(t, announcements, n, s.StoreID, true)
}

// 沒有翻轉的寫入不公告（版本不動，也不發事件）：不影響六個條件的 profile 修改、完成新手引導、KYC 的送出與取消，以及
// 不可曝光的店家上的任何寫入。
func TestWritesThatDoNotFlipExposureAnnounceNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	_, s := f.exposable(t)
	_, other, sub := f.submitted(t, idB, bankAcct)
	n := len(announcements.all())
	_, versionBefore := f.announced(t, s.StoreID)

	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	cur, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, Prize: ptr("新的特色")})
	must(t, err)
	_, err = f.module.CompleteOnboarding(ctx, s.StoreID, cur.Version)
	must(t, err)
	cancelled, err := f.module.CancelKYC(ctx, CancelKYCInput{AccountID: other.AccountID, ExpectedVersion: sub.Version})
	must(t, err)
	if cancelled.Status != KYCStatusCancelled {
		t.Fatalf("cancelled=%+v", cancelled)
	}
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: other.StoreID, ExpectedVersion: other.Version, Prize: ptr("尚未可曝光")})
	must(t, err)

	if got := announcements.all(); len(got) != n {
		t.Fatalf("no flip, yet announced: %+v", got[n:])
	}
	if exposable, version := f.announced(t, s.StoreID); !exposable || version != versionBefore {
		t.Fatalf("announced=(%v,%d), want unchanged (true,%d)", exposable, version, versionBefore)
	}
	if exposable, version := f.announced(t, other.StoreID); exposable || version != 0 {
		t.Fatalf("never exposable store announced=(%v,%d)", exposable, version)
	}
}

// 公告的事件在寫入的 transaction 內發出：訂閱者失敗時整筆寫入回滾，最後公告的值也不動。
func TestAnExposureEventFailureRollsBackTheWriteAndTheAnnouncement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	boom := errors.New("subscriber failed")
	events.Subscribe(f.bus, func(context.Context, StoreExposureChanged) error { return boom })
	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, LogoToken: "logo", UploaderID: accountID})
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	after, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if after.LogoKey != nil || after.Version != cur.Version {
		t.Fatalf("the logo write must roll back: %+v", after)
	}
	if exposable, version := f.announced(t, s.StoreID); exposable || version != 0 {
		t.Fatalf("announced=(%v,%d)", exposable, version)
	}
}

// 處分到期不經任何寫入（沒有跑 measure_expiry、也沒有跑 reconcile_suspensions）：只有對帳補發 true；再跑一次不重發。
func TestReconcileAnnouncesAnExpiryThatNoWriteObserved(t *testing.T) {
	t.Parallel()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	_, s := f.exposable(t)
	ends := f.now.Add(time.Hour)
	f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	n := len(announcements.all())
	if exposable, version := f.announced(t, s.StoreID); exposable || version != 2 {
		t.Fatalf("the sanction must announce false first: (%v,%d)", exposable, version)
	}

	f.now = ends.Add(time.Hour)
	if f.isSuspended(t, s.StoreID) {
		t.Fatal("the measure expired by the clock")
	}
	f.reconcile(t)
	f.wantAnnouncements(t, announcements, n, s.StoreID, true)
	if e := announcements.all()[n]; !e.OccurredAt.Equal(f.now) || e.Version != 3 {
		t.Fatalf("event=%+v, want version 3 at the reconcile time %v", e, f.now)
	}

	f.reconcile(t)
	if got := announcements.all(); len(got) != n+1 {
		t.Fatalf("the second run must not announce again: %+v", got[n+1:])
	}
}

// 對帳的候選：所有未關閉的店，加上最後公告為可曝光的店（關店之後要補發 false）。漏接的寫入由對帳補上，兩個方向都一樣。
func TestReconcileCoversStoresWhoseAnnouncementDisagreesWithTheRule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	_, live := f.exposable(t)
	closer, closed := f.exposable(t)
	_, err := f.module.CloseCurrent(ctx, closer)
	must(t, err)
	_, never := f.open(t)
	n := len(announcements.all())

	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE seller.store_exposures SET exposable=false, version=version+3, exposable_changed_at=$2 WHERE store_id=$1", live.StoreID, f.now)
	must(t, err)
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE seller.store_exposures SET exposable=true, version=version+3, exposable_changed_at=$2 WHERE store_id=$1", closed.StoreID, f.now)
	must(t, err)
	if scanned := f.reconcile(t); scanned != 3 {
		t.Fatalf("scanned=%d, want the live, the closed-but-announced and the never-exposed store", scanned)
	}
	got := announcements.all()[n:]
	if len(got) != 2 {
		t.Fatalf("announcements=%+v", got)
	}
	byStore := map[int64]StoreExposureChanged{}
	for _, e := range got {
		byStore[e.StoreID] = e
	}
	if e := byStore[live.StoreID]; !e.Exposable || e.Version != 5 {
		t.Errorf("live=%+v, want exposable at version 5 (1 announced, then 3 skipped by the corruption, then this)", e)
	}
	if e := byStore[closed.StoreID]; e.Exposable || e.Version != 6 {
		t.Errorf("closed=%+v, want not exposable at version 6", e)
	}
	if exposable, version := f.announced(t, never.StoreID); exposable || version != 0 {
		t.Errorf("the never exposable store announced (%v,%d)", exposable, version)
	}
	if scanned := f.reconcile(t); scanned != 2 || len(announcements.all()) != n+2 {
		t.Fatalf("second run scanned=%d announcements=%d", scanned, len(announcements.all())-n)
	}
}

// 縣市篩選讀公告表，不是現算的規則：公告的值改了，結果就跟著改（Java 的過濾依同一份公告）。
func TestExposableStoreIDsByCountyReadsTheAnnouncedValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, exposable := f.exposable(t)
	_, notExposable := f.open(t)
	list := func() []int64 {
		got, err := f.module.ExposableStoreIDsByCounty(ctx, []string{"63000"})
		must(t, err)
		return got["63000"]
	}
	if got := list(); !slices.Equal(got, []int64{exposable.StoreID}) {
		t.Fatalf("got=%v, want only the announced store", got)
	}
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.store_exposures SET exposable=false, version=version+1, exposable_changed_at=$2 WHERE store_id=$1", exposable.StoreID, f.now)
	must(t, err)
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE seller.store_exposures SET exposable=true, version=1, exposable_changed_at=$2 WHERE store_id=$1", notExposable.StoreID, f.now)
	must(t, err)
	if got := list(); !slices.Equal(got, []int64{notExposable.StoreID}) {
		t.Fatalf("got=%v, want the announced value to decide, not the live rule", got)
	}
	if _, err := f.module.ExposableStoreIDsByCounty(ctx, []string{"00000"}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("unknown county: %v", err)
	}
}

// 先鎖再算：對帳（只讀、不寫店家列）與另一個寫入同時進行時，對帳卡在公告列的鎖上；拿到鎖之後算的是寫入 commit 之後的
// 結果。先算再鎖的話，對帳會以寫入 commit 之前的輸入（還有 logo）覆蓋寫入剛公告的 false。
// 靠 pg_stat_activity 確認對帳真的等在鎖上才放開寫入，不依賴時間。
func TestAnnouncementLocksBeforeItComputes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	_, s := f.exposable(t)
	n := len(announcements.all())

	announcedByWriter := make(chan struct{})
	release := make(chan struct{})
	writer := make(chan error, 1)
	go func() {
		writer <- f.db.InTx(ctx, func(ctx context.Context) error {
			if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET logo_key=NULL, logo_asset_id=NULL WHERE store_id=$1", s.StoreID); err != nil {
				return err
			}
			if err := f.exposures().Announce(ctx, store.New(f.db.Conn(ctx)), f.now, s.StoreID); err != nil {
				return err
			}
			close(announcedByWriter)
			<-release
			return nil
		})
	}()
	select {
	case <-announcedByWriter:
	case err := <-writer:
		t.Fatalf("writer ended before announcing: %v", err)
	}

	reconciler := make(chan error, 1)
	go func() {
		_, err := f.module.ReconcileExposure(ctx)
		reconciler <- err
	}()
	f.waitForLockWaiters(t, 1)
	close(release)
	must(t, <-writer)
	must(t, <-reconciler)

	f.wantAnnouncements(t, announcements, n, s.StoreID, false)
	if exposable, version := f.announced(t, s.StoreID); exposable || version != 2 {
		t.Fatalf("announced=(%v,%d): the reconcile overwrote the writer's announcement with stale input", exposable, version)
	}
}

// 到期 job 以 max(現在, ends_at) 重算停權期間（worker 的時鐘落後時也能結束剛到期的處分）：同一個 transaction 的公告用同一個
// 時間點判斷停權，所以 StoreUnsuspended 與 StoreExposureChanged(true) 一起發出，不必等對帳。
func TestExpiryJobAnnouncesWithTheInstantItRecomputedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	unsuspended := record[StoreUnsuspended](f)
	_, s := f.exposable(t)
	ends := f.now.Add(time.Minute)
	v := f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	n := len(announcements.all())

	must(t, f.module.ExpireMeasure(ctx, v.ID, ends))
	if got := unsuspended.all(); len(got) != 1 {
		t.Fatalf("StoreUnsuspended=%+v", got)
	}
	f.wantAnnouncements(t, announcements, n, s.StoreID, true)
}

// 開店時公告列與店家列同一個 transaction 建立（version 0：還沒公告過），第一次公告的版本是 1。
func TestANewStoreHasItsExposureRowAndTheFirstAnnouncementIsVersionOne(t *testing.T) {
	t.Parallel()
	f := setup(t)
	announcements := record[StoreExposureChanged](f)
	_, opened := f.open(t)
	const neverAnnounced = "SELECT count(*) FROM seller.store_exposures WHERE store_id=$1 AND version=0 AND NOT exposable AND exposable_changed_at IS NULL"
	if got := f.count(t, neverAnnounced, opened.StoreID); got != 1 {
		t.Fatalf("exposure rows for the new store=%d, want exactly the never-announced row", got)
	}
	if got := announcements.all(); len(got) != 0 {
		t.Fatalf("opening a store must not announce: %+v", got)
	}
	_, s := f.exposable(t)
	all := announcements.all()
	if len(all) != 1 || all[0].StoreID != s.StoreID || !all[0].Exposable || all[0].Version != 1 {
		t.Fatalf("announcements=%+v, want one exposable announcement at version 1 for store %d", all, s.StoreID)
	}
}
