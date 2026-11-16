//go:build integration

package characterization

import (
	"context"
	"testing"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	adminsellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	adminsellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	sellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
)

// takedownFixture 是一家有一篇已發布貼文的店，加上管理員的措施 client：內容下架的各項限制都經單體的 Connect 入口驗證（A7-D28 第 1、2 點）。
type takedownFixture struct {
	owner      socialv1alpha1connect.SocialAppServiceClient
	public     socialv1alpha1connect.SocialPublicServiceClient
	buyer      socialv1alpha1connect.SocialPublicServiceClient
	governance adminsellerconnect.SellerGovernanceAdminServiceClient
	post       *pb.Post
	storeID    string
}

func bearer(token string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, req)
		}
	}))
}

func newTakedownFixture(t *testing.T, m *monolith) *takedownFixture {
	t.Helper()
	ctx := t.Context()
	hc := m.srv.Client()
	f := &takedownFixture{
		owner:      socialv1alpha1connect.NewSocialAppServiceClient(hc, m.srv.URL, bearer(m.newCustomer(t, true))),
		public:     socialv1alpha1connect.NewSocialPublicServiceClient(hc, m.srv.URL),
		buyer:      socialv1alpha1connect.NewSocialPublicServiceClient(hc, m.srv.URL, bearer(m.newCustomer(t, false))),
		governance: adminsellerconnect.NewSellerGovernanceAdminServiceClient(hc, m.srv.URL, bearer(m.adminToken)),
	}
	res, err := f.owner.CreatePost(ctx, connect.NewRequest(&pb.CreatePostRequest{Body: "下架測試 " + uuid.NewV4().String(), Publish: true}))
	if err != nil {
		t.Fatal(err)
	}
	f.post = res.Msg.GetPost()
	f.storeID = f.post.GetStore().GetId()
	if f.storeID == "" {
		t.Fatal("建立貼文的回應沒有店家卡片")
	}
	return f
}

// takedown 以管理員的 CreateMeasure（內容下架）下架貼文，回措施（含 etag）。
func (f *takedownFixture) takedown(t *testing.T) *adminsellerpb.Measure {
	t.Helper()
	res, err := f.governance.CreateMeasure(t.Context(), connect.NewRequest(&adminsellerpb.CreateMeasureRequest{
		RequestId: uuid.NewV4().String(), StoreId: f.storeID, Statement: "內容違規", Note: "內部紀錄",
		Measure: &adminsellerpb.CreateMeasureRequest_ContentTakedown{ContentTakedown: &adminsellerpb.ContentTakedownInput{
			Contents: []*sellerpb.ContentRef{{Kind: sellerpb.ContentKind_CONTENT_KIND_POST, Id: f.post.GetId()}},
			Violation: &adminsellerpb.ViolationBasis{Finding: &adminsellerpb.NewFinding{
				Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM, Severity: sellerpb.ViolationSeverity_VIOLATION_SEVERITY_LOW,
			}},
		}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.GetMeasure()
}

func (f *takedownFixture) mine(t *testing.T) *pb.Post {
	t.Helper()
	res, err := f.owner.GetMyPost(t.Context(), connect.NewRequest(&pb.GetMyPostRequest{PostId: f.post.GetId()}))
	if err != nil {
		t.Fatal(err)
	}
	return res.Msg.GetPost()
}

func wantCode(t *testing.T, what string, err error, want connect.Code) {
	t.Helper()
	if connect.CodeOf(err) != want {
		t.Fatalf("%s：err=%v，預期 %v", what, err, want)
	}
}

// 內容下架之後的店主限制與理由（A7-D28 第 1、2 點）：每一條各一個測試，共用同一個單體與資料庫，各自用新的店與貼文。
func TestContentTakedownThroughConnect(t *testing.T) {
	m := startMonolith(t)

	t.Run("public read returns not found", func(t *testing.T) {
		f := newTakedownFixture(t, m)
		if _, err := f.public.GetPost(t.Context(), connect.NewRequest(&pb.GetPostRequest{PostId: f.post.GetId()})); err != nil {
			t.Fatalf("下架前公開讀取要成功：%v", err)
		}
		f.takedown(t)
		_, err := f.public.GetPost(t.Context(), connect.NewRequest(&pb.GetPostRequest{PostId: f.post.GetId()}))
		wantCode(t, "下架後的公開讀取", err, connect.CodeNotFound)
		_, err = f.buyer.GetPost(t.Context(), connect.NewRequest(&pb.GetPostRequest{PostId: f.post.GetId()}))
		wantCode(t, "下架後買家的公開讀取", err, connect.CodeNotFound)
	})

	t.Run("owner update is rejected", func(t *testing.T) {
		f := newTakedownFixture(t, m)
		f.takedown(t)
		mine := f.mine(t)
		_, err := f.owner.UpdatePost(t.Context(), connect.NewRequest(&pb.UpdatePostRequest{
			Post:       &pb.Post{Id: mine.GetId(), Etag: mine.GetEtag(), Body: "下架後的編輯"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body"}},
		}))
		wantCode(t, "下架後的編輯", err, connect.CodeFailedPrecondition)
		if got := f.mine(t); got.GetBody() != f.post.GetBody() || got.GetEtag() != mine.GetEtag() {
			t.Fatalf("被拒絕的編輯不能改內容或版本：%+v", got)
		}
	})

	t.Run("owner publish is rejected", func(t *testing.T) {
		f := newTakedownFixture(t, m)
		f.takedown(t)
		mine := f.mine(t)
		_, err := f.owner.PublishPost(t.Context(), connect.NewRequest(&pb.PublishPostRequest{PostId: mine.GetId(), Etag: mine.GetEtag()}))
		wantCode(t, "下架後的發布", err, connect.CodeFailedPrecondition)
		// 先取消發布再發布也一樣被擋（取消發布照常）。
		un, err := f.owner.UnpublishPost(t.Context(), connect.NewRequest(&pb.UnpublishPostRequest{PostId: mine.GetId(), Etag: mine.GetEtag()}))
		if err != nil {
			t.Fatalf("下架後取消發布要照常：%v", err)
		}
		_, err = f.owner.PublishPost(t.Context(), connect.NewRequest(&pb.PublishPostRequest{PostId: mine.GetId(), Etag: un.Msg.GetPost().GetEtag()}))
		wantCode(t, "取消發布後再發布", err, connect.CodeFailedPrecondition)
	})

	t.Run("owner read shows the measure id", func(t *testing.T) {
		f := newTakedownFixture(t, m)
		if f.mine(t).TakedownMeasureId != nil {
			t.Fatal("沒被下架的貼文不該帶措施 ID")
		}
		measureID := f.takedown(t).GetId()
		if got := f.mine(t).GetTakedownMeasureId(); got != measureID {
			t.Fatalf("GetMyPost 的措施 ID = %q，預期 %q", got, measureID)
		}
		list, err := f.owner.ListMyPosts(t.Context(), connect.NewRequest(&pb.ListMyPostsRequest{PageSize: 50}))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, p := range list.Msg.GetPosts() {
			if p.GetId() == f.post.GetId() {
				found = true
				if p.GetTakedownMeasureId() != measureID {
					t.Fatalf("ListMyPosts 的措施 ID = %q，預期 %q", p.GetTakedownMeasureId(), measureID)
				}
			}
		}
		if !found {
			t.Fatal("ListMyPosts 要列出被下架的貼文")
		}
	})

	t.Run("public read never shows the measure id", func(t *testing.T) {
		f := newTakedownFixture(t, m)
		for name, c := range map[string]socialv1alpha1connect.SocialPublicServiceClient{"visitor": f.public, "buyer": f.buyer} {
			res, err := c.GetPost(t.Context(), connect.NewRequest(&pb.GetPostRequest{PostId: f.post.GetId()}))
			if err != nil || res.Msg.GetPost().TakedownMeasureId != nil {
				t.Fatalf("%s 下架前：err=%v post=%+v", name, err, res.Msg.GetPost())
			}
		}
		measure := f.takedown(t)
		// 撤銷之後貼文重新公開：公開讀取要回得來，而且仍不帶措施 ID。
		if _, err := f.governance.RevokeMeasure(t.Context(), connect.NewRequest(&adminsellerpb.RevokeMeasureRequest{
			Id: measure.GetId(), Reason: adminsellerpb.AdminRevokeReason_ADMIN_REVOKE_REASON_ADMIN_CORRECTION, Etag: measure.GetEtag(),
		})); err != nil {
			t.Fatal(err)
		}
		for name, c := range map[string]socialv1alpha1connect.SocialPublicServiceClient{"visitor": f.public, "buyer": f.buyer} {
			res, err := c.GetPost(t.Context(), connect.NewRequest(&pb.GetPostRequest{PostId: f.post.GetId()}))
			if err != nil || res.Msg.GetPost().TakedownMeasureId != nil {
				t.Fatalf("%s 撤銷後：err=%v post=%+v", name, err, res.Msg.GetPost())
			}
		}
		if f.mine(t).TakedownMeasureId != nil {
			t.Fatal("撤銷之後店主讀到的貼文不該再帶措施 ID")
		}
	})

	t.Run("unpublish and delete still work", func(t *testing.T) {
		f := newTakedownFixture(t, m)
		f.takedown(t)
		mine := f.mine(t)
		if _, err := f.owner.UnpublishPost(t.Context(), connect.NewRequest(&pb.UnpublishPostRequest{PostId: mine.GetId(), Etag: mine.GetEtag()})); err != nil {
			t.Fatalf("下架後取消發布：%v", err)
		}
		if _, err := f.owner.DeletePost(t.Context(), connect.NewRequest(&pb.DeletePostRequest{PostId: mine.GetId()})); err != nil {
			t.Fatalf("下架後刪除：%v", err)
		}
	})
}
