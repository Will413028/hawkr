// Package review 是 review 模組的 Connect handler：前台的 hawkr.review.v1alpha1。這裡只做欄位的轉換與顯示資料的組裝
// （作者名稱與頭像來自 buyer、圖片網址由 rpc.Images 組成），規則在 internal/modules/review。
package review

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Reads 是評價的讀取來源（*review.Module）。viewerBuyerID 大於 0 時模組套用觀看者的封鎖。
type Reads interface {
	ListProductReviews(ctx context.Context, productID, viewerBuyerID int64, f review.Filter, after *review.Cursor, size int32) (review.Page, error)
	ListStoreReviews(ctx context.Context, storeID, viewerBuyerID int64, f review.Filter, after *review.Cursor, size int32) (review.Page, error)
	ListOwnStoreReviews(ctx context.Context, storeID int64, f review.Filter, after *review.Cursor, size int32) (review.Page, error)
	ListMyReviews(ctx context.Context, buyerID, orderItemID int64, after *review.Cursor, size int32) (review.Page, error)
	GetReview(ctx context.Context, id uuid.UUID, viewerBuyerID int64) (review.Review, error)
	ReviewStore(ctx context.Context, id uuid.UUID) (review.ReviewStore, error)
	StoreStats(ctx context.Context, storeID int64) (review.StoreStats, error)
	ProductStats(ctx context.Context, productID int64) (review.ProductStats, error)
}

// Writes 是評價的寫入來源（*review.Module）。
type Writes interface {
	Submit(ctx context.Context, in review.SubmitInput) (review.Review, error)
	Reply(ctx context.Context, in review.ReplyInput) (review.Review, error)
}

// Buyers 是呼叫者的買家身分與作者顯示資料的來源（*buyer.Module）。
type Buyers interface {
	CurrentBuyerID(ctx context.Context, accountID int64) (int64, error)
	BatchDisplay(ctx context.Context, ids []int64) ([]buyer.Display, error)
}

// Stores 是店家的來源（*seller.Module）：呼叫者此刻的店，以及依 ID 讀店（含已關閉的）。
type Stores interface {
	CurrentStore(ctx context.Context, accountID int64) (seller.Store, error)
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// presenter 是 Public 與 App 共用的組裝：觀看者身分與評價轉成回應所需的顯示資料。
type presenter struct {
	buyers Buyers
	images rpc.Images
	log    *slog.Logger
	// reasonOf 取出模組錯誤帶的業務原因；正式組裝是 review.ReasonOf，測試換成固定的原因（模組的原因型別不能在模組外建構）。
	reasonOf func(error) review.Reason
}

func newPresenter(buyers Buyers, images rpc.Images, log *slog.Logger) (*presenter, error) {
	if log == nil {
		return nil, fmt.Errorf("httpapi/review：需要 logger")
	}
	if buyers == nil {
		return nil, fmt.Errorf("httpapi/review：需要 Buyers")
	}
	return &presenter{buyers: buyers, images: images, log: log, reasonOf: review.ReasonOf}, nil
}

// viewer 回傳觀看者的買家 ID；訪客（沒有登入、停權帳號與無效 token 都以訪客執行）與管理員是 0。
func (p *presenter) viewer(ctx context.Context) (int64, error) {
	pr, ok := rpc.PrincipalFrom(ctx)
	if !ok || pr.Kind != identity.PrincipalAccount {
		return 0, nil
	}
	id, err := p.buyers.CurrentBuyerID(ctx, pr.ID)
	if err != nil {
		return 0, rpc.BuyerInvariant(pr.ID, err)
	}
	return id, nil
}

// cursor 是評價頁的 page token 內容：上一頁最後一則的 (create_time, id)。
type cursor struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}

func (c cursor) Valid() bool { return !c.CreatedAt.IsZero() && c.ID != uuid.UUID{} }

func decodeCursor(scope, token string) (*review.Cursor, error) {
	c, err := rpc.ValidCursor[cursor](scope, token)
	if err != nil || c == nil {
		return nil, err
	}
	return &review.Cursor{CreatedAt: c.CreatedAt, ID: c.ID}, nil
}

func nextToken(scope string, next *review.Cursor) string {
	if next == nil {
		return ""
	}
	return rpc.TokenFor(scope, cursor{CreatedAt: next.CreatedAt, ID: next.ID})
}

func boolKey(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// filterOf 把請求的 rating 與 with_images 轉成模組的篩選；明確送出的 rating 不在 1 到 5 是 invalid_argument（不當成不篩選）。
func filterOf(rating *int32, withImages bool) (review.Filter, error) {
	if rating != nil && (*rating < 1 || *rating > 5) {
		return review.Filter{}, rpc.RequiredField("rating", "rating 必須在 1 到 5")
	}
	return review.Filter{Rating: ratingOf(rating), WithImages: withImages}, nil
}

func ratingOf(r *int32) int {
	if r == nil {
		return 0
	}
	return int(*r)
}

// respond 把一頁評價組成列表回應：作者顯示資料補上，有下一頁時帶 page token。
func respond[T any](ctx context.Context, p *presenter, page review.Page, scope string, withOrder bool, viewer int64, build func([]*pb.Review, string) *T) (*connect.Response[T], error) {
	reviews, err := p.toReviews(ctx, page.Reviews, withOrder, viewer)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(build(reviews, nextToken(scope, page.Next))), nil
}

// filterScope 是帶篩選的列表的 page token scope：owner 是商品或店家 ID，換了篩選就不能沿用 token。
func filterScope(procedure, owner string, f review.Filter) string {
	return rpc.FilterScope(procedure, owner, strconv.Itoa(f.Rating), boolKey(f.WithImages))
}

// reviewPaths 把模組回報的欄位名對到請求的欄位路徑。
var reviewPaths = rpc.FieldPaths{
	"Filter.Rating": "rating", "OrderNo": "order_no", "OrderItemID": "order_item_id", "Rating": "rating",
	"Content": "content", "StagedTokens": "image_staged_tokens",
}

// reasons 是模組的業務原因對到契約的 ErrorReason（apperr.Reason）：只有 client 會依它分支的才對應。
var reasons = map[review.Reason]apperr.Reason{
	review.ReasonAlreadyExists:      apperr.ReasonReviewAlreadyExists,
	review.ReasonOrderNotReviewable: apperr.ReasonOrderNotReviewable,
	review.ReasonWindowExpired:      apperr.ReasonReviewWindowExpired,
	review.ReasonNotReplyable:       apperr.ReasonReviewNotReplyable,
}

// failure 把模組的錯誤轉成對外的錯誤：業務原因附上契約的 ErrorReason，欄位對到請求路徑。
func (p *presenter) failure(err error) error {
	if err == nil {
		return nil
	}
	var ae *apperr.Error
	if r, ok := reasons[p.reasonOf(err)]; ok && errors.As(err, &ae) {
		ae.Reason = r
	}
	return rpc.WithFieldPaths(err, reviewPaths)
}

// toReviews 把一頁評價轉成回應：viewer 是觀看者的買家 ID（訪客與店家視圖是 0），評價是他寫的 is_mine 為真；作者名稱與頭像以一次批次查詢補上；withOrder 為真時帶訂單編號與訂單項目 ID（只有作者本人與店家自己的視圖）。
func (p *presenter) toReviews(ctx context.Context, in []review.Review, withOrder bool, viewer int64) ([]*pb.Review, error) {
	ids := make([]int64, 0, len(in))
	seen := make(map[int64]bool, len(in))
	for _, r := range in {
		if !seen[r.BuyerID] {
			seen[r.BuyerID] = true
			ids = append(ids, r.BuyerID)
		}
	}
	displays := make(map[int64]buyer.Display, len(ids))
	for start := 0; start < len(ids); start += buyer.MaxBatchSize {
		rows, err := p.buyers.BatchDisplay(ctx, ids[start:min(start+buyer.MaxBatchSize, len(ids))])
		if err != nil {
			return nil, err
		}
		for _, d := range rows {
			displays[d.BuyerID] = d
		}
	}
	out := make([]*pb.Review, len(in))
	for i, r := range in {
		v, err := p.toReview(r, displays[r.BuyerID], withOrder, viewer)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (p *presenter) toReview(r review.Review, d buyer.Display, withOrder bool, viewer int64) (*pb.Review, error) {
	out := &pb.Review{
		Id:         r.ID.String(),
		ProductId:  rpc.FormatID(r.ProductID),
		StoreId:    rpc.FormatID(r.StoreID),
		Rating:     int32(r.Rating), //nolint:gosec // 評價分數由 reviews_rating_check 保證 1 到 5
		Content:    r.Content,
		SkuName:    r.SkuName,
		Price:      r.Price,
		CreateTime: timestamppb.New(r.CreatedAt),
		Author:     &pb.ReviewAuthor{},
		IsMine:     viewer > 0 && r.BuyerID == viewer,
	}
	for _, img := range r.Images {
		im, err := p.image(img.ObjectKey)
		if err != nil {
			return nil, err
		}
		out.Images = append(out.Images, im)
	}
	if r.ItemImageKey != nil {
		im, err := p.image(*r.ItemImageKey)
		if err != nil {
			return nil, err
		}
		out.ItemImage = im
	}
	if r.Reply != nil {
		out.Reply = &pb.ReviewReply{Content: r.Reply.Content, ReplyTime: timestamppb.New(r.Reply.RepliedAt)}
	}
	if !d.Deleted {
		out.Author.Name = d.DisplayName
		if d.Avatar != nil {
			im, err := p.image(*d.Avatar)
			if err != nil {
				return nil, err
			}
			out.Author.Avatar = im
		}
	}
	if withOrder {
		orderNo, itemID := r.OrderNo, rpc.FormatID(r.OrderItemID)
		out.OrderNo, out.OrderItemId = &orderNo, &itemID
	}
	return out, nil
}

func (p *presenter) image(key string) (*commonpb.Image, error) {
	if err := rpc.ObjectKey(key); err != nil {
		return nil, err
	}
	return p.images.Image(key), nil
}
