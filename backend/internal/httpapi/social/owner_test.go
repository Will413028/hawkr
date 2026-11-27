package social

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

type fakeOwner struct {
	readability
	calls      []string
	actors     []*delegatedactor.Actor
	store      seller.Store
	create     social.CreatePostInput
	update     social.UpdatePostInput
	version    int64
	scheduled  social.SchedulePostInput
	postID     *uuid.UUID
	unreplied  bool
	status     social.PostStatus
	after      *social.CreatedPostCursor
	cafter     *social.CommentCursor
	post       social.Post
	page       social.CreatedPostPage
	comments   social.CommentPage
	summary    social.StoreSummary
	schedules  social.SchedulePage
	schedAfter *social.ScheduleCursor
	schedStore int64
	err        error
}

func (f *fakeOwner) rec(op string, st seller.Store) error {
	f.calls = append(f.calls, op)
	f.store = st
	return f.err
}

func (f *fakeOwner) current(id uuid.UUID, version int64) social.Post {
	p := f.post
	p.ID, p.Version = id, version
	return f.readability.post(p)
}

func (f *fakeOwner) CreatePost(_ context.Context, st seller.Store, in social.CreatePostInput, actor *delegatedactor.Actor) (social.Post, error) {
	f.actors = append(f.actors, actor)
	f.create = in
	return f.readability.post(f.post), f.rec("create", st)
}

func (f *fakeOwner) UpdatePost(_ context.Context, st seller.Store, in social.UpdatePostInput, actor *delegatedactor.Actor) (social.Post, error) {
	f.actors = append(f.actors, actor)
	f.update = in
	return f.current(in.PostID, in.Version+1), f.rec("update", st)
}

func (f *fakeOwner) PublishPost(_ context.Context, st seller.Store, id uuid.UUID, v int64, actor *delegatedactor.Actor) (social.Post, error) {
	f.actors = append(f.actors, actor)
	f.version = v
	return f.current(id, v+1), f.rec("publish", st)
}

func (f *fakeOwner) UnpublishPost(_ context.Context, st seller.Store, id uuid.UUID, v int64, actor *delegatedactor.Actor) (social.Post, error) {
	f.actors = append(f.actors, actor)
	f.version = v
	return f.current(id, v+1), f.rec("unpublish", st)
}

func (f *fakeOwner) DeleteOwnPost(_ context.Context, st seller.Store, _ uuid.UUID, v int64) error {
	f.version = v
	return f.rec("delete", st)
}

func (f *fakeOwner) SchedulePost(_ context.Context, st seller.Store, in social.SchedulePostInput, actor *delegatedactor.Actor) (social.ScheduledPost, error) {
	f.actors = append(f.actors, actor)
	f.scheduled = in
	return social.ScheduledPost{Schedule: social.Schedule{PublishAt: in.PublishAt}, PostVersion: in.Version + 1, Post: f.current(in.PostID, in.Version+1)}, f.rec("schedule", st)
}

func (f *fakeOwner) CancelPostSchedule(_ context.Context, st seller.Store, id uuid.UUID, v int64) (int64, error) {
	f.postID, f.version = &id, v
	return v + 1, f.rec("cancelSchedule", st)
}

func (f *fakeOwner) ListPendingSchedules(_ context.Context, storeID int64, after *social.ScheduleCursor, _ int32) (social.SchedulePage, error) {
	f.schedAfter, f.schedStore = after, storeID
	return f.schedules, f.rec("listSchedules", seller.Store{StoreID: storeID})
}

func (f *fakeOwner) MyPost(_ context.Context, st seller.Store, id uuid.UUID) (social.Post, error) {
	return f.readability.post(f.current(id, f.post.Version)), f.rec("my", st)
}

func (f *fakeOwner) ListMyPosts(_ context.Context, st seller.Store, status social.PostStatus, after *social.CreatedPostCursor, _ int32) (social.CreatedPostPage, error) {
	f.status, f.after = status, after
	return f.createdPage(f.page), f.rec("list", st)
}

func (f *fakeOwner) ListStoreComments(_ context.Context, st seller.Store, postID *uuid.UUID, unreplied bool, after *social.CommentCursor, _ int32) (social.CommentPage, error) {
	f.postID, f.unreplied, f.cafter = postID, unreplied, after
	return f.commentPage(f.comments), f.rec("comments", st)
}

func (f *fakeOwner) StoreSocialSummary(_ context.Context, st seller.Store) (social.StoreSummary, error) {
	return f.summary, f.rec("summary", st)
}

func ownPost(storeID int64, status social.PostStatus) social.Post {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	p := post(storeID)
	p.Status, p.Version, p.CreatedAt, p.UpdatedAt, p.PublishedAt = status, 4, at, at.Add(time.Hour), nil
	pid := uuid.UUID{15: 88}
	p.ProductID = &pid
	p.Hashtags = []social.Hashtag{{Name: "tea", Display: "Tea"}}
	return p
}

func uuidV4() string {
	s := uuid.New().String()
	return s[:14] + "4" + s[15:19] + "8" + s[20:]
}

func TestOwnerSuccessPaths(t *testing.T) {
	own := &seller.Store{StoreID: 5}
	f := &fakeOwner{post: ownPost(5, social.StatusDraft)}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: own})
	ctx := t.Context()
	rid := uuidV4()
	pid := (uuid.UUID{15: 88}).String()

	cp, err := e.client.CreatePost(ctx, signedIn(&pb.CreatePostRequest{Body: "新品", ImageStagedToken: "tok", ProductId: &pid, HashtagInputs: []string{"#Tea", "烘焙"}, Publish: true, RequestId: rid}))
	if err != nil {
		t.Fatal("CreatePost", err)
	}
	if in := f.create; in.Body != "新品" || in.ImageToken != "tok" || in.ProductID == nil || *in.ProductID != (uuid.UUID{15: 88}) || len(in.Hashtags) != 2 || !in.Publish || in.RequestID == nil || in.RequestID.String() != rid {
		t.Errorf("CreatePost 的輸入 = %+v", in)
	}
	got := cp.Msg.GetPost()
	if got.GetStatus() != pb.PostStatus_POST_STATUS_DRAFT || got.GetEtag() != "4" || got.GetCreateTime() == nil || got.GetProductId() != pid || got.GetUrl() == "" || !got.GetViewerCanComment() {
		t.Errorf("CreatePost 回應 = %v", got)
	}
	if a := got.GetViewerActor(); a.GetKind() != pb.ActorKind_ACTOR_KIND_STORE || a.GetId() != "5" {
		t.Errorf("店主寫入的回應 viewer_actor = %v，預期店家 5", a)
	}
	if f.store.StoreID != 5 {
		t.Errorf("店家由伺服器推導：%+v", f.store)
	}

	id := uuid.New()
	mp, err := e.client.GetMyPost(ctx, signedIn(&pb.GetMyPostRequest{PostId: id.String()}))
	if err != nil || mp.Msg.GetPost().GetId() != id.String() || mp.Msg.GetPost().GetStatus() != pb.PostStatus_POST_STATUS_DRAFT || mp.Msg.GetPost().GetEtag() != "4" {
		t.Fatal("GetMyPost", err, mp)
	}

	f.page = social.CreatedPostPage{Posts: []social.Post{ownPost(5, social.StatusPublished), ownPost(5, social.StatusUnpublished)}, Next: &social.CreatedPostCursor{CreatedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), ID: uuid.New()}}
	lp, err := e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{Status: pb.PostStatus_POST_STATUS_UNPUBLISHED}))
	if err != nil || len(lp.Msg.GetPosts()) != 2 || lp.Msg.GetNextPageToken() == "" || f.status != social.StatusUnpublished || f.after != nil {
		t.Fatal("ListMyPosts", err, lp, f.status)
	}
	if lp.Msg.GetPosts()[0].GetStatus() != pb.PostStatus_POST_STATUS_PUBLISHED {
		t.Errorf("狀態 = %v", lp.Msg.GetPosts()[0].GetStatus())
	}
	if _, err := e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{Status: pb.PostStatus_POST_STATUS_UNPUBLISHED, PageToken: lp.Msg.GetNextPageToken()})); err != nil || f.after == nil {
		t.Fatal("ListMyPosts 第二頁", err)
	}
	if _, err := e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{Status: pb.PostStatus_POST_STATUS_DRAFT, PageToken: lp.Msg.GetNextPageToken()})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("換了 status 還用舊的 page_token：%v", err)
	}

	pb2, err := e.client.PublishPost(ctx, signedIn(&pb.PublishPostRequest{PostId: id.String(), Etag: "4"}))
	if err != nil || f.version != 4 || pb2.Msg.GetPost().GetEtag() != "5" {
		t.Fatal("PublishPost", err, f.version)
	}
	if _, err := e.client.UnpublishPost(ctx, signedIn(&pb.UnpublishPostRequest{PostId: id.String(), Etag: "5"})); err != nil || f.version != 5 {
		t.Fatal("UnpublishPost", err, f.version)
	}
	if _, err := e.client.DeletePost(ctx, signedIn(&pb.DeletePostRequest{PostId: id.String()})); err != nil || f.version != 0 {
		t.Fatal("DeletePost 沒帶 etag", err, f.version)
	}
	if _, err := e.client.DeletePost(ctx, signedIn(&pb.DeletePostRequest{PostId: id.String(), Etag: "7"})); err != nil || f.version != 7 {
		t.Fatal("DeletePost", err, f.version)
	}
	at := time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)
	sp, err := e.client.SchedulePost(ctx, signedIn(&pb.SchedulePostRequest{PostId: id.String(), Etag: "4", PublishTime: timestamppb.New(at)}))
	if err != nil || f.scheduled.By.Kind != social.ScheduleByOwner || !f.scheduled.PublishAt.Equal(at) || f.scheduled.Version != 4 || !sp.Msg.GetPublishTime().AsTime().Equal(at) || sp.Msg.GetPost().GetId() != id.String() {
		t.Fatal("SchedulePost", err, f.scheduled)
	}
	if len(f.actors) == 0 {
		t.Fatal("沒有記到任何寫入")
	}
	for i, a := range f.actors {
		if a != nil {
			t.Errorf("店主的第 %d 個寫入帶著代行者 %+v", i, a)
		}
	}
}

func TestOwnerUpdatePostMergesOnlyTheMaskedFields(t *testing.T) {
	f := &fakeOwner{post: ownPost(5, social.StatusDraft)}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	ctx := t.Context()
	id := uuid.New().String()
	up := func(p *pb.Post, paths ...string) error {
		_, err := e.client.UpdatePost(ctx, signedIn(&pb.UpdatePostRequest{Post: p, UpdateMask: mask(paths...)}))
		return err
	}
	if err := up(&pb.Post{Id: id, Etag: "4"}, "product_id"); err != nil {
		t.Fatal(err)
	}
	u := f.update
	if u.Body != nil || !u.SetProduct || u.ProductID != nil || u.ImageToken != "" || u.ClearImage || u.Hashtags != nil || u.Version != 4 {
		t.Errorf("只改商品：%+v", u)
	}
	if err := up(&pb.Post{Id: id, Etag: "4", Body: "新內文"}, "body"); err != nil {
		t.Fatal(err)
	}
	if u = f.update; u.Body == nil || *u.Body != "新內文" || u.SetProduct || u.ProductID != nil {
		t.Errorf("只改內文：%+v", u)
	}
	if err := up(&pb.Post{Id: id, Etag: "4", ImageStagedToken: "tok", HashtagInputs: []string{"a"}}, "image_staged_token", "hashtag_inputs"); err != nil {
		t.Fatal(err)
	}
	if u = f.update; u.ImageToken != "tok" || u.ClearImage || u.Hashtags == nil || len(*u.Hashtags) != 1 {
		t.Errorf("換圖與標籤：%+v", u)
	}
	if err := up(&pb.Post{Id: id, Etag: "4"}, "image_staged_token", "hashtag_inputs"); err != nil {
		t.Fatal(err)
	}
	if u = f.update; u.ImageToken != "" || !u.ClearImage || u.Hashtags == nil || len(*u.Hashtags) != 0 {
		t.Errorf("清圖與清標籤：%+v", u)
	}
	for name, p := range map[string]struct {
		post  *pb.Post
		paths []string
	}{
		"沒有可修改的欄位":           {&pb.Post{Id: id, Etag: "4"}, []string{"etag"}},
		"image 是唯讀":          {&pb.Post{Id: id, Etag: "4"}, []string{"image"}},
		"etag 缺":             {&pb.Post{Id: id}, []string{"body"}},
		"不認得的路徑":             {&pb.Post{Id: id, Etag: "4"}, []string{"nope"}},
		"id 不是 UUID":         {&pb.Post{Id: "x", Etag: "4"}, []string{"body"}},
		"product_id 不是 UUID": {&pb.Post{Id: id, Etag: "4", ProductId: proto.String("x")}, []string{"product_id"}},
	} {
		n := len(f.calls)
		if err := up(p.post, p.paths...); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
		if len(f.calls) != n {
			t.Errorf("%s：格式錯誤不該到模組", name)
		}
	}
	if _, err := e.client.UpdatePost(ctx, signedIn(&pb.UpdatePostRequest{UpdateMask: mask("body")})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("沒帶資源：%v", err)
	}
}

func TestOwnerStoreScopedCommentsAndSummary(t *testing.T) {
	at := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	parent := uuid.New()
	f := &fakeOwner{
		comments: social.CommentPage{
			StoreID: 5,
			Comments: []social.Comment{
				{ID: uuid.New(), PostID: uuid.New(), State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorBuyer, ID: 9}, Body: "請問", CreatedAt: at, UpdatedAt: at, Version: 2},
				{ID: parent, PostID: uuid.New(), State: social.CommentDeleted, CreatedAt: at, ReplyCount: 1},
			},
			Next: &social.CommentCursor{CreatedAt: at, ID: parent},
		},
		summary: social.StoreSummary{Posts: map[social.PostStatus]int64{social.StatusDraft: 3, social.StatusPublished: 12}, Scheduled: 2},
	}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	ctx := t.Context()

	resp, err := e.client.ListStoreComments(ctx, signedIn(&pb.ListStoreCommentsRequest{}))
	if err != nil || f.postID != nil || f.unreplied || len(resp.Msg.GetComments()) != 2 || resp.Msg.GetNextPageToken() == "" {
		t.Fatal("不指定貼文要是整家店的範圍", err, f.postID, f.unreplied)
	}
	first := resp.Msg.GetComments()[0]
	if first.GetBody() != "請問" || !first.GetViewerCanDelete() || !first.GetViewerCanReply() || first.GetIsMine() || first.GetEtag() != "2" {
		t.Errorf("店主看到的留言 = %v", first)
	}
	if resp.Msg.GetComments()[1].GetState() != pb.CommentState_COMMENT_STATE_DELETED || resp.Msg.GetComments()[1].GetBody() != "" {
		t.Errorf("佔位項目 = %v", resp.Msg.GetComments()[1])
	}
	pid := uuid.New()
	ps := pid.String()
	if _, err := e.client.ListStoreComments(ctx, signedIn(&pb.ListStoreCommentsRequest{PostId: &ps, Unreplied: true})); err != nil || f.postID == nil || *f.postID != pid || !f.unreplied {
		t.Fatal("指定貼文與 unreplied", err, f.postID, f.unreplied)
	}
	if _, err := e.client.ListStoreComments(ctx, signedIn(&pb.ListStoreCommentsRequest{Unreplied: true, PageToken: resp.Msg.GetNextPageToken()})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("換了篩選還用舊的 page_token：%v", err)
	}
	if _, err := e.client.ListStoreComments(ctx, signedIn(&pb.ListStoreCommentsRequest{PageToken: resp.Msg.GetNextPageToken()})); err != nil || f.cafter == nil || f.cafter.ID != parent {
		t.Fatal("同一組篩選的下一頁", err)
	}
	bad := "x"
	if _, err := e.client.ListStoreComments(ctx, signedIn(&pb.ListStoreCommentsRequest{PostId: &bad})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("post_id 格式錯：%v", err)
	}

	sum, err := e.client.GetStoreSocialSummary(ctx, signedIn(&pb.GetStoreSocialSummaryRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	want := map[pb.PostStatus]int64{pb.PostStatus_POST_STATUS_DRAFT: 3, pb.PostStatus_POST_STATUS_PUBLISHED: 12, pb.PostStatus_POST_STATUS_UNPUBLISHED: 0}
	got := map[pb.PostStatus]int64{}
	for _, c := range sum.Msg.GetPostCounts() {
		got[c.GetStatus()] = c.GetCount()
	}
	if len(got) != len(want) || len(sum.Msg.GetPostCounts()) != len(want) {
		t.Fatalf("post_counts = %v", sum.Msg.GetPostCounts())
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%v = %d，預期 %d", k, got[k], v)
		}
	}
	if sum.Msg.GetScheduledPostCount() != 2 {
		t.Errorf("scheduled_post_count = %d", sum.Msg.GetScheduledPostCount())
	}
	if len(social.PostStatusValues()) != len(want) {
		t.Errorf("PostStatus 值域多了成員，post_counts 要跟著加：%v", social.PostStatusValues())
	}
}

// 沒有店的帳號呼叫店主方法回 not_found（seller.CurrentStore 的結果），模組不被呼叫。
func TestOwnerMethodsRequireAStore(t *testing.T) {
	f := &fakeOwner{}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{})
	ctx := t.Context()
	id := uuid.New().String()
	cases := map[string]error{}
	_, cases["CreatePost"] = e.client.CreatePost(ctx, signedIn(&pb.CreatePostRequest{Body: "x"}))
	_, cases["GetMyPost"] = e.client.GetMyPost(ctx, signedIn(&pb.GetMyPostRequest{PostId: id}))
	_, cases["ListMyPosts"] = e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{}))
	_, cases["UpdatePost"] = e.client.UpdatePost(ctx, signedIn(&pb.UpdatePostRequest{Post: &pb.Post{Id: id, Etag: "1", Body: "x"}, UpdateMask: mask("body")}))
	_, cases["PublishPost"] = e.client.PublishPost(ctx, signedIn(&pb.PublishPostRequest{PostId: id, Etag: "1"}))
	_, cases["UnpublishPost"] = e.client.UnpublishPost(ctx, signedIn(&pb.UnpublishPostRequest{PostId: id, Etag: "1"}))
	_, cases["DeletePost"] = e.client.DeletePost(ctx, signedIn(&pb.DeletePostRequest{PostId: id}))
	_, cases["SchedulePost"] = e.client.SchedulePost(ctx, signedIn(&pb.SchedulePostRequest{PostId: id, Etag: "1", PublishTime: timestamppb.Now()}))
	_, cases["ListStoreComments"] = e.client.ListStoreComments(ctx, signedIn(&pb.ListStoreCommentsRequest{}))
	_, cases["GetStoreSocialSummary"] = e.client.GetStoreSocialSummary(ctx, signedIn(&pb.GetStoreSocialSummaryRequest{}))
	for name, err := range cases {
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("%s：%v，預期 not_found", name, err)
		}
	}
	if len(f.calls) != 0 {
		t.Fatalf("沒有店不該到模組：%v", f.calls)
	}
}

func TestOwnerRejectsMalformedInput(t *testing.T) {
	f := &fakeOwner{}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	ctx := t.Context()
	id := uuid.New().String()
	notV4 := uuid.New().String()
	notV4 = notV4[:14] + "7" + notV4[15:]
	cases := map[string]error{}
	_, cases["request_id"] = e.client.CreatePost(ctx, signedIn(&pb.CreatePostRequest{Body: "x", RequestId: notV4}))
	_, cases["product_id"] = e.client.CreatePost(ctx, signedIn(&pb.CreatePostRequest{Body: "x", ProductId: proto.String("abc")}))
	_, cases["post_id"] = e.client.GetMyPost(ctx, signedIn(&pb.GetMyPostRequest{PostId: "x"}))
	_, cases["publish etag 缺"] = e.client.PublishPost(ctx, signedIn(&pb.PublishPostRequest{PostId: id}))
	_, cases["unpublish etag 缺"] = e.client.UnpublishPost(ctx, signedIn(&pb.UnpublishPostRequest{PostId: id}))
	_, cases["schedule etag 缺"] = e.client.SchedulePost(ctx, signedIn(&pb.SchedulePostRequest{PostId: id, PublishTime: timestamppb.Now()}))
	_, cases["schedule 沒有時間"] = e.client.SchedulePost(ctx, signedIn(&pb.SchedulePostRequest{PostId: id, Etag: "1"}))
	_, cases["page_token"] = e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{PageToken: "x"}))
	for name, err := range cases {
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
	}
	if _, err := e.client.DeletePost(ctx, signedIn(&pb.DeletePostRequest{PostId: id, Etag: "x"})); connect.CodeOf(err) != connect.CodeAborted {
		t.Errorf("delete etag 格式錯：%v，預期 aborted", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("格式錯誤不該到模組：%v", f.calls)
	}
}

// 模組的拒絕（版本衝突、缺能力）原樣傳到 client。
func TestOwnerPropagatesModuleRefusals(t *testing.T) {
	f := &fakeOwner{err: apperr.VersionConflict("stale")}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	id := uuid.New().String()
	if _, err := e.client.PublishPost(t.Context(), signedIn(&pb.PublishPostRequest{PostId: id, Etag: "1"})); connect.CodeOf(err) != connect.CodeAborted {
		t.Errorf("版本衝突：%v", err)
	}
	f.err = apperr.PermissionDenied("停權").WithReason(apperr.ReasonSellerSuspended)
	if _, err := e.client.CreatePost(t.Context(), signedIn(&pb.CreatePostRequest{Body: "x"})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("缺能力建立：%v", err)
	}
}

// 能力宣告表與 SocialAppService 的方法一一對應：缺一個或多一個都不能啟動（NewApp 檢查的就是這個）。
func TestDeclarationsMatchTheServiceMethods(t *testing.T) {
	svc := serviceDescriptor()
	if err := checkDeclarations(declarations, svc); err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp(&fakeWrites{}, &fakeWrites{}, &fakeOwner{}, fakeBuyers{id: 7}, &fakeStores{}, &fakeProducts{}, mustImages(t), mustWeb(t), slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	for procedure := range declarations {
		short := make(map[string]declaration, len(declarations))
		for k, v := range declarations {
			if k != procedure {
				short[k] = v
			}
		}
		if err := checkDeclarations(short, svc); err == nil {
			t.Errorf("缺 %s 的宣告卻通過了檢查", procedure)
		}
	}
	extra := map[string]declaration{"/hawkr.social.v1alpha1.SocialAppService/Nope": exempt("x")}
	for k, v := range declarations {
		extra[k] = v
	}
	if err := checkDeclarations(extra, svc); err == nil {
		t.Error("多一個指向不存在方法的宣告卻通過了檢查")
	}
	for name, d := range map[string]declaration{
		"兩者皆無":   {},
		"兩者皆有":   {capability: seller.CapMarketing, exemptReason: "x"},
		"不存在的能力": requires("CAP_NOPE"),
	} {
		bad := map[string]declaration{}
		for k, v := range declarations {
			bad[k] = v
		}
		bad[socialv1alpha1connect.SocialAppServiceCreatePostProcedure] = d
		if err := checkDeclarations(bad, svc); err == nil {
			t.Errorf("%s 的宣告通過了檢查", name)
		}
	}
	for procedure, d := range declarations {
		if d.capability == "" && d.exemptReason == "" {
			t.Errorf("%s 沒有能力也沒有理由", procedure)
		}
	}
}

type reaching struct {
	m       *social.Module
	mu      sync.Mutex
	reached map[string]bool
}

var errPastAuthorization = errors.New("past authorization")

func (r *reaching) run(name string, f func() error) (err error) {
	defer func() {
		if recover() != nil {
			r.mu.Lock()
			r.reached[name] = true
			r.mu.Unlock()
			err = errPastAuthorization
		}
	}()
	return f()
}

func (r *reaching) CreatePost(ctx context.Context, st seller.Store, in social.CreatePostInput, actor *delegatedactor.Actor) (p social.Post, err error) {
	err = r.run("CreatePost", func() (e error) { p, e = r.m.CreatePost(ctx, st, in, actor); return })
	return
}

func (r *reaching) UpdatePost(ctx context.Context, st seller.Store, in social.UpdatePostInput, actor *delegatedactor.Actor) (p social.Post, err error) {
	err = r.run("UpdatePost", func() (e error) { p, e = r.m.UpdatePost(ctx, st, in, actor); return })
	return
}

func (r *reaching) PublishPost(ctx context.Context, st seller.Store, id uuid.UUID, v int64, actor *delegatedactor.Actor) (p social.Post, err error) {
	err = r.run("PublishPost", func() (e error) { p, e = r.m.PublishPost(ctx, st, id, v, actor); return })
	return
}

func (r *reaching) UnpublishPost(ctx context.Context, st seller.Store, id uuid.UUID, v int64, actor *delegatedactor.Actor) (p social.Post, err error) {
	err = r.run("UnpublishPost", func() (e error) { p, e = r.m.UnpublishPost(ctx, st, id, v, actor); return })
	return
}

func (r *reaching) DeleteOwnPost(ctx context.Context, st seller.Store, id uuid.UUID, v int64) error {
	return r.run("DeletePost", func() error { return r.m.DeleteOwnPost(ctx, st, id, v) })
}

func (r *reaching) SchedulePost(ctx context.Context, st seller.Store, in social.SchedulePostInput, actor *delegatedactor.Actor) (s social.ScheduledPost, err error) {
	err = r.run("SchedulePost", func() (e error) { s, e = r.m.SchedulePost(ctx, st, in, actor); return })
	return
}

// MyPost 通過授權時回一篇固定的貼文：UpdatePost 的 handler 先讀目前的貼文再合併，讓它走到模組的 UpdatePost 才檢查 CapMarketing。
func (r *reaching) MyPost(ctx context.Context, st seller.Store, id uuid.UUID) (p social.Post, err error) {
	err = r.run("MyPost", func() (e error) { p, e = r.m.MyPost(ctx, st, id); return })
	if errors.Is(err, errPastAuthorization) {
		return social.Post{ID: id, StoreID: st.StoreID, Version: 1}, nil
	}
	return
}

func (r *reaching) ListMyPosts(ctx context.Context, st seller.Store, s social.PostStatus, a *social.CreatedPostCursor, n int32) (p social.CreatedPostPage, err error) {
	err = r.run("ListMyPosts", func() (e error) { p, e = r.m.ListMyPosts(ctx, st, s, a, n); return })
	return
}

func (r *reaching) CancelPostSchedule(ctx context.Context, st seller.Store, id uuid.UUID, v int64) (n int64, err error) {
	err = r.run("CancelPostSchedule", func() (e error) { n, e = r.m.CancelPostSchedule(ctx, st, id, v); return })
	return
}

func (r *reaching) ListPendingSchedules(ctx context.Context, storeID int64, a *social.ScheduleCursor, n int32) (p social.SchedulePage, err error) {
	err = r.run("ListPendingSchedules", func() (e error) { p, e = r.m.ListPendingSchedules(ctx, storeID, a, n); return })
	return
}

func (r *reaching) ListStoreComments(ctx context.Context, st seller.Store, id *uuid.UUID, u bool, a *social.CommentCursor, n int32) (p social.CommentPage, err error) {
	err = r.run("ListStoreComments", func() (e error) { p, e = r.m.ListStoreComments(ctx, st, id, u, a, n); return })
	return
}

func (r *reaching) StoreSocialSummary(ctx context.Context, st seller.Store) (s social.StoreSummary, err error) {
	err = r.run("StoreSocialSummary", func() (e error) { s, e = r.m.StoreSocialSummary(ctx, st); return })
	return
}

type noBlocks struct{}

func (noBlocks) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) { return false, nil }

func storeIn(id int64, suspended, closed bool) seller.Store {
	var st seller.Store
	st.StoreID, st.AccountID, st.Status = id, id+7, seller.Active
	st.Sanctions = seller.NoSanctions()
	if suspended {
		st.Sanctions = seller.SanctionedBy(seller.EffectiveMeasure{MeasureID: uuid.NewV7()})
	}
	st.Conditions = seller.Conditions{
		Derived: true, Active: !closed, NotPendingDeletion: true, KYCApproved: true, TransfersActive: true, StoreName: true, Logo: true, TownCode: true,
	}
	return st
}

// 宣告表的每個店主方法所宣告的能力，與模組實際檢查的能力一致：停權的店家被 CapMarketing 的方法擋下（permission_denied＋SELLER_SUSPENDED）、
// 通過 CapContentContract 的方法（收縮與讀取）；關閉的店家被所有店主方法擋下。判斷「預期擋或放」一律取自宣告的能力在 seller.Authorize 的結果，
// 所以宣告與模組哪一邊改了、另一邊沒跟，這裡就會失敗。
func TestOwnerDeclarationsMatchTheModuleChecks(t *testing.T) {
	allow := social.ScheduleAuthorizerFunc(func(context.Context, social.ScheduledPublish) error { return nil })
	module := social.New(social.Deps{
		StoreBlocks: noBlocks{}, NewID: uuid.NewV7, Now: time.Now,
		ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: allow},
	})
	id := uuid.New().String()
	future := timestamppb.New(time.Now().Add(24 * time.Hour))
	type call struct {
		module string
		do     func(e *appEnv) error
	}
	calls := map[string]call{
		socialv1alpha1connect.SocialAppServiceCreatePostProcedure: {"CreatePost", func(e *appEnv) error {
			_, err := e.client.CreatePost(t.Context(), signedIn(&pb.CreatePostRequest{Body: "x"}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceUpdatePostProcedure: {"UpdatePost", func(e *appEnv) error {
			_, err := e.client.UpdatePost(t.Context(), signedIn(&pb.UpdatePostRequest{Post: &pb.Post{Id: id, Etag: "1", Body: "x"}, UpdateMask: mask("body")}))
			return err
		}},
		socialv1alpha1connect.SocialAppServicePublishPostProcedure: {"PublishPost", func(e *appEnv) error {
			_, err := e.client.PublishPost(t.Context(), signedIn(&pb.PublishPostRequest{PostId: id, Etag: "1"}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceUnpublishPostProcedure: {"UnpublishPost", func(e *appEnv) error {
			_, err := e.client.UnpublishPost(t.Context(), signedIn(&pb.UnpublishPostRequest{PostId: id, Etag: "1"}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceDeletePostProcedure: {"DeletePost", func(e *appEnv) error {
			_, err := e.client.DeletePost(t.Context(), signedIn(&pb.DeletePostRequest{PostId: id}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceSchedulePostProcedure: {"SchedulePost", func(e *appEnv) error {
			_, err := e.client.SchedulePost(t.Context(), signedIn(&pb.SchedulePostRequest{PostId: id, Etag: "1", PublishTime: future}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceCancelPostScheduleProcedure: {"CancelPostSchedule", func(e *appEnv) error {
			_, err := e.client.CancelPostSchedule(t.Context(), signedIn(&pb.CancelPostScheduleRequest{PostId: id, Etag: "1"}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceGetMyPostProcedure: {"MyPost", func(e *appEnv) error {
			_, err := e.client.GetMyPost(t.Context(), signedIn(&pb.GetMyPostRequest{PostId: id}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceListMyPostsProcedure: {"ListMyPosts", func(e *appEnv) error {
			_, err := e.client.ListMyPosts(t.Context(), signedIn(&pb.ListMyPostsRequest{}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceListStoreCommentsProcedure: {"ListStoreComments", func(e *appEnv) error {
			_, err := e.client.ListStoreComments(t.Context(), signedIn(&pb.ListStoreCommentsRequest{}))
			return err
		}},
		socialv1alpha1connect.SocialAppServiceGetStoreSocialSummaryProcedure: {"StoreSocialSummary", func(e *appEnv) error {
			_, err := e.client.GetStoreSocialSummary(t.Context(), signedIn(&pb.GetStoreSocialSummaryRequest{}))
			return err
		}},
	}
	owners := 0
	for procedure, d := range declarations {
		if d.capability == "" {
			continue
		}
		owners++
		c, ok := calls[procedure]
		if !ok {
			t.Errorf("%s 宣告了能力，但這個測試沒有對應的呼叫", procedure)
			continue
		}
		for _, state := range []struct {
			name              string
			suspended, closed bool
		}{{"停權", true, false}, {"關閉", false, true}} {
			st := storeIn(5, state.suspended, state.closed)
			r := &reaching{m: module, reached: map[string]bool{}}
			e := newAppEnvWith(t, &fakeWrites{}, r, &fakeStores{own: &st})
			err := c.do(e)
			wantErr := seller.Authorize(st, d.capability)
			switch {
			case wantErr == nil && !r.reached[c.module]:
				t.Errorf("%s（%s）：宣告 %s 在 seller.Authorize 放行，模組卻擋下：%v", procedure, state.name, d.capability, err)
			case wantErr != nil && r.reached[c.module]:
				t.Errorf("%s（%s）：宣告 %s 在 seller.Authorize 會被擋下，模組卻放行", procedure, state.name, d.capability)
			case wantErr != nil:
				want := connect.CodePermissionDenied
				if apperr.KindOf(wantErr) == apperr.KindNotFound {
					want = connect.CodeNotFound
				}
				if connect.CodeOf(err) != want {
					t.Errorf("%s（%s）：%v，預期 %v", procedure, state.name, err, want)
				}
			}
		}
	}
	if owners != len(calls) {
		t.Errorf("宣告了能力的方法有 %d 個，測試的呼叫有 %d 個", owners, len(calls))
	}
}

// 缺 CapMarketing（停權）時建立貼文被拒，原因是 SELLER_SUSPENDED。
func TestCreatePostWithoutMarketingCapabilityIsDenied(t *testing.T) {
	allow := social.ScheduleAuthorizerFunc(func(context.Context, social.ScheduledPublish) error { return nil })
	module := social.New(social.Deps{
		StoreBlocks: noBlocks{}, NewID: uuid.NewV7, Now: time.Now,
		ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: allow},
	})
	st := storeIn(5, true, false)
	e := newAppEnvWith(t, &fakeWrites{}, &reaching{m: module, reached: map[string]bool{}}, &fakeStores{own: &st})
	_, err := e.client.CreatePost(t.Context(), signedIn(&pb.CreatePostRequest{Body: "x"}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("err = %v，預期 permission_denied", err)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) || !hasReason(ce, "SELLER_SUSPENDED") {
		t.Errorf("原因應是 SELLER_SUSPENDED：%v", err)
	}
}

func hasReason(err *connect.Error, reason string) bool {
	for _, d := range err.Details() {
		v, e := d.Value()
		if e != nil {
			continue
		}
		if info, ok := v.(*errdetails.ErrorInfo); ok && info.GetReason() == reason {
			return true
		}
	}
	return false
}

func mustImages(t *testing.T) rpc.Images {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return images
}

func mustWeb(t *testing.T) rpc.WebOrigin {
	t.Helper()
	web, err := rpc.NewWebOrigin("https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return web
}

func TestOwnerListMyPostsRejectsUnknownStatusOnTheStatusField(t *testing.T) {
	f := &fakeOwner{}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	ctx := t.Context()
	_, err := e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{Status: pb.PostStatus(99)}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || len(f.calls) != 0 {
		t.Fatalf("未知的 status：%v，呼叫 %v", err, f.calls)
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	found := false
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if br, ok := v.(*errdetails.BadRequest); ok {
				for _, fv := range br.GetFieldViolations() {
					found = found || fv.GetField() == "status"
				}
			}
		}
	}
	if !found {
		t.Errorf("錯誤要指向 status 欄位：%v", ce.Details())
	}
	if _, err := e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{})); err != nil || f.status != "" {
		t.Errorf("UNSPECIFIED 是不篩選：%v %q", err, f.status)
	}
}

func TestOwnerPageTokensAreBoundToTheStore(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeOwner{
		page:     social.CreatedPostPage{Posts: []social.Post{ownPost(5, social.StatusDraft)}, Next: &social.CreatedPostCursor{CreatedAt: at, ID: uuid.New()}},
		comments: social.CommentPage{StoreID: 5, Next: &social.CommentCursor{CreatedAt: at, ID: uuid.New()}},
	}
	stores := &fakeStores{own: &seller.Store{StoreID: 5}}
	e := newAppEnvWith(t, &fakeWrites{}, f, stores)
	ctx := t.Context()
	lp, err := e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{}))
	if err != nil || lp.Msg.GetNextPageToken() == "" {
		t.Fatal(err)
	}
	lc, err := e.client.ListStoreComments(ctx, signedIn(&pb.ListStoreCommentsRequest{}))
	if err != nil || lc.Msg.GetNextPageToken() == "" {
		t.Fatal(err)
	}
	if _, err := e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{PageToken: lp.Msg.GetNextPageToken()})); err != nil {
		t.Fatalf("同一家店的 token：%v", err)
	}
	stores.own = &seller.Store{StoreID: 6}
	if _, err := e.client.ListMyPosts(ctx, signedIn(&pb.ListMyPostsRequest{PageToken: lp.Msg.GetNextPageToken()})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("別家店的貼文 token：%v", err)
	}
	if _, err := e.client.ListStoreComments(ctx, signedIn(&pb.ListStoreCommentsRequest{PageToken: lc.Msg.GetNextPageToken()})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("別家店的留言 token：%v", err)
	}
}

func TestOwnerSchedulePostRespondsFromTheScheduleResultWithoutAnotherRead(t *testing.T) {
	f := &fakeOwner{post: ownPost(5, social.StatusDraft)}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	id := uuid.New()
	at := time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)
	resp, err := e.client.SchedulePost(t.Context(), signedIn(&pb.SchedulePostRequest{PostId: id.String(), Etag: "4", PublishTime: timestamppb.New(at)}))
	if err != nil || resp.Msg.GetPost().GetEtag() != "5" {
		t.Fatal(err, resp)
	}
	if len(f.calls) != 1 || f.calls[0] != "schedule" {
		t.Errorf("排程成功後不該再讀貼文：%v", f.calls)
	}
}

func TestOwnerPublishabilityFailuresAreFailedPreconditionWithoutUnmappedFields(t *testing.T) {
	f := &fakeOwner{err: apperr.FailedPrecondition("缺圖")}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	ctx := t.Context()
	id := uuid.New().String()
	_, err := e.client.PublishPost(ctx, signedIn(&pb.PublishPostRequest{PostId: id, Etag: "4"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("PublishPost：%v", err)
	}
	_, err = e.client.SchedulePost(ctx, signedIn(&pb.SchedulePostRequest{PostId: id, Etag: "4", PublishTime: timestamppb.New(time.Now().Add(time.Hour))}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("SchedulePost：%v", err)
	}
}

func TestNewAppRejectsADeclarationTableMissingARow(t *testing.T) {
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	web, err := rpc.NewWebOrigin("https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	short := map[string]declaration{}
	for k, v := range declarations {
		short[k] = v
	}
	delete(short, socialv1alpha1connect.SocialAppServiceUpdatePostProcedure)
	if _, err := newApp(short, &fakeWrites{}, &fakeWrites{}, &fakeOwner{}, fakeBuyers{id: 7}, &fakeStores{}, &fakeProducts{}, images, web, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("缺一列的宣告表要不能啟動")
	}
	if _, err := newApp(declarations, &fakeWrites{}, &fakeWrites{}, &fakeOwner{}, fakeBuyers{id: 7}, &fakeStores{}, &fakeProducts{}, images, web, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerListPendingSchedulesReadsTheOwnStoreAndPages(t *testing.T) {
	at := time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)
	first, second := uuid.New(), uuid.New()
	f := &fakeOwner{schedules: social.SchedulePage{
		Schedules: []social.Schedule{
			{ID: first, PostID: uuid.New(), StoreID: 5, PublishAt: at, By: social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: "9"}, CreatedAt: at.Add(-time.Hour)},
			{ID: second, PostID: uuid.New(), StoreID: 5, PublishAt: at.Add(time.Hour), By: social.ScheduledBy{Kind: social.ScheduleByOwner}, CreatedAt: at.Add(-time.Hour)},
		},
		Next: &social.ScheduleCursor{PublishAt: at.Add(time.Hour), ID: second},
	}}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	resp, err := e.client.ListPendingSchedules(t.Context(), signedIn(&pb.ListPendingSchedulesRequest{PageSize: 2}))
	if err != nil {
		t.Fatal(err)
	}
	if f.schedStore != 5 || f.schedAfter != nil {
		t.Fatalf("店家 %d、游標 %v：預期店家來自 CurrentStore、第一頁沒有游標", f.schedStore, f.schedAfter)
	}
	got := resp.Msg.GetSchedules()
	if len(got) != 2 || got[0].GetScheduledBy() != pb.ScheduleKind_SCHEDULE_KIND_DELEGATION || got[1].GetScheduledBy() != pb.ScheduleKind_SCHEDULE_KIND_OWNER ||
		got[0].GetId() != first.String() || !got[0].GetPublishTime().AsTime().Equal(at) {
		t.Fatalf("schedules = %v", got)
	}
	if resp.Msg.GetNextPageToken() == "" {
		t.Fatal("有下一頁卻沒有 next_page_token")
	}
	f.schedules.Next = nil
	if _, err := e.client.ListPendingSchedules(t.Context(), signedIn(&pb.ListPendingSchedulesRequest{PageToken: resp.Msg.GetNextPageToken()})); err != nil {
		t.Fatal(err)
	}
	if f.schedAfter == nil || f.schedAfter.ID != second || !f.schedAfter.PublishAt.Equal(at.Add(time.Hour)) {
		t.Fatalf("第二頁的游標 = %v", f.schedAfter)
	}
	if _, err := e.client.ListPendingSchedules(t.Context(), signedIn(&pb.ListPendingSchedulesRequest{PageToken: "garbage"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("壞的 page_token：%v", err)
	}
}

func TestOwnerCancelPostScheduleReturnsTheNextEtagAndRequiresOne(t *testing.T) {
	f := &fakeOwner{}
	e := newAppEnvWith(t, &fakeWrites{}, f, &fakeStores{own: &seller.Store{StoreID: 5}})
	id := uuid.New()
	resp, err := e.client.CancelPostSchedule(t.Context(), signedIn(&pb.CancelPostScheduleRequest{PostId: id.String(), Etag: "4"}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetEtag() != "5" || f.postID == nil || *f.postID != id || f.version != 4 || f.store.StoreID != 5 {
		t.Fatalf("etag %q、貼文 %v、版本 %d、店家 %d", resp.Msg.GetEtag(), f.postID, f.version, f.store.StoreID)
	}
	f.calls = nil
	for name, req := range map[string]*pb.CancelPostScheduleRequest{
		"沒有 etag":   {PostId: id.String()},
		"貼文 ID 不正確": {PostId: "x", Etag: "1"},
	} {
		if _, err := e.client.CancelPostSchedule(t.Context(), signedIn(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，預期 invalid_argument", name, err)
		}
	}
	if len(f.calls) != 0 {
		t.Fatalf("請求不合法不該到模組：%v", f.calls)
	}
}
