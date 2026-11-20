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
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Deps 是寫入路徑的依賴。
type Deps struct {
	DB    *db.DB
	NewID func() uuid.UUID
	Now   func() time.Time
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

// SetCustomerType 設定店家對買家的分類。分類值不在值域回 InvalidInput；店家沒有這位客戶（0 列）回 NotFound。
func (s *Service) SetCustomerType(ctx context.Context, storeID, buyerID int64, t model.CustomerType) error {
	if err := checkIDs(storeID, buyerID); err != nil {
		return err
	}
	if !t.Valid() {
		return apperr.InvalidInputOn([]string{"CustomerType"}, "%s", text.CrmCustomerTypeInvalid(uitext.LocaleFrom(ctx)))
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		n, err := s.q(ctx).SetCustomerType(ctx, store.SetCustomerTypeParams{
			CustomerType: string(t), Now: s.d.Now().UTC(), StoreID: storeID, BuyerID: buyerID,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return customerNotFound(ctx)
		}
		return nil
	})
}

// CreateTag 建立標籤。名稱撞同店的標籤回 AlreadyExists；超過每店上限回 FailedPrecondition。
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

// RenameTag 改標籤名稱。標籤不存在（或屬於別家店）回 NotFound，撞同店的別個標籤回 AlreadyExists。
func (s *Service) RenameTag(ctx context.Context, storeID int64, tagID uuid.UUID, name string) error {
	if storeID <= 0 {
		return fmt.Errorf("crm：店家 %d 不是正整數", storeID)
	}
	n, err := NormalizeTagName(ctx, name)
	if err != nil {
		return err
	}
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).RenameTag(ctx, store.RenameTagParams{Name: n, Now: s.d.Now().UTC(), StoreID: storeID, ID: tagID})
		if err != nil {
			return err
		}
		if rows == 0 {
			return tagNotFound(ctx)
		}
		return nil
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "tags_store_name_key" {
		// 唯一鍵違反使 transaction 失敗，InTx 已 rollback；這裡只翻譯錯誤，不在同一個 transaction 繼續下 SQL。
		return nameTaken(ctx)
	}
	return err
}

// DeleteTag 刪標籤；客戶身上的關聯由 FK cascade 刪除。標籤不存在回 NotFound。
func (s *Service) DeleteTag(ctx context.Context, storeID int64, tagID uuid.UUID) error {
	if storeID <= 0 {
		return fmt.Errorf("crm：店家 %d 不是正整數", storeID)
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).DeleteTag(ctx, store.DeleteTagParams{StoreID: storeID, ID: tagID})
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
func (s *Service) AddCustomerTags(ctx context.Context, storeID, buyerID int64, names []string) error {
	if err := checkIDs(storeID, buyerID); err != nil {
		return err
	}
	if len(names) == 0 || len(names) > model.MaxTagsPerCall {
		return apperr.InvalidInputOn([]string{"Names"}, "%s", text.CrmTagNamesInvalid(uitext.LocaleFrom(ctx), text.CrmTagNamesInvalidArgs{Max: model.MaxTagsPerCall}))
	}
	uniq := make([]string, 0, len(names))
	for i, raw := range names {
		n, err := NormalizeTagName(ctx, raw)
		if err != nil {
			var e *apperr.Error
			if errors.As(err, &e) {
				return apperr.InvalidInputOn([]string{apperr.IndexedField("Names", i)}, "%s", e.Message)
			}
			return err
		}
		if !slices.Contains(uniq, n) {
			uniq = append(uniq, n)
		}
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		// 先鎖客戶列（沒有列 = 不是客戶，在 FK 之前就回 NotFound），再鎖店家的標籤建立；全模組都是這個順序。
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
		_, err = q.InsertCustomerTags(ctx, store.InsertCustomerTagsParams{StoreID: storeID, BuyerID: buyerID, TagIds: tagIDs, Now: now})
		return err
	})
}

// RemoveCustomerTag 移除客戶身上的一個標籤。冪等：沒有這個關聯不是錯誤（移除後的狀態與呼叫者要的一致）。
func (s *Service) RemoveCustomerTag(ctx context.Context, storeID, buyerID int64, tagID uuid.UUID) error {
	if err := checkIDs(storeID, buyerID); err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		_, err := s.q(ctx).DeleteCustomerTag(ctx, store.DeleteCustomerTagParams{StoreID: storeID, BuyerID: buyerID, TagID: tagID})
		return err
	})
}
