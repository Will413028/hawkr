// Package deletion 管理買家身分的關閉與個資清理。
package deletion

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Deps 是刪除流程需要的資料庫、事件與背景工作。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Now    func() time.Time
}

// Service 執行買家關閉與個資清理。
type Service struct {
	deps Deps
}

// New 建立刪除服務；Now 由組裝點提供。
func New(deps Deps) *Service {
	return &Service{deps: deps}
}

// Lock 在呼叫方的 transaction 內鎖住買家列，不改任何東西；買家不存在回 NotFound。前置條件同 BeginDeletion。
func (service *Service) Lock(ctx context.Context, buyerID int64) error {
	if err := model.Positive(buyerID); err != nil {
		return err
	}
	_, err := store.New(service.deps.DB.Conn(ctx)).LockBuyer(ctx, buyerID)
	return model.Error(err)
}

// BeginDeletion 將存活的 stint 標記為待刪，並在同一個 transaction 發布 BuyerDeletionRequested；重複呼叫不增加版本、不重複發布。
//
// 前置條件：只由 accountdeletion 在它的 transaction 內、已以 identity.LockAccount 鎖住帳號列之後呼叫（鎖順序：帳號列先於
// 買家列）。本模組不自己鎖帳號，也沒有第二條關閉買家的路徑。
func (service *Service) BeginDeletion(ctx context.Context, buyerID int64) error {
	if err := model.Positive(buyerID); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(service.deps.DB.Conn(ctx))
		current, err := q.LockBuyer(ctx, buyerID)
		if err != nil {
			return model.Error(err)
		}
		if current.Status != string(model.Active) {
			return nil
		}
		now := service.deps.Now().UTC()
		if err := q.Pending(ctx, store.PendingParams{BuyerID: buyerID, UpdatedAt: now}); err != nil {
			return err
		}
		return events.Publish(ctx, service.deps.Events, model.BuyerDeletionRequested{
			EventID:    uuid.NewV7(),
			OccurredAt: now,
			BuyerID:    current.BuyerID,
			AccountID:  current.AccountID,
		})
	})
}

// CloseForDeletion 關閉存活的 stint；已關閉時成功（重送安全）。呼叫方負責確認未結訂單並在同一個 transaction 刪除帳號。
//
// 前置條件同 BeginDeletion：只由 accountdeletion 在它的 transaction 內、已鎖住帳號列之後呼叫；關閉買家只有這一條路徑
// （帳號活著 ⇔ 買家活著，ADR 2026-09-30-account-created-provisions-buyer）。
func (service *Service) CloseForDeletion(ctx context.Context, buyerID int64) error {
	if err := model.Positive(buyerID); err != nil {
		return err
	}
	return service.deps.DB.InTx(ctx, func(ctx context.Context) error {
		queries := store.New(service.deps.DB.Conn(ctx))
		current, err := queries.LockBuyer(ctx, buyerID)
		if err != nil {
			return model.Error(err)
		}
		return service.close(ctx, queries, current)
	})
}

func (service *Service) close(ctx context.Context, queries *store.Queries, current store.BuyerBuyer) error {
	if current.Status == string(model.Closed) {
		return nil
	}
	now := service.deps.Now().UTC()
	if err := queries.Close(ctx, store.CloseParams{BuyerID: current.BuyerID, DeletedAt: &now}); err != nil {
		return err
	}
	if err := queries.DeleteAddresses(ctx, current.BuyerID); err != nil {
		return err
	}
	productIDs, err := queries.ClearFavorite(ctx, current.BuyerID)
	if err != nil {
		return err
	}
	for _, productID := range productIDs {
		if err := events.Publish(ctx, service.deps.Events, model.FavoriteRemoved{
			EventID:    uuid.NewV7(),
			OccurredAt: now,
			BuyerID:    current.BuyerID,
			ProductID:  productID,
		}); err != nil {
			return err
		}
	}
	if _, err := queries.ClearFollow(ctx, current.BuyerID); err != nil {
		return err
	}
	if err := queries.ClearBlockedStores(ctx, current.BuyerID); err != nil {
		return err
	}
	if err := queries.ClearBlockedBuyers(ctx, current.BuyerID); err != nil {
		return err
	}
	if _, err := queries.ClearVisit(ctx, current.BuyerID); err != nil {
		return err
	}
	if err := queries.CancelRestock(ctx, current.BuyerID); err != nil {
		return err
	}
	if err := service.retireAvatar(ctx, queries, current, now); err != nil {
		return err
	}
	return events.Publish(ctx, service.deps.Events, model.BuyerDeleted{
		EventID:    uuid.NewV7(),
		OccurredAt: now,
		BuyerID:    current.BuyerID,
		AccountID:  current.AccountID,
	})
}

// retireAvatar 關閉時把目前的頭像記成退場（終態：關閉的買家沒有目前的資產），引用由同步 job 解除。
func (service *Service) retireAvatar(ctx context.Context, queries *store.Queries, current store.BuyerBuyer, now time.Time) error {
	var retired []string
	if current.AvatarAssetID != nil {
		retired = append(retired, *current.AvatarAssetID)
	}
	return mediaref.Changed(ctx, queries, service.deps.Jobs, now, mediaref.AvatarRef(current.BuyerID), retired...)
}

// CleanupPII 清理已關閉超過 30 天的 stint，按 buyer_id 分批處理至沒有可清理資料。
func (service *Service) CleanupPII(ctx context.Context) error {
	cutoff := service.deps.Now().UTC().AddDate(0, 0, -30)
	for {
		count := 0
		err := service.deps.DB.InTx(ctx, func(ctx context.Context) error {
			queries := store.New(service.deps.DB.Conn(ctx))
			buyerIDs, err := queries.CleanupBatch(ctx, store.CleanupBatchParams{DeletedAt: &cutoff, Limit: 100})
			if err != nil {
				return err
			}
			count = len(buyerIDs)
			now := service.deps.Now().UTC()
			for _, buyerID := range buyerIDs {
				if err := queries.ClearPII(ctx, store.ClearPIIParams{BuyerID: buyerID, PiiClearedAt: &now}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if count < 100 {
			return nil
		}
	}
}
