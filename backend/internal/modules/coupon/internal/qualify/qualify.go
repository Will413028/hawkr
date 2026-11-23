// Package qualify 是「這位買家現在能不能領／用這個活動」的唯一判定：領券與鎖券都呼叫 Check（BD1、BD2、BD11），
// 不重複實作。判定包含活動狀態、時間窗與資格；訂單面的條件（店家範圍、門檻）在 Fit。
package qualify

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Stores 是「店家目前是否停權」的 port：賣家發行的活動在發行店家停權期間不能領、不能鎖（D8 修訂）。
// 讀取失敗就回錯誤，不降級（BD28）。
type Stores interface {
	StoreSuspended(ctx context.Context, storeID int64) (bool, error)
}

// Members 是「買家在該店有消費紀錄」的 port（MEMBER 資格用）。
type Members interface {
	IsMember(ctx context.Context, buyerID, storeID int64) (bool, error)
}

// Deps 是判定的依賴。
type Deps struct {
	DB      *db.DB
	Stores  Stores
	Members Members
}

// Service 是資格判定。
type Service struct{ d Deps }

// New 建立資格判定。
func New(d Deps) *Service { return &Service{d: d} }

// Check 回報 buyerID 在 now 能不能領或使用 c：活動必須是 ACTIVE、now 在 [StartAt, EndAt]，而且符合 TargetType。
// 不合格回 FailedPrecondition 與對應的業務原因；port 讀取失敗原樣回傳。
func (s *Service) Check(ctx context.Context, c model.Campaign, buyerID int64, now time.Time) error {
	if c.Status != model.StatusActive {
		return model.Precondition(ctx, model.ReasonCampaignNotActive)
	}
	if now.Before(c.StartAt) || c.Expired(now) {
		return model.Precondition(ctx, model.ReasonOutsideWindow)
	}
	if c.IssuerStoreID != nil {
		suspended, err := s.d.Stores.StoreSuspended(ctx, *c.IssuerStoreID)
		if err != nil {
			return fmt.Errorf("coupon：讀店家 %d 的停權狀態：%w", *c.IssuerStoreID, err)
		}
		if suspended {
			return model.Precondition(ctx, model.ReasonStoreSuspended)
		}
	}
	ok, err := s.targets(ctx, c, buyerID)
	if err != nil {
		return err
	}
	if !ok {
		return model.Precondition(ctx, model.ReasonNotEligible)
	}
	return nil
}

func (s *Service) targets(ctx context.Context, c model.Campaign, buyerID int64) (bool, error) {
	switch c.TargetType {
	case model.TargetAll:
		return true, nil
	case model.TargetMember:
		if c.StoreID == nil {
			return false, nil
		}
		ok, err := s.d.Members.IsMember(ctx, buyerID, *c.StoreID)
		if err != nil {
			return false, fmt.Errorf("coupon：讀買家 %d 在店家 %d 的消費紀錄：%w", buyerID, *c.StoreID, err)
		}
		return ok, nil
	case model.TargetDesignated, model.TargetCompensation:
		return store.New(s.d.DB.Conn(ctx)).HasTargetAccount(ctx, store.HasTargetAccountParams{CampaignID: c.ID, BuyerID: buyerID})
	default:
		return false, nil
	}
}

// Fit 檢查訂單面的條件並回傳折抵金額：店家在活動的適用範圍內、訂單金額達門檻（折抵可能是 0，由 RequireDiscount 在新鎖定的路徑拒絕）。orderAmount 必須是正數且不超過 MaxOrderAmount。
func Fit(ctx context.Context, c model.Campaign, storeID, orderAmount int64) (int64, error) {
	if storeID <= 0 {
		return 0, model.Invalid(ctx, "StoreID", model.InvalidStoreInvalid)
	}
	if orderAmount <= 0 || orderAmount > model.MaxOrderAmount {
		return 0, model.Invalid(ctx, "OrderAmount", model.InvalidOrderAmountInvalid)
	}
	if c.StoreID != nil && *c.StoreID != storeID {
		return 0, model.Precondition(ctx, model.ReasonNotApplicableToStore)
	}
	if orderAmount < c.ThresholdAmount {
		return 0, model.Precondition(ctx, model.ReasonBelowThreshold)
	}
	return model.Discount(c, orderAmount), nil
}

// RequireDiscount 對新鎖定的路徑拒絕折抵為 0 的結果（百分比折抵可能算出 0 元，不消耗券）；已由同一筆訂單持有的重送不經過它。
func RequireDiscount(ctx context.Context, discount int64) error {
	if discount <= 0 {
		return model.Precondition(ctx, model.ReasonNoDiscount)
	}
	return nil
}
