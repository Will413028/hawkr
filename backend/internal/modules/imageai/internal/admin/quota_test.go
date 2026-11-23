package admin

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func actorOf() adminaudit.Actor {
	return adminaudit.Actor{AdminID: actor.AdminID, Action: actor.Action}
}

type failingAudit struct{}

func (failingAudit) Record(context.Context, adminaudit.Entry) error {
	return adminaudit.ErrNoTransaction
}

func TestGetQuotaReturnsTheDefaultWhenThereIsNoCustomSetting(t *testing.T) {
	h := newHarness(t)
	h.used = 4
	got, err := h.s.GetQuota(context.Background(), 11)
	if err != nil {
		t.Fatal(err)
	}
	if got.Custom || got.WeeklyLimit != model.DefaultWeeklyLimit || got.WeeklyUsage != 4 || got.Version != 0 || got.StoreID != 11 {
		t.Fatalf("got = %+v", got)
	}
}

func TestQuotaMethodsReturnNotFoundForAStoreThatDoesNotExist(t *testing.T) {
	h := newHarness(t)
	h.stores.missing[404] = true
	ctx := context.Background()
	_, err := h.s.GetQuota(ctx, 404)
	wantKind(t, err, apperr.KindNotFound)
	_, err = h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 404, WeeklyLimit: 5})
	wantKind(t, err, apperr.KindNotFound)
	wantKind(t, h.s.DeleteQuota(ctx, actor, 404, 0), apperr.KindNotFound)
	if len(h.q.quotas) != 0 || len(h.audit.entries) != 0 {
		t.Fatal("店家不存在時不該寫入或寫稽核")
	}
}

func TestUpdateQuotaWithoutAVersionCreatesWhenThereIsNoCustomSetting(t *testing.T) {
	h := newHarness(t)
	got, err := h.s.UpdateQuota(context.Background(), model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: 30, Memo: "  活動  "})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Custom || got.WeeklyLimit != 30 || got.Memo != "活動" || got.Version != 1 {
		t.Fatalf("got = %+v", got)
	}
	if len(h.audit.entries) != 1 || h.audit.entries[0].TargetType != "ai_quota" || h.audit.entries[0].TargetID != "11" {
		t.Fatalf("稽核 = %v", h.audit.entries)
	}
}

func TestUpdateQuotaWithoutAVersionWhenACustomSettingExistsIsAlreadyExists(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: 30}); err != nil {
		t.Fatal(err)
	}
	_, err := h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: 99})
	wantKind(t, err, apperr.KindAlreadyExists)
	if reasonOf(err) != apperr.ReasonVersionConflict {
		t.Fatalf("reason = %v，預期 VERSION_CONFLICT", reasonOf(err))
	}
	if h.q.quotas[11].WeeklyLimit != 30 || len(h.audit.entries) != 1 {
		t.Fatal("衝突時不該覆寫或寫稽核")
	}
}

func TestUpdateQuotaVersionRules(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	created, err := h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: 30})
	if err != nil {
		t.Fatal(err)
	}
	// 帶目前的版本：更新並加一。
	got, err := h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: 40, Version: created.Version})
	if err != nil || got.WeeklyLimit != 40 || got.Version != created.Version+1 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	// 過期的版本：ABORTED（VersionConflict）。
	_, err = h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: 50, Version: created.Version})
	wantKind(t, err, apperr.KindVersionConflict)
	// 帶了版本但個別設定已被刪除：也是過期。
	if err := h.s.DeleteQuota(ctx, actor, 11, 0); err != nil {
		t.Fatal(err)
	}
	_, err = h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: 50, Version: got.Version})
	wantKind(t, err, apperr.KindVersionConflict)
}

func TestUpdateQuotaWeeklyLimitRange(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for _, n := range []int32{-1, model.MaxWeeklyLimit + 1} {
		_, err := h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: n})
		wantKind(t, err, apperr.KindInvalidInput)
		if got := fieldsOf(err); len(got) != 1 || got[0] != "WeeklyLimit" {
			t.Fatalf("limit %d 欄位 = %v", n, got)
		}
	}
	for _, n := range []int32{0, model.MaxWeeklyLimit} {
		store := int64(100 + n)
		if _, err := h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: store, WeeklyLimit: n}); err != nil {
			t.Fatalf("limit %d：%v", n, err)
		}
	}
}

func TestDeleteQuotaWithoutACustomSettingIsNotFound(t *testing.T) {
	h := newHarness(t)
	wantKind(t, h.s.DeleteQuota(context.Background(), actor, 11, 0), apperr.KindNotFound)
	if len(h.audit.entries) != 0 {
		t.Fatal("沒有東西可刪時不該寫稽核")
	}
}

func TestDeleteQuotaChecksTheVersionWhenGivenAndAudits(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	created, err := h.s.UpdateQuota(ctx, model.UpdateQuotaInput{Actor: actor, StoreID: 11, WeeklyLimit: 30})
	if err != nil {
		t.Fatal(err)
	}
	wantKind(t, h.s.DeleteQuota(ctx, actor, 11, created.Version+1), apperr.KindVersionConflict)
	if _, ok := h.q.quotas[11]; !ok {
		t.Fatal("版本不符不該刪除")
	}
	if err := h.s.DeleteQuota(ctx, actor, 11, created.Version); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.q.quotas[11]; ok || len(h.audit.entries) != 2 {
		t.Fatalf("quotas=%v 稽核=%d", h.q.quotas, len(h.audit.entries))
	}
}
