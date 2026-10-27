package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	bizcode "github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	"github.com/Will413028/hawkr/go-pkg/moderation"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"github.com/Will413028/hawkr/review-service/internal/domain"
	"go.uber.org/zap"
)

var (
	ErrAlreadyReviewed    = pkgerrors.NewConflict("already reviewed")
	ErrOrderNotFound      = pkgerrors.NewNotFound("order not found")
	ErrNotEligible        = pkgerrors.NewPreconditionFailed("order not eligible for review")
	ErrNotYourOrder       = pkgerrors.NewForbidden("order does not belong to buyer")
	ErrReviewExpired      = pkgerrors.NewPreconditionFailed("review window expired")
	ErrItemNotInOrder     = pkgerrors.NewInvalidInput("item not in order")
	ErrModerationRejected = pkgerrors.NewBusiness(bizcode.ModerationRejected, "內容不符社群守則")
	// #1174 賣家回覆
	ErrReviewNotFound     = pkgerrors.NewNotFound("review not found")
	ErrNotYourReview      = pkgerrors.NewForbidden("review does not belong to seller")
	ErrReviewNotReplyable = pkgerrors.NewConflict("review hidden or deleted, not replyable")
)

// ReviewUseCase 評論業務邏輯
type ReviewUseCase struct {
	repo           domain.ReviewRepository
	orderClient    OrderClient
	mediaClient    MediaClient
	buyerClient    BuyerClient
	sellerClient   SellerClient
	eventPublisher ReviewEventPublisher
	tsid           func() int64 // TSID 生成器，方便測試
	mediaBaseURL   string
	revalidate     *revalidate.Client
}

func NewReviewUseCase(repo domain.ReviewRepository, orderClient OrderClient, mediaClient MediaClient, buyerClient BuyerClient, sellerClient SellerClient, eventPublisher ReviewEventPublisher, cfg *config.Config, tsid func() int64, revalidateClient *revalidate.Client) *ReviewUseCase {
	return &ReviewUseCase{
		repo:           repo,
		orderClient:    orderClient,
		mediaClient:    mediaClient,
		buyerClient:    buyerClient,
		sellerClient:   sellerClient,
		eventPublisher: eventPublisher,
		mediaBaseURL:   cfg.MediaBaseURL,
		tsid:           tsid,
		revalidate:     revalidateClient,
	}
}

// revalidateProductAndSeller 對 review 影響的 product + seller cache 觸發 bust。
// 後端服務責任邊界規範 §9.3。
// 未接線 / 未配置 revalidate（本地開發無 WEB_REVALIDATE_*）時，tag 本來就會被丟棄，
// 故提前短路，省下 sellerCacheTags 的那一次 RPC。
func (uc *ReviewUseCase) revalidateProductAndSeller(ctx context.Context, productID, storeID int64) {
	if !uc.revalidate.Enabled() {
		return
	}
	tags := make([]string, 0, 3)
	if productID > 0 {
		tags = append(tags, revalidate.ProductTag(productID))
	}
	tags = append(tags, uc.sellerCacheTags(ctx, storeID)...)
	uc.revalidate.BustAsync(ctx, tags)
}

// sellerCacheTags 解出「這個賣家在網站快取裡的所有 tag」（issue #2033）。
//
// 網站是用「店鋪頁 URL 路徑片段」當 seller 快取鍵的（hawkr-website
// lib/api/cached.ts 的 getCachedSellerReviewStats / getCachedSellerReviews 直接把
// [id] 片段餵進 sellerTag(id)），而該片段有兩種識別碼空間，gateway
// resolveStorefrontSellerID 兩者都收：
//   - store_code：現行 canonical 片段。由 YY + 8 位 crypto/rand 鑄造
//     （seller-service domain.GenerateStoreCode），與 store_id **無函數關係**——
//     只有早期從已綁定 PayUni merchant_code backfill 的列碰巧相等。
//   - store_id：2026-07-23 前的片段，舊連結仍在用、仍被快取。
//
// review 手上只有 store_id，只發 store_id tag 因此完全打不到 ids 已分歧賣家的
// canonical 店鋪頁：評分與評論統計會 stale 到 CACHE_TTL.SELLER_PROFILE（1800s）才過期。
//
// fail-soft：store_code 解不出來時仍發 store_id tag——退回原本的覆蓋範圍，
// 遠勝於整個不發。
func (uc *ReviewUseCase) sellerCacheTags(ctx context.Context, storeID int64) []string {
	if uc.sellerClient == nil || storeID <= 0 {
		return revalidate.SellerTags(storeID)
	}
	raw, err := uc.sellerClient.ResolveStoreCode(ctx, storeID)
	if err != nil {
		zap.L().Warn("ResolveStoreCode failed, seller cache bust degrades to the store_id tag",
			zap.Int64("storeID", storeID), zap.Error(err))
		return revalidate.SellerTags(storeID)
	}
	// 空字串 = 賣家不存在該欄；"0" = store_code 補齊（backfill）前的殘留列。兩者都無從
	// 得知 canonical 店鋪頁的 tag，仍發其餘 tag，並留下線索指出該列資料不完整。
	storeCode, convErr := strconv.ParseInt(raw, 10, 64)
	if convErr != nil || storeCode <= 0 {
		zap.L().Warn("seller has no usable store_code, canonical storefront cache cannot be busted precisely",
			zap.Int64("storeID", storeID), zap.String("storeCode", raw), zap.Error(convErr))
		return revalidate.SellerTags(storeID)
	}
	return revalidate.SellerTags(storeID, storeCode)
}

// SubmitItemReviewInput 提交商品評論的輸入。
// 註：item image 不再由前端提供——後端從該訂單 item 的商品圖快照回填（49 §5.3 凍結語意），
// 故此處無 ItemImage 欄位；前端即使送 item_image_url 也一律忽略。
type SubmitItemReviewInput struct {
	OrderNo      string
	Content      string
	SkuName      string
	StagedTokens []string // staged token，由 review service 呼叫 FinalizeImages 轉正
	OrderItemID  int64
	BuyerID      int64
	ProductID    int64
	Price        int64
	Rating       int32
}

// SubmitItemReview 買家提交單筆商品評論
func (uc *ReviewUseCase) SubmitItemReview(ctx context.Context, in SubmitItemReviewInput) (*domain.Review, error) {
	// B-135: profanity dictionary
	if hit, _ := moderation.Check(in.Content); hit {
		return nil, ErrModerationRejected
	}
	// 1. 驗證訂單資格
	eligibility, items, err := uc.orderClient.GetOrderForReview(ctx, in.OrderNo)
	if err != nil {
		zap.L().Warn("GetOrderForReview failed", zap.String("orderNo", in.OrderNo), zap.Error(err))
		return nil, ErrOrderNotFound
	}
	if eligibility.Status != domain.OrderStatusCompleted {
		return nil, ErrNotEligible
	}
	if eligibility.BuyerID != in.BuyerID {
		return nil, ErrNotYourOrder
	}

	// 2. 確認訂單未超過評論期限
	deadline := eligibility.CompletedAt.Add(time.Duration(domain.ReviewWindowDays) * 24 * time.Hour)
	if time.Now().UTC().After(deadline) {
		return nil, ErrReviewExpired
	}

	// 3. 確認該 order_item_id 屬於此訂單，並取得權威 product_id 與商品圖快照
	var authorativeProductID int64
	var itemImageKey string
	found := false
	for _, item := range items {
		if item.OrderItemID == in.OrderItemID {
			found = true
			authorativeProductID = item.ProductID
			// item image 回填：來源 = 訂單 item 的商品圖快照（49 §5.3，前端不再送）。
			// 理想解：order 直接回 image_object_key（免剝殼）；相容 fallback：舊 order 只回組好的
			// CDN URL，剝殼還原成 objectKey。DB 只存 key，顯示 URL 由 Gateway 組（§8.1 橋接）。
			if item.ImageObjectKey != "" {
				itemImageKey = item.ImageObjectKey
			} else {
				itemImageKey = mediaurl.StripPrefix(uc.mediaBaseURL, item.ImageURL)
			}
			break
		}
	}
	if !found {
		return nil, ErrItemNotInOrder
	}

	// 4. 確認尚未評論過
	existing, err := uc.repo.GetByOrderItemID(ctx, in.OrderItemID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAlreadyReviewed
	}

	// 5. 預先產生 reviewID，轉正圖片時作為 refID（防止 GC 清除）
	reviewID := uc.tsid()

	// 評論附圖：轉正後只存 objectKey（49 §6.2；顯示 URL 由 Gateway 組，服務端不組對外 URL）。
	imageKeys := []string{}
	if len(in.StagedTokens) > 0 {
		tokens := in.StagedTokens
		if len(tokens) > domain.MaxImageCount {
			tokens = tokens[:domain.MaxImageCount]
		}
		refID := fmt.Sprintf("%d", reviewID)
		for _, token := range tokens {
			asset, err := uc.mediaClient.FinalizeAndLink(ctx, token, pkgmedia.RefTypeOrderReview, refID)
			if err != nil {
				zap.L().Error("FinalizeAndLink failed", zap.String("orderNo", in.OrderNo), zap.Error(err))
				return nil, fmt.Errorf("finalize review images: %w", err)
			}
			imageKeys = append(imageKeys, asset.ObjectKey)
		}
	}

	// 6. 存入評論（product_id 使用 order service 回傳的權威值，忽略客戶端傳入）
	r := &domain.Review{
		ID:           reviewID,
		OrderNo:      in.OrderNo,
		OrderItemID:  in.OrderItemID,
		BuyerID:      in.BuyerID,
		StoreID:      eligibility.StoreID,
		ProductID:    authorativeProductID,
		Rating:       in.Rating,
		Content:      in.Content,
		ImageKeys:    imageKeys,
		SkuName:      in.SkuName,
		ItemImageKey: itemImageKey,
		Price:        in.Price,
	}
	review, err := uc.repo.Insert(ctx, r)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyReviewed) {
			return nil, ErrAlreadyReviewed
		}
		return nil, err
	}

	// Publish REVIEW_CREATED event (best-effort: don't fail review creation if MQ publish fails)
	if uc.eventPublisher != nil {
		// 統計查詢失敗時「不發 event」而非帶 0 發出：consumer 端為 full-recompute 覆寫，
		// 帶 0 會把 seller / product 既有正確聚合清成 0（#1156 code review）。寧可跳過本次（最終一致
		// 靠下一筆評論或 ops backfill 自癒），也不發出假權威 0。
		stats, statsErr := uc.repo.GetStatsBySellerID(ctx, review.StoreID)
		pStats, pStatsErr := uc.repo.GetStatsByProductID(ctx, review.ProductID)
		if statsErr != nil || pStatsErr != nil {
			zap.L().Warn("review stats fetch failed, skip publishing review.created to avoid clobbering aggregates with zeros",
				zap.Int64("storeID", review.StoreID), zap.Int64("productID", review.ProductID),
				zap.Error(statsErr), zap.Error(pStatsErr))
		} else {
			// 取買家暱稱 / 頭像（best-effort，issue #686：賣家通知顯示具體買家而非「買家」）
			var buyerNickname, buyerAvatar string
			if uc.buyerClient != nil {
				// 用 Display 而非 Meta：這一刻買家必然還在（他剛寫了評論），兩者行為相同，
				// 但留兩支 client 方法只為了一個此處不成立的差別是技術債。
				// 本呼叫的結果是寫進 MQ payload 的**快照**，不是即時顯示——買家日後
				// 註銷不影響已送出的通知。
				metas, err := uc.buyerClient.BatchGetBuyerDisplayByBuyerIDs(ctx, []int64{review.BuyerID})
				if err != nil {
					zap.L().Warn("BatchGetBuyerDisplay failed, publishing review.created without buyer info",
						zap.Int64("buyerID", review.BuyerID), zap.Error(err))
				} else if meta, ok := metas[review.BuyerID]; ok {
					buyerNickname = meta.Nickname
					buyerAvatar = meta.AvatarURL
				}
			}

			// #2789：解析不出賣家 principal 時這個值是 0，不是 store_id。
			// event 照發（order / product / seller 三個 consumer 讀的是
			// store_id 與統計欄位，與這個欄位無關）；notification-service 的
			// extractUserID 對 <= 0 直接跳過整筆並記 WARN。填回 store_id 才是
			// 錯的——那會把 store 空間的 id 寫進收件人（account）欄位，誰也收不到
			// 而且完全無聲。預設值同理用 0，不用 review.StoreID。
			var notifySellerAccountID int64
			if uc.sellerClient != nil {
				notifySellerAccountID = uc.sellerClient.ResolvePrincipal(ctx, review.StoreID)
			}
			_ = uc.eventPublisher.PublishReviewCreated(ctx, review, notifySellerAccountID, stats.AvgRating, stats.TotalCount,
				pStats.AvgRating, pStats.TotalCount, buyerNickname, buyerAvatar)
		}
	}

	// 觸發 frontend SSR cache bust（後端服務責任邊界規範 §9.3）— rating / reviewCount 變了
	uc.revalidateProductAndSeller(ctx, review.ProductID, review.StoreID)

	return review, nil
}

// GetProductReviewsInput 查詢商品評論列表的輸入
type GetProductReviewsInput struct {
	ProductID  int64
	Rating     int32
	WithImages bool
	Page       int32
	PageSize   int32
}

// GetProductReviewsOutput 商品評論列表回傳
type GetProductReviewsOutput struct {
	Stats    *domain.ProductStats
	Reviews  []*domain.Review
	Total    int64
	Page     int32
	PageSize int32
}

// GetProductReviews 查詢商品公開評論列表
func (uc *ReviewUseCase) GetProductReviews(ctx context.Context, in GetProductReviewsInput) (*GetProductReviewsOutput, error) {
	page, pageSize, offset := pagination.NormalizePage(in.Page, in.PageSize, domain.DefaultPageSize, domain.MaxPageSize)
	in.Page, in.PageSize = page, pageSize

	reviews, err := uc.repo.ListByProductID(ctx, in.ProductID, in.Rating, in.WithImages, in.PageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := uc.repo.CountByProductID(ctx, in.ProductID, in.Rating, in.WithImages)
	if err != nil {
		return nil, err
	}
	stats, err := uc.repo.GetStatsByProductID(ctx, in.ProductID)
	if err != nil {
		return nil, err
	}

	// 批量取得買家暱稱與頭像
	if uc.buyerClient != nil && len(reviews) > 0 {
		buyerIDs := make([]int64, 0, len(reviews))
		for _, r := range reviews {
			buyerIDs = append(buyerIDs, r.BuyerID)
		}
		metas, err := uc.buyerClient.BatchGetBuyerDisplayByBuyerIDs(ctx, buyerIDs)
		if err != nil {
			zap.S().Warnf("GetProductReviews: BatchGetBuyerDisplay failed: %v", err)
		} else {
			for _, r := range reviews {
				if meta, ok := metas[r.BuyerID]; ok {
					r.BuyerName = meta.Nickname
					r.BuyerAvatar = meta.AvatarURL
					r.BuyerDeleted = meta.Deleted
				}
			}
		}
	}

	return &GetProductReviewsOutput{
		Reviews:  reviews,
		Stats:    stats,
		Total:    total,
		Page:     in.Page,
		PageSize: in.PageSize,
	}, nil
}

// GetReview 查詢單一評論詳情
func (uc *ReviewUseCase) GetReview(ctx context.Context, reviewID int64) (*domain.Review, error) {
	return uc.repo.GetByID(ctx, reviewID)
}

// ReplyReview 賣家回覆評論（#1174）。可編輯覆蓋；每次（含覆蓋）都重跑 moderation。
// 權限與可見性由 repo.ReplyReview 的 SQL WHERE 守門（擁有權 + 非 hidden/deleted）；
// 0 rows 時補查（不過濾 hidden 的 GetByID）分流 404 / 403 / 409（設計書 §2B.3）。
func (uc *ReviewUseCase) ReplyReview(ctx context.Context, reviewID, sellerID int64, content string) (*domain.Review, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, pkgerrors.NewInvalidInput("reply content is required")
	}
	if len(content) > domain.MaxContentLength {
		return nil, pkgerrors.NewInvalidInput(fmt.Sprintf("reply content exceeds %d characters", domain.MaxContentLength))
	}
	// 內容審核（含覆蓋每次都跑，防「先乾淨後改髒」繞審）
	if hit, _ := moderation.Check(content); hit {
		return nil, ErrModerationRejected
	}

	review, err := uc.repo.ReplyReview(ctx, reviewID, sellerID, content)
	if err != nil {
		return nil, err
	}
	if review == nil {
		// UPDATE 0 rows：以不過濾 hidden 的 GetByID 補查分流（唯一可回覆性判斷源為上面的 UPDATE WHERE）
		existing, gErr := uc.repo.GetByID(ctx, reviewID)
		if gErr != nil {
			return nil, gErr
		}
		if existing == nil {
			return nil, ErrReviewNotFound
		}
		if existing.StoreID != sellerID {
			return nil, ErrNotYourReview
		}
		// 存在且屬本賣家但 UPDATE 0 rows → 已 hidden / deleted，不可回覆
		return nil, ErrReviewNotReplyable
	}

	// 條件化 revalidate（設計書 §2B.6）：Phase D 暫定不 bust（reply 不改任何聚合）。
	// 待 Phase F 確認 web SSR 是否快取評論內文後再決定是否補 product tag bust。
	// reply 為單表附屬欄寫入，不發 MQ event、不通知買家（§2B.7）。

	return review, nil
}

// HideReview marks a review as hidden by platform moderation. Used by
// social-service AdminResolveReport for HIDE_TARGET on REVIEW (B-135).
// Idempotent: repo WHERE clause skips already-hidden rows.
func (uc *ReviewUseCase) HideReview(ctx context.Context, reviewID int64, reason string) error {
	review, _ := uc.repo.GetByID(ctx, reviewID)
	if err := uc.repo.HideReview(ctx, reviewID, reason); err != nil {
		return err
	}
	if review != nil {
		// 隱藏 review 改變可見集合 → 重算並發 review.hidden，讓 seller / product 服務覆寫聚合（#1156）。
		// 統計查詢已排除 hidden / deleted 列。查詢失敗時「不發 event」而非帶 0 發出：consumer 為
		// full-recompute 覆寫，帶 0 會清掉既有聚合（#1156 code review）。注意此處 0 可能是合法值
		// （隱藏最後一筆評論 → count=0），故以「查詢是否 error」而非「值是否為 0」決定是否跳過。
		if uc.eventPublisher != nil {
			s, sErr := uc.repo.GetStatsBySellerID(ctx, review.StoreID)
			p, pErr := uc.repo.GetStatsByProductID(ctx, review.ProductID)
			if sErr != nil || pErr != nil {
				zap.L().Warn("HideReview: stats fetch failed, skip publishing review.hidden to avoid clobbering aggregates",
					zap.Int64("storeID", review.StoreID), zap.Int64("productID", review.ProductID),
					zap.Error(sErr), zap.Error(pErr))
			} else {
				_ = uc.eventPublisher.PublishReviewHidden(ctx, reviewID, review.StoreID, review.ProductID,
					s.AvgRating, s.TotalCount, p.AvgRating, p.TotalCount)
			}
		}
		// 隱藏 review 影響 rating 統計 → bust（後端服務責任邊界規範 §9.3）
		uc.revalidateProductAndSeller(ctx, review.ProductID, review.StoreID)
	}
	return nil
}

// GetSellerReviewsInput 查詢賣家收到的評論
type GetSellerReviewsInput struct {
	SellerID int64
	Rating   int32
	Page     int32
	PageSize int32
}

// PaginatedReviews 分頁評論列表回傳
type PaginatedReviews struct {
	Reviews  []*domain.Review
	Total    int64
	Page     int32
	PageSize int32
}

// GetSellerReviews 賣家查詢自己收到的評論
func (uc *ReviewUseCase) GetSellerReviews(ctx context.Context, in GetSellerReviewsInput) (*PaginatedReviews, error) {
	page, pageSize, offset := pagination.NormalizePage(in.Page, in.PageSize, domain.DefaultPageSize, domain.MaxPageSize)
	in.Page, in.PageSize = page, pageSize

	reviews, err := uc.repo.ListBySellerID(ctx, in.SellerID, in.Rating, in.PageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := uc.repo.CountBySellerID(ctx, in.SellerID, in.Rating)
	if err != nil {
		return nil, err
	}

	// 批量取得買家暱稱與頭像
	if len(reviews) > 0 {
		buyerIDs := make([]int64, 0, len(reviews))
		for _, r := range reviews {
			buyerIDs = append(buyerIDs, r.BuyerID)
		}
		metas, err := uc.buyerClient.BatchGetBuyerDisplayByBuyerIDs(ctx, buyerIDs)
		if err != nil {
			// 取不到買家資訊不影響主流程，只記 log
			zap.S().Warnf("BatchGetBuyerDisplay failed: %v", err)
		} else {
			for _, r := range reviews {
				if meta, ok := metas[r.BuyerID]; ok {
					r.BuyerName = meta.Nickname
					r.BuyerAvatar = meta.AvatarURL
					r.BuyerDeleted = meta.Deleted
				}
			}
		}
	}

	return &PaginatedReviews{
		Reviews:  reviews,
		Total:    total,
		Page:     in.Page,
		PageSize: in.PageSize,
	}, nil
}

// GetSellerStats 賣家查詢評論統計
func (uc *ReviewUseCase) GetSellerStats(ctx context.Context, sellerID int64) (*domain.SellerStats, error) {
	return uc.repo.GetStatsBySellerID(ctx, sellerID)
}

// ListStoreRatingStats 批次查詢多家店的評論統計（內部呼叫，推薦賣家排行的重算用）。storeIDs 必須是 1 到
// domain.MaxStoreRatingStatsBatch 個正整數，重複的只算一次；只回有評論的店，依 store_id 升冪。
func (uc *ReviewUseCase) ListStoreRatingStats(ctx context.Context, storeIDs []int64) ([]*domain.StoreRatingStat, error) {
	if len(storeIDs) == 0 || len(storeIDs) > domain.MaxStoreRatingStatsBatch {
		return nil, pkgerrors.NewInvalidInput(fmt.Sprintf("store_ids 必須是 1 到 %d 個", domain.MaxStoreRatingStatsBatch))
	}
	seen := make(map[int64]struct{}, len(storeIDs))
	unique := make([]int64, 0, len(storeIDs))
	for _, id := range storeIDs {
		if id <= 0 {
			return nil, pkgerrors.NewInvalidInput("store_ids 必須是正整數")
		}
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	return uc.repo.ListStoreRatingStats(ctx, unique)
}

// ListMyReviewsInput 買家查詢自己的評論歷史
type ListMyReviewsInput struct {
	BuyerID  int64
	Page     int32
	PageSize int32
}

// ListMyReviews 買家查詢自己的評論歷史
func (uc *ReviewUseCase) ListMyReviews(ctx context.Context, in ListMyReviewsInput) (*PaginatedReviews, error) {
	page, pageSize, offset := pagination.NormalizePage(in.Page, in.PageSize, domain.DefaultPageSize, domain.MaxPageSize)
	in.Page, in.PageSize = page, pageSize

	reviews, err := uc.repo.ListByBuyerID(ctx, in.BuyerID, in.PageSize, offset)
	if err != nil {
		return nil, err
	}
	total, err := uc.repo.CountByBuyerID(ctx, in.BuyerID)
	if err != nil {
		return nil, err
	}
	return &PaginatedReviews{
		Reviews:  reviews,
		Total:    total,
		Page:     in.Page,
		PageSize: in.PageSize,
	}, nil
}
