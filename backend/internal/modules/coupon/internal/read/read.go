// Package read 是 coupon 的讀取路徑：買家的券與可領的活動、賣家的活動列表與明細。排序與分頁在 SQL 內完成（keyset，created_at 與 id 由新到舊）；
// 能不能領、能不能用只問 qualify（領券與鎖券同一個判定），這裡不另寫第二份規則（BD1、BD2、BD11）。
package read

import (
	"context"
	"errors"
	"math"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/qualify"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/rows"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	// DefaultPageSize 與 MaxPageSize 是列表一頁的預設與上限。
	DefaultPageSize = 20
	MaxPageSize     = 100
	// scanBatch 是需要在 Go 端以 qualify 過濾時，每一輪向 SQL 取的筆數。
	scanBatch = 100
)

// Deps 是讀取路徑的依賴。
type Deps struct {
	DB      *db.DB
	Qualify *qualify.Service
	Now     func() time.Time
}

// Service 是讀取路徑。
type Service struct{ d Deps }

// New 建立讀取路徑。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// Cursor 是列表的排序鍵：上一頁最後一筆的建立時間與 ID。
type Cursor struct {
	At time.Time
	ID uuid.UUID
}

func (c *Cursor) args() (*time.Time, pgtype.UUID) {
	if c == nil {
		return nil, pgtype.UUID{}
	}
	return &c.At, pgtype.UUID{Bytes: c.ID, Valid: true}
}

func size(n int32) int {
	switch {
	case n <= 0:
		return DefaultPageSize
	case n > MaxPageSize:
		return MaxPageSize
	}
	return int(n)
}

// rowLimit 是 SQL 的 LIMIT：多取一筆來判斷有沒有下一頁（頁大小最多 MaxPageSize，不會溢位）。
func rowLimit(want int) int32 { return int32(want + 1) } //nolint:gosec // want 最多 MaxPageSize

// Checkout 是結帳的情境：只留這張訂單現在用得上的券，並算預估折抵。
type Checkout struct {
	StoreID     int64
	OrderAmount int64
}

// VoucherView 是一張券與它的活動。Expired 從活動的 EndAt 推導（到期不是狀態）。EstimatedDiscount 只有帶 Checkout 時才有值。
type VoucherView struct {
	Voucher           model.Voucher
	Campaign          model.Campaign
	Expired           bool
	EstimatedDiscount int64
}

// VoucherListInput 是買家券列表的輸入。Statuses 為空是全部狀態；帶 Checkout 時只回 AVAILABLE 而且現在能用的券。
type VoucherListInput struct {
	BuyerID  int64
	Statuses []model.VoucherStatus
	Checkout *Checkout
	Size     int32
	After    *Cursor
}

// VoucherPage 是一頁券；Next 為 nil 表示最後一頁。
type VoucherPage struct {
	Vouchers []VoucherView
	Next     *Cursor
}

// ListVouchers 列出買家的券，新到舊。
func (s *Service) ListVouchers(ctx context.Context, in VoucherListInput) (VoucherPage, error) {
	if in.BuyerID <= 0 {
		return VoucherPage{}, model.Invalid(ctx, "BuyerID", model.InvalidBuyerIDInvalid)
	}
	statuses := make([]string, 0, len(in.Statuses))
	for _, st := range in.Statuses {
		statuses = append(statuses, string(st))
	}
	if in.Checkout != nil {
		statuses = []string{string(model.VoucherAvailable)}
	}
	want := size(in.Size)
	now := s.d.Now()
	qualifier := s.d.Qualify.PerRequest()
	var out VoucherPage
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		after := in.After
		for {
			at, id := after.args()
			batch, err := q.ListBuyerVouchers(ctx, store.ListBuyerVouchersParams{
				BuyerID: in.BuyerID, Statuses: statuses, AfterCreatedAt: at, AfterID: id, MaxRows: scanBatch})
			if err != nil {
				return err
			}
			campaigns, err := s.campaigns(ctx, q, batch)
			if err != nil {
				return err
			}
			for _, row := range batch {
				v := rows.Voucher(row)
				c := campaigns[v.CampaignID]
				view := VoucherView{Voucher: v, Campaign: c, Expired: c.Expired(now)}
				if in.Checkout != nil {
					discount, ok, err := usable(ctx, qualifier, c, in.BuyerID, *in.Checkout, now)
					if err != nil {
						return err
					}
					if !ok {
						continue
					}
					view.EstimatedDiscount = discount
				}
				if len(out.Vouchers) == want {
					last := out.Vouchers[len(out.Vouchers)-1].Voucher
					out.Next = &Cursor{At: last.CreatedAt, ID: last.ID}
					return nil
				}
				out.Vouchers = append(out.Vouchers, view)
			}
			if len(batch) < scanBatch {
				return nil
			}
			last := batch[len(batch)-1]
			after = &Cursor{At: last.CreatedAt, ID: last.ID}
		}
	})
	return out, err
}

func (s *Service) campaigns(ctx context.Context, q *store.Queries, vs []store.CouponCouponVoucher) (map[uuid.UUID]model.Campaign, error) {
	ids := make([]uuid.UUID, 0, len(vs))
	for _, v := range vs {
		ids = append(ids, v.CampaignID)
	}
	out := make(map[uuid.UUID]model.Campaign, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	cs, err := q.ListCampaignsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, c := range cs {
		out[c.ID] = rows.Campaign(c)
	}
	return out, nil
}

// usable 回報這張券（的活動）在結帳情境下現在能不能用：與鎖券同一組檢查（Check、Fit、折抵大於 0）。業務上的不合格是 ok=false，
// 讀不到資料（port 失敗）回錯誤，整個請求失敗。
func usable(ctx context.Context, qualifier *qualify.Service, c model.Campaign, buyerID int64, co Checkout, now time.Time) (int64, bool, error) {
	if err := qualifier.Check(ctx, c, buyerID, now); err != nil {
		return 0, false, rejection(err)
	}
	discount, err := qualify.Fit(ctx, c, co.StoreID, co.OrderAmount)
	if err != nil {
		return 0, false, rejection(err)
	}
	if err := qualify.RequireDiscount(ctx, discount); err != nil {
		return 0, false, rejection(err)
	}
	return discount, true, nil
}

// rejection 把業務原因造成的不合格（有 coupon 原因的錯誤）轉成 nil，其他錯誤原樣回傳。
func rejection(err error) error {
	if model.ReasonOf(err) != "" {
		return nil
	}
	return err
}

// ClaimableInput 是買家可領活動列表的輸入；StoreID 非 0 時只留該店與全平台的活動。
type ClaimableInput struct {
	BuyerID int64
	StoreID int64
	Size    int32
	After   *Cursor
}

// CampaignPage 是一頁活動；Next 為 nil 表示最後一頁。
type CampaignPage struct {
	Campaigns []model.Campaign
	Next      *Cursor
}

// ListClaimable 列出買家現在領得到的活動：還有名額、買家沒有這個活動的券，而且通過 qualify.Check（狀態、時間窗、店家停權、資格）。
func (s *Service) ListClaimable(ctx context.Context, in ClaimableInput) (CampaignPage, error) {
	if in.BuyerID <= 0 {
		return CampaignPage{}, model.Invalid(ctx, "BuyerID", model.InvalidBuyerIDInvalid)
	}
	if in.StoreID < 0 {
		return CampaignPage{}, model.Invalid(ctx, "StoreID", model.InvalidStoreInvalid)
	}
	var storeID *int64
	if in.StoreID > 0 {
		storeID = &in.StoreID
	}
	want := size(in.Size)
	now := s.d.Now()
	qualifier := s.d.Qualify.PerRequest()
	var out CampaignPage
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		after := in.After
		for {
			at, id := after.args()
			batch, err := q.ListClaimCandidates(ctx, store.ListClaimCandidatesParams{
				StoreID: storeID, BuyerID: in.BuyerID, Now: now, AfterCreatedAt: at, AfterID: id, MaxRows: scanBatch})
			if err != nil {
				return err
			}
			for _, row := range batch {
				c := rows.Campaign(row)
				if err := qualifier.Check(ctx, c, in.BuyerID, now); err != nil {
					if rejection(err) != nil {
						return err
					}
					continue
				}
				if len(out.Campaigns) == want {
					last := out.Campaigns[len(out.Campaigns)-1]
					out.Next = &Cursor{At: last.CreatedAt, ID: last.ID}
					return nil
				}
				out.Campaigns = append(out.Campaigns, c)
			}
			if len(batch) < scanBatch {
				return nil
			}
			last := batch[len(batch)-1]
			after = &Cursor{At: last.CreatedAt, ID: last.ID}
		}
	})
	return out, err
}

// SellerListInput 是賣家活動列表的輸入；Status 為空是全部。
type SellerListInput struct {
	StoreID int64
	Status  model.CampaignStatus
	Size    int32
	After   *Cursor
}

// SellerCampaignPage 是一頁賣家活動，加上店家的配額與同時進行中的活動數（與建立時的配額檢查同一個口徑）。
type SellerCampaignPage struct {
	Campaigns     []SellerCampaign
	Next          *Cursor
	MaxActive     int32
	CurrentActive int32
}

// SellerCampaign 是賣家看到的一個活動；Expired 用模組的時鐘從 EndAt 推導（與 VoucherView 同一個口徑）。
type SellerCampaign struct {
	Campaign model.Campaign
	Expired  bool
}

// ListSellerCampaigns 列出店家自己發的活動，新到舊。
func (s *Service) ListSellerCampaigns(ctx context.Context, in SellerListInput) (SellerCampaignPage, error) {
	if in.StoreID <= 0 {
		return SellerCampaignPage{}, model.Invalid(ctx, "StoreID", model.InvalidStoreInvalid)
	}
	var status *string
	if in.Status != "" {
		v := string(in.Status)
		status = &v
	}
	want := size(in.Size)
	now := s.d.Now()
	var out SellerCampaignPage
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		at, id := in.After.args()
		batch, err := q.ListSellerCampaigns(ctx, store.ListSellerCampaignsParams{
			StoreID: &in.StoreID, Status: status, AfterCreatedAt: at, AfterID: id, MaxRows: rowLimit(want)})
		if err != nil {
			return err
		}
		for i, row := range batch {
			if i == want {
				last := out.Campaigns[len(out.Campaigns)-1].Campaign
				out.Next = &Cursor{At: last.CreatedAt, ID: last.ID}
				break
			}
			c := rows.Campaign(row)
			out.Campaigns = append(out.Campaigns, SellerCampaign{Campaign: c, Expired: c.Expired(now)})
		}
		out.MaxActive, err = s.quota(ctx, q, in.StoreID)
		if err != nil {
			return err
		}
		n, err := q.CountActiveSellerCampaigns(ctx, store.CountActiveSellerCampaignsParams{StoreID: &in.StoreID, Now: now})
		out.CurrentActive = int32(min(n, math.MaxInt32)) //nolint:gosec // 夾在 int32 範圍內
		return err
	})
	return out, err
}

func (s *Service) quota(ctx context.Context, q *store.Queries, storeID int64) (int32, error) {
	n, err := q.GetSellerQuota(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DefaultSellerQuota, nil
	}
	return n, err
}

// CampaignDetail 是賣家看到的活動明細：名單（DESIGNATED）與各狀態的券數。
type CampaignDetail struct {
	SellerCampaign
	TargetBuyers []int64
	Available    int32
	Locked       int32
	Redeemed     int32
	Voided       int32
}

// GetSellerCampaign 讀店家自己的活動；不存在、不是這家店發的與系統補償活動都回同一個 NotFound（防枚舉）。
func (s *Service) GetSellerCampaign(ctx context.Context, storeID int64, id uuid.UUID) (CampaignDetail, error) {
	if storeID <= 0 {
		return CampaignDetail{}, model.Invalid(ctx, "StoreID", model.InvalidStoreInvalid)
	}
	var out CampaignDetail
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := q.GetCampaign(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return model.NotFound(ctx, model.ReasonCampaignNotFound)
		}
		if err != nil {
			return err
		}
		c := rows.Campaign(row)
		if c.IssuerStoreID == nil || *c.IssuerStoreID != storeID || c.TargetType == model.TargetCompensation {
			return model.NotFound(ctx, model.ReasonCampaignNotFound)
		}
		out.SellerCampaign = SellerCampaign{Campaign: c, Expired: c.Expired(s.d.Now())}
		if out.TargetBuyers, err = q.ListTargetBuyers(ctx, id); err != nil {
			return err
		}
		counts, err := q.CountCampaignVouchers(ctx, id)
		if err != nil {
			return err
		}
		for _, n := range counts {
			switch model.VoucherStatus(n.Status) {
			case model.VoucherAvailable:
				out.Available = n.N
			case model.VoucherLocked:
				out.Locked = n.N
			case model.VoucherRedeemed:
				out.Redeemed = n.N
			case model.VoucherVoided:
				out.Voided = n.N
			}
		}
		return nil
	})
	return out, err
}

// GetVoucher 讀買家自己的一張券與它的活動；不存在與不是這位買家的回同一個 NotFound。
func (s *Service) GetVoucher(ctx context.Context, buyerID int64, id uuid.UUID) (VoucherView, error) {
	if buyerID <= 0 {
		return VoucherView{}, model.Invalid(ctx, "BuyerID", model.InvalidBuyerIDInvalid)
	}
	var out VoucherView
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		vrow, err := q.GetVoucher(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && vrow.BuyerID != buyerID) {
			return model.NotFound(ctx, model.ReasonVoucherNotFound)
		}
		if err != nil {
			return err
		}
		crow, err := q.GetCampaign(ctx, vrow.CampaignID)
		if err != nil {
			return err
		}
		c := rows.Campaign(crow)
		out = VoucherView{Voucher: rows.Voucher(vrow), Campaign: c, Expired: c.Expired(s.d.Now())}
		return nil
	})
	return out, err
}
