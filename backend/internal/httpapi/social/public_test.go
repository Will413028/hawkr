package social

import (
	"context"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/descriptorpb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeReads struct {
	post     social.Post
	postErr  error
	feed     social.PostPage
	storeID  int64
	comments social.CommentPage
	replies  social.CommentPage
	top      []social.Post
	topN     int32
	callers  []social.Caller
}

func (f *fakeReads) PublicPost(_ context.Context, c social.Caller, _ uuid.UUID) (social.Post, error) {
	f.callers = append(f.callers, c)
	return f.post, f.postErr
}

func (f *fakeReads) ListFeed(_ context.Context, c social.Caller, _ *social.PostCursor, _ int32) (social.PostPage, error) {
	f.callers = append(f.callers, c)
	return f.feed, nil
}

func (f *fakeReads) ListStorePosts(_ context.Context, c social.Caller, storeID int64, _ *social.PostCursor, _ int32) (social.PostPage, error) {
	f.callers = append(f.callers, c)
	f.storeID = storeID
	return f.feed, nil
}

func (f *fakeReads) ListComments(context.Context, social.Caller, uuid.UUID, *social.CommentCursor, int32) (social.CommentPage, error) {
	return f.comments, nil
}

func (f *fakeReads) ListReplies(context.Context, social.Caller, uuid.UUID, *social.CommentCursor, int32) (social.CommentPage, error) {
	return f.replies, nil
}

func (f *fakeReads) TopPosts(_ context.Context, _ social.Caller, n int32) ([]social.Post, error) {
	f.topN = n
	return f.top, nil
}

type fakeBuyers struct{ id int64 }

func (f fakeBuyers) CurrentBuyerID(context.Context, int64) (int64, error) { return f.id, nil }

func (fakeBuyers) BatchDisplay(_ context.Context, ids []int64) ([]buyer.Display, error) {
	out := make([]buyer.Display, len(ids))
	for i, id := range ids {
		name := "buyer"
		out[i] = buyer.Display{BuyerID: id, DisplayName: &name}
	}
	return out, nil
}

type fakeStores struct {
	own      *seller.Store
	blocking map[int64]bool
	asked    [][]int64
}

func (f *fakeStores) CurrentStore(context.Context, int64) (seller.Store, error) {
	if f.own == nil {
		return seller.Store{}, apperr.NotFound("no store")
	}
	return *f.own, nil
}

func (f *fakeStores) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	name := "shop"
	out := make([]seller.StoreCard, len(ids))
	for i, id := range ids {
		out[i] = seller.StoreCard{StoreID: id, StoreCode: id, Name: &name}
	}
	return out, nil
}

func (f *fakeStores) StoresBlockingBuyer(_ context.Context, _ int64, ids []int64) (map[int64]bool, error) {
	f.asked = append(f.asked, ids)
	return f.blocking, nil
}

func newPublic(t *testing.T, r *fakeReads, b fakeBuyers, s *fakeStores) *Public {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	web, err := rpc.NewWebOrigin("https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewPublic(r, b, s, images, web)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func asBuyer(ctx context.Context) context.Context {
	return rpc.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalAccount, ID: 7})
}

func post(storeID int64) social.Post {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return social.Post{ID: uuid.New(), StoreID: storeID, Body: "hi", PublishedAt: &at, UpdatedAt: at, Stats: social.PostStats{Views: 3}}
}

func TestEverySocialPublicMethodIsNoSideEffects(t *testing.T) {
	methods := pb.File_hawkr_social_v1alpha1_social_proto.Services().ByName("SocialPublicService").Methods()
	if methods.Len() == 0 {
		t.Fatal("沒有方法")
	}
	for i := range methods.Len() {
		m := methods.Get(i)
		opts, ok := m.Options().(*descriptorpb.MethodOptions)
		if !ok || opts.GetIdempotencyLevel() != descriptorpb.MethodOptions_NO_SIDE_EFFECTS {
			t.Errorf("%s 不是 NO_SIDE_EFFECTS", m.FullName())
		}
	}
}

func TestNewPublicRequiresWebOrigin(t *testing.T) {
	if _, err := NewPublic(&fakeReads{}, fakeBuyers{}, &fakeStores{}, rpc.Images{}, rpc.WebOrigin{}); err == nil {
		t.Fatal("零值 WebOrigin 應該失敗")
	}
}

func TestGetPostViewerCanCommentFollowsStoreBlocks(t *testing.T) {
	own := &seller.Store{StoreID: 9}
	cases := []struct {
		name  string
		guest bool
		store *seller.Store
		block map[int64]bool
		want  bool
	}{
		{"訪客", true, nil, nil, false},
		{"買家沒被封鎖", false, nil, nil, true},
		{"買家被這家店封鎖", false, nil, map[int64]bool{5: true}, false},
		{"店主讀自己店的貼文，即使有封鎖紀錄", false, own, map[int64]bool{9: true}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := post(5)
			if c.store != nil {
				p = post(9)
			}
			stores := &fakeStores{own: c.store, blocking: c.block}
			h := newPublic(t, &fakeReads{post: p}, fakeBuyers{id: 7}, stores)
			ctx := t.Context()
			if !c.guest {
				ctx = asBuyer(ctx)
			}
			resp, err := h.GetPost(ctx, connect.NewRequest(&pb.GetPostRequest{PostId: p.ID.String()}))
			if err != nil {
				t.Fatal(err)
			}
			got := resp.Msg.GetPost()
			if got.GetViewerCanComment() != c.want {
				t.Errorf("viewer_can_comment = %v，預期 %v", got.GetViewerCanComment(), c.want)
			}
			if got.GetUrl() != "https://web.example.test/feed?post="+p.ID.String() {
				t.Errorf("url = %q", got.GetUrl())
			}
			if got.GetStore().GetName() != "shop" || got.GetViewCount() != 3 {
				t.Errorf("post = %v", got)
			}
			if c.guest && len(stores.asked) != 0 {
				t.Errorf("訪客不該查封鎖：%v", stores.asked)
			}
		})
	}
}

func TestGetPostRejectsMalformedIDAndPropagatesNotFound(t *testing.T) {
	h := newPublic(t, &fakeReads{postErr: apperr.NotFound("x")}, fakeBuyers{}, &fakeStores{})
	if _, err := h.GetPost(t.Context(), connect.NewRequest(&pb.GetPostRequest{PostId: "nope"})); codeOf(t, err) != connect.CodeInvalidArgument {
		t.Errorf("err = %v，預期 invalid_argument", err)
	}
	if _, err := h.GetPost(t.Context(), connect.NewRequest(&pb.GetPostRequest{PostId: uuid.New().String()})); codeOf(t, err) != connect.CodeNotFound {
		t.Errorf("err = %v，預期 not found", err)
	}
}

func TestListPostsBatchesBlocksOncePerPageAndScopesTokens(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	posts := []social.Post{post(5), post(5), post(6)}
	reads := &fakeReads{feed: social.PostPage{Posts: posts, Next: &social.PostCursor{PublishedAt: at, ID: posts[2].ID}}}
	stores := &fakeStores{blocking: map[int64]bool{6: true}}
	h := newPublic(t, reads, fakeBuyers{id: 7}, stores)
	resp, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(stores.asked) != 1 || len(stores.asked[0]) != 2 {
		t.Fatalf("封鎖查詢 = %v，預期一次、兩家店", stores.asked)
	}
	if got := resp.Msg.GetPosts(); !got[0].GetViewerCanComment() || got[2].GetViewerCanComment() {
		t.Errorf("viewer_can_comment = %v %v %v", got[0].GetViewerCanComment(), got[1].GetViewerCanComment(), got[2].GetViewerCanComment())
	}
	token := resp.Msg.GetNextPageToken()
	if token == "" {
		t.Fatal("沒有 next_page_token")
	}
	storeID := "5"
	if _, err := h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{PageToken: token, StoreId: &storeID})); codeOf(t, err) != connect.CodeInvalidArgument {
		t.Errorf("換了 store_id 沿用 token：%v，預期 invalid_argument", err)
	}
	if _, err := h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{PageToken: token})); err != nil {
		t.Errorf("同一組篩選的 token：%v", err)
	}
	if _, err := h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{StoreId: &storeID})); err != nil || reads.storeID != 5 {
		t.Errorf("store_id 篩選：err=%v store=%d", err, reads.storeID)
	}
}

func TestListCommentsFlagsAndPlaceholders(t *testing.T) {
	p := post(5)
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	mine := social.Comment{ID: uuid.New(), PostID: p.ID, State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorBuyer, ID: 7}, Body: "a", CreatedAt: now, UpdatedAt: now}
	other := social.Comment{ID: uuid.New(), PostID: p.ID, State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorBuyer, ID: 8}, Body: "b", CreatedAt: now, UpdatedAt: now, LikedByMe: true}
	holder := social.Comment{ID: uuid.New(), PostID: p.ID, State: social.CommentDeleted, CreatedAt: now, UpdatedAt: now, ReplyCount: 2}
	reads := &fakeReads{post: p, comments: social.CommentPage{Comments: []social.Comment{mine, other, holder}}}

	t.Run("買家", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{id: 7}, &fakeStores{})
		resp, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()}))
		if err != nil {
			t.Fatal(err)
		}
		c := resp.Msg.GetComments()
		if !c[0].GetIsMine() || !c[0].GetViewerCanEdit() || !c[0].GetViewerCanDelete() || !c[0].GetViewerCanReply() {
			t.Errorf("自己的留言旗標 = %v", c[0])
		}
		if c[1].GetIsMine() || c[1].GetViewerCanEdit() || c[1].GetViewerCanDelete() || !c[1].GetViewerCanReply() || !c[1].GetViewerLiked() {
			t.Errorf("別人的留言旗標 = %v", c[1])
		}
		if c[1].GetAuthor().GetName() != "buyer" || c[1].GetAuthor().GetKind() != pb.ActorKind_ACTOR_KIND_BUYER {
			t.Errorf("作者 = %v", c[1].GetAuthor())
		}
		if g := c[2]; g.GetState() != pb.CommentState_COMMENT_STATE_DELETED || g.GetAuthor() != nil || g.GetBody() != "" ||
			g.GetViewerCanReply() || g.GetViewerCanEdit() || g.GetViewerCanDelete() || g.GetReplyCount() != 2 {
			t.Errorf("佔位項目 = %v", g)
		}
	})
	t.Run("訪客全部是 false", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
		resp, err := h.ListComments(t.Context(), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()}))
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range resp.Msg.GetComments() {
			if c.GetIsMine() || c.GetViewerCanEdit() || c.GetViewerCanDelete() || c.GetViewerCanReply() {
				t.Errorf("訪客看到的旗標 = %v", c)
			}
		}
	})
	t.Run("店主可以刪除別人的留言", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{id: 7}, &fakeStores{own: &seller.Store{StoreID: 5}})
		resp, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()}))
		if err != nil {
			t.Fatal(err)
		}
		if c := resp.Msg.GetComments()[1]; !c.GetViewerCanDelete() || c.GetViewerCanEdit() {
			t.Errorf("店主對別人的留言：%v", c)
		}
	})
	t.Run("被店家封鎖的買家不能回覆", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{id: 7}, &fakeStores{blocking: map[int64]bool{5: true}})
		resp, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()}))
		if err != nil {
			t.Fatal(err)
		}
		if c := resp.Msg.GetComments()[1]; c.GetViewerCanReply() {
			t.Errorf("被封鎖仍可回覆：%v", c)
		}
	})
	t.Run("回覆的貼文與 post_id 不符是 not_found", func(t *testing.T) {
		parent := uuid.New().String()
		reads := &fakeReads{post: p, replies: social.CommentPage{Comments: []social.Comment{{ID: uuid.New(), PostID: uuid.New(), State: social.CommentVisible, CreatedAt: now, UpdatedAt: now}}}}
		h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
		_, err := h.ListComments(t.Context(), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String(), ParentId: &parent}))
		if codeOf(t, err) != connect.CodeNotFound {
			t.Errorf("err = %v，預期 not found", err)
		}
	})
}

func TestSuggestTrendingPostsMaxResults(t *testing.T) {
	cases := []struct {
		in      int32
		want    int32
		invalid bool
	}{{0, DefaultTrending, false}, {5, 5, false}, {99, social.MaxTopPosts, false}, {-1, 0, true}}
	for _, c := range cases {
		reads := &fakeReads{top: []social.Post{post(5)}}
		h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
		resp, err := h.SuggestTrendingPosts(t.Context(), connect.NewRequest(&pb.SuggestTrendingPostsRequest{MaxResults: c.in}))
		if c.invalid {
			if codeOf(t, err) != connect.CodeInvalidArgument {
				t.Errorf("max_results=%d：err=%v", c.in, err)
			}
			continue
		}
		if err != nil || reads.topN != c.want || len(resp.Msg.GetPosts()) != 1 {
			t.Errorf("max_results=%d：err=%v n=%d", c.in, err, reads.topN)
		}
	}
}

// codeOf 是 interceptor 轉出的 Connect code（handler 回的是 apperr 或 rpc.Error，轉換在傳輸層）。
func codeOf(t *testing.T, err error) connect.Code {
	t.Helper()
	if err == nil {
		t.Fatal("預期有錯誤")
	}
	return rpc.ConnectError(t.Context(), err).Code()
}
