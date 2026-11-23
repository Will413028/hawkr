package imageai

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/imageai/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/imageai/v1alpha1/imageaiv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/imageai/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
)

var (
	styleID = uuid.MustParse("01930000-0000-7000-8000-000000000001")
	created = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
)

type fakeAdmin struct {
	style   imageai.AdminStyle
	quota   imageai.AdminQuota
	created []imageai.CreateStyleInput
	updated []imageai.UpdateStyleInput
	deleted []struct {
		id      uuid.UUID
		version int64
		actor   imageai.Actor
	}
	listed   []*imageai.StyleCategory
	quotaIn  []imageai.UpdateQuotaInput
	quotaDel []int64
	quotaVer []int64
}

func (m *fakeAdmin) ListStylesForAdmin(_ context.Context, c *imageai.StyleCategory) ([]imageai.AdminStyle, error) {
	m.listed = append(m.listed, c)
	return []imageai.AdminStyle{m.style}, nil
}

func (m *fakeAdmin) ReadStyleForAdmin(context.Context, uuid.UUID) (imageai.AdminStyle, error) {
	return m.style, nil
}

func (m *fakeAdmin) CreateStyle(_ context.Context, in imageai.CreateStyleInput) (imageai.AdminStyle, error) {
	m.created = append(m.created, in)
	return m.style, nil
}

func (m *fakeAdmin) UpdateStyle(_ context.Context, in imageai.UpdateStyleInput) (imageai.AdminStyle, error) {
	m.updated = append(m.updated, in)
	return m.style, nil
}

func (m *fakeAdmin) DeleteStyle(_ context.Context, a imageai.Actor, id uuid.UUID, v int64) error {
	m.deleted = append(m.deleted, struct {
		id      uuid.UUID
		version int64
		actor   imageai.Actor
	}{id, v, a})
	return nil
}

func (m *fakeAdmin) GetQuota(context.Context, int64) (imageai.AdminQuota, error) { return m.quota, nil }

func (m *fakeAdmin) UpdateQuota(_ context.Context, in imageai.UpdateQuotaInput) (imageai.AdminQuota, error) {
	m.quotaIn = append(m.quotaIn, in)
	return m.quota, nil
}

func (m *fakeAdmin) DeleteQuota(_ context.Context, _ imageai.Actor, id, version int64) error {
	m.quotaDel = append(m.quotaDel, id)
	m.quotaVer = append(m.quotaVer, version)
	return nil
}

func newFakeAdmin() *fakeAdmin {
	return &fakeAdmin{
		style: imageai.AdminStyle{
			ID: styleID, Name: "clean", Label: "乾淨", Prompt: "白底", Category: imageai.CategoryProduct, ThumbnailKey: "public/7114/t.jpg",
			DisplayOrder: 3, IsDefault: true, Enabled: true, Version: 4, CreatedAt: created, UpdatedAt: created,
		},
		quota: imageai.AdminQuota{StoreID: 11, WeeklyLimit: 10, WeeklyUsage: 2},
	}
}

func setupAdmin(t *testing.T, m AdminModule) imageaiv1alpha1connect.ImageAiAdminServiceClient {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.com")
	if err != nil {
		t.Fatal(err)
	}
	path, h := imageaiv1alpha1connect.NewImageAiAdminServiceHandler(NewAdmin(m, images),
		connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 99}))
		h.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return imageaiv1alpha1connect.NewImageAiAdminServiceClient(srv.Client(), srv.URL)
}

func wantCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	_ = connectErr(t, err, want)
}

func connectErr(t *testing.T, err error, want connect.Code) *connect.Error {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != want {
		t.Fatalf("err = %v，預期 %v", err, want)
	}
	return ce
}

func violationFields(ce *connect.Error) []string {
	var out []string
	for _, d := range ce.Details() {
		v, err := d.Value()
		if err != nil {
			continue
		}
		if br, ok := v.(*errdetails.BadRequest); ok {
			for _, fv := range br.FieldViolations {
				out = append(out, fv.Field)
			}
		}
	}
	return out
}

func TestAdminFieldPathsExistInTheRequests(t *testing.T) {
	for name, miss := range map[string][]string{
		"create": rpc.CheckFieldPaths((&adminpb.CreateAiStyleRequest{}).ProtoReflect().Descriptor(), stylePaths),
		"update": rpc.CheckFieldPaths((&adminpb.UpdateAiStyleRequest{}).ProtoReflect().Descriptor(), stylePaths),
		"list":   rpc.CheckFieldPaths((&adminpb.ListAiStylesForAdminRequest{}).ProtoReflect().Descriptor(), listStylePaths),
		"quota":  rpc.CheckFieldPaths((&adminpb.UpdateAiQuotaRequest{}).ProtoReflect().Descriptor(), quotaPaths),
	} {
		if len(miss) != 0 {
			t.Errorf("%s：路徑在請求裡不存在：%v", name, miss)
		}
	}
}

func TestUpdateAiStyleRequiresAnEtag(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	_, err := c.UpdateAiStyle(context.Background(), connect.NewRequest(&adminpb.UpdateAiStyleRequest{
		AiStyle: &adminpb.AiStyle{Id: styleID.String(), Label: "x"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"label"}},
	}))
	ce := connectErr(t, err, connect.CodeInvalidArgument)
	if got := violationFields(ce); len(got) != 1 || got[0] != "ai_style.etag" {
		t.Fatalf("欄位 = %v", got)
	}
	if len(m.updated) != 0 {
		t.Fatal("沒帶 etag 不該呼叫模組")
	}
}

func TestUpdateAiStyleMalformedEtagIsAVersionConflict(t *testing.T) {
	c := setupAdmin(t, newFakeAdmin())
	_, err := c.UpdateAiStyle(context.Background(), connect.NewRequest(&adminpb.UpdateAiStyleRequest{
		AiStyle: &adminpb.AiStyle{Id: styleID.String(), Etag: "abc"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"label"}},
	}))
	wantCode(t, err, connect.CodeAborted)
}

func TestUpdateAiStyleMaskedUpdateKeepsTheOtherFieldsAndPassesTheEtag(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	resp, err := c.UpdateAiStyle(context.Background(), connect.NewRequest(&adminpb.UpdateAiStyleRequest{
		AiStyle:    &adminpb.AiStyle{Id: styleID.String(), Etag: "4", Label: "新名", Prompt: "忽略"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"label"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.updated) != 1 {
		t.Fatalf("updated = %v", m.updated)
	}
	got := m.updated[0]
	want := imageai.UpdateStyleInput{
		Actor: imageai.Actor{AdminID: 99, Action: imageaiv1alpha1connect.ImageAiAdminServiceUpdateAiStyleProcedure}, ID: styleID, Version: 4,
		Label: "新名", Prompt: "白底", DisplayOrder: 3, IsDefault: true, Enabled: true,
	}
	if got != want {
		t.Fatalf("模組輸入 = %+v\n預期 %+v", got, want)
	}
	s := resp.Msg.GetAiStyle()
	if s.GetEtag() != "4" || s.GetCategory() != pb.AiStyleCategory_AI_STYLE_CATEGORY_PRODUCT || !s.GetDefaultStyle() || s.GetThumbnail().GetUriTemplate() != "https://media.example.com/i/{preset}/public/7114/t.jpg" {
		t.Fatalf("回應 = %v", s)
	}
}

func TestUpdateAiStyleImmutableFieldsCannotChange(t *testing.T) {
	for name, st := range map[string]*adminpb.AiStyle{
		"name":     {Id: styleID.String(), Etag: "4", Name: "other"},
		"category": {Id: styleID.String(), Etag: "4", Category: pb.AiStyleCategory_AI_STYLE_CATEGORY_POST},
	} {
		t.Run(name, func(t *testing.T) {
			m := newFakeAdmin()
			c := setupAdmin(t, m)
			_, err := c.UpdateAiStyle(context.Background(), connect.NewRequest(&adminpb.UpdateAiStyleRequest{AiStyle: st, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{name}}}))
			wantCode(t, err, connect.CodeInvalidArgument)
			if len(m.updated) != 0 {
				t.Fatal("不可變欄位被改時不該呼叫模組")
			}
		})
	}
}

func TestUpdateAiStyleMaskNamingDefaultStyleFalseReachesTheModule(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	_, err := c.UpdateAiStyle(context.Background(), connect.NewRequest(&adminpb.UpdateAiStyleRequest{
		AiStyle: &adminpb.AiStyle{Id: styleID.String(), Etag: "4"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"default_style"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.updated) != 1 || m.updated[0].IsDefault {
		t.Fatalf("mask 明列 default_style=false 要原樣交給模組（由模組回 invalid_argument）：%+v", m.updated)
	}
}

func TestUpdateAiStyleWithoutAMaskOnlyWritesTheFieldsThatAreSet(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	_, err := c.UpdateAiStyle(context.Background(), connect.NewRequest(&adminpb.UpdateAiStyleRequest{
		AiStyle: &adminpb.AiStyle{Id: styleID.String(), Etag: "4", Label: "只改名稱", ThumbnailStagedToken: "tok"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	got := m.updated[0]
	if got.Label != "只改名稱" || got.ThumbnailToken != "tok" || got.Prompt != "白底" || got.DisplayOrder != 3 || !got.Enabled || !got.IsDefault {
		t.Fatalf("got = %+v", got)
	}
}

func TestCreateAiStyleMapsEveryField(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	_, err := c.CreateAiStyle(context.Background(), connect.NewRequest(&adminpb.CreateAiStyleRequest{AiStyle: &adminpb.AiStyle{
		Name: "n", Label: "l", Prompt: "p", Category: pb.AiStyleCategory_AI_STYLE_CATEGORY_POST, ThumbnailStagedToken: "tok",
		DisplayOrder: 9, DefaultStyle: true, Enabled: true,
	}}))
	if err != nil {
		t.Fatal(err)
	}
	want := imageai.CreateStyleInput{
		Actor: imageai.Actor{AdminID: 99, Action: imageaiv1alpha1connect.ImageAiAdminServiceCreateAiStyleProcedure},
		Name:  "n", Label: "l", Prompt: "p", Category: imageai.CategoryPost, ThumbnailToken: "tok", DisplayOrder: 9, IsDefault: true, Enabled: true,
	}
	if len(m.created) != 1 || m.created[0] != want {
		t.Fatalf("created = %+v\n預期 %+v", m.created, want)
	}
}

func TestCreateAiStyleUnspecifiedCategoryReachesTheModuleAsEmpty(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	if _, err := c.CreateAiStyle(context.Background(), connect.NewRequest(&adminpb.CreateAiStyleRequest{AiStyle: &adminpb.AiStyle{Name: "n"}})); err != nil {
		t.Fatal(err)
	}
	if m.created[0].Category != "" {
		t.Fatalf("UNSPECIFIED 不能被當成有效用途：%q", m.created[0].Category)
	}
}

func TestListAiStylesForAdminCategoryFilter(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	ctx := context.Background()
	if _, err := c.ListAiStylesForAdmin(ctx, connect.NewRequest(&adminpb.ListAiStylesForAdminRequest{})); err != nil {
		t.Fatal(err)
	}
	cat := pb.AiStyleCategory_AI_STYLE_CATEGORY_POST
	if _, err := c.ListAiStylesForAdmin(ctx, connect.NewRequest(&adminpb.ListAiStylesForAdminRequest{Category: &cat})); err != nil {
		t.Fatal(err)
	}
	if m.listed[0] != nil || m.listed[1] == nil || *m.listed[1] != imageai.CategoryPost {
		t.Fatalf("listed = %v", m.listed)
	}
}

func TestDeleteAiStyleEtagIsOptionalButParsedWhenGiven(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	ctx := context.Background()
	if _, err := c.DeleteAiStyle(ctx, connect.NewRequest(&adminpb.DeleteAiStyleRequest{Id: styleID.String()})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteAiStyle(ctx, connect.NewRequest(&adminpb.DeleteAiStyleRequest{Id: styleID.String(), Etag: "6"})); err != nil {
		t.Fatal(err)
	}
	_, err := c.DeleteAiStyle(ctx, connect.NewRequest(&adminpb.DeleteAiStyleRequest{Id: styleID.String(), Etag: "x"}))
	wantCode(t, err, connect.CodeAborted)
	_, err = c.DeleteAiStyle(ctx, connect.NewRequest(&adminpb.DeleteAiStyleRequest{Id: "nope"}))
	wantCode(t, err, connect.CodeInvalidArgument)
	if len(m.deleted) != 2 || m.deleted[0].version != 0 || m.deleted[1].version != 6 || m.deleted[1].actor.AdminID != 99 {
		t.Fatalf("deleted = %+v", m.deleted)
	}
}

func TestGetAiQuotaDefaultHasNoEtag(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	resp, err := c.GetAiQuota(context.Background(), connect.NewRequest(&adminpb.GetAiQuotaRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	q := resp.Msg.GetAiQuota()
	if q.GetStoreId() != "11" || q.GetWeeklyLimit() != 10 || q.GetWeeklyUsage() != 2 || q.GetCustom() || q.GetEtag() != "" || q.GetUpdateTime() != nil {
		t.Fatalf("quota = %v", q)
	}
	_, err = c.GetAiQuota(context.Background(), connect.NewRequest(&adminpb.GetAiQuotaRequest{StoreId: "abc"}))
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestUpdateAiQuotaMaskKeepsTheCurrentLimitAndPassesTheEtag(t *testing.T) {
	m := newFakeAdmin()
	m.quota = imageai.AdminQuota{StoreID: 11, WeeklyLimit: 30, Custom: true, Version: 2, UpdatedAt: created}
	c := setupAdmin(t, m)
	resp, err := c.UpdateAiQuota(context.Background(), connect.NewRequest(&adminpb.UpdateAiQuotaRequest{
		AiQuota:    &adminpb.AiQuota{StoreId: "11", Memo: "活動", WeeklyLimit: 999, Etag: "2"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"memo"}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := imageai.UpdateQuotaInput{
		Actor: imageai.Actor{AdminID: 99, Action: imageaiv1alpha1connect.ImageAiAdminServiceUpdateAiQuotaProcedure}, StoreID: 11, WeeklyLimit: 30, Memo: "活動", Version: 2,
	}
	if len(m.quotaIn) != 1 || m.quotaIn[0] != want {
		t.Fatalf("模組輸入 = %+v\n預期 %+v", m.quotaIn, want)
	}
	if q := resp.Msg.GetAiQuota(); q.GetEtag() != "2" || !q.GetCustom() || q.GetUpdateTime() == nil {
		t.Fatalf("回應 = %v", q)
	}
}

func TestUpdateAiQuotaWithoutAnEtagSendsVersionZero(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	if _, err := c.UpdateAiQuota(context.Background(), connect.NewRequest(&adminpb.UpdateAiQuotaRequest{
		AiQuota: &adminpb.AiQuota{StoreId: "11", WeeklyLimit: 50}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"weekly_limit"}},
	})); err != nil {
		t.Fatal(err)
	}
	if m.quotaIn[0].Version != 0 || m.quotaIn[0].WeeklyLimit != 50 {
		t.Fatalf("quotaIn = %+v", m.quotaIn)
	}
}

func TestUpdateAiQuotaStoreIDCannotBeChangedThroughTheMask(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	if _, err := c.UpdateAiQuota(context.Background(), connect.NewRequest(&adminpb.UpdateAiQuotaRequest{
		AiQuota: &adminpb.AiQuota{StoreId: "11", WeeklyLimit: 5}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"store_id", "weekly_limit"}},
	})); err != nil {
		t.Fatal(err)
	}
	if m.quotaIn[0].StoreID != 11 {
		t.Fatalf("storeID = %d", m.quotaIn[0].StoreID)
	}
}

func TestDeleteAiQuotaPassesTheStoreAndOptionalEtag(t *testing.T) {
	m := newFakeAdmin()
	c := setupAdmin(t, m)
	ctx := context.Background()
	if _, err := c.DeleteAiQuota(ctx, connect.NewRequest(&adminpb.DeleteAiQuotaRequest{StoreId: "11", Etag: "3"})); err != nil {
		t.Fatal(err)
	}
	_, err := c.DeleteAiQuota(ctx, connect.NewRequest(&adminpb.DeleteAiQuotaRequest{StoreId: "0"}))
	wantCode(t, err, connect.CodeInvalidArgument)
	if len(m.quotaDel) != 1 || m.quotaDel[0] != 11 || m.quotaVer[0] != 3 {
		t.Fatalf("quotaDel=%v ver=%v", m.quotaDel, m.quotaVer)
	}
}
