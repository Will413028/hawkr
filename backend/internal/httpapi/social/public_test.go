package social

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// readability 是測試用的公開可讀來源：預設全部可讀；unreadable 列出不可讀的貼文，forumClosed 讓討論區不可讀，asked 記下每次查的貼文。
type readability struct {
	unreadable  map[uuid.UUID]bool
	forumClosed bool
	asked       [][]uuid.UUID
	forumAsked  int
}

func (r *readability) PubliclyReadablePosts(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	r.asked = append(r.asked, ids)
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		if !r.unreadable[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (r *readability) ForumReadable(context.Context, social.Caller, int64) (bool, error) {
	r.forumAsked++
	return !r.forumClosed, nil
}

type fakeReads struct {
	readability
	post     social.Post
	postErr  error
	feed     social.PostPage
	storeID  int64
	comments social.CommentPage
	replies  social.CommentPage
	top      []social.Post
	topN     int32
	callers  []social.Caller

	tagFeed    social.PostPage
	tag        string
	tagCounts  []social.HashtagCount
	suggestQ   string
	suggestN   int32
	suggestSub bool
	from, to   time.Time
	trendN     int32
	forum      social.ForumPage
	forumStore int64
	forumReply social.ForumPage
	repliesTo  uuid.UUID

	commentParents map[uuid.UUID]uuid.UUID
	forumParents   map[uuid.UUID]int64
	repliesPost    uuid.UUID
	repliesStore   int64
	postReads      int
	likedIDs       map[uuid.UUID]bool
	likedAsked     [][]uuid.UUID
}

func (f *fakeReads) PostsLikedBy(_ context.Context, _ int64, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	f.likedAsked = append(f.likedAsked, ids)
	return f.likedIDs, nil
}

func (f *fakeReads) ListHashtagPosts(_ context.Context, _ social.Caller, tag string, _ *social.PostCursor, _ int32) (social.PostPage, error) {
	f.tag = tag
	return f.tagFeed, nil
}

func (f *fakeReads) SuggestHashtags(_ context.Context, q string, contains bool, limit int32) ([]social.HashtagCount, error) {
	f.suggestQ, f.suggestSub, f.suggestN = q, contains, limit
	return f.tagCounts, nil
}

func (f *fakeReads) TrendingHashtags(_ context.Context, from, to time.Time, limit int32) ([]social.HashtagCount, error) {
	f.from, f.to, f.trendN = from, to, limit
	return f.tagCounts, nil
}

func (f *fakeReads) ListForumEntries(_ context.Context, _ social.Caller, storeID int64, _ *social.CommentCursor, _ int32) (social.ForumPage, error) {
	f.forumStore = storeID
	return f.forum, nil
}

// ListForumReplies 與模組一樣先驗證父留言屬於 storeID：不存在或屬於別家店是 NotFound，不論頁面內容。
func (f *fakeReads) ListForumReplies(_ context.Context, _ social.Caller, storeID int64, parentID uuid.UUID, _ *social.CommentCursor, _ int32) (social.ForumPage, error) {
	f.repliesTo, f.repliesStore = parentID, storeID
	if owner, ok := f.forumParents[parentID]; f.forumParents != nil && (!ok || owner != storeID) {
		return social.ForumPage{}, apperr.NotFound("entry")
	}
	return f.forumReply, nil
}

func (f *fakeReads) PublicPost(_ context.Context, c social.Caller, _ uuid.UUID) (social.Post, error) {
	f.postReads++
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
	return f.comments, f.postErr
}

// ListReplies 與模組一樣先驗證父留言屬於 postID：不存在或屬於別篇是 NotFound，不論頁面內容。
func (f *fakeReads) ListReplies(_ context.Context, _ social.Caller, postID, parentID uuid.UUID, _ *social.CommentCursor, _ int32) (social.CommentPage, error) {
	f.repliesPost = postID
	if owner, ok := f.commentParents[parentID]; f.commentParents != nil && (!ok || owner != postID) {
		return social.CommentPage{}, apperr.NotFound("comment")
	}
	return f.replies, f.postErr
}

func (f *fakeReads) TopPosts(_ context.Context, _ social.Caller, n int32) ([]social.Post, error) {
	f.topN = n
	return f.top, nil
}

type fakeBuyers struct {
	id int64
	// displayCalls 非 nil 時記錄 BatchDisplay 被呼叫的次數。
	displayCalls *int
	// displayIDs 非 nil 時記錄每次 BatchDisplay 收到的 id。
	displayIDs *[][]int64
}

func (f fakeBuyers) CurrentBuyerID(context.Context, int64) (int64, error) { return f.id, nil }

func (f fakeBuyers) BatchDisplay(_ context.Context, ids []int64) ([]buyer.Display, error) {
	if f.displayCalls != nil {
		*f.displayCalls++
	}
	if f.displayIDs != nil {
		*f.displayIDs = append(*f.displayIDs, slices.Clone(ids))
	}
	out := make([]buyer.Display, len(ids))
	for i, id := range ids {
		name := "buyer"
		out[i] = buyer.Display{BuyerID: id, DisplayName: &name}
	}
	return out, nil
}

type fakeStores struct {
	own          *seller.Store
	blocking     map[int64]bool
	asked        [][]int64
	cardCalls    int
	storeLookups int
}

func (f *fakeStores) CurrentStore(context.Context, int64) (seller.Store, error) {
	f.storeLookups++
	if f.own == nil {
		return seller.Store{}, apperr.NotFound("no store")
	}
	return *f.own, nil
}

func (f *fakeStores) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	f.cardCalls++
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

// fakeProducts 回 cards 裡有的商品（沒有的就是缺少），記錄每次批次查詢的 ID 與 includeInactive。
type fakeProducts struct {
	cards map[int64]ProductInfo
	asked [][]int64
	incl  []bool
	err   error
}

func (f *fakeProducts) Cards(_ context.Context, ids []int64, includeInactive bool) (map[int64]ProductInfo, error) {
	f.asked = append(f.asked, ids)
	f.incl = append(f.incl, includeInactive)
	out := map[int64]ProductInfo{}
	for _, id := range ids {
		if c, ok := f.cards[id]; ok {
			out[id] = c
		}
	}
	return out, f.err
}

func newPublic(t *testing.T, r *fakeReads, b fakeBuyers, s *fakeStores) *Public {
	t.Helper()
	return newPublicWithViews(t, r, &fakeViews{}, b, s)
}

func newPublicWithViews(t *testing.T, r *fakeReads, v *fakeViews, b fakeBuyers, s *fakeStores) *Public {
	t.Helper()
	return newPublicWithProducts(t, r, v, b, s, &fakeProducts{})
}

func newPublicWithProducts(t *testing.T, r *fakeReads, v *fakeViews, b fakeBuyers, s *fakeStores, products Products) *Public {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	web, err := rpc.NewWebOrigin("https://web.example.test")
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewPublic(r, v, b, s, products, images, web, func() time.Time { return trendNow }, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

var trendNow = time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)

func asBuyer(ctx context.Context) context.Context {
	return rpc.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalAccount, ID: 7})
}

func post(storeID int64) social.Post {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return social.Post{ID: uuid.New(), StoreID: storeID, Body: "hi", PublishedAt: &at, UpdatedAt: at, Stats: social.PostStats{Views: 3}}
}

// 讀取一律 NO_SIDE_EFFECTS（GET）；RecordPostView 是唯一的寫入，POST、不標 NO_SIDE_EFFECTS（規範第三節：讀取不得順便寫入）。
func TestOnlyRecordPostViewIsAWriteInSocialPublicService(t *testing.T) {
	methods := pb.File_hawkr_social_v1alpha1_social_proto.Services().ByName("SocialPublicService").Methods()
	if methods.Len() == 0 {
		t.Fatal("沒有方法")
	}
	sawView := false
	for i := range methods.Len() {
		m := methods.Get(i)
		opts, ok := m.Options().(*descriptorpb.MethodOptions)
		noSideEffects := ok && opts.GetIdempotencyLevel() == descriptorpb.MethodOptions_NO_SIDE_EFFECTS
		if m.Name() == "RecordPostView" {
			sawView = true
			if noSideEffects {
				t.Errorf("%s 會寫入，不能標 NO_SIDE_EFFECTS", m.FullName())
			}
			continue
		}
		if !noSideEffects {
			t.Errorf("%s 不是 NO_SIDE_EFFECTS", m.FullName())
		}
	}
	if !sawView {
		t.Error("沒有 RecordPostView")
	}
}

// SocialAppService 只有店主的五個讀取方法（含 ListPendingSchedules）與作者讀自己處分的兩個方法標 NO_SIDE_EFFECTS（GET）；其餘都是寫入，不得標。
func TestOnlyOwnerReadsInSocialAppServiceAreNoSideEffects(t *testing.T) {
	methods := pb.File_hawkr_social_v1alpha1_social_proto.Services().ByName("SocialAppService").Methods()
	if methods.Len() != 25 {
		t.Fatalf("SocialAppService 有 %d 支方法，預期 25", methods.Len())
	}
	reads := map[string]bool{"GetMyPost": true, "ListMyPosts": true, "ListStoreComments": true, "GetStoreSocialSummary": true, "ListPendingSchedules": true, "ListMySanctions": true, "ListMyHiddenContent": true}
	seen := 0
	for i := range methods.Len() {
		m := methods.Get(i)
		opts, _ := m.Options().(*descriptorpb.MethodOptions)
		noSideEffects := opts.GetIdempotencyLevel() == descriptorpb.MethodOptions_NO_SIDE_EFFECTS
		switch {
		case reads[string(m.Name())]:
			seen++
			if !noSideEffects {
				t.Errorf("%s 是讀取，要標 NO_SIDE_EFFECTS", m.FullName())
			}
		case noSideEffects:
			t.Errorf("%s 是寫入，不能標 NO_SIDE_EFFECTS", m.FullName())
		}
	}
	if seen != len(reads) {
		t.Errorf("只找到 %d 個讀取方法", seen)
	}
}

func TestNewPublicRequiresWebOrigin(t *testing.T) {
	if _, err := NewPublic(&fakeReads{}, &fakeViews{}, fakeBuyers{}, &fakeStores{}, &fakeProducts{}, rpc.Images{}, rpc.WebOrigin{}, nil, slog.New(slog.DiscardHandler)); err == nil {
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

func TestPostViewerLikedFollowsTheBuyersLikes(t *testing.T) {
	liked, other := post(5), post(5)
	reads := &fakeReads{feed: social.PostPage{Posts: []social.Post{liked, other}}, post: liked, likedIDs: map[uuid.UUID]bool{liked.ID: true}}
	h := newPublic(t, reads, fakeBuyers{id: 7}, &fakeStores{})
	resp, err := h.GetPost(asBuyer(t.Context()), connect.NewRequest(&pb.GetPostRequest{PostId: liked.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Msg.GetPost().GetViewerLiked() {
		t.Errorf("買家讚過的貼文 viewer_liked = false")
	}
	list, err := h.ListPosts(asBuyer(t.Context()), connect.NewRequest(&pb.ListPostsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := list.Msg.GetPosts(); !got[0].GetViewerLiked() || got[1].GetViewerLiked() {
		t.Errorf("viewer_liked = %v %v，預期 true false", got[0].GetViewerLiked(), got[1].GetViewerLiked())
	}
	if len(reads.likedAsked) != 2 || len(reads.likedAsked[1]) != 2 {
		t.Errorf("讚的查詢 = %v，預期每次請求一次、列表一次帶兩篇", reads.likedAsked)
	}
	before := len(reads.likedAsked)
	guest, err := h.GetPost(t.Context(), connect.NewRequest(&pb.GetPostRequest{PostId: liked.ID.String()}))
	if err != nil {
		t.Fatal(err)
	}
	if guest.Msg.GetPost().GetViewerLiked() || len(reads.likedAsked) != before {
		t.Errorf("訪客 viewer_liked = true 或查了讚")
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
	holder := social.Comment{ID: uuid.New(), PostID: p.ID, State: social.CommentHidden, CreatedAt: now, UpdatedAt: now, ReplyCount: 2, Likes: 9, Body: "leak", LikedByMe: true, Actor: social.Actor{Kind: social.ActorBuyer, ID: 8}}
	reads := &fakeReads{post: p, comments: social.CommentPage{Comments: []social.Comment{mine, other, holder}, StoreID: 5}}

	t.Run("買家", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{id: 7}, &fakeStores{})
		resp, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()}))
		if err != nil {
			t.Fatal(err)
		}
		c := resp.Msg.GetComments()
		if !c[0].GetIsMine() || !c[0].GetViewerCanEdit() || !c[0].GetViewerCanDelete() || c[0].GetViewerCanReply() {
			t.Errorf("自己的留言旗標 = %v", c[0])
		}
		if c[1].GetIsMine() || c[1].GetViewerCanEdit() || c[1].GetViewerCanDelete() || c[1].GetViewerCanReply() || !c[1].GetViewerLiked() {
			t.Errorf("別人的留言旗標 = %v", c[1])
		}
		if c[1].GetAuthor().GetName() != "buyer" || c[1].GetAuthor().GetKind() != pb.ActorKind_ACTOR_KIND_BUYER {
			t.Errorf("作者 = %v", c[1].GetAuthor())
		}
		want := &pb.Comment{Id: holder.ID.String(), PostId: p.ID.String(), State: pb.CommentState_COMMENT_STATE_HIDDEN, CreateTime: timestamppb.New(now), ReplyCount: 2}
		if g := c[2]; !proto.Equal(g, want) {
			t.Errorf("佔位項目只有 id、post_id、create_time、reply_count、state：%v", g)
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
		if c := resp.Msg.GetComments()[1]; !c.GetViewerCanDelete() || c.GetViewerCanEdit() || !c.GetViewerCanReply() {
			t.Errorf("店主對別人的留言：%v", c)
		}
	})
	t.Run("只有貼文所屬的店能回覆，買家不能", func(t *testing.T) {
		for name, stores := range map[string]*fakeStores{
			"沒有被封鎖的買家": {},
			"被店家封鎖的買家": {blocking: map[int64]bool{5: true}},
			"別家店的店主":   {own: &seller.Store{StoreID: 6}},
		} {
			h := newPublic(t, reads, fakeBuyers{id: 7}, stores)
			resp, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()}))
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range resp.Msg.GetComments() {
				if c.GetViewerCanReply() {
					t.Errorf("%s：viewer_can_reply 應為 false：%v", name, c)
				}
			}
		}
	})
	t.Run("回覆：父留言不存在或屬於別篇貼文是 not_found，即使頁面是空的", func(t *testing.T) {
		known := uuid.New()
		reads := &fakeReads{commentParents: map[uuid.UUID]uuid.UUID{known: uuid.New()}, postErr: nil}
		h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
		for name, parent := range map[string]string{"不存在": uuid.New().String(), "別篇貼文的": known.String()} {
			_, err := h.ListComments(t.Context(), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String(), ParentId: &parent}))
			if codeOf(t, err) != connect.CodeNotFound {
				t.Errorf("%s父留言：err = %v，預期 not found", name, err)
			}
		}
		reads.commentParents[known] = p.ID
		parent := known.String()
		if _, err := h.ListComments(t.Context(), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String(), ParentId: &parent})); err != nil || reads.repliesPost != p.ID {
			t.Errorf("正確的父留言：err=%v post=%v", err, reads.repliesPost)
		}
	})
	t.Run("不另外讀貼文，店 id 取自模組回傳的頁", func(t *testing.T) {
		reads := &fakeReads{comments: social.CommentPage{Comments: []social.Comment{other}, StoreID: 5}}
		stores := &fakeStores{}
		h := newPublic(t, reads, fakeBuyers{id: 7}, stores)
		if _, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()})); err != nil || reads.postReads != 0 {
			t.Fatalf("err=%v PublicPost 讀了 %d 次", err, reads.postReads)
		}
		if len(stores.asked) != 0 {
			t.Errorf("留言列表不查封鎖：%v", stores.asked)
		}
	})
	t.Run("空頁與只有佔位項目不查店家與買家", func(t *testing.T) {
		stores := &fakeStores{}
		reads := &fakeReads{comments: social.CommentPage{StoreID: 5}}
		h := newPublic(t, reads, fakeBuyers{id: 7}, stores)
		if _, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()})); err != nil || len(stores.asked) != 0 {
			t.Fatalf("空頁：err=%v asked=%v", err, stores.asked)
		}
		reads.comments = social.CommentPage{StoreID: 5, Comments: []social.Comment{holder}}
		if _, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()})); err != nil || len(stores.asked) != 0 {
			t.Fatalf("只有佔位項目：err=%v asked=%v", err, stores.asked)
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

func TestListPostsHashtagFilter(t *testing.T) {
	p := post(5)
	reads := &fakeReads{tagFeed: social.PostPage{Posts: []social.Post{p}}, feed: social.PostPage{Posts: []social.Post{post(6), post(6)}}}
	h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
	tag := "#Cake"
	resp, err := h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{Hashtag: &tag}))
	if err != nil || reads.tag != "#Cake" || len(resp.Msg.GetPosts()) != 1 || resp.Msg.GetPosts()[0].GetId() != p.ID.String() {
		t.Fatalf("hashtag 篩選：err=%v tag=%q posts=%v", err, reads.tag, resp.Msg.GetPosts())
	}
	store := "5"
	if _, err := h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{Hashtag: &tag, StoreId: &store})); codeOf(t, err) != connect.CodeInvalidArgument {
		t.Errorf("hashtag 與 store_id 同時設：%v，預期 invalid_argument", err)
	}
	reads.tagFeed.Next = &social.PostCursor{PublishedAt: trendNow, ID: p.ID}
	resp, err = h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{Hashtag: &tag}))
	if err != nil || resp.Msg.GetNextPageToken() == "" {
		t.Fatalf("沒有 token：%v", err)
	}
	lower := "cake"
	if _, err := h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{Hashtag: &lower, PageToken: resp.Msg.GetNextPageToken()})); err != nil {
		t.Errorf("第一頁 %q、第二頁 %q 正規化後相同，token 應該通用：%v", tag, lower, err)
	}
	other := "Pie"
	if _, err := h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{Hashtag: &other, PageToken: resp.Msg.GetNextPageToken()})); codeOf(t, err) != connect.CodeInvalidArgument {
		t.Errorf("換了 hashtag 沿用 token：%v", err)
	}
	if _, err := h.ListPosts(t.Context(), connect.NewRequest(&pb.ListPostsRequest{PageToken: resp.Msg.GetNextPageToken()})); codeOf(t, err) != connect.CodeInvalidArgument {
		t.Errorf("拿標籤的 token 讀動態牆：%v", err)
	}
}

func TestSuggestTags(t *testing.T) {
	reads := &fakeReads{tagCounts: []social.HashtagCount{{Name: "cake", Posts: 4}}}
	h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
	resp, err := h.SuggestTags(t.Context(), connect.NewRequest(&pb.SuggestTagsRequest{Query: "#ca", Contains: true, MaxResults: 7}))
	if err != nil || reads.suggestQ != "#ca" || !reads.suggestSub || reads.suggestN != 7 {
		t.Fatalf("err=%v q=%q contains=%v n=%d", err, reads.suggestQ, reads.suggestSub, reads.suggestN)
	}
	if got := resp.Msg.GetTags(); len(got) != 1 || got[0].GetName() != "cake" || got[0].GetPostCount() != 4 {
		t.Errorf("tags = %v", got)
	}
	if _, err := h.SuggestTags(t.Context(), connect.NewRequest(&pb.SuggestTagsRequest{Query: "c", MaxResults: -1})); codeOf(t, err) != connect.CodeInvalidArgument {
		t.Errorf("max_results=-1：%v", err)
	}
}

func TestSuggestTrendingTagsWindow(t *testing.T) {
	reads := &fakeReads{tagCounts: []social.HashtagCount{{Name: "cake", Posts: 4}}}
	h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
	if _, err := h.SuggestTrendingTags(t.Context(), connect.NewRequest(&pb.SuggestTrendingTagsRequest{})); err != nil ||
		!reads.to.Equal(trendNow) || !reads.from.Equal(trendNow.Add(-social.DefaultTrendingWindow)) {
		t.Fatalf("預設時間窗：err=%v %v ~ %v", err, reads.from, reads.to)
	}
	start, end := trendNow.Add(-48*time.Hour), trendNow.Add(-24*time.Hour)
	resp, err := h.SuggestTrendingTags(t.Context(), connect.NewRequest(&pb.SuggestTrendingTagsRequest{StartTime: timestamppb.New(start), EndTime: timestamppb.New(end), MaxResults: 3}))
	if err != nil || !reads.from.Equal(start) || !reads.to.Equal(end) || reads.trendN != 3 || resp.Msg.GetTags()[0].GetPostCount() != 4 {
		t.Fatalf("指定時間窗：err=%v %v ~ %v n=%d", err, reads.from, reads.to, reads.trendN)
	}
	if _, err := h.SuggestTrendingTags(t.Context(), connect.NewRequest(&pb.SuggestTrendingTagsRequest{MaxResults: -1})); codeOf(t, err) != connect.CodeInvalidArgument {
		t.Errorf("max_results=-1：%v", err)
	}
}

func TestListForumEntriesFlagsAndScoping(t *testing.T) {
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	entry := func(store int64, state social.CommentState, buyerID int64) social.ForumEntry {
		return social.ForumEntry{ID: uuid.New(), StoreID: store, State: state, Actor: social.Actor{Kind: social.ActorBuyer, ID: buyerID}, Body: "x", CreatedAt: now, UpdatedAt: now}
	}
	mine, other, holder := entry(5, social.CommentVisible, 7), entry(5, social.CommentVisible, 8), entry(5, social.CommentDeleted, 8)
	holder.Body, holder.UpdatedAt = "leak", now.Add(time.Hour)
	reads := &fakeReads{forum: social.ForumPage{Entries: []social.ForumEntry{mine, other, holder}, Next: &social.CommentCursor{CreatedAt: now, ID: holder.ID}}}
	id := "5"

	t.Run("買家", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{id: 7}, &fakeStores{})
		resp, err := h.ListForumEntries(asBuyer(t.Context()), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id}))
		if err != nil || reads.forumStore != 5 {
			t.Fatalf("err=%v store=%d", err, reads.forumStore)
		}
		e := resp.Msg.GetEntries()
		if !e[0].GetIsMine() || !e[0].GetViewerCanEdit() || !e[0].GetViewerCanDelete() || !e[0].GetViewerCanReply() {
			t.Errorf("自己的 = %v", e[0])
		}
		if e[1].GetIsMine() || e[1].GetViewerCanEdit() || e[1].GetViewerCanDelete() || !e[1].GetViewerCanReply() {
			t.Errorf("別人的 = %v", e[1])
		}
		want := &pb.ForumEntry{Id: holder.ID.String(), StoreId: "5", State: pb.CommentState_COMMENT_STATE_DELETED, CreateTime: timestamppb.New(now)}
		if g := e[2]; !proto.Equal(g, want) {
			t.Errorf("佔位只有 id、store_id、create_time、reply_count、state：%v", g)
		}
		if resp.Msg.GetNextPageToken() == "" {
			t.Error("沒有 next_page_token")
		}
	})
	t.Run("被店家封鎖的買家不能回覆", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{id: 7}, &fakeStores{blocking: map[int64]bool{5: true}})
		resp, err := h.ListForumEntries(asBuyer(t.Context()), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id}))
		if err != nil || resp.Msg.GetEntries()[1].GetViewerCanReply() {
			t.Errorf("err=%v", err)
		}
	})
	t.Run("訪客", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
		resp, err := h.ListForumEntries(t.Context(), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id}))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range resp.Msg.GetEntries() {
			if e.GetIsMine() || e.GetViewerCanEdit() || e.GetViewerCanDelete() || e.GetViewerCanReply() {
				t.Errorf("訪客旗標 = %v", e)
			}
		}
	})
	t.Run("回覆", func(t *testing.T) {
		known := uuid.New()
		reads := &fakeReads{forumParents: map[uuid.UUID]int64{known: 5}, forumReply: social.ForumPage{Entries: []social.ForumEntry{entry(5, social.CommentVisible, 8)}}}
		h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
		parent := known.String()
		if _, err := h.ListForumEntries(t.Context(), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id, ParentId: &parent})); err != nil || reads.repliesTo != known || reads.repliesStore != 5 {
			t.Errorf("err=%v parent=%v store=%d", err, reads.repliesTo, reads.repliesStore)
		}
	})
	t.Run("回覆：父留言不存在或屬於別家店是 not_found，即使頁面是空的", func(t *testing.T) {
		known := uuid.New()
		reads := &fakeReads{forumParents: map[uuid.UUID]int64{known: 6}}
		h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
		for name, parent := range map[string]string{"不存在": uuid.New().String(), "別家店的": known.String()} {
			if _, err := h.ListForumEntries(t.Context(), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id, ParentId: &parent})); codeOf(t, err) != connect.CodeNotFound {
				t.Errorf("%s父留言：%v，預期 not_found", name, err)
			}
		}
	})
	t.Run("空頁只為觀看者身分查一次封鎖", func(t *testing.T) {
		stores := &fakeStores{}
		h := newPublic(t, &fakeReads{}, fakeBuyers{id: 7}, stores)
		if _, err := h.ListForumEntries(asBuyer(t.Context()), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id})); err != nil || len(stores.asked) != 1 || len(stores.asked[0]) != 1 || stores.asked[0][0] != 5 {
			t.Errorf("err=%v asked=%v，預期只問觀看者要不要被店 5 封鎖", err, stores.asked)
		}
	})
	t.Run("非空頁只查一次封鎖，觀看者與作者的名稱併成一次 BatchDisplay，店家作者查卡片，viewer_can_comment 與 viewer_can_reply 同源", func(t *testing.T) {
		storeAuthored := social.ForumEntry{ID: uuid.New(), StoreID: 5, State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorStore, ID: 5}, Body: "s", CreatedAt: now, UpdatedAt: now}
		mixed := &fakeReads{forum: social.ForumPage{Entries: []social.ForumEntry{entry(5, social.CommentVisible, 8), entry(5, social.CommentVisible, 9), storeAuthored}}}
		stores := &fakeStores{blocking: map[int64]bool{5: true}}
		var displayCalls int
		var displayIDs [][]int64
		h := newPublic(t, mixed, fakeBuyers{id: 7, displayCalls: &displayCalls, displayIDs: &displayIDs}, stores)
		resp, err := h.ListForumEntries(asBuyer(t.Context()), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id}))
		if err != nil {
			t.Fatal(err)
		}
		if len(stores.asked) != 1 || stores.cardCalls != 1 || displayCalls != 1 {
			t.Errorf("asked=%v cardCalls=%d displayCalls=%d，預期封鎖 1 次、卡片 1 次、名稱 1 次", stores.asked, stores.cardCalls, displayCalls)
		}
		if len(displayIDs) != 1 || !slices.Equal(uniqSorted(displayIDs[0]), []int64{7, 8, 9}) {
			t.Errorf("BatchDisplay 收到 %v，預期一次且同時含觀看者 7 與作者 8、9", displayIDs)
		}
		if resp.Msg.GetViewerCanComment() || resp.Msg.GetEntries()[0].GetViewerCanReply() {
			t.Errorf("封鎖時兩者都應是 false：%v %v", resp.Msg.GetViewerCanComment(), resp.Msg.GetEntries()[0].GetViewerCanReply())
		}
		if resp.Msg.GetViewerActor().GetKind() != pb.ActorKind_ACTOR_KIND_BUYER {
			t.Errorf("被封鎖的買家仍有身分：%v", resp.Msg.GetViewerActor())
		}
	})
	t.Run("空頁與全隱藏頁的 viewer_can_comment 取自觀看者而不是頁面內容", func(t *testing.T) {
		hidden := social.ForumPage{Entries: []social.ForumEntry{entry(5, social.CommentHidden, 8), entry(5, social.CommentDeleted, 8)}}
		own := &seller.Store{StoreID: 5}
		cases := []struct {
			name  string
			store *seller.Store
			block map[int64]bool
			want  bool
			asks  int
		}{
			{"未被封鎖的買家", nil, nil, true, 1},
			{"被封鎖的買家", nil, map[int64]bool{5: true}, false, 1},
			{"店主", own, nil, true, 0},
		}
		for pageName, page := range map[string]social.ForumPage{"空頁": {}, "全隱藏頁": hidden} {
			for _, c := range cases {
				stores := &fakeStores{own: c.store, blocking: c.block}
				h := newPublic(t, &fakeReads{forum: page}, fakeBuyers{id: 7}, stores)
				resp, err := h.ListForumEntries(asBuyer(t.Context()), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id}))
				if err != nil {
					t.Fatal(err)
				}
				if resp.Msg.GetViewerCanComment() != c.want || len(stores.asked) != c.asks {
					t.Errorf("%s/%s：viewer_can_comment=%v asked=%v，預期 %v 與 %d 次封鎖查詢", pageName, c.name, resp.Msg.GetViewerCanComment(), stores.asked, c.want, c.asks)
				}
			}
		}
	})
	t.Run("格式與 token", func(t *testing.T) {
		h := newPublic(t, reads, fakeBuyers{}, &fakeStores{})
		if _, err := h.ListForumEntries(t.Context(), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: "abc"})); codeOf(t, err) != connect.CodeInvalidArgument {
			t.Errorf("store_id 格式：%v", err)
		}
		resp, _ := h.ListForumEntries(t.Context(), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: id}))
		if _, err := h.ListForumEntries(t.Context(), connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: "6", PageToken: resp.Msg.GetNextPageToken()})); codeOf(t, err) != connect.CodeInvalidArgument {
			t.Errorf("換店沿用 token：%v", err)
		}
	})
}

// codeOf 是 interceptor 轉出的 Connect code（handler 回的是 apperr 或 rpc.Error，轉換在傳輸層）。
func codeOf(t *testing.T, err error) connect.Code {
	t.Helper()
	if err == nil {
		t.Fatal("預期有錯誤")
	}
	return rpc.ConnectError(t.Context(), err).Code()
}

type viewCall struct {
	caller social.Caller
	post   uuid.UUID
	v      social.Viewer
}

type fakeViews struct {
	calls []viewCall
	err   error
}

func (f *fakeViews) RecordView(_ context.Context, c social.Caller, postID uuid.UUID, v social.Viewer) (bool, error) {
	f.calls = append(f.calls, viewCall{c, postID, v})
	return true, f.err
}

// 瀏覽只由 RecordPostView 記：讀取貼文（GetPost）不計。
func TestGetPostDoesNotRecordAViewButRecordPostViewDoes(t *testing.T) {
	p := post(5)
	views := &fakeViews{}
	h := newPublicWithViews(t, &fakeReads{post: p}, views, fakeBuyers{id: 7}, &fakeStores{})
	if _, err := h.GetPost(asBuyer(t.Context()), connect.NewRequest(&pb.GetPostRequest{PostId: p.ID.String()})); err != nil {
		t.Fatal(err)
	}
	if len(views.calls) != 0 {
		t.Fatalf("GetPost 不該計瀏覽：%v", views.calls)
	}
	if _, err := h.RecordPostView(asBuyer(t.Context()), connect.NewRequest(&pb.RecordPostViewRequest{PostId: p.ID.String()})); err != nil {
		t.Fatal(err)
	}
	if len(views.calls) != 1 || views.calls[0].post != p.ID {
		t.Fatalf("RecordPostView 應計一次：%v", views.calls)
	}
}

func TestRecordPostViewBuildsTheViewer(t *testing.T) {
	p := post(5)
	t.Run("訪客以來源與 User-Agent 計為匿名", func(t *testing.T) {
		views := &fakeViews{}
		h := newPublicWithViews(t, &fakeReads{post: p}, views, fakeBuyers{id: 7}, &fakeStores{})
		req := connect.NewRequest(&pb.RecordPostViewRequest{PostId: p.ID.String()})
		req.Header().Set("User-Agent", "ua-1")
		if _, err := h.RecordPostView(rpc.WithClientIP(t.Context(), "203.0.113.9"), req); err != nil {
			t.Fatal(err)
		}
		want := social.Viewer{IP: "203.0.113.9", UserAgent: "ua-1"}
		if len(views.calls) != 1 || views.calls[0].v != want {
			t.Fatalf("viewer = %+v，預期 %+v", views.calls, want)
		}
	})
	t.Run("登入的買家帶帳號與買家身分", func(t *testing.T) {
		views := &fakeViews{}
		h := newPublicWithViews(t, &fakeReads{post: p}, views, fakeBuyers{id: 11}, &fakeStores{})
		if _, err := h.RecordPostView(asBuyer(t.Context()), connect.NewRequest(&pb.RecordPostViewRequest{PostId: p.ID.String()})); err != nil {
			t.Fatal(err)
		}
		if len(views.calls) != 1 || views.calls[0].v.AccountID != 7 || views.calls[0].caller.BuyerID != 11 {
			t.Fatalf("viewer = %+v", views.calls)
		}
	})
	t.Run("店主的店交給模組排除，handler 不預讀貼文", func(t *testing.T) {
		views := &fakeViews{}
		reads := &fakeReads{post: post(9)}
		h := newPublicWithViews(t, reads, views, fakeBuyers{id: 7}, &fakeStores{own: &seller.Store{StoreID: 9}})
		id := uuid.New()
		if _, err := h.RecordPostView(asBuyer(t.Context()), connect.NewRequest(&pb.RecordPostViewRequest{PostId: id.String()})); err != nil {
			t.Fatal(err)
		}
		if len(views.calls) != 1 || views.calls[0].post != id || !views.calls[0].caller.OwnsStore(9) {
			t.Fatalf("模組應收到店主的 Caller：%+v", views.calls)
		}
		if reads.postReads != 0 {
			t.Fatalf("handler 不該為了排除店主預讀貼文：%d 次", reads.postReads)
		}
	})
	t.Run("管理員代行的讀取不記錄", func(t *testing.T) {
		views := &fakeViews{}
		h := newPublicWithViews(t, &fakeReads{post: p}, views, fakeBuyers{}, &fakeStores{})
		ctx := rpc.WithPrincipal(rpc.WithClientIP(t.Context(), "203.0.113.9"), identity.Principal{Kind: identity.PrincipalAdmin, ID: 3})
		if _, err := h.RecordPostView(ctx, connect.NewRequest(&pb.RecordPostViewRequest{PostId: p.ID.String()})); err != nil {
			t.Fatal(err)
		}
		if len(views.calls) != 0 {
			t.Fatalf("管理員的瀏覽不該計：%v", views.calls)
		}
	})
	t.Run("格式錯誤的 ID", func(t *testing.T) {
		h := newPublic(t, &fakeReads{}, fakeBuyers{}, &fakeStores{})
		if _, err := h.RecordPostView(t.Context(), connect.NewRequest(&pb.RecordPostViewRequest{PostId: "x"})); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPostViewerActorFollowsViewerIdentity(t *testing.T) {
	own := &seller.Store{StoreID: 9}
	cases := []struct {
		name     string
		guest    bool
		store    *seller.Store
		postShop int64
		block    map[int64]bool
		wantKind pb.ActorKind
		wantID   string
		wantName string
	}{
		{"訪客不設", true, nil, 5, nil, pb.ActorKind_ACTOR_KIND_UNSPECIFIED, "", ""},
		{"買家是買家身分", false, nil, 5, nil, pb.ActorKind_ACTOR_KIND_BUYER, "7", "buyer"},
		{"被封鎖的買家身分不變", false, nil, 5, map[int64]bool{5: true}, pb.ActorKind_ACTOR_KIND_BUYER, "7", "buyer"},
		{"店主讀自己店的貼文是店家身分", false, own, 9, nil, pb.ActorKind_ACTOR_KIND_STORE, "9", "shop"},
		{"店主讀別家店的貼文是買家身分", false, own, 5, nil, pb.ActorKind_ACTOR_KIND_BUYER, "7", "buyer"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			check := func(t *testing.T, got *pb.Post) {
				t.Helper()
				a := got.GetViewerActor()
				if c.wantKind == pb.ActorKind_ACTOR_KIND_UNSPECIFIED {
					if a != nil {
						t.Errorf("viewer_actor = %v，預期不設", a)
					}
					return
				}
				if a.GetKind() != c.wantKind || a.GetId() != c.wantID || a.GetName() != c.wantName {
					t.Errorf("viewer_actor = %v，預期 kind=%v id=%s name=%s", a, c.wantKind, c.wantID, c.wantName)
				}
			}
			p := post(c.postShop)
			ctx := t.Context()
			if !c.guest {
				ctx = asBuyer(ctx)
			}
			h := newPublic(t, &fakeReads{post: p, feed: social.PostPage{Posts: []social.Post{p}}}, fakeBuyers{id: 7}, &fakeStores{own: c.store, blocking: c.block})
			single, err := h.GetPost(ctx, connect.NewRequest(&pb.GetPostRequest{PostId: p.ID.String()}))
			if err != nil {
				t.Fatal(err)
			}
			check(t, single.Msg.GetPost())
			list, err := h.ListPosts(ctx, connect.NewRequest(&pb.ListPostsRequest{}))
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Msg.GetPosts()) != 1 {
				t.Fatalf("posts = %d", len(list.Msg.GetPosts()))
			}
			check(t, list.Msg.GetPosts()[0])
		})
	}
}

func TestForumListingViewerActorFollowsViewerIdentity(t *testing.T) {
	own := &seller.Store{StoreID: 9}
	cases := []struct {
		name       string
		guest      bool
		store      *seller.Store
		forumShop  int64
		block      map[int64]bool
		wantKind   pb.ActorKind
		wantID     string
		wantName   string
		wantCanCmt bool
	}{
		{"訪客不設", true, nil, 5, nil, pb.ActorKind_ACTOR_KIND_UNSPECIFIED, "", "", false},
		{"買家是買家身分", false, nil, 5, nil, pb.ActorKind_ACTOR_KIND_BUYER, "7", "buyer", true},
		{"被封鎖的買家身分不變但不能留言", false, nil, 5, map[int64]bool{5: true}, pb.ActorKind_ACTOR_KIND_BUYER, "7", "buyer", false},
		{"店主讀自己店的討論區是店家身分", false, own, 9, nil, pb.ActorKind_ACTOR_KIND_STORE, "9", "shop", true},
		{"店主讀別家店的討論區是買家身分", false, own, 5, nil, pb.ActorKind_ACTOR_KIND_BUYER, "7", "buyer", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := t.Context()
			if !c.guest {
				ctx = asBuyer(ctx)
			}
			// 空的一頁也要帶觀看者身分。
			h := newPublic(t, &fakeReads{forum: social.ForumPage{}}, fakeBuyers{id: 7}, &fakeStores{own: c.store, blocking: c.block})
			resp, err := h.ListForumEntries(ctx, connect.NewRequest(&pb.ListForumEntriesRequest{StoreId: rpc.FormatID(c.forumShop)}))
			if err != nil {
				t.Fatal(err)
			}
			a := resp.Msg.GetViewerActor()
			if c.wantKind == pb.ActorKind_ACTOR_KIND_UNSPECIFIED {
				if a != nil {
					t.Errorf("viewer_actor = %v，預期不設", a)
				}
			} else if a.GetKind() != c.wantKind || a.GetId() != c.wantID || a.GetName() != c.wantName {
				t.Errorf("viewer_actor = %v，預期 kind=%v id=%s name=%s", a, c.wantKind, c.wantID, c.wantName)
			}
			if resp.Msg.GetViewerCanComment() != c.wantCanCmt {
				t.Errorf("viewer_can_comment = %v，預期 %v", resp.Msg.GetViewerCanComment(), c.wantCanCmt)
			}
		})
	}
}

func uniqSorted(ids []int64) []int64 {
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

func TestListCommentsDoesNotLookUpBlocking(t *testing.T) {
	p := post(5)
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	c := social.Comment{ID: uuid.New(), PostID: p.ID, State: social.CommentVisible, Actor: social.Actor{Kind: social.ActorBuyer, ID: 8}, Body: "b", CreatedAt: now, UpdatedAt: now}
	stores := &fakeStores{}
	h := newPublic(t, &fakeReads{post: p, comments: social.CommentPage{Comments: []social.Comment{c}, StoreID: 5}}, fakeBuyers{id: 7}, stores)
	if _, err := h.ListComments(asBuyer(t.Context()), connect.NewRequest(&pb.ListCommentsRequest{PostId: p.ID.String()})); err != nil {
		t.Fatal(err)
	}
	if len(stores.asked) != 0 {
		t.Errorf("留言列表查了封鎖 %v，viewer_can_reply 只看是不是店主，不需要", stores.asked)
	}
}
