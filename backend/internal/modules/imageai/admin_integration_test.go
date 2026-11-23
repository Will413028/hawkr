//go:build integration

package imageai_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// wednesdayWeekStart 是 wednesday 所在那週的起點（週一 00:00 Asia/Taipei）。
func wednesdayWeekStart() time.Time { return time.Date(2026, 11, 23, 0, 0, 0, 0, twtime.Taipei) }

var admin = imageai.Actor{AdminID: 9, Action: "/test/Admin"}

type stubStores struct{ missing map[int64]bool }

func (s stubStores) Store(_ context.Context, id int64) (seller.Store, error) {
	if s.missing[id] {
		return seller.Store{}, apperr.NotFound("沒有這家店")
	}
	return seller.Store{StoreID: id}, nil
}

func (s stubStores) LockNotRetired(ctx context.Context, id int64) error {
	_, err := s.Store(ctx, id)
	return err
}

type adminFixture struct {
	d      *db.DB
	m      *imageai.Module
	media  *media.Fake
	stores stubStores
}

func setupAdmin(t *testing.T) *adminFixture {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Jobs(),
		testdb.Migrations(migrate.Module{Name: "imageai", Dir: imageai.Migrations()}, migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	f := &adminFixture{d: d, media: media.NewFake(), stores: stubStores{missing: map[int64]bool{404: true}}}
	f.m = imageai.New(imageai.Deps{
		DB: d, Now: func() time.Time { return wednesday }, Seller: f.stores, Stores: f.stores, Media: f.media, Jobs: jc, NewID: uuid.NewV7,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return f
}

func (f *adminFixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.d.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *adminFixture) auditRows(t *testing.T, targetType, targetID string) int {
	return f.count(t, `SELECT count(*) FROM adminaudit.records WHERE admin_id = 9 AND action = '/test/Admin' AND target_type = $1 AND target_id = $2`, targetType, targetID)
}

func (f *adminFixture) sync(t *testing.T, id uuid.UUID) {
	t.Helper()
	if _, err := f.m.SyncMedia(context.Background(), media.Ref{Type: media.RefTypeAiStyle, ID: id.String()}); err != nil {
		t.Fatal(err)
	}
}

func (f *adminFixture) referenced(id uuid.UUID) string {
	return fmt.Sprint(f.media.Referenced(media.Ref{Type: media.RefTypeAiStyle, ID: id.String()}))
}

func newStyle(token string) imageai.CreateStyleInput {
	return imageai.CreateStyleInput{Actor: admin, Name: "clean-" + token, Label: "乾淨", Prompt: "白底", Category: imageai.CategoryProduct, ThumbnailToken: token, Enabled: true}
}

func kind(err error) apperr.Kind { return apperr.KindOf(err) }

func TestEachStyleWriteLeavesExactlyOneAuditRowInTheSameTransaction(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	s, err := f.m.CreateStyle(ctx, newStyle("t1"))
	if err != nil {
		t.Fatal(err)
	}
	if n := f.auditRows(t, "ai_style", s.ID.String()); n != 1 {
		t.Fatalf("新增後稽核列 = %d", n)
	}
	s, err = f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: s.ID, Version: s.Version, Label: "新", Prompt: "p", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if n := f.auditRows(t, "ai_style", s.ID.String()); n != 2 {
		t.Fatalf("修改後稽核列 = %d", n)
	}
	if err := f.m.DeleteStyle(ctx, admin, s.ID, 0); err != nil {
		t.Fatal(err)
	}
	if n := f.auditRows(t, "ai_style", s.ID.String()); n != 3 {
		t.Fatalf("刪除後稽核列 = %d", n)
	}
}

func TestQuotaWritesLeaveOneAuditRowEach(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	q, err := f.m.UpdateQuota(ctx, imageai.UpdateQuotaInput{Actor: admin, StoreID: 11, WeeklyLimit: 30, Memo: "活動"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.UpdateQuota(ctx, imageai.UpdateQuotaInput{Actor: admin, StoreID: 11, WeeklyLimit: 40, Version: q.Version}); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteQuota(ctx, admin, 11, 0); err != nil {
		t.Fatal(err)
	}
	if n := f.auditRows(t, "ai_quota", "11"); n != 3 {
		t.Fatalf("額度三個寫入的稽核列 = %d", n)
	}
}

// 業務寫入在 commit 時失敗（延遲的約束觸發器），同一個 transaction 內的稽核列跟著消失：稽核與業務寫入同生同滅。
func TestAuditRowRollsBackWithTheBusinessWrite(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	exec(t, f.d, `CREATE FUNCTION imageai.fail_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced commit failure'; END $$`)
	exec(t, f.d, `CREATE CONSTRAINT TRIGGER fail_commit AFTER INSERT ON imageai.styles DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION imageai.fail_commit()`)
	if _, err := f.m.CreateStyle(ctx, newStyle("t1")); err == nil {
		t.Fatal("commit 應該失敗")
	}
	if n := f.count(t, `SELECT count(*) FROM imageai.styles WHERE name = 'clean-t1'`); n != 0 {
		t.Fatalf("風格列 = %d，預期回滾", n)
	}
	if n := f.count(t, `SELECT count(*) FROM adminaudit.records WHERE action = '/test/Admin'`); n != 0 {
		t.Fatalf("稽核列 = %d，預期隨業務寫入回滾", n)
	}
	if n := f.count(t, `SELECT count(*) FROM imageai.media_ref_sync`); n != 0 {
		t.Fatalf("media_ref_sync = %d，預期回滾", n)
	}
}

func TestStaleStyleUpdateLeavesNoAuditRowAndKeepsTheVersion(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	s, err := f.m.CreateStyle(ctx, newStyle("t1"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: s.ID, Version: s.Version + 1, Label: "x", Prompt: "p", Enabled: true})
	if kind(err) != apperr.KindVersionConflict {
		t.Fatalf("err = %v", err)
	}
	if n := f.auditRows(t, "ai_style", s.ID.String()); n != 1 {
		t.Fatalf("稽核列 = %d，只該有新增那一筆", n)
	}
}

func TestThumbnailIsFinalizedAndLinkedByTheSyncJobAndReleasedOnReplaceAndDelete(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	s, err := f.m.CreateStyle(ctx, newStyle("t1"))
	if err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM river.river_job WHERE kind = 'imageai.media_sync' AND args->>'ref_id' = $1`, s.ID.String()); n != 1 {
		t.Fatalf("同步 job = %d，預期 1（與寫入同 transaction 排入）", n)
	}
	f.sync(t, s.ID)
	if got := f.referenced(s.ID); got != "[asset-t1]" {
		t.Fatalf("references = %s", got)
	}
	s, err = f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: s.ID, Version: s.Version, Label: "新", Prompt: "p", Enabled: true, ThumbnailToken: "t2"})
	if err != nil {
		t.Fatal(err)
	}
	f.sync(t, s.ID)
	if got := f.referenced(s.ID); got != "[asset-t2]" {
		t.Fatalf("換圖後 references = %s，預期只剩 asset-t2", got)
	}
	if n := f.count(t, `SELECT count(*) FROM imageai.retired_assets WHERE asset_id = 'asset-t1'`); n != 1 {
		t.Fatalf("舊縮圖退場紀錄 = %d", n)
	}
	// 沒帶 token 沿用目前的圖：不轉正、引用不變。
	s, err = f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: s.ID, Version: s.Version, Label: "再改", Prompt: "p", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	f.sync(t, s.ID)
	if got := f.referenced(s.ID); got != "[asset-t2]" {
		t.Fatalf("沿用 references = %s", got)
	}
	if err := f.m.DeleteStyle(ctx, admin, s.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.sync(t, s.ID)
	if got := f.referenced(s.ID); got != "[]" {
		t.Fatalf("刪除後 references = %s，預期釋放", got)
	}
}

func TestSeededDefaultStylesHaveNoThumbnailAndCanBeEditedWithANewOne(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	cat := imageai.CategoryPost
	list, err := f.m.ListStylesForAdmin(ctx, &cat)
	if err != nil {
		t.Fatal(err)
	}
	var def imageai.AdminStyle
	for _, s := range list {
		if s.IsDefault {
			def = s
		}
	}
	if def.ID == (uuid.UUID{}) || def.ThumbnailKey != "" {
		t.Fatalf("seed 的預設風格 = %+v", def)
	}
	got, err := f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: def.ID, Version: def.Version, Label: def.Label, Prompt: def.Prompt, Enabled: true, IsDefault: true, ThumbnailToken: "d1"})
	if err != nil {
		t.Fatal(err)
	}
	f.sync(t, got.ID)
	if f.referenced(got.ID) != "[asset-d1]" || got.ThumbnailKey == "" {
		t.Fatalf("got = %+v refs = %s", got, f.referenced(got.ID))
	}
}

func TestDefaultStyleRulesAgainstTheRealConstraints(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	product := imageai.CategoryProduct
	list, err := f.m.ListStylesForAdmin(ctx, &product)
	if err != nil {
		t.Fatal(err)
	}
	var oldDefault imageai.AdminStyle
	for _, s := range list {
		if s.IsDefault {
			oldDefault = s
		}
	}
	if err := f.m.DeleteStyle(ctx, admin, oldDefault.ID, 0); kind(err) != apperr.KindFailedPrecondition {
		t.Fatalf("刪預設風格：err = %v", err)
	}
	if _, err := f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: oldDefault.ID, Version: oldDefault.Version, Label: "x", Prompt: "p", IsDefault: true, Enabled: false}); kind(err) != apperr.KindFailedPrecondition {
		t.Fatalf("停用預設風格：err = %v", err)
	}
	in := newStyle("t1")
	in.IsDefault = true
	created, err := f.m.CreateStyle(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM imageai.styles WHERE category = 'PRODUCT' AND is_default`); n != 1 {
		t.Fatalf("預設風格數 = %d，預期恰好 1", n)
	}
	cur, err := f.m.ReadStyleForAdmin(ctx, oldDefault.ID)
	if err != nil || cur.IsDefault || cur.Version != oldDefault.Version+1 {
		t.Fatalf("舊預設 = %+v err=%v", cur, err)
	}
	if n := f.auditRows(t, "ai_style", oldDefault.ID.String()); n != 1 {
		t.Fatalf("被取消預設的舊風格稽核列數 = %d，預期 1", n)
	}
	// 舊預設現在可以刪。
	if err := f.m.DeleteStyle(ctx, admin, oldDefault.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: created.ID, Version: created.Version, Label: "x", Prompt: "p", IsDefault: false, Enabled: true}); kind(err) != apperr.KindInvalidInput {
		t.Fatalf("取消預設：err = %v", err)
	}
}

func TestStyleNameIsUniquePerCategory(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	if _, err := f.m.CreateStyle(ctx, newStyle("t1")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CreateStyle(ctx, newStyle("t1")); kind(err) != apperr.KindAlreadyExists {
		t.Fatalf("同名：err = %v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM adminaudit.records WHERE action = '/test/Admin'`); n != 1 {
		t.Fatalf("衝突的那次不該留稽核：%d", n)
	}
	post := newStyle("t2")
	post.Name, post.Category = "clean-t1", imageai.CategoryPost
	if _, err := f.m.CreateStyle(ctx, post); err != nil {
		t.Fatalf("不同用途可以同名：%v", err)
	}
}

func TestQuotaAdminAgainstTheRealStore(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	got, err := f.m.GetQuota(ctx, 11)
	if err != nil || got.Custom || got.WeeklyLimit != imageai.DefaultWeeklyLimit {
		t.Fatalf("預設額度 = %+v err=%v", got, err)
	}
	if _, err := f.m.GetQuota(ctx, 404); kind(err) != apperr.KindNotFound {
		t.Fatalf("店不存在：err = %v", err)
	}
	if err := f.m.DeleteQuota(ctx, admin, 11, 0); kind(err) != apperr.KindNotFound {
		t.Fatalf("沒有個別設定：err = %v", err)
	}
	created, err := f.m.UpdateQuota(ctx, imageai.UpdateQuotaInput{Actor: admin, StoreID: 11, WeeklyLimit: 3})
	if err != nil || !created.Custom || created.Version != 1 {
		t.Fatalf("建立 = %+v err=%v", created, err)
	}
	_, err = f.m.UpdateQuota(ctx, imageai.UpdateQuotaInput{Actor: admin, StoreID: 11, WeeklyLimit: 4})
	if kind(err) != apperr.KindAlreadyExists {
		t.Fatalf("已有個別設定又沒帶版本：err = %v", err)
	}
	if _, err := f.m.UpdateQuota(ctx, imageai.UpdateQuotaInput{Actor: admin, StoreID: 11, WeeklyLimit: 4, Version: 9}); kind(err) != apperr.KindVersionConflict {
		t.Fatalf("過期版本：err = %v", err)
	}
	if _, err := f.m.UpdateQuota(ctx, imageai.UpdateQuotaInput{Actor: admin, StoreID: 404, WeeklyLimit: 4}); kind(err) != apperr.KindNotFound {
		t.Fatalf("店不存在：err = %v", err)
	}
	// 用量讀回本週已用的次數。
	exec(t, f.d, `INSERT INTO imageai.usage (store_id, period_start, used, updated_at) VALUES (11, $1, 2, now())`, wednesdayWeekStart())
	got, err = f.m.GetQuota(ctx, 11)
	if err != nil || got.WeeklyUsage != 2 || got.WeeklyLimit != 3 || !got.Custom {
		t.Fatalf("額度 = %+v err=%v", got, err)
	}
	if err := f.m.DeleteQuota(ctx, admin, 11, got.Version); err != nil {
		t.Fatal(err)
	}
	got, err = f.m.GetQuota(ctx, 11)
	if err != nil || got.Custom || got.WeeklyLimit != imageai.DefaultWeeklyLimit {
		t.Fatalf("刪除後 = %+v err=%v", got, err)
	}
}
