package seller

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// PublicModule 是 SellerPublicService 讀寫店家的介面。
type PublicModule interface {
	PublicStoreByCode(ctx context.Context, storeCode int64) (model.PublicStore, error)
	ListPublicStores(ctx context.Context, in model.ListPublicStoresInput) (model.PublicStorePage, error)
	StoreCards(ctx context.Context, storeIDs []int64) ([]model.StoreCard, error)
	RecordProductView(ctx context.Context, productID int64) error
}

// FollowerCounts 是店家粉絲數的來源（buyer 模組擁有追蹤）：沒有粉絲的店家是 0。
type FollowerCounts interface {
	SubscriberCounts(ctx context.Context, storeIDs []int64) (map[int64]int64, error)
}

// RatingStats 是一家有評價的店的統計（Count 至少 1，Average 是 0 到 5）。
type RatingStats struct {
	Count   int64
	Average float64
}

// Ratings 是店家評價統計的來源（review 還在舊服務，由組裝點接上）：只回有評價的店，沒有出現在回傳裡的店沒有評價；
// 讀取失敗回錯誤，不推定為沒有評價。
type Ratings interface {
	StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]RatingStats, error)
}

// maxBatchStores 是 BatchGetStores 一次最多接受的 id 數量。
const maxBatchStores = model.MaxBatchSize

// Public 是 SellerPublicService 的 handler。
type Public struct {
	m      PublicModule
	stats  *StatsSource
	images rpc.Images
	web    rpc.WebOrigin
}

var _ sellerv1alpha1connect.SellerPublicServiceHandler = (*Public)(nil)

// NewPublic 建立前台的店家讀取 handler；stats 是店面頁統計與卡片評分的組裝（NewStatsSource），web 把入駐文件的站內路徑組成絕對網址。
func NewPublic(m PublicModule, stats *StatsSource, images rpc.Images, web rpc.WebOrigin) (*Public, error) {
	if stats == nil {
		return nil, ErrStatsSourceRequired
	}
	if web.IsZero() {
		return nil, ErrWebOriginRequired
	}
	return &Public{m: m, stats: stats, images: images, web: web}, nil
}

// GetSellerAgreements 回傳三份入駐文件的現行版本，內容來自 seller 模組的常數表，不查資料庫；網址一律是絕對網址。
func (h *Public) GetSellerAgreements(context.Context, *connect.Request[pb.GetSellerAgreementsRequest]) (*connect.Response[pb.GetSellerAgreementsResponse], error) {
	kinds := model.AgreementKinds()
	out := make([]*pb.SellerAgreement, 0, len(kinds))
	for _, k := range kinds {
		a, ok := model.CurrentAgreement(k)
		if !ok {
			return nil, fmt.Errorf("httpapi/seller：文件種類 %q 沒有現行版本", k)
		}
		u, err := h.web.URL(a.URL)
		if err != nil {
			return nil, fmt.Errorf("httpapi/seller：文件種類 %q：%w", k, err)
		}
		out = append(out, &pb.SellerAgreement{Kind: rpc.EnumToProto[pb.AgreementKind](k), Version: a.Version, Url: u})
	}
	return connect.NewResponse(&pb.GetSellerAgreementsResponse{Agreements: out}), nil
}

func (h *Public) GetStore(ctx context.Context, req *connect.Request[pb.GetStoreRequest]) (*connect.Response[pb.GetStoreResponse], error) {
	code, err := rpc.ParseID("store_code", req.Msg.GetStoreCode())
	if err != nil {
		return nil, err
	}
	s, err := h.m.PublicStoreByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	store, err := h.toPublicStore(s)
	if err != nil {
		return nil, err
	}
	out := &pb.GetStoreResponse{Store: store}
	out.Stats = h.stats.Store(ctx, s.StoreID)
	return connect.NewResponse(out), nil
}

func (h *Public) toPublicStore(s model.PublicStore) (*pb.PublicStore, error) {
	out := &pb.PublicStore{
		Id: rpc.FormatID(s.StoreID), StoreCode: rpc.FormatID(s.StoreCode), StoreName: s.StoreName,
		Description: deref(s.Description), TownCode: deref(s.TownCode), Prize: deref(s.Prize), WebsiteUrl: deref(s.WebsiteURL),
		FoodBusinessRegistrationNumber: foodRegistrationNumber(s.Store),
		UpdateTime:                     timestamppb.New(s.UpdateTime),
	}
	if s.LogoKey != nil {
		if err := rpc.ObjectKey(*s.LogoKey); err != nil {
			return nil, err
		}
		out.Logo = h.images.Image(*s.LogoKey)
	}
	if s.CoverKey != nil {
		if err := rpc.ObjectKey(*s.CoverKey); err != nil {
			return nil, err
		}
		out.CoverImage = h.images.Image(*s.CoverKey)
	}
	return out, nil
}

func (h *Public) ListStores(ctx context.Context, req *connect.Request[pb.ListStoresRequest]) (*connect.Response[pb.ListStoresResponse], error) {
	scope := rpc.FilterScope(sellerv1alpha1connect.SellerPublicServiceListStoresProcedure)
	after, err := rpc.ValidCursor[model.PublicStoreCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListPublicStores(ctx, model.ListPublicStoresInput{PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	out := &pb.ListStoresResponse{Stores: make([]*pb.PublicStore, len(page.Items))}
	for i, s := range page.Items {
		if out.Stores[i], err = h.toPublicStore(s); err != nil {
			return nil, err
		}
	}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	return connect.NewResponse(out), nil
}

func (h *Public) BatchGetStores(ctx context.Context, req *connect.Request[pb.BatchGetStoresRequest]) (*connect.Response[pb.BatchGetStoresResponse], error) {
	raw := req.Msg.GetIds()
	if len(raw) == 0 {
		return nil, rpc.RequiredField("ids", "ids 至少要有一個店家 id")
	}
	if len(raw) > maxBatchStores {
		return nil, rpc.RequiredField("ids", "ids 最多 "+strconv.Itoa(maxBatchStores)+" 個")
	}
	ids := make([]int64, len(raw))
	for i, s := range raw {
		id, err := rpc.ParseID("ids", s)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	cards, err := h.m.StoreCards(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := &pb.BatchGetStoresResponse{Stores: make([]*commonpb.StoreCard, len(cards))}
	storeIDs := make([]int64, len(cards))
	for i, c := range cards {
		if out.Stores[i], err = ToStoreCard(h.images, c); err != nil {
			return nil, err
		}
		storeIDs[i] = c.StoreID
	}
	h.stats.RateCards(ctx, storeIDs, out.Stores)
	return connect.NewResponse(out), nil
}

// RecordProductView 只計訪客：有呼叫者（登入的帳號或管理員）時回成功但不計數，登入買家的瀏覽由 buyer 的 ProductViewed 計入。
// 每個來源 IP 的額度在入口（httpapi 的 tierProductView）。
func (h *Public) RecordProductView(ctx context.Context, req *connect.Request[pb.RecordProductViewRequest]) (*connect.Response[pb.RecordProductViewResponse], error) {
	productID, err := rpc.ParseID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	if _, signedIn := rpc.PrincipalFrom(ctx); signedIn {
		return connect.NewResponse(&pb.RecordProductViewResponse{}), nil
	}
	if err := h.m.RecordProductView(ctx, productID); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RecordProductViewResponse{}), nil
}
