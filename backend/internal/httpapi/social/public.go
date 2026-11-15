// Package social 是 social 模組的 Connect handler：前台的 hawkr.social.v1alpha1。這裡只做欄位的轉換與顯示資料的組裝
// （店家卡片來自 seller、買家名稱與頭像來自 buyer、店家封鎖來自 seller 的批次查詢），規則在 internal/modules/social。
package social

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1/socialv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
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

// presenter 是 Public 與 App 共用的組裝：呼叫者的身分，與貼文、留言、討論區項目轉成回應所需的顯示資料（店家卡片、買家名稱、封鎖旗標）。
type presenter struct {
	buyers    Buyers
	stores    Stores
	images    rpc.Images
	webOrigin rpc.WebOrigin
	// liked 查買家讚過哪些貼文，只在呼叫者有買家身分時呼叫；店主路徑（App）的呼叫者沒有買家身分，不設。
	liked func(ctx context.Context, buyerID int64, postIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

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
func NewPublic(reads Reads, views Views, buyers Buyers, stores Stores, images rpc.Images, webOrigin rpc.WebOrigin, now func() time.Time) (*Public, error) {
	if webOrigin.IsZero() {
		return nil, fmt.Errorf("httpapi/social：貼文的單篇網址需要 WebOrigin")
	}
	if now == nil {
		now = time.Now
	}
	return &Public{presenter: &presenter{buyers: buyers, stores: stores, images: images, webOrigin: webOrigin, liked: reads.PostsLikedBy}, reads: reads, views: views, now: now}, nil
}

// caller 是請求的呼叫者此刻的兩個身分；訪客（沒有登入、停權帳號與無效 token 都以訪客執行）與管理員是零值。
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
		scopeTag, _ = social.NormalizeHashtag(msg.GetHashtag())
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
	entries, err := h.toForumEntries(ctx, c, storeID, page.Entries)
	if err != nil {
		return nil, err
	}
	out := &pb.ListForumEntriesResponse{Entries: entries}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, commentCursor{CreatedAt: page.Next.CreatedAt, ID: page.Next.ID})
	}
	return connect.NewResponse(out), nil
}

func (h *presenter) toForumEntries(ctx context.Context, c social.Caller, storeID int64, entries []social.ForumEntry) ([]*pb.ForumEntry, error) {
	if len(entries) == 0 {
		return []*pb.ForumEntry{}, nil
	}
	actors := make([]social.Actor, 0, len(entries))
	for _, e := range entries {
		if e.State == social.CommentVisible {
			actors = append(actors, e.Actor)
		}
	}
	shared, err := h.loadShared(ctx, c, storeID, actors)
	if err != nil {
		return nil, err
	}
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
		item.ViewerCanDelete = item.IsMine || c.OwnsStore(storeID)
		item.ViewerCanReply = shared.replyAllowed
		item.Etag = rpc.Etag(e.Version)
		if item.Author, err = h.toActor(e.Actor, shared.display, shared.stores); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// sharedAuthors 是一頁留言共用的顯示資料：作者的買家名稱與店家卡片、觀看者能不能在這個店的貼文或討論區留言。
type sharedAuthors struct {
	display      map[int64]buyer.Display
	stores       postStores
	replyAllowed bool
}

// loadShared 批次讀一頁可見留言的作者資料：卡片只取店家作者的，觀看者的封鎖旗標另外取（針對留言所在的店與店家作者，一次查詢）。
func (h *presenter) loadShared(ctx context.Context, c social.Caller, contextStore int64, actors []social.Actor) (sharedAuthors, error) {
	if len(actors) == 0 {
		return sharedAuthors{}, nil
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
		return sharedAuthors{}, err
	}
	blocking, err := h.blockingStores(ctx, c, []int64{contextStore})
	if err != nil {
		return sharedAuthors{}, err
	}
	display, err := h.buyerDisplays(ctx, buyerIDs)
	if err != nil {
		return sharedAuthors{}, err
	}
	return sharedAuthors{display: display, stores: postStores{cards: cards, blocking: blocking}, replyAllowed: canComment(c, contextStore, blocking)}, nil
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

// postStores 是一頁貼文共用的店家資料：卡片，以及封鎖了觀看者的店（批次查詢，不逐列查）。
type postStores struct {
	cards    map[int64]seller.StoreCard
	blocking map[int64]bool
	liked    map[uuid.UUID]bool
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

// blockingStores 是 storeIDs 之中封鎖了觀看者的店（一次批次查詢）；訪客沒有封鎖可言，不查。
func (h *presenter) blockingStores(ctx context.Context, c social.Caller, storeIDs []int64) (map[int64]bool, error) {
	ids := uniq(storeIDs)
	if c.BuyerID == 0 || len(ids) == 0 {
		return nil, nil
	}
	return h.stores.StoresBlockingBuyer(ctx, c.BuyerID, ids)
}

// canComment 是觀看者能不能在店 storeID 的貼文留言：買家沒有被這家店封鎖，或這就是觀看者此刻的店。寫入時伺服器仍會檢查。
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
	stores := postStores{cards: cards, blocking: blocking, liked: liked}
	out := make([]*pb.Post, len(posts))
	for i, p := range posts {
		if out[i], err = h.toPost(c, p, stores); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// likedPosts 批次查呼叫者的買家身分讚過 posts 裡的哪幾篇；呼叫者沒有買家身分（訪客、店主）時是空的。
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

func (h *presenter) toPost(c social.Caller, p social.Post, stores postStores) (*pb.Post, error) {
	postURL, err := h.webOrigin.URL("/feed?post=" + url.QueryEscape(p.ID.String()))
	if err != nil {
		return nil, err
	}
	out := &pb.Post{
		Id: p.ID.String(), Body: p.Body, UpdateTime: timestamppb.New(p.UpdatedAt),
		LikeCount: p.Stats.Likes, CommentCount: p.Stats.Comments, ViewCount: p.Stats.Views, ShareCount: p.Stats.Shares,
		Url: postURL, ViewerCanComment: canComment(c, p.StoreID, stores.blocking), ViewerLiked: stores.liked[p.ID],
	}
	if p.PublishedAt != nil {
		out.PublishTime = timestamppb.New(*p.PublishedAt)
	}
	if p.ImageKey != "" {
		if err := rpc.ObjectKey(p.ImageKey); err != nil {
			return nil, err
		}
		out.Image = h.images.Image(p.ImageKey)
	}
	if p.ProductID != nil {
		id := rpc.FormatID(*p.ProductID)
		out.ProductId = &id
	}
	for _, t := range p.Hashtags {
		out.Hashtags = append(out.Hashtags, &pb.Hashtag{Name: t.Name, Display: t.Display})
	}
	if card, ok := stores.cards[p.StoreID]; ok {
		if out.Store, err = httpseller.ToStoreCard(h.images, card); err != nil {
			return nil, err
		}
	}
	return out, nil
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
	shared, err := h.loadShared(ctx, c, postStore, actors)
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
		item.ViewerCanReply = c.OwnsStore(postStore)
		item.Etag = rpc.Etag(m.Version)
		if item.Author, err = h.toActor(m.Actor, shared.display, shared.stores); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *presenter) toActor(a social.Actor, display map[int64]buyer.Display, stores postStores) (*pb.Actor, error) {
	out := &pb.Actor{Kind: rpc.EnumToProto[pb.ActorKind](a.Kind), Id: rpc.FormatID(a.ID)}
	var avatar *string
	switch a.Kind {
	case social.ActorBuyer:
		if d, ok := display[a.ID]; ok && !d.Deleted {
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
