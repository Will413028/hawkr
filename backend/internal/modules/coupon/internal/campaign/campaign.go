// Package campaign 是活動的寫入路徑：建立（admin 平台活動、賣家活動）、發布、提前結束（D5）與賣家配額（D6）。
// 所有寫入都在呼叫方的 transaction 內（ctx 沒有 transaction 時自己開一個）。
package campaign

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Deps 是活動寫入路徑的依賴。
type Deps struct {
	DB    *db.DB
	NewID func() uuid.UUID
	Now   func() time.Time
}

// Service 是活動寫入路徑的 use case。
type Service struct{ d Deps }

// New 建立活動寫入路徑。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// CreateInput 是建立活動的輸入。
type CreateInput struct {
	model.CampaignSpec
	TargetType model.TargetType
	// StoreID 是適用店家；0 是全平台（只有 admin 的 ALL、DESIGNATED 可以）。賣家發的券一律是自己的店。
	StoreID int64
	// TotalLimit 是總發行量；0 是無上限（有上限時至少 1）。
	TotalLimit int32
	Issuer     model.Issuer
	// TargetBuyers 是 DESIGNATED 的資格名單；賣家的 DESIGNATED 活動不得為空。
	TargetBuyers []int64
}

// Create 建立活動。賣家（Issuer.StoreID）建立的活動直接 ACTIVE，並在鎖住配額列之後檢查同時進行的活動數（BD15）；
// admin 建立的是 DRAFT，經 Publish 才生效。COMPENSATION 只由 voucher.IssueCompensation 建立。
func (s *Service) Create(ctx context.Context, in CreateInput) (model.Campaign, error) {
	title, err := in.Validate()
	if err != nil {
		return model.Campaign{}, err
	}
	if !in.Issuer.Valid() {
		return model.Campaign{}, model.Invalid("Issuer", "issuer")
	}
	if in.TotalLimit < 0 {
		return model.Campaign{}, model.Invalid("TotalLimit", "total limit")
	}
	if in.StoreID < 0 || len(in.TargetBuyers) > model.MaxTargetBuyers {
		return model.Campaign{}, model.Invalid("StoreID", "store or target list")
	}
	seller := in.Issuer.StoreID > 0
	switch in.TargetType {
	case model.TargetAll, model.TargetDesignated:
	case model.TargetMember:
		if in.StoreID == 0 && !seller {
			return model.Campaign{}, model.Invalid("StoreID", "member campaign needs a store")
		}
	case model.TargetCompensation:
		return model.Campaign{}, model.Invalid("TargetType", "compensation campaigns are issued only through IssueCompensation")
	default:
		return model.Campaign{}, model.Invalid("TargetType", "target type")
	}
	if in.TargetType != model.TargetDesignated && len(in.TargetBuyers) > 0 {
		return model.Campaign{}, model.Invalid("TargetBuyers", "target list only for designated")
	}
	buyers, err := model.PositiveUnique("TargetBuyers", in.TargetBuyers)
	if err != nil {
		return model.Campaign{}, err
	}
	if seller && in.TargetType == model.TargetDesignated && len(buyers) == 0 {
		return model.Campaign{}, model.Invalid("TargetBuyers", "designated campaign needs buyers")
	}
	if seller && in.StoreID != 0 && in.StoreID != in.Issuer.StoreID {
		return model.Campaign{}, model.Invalid("StoreID", "seller campaign is for its own store")
	}

	now := s.d.Now()
	p := store.InsertCampaignParams{
		ID: s.d.NewID(), Title: title, DiscountType: string(in.DiscountType), ThresholdAmount: in.ThresholdAmount,
		TargetType: string(in.TargetType), StartAt: in.StartAt, EndAt: in.EndAt, Status: string(model.StatusDraft), Now: now,
	}
	if in.DiscountType == model.DiscountCash {
		p.DiscountAmount = &in.DiscountAmount
	} else {
		p.DiscountPercent = &in.DiscountPercent
		p.MaxDiscountAmount = &in.MaxDiscountAmount
	}
	if in.TotalLimit > 0 {
		p.TotalLimit = &in.TotalLimit
	}
	if seller {
		p.IssuerType = "SELLER"
		p.IssuerStoreID = &in.Issuer.StoreID
		p.StoreID = &in.Issuer.StoreID
		p.Status = string(model.StatusActive)
	} else {
		p.IssuerType = "ADMIN"
		p.IssuerAdminID = &in.Issuer.AdminID
		if in.StoreID > 0 {
			p.StoreID = &in.StoreID
		}
	}
	var out model.Campaign
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if seller {
			if err := s.checkQuota(ctx, q, in.Issuer.StoreID, now); err != nil {
				return err
			}
		}
		n, err := q.InsertCampaign(ctx, p)
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("coupon：建立活動沒有寫入任何列")
		}
		if len(buyers) > 0 {
			if _, err := q.InsertTargetAccounts(ctx, store.InsertTargetAccountsParams{CampaignID: p.ID, BuyerIds: buyers, Now: now}); err != nil {
				return err
			}
		}
		row, err := q.GetCampaign(ctx, p.ID)
		if err != nil {
			return err
		}
		out = rows.Campaign(row)
		return nil
	})
	return out, err
}

// checkQuota 鎖住店家的配額列（沒有列先建預設值），讓同一家店的配額檢查與插入串行。
func (s *Service) checkQuota(ctx context.Context, q *store.Queries, storeID int64, now time.Time) error {
	if err := q.InsertQuotaIfAbsent(ctx, store.InsertQuotaIfAbsentParams{StoreID: storeID, Now: now}); err != nil {
		return err
	}
	maxCount, err := q.LockQuota(ctx, storeID)
	if err != nil {
		return err
	}
	active, err := q.CountActiveSellerCampaigns(ctx, &storeID)
	if err != nil {
		return err
	}
	if active >= int64(maxCount) {
		return model.Precondition(model.ReasonQuotaExceeded)
	}
	return nil
}

// SetQuota 設定店家同時進行的活動數上限（admin，D6）。
func (s *Service) SetQuota(ctx context.Context, storeID int64, maxCount int32) error {
	if storeID <= 0 || maxCount < 0 {
		return model.Invalid("StoreID", "quota")
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		return s.q(ctx).UpsertQuota(ctx, store.UpsertQuotaParams{StoreID: storeID, MaxCampaignCount: maxCount, Now: s.d.Now()})
	})
}

// Publish 把 DRAFT 的活動發布成 ACTIVE。
func (s *Service) Publish(ctx context.Context, id uuid.UUID) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		n, err := q.PublishCampaign(ctx, store.PublishCampaignParams{ID: id, Now: s.d.Now()})
		if err != nil {
			return err
		}
		if n == 1 {
			return nil
		}
		if _, err := q.GetCampaign(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return model.NotFound(model.ReasonCampaignNotFound)
		} else if err != nil {
			return err
		}
		return model.Precondition(model.ReasonInvalidStatus)
	})
}

// EndInput 是提前結束活動的輸入。
type EndInput struct {
	CampaignID uuid.UUID
	// StoreID 不為 0 時只能結束這家店發的活動（賣家路徑）；別人的活動回 NotFound，不洩漏存在與否。
	StoreID int64
	// AdminID 與 Reason 只進稽核紀錄。
	AdminID int64
	Reason  string
}

// EndResult 是提前結束的結果。
type EndResult struct {
	// Voided 是這次作廢的 AVAILABLE 券數；重送（活動已結束）回 0。
	Voided int64
}

// End 提前結束活動（D5）：擋新的領取與鎖定、作廢 AVAILABLE 的券，LOCKED 的券照常核銷或釋放（BD8）。
// 先鎖活動列：與領券、發券排隊，結束之後進來的都看到 ENDED。已結束的活動重送是成功（Voided = 0）。
func (s *Service) End(ctx context.Context, in EndInput) (EndResult, error) {
	reason, err := model.NormalizeReason(in.Reason)
	if err != nil {
		return EndResult{}, err
	}
	var res EndResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := q.GetCampaignForUpdate(ctx, in.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return model.NotFound(model.ReasonCampaignNotFound)
		}
		if err != nil {
			return err
		}
		if in.StoreID > 0 && (row.IssuerStoreID == nil || *row.IssuerStoreID != in.StoreID) {
			return model.NotFound(model.ReasonCampaignNotFound)
		}
		if model.CampaignStatus(row.Status) == model.StatusEnded {
			return nil
		}
		if model.CampaignStatus(row.Status) != model.StatusActive {
			return model.Precondition(model.ReasonInvalidStatus)
		}
		now := s.d.Now()
		if n, err := q.EndCampaign(ctx, store.EndCampaignParams{ID: in.CampaignID, Now: rows.TimePtr(now)}); err != nil {
			return err
		} else if n != 1 {
			return errors.New("coupon：結束活動沒有更新任何列")
		}
		res.Voided, err = q.VoidAvailableVouchers(ctx, store.VoidAvailableVouchersParams{CampaignID: in.CampaignID, Now: rows.TimePtr(now)})
		if err != nil {
			return err
		}
		log := store.InsertLogParams{CampaignID: in.CampaignID, Action: "END_CAMPAIGN", Now: now}
		if in.AdminID > 0 {
			log.OperatorAdminID = &in.AdminID
		}
		if reason != "" {
			log.Reason = &reason
		}
		return q.InsertLog(ctx, log)
	})
	return res, err
}
