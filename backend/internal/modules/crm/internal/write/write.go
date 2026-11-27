// Package write 是 crm 的寫入路徑：客戶分類與標籤。所有寫入帶 store_id（呼叫者由 seller session 決定），use case 自己開 transaction。
package write

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Buyers 是寫入的買家守門（buyer 公開 API）：transaction 內以 FOR KEY SHARE 確認買家尚未關閉，已關閉與不存在回 NotFound。
type Buyers interface {
	LockNotClosed(ctx context.Context, buyerID int64) error
}

// Stores 是寫入的店家守門（seller 公開 API）：transaction 內以 FOR KEY SHARE 確認店家尚未退場，已退場與不存在回 NotFound。
type Stores interface {
	LockNotRetired(ctx context.Context, storeID int64) error
	// Store 讀店家此刻的狀態（含停權的推導），給 authorize 在 transaction 內檢查賣家能力。
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// Deps 是寫入路徑的依賴。
type Deps struct {
	DB     *db.DB
	Buyers Buyers
	Stores Stores
	NewID  func() uuid.UUID
	Now    func() time.Time
}

// Service 是寫入路徑的 use case。
type Service struct{ d Deps }

// New 建立寫入路徑。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

func checkIDs(storeID, buyerID int64) error {
	if storeID <= 0 || buyerID <= 0 {
		return fmt.Errorf("crm：店家 %d 或買家 %d 不是正整數", storeID, buyerID)
	}
	return nil
}

func customerNotFound(ctx context.Context) error {
	return model.WithReason(apperr.NotFound("%s", text.CrmCustomerNotFound(uitext.LocaleFrom(ctx))), model.ReasonCustomerNotFound)
}

func tagNotFound(ctx context.Context) error {
	return model.WithReason(apperr.NotFound("%s", text.CrmTagNotFound(uitext.LocaleFrom(ctx))), model.ReasonTagNotFound)
}

func (s *Service) lockStore(ctx context.Context, storeID int64, notFound func(context.Context) error) error {
	if err := s.d.Stores.LockNotRetired(ctx, storeID); err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return notFound(ctx)
		}
		return err
	}
	return s.authorize(ctx, storeID)
}

func (s *Service) authorize(ctx context.Context, storeID int64) error {
	st, err := s.d.Stores.Store(ctx, storeID)
	if err != nil {
		return err
	}
	return seller.Authorize(st, model.RequiredCapability)
}

func (s *Service) lockCustomerParties(ctx context.Context, storeID, buyerID int64) error {
	if err := s.d.Buyers.LockNotClosed(ctx, buyerID); err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return customerNotFound(ctx)
		}
		return err
	}
	return s.lockStore(ctx, storeID, customerNotFound)
}

func nameTaken(ctx context.Context) error {
	return model.WithReason(apperr.AlreadyExists("%s", text.CrmTagNameTaken(uitext.LocaleFrom(ctx))), model.ReasonTagNameTaken)
}

// NormalizeTagName 去頭尾空白並檢查長度（1 到 MaxTagNameRunes 個字元）；空白或超長回 InvalidInput。
func NormalizeTagName(ctx context.Context, name string) (string, error) {
	n := trimtext.Trim(name)
	if r := utf8.RuneCountInString(n); r < 1 || r > model.MaxTagNameRunes {
		return "", apperr.InvalidInputOn([]string{"Name"}, "%s", text.CrmTagNameInvalid(uitext.LocaleFrom(ctx), text.CrmTagNameInvalidArgs{Max: model.MaxTagNameRunes}))
	}
	return n, nil
}

func customerOf(ctx context.Context, q *store.Queries, storeID, buyerID int64, typ string, spent int64, orders int32, first time.Time) (model.Customer, error) {
	c := model.Customer{StoreID: storeID, BuyerID: buyerID, Type: model.CustomerType(typ), TotalSpent: spent, OrderCount: orders, FirstOrderAt: first, Tags: []model.Tag{}}
	tags, err := q.ListCustomerTags(ctx, store.ListCustomerTagsParams{StoreID: storeID, BuyerIds: []int64{buyerID}})
	if err != nil {
		return model.Customer{}, err
	}
	for _, t := range tags {
		c.Tags = append(c.Tags, model.Tag{ID: t.ID, Name: t.Name})
	}
	return c, nil
}

func loadCustomer(ctx context.Context, q *store.Queries, storeID, buyerID int64) (model.Customer, error) {
	r, err := q.GetCustomer(ctx, store.GetCustomerParams{StoreID: storeID, BuyerID: buyerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Customer{}, customerNotFound(ctx)
	}
	if err != nil {
		return model.Customer{}, err
	}
	return customerOf(ctx, q, storeID, buyerID, r.CustomerType, r.TotalSpent, r.OrderCount, r.FirstOrderAt)
}

// SetCustomerType 設定店家對買家的分類，回傳寫入當下的客戶。分類值不在值域回 InvalidInput；店家沒有這位客戶（沒有列）回 NotFound。
func (s *Service) SetCustomerType(ctx context.Context, storeID, buyerID int64, t model.CustomerType) (model.Customer, error) {
	if err := checkIDs(storeID, buyerID); err != nil {
		return model.Customer{}, err
	}
	if !t.Valid() {
		return model.Customer{}, apperr.InvalidInputOn([]string{"CustomerType"}, "%s", text.CrmCustomerTypeInvalid(uitext.LocaleFrom(ctx)))
	}
	var out model.Customer
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.lockCustomerParties(ctx, storeID, buyerID); err != nil {
			return err
		}
		q := s.q(ctx)
		r, err := q.SetCustomerType(ctx, store.SetCustomerTypeParams{
			CustomerType: string(t), Now: s.d.Now().UTC(), StoreID: storeID, BuyerID: buyerID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return customerNotFound(ctx)
		}
		if err != nil {
			return err
		}
		out, err = customerOf(ctx, q, storeID, buyerID, r.CustomerType, r.TotalSpent, r.OrderCount, r.FirstOrderAt)
		return err
	})
	return out, err
}

// CreateTag 建立標籤。名稱撞同店的標籤回 AlreadyExists；超過每店上限回 FailedPrecondition；店家已退場（守門 LockNotRetired 回 NotFound）回 NotFound（標籤不存在）。
func (s *Service) CreateTag(ctx context.Context, storeID int64, name string) (model.Tag, error) {
	if storeID <= 0 {
		return model.Tag{}, fmt.Errorf("crm：店家 %d 不是正整數", storeID)
	}
	n, err := NormalizeTagName(ctx, name)
	if err != nil {
		return model.Tag{}, err
	}
	tag := model.Tag{ID: s.d.NewID(), Name: n}
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := s.lockStore(ctx, storeID, tagNotFound); err != nil {
			return err
		}
		if err := q.LockStoreTags(ctx, storeID); err != nil {
			return err
		}
		if err := s.checkStoreCap(ctx, q, storeID, 1); err != nil {
			return err
		}
		rows, err := q.InsertTag(ctx, store.InsertTagParams{ID: tag.ID, StoreID: storeID, Name: n, Now: s.d.Now().UTC()})
		if err != nil {
			return err
		}
		if rows == 0 {
			return nameTaken(ctx)
		}
		return nil
	})
	if err != nil {
		return model.Tag{}, err
	}
	return tag, nil
}

func (s *Service) checkStoreCap(ctx context.Context, q *store.Queries, storeID int64, adding int) error {
	count, err := q.CountStoreTags(ctx, storeID)
	if err != nil {
		return err
	}
	if count+int64(adding) > model.MaxTagsPerStore {
		return model.WithReason(apperr.FailedPrecondition("%s", text.CrmTagLimitStore(uitext.LocaleFrom(ctx), text.CrmTagLimitStoreArgs{Max: model.MaxTagsPerStore})), model.ReasonTagLimit)
	}
	return nil
}

// RenameTag 改標籤名稱，回傳寫入當下的標籤。標籤不存在（或屬於別家店）回 NotFound，撞同店的別個標籤回 AlreadyExists。
func (s *Service) RenameTag(ctx context.Context, storeID int64, tagID uuid.UUID, name string) (model.Tag, error) {
	if storeID <= 0 {
		return model.Tag{}, fmt.Errorf("crm：店家 %d 不是正整數", storeID)
	}
	n, err := NormalizeTagName(ctx, name)
	if err != nil {
		return model.Tag{}, err
	}
	var out model.Tag
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := s.lockStore(ctx, storeID, tagNotFound); err != nil {
			return err
		}
		if err := q.LockStoreTags(ctx, storeID); err != nil {
			return err
		}
		r, err := q.RenameTag(ctx, store.RenameTagParams{Name: n, Now: s.d.Now().UTC(), StoreID: storeID, ID: tagID})
		if errors.Is(err, pgx.ErrNoRows) {
			return tagNotFound(ctx)
		}
		if err != nil {
			return err
		}
		out = model.Tag{ID: r.ID, Name: r.Name}
		return nil
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "tags_store_name_key" {
		return model.Tag{}, nameTaken(ctx)
	}
	return out, err
}

// DeleteTag 刪標籤；客戶身上的關聯由 FK cascade 刪除。標籤不存在回 NotFound。
func (s *Service) DeleteTag(ctx context.Context, storeID int64, tagID uuid.UUID) error {
	if storeID <= 0 {
		return fmt.Errorf("crm：店家 %d 不是正整數", storeID)
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := s.lockStore(ctx, storeID, tagNotFound); err != nil {
			return err
		}
		if err := q.LockStoreTags(ctx, storeID); err != nil {
			return err
		}
		rows, err := q.DeleteTag(ctx, store.DeleteTagParams{StoreID: storeID, ID: tagID})
		if err != nil {
			return err
		}
		if rows == 0 {
			return tagNotFound(ctx)
		}
		return nil
	})
}

// AddCustomerTags 替客戶貼上一批標籤（名稱不存在的先建立）。整批是一個 transaction：任何一步失敗，整批都沒有寫入，並回傳那個錯誤。
// 客戶不存在回 NotFound；names 為空或超過 MaxTagsPerCall、名稱不合法回 InvalidInput；超過每店或每位客戶的上限回 FailedPrecondition。
// 已貼過的標籤略過（冪等）。
func (s *Service) AddCustomerTags(ctx context.Context, storeID, buyerID int64, names []string) (model.Customer, error) {
	if err := checkIDs(storeID, buyerID); err != nil {
		return model.Customer{}, err
	}
	if len(names) == 0 || len(names) > model.MaxTagsPerCall {
		return model.Customer{}, apperr.InvalidInputOn([]string{"Names"}, "%s", text.CrmTagNamesInvalid(uitext.LocaleFrom(ctx), text.CrmTagNamesInvalidArgs{Max: model.MaxTagsPerCall}))
	}
	uniq := make([]string, 0, len(names))
	for i, raw := range names {
		n, err := NormalizeTagName(ctx, raw)
		if err != nil {
			var e *apperr.Error
			if errors.As(err, &e) {
				return model.Customer{}, apperr.InvalidInputOn([]string{apperr.IndexedField("Names", i)}, "%s", e.Message)
			}
			return model.Customer{}, err
		}
		if !slices.Contains(uniq, n) {
			uniq = append(uniq, n)
		}
	}
	var out model.Customer
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := s.lockCustomerParties(ctx, storeID, buyerID); err != nil {
			return err
		}
		if _, err := q.LockCustomer(ctx, store.LockCustomerParams{StoreID: storeID, BuyerID: buyerID}); errors.Is(err, pgx.ErrNoRows) {
			return customerNotFound(ctx)
		} else if err != nil {
			return err
		}
		if err := q.LockStoreTags(ctx, storeID); err != nil {
			return err
		}
		existing, err := q.ListTagIDsByNames(ctx, store.ListTagIDsByNamesParams{StoreID: storeID, Names: uniq})
		if err != nil {
			return err
		}
		ids := make(map[string]uuid.UUID, len(uniq))
		for _, t := range existing {
			ids[t.Name] = t.ID
		}
		var missing []string
		for _, n := range uniq {
			if _, ok := ids[n]; !ok {
				missing = append(missing, n)
			}
		}
		if err := s.checkStoreCap(ctx, q, storeID, len(missing)); err != nil {
			return err
		}
		tagIDs := make([]uuid.UUID, 0, len(uniq))
		for _, n := range uniq {
			if id, ok := ids[n]; ok {
				tagIDs = append(tagIDs, id)
			}
		}
		linked, err := q.CountCustomerTagsAmong(ctx, store.CountCustomerTagsAmongParams{StoreID: storeID, BuyerID: buyerID, TagIds: tagIDs})
		if err != nil {
			return err
		}
		have, err := q.CountCustomerTags(ctx, store.CountCustomerTagsParams{StoreID: storeID, BuyerID: buyerID})
		if err != nil {
			return err
		}
		if have+int64(len(uniq))-linked > model.MaxTagsPerCustomer {
			return model.WithReason(apperr.FailedPrecondition("%s", text.CrmTagLimitCustomer(uitext.LocaleFrom(ctx), text.CrmTagLimitCustomerArgs{Max: model.MaxTagsPerCustomer})), model.ReasonTagLimit)
		}
		now := s.d.Now().UTC()
		for _, n := range missing {
			id := s.d.NewID()
			rows, err := q.InsertTag(ctx, store.InsertTagParams{ID: id, StoreID: storeID, Name: n, Now: now})
			if err != nil {
				return err
			}
			if rows == 0 {
				return fmt.Errorf("crm：標籤 %q 在持有店家鎖時被別人建立", n)
			}
			tagIDs = append(tagIDs, id)
		}
		if _, err = q.InsertCustomerTags(ctx, store.InsertCustomerTagsParams{StoreID: storeID, BuyerID: buyerID, TagIds: tagIDs, Now: now}); err != nil {
			return err
		}
		out, err = loadCustomer(ctx, q, storeID, buyerID)
		return err
	})
	return out, err
}

// RemoveCustomerTag 移除客戶身上的一個標籤，回傳移除之後（同 transaction）的客戶。冪等：沒有這個關聯不是錯誤（移除後的狀態與呼叫者要的一致）。
func (s *Service) RemoveCustomerTag(ctx context.Context, storeID, buyerID int64, tagID uuid.UUID) (model.Customer, error) {
	if err := checkIDs(storeID, buyerID); err != nil {
		return model.Customer{}, err
	}
	var out model.Customer
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.lockCustomerParties(ctx, storeID, buyerID); err != nil {
			return err
		}
		q := s.q(ctx)
		if _, err := q.DeleteCustomerTag(ctx, store.DeleteCustomerTagParams{StoreID: storeID, BuyerID: buyerID, TagID: tagID}); err != nil {
			return err
		}
		var err error
		out, err = loadCustomer(ctx, q, storeID, buyerID)
		return err
	})
	return out, err
}
