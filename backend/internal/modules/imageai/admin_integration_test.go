//go:build integration

package imageai_test

import (
	"context"
	"errors"
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
	f := &adminFixture{d: d, media: media.NewFake(), stores: stubStores{missing: map[int64]bool{404: true}}}
	f.m = imageai.New(imageai.Deps{
		DB: d, Now: func() time.Time { return wednesday }, Seller: f.stores, Stores: f.stores, Media: f.media, NewID: uuid.NewV7,
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

func (f *adminFixture) thumbnailAsset(t *testing.T, id uuid.UUID) *uuid.UUID {
	t.Helper()
	var asset *uuid.UUID
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT thumbnail_asset_id FROM imageai.styles WHERE id = $1`, id).Scan(&asset); err != nil {
		t.Fatal(err)
	}
	return asset
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

func TestThumbnailIsConfirmedInTheWriteAndTheReplacedOrDeletedAssetIsMarkedDeleted(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	t1, t2 := media.AssetFor("t1"), media.AssetFor("t2")
	s, err := f.m.CreateStyle(ctx, newStyle("t1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.thumbnailAsset(t, s.ID); got == nil || *got != t1 || !f.media.IsConfirmed(t1) || f.media.IsDeleted(t1) {
		t.Fatalf("新增後 asset = %v，t1 確認 = %v、刪除 = %v", got, f.media.IsConfirmed(t1), f.media.IsDeleted(t1))
	}
	s, err = f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: s.ID, Version: s.Version, Label: "新", Prompt: "p", Enabled: true, ThumbnailToken: "t2"})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.thumbnailAsset(t, s.ID); got == nil || *got != t2 || !f.media.IsDeleted(t1) || f.media.IsDeleted(t2) {
		t.Fatalf("換圖後 asset = %v，t1 刪除 = %v、t2 刪除 = %v", got, f.media.IsDeleted(t1), f.media.IsDeleted(t2))
	}
	s, err = f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: s.ID, Version: s.Version, Label: "再改", Prompt: "p", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.thumbnailAsset(t, s.ID); got == nil || *got != t2 || f.media.IsDeleted(t2) {
		t.Fatalf("沿用縮圖後 asset = %v，t2 刪除 = %v", got, f.media.IsDeleted(t2))
	}
	if err := f.m.DeleteStyle(ctx, admin, s.ID, 0); err != nil {
		t.Fatal(err)
	}
	if !f.media.IsDeleted(t2) {
		t.Fatal("刪除風格後縮圖沒有標記刪除")
	}
}

func TestFailedMarkDeletedRollsBackTheStyleWrite(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	s, err := f.m.CreateStyle(ctx, newStyle("t1"))
	if err != nil {
		t.Fatal(err)
	}
	f.media.FailMarkDeleted = errors.New("media 失敗")
	_, err = f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: s.ID, Version: s.Version, Label: "新", Prompt: "p", Enabled: true, ThumbnailToken: "t2"})
	if err == nil {
		t.Fatal("MarkDeleted 失敗卻成功")
	}
	if got := f.thumbnailAsset(t, s.ID); got == nil || *got != media.AssetFor("t1") {
		t.Fatalf("回滾後 asset = %v，預期仍是 t1", got)
	}
	if err := f.m.DeleteStyle(ctx, admin, s.ID, 0); err == nil {
		t.Fatal("MarkDeleted 失敗卻刪除成功")
	}
	if n := f.count(t, `SELECT count(*) FROM imageai.styles WHERE id = $1`, s.ID); n != 1 {
		t.Fatalf("回滾後風格列 = %d，預期 1", n)
	}
	if n := f.auditRows(t, "ai_style", s.ID.String()); n != 1 {
		t.Fatalf("稽核列 = %d，只該有新增那一筆", n)
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
	if def.ID == (uuid.UUID{}) || def.ThumbnailKey != "" || f.thumbnailAsset(t, def.ID) != nil {
		t.Fatalf("seed 的預設風格 = %+v", def)
	}
	got, err := f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: def.ID, Version: def.Version, Label: def.Label, Prompt: def.Prompt, Enabled: true, ThumbnailToken: "d1"})
	if err != nil {
		t.Fatal(err)
	}
	if asset := f.thumbnailAsset(t, got.ID); asset == nil || *asset != media.AssetFor("d1") || got.ThumbnailKey == "" {
		t.Fatalf("got = %+v asset = %v", got, asset)
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
	if _, err := f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: oldDefault.ID, Version: oldDefault.Version, Label: "x", Prompt: "p", Enabled: false}); kind(err) != apperr.KindFailedPrecondition {
		t.Fatalf("停用預設風格：err = %v", err)
	}
	created, err := f.m.CreateStyle(ctx, newStyle("t1"))
	if err != nil {
		t.Fatal(err)
	}
	if created.IsDefault || f.count(t, `SELECT count(*) FROM imageai.styles WHERE category = 'PRODUCT' AND is_default`) != 1 {
		t.Fatalf("新增的風格不該是預設：%+v", created)
	}
	if _, err := f.m.SetDefaultStyle(ctx, admin, created.ID, created.Version+1); kind(err) != apperr.KindVersionConflict {
		t.Fatalf("etag 過期：err = %v", err)
	}
	got, err := f.m.SetDefaultStyle(ctx, admin, created.ID, created.Version)
	if err != nil || !got.IsDefault || got.Version != created.Version+1 {
		t.Fatalf("設成預設：%+v err=%v", got, err)
	}
	if n := f.count(t, `SELECT count(*) FROM imageai.styles WHERE category = 'PRODUCT' AND is_default`); n != 1 {
		t.Fatalf("預設風格數 = %d，預期恰好 1", n)
	}
	cur, err := f.m.ReadStyleForAdmin(ctx, oldDefault.ID)
	if err != nil || cur.IsDefault || cur.Version != oldDefault.Version+1 {
		t.Fatalf("舊預設 = %+v err=%v", cur, err)
	}
	if n := f.auditRows(t, "ai_style", oldDefault.ID.String()); n != 0 {
		t.Fatalf("被取消預設的舊風格稽核列數 = %d，預期 0（只寫目標的一筆）", n)
	}
	if n := f.auditRows(t, "ai_style", created.ID.String()); n != 2 {
		t.Fatalf("新預設的稽核列數 = %d，預期 2（新增與設成預設）", n)
	}
	// 已經是預設：成功、不寫稽核。
	if _, err := f.m.SetDefaultStyle(ctx, admin, created.ID, 0); err != nil || f.auditRows(t, "ai_style", created.ID.String()) != 2 {
		t.Fatalf("重複設定：err = %v", err)
	}
	// 停用的風格不能設成預設（CHECK 之前就被擋）。
	other, err := f.m.CreateStyle(ctx, newStyle("t2"))
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := f.m.UpdateStyle(ctx, imageai.UpdateStyleInput{Actor: admin, ID: other.ID, Version: other.Version, Label: "x", Prompt: "p", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.SetDefaultStyle(ctx, admin, disabled.ID, 0); kind(err) != apperr.KindFailedPrecondition {
		t.Fatalf("設定停用的風格：err = %v", err)
	}
	// 舊預設現在可以刪。
	if err := f.m.DeleteStyle(ctx, admin, oldDefault.ID, 0); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentSetDefaultSerializesWithoutAUniqueViolation(t *testing.T) {
	f := setupAdmin(t)
	ctx := context.Background()
	var ids []uuid.UUID
	for _, tok := range []string{"c1", "c2", "c3", "c4"} {
		s, err := f.m.CreateStyle(ctx, newStyle(tok))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, s.ID)
	}
	errs := make(chan error, len(ids))
	for _, id := range ids {
		go func() {
			_, err := f.m.SetDefaultStyle(ctx, admin, id, 0)
			errs <- err
		}()
	}
	for range ids {
		if err := <-errs; err != nil {
			t.Fatalf("並行 SetDefault：%v", err)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM imageai.styles WHERE category = 'PRODUCT' AND is_default`); n != 1 {
		t.Fatalf("預設風格數 = %d，預期恰好 1", n)
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
