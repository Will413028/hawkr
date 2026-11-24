//go:build integration

package content_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	goimage "image"
	"image/png"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

// failingDelete 在 MarkDeleted 時失敗，用來讓寫入的 transaction 在 Confirm 之後回滾。
type failingDelete struct {
	*media.Module
	fail bool
}

func (m *failingDelete) MarkDeleted(ctx context.Context, ids []uuid.UUID) error {
	if m.fail {
		return errors.New("boom")
	}
	return m.Module.MarkDeleted(ctx, ids)
}

type realFixture struct {
	m     *content.Module
	db    *db.DB
	media *failingDelete
}

func setupReal(t *testing.T) *realFixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "media", Dir: media.Migrations()}, migrate.Module{Name: "content", Dir: content.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := media.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	f := &realFixture{db: d}
	f.media = &failingDelete{Module: media.New(media.Deps{
		DB: d, Jobs: jc, Store: media.NewMemoryStore(), Keyring: ring, NewID: uuid.NewV7,
		Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})}
	f.m = content.New(content.Deps{
		DB: d, Social: &fakeSocial{readable: map[uuid.UUID]bool{}}, Media: f.media, NewID: uuid.NewV7,
		Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return f
}

// stage 以後台管理員放一張公開 PNG，回傳可 Confirm 的 token 與資產 ID。
func (f *realFixture) stage(t *testing.T) (string, uuid.UUID) {
	t.Helper()
	put, err := f.media.PutPublic(context.Background(), media.Uploader{ID: adminID, Admin: true}, bytes.NewReader(pngOf(t)))
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT asset_id FROM media.staged_uploads WHERE token_hash = sha256($1::bytea)", put.Token).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return put.Token, id
}

func (f *realFixture) assetState(t *testing.T, id uuid.UUID) string {
	t.Helper()
	st, err := media.AssetStateOf(context.Background(), f.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return string(st)
}

func (f *realFixture) create(t *testing.T) (content.Banner, [2]uuid.UUID) {
	t.Helper()
	pt, pa := f.stage(t)
	qt, qa := f.stage(t)
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = pt, qt
	b, err := f.m.CreateBanner(context.Background(), adminID, in)
	if err != nil {
		t.Fatal(err)
	}
	return b, [2]uuid.UUID{pa, qa}
}

// 換圖：新資產在同一個 transaction 內確認，被換掉的資產標為刪除，沿用的不動。
func TestReplacingABannerImageConfirmsTheNewAssetAndMarksTheOldDeleted(t *testing.T) {
	t.Parallel()
	f := setupReal(t)
	b, old := f.create(t)
	for _, id := range old {
		if got := f.assetState(t, id); got != "ACTIVE" {
			t.Fatalf("新增後資產 state=%s，預期 ACTIVE", got)
		}
	}
	token, fresh := f.stage(t)
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = token, ""
	if _, err := f.m.UpdateBanner(context.Background(), content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in}); err != nil {
		t.Fatal(err)
	}
	if got := f.assetState(t, fresh); got != "ACTIVE" {
		t.Fatalf("新資產 state=%s，預期 ACTIVE", got)
	}
	if got := f.assetState(t, old[0]); got != "DELETING" {
		t.Fatalf("被換掉的資產 state=%s，預期 DELETING", got)
	}
	if got := f.assetState(t, old[1]); got != "ACTIVE" {
		t.Fatalf("沿用的資產 state=%s，預期 ACTIVE", got)
	}
}

// 回滾：換圖的 transaction 在 Confirm 之後失敗，新資產仍是暫存、舊資產與 banner 不變，同一個 token 重送成功。
func TestRolledBackBannerWriteLeavesTheNewAssetStaged(t *testing.T) {
	t.Parallel()
	f := setupReal(t)
	b, old := f.create(t)
	token, fresh := f.stage(t)
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = token, ""
	input := content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in}

	f.media.fail = true
	if _, err := f.m.UpdateBanner(context.Background(), input); err == nil {
		t.Fatal("MarkDeleted 失敗應讓換圖回滾")
	}
	f.media.fail = false
	if got := f.assetState(t, fresh); got != "STAGED" {
		t.Fatalf("回滾後新資產 state=%s，預期 STAGED", got)
	}
	if got := f.assetState(t, old[0]); got != "ACTIVE" {
		t.Fatalf("回滾後舊資產 state=%s，預期 ACTIVE", got)
	}
	if _, err := f.m.UpdateBanner(context.Background(), input); err != nil {
		t.Fatalf("回滾後同一個 token 重送應成功：%v", err)
	}
}

// 終態：刪除 banner 時兩張圖都標為刪除。
func TestDeletingABannerMarksBothImagesDeleted(t *testing.T) {
	t.Parallel()
	f := setupReal(t)
	b, old := f.create(t)
	if err := f.m.DeleteBanner(context.Background(), b.ID, 0); err != nil {
		t.Fatal(err)
	}
	for _, id := range old {
		if got := f.assetState(t, id); got != "DELETING" {
			t.Fatalf("刪除後資產 state=%s，預期 DELETING", got)
		}
	}
}

// 別的上傳者的 token 不能確認：回錯誤，沒有 banner 寫入，資產維持暫存。
func TestBannerRefusesATokenStagedByAnotherUploader(t *testing.T) {
	t.Parallel()
	f := setupReal(t)
	put, err := f.media.PutPublic(context.Background(), media.Uploader{ID: adminID + 1, Admin: true}, bytes.NewReader(pngOf(t)))
	if err != nil {
		t.Fatal(err)
	}
	qt, _ := f.stage(t)
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = put.Token, qt
	if _, err := f.m.CreateBanner(context.Background(), adminID, in); err == nil {
		t.Fatal("別人的 token 應被拒絕")
	}
	page, err := f.m.ListBanners(context.Background(), nil, 50)
	if err != nil || len(page.Banners) != 0 {
		t.Fatalf("被拒的建立不得寫入：%v %v", page.Banners, err)
	}
}

func pngOf(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	if err := png.Encode(&body, goimage.NewNRGBA(goimage.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
