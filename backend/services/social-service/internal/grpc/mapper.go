package grpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
)

func toForumEntryInfo(e *gen.CmsSellerForum, viewer usecase.Caller) *pb.ForumEntryInfo {
	if e == nil {
		return nil
	}
	return &pb.ForumEntryInfo{
		Id:            strconv.FormatInt(e.ID, 10),
		SellerId:      strconv.FormatInt(e.SellerID, 10),
		UserName:      optionalString(e.UserName),
		UserAvatarKey: optionalString(e.UserAvatar),
		Content:       e.Content,
		ParentId:      strconv.FormatInt(e.ParentID, 10),
		ReplyToId:     strconv.FormatInt(e.ReplyToID, 10),
		ReplyToName:   optionalString(e.ReplyToName),
		ReplyCount:    e.ReplyCount,
		CreatedAt:     e.CreateTime.Time.Unix(),
		ActorKind:     int32(e.Role),
		IsMine:        viewer.IsAuthor(e.Role, e.UserID),
	}
}

// formatOptionalISO8601 renders a nullable timestamptz as ISO 8601 UTC, with the
// empty string for NULL. Empty rather than a zero time: "" reads unambiguously
// as "not set", while 0001-01-01T00:00:00Z is a real instant that a client could
// try to compare against.
func formatOptionalISO8601(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return timeutil.FormatISO8601(*ts)
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func toPostDetailInfo(p *gen.CmsPost, userID int64, isLiked bool, sellerName, sellerLogo string, productMap map[string]*usecase.ProductBasicInfo) *pb.PostDetailInfo {
	if p == nil {
		return nil
	}

	info := &pb.PostDetailInfo{
		Id:            strconv.FormatInt(p.ID, 10),
		Content:       optionalString(p.Content),
		SellerId:      strconv.FormatInt(p.SellerID, 10),
		SellerName:    sellerName,
		SellerLogoKey: sellerLogo,
		LikeCount:     p.LikeCount,
		CommentCount:  p.CommentCount,
		ViewCount:     p.ViewCount,
		ShareCount:    p.ShareCount,
		IsLiked:       isLiked,
		CreatedAt:     p.CreateTime.Time.Unix(),
		// 56 §6.2.3：發布狀態與樂觀鎖。publish_status 是 PostPublishStatus 的
		// 數字碼字串（4401/4402/4403）；「已排程」不是本欄的值，由
		// scheduled_publish_at 非空表達。兩個時間欄為 ISO 8601 UTC，空字串 = 未設定。
		PublishStatus:      strconv.FormatInt(int64(p.PublishStatus), 10),
		ScheduledPublishAt: formatOptionalISO8601(p.ScheduledPublishAt),
		PublishedAt:        formatOptionalISO8601(p.PublishedAt),
		Version:            strconv.FormatInt(p.Version, 10),
	}
	if p.Image != nil && *p.Image != "" {
		info.ImageKeys = []string{*p.Image}
	}
	if p.ProductID != nil {
		prod := toPostProduct(*p.ProductID, true, productMap)
		if prod != nil {
			info.Products = []*pb.PostProduct{prod}
		}
	}
	return info
}

// toPostProduct converts a product_id to a PostProduct proto, enriched with data from productMap.
// Returns nil if product data cannot be found (deleted/unlisted product).
func toPostProduct(productID int64, valid bool, productMap map[string]*usecase.ProductBasicInfo) *pb.PostProduct {
	if !valid {
		return nil
	}
	key := fmt.Sprintf("%d", productID)
	if productMap != nil {
		if info, ok := productMap[key]; ok {
			return &pb.PostProduct{
				Id:       strconv.FormatInt(productID, 10),
				Name:     info.Name,
				ImageKey: info.ImageURL,
				Price:    info.Price,
			}
		}
		// productMap exists but product not found → product deleted/unlisted
		return nil
	}
	// productMap is nil → product service unavailable, return ID-only as fallback
	return &pb.PostProduct{Id: strconv.FormatInt(productID, 10)}
}

func postProductFromPointer(productID *int64, productMap map[string]*usecase.ProductBasicInfo) *pb.PostProduct {
	if productID == nil {
		return nil
	}
	return toPostProduct(*productID, true, productMap)
}

// resolveSellerDisplay batch-resolves store_id -> (name, logoKey) for display purposes.
//
// 🔴 Deliberately goes through BatchGetSellerMeta, not GetSellerInfo. GetSellerInfo ends up
// at seller-service GetPublicSellerProfileByStoreIDOrCode, whose predicate carries
// `AND m.deleted_at IS NULL` — a public-visibility filter. Display surfaces that structurally
// contain soft-deleted sellers (most visibly the ADM moderation queue's `deleted` tab, which is
// populated by the very seller.deleted event that soft-deletes the seller) then render a blank
// author. BatchGetSellerMeta is a bare PK lookup with no visibility predicate, which is the
// correct semantics for "resolve a name for display" — the same choice gateway's
// enrichPostsWithSellerInfo already makes via GetSellerBatch.
//
// GetSellerInfo itself is deliberately left alone: it carries `AND deleted_at IS NULL` and is
// used where an active seller is required.
func (s *ContentServer) resolveSellerDisplay(ctx context.Context, storeIDs []int64) map[int64][2]string {
	seen := make(map[int64]struct{}, len(storeIDs))
	ids := make([]int64, 0, len(storeIDs))
	for _, id := range storeIDs {
		if id == 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	metas, err := s.postUC.GetSellerMetaBatch(ctx, ids)
	if err != nil {
		return nil
	}
	result := make(map[int64][2]string, len(metas))
	for id, m := range metas {
		if m == nil {
			continue
		}
		result[id] = [2]string{m.Nickname, m.Avatar}
	}
	return result
}

// fetchAdminSellerMap collects unique seller IDs from admin post rows and batch-fetches seller name.
func (s *ContentServer) fetchAdminSellerMap(ctx context.Context, posts []gen.AdminListPostsRow) map[int64]string {
	ids := make([]int64, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.SellerID)
	}
	pairs := s.resolveSellerDisplay(ctx, ids)
	if len(pairs) == 0 {
		return nil
	}
	result := make(map[int64]string, len(pairs))
	for id, pair := range pairs {
		result[id] = pair[0]
	}
	return result
}

// fetchSellerMap collects unique seller IDs from posts and batch-fetches seller name/logo.
func (s *ContentServer) fetchSellerMap(ctx context.Context, posts []*gen.CmsPost) map[int64][2]string {
	seen := make(map[int64]struct{})
	for _, p := range posts {
		if p.SellerID != 0 {
			seen[p.SellerID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(seen))
	for sellerID := range seen {
		ids = append(ids, sellerID)
	}
	return s.resolveSellerDisplay(ctx, ids)
}

// fetchProductMap collects product IDs from posts and batch-fetches product data.
func (s *ContentServer) fetchProductMap(ctx context.Context, posts []*gen.CmsPost) map[string]*usecase.ProductBasicInfo {
	seen := make(map[string]struct{})
	var ids []string
	for _, p := range posts {
		if p.ProductID != nil {
			key := fmt.Sprintf("%d", *p.ProductID)
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				ids = append(ids, key)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return s.postUC.GetProductsByIDs(ctx, ids)
}

func toCommentInfo(c *gen.CmsComment, viewer usecase.Caller, isLiked bool, replies []*pb.CommentInfo) *pb.CommentInfo {
	if c == nil {
		return nil
	}
	return &pb.CommentInfo{
		Id:            strconv.FormatInt(c.ID, 10),
		PostId:        strconv.FormatInt(c.PostID, 10),
		UserName:      optionalString(c.UserName),
		UserAvatarKey: optionalString(c.UserAvatar),
		Content:       optionalString(c.Content),
		ParentId:      strconv.FormatInt(c.ParentID, 10),
		ReplyToId:     strconv.FormatInt(c.ReplyToID, 10),
		ReplyToName:   optionalString(c.ReplyToName),
		LikeCount:     c.LikeCount,
		ReplyCount:    c.ReplyCount,
		IsLiked:       isLiked,
		CreatedAt:     c.CreateTime.Time.Unix(),
		Replies:       replies,
		ActorKind:     int32(c.Role),
		IsMine:        viewer.IsAuthor(c.Role, c.UserID),
	}
}
