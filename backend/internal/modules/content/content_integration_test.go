//go:build integration

package content_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/content"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type fakeMedia struct {
	mu        sync.Mutex
	ids       map[string]uuid.UUID
	confirmed []confirmCall
	deleted   []uuid.UUID
}

type confirmCall struct {
	uploader media.Uploader
	token    string
}

func (f *fakeMedia) id(token string) uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ids == nil {
		f.ids = map[string]uuid.UUID{}
	}
	if _, ok := f.ids[token]; !ok {
		f.ids[token] = uuid.NewV7()
	}
	return f.ids[token]
}

func (f *fakeMedia) Confirm(_ context.Context, u media.Uploader, token string, _ media.Visibility) (media.ConfirmedAsset, error) {
	if strings.HasPrefix(token, "bad") {
		return media.ConfirmedAsset{}, errors.New("staged token 無效")
	}
	if strings.HasPrefix(token, "gone") {
		return media.ConfirmedAsset{}, apperr.FailedPrecondition("圖片已失效，請重新上傳")
	}
	f.mu.Lock()
	f.confirmed = append(f.confirmed, confirmCall{u, token})
	f.mu.Unlock()
	return media.ConfirmedAsset{ID: f.id(token), ObjectKey: "public/7109/" + token + ".jpg", Visibility: media.VisibilityPublic}, nil
}

func (f *fakeMedia) MarkDeleted(_ context.Context, ids []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, ids...)
	return nil
}

type fakeSocial struct {
	mu       sync.Mutex
	readable map[uuid.UUID]bool
}

func (s *fakeSocial) set(id uuid.UUID, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readable[id] = ok
}

func (s *fakeSocial) PubliclyReadablePosts(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		if s.readable[id] {
			out[id] = true
		}
	}
	return out, nil
}

type fakeProducts struct {
	mu     sync.Mutex
	public map[uuid.UUID]bool
	err    error
	calls  int
	at     []time.Time
}

func (p *fakeProducts) set(id uuid.UUID, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.public[id] = ok
}

func (p *fakeProducts) PublicProduct(_ context.Context, id uuid.UUID, at time.Time) (catalog.Product, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.at = append(p.at, at)
	if p.err != nil {
		return catalog.Product{}, p.err
	}
	if !p.public[id] {
		return catalog.Product{}, catalog.ErrProductNotFound
	}
	return catalog.Product{ID: id}, nil
}

var publicProduct = uuid.MustParse("018f0000-0000-7000-8000-0000000000a1")

type fixture struct {
	m        *content.Module
	social   *fakeSocial
	products *fakeProducts
	db       *db.DB
	media    *fakeMedia
	now      time.Time
	newID    func() uuid.UUID
}

const adminID = 9001
const activeBannerLimit = 20

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "content", Dir: content.Migrations()}))
	f := &fixture{social: &fakeSocial{readable: map[uuid.UUID]bool{}}, products: &fakeProducts{public: map[uuid.UUID]bool{publicProduct: true}}, db: d, media: &fakeMedia{}, now: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), newID: uuid.NewV7}
	f.m = content.New(content.Deps{
		DB:       d,
		Social:   f.social,
		Products: f.products,
		Media:    f.media,
		NewID:    func() uuid.UUID { return f.newID() },
		Now:      func() time.Time { return f.now },
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return f
}

func wantKind(t *testing.T, err error, want apperr.Kind) {
	t.Helper()
	if got := apperr.KindOf(err); got != want || err == nil {
		t.Fatalf("err = %v（分類 %v），預期分類 %v", err, got, want)
	}
}

func bannerInput() content.BannerInput {
	return content.BannerInput{
		Title:             "中秋禮盒",
		ButtonText:        "看看",
		PreviewImageToken: "p1",
		PopupImageToken:   "q1",
		Action:            content.BannerAction{Type: content.BannerActionProduct, Target: publicProduct.String()},
		DisplayOrder:      1,
		IsActive:          true,
	}
}

func TestCreateBannerConfirmsBothImages(t *testing.T) {
	t.Parallel()
	f := setup(t)
	b, err := f.m.CreateBanner(context.Background(), adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	admin := media.Uploader{ID: adminID, Admin: true}
	want := []confirmCall{{admin, "p1"}, {admin, "q1"}}
	if !slices.Equal(f.media.confirmed, want) {
		t.Fatalf("確認 = %+v", f.media.confirmed)
	}
	if len(f.media.deleted) != 0 {
		t.Fatalf("新增不得標記刪除：%v", f.media.deleted)
	}
	if got := assetIDs(t, f, b.ID); got != [2]uuid.UUID{f.media.id("p1"), f.media.id("q1")} {
		t.Fatalf("banner 的資產 = %v", got)
	}
	got, err := f.m.Banner(context.Background(), b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PreviewImageKey != "public/7109/p1.jpg" || got.PopupImageKey != "public/7109/q1.jpg" {
		t.Errorf("圖片 = %q、%q", got.PreviewImageKey, got.PopupImageKey)
	}
	if got.Action != (content.BannerAction{Type: content.BannerActionProduct, Target: publicProduct.String()}) {
		t.Errorf("點擊行為 = %+v", got.Action)
	}
}

func TestCreateBannerValidatesBeforeTouchingMedia(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*content.BannerInput){
		"缺彈窗圖":    func(in *content.BannerInput) { in.PopupImageToken = "" },
		"點擊行為不完整": func(in *content.BannerInput) { in.Action = content.BannerAction{Type: content.BannerActionURL} },
		"沒有標題":    func(in *content.BannerInput) { in.Title = " " },
		"結束等於開始": func(in *content.BannerInput) {
			at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			in.StartsAt, in.EndsAt = &at, &at
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			in := bannerInput()
			mutate(&in)
			_, err := f.m.CreateBanner(context.Background(), adminID, in)
			wantKind(t, err, apperr.KindInvalidInput)
			if len(f.media.confirmed) != 0 {
				t.Fatalf("驗證失敗仍確認了圖片：%+v", f.media.confirmed)
			}
		})
	}
}

func TestCreateBannerWritesNothingWhenFinalizeFails(t *testing.T) {
	t.Parallel()
	f := setup(t)
	in := bannerInput()
	in.PopupImageToken = "bad"
	if _, err := f.m.CreateBanner(context.Background(), adminID, in); err == nil {
		t.Fatal("預期失敗")
	}
	all, err := f.m.ListBanners(context.Background(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Banners) != 0 {
		t.Fatalf("轉正失敗仍寫入了 banner：%+v", all.Banners)
	}
}

func TestUpdateBannerMarksExactlyTheReplacedImagesDeleted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}

	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "p2", ""
	b, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	if want := []uuid.UUID{f.media.id("p1")}; !slices.Equal(f.media.deleted, want) {
		t.Fatalf("只換預覽圖之後標記刪除 = %v，預期 %v", f.media.deleted, want)
	}
	if got := assetIDs(t, f, b.ID); got != [2]uuid.UUID{f.media.id("p2"), f.media.id("q1")} {
		t.Fatalf("只換預覽圖之後的資產 = %v", got)
	}

	f.media.deleted = nil
	in.PreviewImageToken, in.PopupImageToken = "p3", "q3"
	got, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	if want := []uuid.UUID{f.media.id("p2"), f.media.id("q1")}; !slices.Equal(f.media.deleted, want) {
		t.Fatalf("兩張都換之後標記刪除 = %v，預期 %v", f.media.deleted, want)
	}
	if got.PopupImageKey != "public/7109/q3.jpg" {
		t.Errorf("彈窗圖 = %q", got.PopupImageKey)
	}
}

func assetIDs(t *testing.T, f *fixture, id uuid.UUID) [2]uuid.UUID {
	t.Helper()
	var out [2]uuid.UUID
	if err := f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT preview_image_asset_id, popup_image_asset_id FROM content.banners WHERE id=$1", id).Scan(&out[0], &out[1]); err != nil {
		t.Fatal(err)
	}
	return out
}

// 沒有帶新圖的欄位沿用這筆 banner 現在的圖，不轉正任何東西。
func TestUpdateBannerKeepsItsCurrentImages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	f.media.confirmed = nil
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "", ""
	got, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	if got.PreviewImageKey != b.PreviewImageKey || got.PopupImageKey != b.PopupImageKey || len(f.media.confirmed) != 0 {
		t.Fatalf("圖片 = %q、%q，確認 %+v", got.PreviewImageKey, got.PopupImageKey, f.media.confirmed)
	}
}

// media 拒絕確認 staged token 時，欄位錯誤指向該 token、訊息用 media 給使用者的文字，media 原本的錯誤留在 cause，banner 不寫入。
func TestMediaRejectionOfAnImageSurfacesUnchanged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	in := bannerInput()
	in.PopupImageToken = "gone1"
	_, err := f.m.CreateBanner(context.Background(), adminID, in)
	wantKind(t, err, apperr.KindInvalidInput)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Message != "圖片已失效，請重新上傳" || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "PopupImageToken" {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "failed_precondition") {
		t.Fatalf("media 原本的錯誤不在 cause：%v", err)
	}
}

func TestUpdateBannerThatDoesNotExistTouchesNoMedia(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, err := f.m.UpdateBanner(context.Background(), content.UpdateBannerInput{AdminID: adminID, ID: uuid.NewV7(), Version: 1, Banner: bannerInput()})
	wantKind(t, err, apperr.KindNotFound)
	if len(f.media.confirmed) != 0 {
		t.Fatalf("不存在的 banner 仍確認了圖片：%+v", f.media.confirmed)
	}
}

func TestListLiveBannersRespectsActiveFlagAndSchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	create := func(title string, active bool, start, end *time.Time) {
		in := bannerInput()
		in.Title, in.IsActive, in.StartsAt, in.EndsAt = title, active, start, end
		if _, err := f.m.CreateBanner(ctx, adminID, in); err != nil {
			t.Fatal(err)
		}
	}
	past, now, future := f.now.Add(-time.Hour), f.now, f.now.Add(time.Hour)
	create("沒排程", true, nil, nil)
	create("停用", false, nil, nil)
	create("進行中", true, &past, &future)
	create("還沒開始", true, &future, nil)
	create("已結束", true, nil, &past)
	create("剛好開始", true, &now, nil)
	create("剛好結束", true, nil, &now)

	live, err := f.m.ListLiveBanners(ctx, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, b := range live.Banners {
		titles = append(titles, b.Title)
	}
	if !slices.Equal(titles, []string{"沒排程", "進行中", "剛好開始"}) {
		t.Fatalf("顯示中的 banner = %v", titles)
	}
}

func TestBannersCanOnlyBeActivatedUpToTheLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	var last content.Banner
	for i := range activeBannerLimit {
		b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
		if err != nil {
			t.Fatalf("第 %d 則：%v", i+1, err)
		}
		last = b
	}
	confirmed := len(f.media.confirmed)
	_, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	wantKind(t, err, apperr.KindFailedPrecondition)
	if len(f.media.confirmed) != confirmed {
		t.Fatal("超過上限仍確認了圖片")
	}

	off := bannerInput()
	off.IsActive = false
	inactive, err := f.m.CreateBanner(ctx, adminID, off)
	if err != nil {
		t.Fatal(err)
	}
	keep := bannerInput()
	keep.PreviewImageToken, keep.PopupImageToken = "", ""
	_, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: inactive.ID, Version: inactive.Version, Banner: keep})
	wantKind(t, err, apperr.KindFailedPrecondition)

	edit := keep
	edit.Title = "改標題"
	last, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: last.ID, Version: last.Version, Banner: edit})
	if err != nil {
		t.Fatalf("修改已上架的 banner：%v", err)
	}
	down := keep
	down.IsActive = false
	if _, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: last.ID, Version: last.Version, Banner: down}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: inactive.ID, Version: inactive.Version, Banner: keep}); err != nil {
		t.Fatalf("有空位之後上架：%v", err)
	}
}

func TestDeleteBannerMarksItsImagesDeleted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteBanner(ctx, b.ID, 0); err != nil {
		t.Fatal(err)
	}
	if want := []uuid.UUID{f.media.id("p1"), f.media.id("q1")}; !slices.Equal(f.media.deleted, want) {
		t.Fatalf("刪除之後標記刪除 = %v，預期 %v", f.media.deleted, want)
	}
	_, err = f.m.Banner(ctx, b.ID)
	wantKind(t, err, apperr.KindNotFound)
	wantKind(t, f.m.DeleteBanner(ctx, b.ID, 0), apperr.KindNotFound)
}

func TestListBannersPagesInDisplayOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	var created []uuid.UUID
	for _, order := range []int32{3, 1, 2, 1} {
		in := bannerInput()
		in.DisplayOrder = order
		b, err := f.m.CreateBanner(ctx, adminID, in)
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, b.ID)
	}
	var got []uuid.UUID
	var after *content.BannerCursor
	for range 10 {
		p, err := f.m.ListBanners(ctx, after, 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range p.Banners {
			got = append(got, b.ID)
		}
		if after = p.Next; after == nil {
			break
		}
	}
	if want := []uuid.UUID{created[1], created[3], created[2], created[0]}; !slices.Equal(got, want) {
		t.Fatalf("逐頁讀完 = %v，預期 %v", got, want)
	}
}

func TestUpdateBannerRejectsAStaleVersionBeforeFinalizing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	in := bannerInput()
	in.Title = "第一個人改的"
	in.PreviewImageToken, in.PopupImageToken = "", ""
	first, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != b.Version+1 {
		t.Fatalf("修改後的版本 = %d", first.Version)
	}

	f.media.confirmed = nil
	in.Title = "第二個人改的"
	in.PreviewImageToken = "p2"
	_, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	wantKind(t, err, apperr.KindVersionConflict)
	if len(f.media.confirmed) != 0 {
		t.Fatalf("版本已經不符，卻還確認了圖：%+v", f.media.confirmed)
	}
	got, err := f.m.Banner(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "第一個人改的" {
		t.Fatalf("被拒的修改蓋掉了別人的：%q", got.Title)
	}

	_, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: 0, Banner: in})
	wantKind(t, err, apperr.KindInvalidInput)
}

// 兩個人同時從同一個版本修改同一筆食譜：只有一個人寫入，另一個人拿到版本衝突。
func TestDeleteChecksTheVersionWhenGiven(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	wantKind(t, f.m.DeleteBanner(ctx, b.ID, b.Version+1), apperr.KindVersionConflict)
	if err := f.m.DeleteBanner(ctx, b.ID, b.Version); err != nil {
		t.Fatal(err)
	}
	wantKind(t, f.m.DeleteBanner(ctx, b.ID, b.Version), apperr.KindNotFound)
}

func TestCheckAppVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)

	got, err := f.m.CheckAppVersion(ctx, "BUYER", "1.0.0")
	if err != nil || got.UpdateRequired {
		t.Fatalf("沒有設定時應放行：%+v, %v", got, err)
	}

	if _, err := f.m.CreateAppVersionGate(ctx, content.AppVersionGate{
		AppType: "BUYER", MinimumVersion: "1.2.10",
		StoreURL: "itms-apps://apps.apple.com/app/id1", NoticeTitle: "請更新",
	}); err != nil {
		t.Fatal(err)
	}
	for client, want := range map[string]bool{"1.2.9": true, "1.2.10": false, "1.10.0": false, "1.2.3(7)": false} {
		got, err := f.m.CheckAppVersion(ctx, "BUYER", client)
		if err != nil {
			t.Fatal(err)
		}
		if got.UpdateRequired != want {
			t.Errorf("client %s：UpdateRequired = %v", client, got.UpdateRequired)
		}
		if got.NoticeTitle != "請更新" || got.StoreURL == "" {
			t.Errorf("client %s：提示沒帶回來：%+v", client, got)
		}
	}

	_, err = f.m.CheckAppVersion(ctx, "ADMIN", "1.0.0")
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = f.m.CheckAppVersion(ctx, "BUYER", "")
	wantKind(t, err, apperr.KindInvalidInput)
}

func TestAppVersionGateWritesValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	for name, g := range map[string]content.AppVersionGate{
		"App 類型":  {AppType: "ADMIN", MinimumVersion: "2.0.0"},
		"版本格式":    {AppType: "SELLER", MinimumVersion: "2.0"},
		"商店網址的協定": {AppType: "SELLER", MinimumVersion: "2.0.0", StoreURL: "javascript:alert(1)"},
	} {
		_, err := f.m.CreateAppVersionGate(ctx, g)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("建立，%s：err = %v", name, err)
		}
		_, err = f.m.UpdateAppVersionGate(ctx, g, 1)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("修改，%s：err = %v", name, err)
		}
	}
	got, err := f.m.CreateAppVersionGate(ctx, content.AppVersionGate{AppType: "SELLER", MinimumVersion: "2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if got.StoreURL != "" || got.Version != 1 {
		t.Errorf("沒填商店網址應回空字串、版本從 1 開始：%+v", got)
	}
}

// 以 key 識別的門檻：建立只建立、不覆寫；修改要帶目前的版本（ADR 的 D3）。
func TestAppVersionGateCreateOnlyCreatesAndUpdateNeedsTheVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	g := content.AppVersionGate{AppType: "SELLER", MinimumVersion: "2.0.0"}

	_, err := f.m.UpdateAppVersionGate(ctx, g, 1)
	wantKind(t, err, apperr.KindNotFound)

	created, err := f.m.CreateAppVersionGate(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	again := g
	again.MinimumVersion = "3.0.0"
	_, err = f.m.CreateAppVersionGate(ctx, again)
	wantKind(t, err, apperr.KindAlreadyExists)

	updated, err := f.m.UpdateAppVersionGate(ctx, again, created.Version)
	if err != nil {
		t.Fatal(err)
	}
	if updated.MinimumVersion != "3.0.0" || updated.Version != created.Version+1 {
		t.Fatalf("修改後 = %+v", updated)
	}
	_, err = f.m.UpdateAppVersionGate(ctx, g, created.Version)
	wantKind(t, err, apperr.KindVersionConflict)
	_, err = f.m.UpdateAppVersionGate(ctx, g, 0)
	wantKind(t, err, apperr.KindInvalidInput)

	all, err := f.m.ListAppVersionGates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].MinimumVersion != "3.0.0" {
		t.Fatalf("門檻 = %+v", all)
	}
}

// 兩個人同時建立同一組門檻：只有一個人建立成功，另一個人拿到已經存在，不會覆寫。
func TestConcurrentCreatesOfTheSameGate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	var start sync.WaitGroup
	start.Add(1)
	errs := make(chan error, 2)
	for _, v := range []string{"2.0.0", "3.0.0"} {
		go func() {
			start.Wait()
			_, err := f.m.CreateAppVersionGate(ctx, content.AppVersionGate{AppType: "BUYER", MinimumVersion: v})
			errs <- err
		}()
	}
	start.Done()
	var ok, exists int
	for range 2 {
		err := <-errs
		if apperr.KindOf(err) == apperr.KindAlreadyExists {
			exists++
		} else {
			if err != nil {
				t.Fatal(err)
			}
			ok++
		}
	}
	if ok != 1 || exists != 1 {
		t.Fatalf("成功 %d、已經存在 %d，預期各 1", ok, exists)
	}
	all, err := f.m.ListAppVersionGates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Version != 1 {
		t.Fatalf("門檻 = %+v（被覆寫過的話版本會大於 1）", all)
	}
}

// 排程的兩道防線判定一致：公開寫入 API 與 DB 的 banners_schedule_check 對同一組開始、結束時間
// 必須給出同樣的結論。排程是 [開始, 結束)，所以開始等於結束也不合法。
func TestBannerScheduleCheckMatchesTheGoRule(t *testing.T) {
	t.Parallel()
	f := setup(t)
	start := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	for _, end := range []time.Time{start.Add(time.Hour), start, start.Add(-time.Hour)} {
		in := bannerInput()
		in.StartsAt, in.EndsAt = &start, &end
		_, writeErr := f.m.CreateBanner(context.Background(), adminID, in)
		goOK := writeErr == nil
		if writeErr != nil {
			var inputErr *apperr.Error
			if !errors.As(writeErr, &inputErr) || inputErr.Kind() != apperr.KindInvalidInput ||
				!slices.ContainsFunc(inputErr.FieldViolations(), func(v apperr.FieldViolation) bool {
					return v.Field == "StartsAt" || v.Field == "EndsAt"
				}) {
				t.Fatalf("模組未因排程欄位拒絕寫入：%v", writeErr)
			}
		}
		_, err := f.db.Pool().Exec(context.Background(),
			`INSERT INTO content.banners (id, title, preview_image_key, popup_image_key, preview_image_asset_id, popup_image_asset_id, action_type, start_date, end_date)
			 VALUES ($1, '排程', 'k', 'k', $4, $4, 'POPUP', $2, $3)`, uuid.NewV7(), start, end, uuid.NewV7())
		if err != nil && !strings.Contains(err.Error(), "banners_schedule_check") {
			t.Fatalf("結束減開始 = %v：%v", end.Sub(start), err)
		}
		if dbOK := err == nil; dbOK != goOK {
			t.Errorf("結束減開始 = %v：模組判定 ok = %v，DB 判定 ok = %v", end.Sub(start), goOK, dbOK)
		}
	}
}

// POST 目標的標準形式（小寫的 UUID）不只由模組的寫入守住：直接寫入非標準形式的列，DB 的 banners_action_check 也拒絕。
func TestBannerPostTargetMustBeCanonicalInTheDatabase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	canonical := uuid.NewV7().String()
	for target, wantOK := range map[string]bool{
		canonical:                              true,
		strings.ToUpper(canonical):             false,
		" " + canonical:                        false,
		strings.ReplaceAll(canonical, "-", ""): false,
		"not-a-uuid":                           false,
	} {
		_, err := f.db.Pool().Exec(context.Background(),
			`INSERT INTO content.banners (id, title, preview_image_key, popup_image_key, preview_image_asset_id, popup_image_asset_id, action_type, action_target)
			 VALUES ($1, '貼文', 'k', 'k', $3, $3, 'POST', $2)`, uuid.NewV7(), target, uuid.NewV7())
		if wantOK && err != nil {
			t.Errorf("標準形式 %q 被拒絕：%v", target, err)
		}
		if !wantOK && (err == nil || !strings.Contains(err.Error(), "banners_action_check")) {
			t.Errorf("非標準形式 %q 預期被 banners_action_check 拒絕：%v", target, err)
		}
	}
}

// 資料庫的 CHECK 與模組的值域一致（ADR 2026-09-25-codemeta-retirement D3）：值域只在 Go 與 CHECK 兩處，
// 改了一邊沒改另一邊，這支測試就會失敗。
func TestAppTargetChecksMatchTheGoTypes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	quoted := regexp.MustCompile(`'([^']+)'`)
	cases := []struct {
		constraint string
		want       []string
	}{
		{"app_version_gates_app_type_check", stringsOf(content.AppTypes())},
	}
	for _, c := range cases {
		var def string
		err := f.db.Pool().QueryRow(context.Background(),
			`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = $1`, c.constraint).Scan(&def)
		if err != nil {
			t.Fatalf("%s：%v", c.constraint, err)
		}
		var got []string
		for _, m := range quoted.FindAllStringSubmatch(def, -1) {
			got = append(got, m[1])
		}
		slices.Sort(got)
		slices.Sort(c.want)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s 允許 %v，Go 的值域是 %v（%s）", c.constraint, got, c.want, def)
		}
	}
}

func productBannerInput(target string) content.BannerInput {
	in := bannerInput()
	in.Action = content.BannerAction{Type: content.BannerActionProduct, Target: target}
	return in
}

// 商品目標建立時要經 catalog 確認公開可見。不存在的商品用合法的 UUID：被拒的原因只能是 catalog 回找不到，不是格式。
func TestCreateBannerRequiresTheProductToBePubliclyVisible(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	missing := uuid.New()
	_, err := f.m.CreateBanner(ctx, adminID, productBannerInput(missing.String()))
	wantKind(t, err, apperr.KindInvalidInput)
	if f.products.calls != 1 {
		t.Fatalf("問了 catalog %d 次，預期 1 次（格式合法，被拒是因為商品不存在）", f.products.calls)
	}
	if len(f.media.confirmed) != 0 {
		t.Errorf("被拒的建立轉正了 %d 張圖", len(f.media.confirmed))
	}
	page, err := f.m.ListBanners(ctx, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Banners) != 0 {
		t.Errorf("被拒的建立留下 %d 則 banner", len(page.Banners))
	}

	exists := uuid.New()
	f.products.set(exists, true)
	b, err := f.m.CreateBanner(ctx, adminID, productBannerInput(exists.String()))
	if err != nil {
		t.Fatal(err)
	}
	if b.Action.Target != exists.String() {
		t.Errorf("目標 = %q", b.Action.Target)
	}
	if got := f.products.at[len(f.products.at)-1]; !got.Equal(f.now) {
		t.Errorf("問 catalog 的時間 = %v，要用模組的時鐘 %v", got, f.now)
	}
}

// 商品目標不是 UUID（舊的數字 ID）在問 catalog 之前就被拒；空白與大寫寫入前轉成小寫的標準形。
func TestCreateBannerRejectsAProductTargetThatIsNotAUUIDAndStoresTheCanonicalForm(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, err := f.m.CreateBanner(ctx, adminID, productBannerInput("42"))
	wantKind(t, err, apperr.KindInvalidInput)
	if f.products.calls != 0 {
		t.Fatalf("格式不對不該問 catalog：%d 次", f.products.calls)
	}
	b, err := f.m.CreateBanner(ctx, adminID, productBannerInput("  "+strings.ToUpper(publicProduct.String())+"\t"))
	if err != nil {
		t.Fatal(err)
	}
	if b.Action.Target != publicProduct.String() {
		t.Fatalf("存的目標 = %q，預期 %q", b.Action.Target, publicProduct.String())
	}
}

func TestCreateBannerDoesNotSwallowACatalogFailure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	boom := errors.New("boom")
	f.products.err = boom
	_, err := f.m.CreateBanner(context.Background(), adminID, bannerInput())
	if !errors.Is(err, boom) || apperr.KindOf(err) == apperr.KindInvalidInput {
		t.Fatalf("err = %v，預期原樣包著 catalog 的錯誤、不是 InvalidInput", err)
	}
}

// 修改時只有換掉目標才檢查商品：目標沒變的 banner（例如商品後來下架）仍可修改其他欄位。
func TestUpdateBannerChecksTheProductOnlyWhenTheTargetChanges(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "", ""
	in.Action = content.BannerAction{Type: content.BannerActionProduct, Target: uuid.New().String()}
	_, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	wantKind(t, err, apperr.KindInvalidInput)

	f.products.set(publicProduct, false)
	callsBefore := f.products.calls
	in.Action = b.Action
	in.Title = "改標題"
	b, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatalf("目標沒變時不再檢查商品：%v", err)
	}
	if b.Title != "改標題" || f.products.calls != callsBefore {
		t.Fatalf("title=%q，問 catalog %d 次（原本 %d）", b.Title, f.products.calls, callsBefore)
	}
}

func postBannerInput(postID uuid.UUID) content.BannerInput {
	in := bannerInput()
	in.Action = content.BannerAction{Type: content.BannerActionPost, Target: postID.String()}
	return in
}

// A7-D28 第 7 點：貼文目標建立時要存在而且公開可讀；不存在、草稿、隱藏、下架、已刪除在 social 都是不可讀。
func TestCreateBannerRequiresThePostToBePubliclyReadable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	notReadable := uuid.New()
	_, err := f.m.CreateBanner(ctx, adminID, postBannerInput(notReadable))
	wantKind(t, err, apperr.KindInvalidInput)
	if len(f.media.confirmed) != 0 {
		t.Errorf("被拒的建立轉正了 %d 張圖", len(f.media.confirmed))
	}
	page, err := f.m.ListBanners(ctx, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Banners) != 0 {
		t.Errorf("被拒的建立留下 %d 則 banner", len(page.Banners))
	}

	readable := uuid.New()
	f.social.set(readable, true)
	b, err := f.m.CreateBanner(ctx, adminID, postBannerInput(readable))
	if err != nil {
		t.Fatal(err)
	}
	if b.Action.Target != readable.String() {
		t.Errorf("目標 = %q", b.Action.Target)
	}
}

// A7-D23 O5：前台讀取時，貼文目標已經不公開可讀的 banner 不顯示；貼文回到公開後再度出現；後台清單不過濾。
func TestPublicReadHidesABannerWhoseTargetBecameNotReadable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	post := uuid.New()
	f.social.set(post, true)
	if _, err := f.m.CreateBanner(ctx, adminID, postBannerInput(post)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CreateBanner(ctx, adminID, bannerInput()); err != nil {
		t.Fatal(err)
	}
	live := func() int {
		page, err := f.m.ListLiveBanners(ctx, nil, 50)
		if err != nil {
			t.Fatal(err)
		}
		return len(page.Banners)
	}
	if got := live(); got != 2 {
		t.Fatalf("公開時前台看到 %d 則，預期 2", got)
	}
	f.social.set(post, false)
	if got := live(); got != 1 {
		t.Errorf("貼文不可讀後前台看到 %d 則，預期 1（商品 banner）", got)
	}
	if page, err := f.m.ListBanners(ctx, nil, 50); err != nil || len(page.Banners) != 2 {
		t.Errorf("後台清單 = %d 則（err %v），預期 2", len(page.Banners), err)
	}
	f.social.set(post, true)
	if got := live(); got != 2 {
		t.Errorf("貼文回到公開後前台看到 %d 則，預期 2", got)
	}
}

// 目標前後有空白時存正規形的 UUID，前台讀取才找得到它。
func TestCreateBannerStoresTheCanonicalPostTarget(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	post := uuid.New()
	f.social.set(post, true)
	in := postBannerInput(post)
	in.Action.Target = "  " + post.String() + "\t"
	b, err := f.m.CreateBanner(ctx, adminID, in)
	if err != nil {
		t.Fatal(err)
	}
	if b.Action.Target != post.String() {
		t.Errorf("存的目標 = %q，預期 %q", b.Action.Target, post.String())
	}
	page, err := f.m.ListLiveBanners(ctx, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Banners) != 1 {
		t.Errorf("前台看到 %d 則，預期 1", len(page.Banners))
	}
}

// 更新時換成貼文目標：目標前後有空白也存正規形的 UUID（前台讀取才找得到），換成不可讀的貼文被拒，目標沒變（只差空白）時不再檢查貼文。
func TestUpdateBannerStoresTheCanonicalPostTarget(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	post := uuid.New()
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "", ""
	in.Action = content.BannerAction{Type: content.BannerActionPost, Target: "  " + post.String() + "\t"}
	_, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	wantKind(t, err, apperr.KindInvalidInput)

	f.social.set(post, true)
	b, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	if b.Action.Target != post.String() {
		t.Fatalf("存的目標 = %q，預期 %q", b.Action.Target, post.String())
	}
	page, err := f.m.ListLiveBanners(ctx, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Banners) != 1 {
		t.Fatalf("前台看到 %d 則，預期 1", len(page.Banners))
	}

	f.social.set(post, false)
	in.Action.Target = " " + post.String() + " "
	b, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatalf("目標沒變（只差空白）時不再檢查貼文：%v", err)
	}
	if b.Action.Target != post.String() {
		t.Fatalf("存的目標 = %q，預期 %q", b.Action.Target, post.String())
	}
}
