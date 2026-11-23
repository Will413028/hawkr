// Package voucher 是券的寫入路徑：領券、admin 指定發放、系統補償，以及訂單面的 Lock、Redeem、Release 與 admin 作廢。
// 所有寫入都在呼叫方的 transaction 內（ctx 沒有 transaction 時自己開一個）；每一個狀態轉換都是條件式 UPDATE，
// 條件寫在 WHERE 裡，不做「先讀、再判斷、再寫」，稽核紀錄與狀態在同一個 transaction，寫入失敗整筆失敗。
package voucher

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/qualify"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Deps 是券寫入路徑的依賴。
type Deps struct {
	DB      *db.DB
	Qualify *qualify.Service
	NewID   func() uuid.UUID
	Now     func() time.Time
}

// Service 是券寫入路徑的 use case。
type Service struct{ d Deps }

// New 建立券寫入路徑。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

func uid(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func (s *Service) logOp(ctx context.Context, q *store.Queries, p store.InsertLogParams) error {
	p.Now = s.d.Now()
	return q.InsertLog(ctx, p)
}

// isOrderNoConflict 回報 err 是不是 vouchers_order_no_key 的唯一鍵衝突：同一筆訂單並發鎖兩張不同的券時，後到的撞上它。
// 衝突發生在 lockVoucher 的 savepoint 內，回滾到 savepoint 後呼叫方的 transaction 仍可使用。
func isOrderNoConflict(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "vouchers_order_no_key"
}

func validOrderNo(ctx context.Context, orderNo string) error {
	if n := utf8.RuneCountInString(orderNo); n < 1 || n > model.MaxOrderNoLen {
		return model.Invalid(ctx, "OrderNo", model.InvalidOrderNoInvalid)
	}
	return nil
}

// ClaimResult 是領券的結果；Created 為 false 表示買家已經有這個活動的券（重送，回既有的）。
type ClaimResult struct {
	Voucher model.Voucher
	Created bool
}

// Claim 讓買家領取活動的券：鎖住活動列、檢查資格（與 Lock 同一個判定）、以條件式 UPDATE 遞增發行數，再建立券。
// 買家已有這個活動的券時回既有的那張（冪等，不遞增）。超過總量回 ReasonIssueLimitReached。
func (s *Service) Claim(ctx context.Context, campaignID uuid.UUID, buyerID int64) (ClaimResult, error) {
	if buyerID <= 0 {
		return ClaimResult{}, model.Invalid(ctx, "BuyerID", model.InvalidBuyerIDInvalid)
	}
	var res ClaimResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		crow, err := q.GetCampaignForUpdate(ctx, campaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return model.NotFound(ctx, model.ReasonCampaignNotFound)
		}
		if err != nil {
			return err
		}
		c := rows.Campaign(crow)
		held, err := q.GetVoucherByCampaignBuyer(ctx, store.GetVoucherByCampaignBuyerParams{CampaignID: campaignID, BuyerID: buyerID})
		if err == nil {
			if model.VoucherStatus(held.Status) == model.VoucherVoided {
				return model.Precondition(ctx, model.ReasonVoucherUnavailable)
			}
			res = ClaimResult{Voucher: rows.Voucher(held)}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		now := s.d.Now()
		if err := s.d.Qualify.Check(ctx, c, buyerID, now); err != nil {
			return err
		}
		v, err := s.create(ctx, q, c.ID, buyerID, model.OriginClaim, nil, nil, now)
		if err != nil {
			return err
		}
		if err := s.logOp(ctx, q, store.InsertLogParams{CampaignID: c.ID, VoucherID: uid(v.ID), BuyerID: &buyerID, Action: "CLAIM"}); err != nil {
			return err
		}
		res = ClaimResult{Voucher: v, Created: true}
		return nil
	})
	return res, err
}

// create 遞增發行數並建立一張券；呼叫方已鎖住活動列。遞增的 WHERE 條件（ACTIVE、不超過總量）是唯一的上限守門：
// 回 0 列就是 ReasonIssueLimitReached。
func (s *Service) create(ctx context.Context, q *store.Queries, campaignID uuid.UUID, buyerID int64, origin model.Origin, srcType, srcID *string, now time.Time) (model.Voucher, error) {
	n, err := q.IncrementIssued(ctx, store.IncrementIssuedParams{ID: campaignID, Count: 1, Now: now})
	if err != nil {
		return model.Voucher{}, err
	}
	if n != 1 {
		return model.Voucher{}, model.Precondition(ctx, model.ReasonIssueLimitReached)
	}
	id := s.d.NewID()
	n, err = q.InsertVoucher(ctx, store.InsertVoucherParams{
		ID: id, CampaignID: campaignID, BuyerID: buyerID, Origin: string(origin), SourceType: srcType, SourceID: srcID, Now: now})
	if err != nil {
		return model.Voucher{}, err
	}
	if n != 1 {
		return model.Voucher{}, errors.New("coupon：建立券沒有寫入任何列（呼叫方應已鎖住活動列並確認買家沒有這個活動的券）")
	}
	row, err := q.GetVoucher(ctx, id)
	if err != nil {
		return model.Voucher{}, err
	}
	return rows.Voucher(row), nil
}

// IssueInput 是 admin 指定發放的輸入。
type IssueInput struct {
	CampaignID uuid.UUID
	BuyerIDs   []int64
	AdminID    int64
}

// IssueResult 是指定發放的結果。
type IssueResult struct {
	// Issued 是這次新建的券數；AlreadyHeld 是原本就持有這個活動的券的買家數。
	Issued      int
	AlreadyHeld int
}

// Issue 對 DESIGNATED 活動指定發放：把買家加進資格名單（之後 Lock 重驗資格時才過得了）並各建一張券。
// 全部或全不發：新建的券數會超過總量就整筆失敗（ReasonIssueLimitReached）。
func (s *Service) Issue(ctx context.Context, in IssueInput) (IssueResult, error) {
	if in.AdminID <= 0 {
		return IssueResult{}, model.Invalid(ctx, "AdminID", model.InvalidAdminIDInvalid)
	}
	if len(in.BuyerIDs) == 0 || len(in.BuyerIDs) > model.MaxIssueBatch {
		return IssueResult{}, model.Invalid(ctx, "BuyerIDs", model.InvalidBuyerCountInvalid)
	}
	buyers, err := model.PositiveUnique(ctx, "BuyerIDs", in.BuyerIDs)
	if err != nil {
		return IssueResult{}, err
	}
	var res IssueResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		crow, err := q.GetCampaignForUpdate(ctx, in.CampaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return model.NotFound(ctx, model.ReasonCampaignNotFound)
		}
		if err != nil {
			return err
		}
		c := rows.Campaign(crow)
		if c.TargetType != model.TargetDesignated {
			return model.Precondition(ctx, model.ReasonNotDesignated)
		}
		now := s.d.Now()
		if c.Status != model.StatusActive {
			return model.Precondition(ctx, model.ReasonCampaignNotActive)
		}
		if c.Expired(now) {
			return model.Precondition(ctx, model.ReasonOutsideWindow)
		}
		held, err := q.ListHeldBuyers(ctx, store.ListHeldBuyersParams{CampaignID: c.ID, BuyerIds: buyers})
		if err != nil {
			return err
		}
		isHeld := make(map[int64]struct{}, len(held))
		for _, b := range held {
			isHeld[b] = struct{}{}
		}
		fresh := make([]int64, 0, len(buyers))
		for _, b := range buyers {
			if _, ok := isHeld[b]; !ok {
				fresh = append(fresh, b)
			}
		}
		res = IssueResult{Issued: len(fresh), AlreadyHeld: len(held)}
		if _, err := q.InsertTargetAccounts(ctx, store.InsertTargetAccountsParams{CampaignID: c.ID, BuyerIds: buyers, Now: now}); err != nil {
			return err
		}
		if len(fresh) == 0 {
			return nil
		}
		n, err := q.IncrementIssued(ctx, store.IncrementIssuedParams{ID: c.ID, Count: int32(len(fresh)), Now: now}) //nolint:gosec // len(fresh) <= MaxIssueBatch
		if err != nil {
			return err
		}
		if n != 1 {
			return model.Precondition(ctx, model.ReasonIssueLimitReached)
		}
		for _, b := range fresh {
			id := s.d.NewID()
			if n, err := q.InsertVoucher(ctx, store.InsertVoucherParams{ID: id, CampaignID: c.ID, BuyerID: b, Origin: string(model.OriginGrant), Now: now}); err != nil {
				return err
			} else if n != 1 {
				return errors.New("coupon：指定發放沒有寫入券（呼叫方應已鎖住活動列並濾掉已持有的買家）")
			}
			buyer := b
			admin := in.AdminID
			if err := s.logOp(ctx, q, store.InsertLogParams{CampaignID: c.ID, VoucherID: uid(id), BuyerID: &buyer, Action: "ISSUE", OperatorAdminID: &admin}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return IssueResult{}, err
	}
	return res, nil
}

// CompensationInput 是系統補償券的輸入。(SourceType, SourceID) 是冪等鍵（D4）：同一個來源重送只建一張。
type CompensationInput struct {
	SourceType model.SourceType
	SourceID   string
	BuyerID    int64
	// StoreID 是補償券適用的店家，必填：缺值不能被放大成全平台券。
	StoreID int64
	Amount  int64
	// MinOrderAmount 是門檻；賣家發的必須大於 0，且至少是面額的 5 倍、與面額相差至少 100（沿用 Java 的補償券規則）。
	MinOrderAmount int64
	EndAt          time.Time
	Title          string
	Issuer         model.Issuer
}

// CompensationResult 是補償券的結果；Created 為 false 表示這個來源已經發過（重送，回原本的）。
type CompensationResult struct {
	Voucher  model.Voucher
	Campaign model.Campaign
	Created  bool
}

// IssueCompensation 建立一張補償券：一券一活動（總量 1），活動與券都以 (SourceType, SourceID) 去重。重送時回原本的券，
// 不比對其他參數；同一個來源給不同買家回 InvalidInput。
func (s *Service) IssueCompensation(ctx context.Context, in CompensationInput) (CompensationResult, error) {
	sourceID := in.SourceID
	if !in.SourceType.Valid() || utf8.RuneCountInString(sourceID) < 1 || utf8.RuneCountInString(sourceID) > model.MaxSourceIDLen {
		return CompensationResult{}, model.Invalid(ctx, "SourceID", model.InvalidSourceInvalid)
	}
	if in.BuyerID <= 0 || in.StoreID <= 0 || !in.Issuer.Valid() || (in.Issuer.StoreID > 0 && in.Issuer.StoreID != in.StoreID) {
		return CompensationResult{}, model.Invalid(ctx, "StoreID", model.InvalidBuyerStoreIssuerInvalid)
	}
	if in.Amount <= 0 || in.Amount > model.MaxOrderAmount/5 || in.MinOrderAmount < 0 || in.MinOrderAmount > model.MaxOrderAmount {
		return CompensationResult{}, model.Invalid(ctx, "Amount", model.InvalidAmountInvalid)
	}
	if in.MinOrderAmount > 0 && (in.MinOrderAmount < in.Amount*5 || in.MinOrderAmount-in.Amount < 100) {
		return CompensationResult{}, model.Invalid(ctx, "MinOrderAmount", model.InvalidCompensationRuleInvalid)
	}
	if in.Issuer.StoreID > 0 && in.MinOrderAmount == 0 {
		return CompensationResult{}, model.Invalid(ctx, "MinOrderAmount", model.InvalidSellerCompensationNeedsThreshold)
	}
	now := s.d.Now()
	spec := model.CampaignSpec{Title: in.Title, DiscountType: model.DiscountCash, DiscountAmount: in.Amount, ThresholdAmount: in.MinOrderAmount, StartAt: now, EndAt: in.EndAt}
	title, err := spec.Validate(ctx)
	if err != nil {
		return CompensationResult{}, err
	}
	if !in.EndAt.After(now) {
		return CompensationResult{}, model.Invalid(ctx, "EndAt", model.InvalidEndInPast)
	}

	var res CompensationResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		srcType := string(in.SourceType)
		campaignID := s.d.NewID()
		one := int32(1)
		p := store.InsertCampaignParams{
			ID: campaignID, Title: title, DiscountType: string(model.DiscountCash), DiscountAmount: &in.Amount, ThresholdAmount: in.MinOrderAmount,
			TargetType: string(model.TargetCompensation), StoreID: &in.StoreID, StartAt: now, EndAt: in.EndAt, TotalLimit: &one,
			Status: string(model.StatusActive), SourceType: &srcType, SourceID: &sourceID, Now: now,
		}
		if in.Issuer.StoreID > 0 {
			p.IssuerType, p.IssuerStoreID = "SELLER", &in.Issuer.StoreID
		} else {
			p.IssuerType, p.IssuerAdminID = "ADMIN", &in.Issuer.AdminID
		}
		n, err := q.InsertCampaign(ctx, p)
		if err != nil {
			return err
		}
		if n == 0 {
			return s.existingCompensation(ctx, q, in, &res)
		}
		if _, err := q.InsertTargetAccounts(ctx, store.InsertTargetAccountsParams{CampaignID: campaignID, BuyerIds: []int64{in.BuyerID}, Now: now}); err != nil {
			return err
		}
		v, err := s.create(ctx, q, campaignID, in.BuyerID, model.OriginCompensation, &srcType, &sourceID, now)
		if err != nil {
			return err
		}
		crow, err := q.GetCampaign(ctx, campaignID)
		if err != nil {
			return err
		}
		buyer := in.BuyerID
		log := store.InsertLogParams{CampaignID: campaignID, VoucherID: uid(v.ID), BuyerID: &buyer, Action: "ISSUE"}
		if in.Issuer.AdminID > 0 {
			log.OperatorAdminID = &in.Issuer.AdminID
		}
		if err := s.logOp(ctx, q, log); err != nil {
			return err
		}
		res = CompensationResult{Voucher: v, Campaign: rows.Campaign(crow), Created: true}
		return nil
	})
	return res, err
}

func (s *Service) existingCompensation(ctx context.Context, q *store.Queries, in CompensationInput, res *CompensationResult) error {
	srcType, srcID := string(in.SourceType), in.SourceID
	vrow, err := q.GetVoucherBySource(ctx, store.GetVoucherBySourceParams{SourceType: &srcType, SourceID: &srcID})
	if err != nil {
		return err
	}
	if vrow.BuyerID != in.BuyerID {
		return model.Invalid(ctx, "SourceID", model.InvalidSourceOtherBuyer)
	}
	crow, err := q.GetCampaign(ctx, vrow.CampaignID)
	if err != nil {
		return err
	}
	*res = CompensationResult{Voucher: rows.Voucher(vrow), Campaign: rows.Campaign(crow)}
	return nil
}

// LockInput 是鎖券與驗券的輸入：訂單面的 StoreID 與 OrderAmount 用來檢查店家範圍與門檻並算出折抵。
type LockInput struct {
	VoucherID   uuid.UUID
	BuyerID     int64
	OrderNo     string
	StoreID     int64
	OrderAmount int64
}

// LockResult 是鎖券與驗券的結果。Replayed 為 true 表示這張券本來就由同一筆訂單持有（冪等重送）。
type LockResult struct {
	Voucher  model.Voucher
	Discount int64
	Replayed bool
}

func heldBy(v store.CouponCouponVoucher, orderNo string) bool {
	st := model.VoucherStatus(v.Status)
	return (st == model.VoucherLocked || st == model.VoucherRedeemed) && v.OrderNo != nil && *v.OrderNo == orderNo
}

// Quote 驗券但不鎖：與 Lock 同一組檢查（資格、時間窗、店家範圍、門檻），回傳折抵金額。
func (s *Service) Quote(ctx context.Context, in LockInput) (LockResult, error) {
	if err := validOrderNo(ctx, in.OrderNo); err != nil {
		return LockResult{}, err
	}
	var res LockResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		vrow, c, err := s.load(ctx, q, in)
		if err != nil {
			return err
		}
		if !heldBy(vrow, in.OrderNo) {
			if err := availability(ctx, vrow); err != nil {
				return err
			}
			if err := s.d.Qualify.Check(ctx, c, in.BuyerID, s.d.Now()); err != nil {
				return err
			}
		}
		discount, err := qualify.Fit(ctx, c, in.StoreID, in.OrderAmount)
		if err != nil {
			return err
		}
		if !heldBy(vrow, in.OrderNo) {
			if err := qualify.RequireDiscount(ctx, discount); err != nil {
				return err
			}
		}
		res = LockResult{Voucher: rows.Voucher(vrow), Discount: discount, Replayed: heldBy(vrow, in.OrderNo)}
		return nil
	})
	return res, err
}

// availability 對已經不是 AVAILABLE 的券回對應的原因：被別的訂單鎖著、已核銷或已作廢。AVAILABLE 回 nil。
func availability(ctx context.Context, v store.CouponCouponVoucher) error {
	if st := model.VoucherStatus(v.Status); st == model.VoucherAvailable {
		return nil
	} else if st == model.VoucherLocked {
		return model.Precondition(ctx, model.ReasonVoucherLockedByOtherOrder)
	}
	return model.Precondition(ctx, model.ReasonVoucherUnavailable)
}

func (s *Service) load(ctx context.Context, q *store.Queries, in LockInput) (store.CouponCouponVoucher, model.Campaign, error) {
	if in.BuyerID <= 0 {
		return store.CouponCouponVoucher{}, model.Campaign{}, model.Invalid(ctx, "BuyerID", model.InvalidBuyerIDInvalid)
	}
	vrow, err := q.GetVoucher(ctx, in.VoucherID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && vrow.BuyerID != in.BuyerID) {
		return store.CouponCouponVoucher{}, model.Campaign{}, model.NotFound(ctx, model.ReasonVoucherNotFound)
	}
	if err != nil {
		return store.CouponCouponVoucher{}, model.Campaign{}, err
	}
	crow, err := q.GetCampaignForShare(ctx, vrow.CampaignID)
	if err != nil {
		return store.CouponCouponVoucher{}, model.Campaign{}, err
	}
	return vrow, rows.Campaign(crow), nil
}

// Lock 把一張券鎖給一筆訂單：一張券同時只被一筆訂單鎖，同一筆訂單重送是冪等成功，別的訂單回 ReasonVoucherLockedByOtherOrder。
// 鎖沒有到期時間：只有持鎖訂單的 Release 或 Redeem（或 admin 作廢）能解除（D3）。
// 每次鎖定都重驗資格、時間窗、店家範圍與門檻（BD1、BD2），與領券同一個判定；只有「已由同一筆訂單持有」的重送略過資格
// （活動結束後重送仍成功，D5：locked 照常完成）。
func (s *Service) Lock(ctx context.Context, in LockInput) (LockResult, error) {
	if err := validOrderNo(ctx, in.OrderNo); err != nil {
		return LockResult{}, err
	}
	var res LockResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) (err error) {
		res, err = s.lock(ctx, in)
		return err
	})
	return res, err
}

func (s *Service) lock(ctx context.Context, in LockInput) (LockResult, error) {
	q := s.q(ctx)
	vrow, c, err := s.load(ctx, q, in)
	if err != nil {
		return LockResult{}, err
	}
	if heldBy(vrow, in.OrderNo) {
		discount, err := qualify.Fit(ctx, c, in.StoreID, in.OrderAmount)
		if err != nil {
			return LockResult{}, err
		}
		return LockResult{Voucher: rows.Voucher(vrow), Discount: discount, Replayed: true}, nil
	}
	now := s.d.Now()
	if err := s.d.Qualify.Check(ctx, c, in.BuyerID, now); err != nil {
		return LockResult{}, err
	}
	discount, err := qualify.Fit(ctx, c, in.StoreID, in.OrderAmount)
	if err != nil {
		return LockResult{}, err
	}
	if err := qualify.RequireDiscount(ctx, discount); err != nil {
		return LockResult{}, err
	}
	if other, err := q.GetVoucherByOrderNo(ctx, &in.OrderNo); err == nil && other.ID != vrow.ID {
		return LockResult{}, model.Precondition(ctx, model.ReasonOrderHoldsAnotherVoucher)
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return LockResult{}, err
	}
	locked, err := s.lockVoucher(ctx, store.LockVoucherParams{ID: vrow.ID, BuyerID: in.BuyerID, OrderNo: &in.OrderNo, Now: rows.TimePtr(now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return s.lostLock(ctx, q, vrow.ID, in, discount)
	}
	if isOrderNoConflict(err) {
		return LockResult{}, model.Precondition(ctx, model.ReasonOrderHoldsAnotherVoucher)
	}
	if err != nil {
		return LockResult{}, err
	}
	if err := s.logOp(ctx, q, store.InsertLogParams{CampaignID: locked.CampaignID, VoucherID: uid(locked.ID), BuyerID: &in.BuyerID, Action: "LOCK", OrderNo: &in.OrderNo}); err != nil {
		return LockResult{}, err
	}
	fresh, err := q.GetVoucher(ctx, locked.ID)
	if err != nil {
		return LockResult{}, err
	}
	return LockResult{Voucher: rows.Voucher(fresh), Discount: discount}, nil
}

// lockVoucher 在 savepoint 內執行鎖定的 UPDATE：失敗（含 order_no 的唯一鍵衝突）就回滾到 savepoint，
// 呼叫方的 transaction 不會被 Postgres 中止，之後還能下 SQL 或 commit。
func (s *Service) lockVoucher(ctx context.Context, p store.LockVoucherParams) (store.LockVoucherRow, error) {
	tx, ok := db.TxFrom(ctx)
	if !ok {
		return store.New(s.d.DB.Conn(ctx)).LockVoucher(ctx, p)
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return store.LockVoucherRow{}, err
	}
	v, err := store.New(sp).LockVoucher(ctx, p)
	if err != nil {
		if rbErr := sp.Rollback(context.WithoutCancel(ctx)); rbErr != nil {
			return store.LockVoucherRow{}, errors.Join(err, rbErr)
		}
		return store.LockVoucherRow{}, err
	}
	if err := sp.Commit(ctx); err != nil {
		return store.LockVoucherRow{}, err
	}
	return v, nil
}

// lostLock 處理條件式 UPDATE 沒有更新任何列：重讀券，同一筆訂單已持有（並發的重送）是冪等成功，否則回券現在的狀態對應的原因。
func (s *Service) lostLock(ctx context.Context, q *store.Queries, id uuid.UUID, in LockInput, discount int64) (LockResult, error) {
	cur, err := q.GetVoucher(ctx, id)
	if err != nil {
		return LockResult{}, err
	}
	if heldBy(cur, in.OrderNo) {
		return LockResult{Voucher: rows.Voucher(cur), Discount: discount, Replayed: true}, nil
	}
	if err := availability(ctx, cur); err != nil {
		return LockResult{}, err
	}
	return LockResult{}, errors.New("coupon：券仍是 AVAILABLE 但條件式鎖定沒有更新任何列")
}

// RedeemResult 是核銷的結果；Replayed 為 true 表示這筆訂單的券早就核銷了（冪等重送）。
type RedeemResult struct {
	VoucherID uuid.UUID
	Replayed  bool
}

// Redeem 核銷持有這筆訂單的鎖定中的券。重送冪等；這筆訂單沒有持有任何券回 NotFound（ReasonNoVoucherForOrder）。
// 核銷不檢查活動狀態與時間：鎖定當下已驗過，結束活動或過期都不影響已鎖的券（D5、BD8）。
func (s *Service) Redeem(ctx context.Context, orderNo string) (RedeemResult, error) {
	if err := validOrderNo(ctx, orderNo); err != nil {
		return RedeemResult{}, err
	}
	var res RedeemResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		now := s.d.Now()
		row, err := q.RedeemVoucher(ctx, store.RedeemVoucherParams{OrderNo: &orderNo, Now: rows.TimePtr(now)})
		if errors.Is(err, pgx.ErrNoRows) {
			cur, err := q.GetVoucherByOrderNo(ctx, &orderNo)
			if errors.Is(err, pgx.ErrNoRows) {
				return model.NotFound(ctx, model.ReasonNoVoucherForOrder)
			}
			if err != nil {
				return err
			}
			if model.VoucherStatus(cur.Status) != model.VoucherRedeemed {
				return errors.New("coupon：持有訂單的券不是 LOCKED 也不是 REDEEMED")
			}
			res = RedeemResult{VoucherID: cur.ID, Replayed: true}
			return nil
		}
		if err != nil {
			return err
		}
		res = RedeemResult{VoucherID: row.ID}
		buyer := row.BuyerID
		return s.logOp(ctx, q, store.InsertLogParams{CampaignID: row.CampaignID, VoucherID: uid(row.ID), BuyerID: &buyer, Action: "REDEEM", OrderNo: &orderNo})
	})
	return res, err
}

// ReleaseResult 是釋放的結果；Released 為 false 表示這筆訂單沒有持有任何券（重送，或本來就沒用券）。
// Voided 為 true 表示活動已結束，券沒有回到可用而是作廢（D5）。
type ReleaseResult struct {
	Released bool
	Voided   bool
}

// Release 釋放持有這筆訂單的券（以 order_no 為鍵，D3 A）：鎖定中的回到可用；已核銷的（賣家取消已付款訂單）也回到可用。
// 活動已結束時改為作廢（D5：結束之後沒有 AVAILABLE）。重送冪等。讀活動列時共享鎖，與結束活動排隊，所以不會在結束
// 掃過之後才回到可用。
func (s *Service) Release(ctx context.Context, orderNo string) (ReleaseResult, error) {
	if err := validOrderNo(ctx, orderNo); err != nil {
		return ReleaseResult{}, err
	}
	var res ReleaseResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		cur, err := q.GetVoucherByOrderNo(ctx, &orderNo)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		crow, err := q.GetCampaignForShare(ctx, cur.CampaignID)
		if err != nil {
			return err
		}
		now := s.d.Now()
		p := store.ReleaseVoucherParams{ID: cur.ID, OrderNo: &orderNo, ToStatus: string(model.VoucherAvailable), Now: now}
		if model.CampaignStatus(crow.Status) == model.StatusEnded {
			p.ToStatus, p.VoidedAt = string(model.VoucherVoided), rows.TimePtr(now)
		}
		row, err := q.ReleaseVoucher(ctx, p)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		res = ReleaseResult{Released: true, Voided: p.ToStatus == string(model.VoucherVoided)}
		buyer := row.BuyerID
		return s.logOp(ctx, q, store.InsertLogParams{CampaignID: row.CampaignID, VoucherID: uid(row.ID), BuyerID: &buyer, Action: "RELEASE", OrderNo: &orderNo})
	})
	return res, err
}

// VoidInput 是 admin 作廢單張券的輸入。
type VoidInput struct {
	VoucherID uuid.UUID
	AdminID   int64
	Reason    string
}

// Void 作廢單張券（admin，D6）：AVAILABLE 或 LOCKED（Release 漏送而卡住的券）；已核銷的與已作廢的回 ReasonInvalidStatus。
func (s *Service) Void(ctx context.Context, in VoidInput) error {
	if in.AdminID <= 0 {
		return model.Invalid(ctx, "AdminID", model.InvalidAdminIDInvalid)
	}
	reason, err := model.NormalizeReason(ctx, in.Reason)
	if err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		now := s.d.Now()
		row, err := q.VoidVoucher(ctx, store.VoidVoucherParams{ID: in.VoucherID, Now: rows.TimePtr(now)})
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err := q.GetVoucher(ctx, in.VoucherID); errors.Is(err, pgx.ErrNoRows) {
				return model.NotFound(ctx, model.ReasonVoucherNotFound)
			} else if err != nil {
				return err
			}
			return model.Precondition(ctx, model.ReasonInvalidStatus)
		}
		if err != nil {
			return err
		}
		buyer, admin := row.BuyerID, in.AdminID
		log := store.InsertLogParams{CampaignID: row.CampaignID, VoucherID: uid(row.ID), BuyerID: &buyer, Action: "VOID", OperatorAdminID: &admin}
		if reason != "" {
			log.Reason = &reason
		}
		return s.logOp(ctx, q, log)
	})
}
