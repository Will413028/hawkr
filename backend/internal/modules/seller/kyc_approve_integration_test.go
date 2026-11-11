//go:build integration

package seller_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// approvedStore 建立一間已核准的店家（申請送出並核准；收款帳戶由 payment 的 fake 另外建立）。
func (f *fixture) approvedStore(t *testing.T) (int64, Store) {
	t.Helper()
	accountID, s, sub := f.submitted(t, idA, bankAcct)
	must(t, approve(f, sub))
	return accountID, s
}

func (q *queue) count(kind string) int {
	n := 0
	for _, k := range q.kinds() {
		if k == kind {
			n++
		}
	}
	return n
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func subscribeFail[E any](f *fixture, err error) {
	events.Subscribe(f.bus, func(context.Context, E) error { return err })
}

// 不推斷重送：核准、退回生效後帶舊版本再送一次一律 VersionConflict（先於狀態檢查），不重複寫審核紀錄、不重發事件、不動申請。
func TestStaleEtagAfterApproveAndRejectConflicts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	approved, rejected := record[KYCApproved](f), record[KYCRejected](f)
	_, _, sub := f.submitted(t, idA, bankAcct)
	first, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9})
	must(t, err)
	for _, v := range []int64{sub.Version, sub.Version - 1} {
		if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: v, AdminID: 9}); apperr.KindOf(err) != apperr.KindVersionConflict {
			t.Fatalf("approve version %d: %v", v, err)
		}
	}
	if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub.ID, ExpectedVersion: sub.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不符"}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("reject after approve: %v", err)
	}
	// 帶目前的版本再核准：申請已不是審核中。
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub.ID, ExpectedVersion: first.Version, AdminID: 9}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("approve at the current version: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action='APPROVE'"); n != 1 || len(approved.all()) != 1 {
		t.Fatalf("reviews=%d events=%d", n, len(approved.all()))
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE application_id=$1 AND version=$2 AND status='APPROVED'", sub.ID, first.Version); n != 1 {
		t.Fatalf("application changed after the stale requests")
	}

	// 退回生效後帶退回之前的版本：衝突，審核紀錄與事件各只有一筆。
	_, _, sub2 := f.submitted(t, idB, "012345678902")
	done, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub2.ID, ExpectedVersion: sub2.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不符"})
	must(t, err)
	if _, err := f.module.RejectKYC(ctx, RejectKYCInput{ApplicationID: sub2.ID, ExpectedVersion: sub2.Version, AdminID: 9, Issues: anIssue(), Reason: "資料不符"}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("reject retry: %v", err)
	}
	if _, err := f.module.ApproveKYC(ctx, ApproveKYCInput{ApplicationID: sub2.ID, ExpectedVersion: sub2.Version, AdminID: 9}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("approve after reject: %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE action='REJECT'"); n != 1 || len(rejected.all()) != 1 || f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE application_id=$1 AND version=$2 AND status='REJECTED'", sub2.ID, done.Version) != 1 {
		t.Fatalf("reviews=%d events=%d", n, len(rejected.all()))
	}
}

// 核准的寫入在同一個 transaction：核准失敗（事件的訂閱者失敗）時，申請維持審核中，收款帳戶的「建立中」列也不存在。
func TestApproveIsAtomicWithTheAccountRequest(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s, sub := f.submitted(t, idB, bankAcct)
	boom := errors.New("boom")
	subscribeFail[KYCApproved](f, boom)
	if err := approve(f, sub); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM payment.accounts WHERE store_id=$1", s.StoreID); n != 0 {
		t.Fatalf("payment accounts=%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_applications WHERE store_id=$1 AND status='SUBMITTED'", s.StoreID); n != 1 {
		t.Fatalf("application is no longer SUBMITTED")
	}
}
