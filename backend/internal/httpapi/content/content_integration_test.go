//go:build integration

package content_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	"github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1"
	adminconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1/contentv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	contentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1/contentv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// fakeMedia 讓每個 staged token 都轉正成功，object key 由 ref 與 token 組成；以 "gone" 開頭的 token 回
// media-service 的「token 無效」。
type fakeMedia struct{}

func (f *fakeMedia) Finalize(_ context.Context, _ media.Uploader, token, refType string) (media.Asset, error) {
	if strings.HasPrefix(token, "gone") {
		return media.Asset{}, fmt.Errorf("%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("圖片無法使用，請重新上傳"))
	}
	return media.Asset{ID: "asset-" + token, ObjectKey: "public/" + refType + "/" + token + ".jpg"}, nil
}

func (f *fakeMedia) Link(context.Context, media.Ref, []string) error   { return nil }
func (f *fakeMedia) Unlink(context.Context, media.Ref, []string) error { return nil }

type fixture struct {
	url    string
	public contentv1alpha1connect.ContentPublicServiceClient
	admin  adminconnect.ContentAdminServiceClient
	// adminToken、buyerToken 是管理員與買家的 session（以 Authorization: Bearer 帶）。
	adminToken string
	buyerToken string
}

// setup 起完整的 HTTP 層（httpapi.NewHandler），client 以 JSON 呼叫，讀取走 GET，與 web 相同。
// 後台的 client 帶著管理員的 dev token（第一個請求時建立管理員）。
func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "content", Dir: content.Migrations()},
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
	))
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := content.New(content.Deps{DB: d, Media: &fakeMedia{}, Jobs: jc, NewID: uuid.NewV7, Now: time.Now, Logger: log})
	dev, err := identity.NewDevAuth(nil)
	if err != nil {
		t.Fatal(err)
	}
	id := identity.New(identity.Deps{
		DB: d, Events: events.NewBus(), Jobs: jc, Now: time.Now, Logger: log,
		Providers: identity.Providers{Customers: []identity.Clerk{dev.Customers()}, Admin: dev.Admins()},
	})
	admin, err := dev.AdminToken("dev_ops")
	if err != nil {
		t.Fatal(err)
	}
	characterize.GrantAdminOwner(t, id, admin)
	buyer, err := id.DevSession(t.Context(), "dev_buyer")
	if err != nil {
		t.Fatal(err)
	}
	images, err := rpc.NewImages(mediaOrigin)
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpapi.NewHandler(httpapi.Deps{Social: &social.Module{}, Ready: d.Ping, Content: m, Identity: id, Deletion: &accountdeletion.Module{}, Seller: &seller.Module{}, Stats: characterize.Stats(t), WebOrigin: characterize.WebOrigin(t), DashboardOrders: noDashboardOrders{}, Discovery: &discovery.Module{}, Agent: &agent.Module{}, ReviewQueue: &reviewqueue.Module{}, Delegation: delegation.New(delegation.Deps{DB: d, Identity: id, Seller: &seller.Module{}, Now: time.Now, Logger: log}), Images: images, Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &fixture{
		url:        srv.URL,
		public:     contentv1alpha1connect.NewContentPublicServiceClient(srv.Client(), srv.URL, connect.WithProtoJSON(), connect.WithHTTPGet()),
		admin:      adminClient(srv.URL, admin),
		adminToken: admin,
		buyerToken: buyer,
	}
}

// adminClient 是後台的 client；token 為空表示沒有登入。
func adminClient(base, token string) adminconnect.ContentAdminServiceClient {
	session := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			return next(ctx, req)
		}
	})
	return adminconnect.NewContentAdminServiceClient(http.DefaultClient, base,
		connect.WithProtoJSON(), connect.WithHTTPGet(), connect.WithInterceptors(session))
}

// mediaOrigin 是測試的媒體網域；回應的圖片以它開頭。
const mediaOrigin = "https://media.example.test"

// errorDetails 是 client 從錯誤取得的 details。
type errorDetails struct {
	localized string   // LocalizedMessage 的文字
	fields    []string // BadRequest 的欄位路徑
}

// wantError 確認錯誤的 Connect 碼與 ErrorInfo 的業務原因（空字串表示不該有 ErrorInfo），回傳其他 details。
func wantError(t *testing.T, err error, code connect.Code, reason string) errorDetails {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != code {
		t.Fatalf("err = %v，預期 %v", err, code)
	}
	got := ""
	var out errorDetails
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatal(derr)
		}
		switch m := v.(type) {
		case *errdetails.ErrorInfo:
			got = m.GetReason()
		case *errdetails.LocalizedMessage:
			out.localized = m.GetMessage()
		case *errdetails.BadRequest:
			for _, f := range m.GetFieldViolations() {
				out.fields = append(out.fields, f.GetField())
			}
		}
	}
	if got != reason {
		t.Fatalf("業務原因 = %q，預期 %q", got, reason)
	}
	return out
}

// 模組的驗證錯誤帶給使用者的文字，並以請求裡的路徑指出錯的欄位（ADR 2026-09-25-api-error-model D3、D4）；
// 認證錯誤帶文字；client 的錯（沒帶 etag）只給開發者看。
func TestErrorsCarryUserTextAndFieldPaths(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()

	noTitle := newBanner()
	noTitle.Title = " "
	_, err := f.admin.CreateBanner(ctx, connect.NewRequest(&adminpb.CreateBannerRequest{Banner: noTitle}))
	d := wantError(t, err, connect.CodeInvalidArgument, "")
	if d.localized != "標題 必填" || strings.Join(d.fields, ",") != "banner.title" {
		t.Fatalf("標題空白：文字 %q、欄位 %v", d.localized, d.fields)
	}

	noImages := newBanner()
	noImages.PreviewImageStagedToken, noImages.PopupImageStagedToken = "", ""
	_, err = f.admin.CreateBanner(ctx, connect.NewRequest(&adminpb.CreateBannerRequest{Banner: noImages}))
	d = wantError(t, err, connect.CodeInvalidArgument, "")
	if strings.Join(d.fields, ",") != "banner.preview_image_staged_token,banner.popup_image_staged_token" {
		t.Fatalf("沒有圖：欄位 %v", d.fields)
	}

	_, err = f.public.CheckAppVersion(ctx, connect.NewRequest(&contentpb.CheckAppVersionRequest{
		ClientVersion: "1.0.0"}))
	d = wantError(t, err, connect.CodeInvalidArgument, "")
	if strings.Join(d.fields, ",") != "app_type" {
		t.Fatalf("沒帶 App 類型：欄位 %v", d.fields)
	}

	_, err = adminClient(f.url, "").ListBanners(ctx, connect.NewRequest(&adminpb.ListBannersRequest{}))
	if d = wantError(t, err, connect.CodeUnauthenticated, ""); d.localized != "未登入" {
		t.Fatalf("未登入的文字 = %q", d.localized)
	}

	_, err = f.admin.UpdateBanner(ctx, connect.NewRequest(&adminpb.UpdateBannerRequest{Banner: &adminpb.Banner{Id: "0199a1c2-0000-7000-8000-000000000000"}}))
	if d = wantError(t, err, connect.CodeInvalidArgument, ""); d.localized != "" {
		t.Fatalf("沒帶 etag 是 client 的錯，不該有給使用者的文字：%q", d.localized)
	}
}

// 模組回報的每個欄位都對應到請求的路徑（ADR 2026-09-25-api-error-model D4）：對應表漏了一個欄位，client
// 就只拿到摘要文字、標不出欄位。
func TestEveryModuleFieldMapsToARequestPath(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	banner := func(mutate func(*adminpb.Banner)) error {
		b := newBanner()
		mutate(b)
		_, err := f.admin.CreateBanner(ctx, connect.NewRequest(&adminpb.CreateBannerRequest{Banner: b}))
		return err
	}
	gate := func(mutate func(*adminpb.AppVersionGate)) error {
		g := &adminpb.AppVersionGate{AppType: contentpb.AppType_APP_TYPE_SELLER,
			MinimumVersion: "1.2.0", StoreUrl: "itms-apps://apps.apple.com/app/id1"}
		mutate(g)
		_, err := f.admin.CreateAppVersionGate(ctx, connect.NewRequest(&adminpb.CreateAppVersionGateRequest{AppVersionGate: g}))
		return err
	}
	check := func(mutate func(*contentpb.CheckAppVersionRequest)) error {
		req := &contentpb.CheckAppVersionRequest{AppType: contentpb.AppType_APP_TYPE_BUYER, ClientVersion: "1.0.0"}
		mutate(req)
		_, err := f.public.CheckAppVersion(ctx, connect.NewRequest(req))
		return err
	}
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"banner 標題", banner(func(b *adminpb.Banner) { b.Title = "" }), "banner.title"},
		{"banner 按鈕文字", banner(func(b *adminpb.Banner) { b.ButtonText = "一二三四五六七八九十一二三" }), "banner.button_text"},
		{"banner 結束時間", banner(func(b *adminpb.Banner) {
			b.StartTime, b.EndTime = timestamppb.New(time.Now()), timestamppb.New(time.Now().Add(-time.Hour))
		}), "banner.end_time"},
		{"banner 沒有點擊行為", banner(func(b *adminpb.Banner) { b.Action = nil }), "banner.action"},
		{"banner 行為的網址", banner(func(b *adminpb.Banner) {
			b.Action = &contentpb.BannerAction{Kind: &contentpb.BannerAction_Url{Url: "ftp://x"}}
		}), "banner.action.url"},
		{"banner 行為的商品", banner(func(b *adminpb.Banner) {
			b.Action = &contentpb.BannerAction{Kind: &contentpb.BannerAction_ProductId{}}
		}), "banner.action.product_id"},
		{"banner 行為的貼文", banner(func(b *adminpb.Banner) {
			b.Action = &contentpb.BannerAction{Kind: &contentpb.BannerAction_PostId{}}
		}), "banner.action.post_id"},
		{"banner 被拒的圖", banner(func(b *adminpb.Banner) { b.PopupImageStagedToken = "gone1" }), "banner.popup_image_staged_token"},
		{"門檻的 App 類型", gate(func(g *adminpb.AppVersionGate) { g.AppType = 0 }), "app_version_gate.app_type"},
		{"最低版本", gate(func(g *adminpb.AppVersionGate) { g.MinimumVersion = "1.2" }), "app_version_gate.minimum_version"},
		{"商店網址", gate(func(g *adminpb.AppVersionGate) { g.StoreUrl = "javascript:alert(1)" }), "app_version_gate.store_url"},
		{"檢查版本的 App 版本", check(func(r *contentpb.CheckAppVersionRequest) { r.ClientVersion = "" }), "client_version"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := wantError(t, c.err, connect.CodeInvalidArgument, "")
			if strings.Join(d.fields, ",") != c.want || d.localized == "" {
				t.Fatalf("欄位 %v、文字 %q，預期 %s", d.fields, d.localized, c.want)
			}
		})
	}
}

// proto 上標 REQUIRED 的欄位（AIP-203），模組真的會擋：從一份合法的請求清掉那個欄位，錯誤要指出它的路徑。
// 標了卻沒擋，或擋了別的欄位，這裡都會失敗。
func TestRequiredFieldsAreEnforced(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	cases := []struct {
		valid func() proto.Message
		call  func(proto.Message) error
	}{
		{func() proto.Message { return &adminpb.CreateBannerRequest{Banner: newBanner()} },
			func(m proto.Message) error {
				_, err := f.admin.CreateBanner(ctx, connect.NewRequest(m.(*adminpb.CreateBannerRequest)))
				return err
			}},
		{func() proto.Message {
			return &adminpb.CreateAppVersionGateRequest{AppVersionGate: &adminpb.AppVersionGate{AppType: contentpb.AppType_APP_TYPE_SELLER, MinimumVersion: "1.2.0"}}
		}, func(m proto.Message) error {
			_, err := f.admin.CreateAppVersionGate(ctx, connect.NewRequest(m.(*adminpb.CreateAppVersionGateRequest)))
			return err
		}},
		{func() proto.Message {
			return &contentpb.CheckAppVersionRequest{AppType: contentpb.AppType_APP_TYPE_BUYER, ClientVersion: "1.0.0"}
		}, func(m proto.Message) error {
			_, err := f.public.CheckAppVersion(ctx, connect.NewRequest(m.(*contentpb.CheckAppVersionRequest)))
			return err
		}},
	}
	checked := 0
	for _, c := range cases {
		desc := c.valid().ProtoReflect().Descriptor()
		for _, path := range requiredPaths(desc) {
			want := joinNames(path)
			t.Run(string(desc.Name())+"."+want, func(t *testing.T) {
				m := c.valid()
				clearPath(m.ProtoReflect(), path)
				d := wantError(t, c.call(m), connect.CodeInvalidArgument, "")
				if strings.Join(d.fields, ",") != want {
					t.Fatalf("欄位 %v，預期 %s", d.fields, want)
				}
			})
			checked++
		}
	}
	// 下限是目前的實際數量（2026-10-27 刪掉食譜後是 6）。
	if checked < 6 {
		t.Fatalf("只找到 %d 個 REQUIRED 欄位，descriptor 的讀取可能失效", checked)
	}
}

// requiredPaths 回傳訊息裡標了 REQUIRED 的欄位路徑。欄位是訊息、而且裡面也有 REQUIRED 的欄位時往下找，
// 否則這個欄位本身就是一條路徑。
func requiredPaths(desc protoreflect.MessageDescriptor) [][]protoreflect.Name {
	var out [][]protoreflect.Name
	fields := desc.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if !rpc.HasBehavior(fd, annotations.FieldBehavior_REQUIRED) {
			continue
		}
		if fd.Kind() == protoreflect.MessageKind && !fd.IsList() && !fd.IsMap() {
			if sub := requiredPaths(fd.Message()); len(sub) > 0 {
				for _, p := range sub {
					out = append(out, append([]protoreflect.Name{fd.Name()}, p...))
				}
				continue
			}
		}
		out = append(out, []protoreflect.Name{fd.Name()})
	}
	return out
}

func joinNames(path []protoreflect.Name) string {
	parts := make([]string, len(path))
	for i, n := range path {
		parts[i] = string(n)
	}
	return strings.Join(parts, ".")
}

func clearPath(m protoreflect.Message, path []protoreflect.Name) {
	for _, name := range path[:len(path)-1] {
		m = m.Mutable(m.Descriptor().Fields().ByName(name)).Message()
	}
	m.Clear(m.Descriptor().Fields().ByName(path[len(path)-1]))
}

// wantImage 確認圖片是媒體網域上、只有 {preset} 一個變數的樣板，key 以 suffix 結尾。
func wantImage(t *testing.T, img *commonpb.Image, suffix string) {
	t.Helper()
	tpl := img.GetUriTemplate()
	if !strings.HasPrefix(tpl, mediaOrigin+"/i/{preset}/") || !strings.HasSuffix(tpl, suffix) || strings.Count(tpl, "{") != 1 {
		t.Fatalf("uri_template = %q，預期 %s/i/{preset}/…%s", tpl, mediaOrigin, suffix)
	}
}

func newBanner() *adminpb.Banner {
	return &adminpb.Banner{
		Title:                   "中秋禮盒",
		ButtonText:              "看看",
		PreviewImageStagedToken: "p1",
		PopupImageStagedToken:   "q1",
		CanvasJson:              `{"objects":[]}`,
		Action:                  &contentpb.BannerAction{Kind: &contentpb.BannerAction_ProductId{ProductId: "42"}},
		DisplayOrder:            1,
		Active:                  true,
	}
}

func mask(paths ...string) *fieldmaskpb.FieldMask { return &fieldmaskpb.FieldMask{Paths: paths} }

func TestAdminServiceRequiresAnAdmin(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for name, c := range map[string]struct {
		client adminconnect.ContentAdminServiceClient
		code   connect.Code
		reason string
	}{
		"沒有身分": {adminClient(f.url, ""), connect.CodeUnauthenticated, ""},
		"買家":   {adminClient(f.url, f.buyerToken), connect.CodePermissionDenied, ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.client.ListBanners(ctx, connect.NewRequest(&adminpb.ListBannersRequest{}))
			wantError(t, err, c.code, c.reason)
		})
	}
}

// banner 的完整流程：建立、部分更新、版本衝突、換圖、刪除，前台只看得到該看的。
func TestBannerLifecycle(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	created, err := f.admin.CreateBanner(ctx, connect.NewRequest(&adminpb.CreateBannerRequest{Banner: newBanner()}))
	if err != nil {
		t.Fatal(err)
	}
	b := created.Msg.GetBanner()
	if b.GetEtag() != "1" || b.GetPreviewImageStagedToken() != "" || b.GetCreateTime() == nil {
		t.Fatalf("建立 = %v", b)
	}
	wantImage(t, b.GetPreviewImage(), "/p1.jpg")
	wantImage(t, b.GetPopupImage(), "/q1.jpg")

	live, err := f.public.ListLiveBanners(ctx, connect.NewRequest(&contentpb.ListLiveBannersRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(live.Msg.GetBanners()) != 1 || live.Msg.GetBanners()[0].GetAction().GetProductId() != "42" {
		t.Fatalf("前台 = %v", live.Msg)
	}

	// 只停用：其他欄位不變，etag 加一。
	off, err := f.admin.UpdateBanner(ctx, connect.NewRequest(&adminpb.UpdateBannerRequest{
		Banner:     &adminpb.Banner{Id: b.GetId(), Etag: b.GetEtag(), Active: false},
		UpdateMask: mask("active"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := off.Msg.GetBanner(); got.GetActive() || got.GetTitle() != "中秋禮盒" || got.GetEtag() != "2" || got.GetCanvasJson() != `{"objects":[]}` {
		t.Fatalf("停用後 = %v", got)
	}
	live, err = f.public.ListLiveBanners(ctx, connect.NewRequest(&contentpb.ListLiveBannersRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(live.Msg.GetBanners()) != 0 {
		t.Fatalf("停用的 banner 還在前台：%v", live.Msg)
	}

	// 從舊版本改：版本衝突，不寫入。
	_, err = f.admin.UpdateBanner(ctx, connect.NewRequest(&adminpb.UpdateBannerRequest{
		Banner: &adminpb.Banner{Id: b.GetId(), Etag: b.GetEtag(), Title: "晚了一步"}, UpdateMask: mask("title"),
	}))
	wantError(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	_, err = f.admin.UpdateBanner(ctx, connect.NewRequest(&adminpb.UpdateBannerRequest{
		Banner: &adminpb.Banner{Id: b.GetId(), Title: "沒帶 etag"}, UpdateMask: mask("title"),
	}))
	wantError(t, err, connect.CodeInvalidArgument, "")

	// 只換預覽圖：彈窗圖沿用。
	swapped, err := f.admin.UpdateBanner(ctx, connect.NewRequest(&adminpb.UpdateBannerRequest{
		Banner:     &adminpb.Banner{Id: b.GetId(), Etag: "2", PreviewImageStagedToken: "p2"},
		UpdateMask: mask("preview_image_staged_token"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := swapped.Msg.GetBanner(); !strings.HasSuffix(got.GetPreviewImage().GetUriTemplate(), "/p2.jpg") || got.GetPopupImage().GetUriTemplate() != b.GetPopupImage().GetUriTemplate() {
		t.Fatalf("換圖後 = %v", got)
	}

	_, err = f.admin.DeleteBanner(ctx, connect.NewRequest(&adminpb.DeleteBannerRequest{Id: b.GetId(), Etag: "2"}))
	wantError(t, err, connect.CodeAborted, "VERSION_CONFLICT")
	if _, err := f.admin.DeleteBanner(ctx, connect.NewRequest(&adminpb.DeleteBannerRequest{Id: b.GetId(), Etag: "3"})); err != nil {
		t.Fatal(err)
	}
	_, err = f.admin.GetBanner(ctx, connect.NewRequest(&adminpb.GetBannerRequest{Id: b.GetId()}))
	wantError(t, err, connect.CodeNotFound, "")
	_, err = f.admin.GetBanner(ctx, connect.NewRequest(&adminpb.GetBannerRequest{Id: "42"}))
	wantError(t, err, connect.CodeInvalidArgument, "")

	_, err = f.admin.CreateBanner(ctx, connect.NewRequest(&adminpb.CreateBannerRequest{Banner: &adminpb.Banner{
		Title: "沒有點擊行為", PreviewImageStagedToken: "p", PopupImageStagedToken: "q",
	}}))
	wantError(t, err, connect.CodeInvalidArgument, "")
}

func TestAppVersionGates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	gate := &adminpb.AppVersionGate{AppType: contentpb.AppType_APP_TYPE_BUYER, MinimumVersion: "1.2.0", StoreUrl: "itms-apps://apps.apple.com/app/id1"}
	created, err := f.admin.CreateAppVersionGate(ctx, connect.NewRequest(&adminpb.CreateAppVersionGateRequest{AppVersionGate: gate}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.admin.CreateAppVersionGate(ctx, connect.NewRequest(&adminpb.CreateAppVersionGateRequest{AppVersionGate: gate}))
	wantError(t, err, connect.CodeAlreadyExists, "VERSION_CONFLICT")

	updated, err := f.admin.UpdateAppVersionGate(ctx, connect.NewRequest(&adminpb.UpdateAppVersionGateRequest{
		AppVersionGate: &adminpb.AppVersionGate{AppType: contentpb.AppType_APP_TYPE_BUYER, MinimumVersion: "1.3.0", Etag: created.Msg.GetAppVersionGate().GetEtag()},
		UpdateMask:     mask("minimum_version"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if g := updated.Msg.GetAppVersionGate(); g.GetMinimumVersion() != "1.3.0" || g.GetStoreUrl() == "" || g.GetEtag() != "2" {
		t.Fatalf("修改後 = %v", g)
	}
	check, err := f.public.CheckAppVersion(ctx, connect.NewRequest(&contentpb.CheckAppVersionRequest{AppType: contentpb.AppType_APP_TYPE_BUYER, ClientVersion: "1.2.9"}))
	if err != nil {
		t.Fatal(err)
	}
	if !check.Msg.GetUpdateRequired() || check.Msg.GetMinimumVersion() != "1.3.0" {
		t.Fatalf("版本檢查 = %v", check.Msg)
	}
	list, err := f.admin.ListAppVersionGates(ctx, connect.NewRequest(&adminpb.ListAppVersionGatesRequest{}))
	if err != nil || len(list.Msg.GetAppVersionGates()) != 1 {
		t.Fatalf("清單 = %v, %v", list, err)
	}
}

// 公開讀取以 GET 呼叫：public, no-cache 加上 ETag，條件請求沒變時回 304；後台一律 private, no-store。
func TestHTTPCaching(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	type responseMeta struct {
		StatusCode int
		Header     http.Header
	}
	get := func(procedure, ifNoneMatch string, admin bool) responseMeta {
		t.Helper()
		u := f.url + procedure + "?" + url.Values{"connect": {"v1"}, "encoding": {"json"}, "message": {"{}"}}.Encode()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, u, nil)
		if err != nil {
			t.Fatal(err)
		}
		if ifNoneMatch != "" {
			req.Header.Set("If-None-Match", ifNoneMatch)
		}
		if admin {
			req.Header.Set("Authorization", "Bearer "+f.adminToken)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Fatal(err)
		}
		return responseMeta{StatusCode: resp.StatusCode, Header: resp.Header}
	}
	const live = contentv1alpha1connect.ContentPublicServiceListLiveBannersProcedure

	first := get(live, "", false)
	tag := first.Header.Get("ETag")
	if first.StatusCode != http.StatusOK || first.Header.Get("Cache-Control") != "public, no-cache" || !strings.HasPrefix(tag, `W/"`) {
		t.Fatalf("第一次 = %d，Cache-Control %q，ETag %q", first.StatusCode, first.Header.Get("Cache-Control"), tag)
	}
	// connect-go 對 GET 已經加上 Vary，cachePolicy 不再重複一次。
	if vary := first.Header.Values("Vary"); len(vary) != 1 || vary[0] != "Accept-Encoding" {
		t.Fatalf("Vary = %q，預期只有一個 Accept-Encoding", vary)
	}
	if again := get(live, tag, false); again.StatusCode != http.StatusNotModified || again.Header.Get("ETag") != tag {
		t.Fatalf("內容沒變 = %d（ETag %q）", again.StatusCode, again.Header.Get("ETag"))
	}
	if _, err := f.admin.CreateBanner(ctx, connect.NewRequest(&adminpb.CreateBannerRequest{Banner: newBanner()})); err != nil {
		t.Fatal(err)
	}
	if changed := get(live, tag, false); changed.StatusCode != http.StatusOK || changed.Header.Get("ETag") == tag {
		t.Fatalf("內容變了 = %d（ETag %q）", changed.StatusCode, changed.Header.Get("ETag"))
	}

	admin := get(adminconnect.ContentAdminServiceListBannersProcedure, "", true)
	if admin.StatusCode != http.StatusOK || admin.Header.Get("Cache-Control") != "private, no-store" || admin.Header.Get("ETag") != "" {
		t.Fatalf("後台 = %d，Cache-Control %q，ETag %q", admin.StatusCode, admin.Header.Get("Cache-Control"), admin.Header.Get("ETag"))
	}
	unauth := get(adminconnect.ContentAdminServiceListBannersProcedure, "", false)
	if unauth.StatusCode != http.StatusUnauthorized || unauth.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("沒有身分 = %d，Cache-Control %q", unauth.StatusCode, unauth.Header.Get("Cache-Control"))
	}
}

func TestTraceIDAndRequestLimit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	// 單體是入口：不接續 client 帶來的 traceparent，自己開新的 trace，錯誤的回應也帶著它。
	const clientTrace = "4bf92f3577b34da6a3ce929d0e0e4736"
	req := connect.NewRequest(&contentpb.CheckAppVersionRequest{})
	req.Header().Set("traceparent", "00-"+clientTrace+"-00f067aa0ba902b7-01")
	_, err := f.public.CheckAppVersion(ctx, req)
	var ce *connect.Error
	if !errors.As(err, &ce) || len(ce.Meta().Get("X-Trace-Id")) != 32 || ce.Meta().Get("X-Trace-Id") == clientTrace {
		t.Fatalf("錯誤的回應要帶入口自己的 trace id：%v（%q）", err, ce.Meta().Get("X-Trace-Id"))
	}
	resp, err := f.public.ListLiveBanners(ctx, connect.NewRequest(&contentpb.ListLiveBannersRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Header().Get("X-Trace-Id"); len(got) != 32 {
		t.Fatalf("沒帶 trace id 時自己產生：%q", got)
	}

	big := newBanner()
	big.CanvasJson = strings.Repeat("x", 3<<20)
	_, err = f.admin.CreateBanner(ctx, connect.NewRequest(&adminpb.CreateBannerRequest{Banner: big}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("超過 2 MiB 的請求：err = %v", err)
	}
}

// noDashboardOrders 是這個測試不讀儀表板的訂單時的替身：NewHandler 要求提供儀表板訂單的來源。
type noDashboardOrders struct{}

func (noDashboardOrders) SalesStats(context.Context, int64, []httpseller.SalesWindow) ([]httpseller.SalesStats, error) {
	return nil, errors.New("not used")
}

func (noDashboardOrders) StatusCounts(context.Context, int64) (httpseller.OrderStatusCounts, error) {
	return httpseller.OrderStatusCounts{}, errors.New("not used")
}

// ReadSecure 實作 media.Media；這些測試不讀機敏資產。
func (*fakeMedia) ReadSecure(context.Context, string, media.Ref) (media.SecureAsset, error) {
	return media.SecureAsset{}, errors.New("not used")
}
