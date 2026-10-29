//go:build integration

package identity_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func (f *fixture) suspendedAt(t *testing.T, id int64) bool {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM identity.accounts WHERE id = $1 AND suspended_at IS NOT NULL`, id) == 1
}

func (f *fixture) suspensionRows(t *testing.T, id int64) (all, unreleased int) {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM identity.account_suspensions WHERE account_id = $1`, id),
		f.count(t, `SELECT count(*) FROM identity.account_suspensions WHERE account_id = $1 AND released_at IS NULL`, id)
}

func TestSuspendWritesTheRecordAndTheStateTogether(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")

	rec, created, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Spam, Reason: "  洗版  ", SourceRef: " case-7 "})
	if err != nil || !created {
		t.Fatalf("SuspendAccount = %+v, %v, %v", rec, created, err)
	}
	if rec.AccountID != id || rec.Kind != SuspensionPermanent || rec.Category != policy.Spam || rec.Reason != "洗版" || rec.SourceRef != "case-7" ||
		rec.ReleasedAt != nil || rec.ReleaseReason != "" || !rec.SuspendedAt.Equal(f.now) {
		t.Fatalf("紀錄 = %+v", rec)
	}
	if !f.suspendedAt(t, id) {
		t.Fatal("suspended_at 沒有設定")
	}
	cur, ok, err := f.m.CurrentSuspension(ctx, id)
	if err != nil || !ok || cur.ID != rec.ID {
		t.Fatalf("CurrentSuspension = %+v, %v, %v", cur, ok, err)
	}
	acc, err := f.m.Account(ctx, id)
	if err != nil || !acc.Suspended {
		t.Fatalf("Account = %+v, %v", acc, err)
	}
}

func TestRepeatedSuspendKeepsTheFirstRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	first, created, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Fraud, Reason: "第一次"})
	if err != nil || !created {
		t.Fatalf("第一次 = %+v, %v, %v", first, created, err)
	}
	second, created, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Hate, Reason: "第二次"})
	if err != nil || created {
		t.Fatalf("第二次 = %+v, %v, %v", second, created, err)
	}
	if second.ID != first.ID || second.Category != policy.Fraud || second.Reason != "第一次" {
		t.Fatalf("重複停權改了紀錄：%+v", second)
	}
	if all, open := f.suspensionRows(t, id); all != 1 || open != 1 {
		t.Fatalf("紀錄 %d 筆、未解除 %d 筆，預期 1 與 1", all, open)
	}
	if kinds := f.jobKinds(t); len(kinds) != 0 {
		t.Fatalf("停權排了 job：%v", kinds)
	}
}

func TestUnsuspendClearsTheStateAndKeepsTheRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	suspended, _, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Harassment})
	if err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Hour)
	rec, released, err := f.m.UnsuspendAccount(ctx, id, " 申覆成立 ")
	if err != nil || !released {
		t.Fatalf("UnsuspendAccount = %+v, %v, %v", rec, released, err)
	}
	if rec.ID != suspended.ID || rec.ReleasedAt == nil || !rec.ReleasedAt.Equal(f.now) || rec.ReleaseReason != "申覆成立" {
		t.Fatalf("解除的紀錄 = %+v", rec)
	}
	if f.suspendedAt(t, id) {
		t.Fatal("解除後 suspended_at 還在")
	}
	if _, ok, err := f.m.CurrentSuspension(ctx, id); err != nil || ok {
		t.Fatalf("解除後 CurrentSuspension = %v, %v", ok, err)
	}
	if _, err := f.m.Authenticate(ctx, token(customerIssuer, "user_1")); err != nil {
		t.Fatalf("解除後帳號不能使用：%v", err)
	}
	if all, open := f.suspensionRows(t, id); all != 1 || open != 0 {
		t.Fatalf("紀錄 %d 筆、未解除 %d 筆，預期 1 與 0", all, open)
	}
	// 沒有停權時解除是成功的空操作。
	if _, released, err := f.m.UnsuspendAccount(ctx, id, "再解除"); err != nil || released {
		t.Fatalf("沒有停權時解除 = %v, %v", released, err)
	}
	// 可以再停權，這時是新的一筆。
	again, created, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Spam})
	if err != nil || !created || again.ID == suspended.ID {
		t.Fatalf("再停權 = %+v, %v, %v", again, created, err)
	}
	if all, open := f.suspensionRows(t, id); all != 2 || open != 1 {
		t.Fatalf("紀錄 %d 筆、未解除 %d 筆，預期 2 與 1", all, open)
	}
}

func TestSuspendInputIsValidated(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	for name, in := range map[string]SuspendInput{
		"沒有類別":   {AccountID: id},
		"不是政策分類": {AccountID: id, Category: "BAN_USER"},
		"說明太長":   {AccountID: id, Category: policy.Other, Reason: strings.Repeat("理", 1001)},
		"來源參照太長": {AccountID: id, Category: policy.Other, SourceRef: strings.Repeat("x", 201)},
	} {
		_, _, err := f.m.SuspendAccount(ctx, in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v，預期 invalid input", name, err)
		}
	}
	if f.suspendedAt(t, id) {
		t.Fatal("無效的輸入也停權了")
	}
	if _, _, err := f.m.UnsuspendAccount(ctx, id, " ACCOUNT_DELETED "); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("ACCOUNT_DELETED 只有帳號刪除能寫：%v", err)
	}
	_, _, err := f.m.UnsuspendAccount(ctx, id, "  ")
	wantKind(t, err, apperr.KindInvalidInput)
	_, _, err = f.m.SuspendAccount(ctx, SuspendInput{AccountID: 999, Category: policy.Other})
	wantKind(t, err, apperr.KindNotFound)
	_, _, err = f.m.UnsuspendAccount(ctx, 999, "r")
	wantKind(t, err, apperr.KindNotFound)
}

// 紀錄寫入失敗時 suspended_at 不變：兩者在同一個 transaction。
func TestFailedRecordWriteLeavesTheAccountNotSuspended(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	// 與 suspended_at 不一致的殘留紀錄：寫入新紀錄會違反「未解除的紀錄最多一筆」。
	if _, err := f.db.Pool().Exec(ctx, `INSERT INTO identity.account_suspensions (account_id, reason_category, suspended_at) VALUES ($1, 'OTHER', now())`, id); err != nil {
		t.Fatal(err)
	}
	_, _, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Spam})
	if err == nil {
		t.Fatal("紀錄寫入失敗時停權應該失敗")
	}
	if f.suspendedAt(t, id) {
		t.Fatal("紀錄寫入失敗，suspended_at 卻改了")
	}
}

// 紀錄之後的步驟（寫 accounts.suspended_at）失敗時，紀錄也不留下：紀錄與狀態在同一個 transaction。
func TestFailureAfterTheRecordRollsTheRecordBack(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	if _, err := f.db.Pool().Exec(ctx, `ALTER TABLE identity.accounts ADD CONSTRAINT never_suspended CHECK (suspended_at IS NULL) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Spam}); err == nil {
		t.Fatal("寫 suspended_at 失敗時停權應該失敗")
	}
	if all, _ := f.suspensionRows(t, id); all != 0 {
		t.Fatalf("停權失敗後留下 %d 筆紀錄", all)
	}
	if f.suspendedAt(t, id) {
		t.Fatal("停權失敗後 suspended_at 已設定")
	}
}

func TestDeletingASuspendedAccountReleasesItsRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	if _, _, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Spam}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteAccount(ctx, id); err != nil {
		t.Fatal(err)
	}
	if all, open := f.suspensionRows(t, id); all != 1 || open != 0 {
		t.Fatalf("紀錄 %d 筆、未解除 %d 筆，預期 1 與 0", all, open)
	}
	if n := f.count(t, `SELECT count(*) FROM identity.account_suspensions WHERE account_id = $1 AND release_reason = $2`, id, ReleaseReasonAccountDeleted); n != 1 {
		t.Fatalf("刪帳解除的紀錄 %d 筆", n)
	}
	_, _, err := f.m.SuspendAccount(ctx, SuspendInput{AccountID: id, Category: policy.Spam})
	wantKind(t, err, apperr.KindNotFound)
	if !slices.Equal(f.jobKinds(t), []string{"identity.clerk_user"}) {
		t.Fatalf("job = %v", f.jobKinds(t))
	}
}

func TestSuspensionTableRejectsInconsistentRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	id := f.account(t, "user_1")
	for name, q := range map[string]string{
		"不是永久":     `INSERT INTO identity.account_suspensions (account_id, kind, reason_category, suspended_at) VALUES ($1, 'TEMPORARY', 'OTHER', now())`,
		"不是政策分類":   `INSERT INTO identity.account_suspensions (account_id, reason_category, suspended_at) VALUES ($1, 'BAN_USER', now())`,
		"解除時間沒有原因": `INSERT INTO identity.account_suspensions (account_id, reason_category, suspended_at, released_at) VALUES ($1, 'OTHER', now(), now())`,
		"原因沒有解除時間": `INSERT INTO identity.account_suspensions (account_id, reason_category, suspended_at, release_reason) VALUES ($1, 'OTHER', now(), 'r')`,
		"解除早於停權":   `INSERT INTO identity.account_suspensions (account_id, reason_category, suspended_at, released_at, release_reason) VALUES ($1, 'OTHER', now(), now() - interval '1 hour', 'r')`,
		"說明頭尾有空白":  `INSERT INTO identity.account_suspensions (account_id, reason_category, reason_text, suspended_at) VALUES ($1, 'OTHER', ' x', now())`,
	} {
		if _, err := f.db.Pool().Exec(ctx, q, id); err == nil {
			t.Errorf("%s：資料庫沒有擋下", name)
		}
	}
}
