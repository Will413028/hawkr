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
	ListReplies(ctx context.Context, c social.Caller, parentID uuid.UUID, after *social.CommentCursor, size int32) (social.CommentPage, error)
	TopPosts(ctx context.Context, c social.Caller, n int32) ([]social.Post, error)
	ListHashtagPosts(ctx context.Context, c social.Caller, tag string, after *social.PostCursor, size int32) (social.PostPage, error)
	SuggestHashtags(ctx context.Context, q string, contains bool, limit int32) ([]social.HashtagCount, error)
	TrendingHashtags(ctx context.Context, from, to time.Time, limit int32) ([]social.HashtagCount, error)
	ListForumEntries(ctx context.Context, c social.Caller, storeID int64, after *social.CommentCursor, size int32) (social.ForumPage, error)
	ListForumReplies(ctx context.Context, c social.Caller, parentID uuid.UUID, after *social.CommentCursor, size int32) (social.ForumPage, error)
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

// Public 實作 SocialPublicService。
type Public struct {
	reads     Reads
	buyers    Buyers
	stores    Stores
	images    rpc.Images
	webOrigin rpc.WebOrigin
	now       func() time.Time
}

var _ socialv1alpha1connect.SocialPublicServiceHandler = (*Public)(nil)

// NewPublic 建立前台的 handler；webOrigin 組貼文的單篇網址，不可是零值；now 是熱門標籤預設時間窗的終點，nil 時用 time.Now。
func NewPublic(reads Reads, buyers Buyers, stores Stores, images rpc.Images, webOrigin rpc.WebOrigin, now func() time.Time) (*Public, error) {
	if webOrigin.IsZero() {
		return nil, fmt.Errorf("httpapi/social：貼文的單篇網址需要 WebOrigin")
	}
	if now == nil {
		now = time.Now
	}
	return &Public{reads: reads, buyers: buyers, stores: stores, images: images, webOrigin: webOrigin, now: now}, nil
}

// caller 是請求的呼叫者此刻的兩個身分；訪客（沒有登入、停權帳號與無效 token 都以訪客執行）與管理員是零值。
func (h *Public) caller(ctx context.Context) (social.Caller, error) {
	p, ok := rpc.PrincipalFrom(ctx)
	if !ok || p.Kind != identity.PrincipalAccount {
		return social.Caller{}, nil
	}
	buyerID, err := h.buyers.CurrentBuyerID(ctx, p.ID)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return social.Caller{}, fmt.Errorf("httpapi/social：存活的帳號 %d 沒有現任買家（不變量被破壞）", p.ID)
	}
	if err != nil {
		return social.Caller{}, err
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
		scopeTag = msg.GetHashtag()
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
	post, err := h.reads.PublicPost(ctx, c, postID)
	if err != nil {
		return nil, err
	}
	var cur *social.CommentCursor
	if after != nil {
		cur = &social.CommentCursor{CreatedAt: after.CreatedAt, ID: after.ID}
	}
	var page social.CommentPage
	if msg.ParentId != nil {
		page, err = h.reads.ListReplies(ctx, c, parentID, cur, msg.GetPageSize())
	} else {
		page, err = h.reads.ListComments(ctx, c, postID, cur, msg.GetPageSize())
	}
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(page.Comments, func(m social.Comment) bool { return m.PostID != postID }) {
		return nil, apperr.NotFound("留言不存在")
	}
	comments, err := h.toComments(ctx, c, post, page.Comments)
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

func (h *Public) ListTrendingTags(ctx context.Context, req *connect.Request[pb.ListTrendingTagsRequest]) (*connect.Response[pb.ListTrendingTagsResponse], error) {
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
	return connect.NewResponse(&pb.ListTrendingTagsResponse{Tags: toTagCounts(tags)}), nil
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
		page, err = h.reads.ListForumReplies(ctx, c, parentID, cur, msg.GetPageSize())
	} else {
		page, err = h.reads.ListForumEntries(ctx, c, storeID, cur, msg.GetPageSize())
	}
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(page.Entries, func(e social.ForumEntry) bool { return e.StoreID != storeID }) {
		return nil, apperr.NotFound("留言不存在")
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

func (h *Public) toForumEntries(ctx context.Context, c social.Caller, storeID int64, entries []social.ForumEntry) ([]*pb.ForumEntry, error) {
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
	stores, err := h.loadStores(ctx, c, append(storeIDs, storeID), true)
	if err != nil {
		return nil, err
	}
	display, err := h.buyerDisplays(ctx, buyerIDs)
	if err != nil {
		return nil, err
	}
	replyAllowed := canComment(c, storeID, stores.blocking)
	out := make([]*pb.ForumEntry, len(entries))
	for i, e := range entries {
		item := &pb.ForumEntry{
			Id: e.ID.String(), StoreId: rpc.FormatID(e.StoreID), State: rpc.EnumToProto[pb.CommentState](e.State),
			CreateTime: timestamppb.New(e.CreatedAt), UpdateTime: timestamppb.New(e.UpdatedAt), ReplyCount: e.ReplyCount,
		}
		if e.ParentID != nil {
			s := e.ParentID.String()
			item.ParentId = &s
		}
		if e.ReplyToID != nil {
			s := e.ReplyToID.String()
			item.ReplyToId = &s
		}
		out[i] = item
		if e.State != social.CommentVisible {
			continue
		}
		item.Body = e.Body
		item.IsMine = c.IsAuthor(e.Actor)
		item.ViewerCanEdit = item.IsMine
		item.ViewerCanDelete = item.IsMine || c.OwnsStore(storeID)
		item.ViewerCanReply = replyAllowed
		if item.Author, err = h.toActor(e.Actor, display, stores); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *Public) buyerDisplays(ctx context.Context, buyerIDs []int64) (map[int64]buyer.Display, error) {
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
}

func (h *Public) loadStores(ctx context.Context, c social.Caller, storeIDs []int64, wantBlocks bool) (postStores, error) {
	ids := uniq(storeIDs)
	out := postStores{cards: map[int64]seller.StoreCard{}}
	if len(ids) == 0 {
		return out, nil
	}
	cards, err := h.stores.StoreCards(ctx, ids)
	if err != nil {
		return postStores{}, err
	}
	for _, card := range cards {
		out.cards[card.StoreID] = card
	}
	if wantBlocks && c.BuyerID != 0 {
		if out.blocking, err = h.stores.StoresBlockingBuyer(ctx, c.BuyerID, ids); err != nil {
			return postStores{}, err
		}
	}
	return out, nil
}

// canComment 是觀看者能不能在店 storeID 的貼文留言：買家沒有被這家店封鎖，或這就是觀看者此刻的店。寫入時伺服器仍會檢查。
func canComment(c social.Caller, storeID int64, blocking map[int64]bool) bool {
	if c.OwnsStore(storeID) {
		return true
	}
	return c.BuyerID != 0 && !blocking[storeID]
}

func (h *Public) toPosts(ctx context.Context, c social.Caller, posts []social.Post) ([]*pb.Post, error) {
	storeIDs := make([]int64, len(posts))
	for i, p := range posts {
		storeIDs[i] = p.StoreID
	}
	stores, err := h.loadStores(ctx, c, storeIDs, true)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Post, len(posts))
	for i, p := range posts {
		if out[i], err = h.toPost(c, p, stores); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *Public) toPost(c social.Caller, p social.Post, stores postStores) (*pb.Post, error) {
	postURL, err := h.webOrigin.URL("/feed?post=" + url.QueryEscape(p.ID.String()))
	if err != nil {
		return nil, err
	}
	out := &pb.Post{
		Id: p.ID.String(), Body: p.Body, UpdateTime: timestamppb.New(p.UpdatedAt),
		LikeCount: p.Stats.Likes, CommentCount: p.Stats.Comments, ViewCount: p.Stats.Views, ShareCount: p.Stats.Shares,
		Url: postURL, ViewerCanComment: canComment(c, p.StoreID, stores.blocking),
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

func (h *Public) toComments(ctx context.Context, c social.Caller, post social.Post, comments []social.Comment) ([]*pb.Comment, error) {
	var buyerIDs, storeIDs []int64
	for _, m := range comments {
		if m.State != social.CommentVisible {
			continue
		}
		switch m.Actor.Kind {
		case social.ActorBuyer:
			buyerIDs = append(buyerIDs, m.Actor.ID)
		case social.ActorStore:
			storeIDs = append(storeIDs, m.Actor.ID)
		}
	}
	stores, err := h.loadStores(ctx, c, append(storeIDs, post.StoreID), true)
	if err != nil {
		return nil, err
	}
	display, err := h.buyerDisplays(ctx, buyerIDs)
	if err != nil {
		return nil, err
	}
	replyAllowed := canComment(c, post.StoreID, stores.blocking)
	out := make([]*pb.Comment, len(comments))
	for i, m := range comments {
		item := &pb.Comment{
			Id: m.ID.String(), PostId: m.PostID.String(), State: rpc.EnumToProto[pb.CommentState](m.State),
			CreateTime: timestamppb.New(m.CreatedAt), UpdateTime: timestamppb.New(m.UpdatedAt),
			LikeCount: m.Likes, ReplyCount: m.ReplyCount,
		}
		if m.ParentID != nil {
			s := m.ParentID.String()
			item.ParentId = &s
		}
		if m.ReplyToID != nil {
			s := m.ReplyToID.String()
			item.ReplyToId = &s
		}
		out[i] = item
		if m.State != social.CommentVisible {
			continue
		}
		item.Body = m.Body
		item.ViewerLiked = m.LikedByMe
		item.IsMine = c.IsAuthor(m.Actor)
		item.ViewerCanEdit = item.IsMine
		item.ViewerCanDelete = item.IsMine || c.OwnsStore(post.StoreID)
		item.ViewerCanReply = replyAllowed
		if item.Author, err = h.toActor(m.Actor, display, stores); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h *Public) toActor(a social.Actor, display map[int64]buyer.Display, stores postStores) (*pb.Actor, error) {
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
