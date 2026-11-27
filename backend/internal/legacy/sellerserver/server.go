// Package sellerserver exposes the SellerService methods still used by legacy services (Java and the late-migrating Go services):
// the read methods served from the seller module. The other methods stay Unimplemented.
package sellerserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/seller"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/seller/sellerconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerreason"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

// RatingStats 是一家有評價的店的統計。
type RatingStats struct {
	Count   int64
	Average float64
}

// Ratings 是店家評價統計的來源（review 還在舊服務，由組裝點接上）：只回有評價的店，沒有出現在回傳裡的店沒有評價；讀取失敗回錯誤，
// 不推定為沒有評價。
type Ratings interface {
	StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]RatingStats, error)
}

// Followers 是店家粉絲數的來源（buyer 模組擁有追蹤）：沒有粉絲的店家是 0。
type Followers interface {
	SubscriberCounts(ctx context.Context, storeIDs []int64) (map[int64]int64, error)
}

const ratingTimeout = 300 * time.Millisecond

const (
	codeSellerNotFound = 9233
)

// Server 實作 legacy 的 SellerService 讀取方法。
type Server struct {
	sellerconnect.UnimplementedSellerServiceHandler
	seller    *seller.Module
	ratings   Ratings
	followers Followers
	log       *slog.Logger
}

var _ sellerconnect.SellerServiceHandler = (*Server)(nil)

// New 建立 server；ratings 與 followers 是評分與粉絲數的來源，必填。
func New(m *seller.Module, ratings Ratings, followers Followers, log *slog.Logger) *Server {
	if m == nil || ratings == nil || followers == nil {
		panic("sellerserver: missing dependency")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Server{seller: m, ratings: ratings, followers: followers, log: log}
}

func failure(err error) (int32, string, error) {
	switch apperr.KindOf(err) {
	case apperr.KindNotFound:
		return 404, err.Error(), nil
	case apperr.KindInvalidInput:
		return 400, err.Error(), nil
	case apperr.KindFailedPrecondition:
		return 428, err.Error(), nil
	case apperr.KindInternal, apperr.KindVersionConflict, apperr.KindAlreadyExists, apperr.KindUnauthenticated,
		apperr.KindPermissionDenied, apperr.KindUnavailable, apperr.KindResourceExhausted:
		return 0, "", err
	default:
		return 0, "", err
	}
}

func format(id int64) string { return strconv.FormatInt(id, 10) }

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func roundRating(r float64) float64 { return math.Round(r*100) / 100 }

func lifecycleStatus(s seller.Store) (string, error) {
	class, err := sellerreason.Classify(s.Sanctions)
	if err != nil {
		return "", err
	}
	switch {
	case s.Status == seller.Closed:
		return "closed", nil
	case class == sellerreason.Admin:
		return "suspended", nil
	case class == sellerreason.Governance:
		return "suspended_governance", nil
	case s.Status == seller.PendingDeletion:
		return "pending_deletion", nil
	case s.Status == seller.Active && class == sellerreason.None:
		return "active", nil
	}
	return "", fmt.Errorf("sellerserver：不認得的店家狀態 status=%q class=%d", s.Status, class)
}

func location(town *string) (cityName, district, postal string) {
	if town == nil {
		return "", "", ""
	}
	t, ok := twaddr.TownByCode(*town)
	if !ok {
		return "", "", ""
	}
	return strings.ReplaceAll(t.CountyName, "臺", "台"), t.Name, t.PostalCode
}

func sellerInfo(s seller.Store, rating float64) (*pb.SellerInfo, error) {
	lifecycle, err := lifecycleStatus(s)
	if err != nil {
		return nil, err
	}
	cityName, district, postal := location(s.TownCode)
	return &pb.SellerInfo{
		AccountId:           s.AccountID,
		Name:                s.StoreName,
		LogoKey:             deref(s.LogoKey),
		Description:         deref(s.Description),
		ContactPhone:        deref(s.ContactPhone),
		Rating:              roundRating(rating),
		CoverKey:            deref(s.CoverKey),
		Prize:               deref(s.Prize),
		City:                cityName,
		District:            district,
		AddressDetail:       deref(s.AddressDetail),
		PostalCode:          postal,
		OnboardingCompleted: s.OnboardingCompleted,
		Nickname:            s.StoreName,
		Phone:               deref(s.ContactPhone),
		StoreId:             format(s.StoreID),
		StoreCode:           format(s.StoreCode),
		LifecycleStatus:     lifecycle,
		WebsiteUrl:          deref(s.WebsiteURL),
		Version:             format(s.Version),
	}, nil
}

func sellerMeta(e seller.StoreExposure) *pb.SellerMeta {
	s := e.Store
	cityName, district, postal := location(s.TownCode)
	return &pb.SellerMeta{
		AccountId:         s.AccountID,
		SellerName:        s.StoreName,
		LogoKey:           deref(s.LogoKey),
		ZipCode:           postal,
		CanAcceptOrders:   e.Exposable,
		CanManageProducts: s.Can(seller.CapProductManage),
		HasPublicCard:     e.HasPublicCard,
		City:              cityName,
		District:          district,
		StoreId:           format(s.StoreID),
		StoreCode:         format(s.StoreCode),
	}
}

func (s *Server) bestEffortRatings(ctx context.Context, storeIDs []int64) map[int64]RatingStats {
	ctx, cancel := context.WithTimeout(ctx, ratingTimeout)
	defer cancel()
	stats, err := s.ratings.StoreRatingStats(ctx, storeIDs)
	if err != nil {
		s.log.WarnContext(ctx, "sellerserver：讀取評價統計失敗，rating 不填", "store_ids", storeIDs, "error", err)
		return nil
	}
	return stats
}

// GetSeller 對已關閉的店回找不到：舊 seller-service 關店一定同時寫 deleted_at，兩條查詢都濾掉它（store_id 路徑回 404，account_id
// 路徑回 9233）。停權狀態與原因碼來自模組同一次讀取。
func (s *Server) GetSeller(ctx context.Context, req *connect.Request[pb.GetSellerRequest]) (*connect.Response[pb.SellerResponse], error) {
	var (
		st  seller.Store
		err error
	)
	byAccount := req.Msg.GetStoreId() <= 0
	switch {
	case !byAccount:
		st, err = s.seller.Store(ctx, req.Msg.GetStoreId())
	case req.Msg.GetAccountId() <= 0:
		err = apperr.NotFound("找不到目前的店家")
	default:
		st, err = s.seller.CurrentStore(ctx, req.Msg.GetAccountId())
	}
	if err == nil && st.Status == seller.Closed {
		err = apperr.NotFound("商家不存在或已關閉")
	}
	if err != nil {
		if byAccount && apperr.KindOf(err) == apperr.KindNotFound {
			return connect.NewResponse(&pb.SellerResponse{Code: codeSellerNotFound, Message: "找不到您的商家資料"}), nil
		}
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.SellerResponse{Code: code, Message: msg}), nil
	}
	info, err := sellerInfo(st, s.bestEffortRatings(ctx, []int64{st.StoreID})[st.StoreID].Average)
	if err != nil {
		return nil, connectError(err)
	}
	if info.LifecycleStatus == "suspended" {
		if info.SuspensionReasonCode, _, err = sellerreason.Of(st.Sanctions); err != nil {
			return nil, connectError(err)
		}
	}
	return connect.NewResponse(&pb.SellerResponse{Data: info}), nil
}

func (s *Server) currentStore(ctx context.Context, accountID int64) (seller.Store, error) {
	if accountID <= 0 {
		return seller.Store{}, apperr.NotFound("找不到目前的店家")
	}
	return s.seller.CurrentStore(ctx, accountID)
}

func positive(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}

func (s *Server) ResolveSellerAccountID(ctx context.Context, req *connect.Request[pb.ResolveSellerAccountIDRequest]) (*connect.Response[pb.ResolveSellerAccountIDResponse], error) {
	if req.Msg.GetStoreId() <= 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("seller not found for store_id"))
	}
	st, err := s.seller.Store(ctx, req.Msg.GetStoreId())
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&pb.ResolveSellerAccountIDResponse{AccountId: st.AccountID}), nil
}

func (s *Server) GetCurrentStoreID(ctx context.Context, req *connect.Request[pb.GetCurrentStoreIDRequest]) (*connect.Response[pb.GetCurrentStoreIDResponse], error) {
	st, err := s.currentStore(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&pb.GetCurrentStoreIDResponse{StoreId: st.StoreID}), nil
}

// GetSellerBatch 不填 rating：呼叫方（im 讀 store_id、name、logo_key、store_code、lifecycle_status，aftersales 讀 store_id、name、
// logo_key）都不讀它，補評分只會讓每次批次多一個 review-service 的依賴與最多 300ms 的延遲。
func (s *Server) GetSellerBatch(ctx context.Context, req *connect.Request[pb.GetSellerBatchRequest]) (*connect.Response[pb.SellerBatchResponse], error) {
	stores, err := s.seller.BatchStores(ctx, positive(req.Msg.GetStoreIds()))
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.SellerBatchResponse{Code: code, Message: msg}), nil
	}
	out := make([]*pb.SellerInfo, 0, len(stores))
	for _, st := range stores {
		info, err := sellerInfo(st, 0)
		if err != nil {
			return nil, connectError(err)
		}
		out = append(out, info)
	}
	return connect.NewResponse(&pb.SellerBatchResponse{Data: out}), nil
}

func (s *Server) BatchGetSellerMeta(ctx context.Context, req *connect.Request[pb.BatchGetSellerMetaRequest]) (*connect.Response[pb.BatchGetSellerMetaResponse], error) {
	rows, err := s.seller.StoresWithExposure(ctx, positive(req.Msg.GetStoreIds()))
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.BatchGetSellerMetaResponse{Code: code, Message: msg}), nil
	}
	out := make(map[int64]*pb.SellerMeta, len(rows))
	for _, row := range rows {
		out[row.Store.StoreID] = sellerMeta(row)
	}
	return connect.NewResponse(&pb.BatchGetSellerMetaResponse{Sellers: out}), nil
}

func (s *Server) ResolvePrincipals(ctx context.Context, req *connect.Request[pb.ResolvePrincipalsRequest]) (*connect.Response[pb.ResolvePrincipalsResponse], error) {
	resolved, err := s.seller.ResolvePrincipals(ctx, positive(req.Msg.GetIds()))
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.ResolvePrincipalsResponse{Code: code, Message: msg}), nil
	}
	unresolved := make([]int64, 0)
	for _, id := range req.Msg.GetIds() {
		if _, ok := resolved[id]; !ok {
			unresolved = append(unresolved, id)
		}
	}
	return connect.NewResponse(&pb.ResolvePrincipalsResponse{Principals: resolved, UnresolvedIds: unresolved}), nil
}

func (s *Server) ListStoreIDsByAccount(ctx context.Context, req *connect.Request[pb.ListStoreIDsByAccountRequest]) (*connect.Response[pb.ListStoreIDsByAccountResponse], error) {
	if req.Msg.GetAccountId() <= 0 {
		return connect.NewResponse(&pb.ListStoreIDsByAccountResponse{StoreIds: []int64{}}), nil
	}
	ids, err := s.seller.StoreIDsByAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.ListStoreIDsByAccountResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.ListStoreIDsByAccountResponse{StoreIds: ids}), nil
}

// GetSellerStats 的 total_products、total_orders、total_sales 一直是 0（舊 server 也沒填，舊 AI 服務只讀 rating 與 followers），
// 不填。評分與粉絲數各自降級（舊 server 的行為）：讀不到的那一欄留 0、記 warn（含店家與欄位），另一欄照填；店家本身讀不到
// 仍是錯誤。
func (s *Server) GetSellerStats(ctx context.Context, req *connect.Request[pb.GetSellerStatsRequest]) (*connect.Response[pb.SellerStatsResponse], error) {
	var (
		st  seller.Store
		err error
	)
	if req.Msg.GetStoreId() > 0 {
		st, err = s.seller.Store(ctx, req.Msg.GetStoreId())
	} else {
		st, err = s.currentStore(ctx, req.Msg.GetAccountId())
	}
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.SellerStatsResponse{Code: code, Message: msg}), nil
	}
	if st.Status == seller.Closed {
		return connect.NewResponse(&pb.SellerStatsResponse{Code: 404, Message: "商家不存在或已關閉"}), nil
	}
	ids := []int64{st.StoreID}
	stats := &pb.SellerStats{StoreId: format(st.StoreID)}
	if counts, err := s.followers.SubscriberCounts(ctx, ids); err != nil {
		s.log.WarnContext(ctx, "sellerserver：讀取粉絲數失敗，followers 留 0", "store_id", st.StoreID, "field", "followers", "error", err)
	} else {
		stats.Followers = clamp32(counts[st.StoreID])
	}
	ratingCtx, cancel := context.WithTimeout(ctx, ratingTimeout)
	defer cancel()
	if ratings, err := s.ratings.StoreRatingStats(ratingCtx, ids); err != nil {
		s.log.WarnContext(ctx, "sellerserver：讀取評價統計失敗，rating 與 review_count 留 0", "store_id", st.StoreID, "field", "rating", "error", err)
	} else {
		stats.ReviewCount = clamp32(ratings[st.StoreID].Count)
		stats.Rating = roundRating(ratings[st.StoreID].Average)
	}
	return connect.NewResponse(&pb.SellerStatsResponse{Data: stats}), nil
}

func clamp32(n int64) int32 {
	return int32(min(max(n, 0), math.MaxInt32))
}

// GetSellerShippingProfile 只有 contact_email 有讀者（shipping-service）。已關閉的店回找不到（舊 seller-service 的 store_id 路徑濾掉 deleted_at，關店一定寫它）。
// 商店電話、地址是選填，缺的欄位照實回空字串，不因此拒絕：contact_email 讀店家的 support_email（沒有填時空字串）。
func (s *Server) GetSellerShippingProfile(ctx context.Context, req *connect.Request[pb.GetSellerShippingProfileRequest]) (*connect.Response[pb.GetSellerShippingProfileResponse], error) {
	var (
		st  seller.Store
		err error
	)
	if req.Msg.GetStoreId() > 0 {
		st, err = s.seller.Store(ctx, req.Msg.GetStoreId())
	} else {
		st, err = s.currentStore(ctx, req.Msg.GetAccountId())
	}
	if err == nil && st.Status == seller.Closed {
		err = apperr.NotFound("商家不存在或已關閉")
	}
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.GetSellerShippingProfileResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.GetSellerShippingProfileResponse{Data: shippingProfile(st)}), nil
}

func shippingProfile(st seller.Store) *pb.SellerShippingProfile {
	cityName, district, postal := location(st.TownCode)
	return &pb.SellerShippingProfile{
		StoreId:       format(st.StoreID),
		SenderName:    st.StoreName,
		SenderPhone:   deref(st.ContactPhone),
		City:          cityName,
		District:      district,
		AddressDetail: deref(st.AddressDetail),
		ZipCode:       postal,
		ContactEmail:  deref(st.SupportEmail),
	}
}

func (s *Server) MatchCertificateHolder(ctx context.Context, req *connect.Request[pb.MatchCertificateHolderRequest]) (*connect.Response[pb.MatchCertificateHolderResponse], error) {
	res, err := s.seller.MatchCertificateHolder(ctx, req.Msg.GetStoreId(), req.Msg.GetHolderName())
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&pb.MatchCertificateHolderResponse{Matched: res.Matched, MatchedField: string(res.Field), HasReference: res.HasReference}), nil
}

func (s *Server) ListPublicStoreIDsByCity(ctx context.Context, req *connect.Request[pb.ListPublicStoreIDsByCityRequest]) (*connect.Response[pb.ListPublicStoreIDsByCityResponse], error) {
	c, ok := cityOf(req.Msg.GetCity())
	if !ok {
		return connect.NewResponse(&pb.ListPublicStoreIDsByCityResponse{Code: 400, Message: "invalid Taiwan city"}), nil
	}
	byCounty, err := s.seller.ExposableStoreIDsByCounty(ctx, []string{c.CountyCode})
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.ListPublicStoreIDsByCityResponse{Code: code, Message: msg}), nil
	}
	return connect.NewResponse(&pb.ListPublicStoreIDsByCityResponse{City: c.Name, CityCode: c.ISO, StoreIds: formatAll(byCounty[c.CountyCode])}), nil
}

const previewSize = 6

func (s *Server) ListPublicStoresByCity(ctx context.Context, _ *connect.Request[pb.ListPublicStoresByCityRequest]) (*connect.Response[pb.ListPublicStoresByCityResponse], error) {
	byCounty, err := s.seller.ExposableStoreIDsByCounty(ctx, countyCodes())
	if err != nil {
		code, msg, fatal := failure(err)
		if fatal != nil {
			return nil, connectError(fatal)
		}
		return connect.NewResponse(&pb.ListPublicStoresByCityResponse{Code: code, Message: msg}), nil
	}
	out := make([]*pb.PublicCityStores, len(cities))
	for i, c := range cities {
		ids := byCounty[c.CountyCode]
		out[i] = &pb.PublicCityStores{CityCode: c.ISO, CityName: c.Name, StoreIds: formatAll(ids), PreviewStoreIds: formatAll(ids[:min(len(ids), previewSize)])}
	}
	return connect.NewResponse(&pb.ListPublicStoresByCityResponse{Message: "success", Cities: out}), nil
}

func formatAll(ids []int64) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = format(id)
	}
	return out
}

// SearchSellerProspects 不填 public_email、public_phone、public_social（舊 server 的窄投影也清空它們，採購候選只需要網站與品牌圖）。
func (s *Server) SearchSellerProspects(ctx context.Context, req *connect.Request[pb.SearchSellerProspectsRequest]) (*connect.Response[pb.SearchSellerProspectsResponse], error) {
	hits, err := s.seller.SearchProspects(ctx, seller.ProspectSearchInput{
		Category: req.Msg.GetCategory(), Region: req.Msg.GetRegion(), Keywords: req.Msg.GetKeywords(),
		CategoryIDs: req.Msg.GetCategoryIds(), Limit: req.Msg.GetLimit(),
	})
	if err != nil {
		return nil, connectError(err)
	}
	out := make([]*pb.SellerProspectHit, len(hits))
	for i, h := range hits {
		p := h.Prospect
		out[i] = &pb.SellerProspectHit{
			Prospect: &pb.SellerProspectItem{
				Id:              format(p.ID),
				DisplayName:     p.DisplayName,
				VendorType:      strings.ToLower(string(p.VendorType)),
				Category:        deref(p.Category),
				Region:          deref(p.Region),
				ProductSummary:  deref(p.ProductSummary),
				Positioning:     deref(p.Positioning),
				ResearchSummary: deref(p.ResearchSummary),
				PriorityLetter:  deref(p.PriorityLetter),
				PriorityStars:   strconv.FormatInt(int64(p.PriorityStars), 10),
				PublicWebsite:   deref(h.Website),
				UpdatedAt:       p.UpdatedAt.UTC().Format(time.RFC3339),
				LogoKey:         deref(p.LogoKey),
				CoverKey:        deref(p.CoverKey),
			},
			HitCategory:    h.HitCategory,
			HitRegion:      h.HitRegion,
			HitKeywords:    h.HitKeywords,
			HitCategoryIds: h.HitCategoryIDs,
		}
	}
	return connect.NewResponse(&pb.SearchSellerProspectsResponse{Data: out}), nil
}
