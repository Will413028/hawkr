// Package social 是 social 模組的 Connect handler：前台的 hawkr.social.v1alpha1。這裡只做欄位的轉換與顯示資料的組裝
// （店家卡片來自 seller、買家名稱與頭像來自 buyer、店家封鎖來自 seller 的批次查詢），規則在 internal/modules/social。
package social

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/hashtag"
)

// DefaultTrending 是 SuggestTrendingPosts 沒有指定 max_results 時回傳的篇數。
const DefaultTrending = 10

// Reads 是貼文與留言的讀取來源（*social.Module）。
type Reads interface {
	PublicPost(ctx context.Context, c social.Caller, id uuid.UUID) (social.Post, error)
	ListFeed(ctx context.Context, c social.Caller, after *social.PostCursor, size int32) (social.PostPage, error)
	ListStorePosts(ctx context.Context, c social.Caller, storeID int64, after *social.PostCursor, size int32) (social.PostPage, error)
	ListComments(ctx context.Context, c social.Caller, postID uuid.UUID, after *social.CommentCursor, size int32) (social.CommentPage, error)
	ListReplies(ctx context.Context, c social.Caller, postID, parentID uuid.UUID, after *social.CommentCursor, size int32) (social.CommentPage, error)
	TopPosts(ctx context.Context, c social.Caller, n int32) ([]social.Post, error)
	PostsLikedBy(ctx context.Context, buyerID int64, postIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	ListHashtagPosts(ctx context.Context, c social.Caller, tag string, after *social.PostCursor, size int32) (social.PostPage, error)
	SuggestHashtags(ctx context.Context, q string, contains bool, limit int32) ([]social.HashtagCount, error)
	TrendingHashtags(ctx context.Context, from, to time.Time, limit int32) ([]social.HashtagCount, error)
	ListForumEntries(ctx context.Context, c social.Caller, storeID int64, after *social.CommentCursor, size int32) (social.ForumPage, error)
	ListForumReplies(ctx context.Context, c social.Caller, storeID int64, parentID uuid.UUID, after *social.CommentCursor, size int32) (social.ForumPage, error)
	GetForumThread(ctx context.Context, c social.Caller, storeID int64, threadID uuid.UUID, size int32) (social.ForumThreadView, error)
}

// Buyers 是呼叫者的買家身分與留言作者顯示資料的來源（*buyer.Module）。
type Buyers interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
	BatchDisplay(ctx context.Context, ids []int64) ([]buyer.Display, error)
}

// Stores 是呼叫者此刻的店、店家卡片與店家封鎖的來源（*seller.Module）。
type Stores interface {
	CurrentStore(ctx context.Context, accountID int64) (seller.Store, error)
	StoreCards(ctx context.Context, storeIDs []int64) ([]seller.StoreCard, error)
	StoresBlockingBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]bool, error)
}

// ProductInfo 是貼文帶的商品的顯示資料（catalog 的卡片）。
type ProductInfo struct {
	ID       uuid.UUID
	StoreID  int64
	Name     string
	ImageKey string
	MinPrice int64
	MaxPrice int64
	// Available 是現在可以購買（上架且有庫存）。
	Available bool
}

// Products 以一個批次取得商品顯示資料；includeInactive 為假時只回上架中的，
// 缺少的 ID（已刪除或被排除）不在回傳裡。
type Products interface {
	Cards(ctx context.Context, ids []uuid.UUID, includeInactive bool) (map[uuid.UUID]ProductInfo, error)
}

type presenter struct {
	buyers         Buyers
	stores         Stores
	images         rpc.Images
	webOrigin      rpc.WebOrigin
	products       Products
	ownerView      bool
	liked          func(ctx context.Context, buyerID int64, postIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	log            *slog.Logger
	productTimeout time.Duration
}

const productBudget = 500 * time.Millisecond

// Views 是瀏覽打點的寫入端（*social.Module）。
type Views interface {
	RecordView(ctx context.Context, c social.Caller, postID uuid.UUID, v social.Viewer) (bool, error)
}

// Public 實作 SocialPublicService。
type Public struct {
	*presenter
	reads Reads
	views Views
	now   func() time.Time
}

var _ socialv1alpha1connect.SocialPublicServiceHandler = (*Public)(nil)

// NewPublic 建立前台的 handler；webOrigin 組貼文的單篇網址，不可是零值；now 是熱門標籤預設時間窗的終點，nil 時用 time.Now。
func NewPublic(reads Reads, views Views, buyers Buyers, stores Stores, products Products, images rpc.Images, webOrigin rpc.WebOrigin, now func() time.Time, log *slog.Logger) (*Public, error) {
	if log == nil {
		return nil, fmt.Errorf("httpapi/social：需要 logger")
	}
	if webOrigin.IsZero() {
		return nil, fmt.Errorf("httpapi/social：貼文的單篇網址需要 WebOrigin")
	}
	if now == nil {
		now = time.Now
	}
	return &Public{presenter: &presenter{buyers: buyers, stores: stores, products: products, images: images, webOrigin: webOrigin, log: log, liked: reads.PostsLikedBy}, reads: reads, views: views, now: now}, nil
}

func (h *presenter) caller(ctx context.Context) (social.Caller, error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAccount {
		return social.Caller{}, nil
	}
	buyerID, err := h.buyers.CurrentBuyerID(ctx, p.ID)
	if err != nil {
		return social.Caller{}, rpc.BuyerInvariant(p.ID, err)
	}
	c := social.Caller{BuyerID: buyerID}
	store, err := h.stores.CurrentStore(ctx, p.ID)
	switch {
	case apperr.KindOf(err) == apperr.KindNotFound:
	case err != nil:
		return social.Caller{}, err
	default:
		c.Store = &store
	}
	return c, nil
}

func (h *Public) GetPost(ctx context.Context, req *connect.Request[pb.GetPostRequest]) (*connect.Response[pb.GetPostResponse], error) {
	id, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	c, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.reads.PublicPost(ctx, c, id)
	if err != nil {
		return nil, err
	}
	posts, err := h.toPosts(ctx, c, []social.Post{p})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetPostResponse{Post: posts[0]}), nil
}

// RecordPostView 只計觀看者讀得到的貼文，而且店主看自己的貼文不計（由模組依貼文的店家排除）；管理員代行的讀取不記錄（回成功、不寫入）；讀不到與沒有計入都回成功，不洩漏貼文是否存在。
// 停權的帳號與無效的憑證在入口已經是訪客，這裡自然計為匿名瀏覽。每個來源 IP 的額度在入口（httpapi 的 tierPostView）。
func (h *Public) RecordPostView(ctx context.Context, req *connect.Request[pb.RecordPostViewRequest]) (*connect.Response[pb.RecordPostViewResponse], error) {
	id, err := rpc.ParseUUID("post_id", req.Msg.GetPostId())
	if err != nil {
		return nil, err
	}
	c, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	done := connect.NewResponse(&pb.RecordPostViewResponse{})
	v := social.Viewer{IP: rpc.ClientIPFrom(ctx), UserAgent: req.Header().Get("User-Agent")}
	if p, ok := rpc.PrincipalFrom(ctx); ok {
		if p.Kind != identity.PrincipalAccount {
			return done, nil
		}
		v.AccountID = p.ID
	}
	if _, err := h.views.RecordView(ctx, c, id, v); err != nil {
		return nil, err
	}
	return done, nil
}

func (h *Public) ListPosts(ctx context.Context, req *connect.Request[pb.ListPostsRequest]) (*connect.Response[pb.ListPostsResponse], error) {
	msg := req.Msg
	var storeID int64
	scopeStore := ""
	if msg.StoreId != nil {
		var err error
		if storeID, err = rpc.ParseID("store_id", msg.GetStoreId()); err != nil {
			return nil, err
		}
		scopeStore = rpc.FormatID(storeID)
	}
	if msg.Hashtag != nil && msg.StoreId != nil {
		return nil, rpc.RequiredField("hashtag", "hashtag 與 store_id 不能同時設")
	}
	scopeTag := ""
	if msg.Hashtag != nil {
		scopeTag, _, _ = hashtag.Normalize(msg.GetHashtag())
	}
	scope := rpc.FilterScope(socialv1alpha1connect.SocialPublicServiceListPostsProcedure, scopeStore, scopeTag)
	after, err := rpc.ValidCursor[postCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	c, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	var cur *social.PostCursor
	if after != nil {
		cur = &social.PostCursor{PublishedAt: after.PublishedAt, ID: after.ID}
	}
	var page social.PostPage
	switch {
	case msg.Hashtag != nil:
		page, err = h.reads.ListHashtagPosts(ctx, c, msg.GetHashtag(), cur, msg.GetPageSize())
	case msg.StoreId != nil:
		page, err = h.reads.ListStorePosts(ctx, c, storeID, cur, msg.GetPageSize())
	default:
		page, err = h.reads.ListFeed(ctx, c, cur, msg.GetPageSize())
	}
	if err != nil {
		return nil, err
	}
	posts, err := h.toPosts(ctx, c, page.Posts)
	if err != nil {
		return nil, err
	}
	out := &pb.ListPostsResponse{Posts: posts}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, postCursor{PublishedAt: page.Next.PublishedAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

func (h *Public) ListComments(ctx context.Context, req *connect.Request[pb.ListCommentsRequest]) (*connect.Response[pb.ListCommentsResponse], error) {
	msg := req.Msg
	postID, err := rpc.ParseUUID("post_id", msg.GetPostId())
	if err != nil {
		return nil, err
	}
	var parentID uuid.UUID
	scopeParent := ""
	if msg.ParentId != nil {
		if parentID, err = rpc.ParseUUID("parent_id", msg.GetParentId()); err != nil {
			return nil, err
		}
		scopeParent = parentID.String()
	}
	scope := rpc.FilterScope(socialv1alpha1connect.SocialPublicServiceListCommentsProcedure, postID.String(), scopeParent)
	after, err := rpc.ValidCursor[commentCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	c, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	var cur *social.CommentCursor
	if after != nil {
		cur = &social.CommentCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	var page social.CommentPage
	if msg.ParentId != nil {
		page, err = h.reads.ListReplies(ctx, c, postID, parentID, cur, msg.GetPageSize())
	} else {
		page, err = h.reads.ListComments(ctx, c, postID, cur, msg.GetPageSize())
	}
	if err != nil {
		return nil, err
	}
	comments, err := h.toComments(ctx, c, page.StoreID, page.Comments)
	if err != nil {
		return nil, err
	}
	out := &pb.ListCommentsResponse{Comments: comments}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, commentCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

func (h *Public) SuggestTrendingPosts(ctx context.Context, req *connect.Request[pb.SuggestTrendingPostsRequest]) (*connect.Response[pb.SuggestTrendingPostsResponse], error) {
	n := req.Msg.GetMaxResults()
	switch {
	case n == 0:
		n = DefaultTrending
	case n < 0:
		return nil, rpc.RequiredField("max_results", "max_results 不可以是負數")
	case n > social.MaxTopPosts:
		n = social.MaxTopPosts
	}
	c, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	top, err := h.reads.TopPosts(ctx, c, n)
	if err != nil {
		return nil, err
	}
	posts, err := h.toPosts(ctx, c, top)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SuggestTrendingPostsResponse{Posts: posts}), nil
}

func (h *Public) SuggestTags(ctx context.Context, req *connect.Request[pb.SuggestTagsRequest]) (*connect.Response[pb.SuggestTagsResponse], error) {
	n := req.Msg.GetMaxResults()
	if n < 0 {
		return nil, rpc.RequiredField("max_results", "max_results 不可以是負數")
	}
	tags, err := h.reads.SuggestHashtags(ctx, req.Msg.GetQuery(), req.Msg.GetContains(), n)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SuggestTagsResponse{Tags: toTagCounts(tags)}), nil
}

func (h *Public) SuggestTrendingTags(ctx context.Context, req *connect.Request[pb.SuggestTrendingTagsRequest]) (*connect.Response[pb.SuggestTrendingTagsResponse], error) {
	msg := req.Msg
	if msg.GetMaxResults() < 0 {
		return nil, rpc.RequiredField("max_results", "max_results 不可以是負數")
	}
	to := h.now().UTC()
	if msg.EndTime != nil {
		to = msg.GetEndTime().AsTime()
	}
	from := to.Add(-social.DefaultTrendingWindow)
	if msg.StartTime != nil {
		from = msg.GetStartTime().AsTime()
	}
	tags, err := h.reads.TrendingHashtags(ctx, from, to, msg.GetMaxResults())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SuggestTrendingTagsResponse{Tags: toTagCounts(tags)}), nil
}

func toTagCounts(tags []social.HashtagCount) []*pb.TagCount {
	out := make([]*pb.TagCount, len(tags))
	for i, t := range tags {
		out[i] = &pb.TagCount{Name: t.Name, PostCount: t.Posts}
	}
	return out
}

func (h *Public) ListForumEntries(ctx context.Context, req *connect.Request[pb.ListForumEntriesRequest]) (*connect.Response[pb.ListForumEntriesResponse], error) {
	msg := req.Msg
	storeID, err := rpc.ParseID("store_id", msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	var parentID uuid.UUID
	scopeParent := ""
	if msg.ParentId != nil {
		if parentID, err = rpc.ParseUUID("parent_id", msg.GetParentId()); err != nil {
			return nil, err
		}
		scopeParent = parentID.String()
	}
	scope := rpc.FilterScope(socialv1alpha1connect.SocialPublicServiceListForumEntriesProcedure, rpc.FormatID(storeID), scopeParent)
	after, err := rpc.ValidCursor[commentCursor](scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	c, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	var cur *social.CommentCursor
	if after != nil {
		cur = &social.CommentCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	var page social.ForumPage
	if msg.ParentId != nil {
		page, err = h.reads.ListForumReplies(ctx, c, storeID, parentID, cur, msg.GetPageSize())
	} else {
		page, err = h.reads.ListForumEntries(ctx, c, storeID, cur, msg.GetPageSize())
	}
	if err != nil {
		return nil, err
	}
	viewer, actor, err := h.forumViewerState(ctx, c, storeID, page.Entries, page.Readable)
	if err != nil {
		return nil, err
	}
	entries, err := h.toForumEntries(c, viewer, page.Entries)
	if err != nil {
		return nil, err
	}
	out := &pb.ListForumEntriesResponse{Entries: entries, ViewerActor: actor, ViewerCanComment: viewer.canComment}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, commentCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

func (h *Public) GetForumThread(ctx context.Context, req *connect.Request[pb.GetForumThreadRequest]) (*connect.Response[pb.GetForumThreadResponse], error) {
	msg := req.Msg
	storeID, err := rpc.ParseID("store_id", msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	threadID, err := rpc.ParseUUID("thread_id", msg.GetThreadId())
	if err != nil {
		return nil, err
	}
	c, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	view, err := h.reads.GetForumThread(ctx, c, storeID, threadID, msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	all := append([]social.ForumEntry{view.Top}, view.Replies.Entries...)
	viewer, actor, err := h.forumViewerState(ctx, c, storeID, all, true)
	if err != nil {
		return nil, err
	}
	items, err := h.toForumEntries(c, viewer, all)
	if err != nil {
		return nil, err
	}
	out := &pb.GetForumThreadResponse{Entry: items[0], Replies: items[1:], ViewerActor: actor, ViewerCanComment: viewer.canComment}
	if view.Replies.Next != nil {
		scope := rpc.FilterScope(socialv1alpha1connect.SocialPublicServiceListForumEntriesProcedure, rpc.FormatID(storeID), threadID.String())
		out.NextPageToken = rpc.TokenFor(scope, commentCursor{CreatedAt: view.Replies.Next.CreatedAt, ID: view.Replies.Next.ID})
	}
	return connect.NewResponse(out), nil
}

type forumViewer struct {
	storeID    int64
	canComment bool
	stores     postStores
}

func (h *presenter) forumPageState(ctx context.Context, c social.Caller, storeID int64, entries []social.ForumEntry, readable bool) (forumViewer, error) {
	return h.loadForumState(ctx, c, storeID, entries, readable, false)
}

func (h *presenter) forumViewerState(ctx context.Context, c social.Caller, storeID int64, entries []social.ForumEntry, readable bool) (forumViewer, *pb.Actor, error) {
	viewer, err := h.loadForumState(ctx, c, storeID, entries, readable, true)
	if err != nil {
		return forumViewer{}, nil, err
	}
	actor, err := h.viewerActor(c, storeID, viewer.stores)
	if err != nil {
		return forumViewer{}, nil, err
	}
	return viewer, actor, nil
}

func (h *presenter) loadForumState(ctx context.Context, c social.Caller, storeID int64, entries []social.ForumEntry, readable, withViewer bool) (forumViewer, error) {
	var buyerIDs, storeIDs []int64
	for _, e := range entries {
		if e.State != social.CommentVisible {
			continue
		}
		switch e.Actor.Kind {
		case social.ActorBuyer:
			buyerIDs = append(buyerIDs, e.Actor.ID)
		case social.ActorStore:
			storeIDs = append(storeIDs, e.Actor.ID)
		}
	}
	owns := c.OwnsStore(storeID)
	if withViewer {
		switch {
		case owns:
			storeIDs = append(storeIDs, storeID)
		case c.BuyerID != 0:
			buyerIDs = append(buyerIDs, c.BuyerID)
		}
	}
	var stores postStores
	var err error
	if stores.cards, err = h.storeCards(ctx, storeIDs); err != nil {
		return forumViewer{}, err
	}
	if stores.buyerDisplay, err = h.buyerDisplays(ctx, buyerIDs); err != nil {
		return forumViewer{}, err
	}
	can := readable && (owns || c.BuyerID != 0)
	return forumViewer{storeID: storeID, canComment: can, stores: stores}, nil
}

func (h *presenter) toForumEntries(c social.Caller, viewer forumViewer, entries []social.ForumEntry) ([]*pb.ForumEntry, error) {
	if len(entries) == 0 {
		return []*pb.ForumEntry{}, nil
	}
	var err error
	out := make([]*pb.ForumEntry, len(entries))
	for i, e := range entries {
		item := &pb.ForumEntry{
			Id: e.ID.String(), StoreId: rpc.FormatID(e.StoreID), State: rpc.EnumToProto[pb.CommentState](e.State),
			CreateTime: timestamppb.New(e.CreatedAt), ReplyCount: e.ReplyCount,
		}
		out[i] = item
		if e.State != social.CommentVisible {
			continue
		}
		if e.ParentID != nil {
			s := e.ParentID.String()
			item.ParentId = &s
		}
		if e.ReplyToID != nil {
			s := e.ReplyToID.String()
			item.ReplyToId = &s
		}
		item.UpdateTime = timestamppb.New(e.UpdatedAt)
		item.Body = e.Body
		item.IsMine = c.IsAuthor(e.Actor)
		item.ViewerCanEdit = item.IsMine
		item.ViewerCanDelete = item.IsMine || c.OwnsStore(viewer.storeID)
		item.ViewerCanReply = viewer.canComment
		item.Etag = rpc.Etag(e.Version)
		if item.Author, err = h.toActor(e.Actor, viewer.stores); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *presenter) loadShared(ctx context.Context, actors []social.Actor) (postStores, error) {
	if len(actors) == 0 {
		return postStores{}, nil
	}
	var buyerIDs, storeIDs []int64
	for _, a := range actors {
		switch a.Kind {
		case social.ActorBuyer:
			buyerIDs = append(buyerIDs, a.ID)
		case social.ActorStore:
			storeIDs = append(storeIDs, a.ID)
		}
	}
	cards, err := h.storeCards(ctx, storeIDs)
	if err != nil {
		return postStores{}, err
	}
	display, err := h.buyerDisplays(ctx, buyerIDs)
	if err != nil {
		return postStores{}, err
	}
	return postStores{cards: cards, buyerDisplay: display}, nil
}

func (h *presenter) buyerDisplays(ctx context.Context, buyerIDs []int64) (map[int64]buyer.Display, error) {
	display := map[int64]buyer.Display{}
	if buyerIDs = uniq(buyerIDs); len(buyerIDs) == 0 {
		return display, nil
	}
	ds, err := h.buyers.BatchDisplay(ctx, buyerIDs)
	if err != nil {
		return nil, err
	}
	for _, d := range ds {
		display[d.BuyerID] = d
	}
	return display, nil
}

type postStores struct {
	cards        map[int64]seller.StoreCard
	blocking     map[int64]bool
	liked        map[uuid.UUID]bool
	products     map[uuid.UUID]ProductInfo
	buyerDisplay map[int64]buyer.Display
}

func (h *presenter) storeCards(ctx context.Context, storeIDs []int64) (map[int64]seller.StoreCard, error) {
	out := map[int64]seller.StoreCard{}
	ids := uniq(storeIDs)
	if len(ids) == 0 {
		return out, nil
	}
	cards, err := h.stores.StoreCards(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, card := range cards {
		out[card.StoreID] = card
	}
	return out, nil
}

func (h *presenter) blockingStores(ctx context.Context, c social.Caller, storeIDs []int64) (map[int64]bool, error) {
	ids := uniq(storeIDs)
	if c.BuyerID == 0 || len(ids) == 0 {
		return nil, nil
	}
	return h.stores.StoresBlockingBuyer(ctx, c.BuyerID, ids)
}

func canComment(c social.Caller, storeID int64, blocking map[int64]bool) bool {
	if c.OwnsStore(storeID) {
		return true
	}
	return c.BuyerID != 0 && !blocking[storeID]
}

func (h *presenter) toPosts(ctx context.Context, c social.Caller, posts []social.Post) ([]*pb.Post, error) {
	storeIDs := make([]int64, len(posts))
	for i, p := range posts {
		storeIDs[i] = p.StoreID
	}
	if len(posts) == 0 {
		return []*pb.Post{}, nil
	}
	productsCh := make(chan map[uuid.UUID]ProductInfo, 1)
	go func() { productsCh <- h.postProducts(ctx, posts) }()
	cards, err := h.storeCards(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	blocking, err := h.blockingStores(ctx, c, storeIDs)
	if err != nil {
		return nil, err
	}
	liked, err := h.likedPosts(ctx, c, posts)
	if err != nil {
		return nil, err
	}
	products := <-productsCh
	stores := postStores{cards: cards, blocking: blocking, liked: liked, products: products}
	if c.BuyerID != 0 && slices.ContainsFunc(posts, func(p social.Post) bool { return !c.OwnsStore(p.StoreID) }) {
		if stores.buyerDisplay, err = h.buyerDisplays(ctx, []int64{c.BuyerID}); err != nil {
			return nil, err
		}
	}
	out := make([]*pb.Post, len(posts))
	for i, p := range posts {
		if out[i], err = h.toPost(c, p, stores); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func postIsCommentable(c social.Caller, p social.Post, stores postStores) bool {
	if !canComment(c, p.StoreID, stores.blocking) {
		return false
	}
	return !c.OwnsStore(p.StoreID) || p.Readable
}

func (h *presenter) postProducts(ctx context.Context, posts []social.Post) map[uuid.UUID]ProductInfo {
	var ids []uuid.UUID
	for _, p := range posts {
		if p.ProductID != nil {
			ids = append(ids, *p.ProductID)
		}
	}
	if ids = uniqUUIDs(ids); len(ids) == 0 {
		return nil
	}
	budget := h.productTimeout
	if budget <= 0 {
		budget = productBudget
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	cards, err := h.products.Cards(ctx, ids, h.ownerView)
	if err != nil {
		h.log.WarnContext(ctx, "httpapi/social：商品顯示資料查詢失敗，貼文的 product 不設", "product_count", len(ids), "error", err)
		return nil
	}
	return cards
}

func (h *presenter) toPostProduct(info ProductInfo) *pb.PostProduct {
	out := &pb.PostProduct{Id: info.ID.String(), Name: info.Name, MinPrice: info.MinPrice, MaxPrice: info.MaxPrice, Available: info.Available}
	if info.ImageKey != "" {
		if err := rpc.ObjectKey(info.ImageKey); err != nil {
			h.log.Warn("httpapi/social：商品的圖片 key 不合法，product.image 不設", "product_id", info.ID, "error", err)
			return out
		}
		out.Image = h.images.Image(info.ImageKey)
	}
	return out
}

func (h *presenter) likedPosts(ctx context.Context, c social.Caller, posts []social.Post) (map[uuid.UUID]bool, error) {
	if c.BuyerID == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	return h.liked(ctx, c.BuyerID, ids)
}

func imageAndProduct(images rpc.Images, p social.Post) (*commonpb.Image, *string, error) {
	var img *commonpb.Image
	if p.ImageKey != "" {
		if err := rpc.ObjectKey(p.ImageKey); err != nil {
			return nil, nil, err
		}
		img = images.Image(p.ImageKey)
	}
	var product *string
	if p.ProductID != nil {
		id := p.ProductID.String()
		product = &id
	}
	return img, product, nil
}

func (h *presenter) toPost(c social.Caller, p social.Post, stores postStores) (*pb.Post, error) {
	postURL, err := h.webOrigin.URL("/feed?post=" + url.QueryEscape(p.ID.String()))
	if err != nil {
		return nil, err
	}
	out := &pb.Post{
		Id: p.ID.String(), Body: p.Body, UpdateTime: timestamppb.New(p.UpdatedAt),
		LikeCount: p.Stats.Likes, CommentCount: p.Stats.Comments, ViewCount: p.Stats.Views, ShareCount: p.Stats.Shares,
		Url: postURL, ViewerCanComment: postIsCommentable(c, p, stores), ViewerLiked: stores.liked[p.ID],
	}
	if p.PublishedAt != nil {
		out.PublishTime = timestamppb.New(*p.PublishedAt)
	}
	if out.Image, out.ProductId, err = imageAndProduct(h.images, p); err != nil {
		return nil, err
	}
	for _, t := range p.Hashtags {
		out.Hashtags = append(out.Hashtags, &pb.Hashtag{Name: t.Name, Display: t.Display})
	}
	if out.ViewerActor, err = h.viewerActor(c, p.StoreID, stores); err != nil {
		return nil, err
	}
	if card, ok := stores.cards[p.StoreID]; ok {
		if out.Store, err = httpseller.ToStoreCard(h.images, card); err != nil {
			return nil, err
		}
	}
	if p.ProductID != nil {
		if info, ok := stores.products[*p.ProductID]; ok {
			out.Product = h.toPostProduct(info)
		}
	}
	return out, nil
}

func (h *presenter) viewerActor(c social.Caller, storeID int64, stores postStores) (*pb.Actor, error) {
	switch {
	case c.OwnsStore(storeID):
		return h.toActor(social.Actor{Kind: social.ActorStore, ID: storeID}, stores)
	case c.BuyerID != 0:
		return h.toActor(social.Actor{Kind: social.ActorBuyer, ID: c.BuyerID}, stores)
	}
	return nil, nil
}

func (h *presenter) toComments(ctx context.Context, c social.Caller, postStore int64, comments []social.Comment) ([]*pb.Comment, error) {
	if len(comments) == 0 {
		return []*pb.Comment{}, nil
	}
	actors := make([]social.Actor, 0, len(comments))
	for _, m := range comments {
		if m.State == social.CommentVisible {
			actors = append(actors, m.Actor)
		}
	}
	shared, err := h.loadShared(ctx, actors)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Comment, len(comments))
	for i, m := range comments {
		item := &pb.Comment{
			Id: m.ID.String(), PostId: m.PostID.String(), State: rpc.EnumToProto[pb.CommentState](m.State),
			CreateTime: timestamppb.New(m.CreatedAt), ReplyCount: m.ReplyCount,
		}
		out[i] = item
		if m.State != social.CommentVisible {
			continue
		}
		if m.ParentID != nil {
			s := m.ParentID.String()
			item.ParentId = &s
		}
		if m.ReplyToID != nil {
			s := m.ReplyToID.String()
			item.ReplyToId = &s
		}
		item.UpdateTime = timestamppb.New(m.UpdatedAt)
		item.LikeCount = m.Likes
		item.Body = m.Body
		item.ViewerLiked = m.LikedByMe
		item.IsMine = c.IsAuthor(m.Actor)
		item.ViewerCanEdit = item.IsMine
		item.ViewerCanDelete = item.IsMine || c.OwnsStore(postStore)
		item.ViewerCanReply = c.OwnsStore(postStore) && m.PostReadable
		item.Etag = rpc.Etag(m.Version)
		if item.Author, err = h.toActor(m.Actor, shared); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *presenter) toActor(a social.Actor, stores postStores) (*pb.Actor, error) {
	out := &pb.Actor{Kind: rpc.EnumToProto[pb.ActorKind](a.Kind), Id: rpc.FormatID(a.ID)}
	var avatar *string
	switch a.Kind {
	case social.ActorBuyer:
		if d, ok := stores.buyerDisplay[a.ID]; ok && !d.Deleted {
			out.Name, avatar = d.DisplayName, d.Avatar
		}
	case social.ActorStore:
		if card, ok := stores.cards[a.ID]; ok {
			out.Name, avatar = card.Name, card.LogoKey
		}
	}
	if avatar != nil {
		if err := rpc.ObjectKey(*avatar); err != nil {
			return nil, err
		}
		out.Avatar = h.images.Image(*avatar)
	}
	return out, nil
}

func uniq(ids []int64) []int64 {
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

func uniqUUIDs(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, uuid.UUID.Compare)
	return slices.Compact(out)
}
