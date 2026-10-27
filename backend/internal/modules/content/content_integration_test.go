//go:build integration

package content_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/content"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// fakeMedia 在 media.Fake（引用語意）之上記錄轉正；staged token 以 "bad" 開頭時轉正失敗，以 "gone" 開頭時回
// media-service 的「token 無效」（InvalidInput，錯誤文字帶下游的訊息）。onFinalize 不為 nil 時，每次轉正前先呼叫它，
// 用來在「比對版本之後、寫入之前」插入別人的寫入。
type fakeMedia struct {
	*media.Fake
	mu         sync.Mutex
	finalized  []finalizeCall
	onFinalize func()
}

type finalizeCall struct {
	uploader media.Uploader
	token    string
	refType  string
}

func (f *fakeMedia) Finalize(ctx context.Context, u media.Uploader, token, refType string) (media.Asset, error) {
	if f.onFinalize != nil {
		f.onFinalize()
	}
	if len(token) >= 3 && token[:3] == "bad" {
		return media.Asset{}, errors.New("staged token 無效")
	}
	if len(token) >= 4 && token[:4] == "gone" {
		return media.Asset{}, fmt.Errorf("mediaservice：轉正（code=404）：staged token expired：%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("圖片無法使用，請重新上傳"))
	}
	f.mu.Lock()
	f.finalized = append(f.finalized, finalizeCall{u, token, refType})
	f.mu.Unlock()
	return f.Fake.Finalize(ctx, u, token, refType)
}

type fixture struct {
	m     *content.Module
	db    *db.DB
	media *fakeMedia
	now   time.Time
	newID func() uuid.UUID
}

const adminID = 9001
const activeBannerLimit = 20

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "content", Dir: content.Migrations()}))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: d, media: &fakeMedia{Fake: media.NewFake()}, now: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), newID: uuid.NewV7}
	f.m = content.New(content.Deps{
		DB:     d,
		Media:  f.media,
		Jobs:   jc,
		NewID:  func() uuid.UUID { return f.newID() },
		Now:    func() time.Time { return f.now },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return f
}

// sync 執行 ref 的同步 job 一輪。
func (f *fixture) sync(t *testing.T, ref media.Ref) media.SyncResult {
	t.Helper()
	res, err := f.m.SyncMedia(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func wantKind(t *testing.T, err error, want apperr.Kind) {
	t.Helper()
	if got := apperr.KindOf(err); got != want || err == nil {
		t.Fatalf("err = %v（分類 %v），預期分類 %v", err, got, want)
	}
}

func ptr[T any](v T) *T { return &v }

// ==================== banner ====================

func bannerInput() content.BannerInput {
	return content.BannerInput{
		Title:             "中秋禮盒",
		ButtonText:        "看看",
		PreviewImageToken: "p1",
		PopupImageToken:   "q1",
		Action:            content.BannerAction{Type: content.BannerActionProduct, Target: "42"},
		DisplayOrder:      1,
		IsActive:          true,
	}
}

func TestCreateBannerFinalizesBothImagesAndTheSyncJobLinksThem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	b, err := f.m.CreateBanner(context.Background(), adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	admin := media.Uploader{ID: adminID, Admin: true}
	want := []finalizeCall{{admin, "p1", media.RefTypeBanner}, {admin, "q1", media.RefTypeBanner}}
	if !slices.Equal(f.media.finalized, want) {
		t.Fatalf("轉正 = %+v", f.media.finalized)
	}
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("寫入不得建立引用：%v", got)
	}
	f.sync(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-p1 asset-q1]" {
		t.Fatalf("同步之後的引用 = %v", got)
	}
	got, err := f.m.Banner(context.Background(), b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PreviewImageKey != "public/7109/p1.jpg" || got.PopupImageKey != "public/7109/q1.jpg" {
		t.Errorf("圖片 = %q、%q", got.PreviewImageKey, got.PopupImageKey)
	}
	if got.Action != (content.BannerAction{Type: content.BannerActionProduct, Target: "42"}) {
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
			if len(f.media.finalized) != 0 {
				t.Fatalf("驗證失敗仍轉正了圖片：%+v", f.media.finalized)
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

func TestUpdateBannerRetiresExactlyTheReplacedImages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	f.sync(t, ref)

	// 只換預覽圖：只有被換掉的預覽圖退場，沿用的彈窗圖照舊。
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "p2", ""
	b, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	f.sync(t, ref)
	if got := f.media.Referenced(ref); fmt.Sprint(got) != "[asset-p2 asset-q1]" {
		t.Fatalf("只換預覽圖之後的引用 = %v", got)
	}

	// 兩張都換：舊的兩張都退場。
	in.PreviewImageToken, in.PopupImageToken = "p3", "q3"
	got, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	f.sync(t, ref)
	if refs := f.media.Referenced(ref); fmt.Sprint(refs) != "[asset-p3 asset-q3]" {
		t.Fatalf("兩張都換之後的引用 = %v", refs)
	}
	if got.PopupImageKey != "public/7109/q3.jpg" {
		t.Errorf("彈窗圖 = %q", got.PopupImageKey)
	}

	// 已退場的圖不能寫回：同一個 staged token 重新轉正會回到同一個資產。
	in.PreviewImageToken, in.PopupImageToken = "p2", ""
	_, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: got.Version, Banner: in})
	wantKind(t, err, apperr.KindInvalidInput)
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
	f.media.finalized = nil
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "", ""
	got, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	if err != nil {
		t.Fatal(err)
	}
	if got.PreviewImageKey != b.PreviewImageKey || got.PopupImageKey != b.PopupImageKey || len(f.media.finalized) != 0 {
		t.Fatalf("圖片 = %q、%q，轉正 %+v", got.PreviewImageKey, got.PopupImageKey, f.media.finalized)
	}
}

// media 拒絕 staged token 時，錯誤指出是哪一張圖，給使用者的文字由模組寫，下游的訊息只留在錯誤的文字裡。
func TestRejectedImageIsReportedOnItsField(t *testing.T) {
	t.Parallel()
	f := setup(t)
	in := bannerInput()
	in.PopupImageToken = "gone1"
	_, err := f.m.CreateBanner(context.Background(), adminID, in)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("err = %v，預期 InvalidInput", err)
	}
	if ae.Message != "彈窗圖無法使用，請重新上傳" || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "PopupImageToken" {
		t.Fatalf("文字 %q、欄位 %+v", ae.Message, ae.FieldViolations())
	}
	if !strings.Contains(err.Error(), "staged token expired") {
		t.Fatalf("下游的訊息要留在錯誤的文字裡給 log：%v", err)
	}
}

func TestUpdateBannerThatDoesNotExistTouchesNoMedia(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, err := f.m.UpdateBanner(context.Background(), content.UpdateBannerInput{AdminID: adminID, ID: uuid.NewV7(), Version: 1, Banner: bannerInput()})
	wantKind(t, err, apperr.KindNotFound)
	if len(f.media.finalized) != 0 {
		t.Fatalf("不存在的 banner 仍轉正了圖片：%+v", f.media.finalized)
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
	// 排程是 [開始, 結束)：開始的那一刻已上架，結束的那一刻已下架。
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
	finalized := len(f.media.finalized)
	_, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	wantKind(t, err, apperr.KindFailedPrecondition)
	if len(f.media.finalized) != finalized {
		t.Fatal("超過上限仍轉正了圖片")
	}

	// 下架的可以新增，但把它改成上架也受上限限制。
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

	// 已經上架的修改其他欄位不受影響；下架一則之後就能上架另一則。
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

func TestDeleteBannerRetiresItsImages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	ref := media.Ref{Type: media.RefTypeBanner, ID: b.ID.String()}
	f.sync(t, ref)
	if err := f.m.DeleteBanner(ctx, b.ID, 0); err != nil {
		t.Fatal(err)
	}
	f.sync(t, ref)
	if got := f.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("刪除之後引用應全部解除：%v", got)
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
	// 顯示順序相同時依建立時間。
	if want := []uuid.UUID{created[1], created[3], created[2], created[0]}; !slices.Equal(got, want) {
		t.Fatalf("逐頁讀完 = %v，預期 %v", got, want)
	}
}

// ==================== 版本（etag） ====================

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

	// 第二個人從同一個版本開始改，還換了圖：在轉正之前就被拒。
	f.media.finalized = nil
	in.Title = "第二個人改的"
	in.PreviewImageToken = "p2"
	_, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	wantKind(t, err, apperr.KindVersionConflict)
	if len(f.media.finalized) != 0 {
		t.Fatalf("版本已經不符，卻還轉正了圖：%+v", f.media.finalized)
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

// 比對版本之後、寫入之前有別人寫入：以條件式 UPDATE 為準，後寫入的一方被拒。
func TestUpdateBannerLosesToAWriteAfterItsVersionCheck(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	b, err := f.m.CreateBanner(ctx, adminID, bannerInput())
	if err != nil {
		t.Fatal(err)
	}
	f.media.onFinalize = func() {
		f.media.onFinalize = nil
		in := bannerInput()
		in.Title = "搶先寫入"
		in.PreviewImageToken, in.PopupImageToken = "", ""
		if _, err := f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in}); err != nil {
			t.Error(err)
		}
	}
	in := bannerInput()
	in.PreviewImageToken, in.PopupImageToken = "p2", ""
	_, err = f.m.UpdateBanner(ctx, content.UpdateBannerInput{AdminID: adminID, ID: b.ID, Version: b.Version, Banner: in})
	wantKind(t, err, apperr.KindVersionConflict)
	got, err := f.m.Banner(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "搶先寫入" || got.Version != b.Version+1 {
		t.Fatalf("banner = %q（版本 %d）", got.Title, got.Version)
	}
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

// ==================== 版本門檻 ====================

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
			 VALUES ($1, '排程', 'k', 'k', 'a', 'a', 'popup', $2, $3)`, uuid.NewV7(), start, end)
		if err != nil && !strings.Contains(err.Error(), "banners_schedule_check") {
			t.Fatalf("結束減開始 = %v：%v", end.Sub(start), err)
		}
		if dbOK := err == nil; dbOK != goOK {
			t.Errorf("結束減開始 = %v：模組判定 ok = %v，DB 判定 ok = %v", end.Sub(start), goOK, dbOK)
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

// ReadSecure 實作 media.Media；這些測試不讀機敏資產。
func (*fakeMedia) ReadSecure(context.Context, string, media.Ref) (media.SecureAsset, error) {
	return media.SecureAsset{}, errors.New("not used")
}
