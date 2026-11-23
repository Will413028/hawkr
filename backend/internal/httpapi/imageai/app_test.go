package imageai

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/imageai/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/imageai/v1alpha1/imageaiv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	sellerAccount = int64(7)
	storeID       = int64(4242)
)

type stores struct{}

func (stores) CurrentStore(_ context.Context, account int64) (seller.Store, error) {
	if account == sellerAccount {
		return seller.Store{StoreID: storeID}, nil
	}
	return seller.Store{}, apperr.NotFound("沒有店家")
}

type fakeModule struct {
	transformIn  []imageai.TransformInput
	transformOut imageai.TransformOutput
	transformErr error
	categories   []imageai.StyleCategory
	list         imageai.StyleList
	usage        imageai.Usage
	usageErr     error
	usageStores  []int64
}

func (m *fakeModule) Transform(_ context.Context, in imageai.TransformInput) (imageai.TransformOutput, error) {
	m.transformIn = append(m.transformIn, in)
	return m.transformOut, m.transformErr
}

func (m *fakeModule) ListStyles(_ context.Context, c imageai.StyleCategory) (imageai.StyleList, error) {
	m.categories = append(m.categories, c)
	if c != imageai.CategoryProduct && c != imageai.CategoryPost {
		return imageai.StyleList{}, apperr.InvalidInputOn([]string{"Category"}, "%s", "用途不合法")
	}
	return m.list, nil
}

func (m *fakeModule) CurrentUsage(_ context.Context, id int64) (imageai.Usage, error) {
	m.usageStores = append(m.usageStores, id)
	return m.usage, m.usageErr
}

func setup(t *testing.T, m Module) imageaiv1alpha1connect.ImageAiAppServiceClient {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.com")
	if err != nil {
		t.Fatal(err)
	}
	path, h := imageaiv1alpha1connect.NewImageAiAppServiceHandler(NewApp(m, stores{}, images, slog.New(slog.DiscardHandler)),
		connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("X-Test-Account"); v != "" {
			acct, _ := strconv.ParseInt(v, 10, 64)
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: acct}))
		}
		h.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return imageaiv1alpha1connect.NewImageAiAppServiceClient(srv.Client(), srv.URL)
}

func as[T any](req *connect.Request[T], account int64) *connect.Request[T] {
	req.Header().Set("X-Test-Account", strconv.FormatInt(account, 10))
	return req
}

func details[T any](t *testing.T, err error) []T {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	var out []T
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatal(derr)
		}
		if x, ok := any(v).(T); ok {
			out = append(out, x)
		}
	}
	return out
}

func TestRequestsWithoutAnAccountAreUnauthenticated(t *testing.T) {
	c := setup(t, &fakeModule{})
	if _, err := c.TransformImage(t.Context(), connect.NewRequest(&pb.TransformImageRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("TransformImage 未登入 = %v", err)
	}
	if _, err := c.ListAiStyles(t.Context(), connect.NewRequest(&pb.ListAiStylesRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("ListAiStyles 未登入 = %v", err)
	}
}

func TestAccountWithoutAStoreIsPermissionDeniedAndNothingRuns(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	_, err := c.TransformImage(t.Context(), as(connect.NewRequest(&pb.TransformImageRequest{ImageData: []byte("x"), MimeType: "image/png", Prompt: "p"}), sellerAccount+1))
	if connect.CodeOf(err) != connect.CodePermissionDenied || len(m.transformIn) != 0 {
		t.Fatalf("TransformImage = %v，模組呼叫 %d 次", err, len(m.transformIn))
	}
	_, err = c.ListAiStyles(t.Context(), as(connect.NewRequest(&pb.ListAiStylesRequest{}), sellerAccount+1))
	if connect.CodeOf(err) != connect.CodePermissionDenied || len(m.categories) != 0 {
		t.Fatalf("ListAiStyles = %v", err)
	}
}

func TestTransformImageUsesTheCallersStoreAndReturnsTheUsage(t *testing.T) {
	m := &fakeModule{transformOut: imageai.TransformOutput{Image: []byte("out"), MimeType: "image/png", Usage: imageai.Usage{Used: 3, Limit: 10}}}
	c := setup(t, m)
	resp, err := c.TransformImage(t.Context(), as(connect.NewRequest(&pb.TransformImageRequest{ImageData: []byte("in"), MimeType: "image/jpeg", Prompt: "白底"}), sellerAccount))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg
	if string(got.GetImageData()) != "out" || got.GetMimeType() != "image/png" || got.GetWeeklyUsage() != 3 || got.GetWeeklyLimit() != 10 {
		t.Fatalf("回應 = %v", got)
	}
	in := m.transformIn[0]
	if in.StoreID != storeID || string(in.Image) != "in" || in.MimeType != "image/jpeg" || in.Prompt != "白底" {
		t.Fatalf("模組輸入 = %+v", in)
	}
}

func TestTransformImageMapsModuleFieldsToRequestPaths(t *testing.T) {
	for field, want := range map[string]string{"Image": "image_data", "MimeType": "mime_type", "Prompt": "prompt"} {
		m := &fakeModule{transformErr: apperr.InvalidInputOn([]string{field}, "%s", "x")}
		c := setup(t, m)
		_, err := c.TransformImage(t.Context(), as(connect.NewRequest(&pb.TransformImageRequest{}), sellerAccount))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s：%v", field, err)
		}
		var fields []string
		for _, br := range details[*errdetails.BadRequest](t, err) {
			for _, v := range br.GetFieldViolations() {
				fields = append(fields, v.GetField())
			}
		}
		if len(fields) != 1 || fields[0] != want {
			t.Errorf("%s：欄位 = %v，要 [%s]", field, fields, want)
		}
	}
}

func TestTransformImageCarriesTheModuleReasonAndMetadata(t *testing.T) {
	m := &fakeModule{transformErr: apperr.ResourceExhausted(0, "用完").WithReason(apperr.ReasonImageQuotaExceeded).WithMetadata("remaining", "0").WithMetadata("resetTime", "2026-11-29T16:00:00Z")}
	c := setup(t, m)
	_, err := c.TransformImage(t.Context(), as(connect.NewRequest(&pb.TransformImageRequest{}), sellerAccount))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("code = %v", connect.CodeOf(err))
	}
	infos := details[*errdetails.ErrorInfo](t, err)
	if len(infos) != 1 || infos[0].GetReason() != "IMAGE_QUOTA_EXCEEDED" || infos[0].GetMetadata()["remaining"] != "0" || infos[0].GetMetadata()["resetTime"] == "" {
		t.Fatalf("ErrorInfo = %v", infos)
	}
}

func TestBusyCarriesRetryInfoAndNoReason(t *testing.T) {
	m := &fakeModule{transformErr: apperr.ResourceExhausted(10_000_000_000, "忙")}
	c := setup(t, m)
	_, err := c.TransformImage(t.Context(), as(connect.NewRequest(&pb.TransformImageRequest{}), sellerAccount))
	if connect.CodeOf(err) != connect.CodeResourceExhausted || len(details[*errdetails.RetryInfo](t, err)) != 1 || len(details[*errdetails.ErrorInfo](t, err)) != 0 {
		t.Fatalf("err = %v", err)
	}
}

func TestListAiStylesDefaultsToProductAndRejectsUnspecified(t *testing.T) {
	m := &fakeModule{list: imageai.StyleList{DefaultPrompt: "預設"}}
	c := setup(t, m)
	if _, err := c.ListAiStyles(t.Context(), as(connect.NewRequest(&pb.ListAiStylesRequest{}), sellerAccount)); err != nil {
		t.Fatal(err)
	}
	post := pb.AiStyleCategory_AI_STYLE_CATEGORY_POST
	if _, err := c.ListAiStyles(t.Context(), as(connect.NewRequest(&pb.ListAiStylesRequest{Category: &post}), sellerAccount)); err != nil {
		t.Fatal(err)
	}
	unspecified := pb.AiStyleCategory_AI_STYLE_CATEGORY_UNSPECIFIED
	_, err := c.ListAiStyles(t.Context(), as(connect.NewRequest(&pb.ListAiStylesRequest{Category: &unspecified}), sellerAccount))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UNSPECIFIED = %v", err)
	}
	bad := details[*errdetails.BadRequest](t, err)
	if len(bad) != 1 || bad[0].GetFieldViolations()[0].GetField() != "category" {
		t.Fatalf("欄位 = %v", bad)
	}
	if len(m.categories) != 3 || m.categories[0] != imageai.CategoryProduct || m.categories[1] != imageai.CategoryPost || m.categories[2] != "" {
		t.Fatalf("模組收到的用途 = %v", m.categories)
	}
}

func TestListAiStylesBuildsStylesThumbnailsAndUsage(t *testing.T) {
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	m := &fakeModule{
		list: imageai.StyleList{DefaultPrompt: "預設提示詞", Styles: []imageai.Style{
			{ID: id, Name: "white", Label: "白底", Prompt: "p1", ThumbnailKey: "styles/a.jpg", DisplayOrder: 1},
			{ID: id, Name: "plain", Label: "素面", Prompt: "p2", DisplayOrder: 2},
		}},
		usage: imageai.Usage{Used: 4, Limit: 10},
	}
	c := setup(t, m)
	resp, err := c.ListAiStyles(t.Context(), as(connect.NewRequest(&pb.ListAiStylesRequest{}), sellerAccount))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg
	if got.GetDefaultPrompt() != "預設提示詞" || got.GetWeeklyUsage() != 4 || got.GetWeeklyLimit() != 10 || len(got.GetStyles()) != 2 {
		t.Fatalf("回應 = %v", got)
	}
	if got.Styles[0].GetThumbnail().GetUriTemplate() != "https://media.example.com/i/{preset}/styles/a.jpg" || got.Styles[1].GetThumbnail() != nil {
		t.Fatalf("縮圖 = %v / %v", got.Styles[0].GetThumbnail(), got.Styles[1].GetThumbnail())
	}
	if got.Styles[0].GetId() != id.String() || got.Styles[0].GetLabel() != "白底" || got.Styles[0].GetDisplayOrder() != 1 {
		t.Fatalf("風格 = %v", got.Styles[0])
	}
	if len(m.usageStores) != 1 || m.usageStores[0] != storeID {
		t.Fatalf("用量查的店 = %v", m.usageStores)
	}
}

func TestListAiStylesPropagatesUsageReadErrors(t *testing.T) {
	m := &fakeModule{usageErr: errors.New("db down")}
	c := setup(t, m)
	_, err := c.ListAiStyles(t.Context(), as(connect.NewRequest(&pb.ListAiStylesRequest{}), sellerAccount))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("用量讀取失敗不能被吞掉：%v", err)
	}
}

func TestNonSellerPermissionDeniedUsesTheCatalogTextAndItsLocale(t *testing.T) {
	c := setup(t, &fakeModule{})
	for _, lang := range []string{"zh-TW", "en"} {
		req := as(connect.NewRequest(&pb.ListAiStylesRequest{}), sellerAccount+1)
		req.Header().Set("Accept-Language", lang)
		_, err := c.ListAiStyles(t.Context(), req)
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatalf("%s：err = %v", lang, err)
		}
		msgs := details[*errdetails.LocalizedMessage](t, err)
		if len(msgs) != 1 {
			t.Fatalf("%s：LocalizedMessage %d 筆", lang, len(msgs))
		}
		// en 目錄沒有 common.error.PERMISSION_DENIED：文字退回 zh-TW，標籤要同樣是 zh-TW。
		if msgs[0].GetMessage() != "無權操作" || msgs[0].GetLocale() != "zh-TW" {
			t.Errorf("%s：%q（%s）", lang, msgs[0].GetMessage(), msgs[0].GetLocale())
		}
	}
}

func TestModuleErrorKeepsItsMessageLocale(t *testing.T) {
	m := &fakeModule{transformErr: apperr.PermissionDenied("%s", uitext.Message{Text: "No access", Locale: uitext.EN})}
	c := setup(t, m)
	_, err := c.TransformImage(t.Context(), as(connect.NewRequest(&pb.TransformImageRequest{ImageData: []byte("x"), MimeType: "image/png", Prompt: "p"}), sellerAccount))
	msgs := details[*errdetails.LocalizedMessage](t, err)
	if len(msgs) != 1 || msgs[0].GetMessage() != "No access" || msgs[0].GetLocale() != "en" {
		t.Fatalf("LocalizedMessage = %v", msgs)
	}
}
