package admin

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

func storeNotFound(ctx context.Context, err error) error {
	if err != nil && apperr.KindOf(err) == apperr.KindNotFound {
		return apperr.NotFound("%s", text.AdminQuotaStoreNotFound(loc(ctx)))
	}
	return err
}

func quotaStale(ctx context.Context) *apperr.Error {
	return apperr.VersionConflict("%s", text.AdminQuotaStale(loc(ctx)))
}

func (s *Service) toQuota(ctx context.Context, r store.ImageaiQuota) (model.AdminQuota, error) {
	used, err := s.d.Used(ctx, r.StoreID)
	if err != nil {
		return model.AdminQuota{}, err
	}
	return model.AdminQuota{StoreID: r.StoreID, WeeklyLimit: r.WeeklyLimit, WeeklyUsage: used, Memo: r.Memo, Custom: true, Version: r.Version, UpdatedAt: r.UpdatedAt}, nil
}

// GetQuota 回傳一家店的每週額度與本週使用量；沒有個別設定時回預設額度（Custom 為 false）。店家不存在回 NotFound（A9-D41）。
func (s *Service) GetQuota(ctx context.Context, storeID int64) (model.AdminQuota, error) {
	if err := storeNotFound(ctx, s.d.Stores.Exists(ctx, storeID)); err != nil {
		return model.AdminQuota{}, err
	}
	row, err := s.d.Q(ctx).GetQuota(ctx, storeID)
	if notFound(err) {
		used, err := s.d.Used(ctx, storeID)
		if err != nil {
			return model.AdminQuota{}, err
		}
		return model.AdminQuota{StoreID: storeID, WeeklyLimit: model.DefaultWeeklyLimit, WeeklyUsage: used}, nil
	}
	if err != nil {
		return model.AdminQuota{}, fmt.Errorf("admin：讀店家 %d 的額度：%w", storeID, err)
	}
	return s.toQuota(ctx, row)
}

// UpdateQuota 為一家店個別設定每週額度（0 到 MaxWeeklyLimit）。沒有個別設定時不帶版本的呼叫建立它；已經有個別設定卻沒帶版本回 AlreadyExists
// （VERSION_CONFLICT）；版本與目前的不符、或帶了版本但個別設定已被刪除回 VersionConflict。店家不存在或已退場回 NotFound（A9-D41）。
func (s *Service) UpdateQuota(ctx context.Context, in model.UpdateQuotaInput) (model.AdminQuota, error) {
	l := loc(ctx)
	if in.WeeklyLimit < 0 || in.WeeklyLimit > model.MaxWeeklyLimit {
		return model.AdminQuota{}, apperr.InvalidInputOn([]string{"WeeklyLimit"}, "%s", text.AdminQuotaLimitRange(l, text.AdminQuotaLimitRangeArgs{Max: int(model.MaxWeeklyLimit)}))
	}
	memo := trimtext.Trim(in.Memo)
	var row store.ImageaiQuota
	err := s.d.Tx(ctx, func(ctx context.Context) error {
		now := s.d.Now().UTC()
		if err := storeNotFound(ctx, s.d.Stores.LockNotRetired(ctx, in.StoreID)); err != nil {
			return err
		}
		q := s.d.Q(ctx)
		_, err := q.LockQuota(ctx, in.StoreID)
		created := false
		switch {
		case notFound(err):
			if in.Version != 0 {
				return quotaStale(ctx)
			}
			row, err = q.InsertQuota(ctx, store.InsertQuotaParams{StoreID: in.StoreID, WeeklyLimit: in.WeeklyLimit, Memo: memo, Now: now})
			if notFound(err) {
				return apperr.AlreadyExists("%s", text.AdminQuotaExists(l)).WithReason(apperr.ReasonVersionConflict)
			}
			if err != nil {
				return fmt.Errorf("admin：建立店家 %d 的額度：%w", in.StoreID, err)
			}
			created = true
		case err != nil:
			return fmt.Errorf("admin：鎖定店家 %d 的額度：%w", in.StoreID, err)
		case in.Version == 0:
			return apperr.AlreadyExists("%s", text.AdminQuotaExists(l)).WithReason(apperr.ReasonVersionConflict)
		default:
			row, err = q.UpdateQuota(ctx, store.UpdateQuotaParams{StoreID: in.StoreID, WeeklyLimit: in.WeeklyLimit, Memo: memo, Now: now, ExpectedVersion: in.Version})
			if notFound(err) {
				return quotaStale(ctx)
			}
			if err != nil {
				return fmt.Errorf("admin：修改店家 %d 的額度：%w", in.StoreID, err)
			}
		}
		return s.record(ctx, in.Actor, quotaTarget, fmt.Sprint(in.StoreID), map[string]any{"weekly_limit": row.WeeklyLimit, "created": created, "version": row.Version})
	})
	if err != nil {
		return model.AdminQuota{}, err
	}
	return s.toQuota(ctx, row)
}

// DeleteQuota 刪除一家店的個別設定，回到預設額度。version 為 0 表示不檢查版本；不為 0 時必須與目前的相符。店家不存在（A9-D41）或沒有個別設定回 NotFound。
func (s *Service) DeleteQuota(ctx context.Context, actor model.Actor, storeID int64, version int64) error {
	return s.d.Tx(ctx, func(ctx context.Context) error {
		if err := storeNotFound(ctx, s.d.Stores.LockNotRetired(ctx, storeID)); err != nil {
			return err
		}
		q := s.d.Q(ctx)
		old, err := q.LockQuota(ctx, storeID)
		if notFound(err) {
			return apperr.NotFound("%s", text.AdminQuotaNotFound(loc(ctx)))
		}
		if err != nil {
			return fmt.Errorf("admin：鎖定店家 %d 的額度：%w", storeID, err)
		}
		if version != 0 && old.Version != version {
			return quotaStale(ctx)
		}
		if _, err := q.DeleteQuota(ctx, storeID); err != nil {
			return fmt.Errorf("admin：刪除店家 %d 的額度：%w", storeID, err)
		}
		return s.record(ctx, actor, quotaTarget, fmt.Sprint(storeID), map[string]any{"weekly_limit": old.WeeklyLimit})
	})
}
