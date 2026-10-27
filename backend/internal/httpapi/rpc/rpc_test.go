package rpc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	contentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func TestAudienceFollowsTheServiceName(t *testing.T) {
	cases := []struct {
		procedure string
		want      Audience
		ok        bool
	}{
		{"/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners", AudiencePublic, true},
		{"/hawkr.content.v1alpha1.ContentPublicService/", AudiencePublic, true},
		{"/hawkr.ordering.v1alpha1.OrderingAppService/GetOrder", AudienceApp, true},
		{"/hawkr.admin.content.v1alpha1.ContentAdminService/UpdateBanner", AudienceAdmin, true},
		{"/hawkr.agent.v1alpha1.VendorPortalInviteeService/GetSession", AudienceInvitee, true},
		{"/hawkr.agent.v1alpha1.VendorPortalInviteeService/", AudienceInvitee, true},
		{"/hawkr.admin.agent.v1alpha1.VendorPortalInviteeService/GetSession", 0, false},
		// 後台的 service 不在 hawkr.admin.*，或 hawkr.admin.* 裡放了別的 service：名稱錯了就不掛。
		{"/hawkr.content.v1alpha1.ContentAdminService/UpdateBanner", 0, false},
		{"/hawkr.admin.content.v1alpha1.ContentPublicService/ListLiveBanners", 0, false},
		{"/hawkr.content.v1alpha1.ContentService/GetBanner", 0, false},
	}
	for _, c := range cases {
		got, ok := AudienceOf(c.procedure)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("AudienceOf(%q) = %v, %v", c.procedure, got, ok)
		}
	}
}

func TestAuthorizeChecksTheCallerAgainstTheAudience(t *testing.T) {
	const (
		public = "/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners"
		app    = "/hawkr.ordering.v1alpha1.OrderingAppService/GetOrder"
		admin  = "/hawkr.admin.content.v1alpha1.ContentAdminService/GetBanner"
		invite = "/hawkr.agent.v1alpha1.VendorPortalInviteeService/GetSession"
	)
	buyer := &identity.Principal{Kind: identity.PrincipalAccount, ID: 7}
	staff := &identity.Principal{Kind: identity.PrincipalAdmin, ID: 7}
	cases := []struct {
		name      string
		procedure string
		caller    *identity.Principal
		want      connect.Code // 0 表示通過
	}{
		{"public 不需要身分", public, nil, 0},
		{"public 帶了身分也通過", public, buyer, 0},
		{"invitee 不讀身分，憑證在 service 自己的 interceptor", invite, nil, 0},
		{"invitee 帶了身分也通過", invite, buyer, 0},
		{"admin 沒有身分", admin, nil, connect.CodeUnauthenticated},
		{"admin 是買家", admin, buyer, connect.CodePermissionDenied},
		{"admin 是管理員", admin, staff, 0},
		{"app 沒有身分", app, nil, connect.CodeUnauthenticated},
		{"app 是帳號", app, buyer, 0},
		// 管理員與帳號的 ID 各自發號：同號的管理員不能當成帳號。
		{"app 是同號的管理員", app, staff, connect.CodePermissionDenied},
		{"不符合命名規則", "/hawkr.content.v1alpha1.ContentService/GetBanner", staff, connect.CodePermissionDenied},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.caller != nil {
				ctx = WithPrincipal(ctx, *c.caller)
			}
			err := authorize(ctx, c.procedure)
			if c.want == 0 && err != nil {
				t.Fatalf("err = %v", err)
			}
			if c.want != 0 && codeOf(err) != c.want {
				t.Fatalf("err = %v，預期 %v", err, c.want)
			}
		})
	}
}

// detailsOf 取出錯誤的 ErrorInfo.reason、LocalizedMessage 的文字與 BadRequest 的欄位路徑。
func detailsOf(t *testing.T, err *connect.Error) (reason, localized string, fields []string) {
	t.Helper()
	for _, d := range err.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatal(derr)
		}
		switch m := v.(type) {
		case *errdetails.ErrorInfo:
			if m.GetDomain() != "hawkr" {
				t.Fatalf("ErrorInfo 的 domain = %q", m.GetDomain())
			}
			reason = m.GetReason()
		case *errdetails.LocalizedMessage:
			if m.GetLocale() != "zh-TW" {
				t.Fatalf("LocalizedMessage 的 locale = %q", m.GetLocale())
			}
			localized = m.GetMessage()
		case *errdetails.BadRequest:
			for _, f := range m.GetFieldViolations() {
				if f.GetLocalizedMessage().GetMessage() == "" {
					t.Fatalf("欄位 %s 沒有給使用者的文字", f.GetField())
				}
				fields = append(fields, f.GetField())
			}
		}
	}
	return reason, localized, fields
}

func TestToConnectMapsErrors(t *testing.T) {
	downstream := connect.NewError(connect.CodeUnauthenticated, errors.New("media-service：token 無效"))
	cases := []struct {
		name      string
		err       error
		code      connect.Code
		reason    string
		localized string
		internal  bool
	}{
		{"InvalidInput", apperr.InvalidInput("標題必填"), connect.CodeInvalidArgument, "", "標題必填", false},
		{"包過一層的 NotFound", fmt.Errorf("讀取：%w", apperr.NotFound("食譜不存在")), connect.CodeNotFound, "", "食譜不存在", false},
		{"VersionConflict 帶業務原因", apperr.VersionConflict("已被修改"), connect.CodeAborted, "VERSION_CONFLICT", "已被修改", false},
		{"AlreadyExists 帶模組給的原因", apperr.AlreadyExists("已經有人建立").WithReason(apperr.ReasonVersionConflict),
			connect.CodeAlreadyExists, "VERSION_CONFLICT", "已經有人建立", false},
		{"AlreadyExists 沒有原因", apperr.AlreadyExists("名稱重複"), connect.CodeAlreadyExists, "", "名稱重複", false},
		{"FailedPrecondition", apperr.FailedPrecondition("已達上限"), connect.CodeFailedPrecondition, "", "已達上限", false},
		{"Unauthenticated", apperr.Unauthenticated("登入已失效"), connect.CodeUnauthenticated, "", "登入已失效", false},
		{"PermissionDenied", apperr.PermissionDenied("帳號已停權"), connect.CodePermissionDenied, "", "帳號已停權", false},
		{"Unavailable", apperr.Unavailable("此登入方式目前無法使用"), connect.CodeUnavailable, "", "此登入方式目前無法使用", false},
		{"ResourceExhausted", apperr.ResourceExhausted(0, "今天的次數已用完"), connect.CodeResourceExhausted, "", "今天的次數已用完", false},
		{"沒有分類：訊息固定、不外洩", errors.New("pq: connection refused 10.0.0.3"), connect.CodeInternal, "", "", true},
		// 下游 client 回的錯誤（例如 media-service 的 unauthenticated）不能原樣轉給 client：App 會把它當成
		// 自己的登入失效而登出使用者。
		{"下游的 Connect 錯誤變成 internal", fmt.Errorf("content：轉正圖片：%w", downstream), connect.CodeInternal, "", "", true},
		{"傳輸層的錯誤：只給開發者看", Error(connect.CodeInvalidArgument, "id %q 不是有效的 ID", "x"), connect.CodeInvalidArgument, "", "", false},
		{"傳輸層的錯誤：給使用者看", userError(connect.CodeUnauthenticated, CommonText(context.Background(), connect.CodeUnauthenticated)), connect.CodeUnauthenticated, "", "未登入", false},
		{"逾時", fmt.Errorf("查詢：%w", context.DeadlineExceeded), connect.CodeDeadlineExceeded, "", "服務暫時不可用", false},
		{"取消", fmt.Errorf("查詢：%w", context.Canceled), connect.CodeCanceled, "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toConnect(context.Background(), c.err)
			ce := got.err
			if ce.Code() != c.code || got.internal != c.internal {
				t.Fatalf("= %v（%q, internal %v）", ce.Code(), ce.Message(), got.internal)
			}
			reason, localized, _ := detailsOf(t, ce)
			if reason != c.reason || localized != c.localized {
				t.Fatalf("reason = %q、文字 = %q，預期 %q、%q", reason, localized, c.reason, c.localized)
			}
			if c.internal && ce.Message() != "系統錯誤" {
				t.Fatalf("internal 的訊息 = %q，內部細節外洩", ce.Message())
			}
		})
	}
	if got := toConnect(context.Background(), nil); got.err != nil {
		t.Fatal("nil 應回 nil")
	}
}

// ErrorInfo.metadata 只在 Metadata 有值時出現，沒有設定時是空的。
func TestMetadataReachesErrorInfoOnlyWhenSet(t *testing.T) {
	metadata := func(err error) map[string]string {
		t.Helper()
		for _, d := range toConnect(context.Background(), err).err.Details() {
			v, derr := d.Value()
			if derr != nil {
				t.Fatal(derr)
			}
			if info, ok := v.(*errdetails.ErrorInfo); ok {
				return info.GetMetadata()
			}
		}
		t.Fatal("沒有 ErrorInfo")
		return nil
	}
	with := apperr.FailedPrecondition("相符").WithReason(apperr.ReasonProspectMatchFound).WithMetadata("prospectIds", "3,7")
	if got := metadata(with); len(got) != 1 || got["prospectIds"] != "3,7" {
		t.Fatalf("metadata = %v", got)
	}
	if got := metadata(apperr.VersionConflict("過期")); len(got) != 0 {
		t.Fatalf("沒有設定 Metadata 的錯誤不應帶 metadata：%v", got)
	}
}

func TestResourceExhaustedCarriesRetryInfo(t *testing.T) {
	ce := toConnect(context.Background(), apperr.ResourceExhausted(45*time.Second, "請在 45 秒後再試")).err
	var delay time.Duration
	for _, d := range ce.Details() {
		v, err := d.Value()
		if err != nil {
			t.Fatal(err)
		}
		if ri, ok := v.(*errdetails.RetryInfo); ok {
			delay = ri.GetRetryDelay().AsDuration()
		}
	}
	if delay != 45*time.Second {
		t.Fatalf("RetryInfo = %v，預期 45s", delay)
	}
	for _, d := range toConnect(context.Background(), apperr.ResourceExhausted(0, "今天的次數已用完")).err.Details() {
		if v, _ := d.Value(); v != nil {
			if _, ok := v.(*errdetails.RetryInfo); ok {
				t.Fatal("不知道何時可以再試時不帶 RetryInfo")
			}
		}
	}
}

func TestFieldPathsMapModuleFields(t *testing.T) {
	paths := FieldPaths{
		"Title":              "banner.title",
		"Ingredients[].Name": "item.ingredients[].name",
		"Tags[]":             "item.tags[]",
		"Steps[].Photos[]":   "item.steps[].photos[]",
	}
	cases := []struct {
		name     string
		err      error
		want     []string
		unmapped []string
	}{
		{"對應到請求的路徑", WithFieldPaths(apperr.InvalidInputOn([]string{"Title"}, "不合法"), paths), []string{"banner.title"}, nil},
		{"索引依序填入", WithFieldPaths(apperr.InvalidInputOn([]string{"Ingredients[2].Name"}, "不合法"), paths), []string{"item.ingredients[2].name"}, nil},
		{"repeated 欄位的單一元素接上同一個索引", WithFieldPaths(apperr.InvalidInputOn([]string{"Tags[1]", "Tags"}, "不合法"), paths), []string{"item.tags[1]", "item.tags"}, nil},
		{"巢狀 repeated 的元素", WithFieldPaths(apperr.InvalidInputOn([]string{"Steps[3].Photos[0]"}, "不合法"), paths), []string{"item.steps[3].photos[0]"}, nil},
		{"單值欄位被回報成索引：沒有明列 repeated 目標就對應不到", WithFieldPaths(apperr.InvalidInputOn([]string{"Title[2]"}, "不合法"), paths), nil, []string{"Title[2]"}},
		{"元素的欄位沒有對應仍要回報", WithFieldPaths(apperr.InvalidInputOn([]string{"Servings[2]"}, "不合法"), paths), nil, []string{"Servings[2]"}},
		{"包在更外層也找得到", fmt.Errorf("外層：%w", WithFieldPaths(apperr.InvalidInputOn([]string{"Title"}, "不合法"), paths)), []string{"banner.title"}, nil},
		{"沒有對應的欄位要回報", WithFieldPaths(apperr.InvalidInputOn([]string{"Title", "Servings"}, "不合法"), paths), []string{"banner.title"}, []string{"Servings"}},
		{"handler 沒有包：全部回報", apperr.InvalidInputOn([]string{"Title"}, "不合法"), nil, []string{"Title"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toConnect(context.Background(), c.err)
			_, localized, fields := detailsOf(t, got.err)
			if localized != "不合法" {
				t.Fatalf("文字 = %q，對應不到欄位時也要有", localized)
			}
			if fmt.Sprint(fields) != fmt.Sprint(c.want) || fmt.Sprint(got.unmapped) != fmt.Sprint(c.unmapped) {
				t.Fatalf("欄位 = %v、沒對應到 %v，預期 %v、%v", fields, got.unmapped, c.want, c.unmapped)
			}
		})
	}
	if WithFieldPaths(nil, paths) != nil {
		t.Fatal("nil 應回 nil")
	}
}

// BadRequest 只屬於 invalid_argument：欄位只能在建立時帶、分類建立後不能改，所以其他分類不可能帶欄位，
// 也就不會有 BadRequest。
func TestOnlyInvalidArgumentCarriesFieldViolations(t *testing.T) {
	paths := FieldPaths{"EndsAt": "end_time"}
	got := toConnect(context.Background(), WithFieldPaths(apperr.InvalidInputOn([]string{"EndsAt"}, "不合法"), paths))
	_, _, fields := detailsOf(t, got.err)
	if fmt.Sprint(fields) != "[end_time]" || len(got.unmapped) != 0 {
		t.Fatalf("invalid_argument 欄位 = %v、unmapped = %v", fields, got.unmapped)
	}
}

func TestCheckFieldPathsFindsMissingPaths(t *testing.T) {
	desc := (&adminpb.ListBannersResponse{}).ProtoReflect().Descriptor()
	got := CheckFieldPaths(desc, FieldPaths{
		"Title":          "banners[].title",
		"Gone":           "banners[].no_such_field",
		"TooDeep":        "banners[].title.x",
		"Title[]":        "banners[].title[]",
		"Banners":        "banners",
		"Banners[].Bare": "banners.title",
	})
	if fmt.Sprint(got) != "[banners.title banners[].no_such_field banners[].title.x banners[].title[]]" {
		t.Fatalf("CheckFieldPaths = %v", got)
	}
}

// apperr 的 Reason 與契約的 ErrorReason 雙向一致（ADR 2026-09-25-api-error-model）：Go 宣告的每個 reason
// 都在 proto，proto 的每個值（UNSPECIFIED 除外）也都有 Go 的 reason。
func TestReasonsMatchTheContract(t *testing.T) {
	var goNames []string
	for _, r := range apperr.Reasons() {
		goNames = append(goNames, r.String())
		if n, ok := commonpb.ErrorReason_value[r.String()]; !ok || n == 0 {
			t.Errorf("apperr 的 %q 不在 api/ 的 ErrorReason", r)
		}
	}
	for name, n := range commonpb.ErrorReason_value {
		if n != 0 && !slices.Contains(goNames, name) {
			t.Errorf("ErrorReason 的 %s 在 apperr 沒有對應的 Reason", name)
		}
	}
}

func TestEnumsConvertByDroppingThePrefix(t *testing.T) {
	type appType string
	if got := EnumFromProto[appType](contentpb.AppType_APP_TYPE_SELLER); got != "SELLER" {
		t.Fatalf("EnumFromProto = %q", got)
	}
	if got := EnumFromProto[appType](contentpb.AppType_APP_TYPE_UNSPECIFIED); got != "" {
		t.Fatalf("UNSPECIFIED 轉成 %q，預期空字串", got)
	}
	if got := EnumFromProto[appType](contentpb.AppType(9)); got != "9" {
		t.Fatalf("不認得的數字轉成 %q，預期 \"9\"", got)
	}
	if got := EnumToProto[contentpb.AppType](appType("BUYER")); got != contentpb.AppType_APP_TYPE_BUYER {
		t.Fatalf("EnumToProto = %v", got)
	}
	if got := EnumToProto[contentpb.AppType](appType("WEB")); got != contentpb.AppType_APP_TYPE_UNSPECIFIED {
		t.Fatalf("契約沒有的值轉成 %v，預期 UNSPECIFIED", got)
	}

	if got := EnumMismatches[contentpb.AppType]([]appType{"BUYER", "SELLER"}); len(got) != 0 {
		t.Fatalf("一致時 = %v", got)
	}
	got := EnumMismatches[contentpb.AppType]([]appType{"BUYER", "WEB"})
	want := []string{
		`模組的 "WEB" 在 hawkr.content.v1alpha1.AppType 沒有對應的值`,
		"hawkr.content.v1alpha1.AppType 的 APP_TYPE_SELLER 不是模組的值",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("EnumMismatches = %q", got)
	}
}

func TestPageTokenIsBoundToItsScope(t *testing.T) {
	key := []byte{1, 2, 3}
	tok := EncodePageToken("/svc/List?cat=湯品", key)
	got, err := DecodePageToken("/svc/List?cat=湯品", tok)
	if err != nil || string(got) != string(key) {
		t.Fatalf("解回 = %v, %v", got, err)
	}
	if EncodePageToken("/svc/List", nil) != "" {
		t.Fatal("沒有下一頁時應回空字串")
	}
	if got, err := DecodePageToken("/svc/List", ""); got != nil || err != nil {
		t.Fatalf("空的 token 是第一頁：%v, %v", got, err)
	}
	for name, bad := range map[string]string{
		"換了查詢條件":    EncodePageToken("/svc/List?cat=甜點", key),
		"不是 base64": "***",
		"太短":        EncodePageToken("/svc/List?cat=湯品", key)[:4],
	} {
		if _, err := DecodePageToken("/svc/List?cat=湯品", bad); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：err = %v", name, err)
		}
	}
}

func TestApplyUpdateMask(t *testing.T) {
	created := timestamppb.New(timestamppb.Now().AsTime().Add(-time.Hour))
	cur := func() *adminpb.Banner {
		return &adminpb.Banner{
			Id: "b1", Title: "舊標題", ButtonText: "舊按鈕", Active: true, DisplayOrder: 3,
			StartTime: timestamppb.New(timestamppb.Now().AsTime()), CreateTime: created, Etag: "3",
			Action: &contentpb.BannerAction{Kind: &contentpb.BannerAction_Url{Url: "https://a"}},
		}
	}
	src := &adminpb.Banner{Id: "b1", Title: "新標題", Active: false, Etag: "3", PreviewImageStagedToken: "p2",
		Action: &contentpb.BannerAction{Kind: &contentpb.BannerAction_ProductId{ProductId: "p9"}}}

	t.Run("列出的欄位照 src，包括零值與沒有值", func(t *testing.T) {
		dst := cur()
		if err := ApplyUpdateMask(dst, src, &fieldmaskpb.FieldMask{Paths: []string{"active", "start_time", "action", "preview_image_staged_token"}}); err != nil {
			t.Fatal(err)
		}
		if dst.GetActive() || dst.GetStartTime() != nil || dst.GetAction().GetProductId() != "p9" || dst.GetTitle() != "舊標題" ||
			dst.GetPreviewImageStagedToken() != "p2" {
			t.Fatalf("= %v", dst)
		}
	})
	t.Run("沒帶 mask：只套 src 裡非零值的欄位", func(t *testing.T) {
		dst := cur()
		if err := ApplyUpdateMask(dst, src, nil); err != nil {
			t.Fatal(err)
		}
		if dst.GetTitle() != "新標題" || !dst.GetActive() || dst.GetButtonText() != "舊按鈕" || dst.GetAction().GetProductId() != "p9" {
			t.Fatalf("= %v", dst)
		}
	})
	t.Run("*：所有欄位，OUTPUT_ONLY 與 IDENTIFIER 除外", func(t *testing.T) {
		dst := cur()
		if err := ApplyUpdateMask(dst, src, &fieldmaskpb.FieldMask{Paths: []string{"*"}}); err != nil {
			t.Fatal(err)
		}
		want := proto.Clone(src).(*adminpb.Banner)
		want.CreateTime = created
		if !proto.Equal(dst, want) {
			t.Fatalf("= %v", dst)
		}
	})
	t.Run("OUTPUT_ONLY 與 IDENTIFIER 的欄位列在 mask 裡也不複製", func(t *testing.T) {
		dst := cur()
		other := &adminpb.Banner{Id: "b2", CreateTime: timestamppb.Now()}
		if err := ApplyUpdateMask(dst, other, &fieldmaskpb.FieldMask{Paths: []string{"id", "create_time"}}); err != nil {
			t.Fatal(err)
		}
		if dst.GetId() != "b1" || !proto.Equal(dst.GetCreateTime(), created) {
			t.Fatalf("= %v", dst)
		}
	})
	t.Run("IMMUTABLE 的欄位不同時回錯，相同時不動", func(t *testing.T) {
		gate := func() *adminpb.AppVersionGate {
			return &adminpb.AppVersionGate{AppType: contentpb.AppType_APP_TYPE_BUYER, MinimumVersion: "1.0.0"}
		}
		same := &adminpb.AppVersionGate{AppType: contentpb.AppType_APP_TYPE_BUYER, MinimumVersion: "2.0.0"}
		dst := gate()
		if err := ApplyUpdateMask(dst, same, &fieldmaskpb.FieldMask{Paths: []string{"app_type", "minimum_version"}}); err != nil || dst.GetMinimumVersion() != "2.0.0" {
			t.Fatalf("相同的值：err = %v、%v", err, dst)
		}
		changed := &adminpb.AppVersionGate{AppType: contentpb.AppType_APP_TYPE_SELLER}
		if err := ApplyUpdateMask(gate(), changed, &fieldmaskpb.FieldMask{Paths: []string{"app_type"}}); codeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("改了 IMMUTABLE 的欄位：err = %v", err)
		}
	})
	for _, p := range []string{"no_such_field", "action.url", "buttonText"} {
		if err := ApplyUpdateMask(cur(), src, &fieldmaskpb.FieldMask{Paths: []string{p}}); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("路徑 %q：err = %v", p, err)
		}
	}
}

// codeOf 是 client 收到的錯誤碼：經過唯一的轉換點之後的 code。
func codeOf(err error) connect.Code {
	ce := toConnect(context.Background(), err).err
	if ce == nil {
		return 0
	}
	return ce.Code()
}

// LocalizedMessage 的 locale 取自文字本身（實際使用的語系），不接受另外傳入。
func TestLocalizedMessageCarriesTheMessageLocale(t *testing.T) {
	for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
		got := LocalizedMessage(uitext.Message{Text: "文字", Locale: l})
		if got.GetLocale() != string(l) || got.GetMessage() != "文字" {
			t.Errorf("%s：%v", l, got)
		}
	}
}

// 傳輸層的文字只有共用文案目錄一個來源；請求語系是 en 但目錄沒有 en 的文字時，LocalizedMessage 標的是實際使用的 zh-TW，不是 en。
func TestTransportTextComesFromTheCatalogWithTheResolvedLocale(t *testing.T) {
	want := map[connect.Code]string{
		connect.CodeUnauthenticated:   "未登入",
		connect.CodePermissionDenied:  "無權操作",
		connect.CodeUnavailable:       "服務暫時不可用",
		connect.CodeInternal:          "系統錯誤",
		connect.CodeDeadlineExceeded:  "服務暫時不可用",
		connect.CodeResourceExhausted: "操作過於頻繁，請稍後再試",
	}
	ctx := uitext.WithLocale(context.Background(), uitext.EN)
	for code, text := range want {
		m := CommonText(ctx, code)
		if m.Text != text {
			t.Errorf("%v：文字 %q，want %q", code, m.Text, text)
		}
		zh, _ := uitext.For(uitext.ZhTW).CommonError(strings.ToUpper(code.String()))
		if m.Text == zh.Text && m.Locale != uitext.ZhTW {
			t.Errorf("%v：文字是 zh-TW 的字，locale 卻標成 %q", code, m.Locale)
		}
	}
	ce := toConnect(ctx, userError(connect.CodeUnauthenticated, CommonText(ctx, connect.CodeUnauthenticated))).err
	var lm *errdetails.LocalizedMessage
	for _, d := range ce.Details() {
		if v, err := d.Value(); err == nil {
			if m, ok := v.(*errdetails.LocalizedMessage); ok {
				lm = m
			}
		}
	}
	if lm == nil || lm.GetLocale() != "zh-TW" || lm.GetMessage() != "未登入" {
		t.Errorf("LocalizedMessage = %v", lm)
	}
	ce = toConnect(ctx, fmt.Errorf("查詢：%w", context.DeadlineExceeded)).err
	for _, d := range ce.Details() {
		if v, err := d.Value(); err == nil {
			if m, ok := v.(*errdetails.LocalizedMessage); ok && m.GetLocale() != "zh-TW" {
				t.Errorf("逾時的 LocalizedMessage locale = %q", m.GetLocale())
			}
		}
	}
}
