package worker

import (
	"context"
	"fmt"
	"strconv"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/social-service/internal/domain"
)

// mediaReleaser 是這條路徑需要的 media 能力子集：整店退場時批次釋放貼文圖片的引用。
// 刻意只有 Release——帳號終止只縮不長，收窄成一支方法之後，任何在這裡呼叫 Link 的
// 改動都會編不過，而不是靠註解或測試的 panic 攔。
type mediaReleaser interface {
	ReleaseReferences(ctx context.Context, refType string, refIDs, keepAssetIDs []string) error
}

// releaseRefBatchSize 是一次 ReleaseReferences 帶的 ref_id 上限。
//
// pkg/media.ReleaseReferences 的 deadline 是 5s 且**每次呼叫固定**，不隨筆數放大，
// 所以「一次送完」在大店上是保證逾時，不是機率問題。200 是與其他批次呼叫端同一個
// 量級的保守值：小到單批遠在 5s 內完成，大到常見規模的店只需個位數次往返。
const releaseRefBatchSize = 200

// newSellerDeletedHandler 處理帳號終止：軟刪該店貼文、清該店身分的留言讚（貼文讚永遠是買家身分，
// 不屬於店家），並釋放該店貼文的全部媒體引用。
//
// 釋放為什麼非做在這裡不可：寫入路徑的 ③ 只在使用者自己換圖／清圖時送出（見
// PostUseCase.UpdatePost），軟刪列依契約刻意保留引用不釋放，所以沒有任何其他路徑
// 會替帳號終止補做這一步。Will 2026-09-05：賣家刪除後貼文圖片不用留。
//
// 三個操作任一失敗都直接 return err，交給既有的 retry／DLQ 兜底，不做 best-effort
// 吞掉、也不另寫補償任務（《後端服務責任邊界規範》§二）。前提是整支重跑無害：三個
// 操作各自冪等，而且釋放對象來自 ListPostIDsBySellerID（述詞不看 deleted_at），重投
// 時回的是同一個集合——這正是不能改用 SoftDeleteBySellerID 的 RETURNING id 的理由。
func newSellerDeletedHandler(postRepo domain.PostRepository, commentRepo domain.CommentRepository, mediaRef mediaReleaser, logger *zap.Logger) func(ctx context.Context, routingKey string, body []byte) error {
	return mq.BindTypedHandler(logger, func(ctx context.Context, routingKey string, payload *mq.SellerDeletedEvent) error {
		if payload.StoreID == 0 {
			logger.Warn("seller.deleted event missing store_id")
			return fmt.Errorf("seller.deleted event missing store_id")
		}

		logger.Info("processing seller.deleted event", zap.Int64("account_id", payload.AccountID), zap.Int64("store_id", payload.StoreID))

		// 先讀 id 再軟刪：這樣送去釋放的每一列，在釋放的當下都已經是軟刪狀態
		// （本來就已軟刪的，或被下面這句軟刪掉的）。讀完之後才建立的貼文不在這個
		// 集合裡，最多落得「已軟刪、引用還在」——那是安全方向（引用還在＝圖還在），
		// 也正是軟刪列的常態。
		postIDs, err := postRepo.ListPostIDsBySellerID(ctx, payload.StoreID)
		if err != nil {
			logger.Error("failed to list post ids for seller", zap.Int64("store_id", payload.StoreID), zap.Error(err))
			return err
		}

		if err := postRepo.SoftDeleteBySellerID(ctx, payload.StoreID); err != nil {
			logger.Error("failed to soft-delete posts for seller", zap.Int64("store_id", payload.StoreID), zap.Error(err))
			return err
		}

		if err := commentRepo.DeleteLikesByActor(ctx, domain.ActorKindStore, payload.StoreID); err != nil {
			logger.Error("failed to delete comment likes for seller", zap.Int64("store_id", payload.StoreID), zap.Error(err))
			return err
		}

		// 排在軟刪之後。反過來（先釋放再軟刪）中途失敗會留下「業務列還活著、引用卻
		// 已被釋放」，那種資產會被 media GC 連同物件實體刪除；這個方向中途失敗只會
		// 留下「已軟刪但引用還在」，重投會補完。
		if len(postIDs) > 0 {
			refIDs := make([]string, 0, len(postIDs))
			for _, id := range postIDs {
				refIDs = append(refIDs, strconv.FormatInt(id, 10))
			}
			// 分批，不是一次把整店貼文塞進同一個 RPC。pkg/media.ReleaseReferences 對
			// **每一次呼叫**套一個固定 5s deadline（releaseTimeout），與 refIDs 筆數無關；
			// 大店一次送幾千筆必定逾時，而這支 handler 是軟刪列釋放引用的**唯一**路徑
			// （軟刪列不在 reconciler 候選集合內），一路重試進 DLQ 之後沒有任何背景工作
			// 會補做，也沒有任何 metric 會叫——引用就永久留著。
			//
			// 冪等性不受影響：ListPostIDsBySellerID 的述詞不看 deleted_at，重投會重新
			// 導出同一個集合，已釋放過的批次再釋放一次是 no-op。所以「前幾批成功、某一批
			// 失敗 → return err → 重投整支」是安全的，不需要記錄進度。
			//
			// 不要「簡化」回單次呼叫。
			for start := 0; start < len(refIDs); start += releaseRefBatchSize {
				end := start + releaseRefBatchSize
				if end > len(refIDs) {
					end = len(refIDs)
				}
				// keep 傳 nil＝釋放全部，正是 media proto 記載的帳號／實體刪除語意。
				if err := mediaRef.ReleaseReferences(ctx, media.RefTypePost, refIDs[start:end], nil); err != nil {
					logger.Error("failed to release post media references for seller",
						zap.Int64("store_id", payload.StoreID),
						zap.Int("posts", len(refIDs)),
						zap.Int("batch_start", start),
						zap.Int("batch_size", end-start),
						zap.Error(err))
					return err
				}
			}
		}

		logger.Info("soft-deleted posts and likes for deleted seller",
			zap.Int64("store_id", payload.StoreID), zap.Int("released_post_refs", len(postIDs)))
		return nil
	})
}
