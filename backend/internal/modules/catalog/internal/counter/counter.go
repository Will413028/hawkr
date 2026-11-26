// Package counter 是商品上由別的模組擁有事實的衍生計數：評分與則數（review）、銷量（ordering）。
//   - 事實擁有者在自己的 transaction 內呼叫（D3）：這裡用呼叫方的 transaction（ctx 沒有時各是單句寫入），不另開。
//   - 評分傳絕對值，不用 ±1；沒有版本欄，並行的更新靠商品列的行鎖序列化，後提交的絕對值勝出（D26）。
//   - 計數不改 content_version：etag 只涵蓋賣家可編輯的內容。
package counter

import (
	"bytes"
	"context"
	"math"
	"slices"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

const maxOrderNoRunes = 200

// Deps 是計數的依賴。
type Deps struct {
	DB  *db.DB
	Now func() time.Time
}

// Service 是計數的 use case。
type Service struct{ d Deps }

// New 建立計數 use case。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// SetRating 把商品的評分平均與評價則數設成絕對值。average 在 0 到 5 之間（不是 NaN），count 不為負，count 為 0 時 average 必為 0
// （與 products_rating_check 一致）；不合法回 *model.InvalidInputError，不寫入。商品不存在回 model.ErrProductNotFound；已刪除的商品仍更新（評價還在）。
func (s *Service) SetRating(ctx context.Context, productID uuid.UUID, average float64, count int32) error {
	switch {
	case productID == uuid.Nil():
		return model.Invalid("ProductID", model.ReasonRequired)
	case math.IsNaN(average) || average < 0 || average > 5:
		return model.Invalid("Average", model.ReasonOutOfRange)
	case count < 0:
		return model.Invalid("Count", model.ReasonOutOfRange)
	case count == 0 && average != 0:
		return model.Invalid("Average", model.ReasonInvalid)
	}
	n, err := s.q(ctx).SetProductRating(ctx, store.SetProductRatingParams{ID: productID, RatingAverage: average, ReviewCount: count, Now: s.d.Now()})
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrProductNotFound
	}
	return nil
}

// RecordSales 把一筆訂單的銷量累計到各商品，同一個 orderNo 只累計一次：先 INSERT … ON CONFLICT DO NOTHING 登記 orderNo，只有第一次插入成功才增加 sales_count，
// 重送回 nil 且不改任何計數。登記與累計在同一個 transaction（呼叫方的，ctx 沒有時自己開一個），任何商品不存在整筆回滾（含登記），回 model.ErrProductNotFound。
//
// 必要條件：Java order 以 MQ 送出銷量事件，MQ 會重送，直到 ordering 搬進單體（B8）改成同 transaction 呼叫為止；到那時去重表 catalog.sales_orders 退場。
// 同一個商品出現多次（不同 SKU）先加總；商品依 ID 升序更新，兩筆同時處理的訂單不會互相死結。
// 在呼叫方的 transaction 內回 error（含 ErrProductNotFound）時，呼叫方必須回滾那個 transaction：這裡不開 savepoint，已寫入的訂單登記仍在 transaction 裡，繼續 commit 會留下登記了但沒累計的訂單，之後的重送被去重擋下。未知商品是否當毒訊息丟棄由 B4-4 的消費端決定。
func (s *Service) RecordSales(ctx context.Context, orderNo string, items []model.SalesItem) error {
	if n := utf8.RuneCountInString(orderNo); n < 1 || n > maxOrderNoRunes || trimtext.Trim(orderNo) != orderNo {
		return model.Invalid("OrderNo", model.ReasonInvalid)
	}
	if len(items) == 0 || len(items) > model.MaxSalesItems {
		return model.Invalid("Items", model.ReasonOutOfRange)
	}
	byProduct := make(map[uuid.UUID]int64, len(items))
	for _, it := range items {
		if it.ProductID == uuid.Nil() || it.Quantity <= 0 {
			return model.Invalid("Items", model.ReasonInvalid)
		}
		byProduct[it.ProductID] += int64(it.Quantity)
	}
	ids := make([]uuid.UUID, 0, len(byProduct))
	for id := range byProduct {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		now := s.d.Now()
		fresh, err := q.ClaimSalesOrder(ctx, store.ClaimSalesOrderParams{OrderNo: orderNo, Now: now})
		if err != nil {
			return err
		}
		if fresh == 0 {
			return nil
		}
		for _, id := range ids {
			n, err := q.AddProductSales(ctx, store.AddProductSalesParams{ID: id, Quantity: byProduct[id], Now: now})
			if err != nil {
				return err
			}
			if n == 0 {
				return model.ErrProductNotFound
			}
		}
		return nil
	})
}
