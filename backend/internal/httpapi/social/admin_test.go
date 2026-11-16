package social

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

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/social/v1alpha1/socialv1alpha1connect"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
)

type fakeAdminPosts struct {
	page      social.OwnPostPage
	err       error
	calls     int
	gotFilter social.AdminPostFilter
	gotAfter  *social.OwnPostCursor
	gotSize   int32
}

func (f *fakeAdminPosts) ListAdminPosts(_ context.Context, fl social.AdminPostFilter, after *social.OwnPostCursor, size int32) (social.OwnPostPage, error) {
	f.calls++
	f.gotFilter, f.gotAfter, f.gotSize = fl, after, size
	return f.page, f.err
}

type fakeCounts struct {
	counts map[uuid.UUID]int64
	asked  []uuid.UUID
	err    error
}

func (f *fakeCounts) OpenReportCounts(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	f.asked = ids
	return f.counts, f.err
}

func newAdminHandlerEnv(t *testing.T, posts *fakeAdminPosts, counts *fakeCounts, guarded bool) socialv1alpha1connect.SocialAdminServiceClient {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewAdmin(posts, counts, images)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	interceptors := []connect.Interceptor{rpc.NewInterceptor(log)}
	if guarded {
		interceptors = append(interceptors, adminauth.NewInterceptor(log))
	}
	path, handler := socialv1alpha1connect.NewSocialAdminServiceHandler(h, connect.WithInterceptors(interceptors...))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		p := identity.Principal{Kind: identity.PrincipalAdmin, ID: 42}
		if role := req.Header.Get("X-Test-Role"); role != "" {
			p.AdminRoles = []adminperm.Role{adminperm.Role(role)}
		}
		handler.ServeHTTP(rw, req.WithContext(rpc.WithPrincipal(req.Context(), p)))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return socialv1alpha1connect.NewSocialAdminServiceClient(srv.Client(), srv.URL)
}

func withRole[T any](msg *T, role string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("X-Test-Role", role)
	return req
}

func TestListPostsWithoutThePermissionIsDeniedAndNeverReachesTheModules(t *testing.T) {
	posts, counts := &fakeAdminPosts{}, &fakeCounts{}
	c := newAdminHandlerEnv(t, posts, counts, true)
	_, err := c.ListPosts(t.Context(), withRole(&adminpb.ListPostsRequest{}, ""))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("沒有角色：err = %v，預期 PERMISSION_DENIED", err)
	}
	if posts.calls != 0 || counts.asked != nil {
		t.Fatalf("被拒絕的請求不該到模組：posts=%d counts=%v", posts.calls, counts.asked)
	}
	if _, err := c.ListPosts(t.Context(), withRole(&adminpb.ListPostsRequest{}, string(adminperm.RoleOwner))); err != nil {
		t.Fatalf("OWNER：%v", err)
	}
	if posts.calls != 1 {
		t.Fatalf("OWNER 通過後恰好到模組一次：%d", posts.calls)
	}
}

func TestListPostsComposesPostsMeasureIDsAndOpenReportCounts(t *testing.T) {
	created := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	published := created.Add(time.Hour)
	taken, plain := uuid.New(), uuid.New()
	measure := uuid.New()
	productID := int64(55)
	posts := &fakeAdminPosts{page: social.OwnPostPage{
		Posts: []social.Post{
			{ID: taken, StoreID: 7, Body: "下架的", ImageKey: "posts/a.jpg", ProductID: &productID, Status: social.PostStatus("PUBLISHED"), CreatedAt: created, UpdatedAt: created, PublishedAt: &published, TakedownMeasureID: &measure},
			{ID: plain, StoreID: 8, Body: "正常", Status: social.PostStatus("UNPUBLISHED"), CreatedAt: created, UpdatedAt: created, PublishedAt: &published},
		},
		Next: &social.OwnPostCursor{CreatedAt: created, ID: plain},
	}}
	counts := &fakeCounts{counts: map[uuid.UUID]int64{taken: 3}}
	c := newAdminHandlerEnv(t, posts, counts, false)

	res, err := c.ListPosts(t.Context(), connect.NewRequest(&adminpb.ListPostsRequest{PageSize: 2, TakenDownOnly: true}))
	if err != nil {
		t.Fatal(err)
	}
	if !posts.gotFilter.TakenDown || posts.gotSize != 2 || posts.gotAfter != nil {
		t.Fatalf("傳給模組：%+v size=%d after=%v", posts.gotFilter, posts.gotSize, posts.gotAfter)
	}
	if len(counts.asked) != 2 || counts.asked[0] != taken || counts.asked[1] != plain {
		t.Fatalf("檢舉數的查詢對象 = %v", counts.asked)
	}
	got := res.Msg.GetPosts()
	if len(got) != 2 {
		t.Fatalf("posts = %d", len(got))
	}
	if got[0].GetId() != taken.String() || got[0].GetStoreId() != "7" || got[0].GetTakedownMeasureId() != measure.String() ||
		got[0].GetOpenReportCount() != 3 || got[0].GetStatus() != pb.PostStatus_POST_STATUS_PUBLISHED || !got[0].GetPublishTime().AsTime().Equal(published) {
		t.Fatalf("下架的貼文：%+v", got[0])
	}
	if got[0].GetImage().GetUriTemplate() == "" || got[0].GetProductId() != "55" || got[1].Image != nil || got[1].ProductId != nil {
		t.Fatalf("圖片與商品：%+v / %+v", got[0], got[1])
	}
	if got[1].TakedownMeasureId != nil || got[1].GetOpenReportCount() != 0 || got[1].GetStatus() != pb.PostStatus_POST_STATUS_UNPUBLISHED {
		t.Fatalf("沒下架的貼文：%+v", got[1])
	}
	if res.Msg.GetNextPageToken() == "" {
		t.Fatal("有下一頁要給 token")
	}

	// token 帶回後解成模組的游標；token 綁篩選，換篩選回 INVALID_ARGUMENT。
	if _, err := c.ListPosts(t.Context(), connect.NewRequest(&adminpb.ListPostsRequest{TakenDownOnly: true, PageToken: res.Msg.GetNextPageToken()})); err != nil {
		t.Fatal(err)
	}
	if posts.gotAfter == nil || posts.gotAfter.ID != plain || !posts.gotAfter.CreatedAt.Equal(created) {
		t.Fatalf("游標 = %+v", posts.gotAfter)
	}
	_, err = c.ListPosts(t.Context(), connect.NewRequest(&adminpb.ListPostsRequest{PageToken: res.Msg.GetNextPageToken()}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("換篩選沿用 token：%v", err)
	}
}

func TestNewAdminRequiresBothSources(t *testing.T) {
	if _, err := NewAdmin(nil, &fakeCounts{}, rpc.Images{}); err == nil {
		t.Fatal("缺貼文來源要失敗")
	}
	if _, err := NewAdmin(&fakeAdminPosts{}, nil, rpc.Images{}); err == nil {
		t.Fatal("缺檢舉數來源要失敗")
	}
}

func TestListPostsSurfacesSourceErrorsInsteadOfZeros(t *testing.T) {
	boom := errors.New("boom")
	page := social.OwnPostPage{Posts: []social.Post{{ID: uuid.New(), StoreID: 7, Status: social.PostStatus("PUBLISHED"), CreatedAt: time.Now(), UpdatedAt: time.Now()}}}
	c := newAdminHandlerEnv(t, &fakeAdminPosts{page: page}, &fakeCounts{err: boom}, false)
	if _, err := c.ListPosts(t.Context(), connect.NewRequest(&adminpb.ListPostsRequest{})); err == nil {
		t.Fatal("檢舉數來源失敗要回錯誤，不是 0")
	}
	c = newAdminHandlerEnv(t, &fakeAdminPosts{err: boom}, &fakeCounts{}, false)
	if _, err := c.ListPosts(t.Context(), connect.NewRequest(&adminpb.ListPostsRequest{})); err == nil {
		t.Fatal("貼文來源失敗要回錯誤")
	}
}
