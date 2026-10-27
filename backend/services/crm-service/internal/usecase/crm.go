package usecase

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
	"github.com/Will413028/hawkr/crm-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
)

type CrmUseCase struct {
	repo          domain.CrmRepository
	accountClient AccountClient
	buyerClient   BuyerClient
	couponClient  CouponClient
	orderClient   OrderClient
	tsid          func() int64
	txManager     database.TxManager
}

func NewCrmUseCase(repo domain.CrmRepository, accountClient AccountClient, buyerClient BuyerClient, couponClient CouponClient, orderClient OrderClient, tsid func() int64, txManager database.TxManager) *CrmUseCase {
	return &CrmUseCase{
		repo:          repo,
		accountClient: accountClient,
		buyerClient:   buyerClient,
		couponClient:  couponClient,
		orderClient:   orderClient,
		tsid:          tsid,
		txManager:     txManager,
	}
}

func (uc *CrmUseCase) GetCustomer(ctx context.Context, storeID, buyerID int64) (*dto.Customer, error) {
	customer, err := uc.repo.GetCustomer(ctx, storeID, buyerID)
	if err != nil {
		return nil, err
	}
	result := toCustomerDTO(customer)

	tags, err := uc.repo.GetCustomerTags(ctx, storeID, buyerID)
	if err != nil {
		logger.Warn(ctx, "failed to fetch customer tags", zap.Int64("store_id", storeID), zap.Int64("buyer_id", buyerID), zap.Error(err))
	} else {
		result.Tags = make([]*dto.Tag, 0, len(tags))
		for _, t := range tags {
			result.Tags = append(result.Tags, toTagDTO(t))
		}
	}

	var resolvedAccountID int64
	if uc.buyerClient != nil {
		profile, err := uc.buyerClient.GetProfileByBuyerID(ctx, buyerID)
		switch {
		case err != nil:
			logger.Warn(ctx, "failed to fetch buyer profile", zap.Int64("buyer_id", buyerID), zap.Error(err))
		case profile == nil:
			logger.Warn(ctx, "buyer profile not found, leaving enrichment to DB cache", zap.Int64("buyer_id", buyerID))
		default:
			// 空值不覆寫快取——與 ListCustomers / SearchCustomersByTags 同一條
			// 規則。少了這層守衛，同一個買家會在顧客列表顯示快取的名字、在顧客
			// 詳情顯示空白。
			if profile.Nickname != "" {
				result.Nickname = profile.Nickname
			}
			// avatar 刻意沒有守衛：空頭像是有效狀態、不影響 keyword 搜尋，
			// 且 ListCustomers 那側也是無條件覆寫，兩邊維持一致。
			result.Avatar = profile.AvatarURL
			resolvedAccountID = profile.AccountID
		}
	}

	if uc.accountClient != nil && resolvedAccountID != 0 {
		info, err := uc.accountClient.GetAccountInfo(ctx, resolvedAccountID)
		if err != nil {
			logger.Warn(ctx, "failed to fetch account info", zap.Int64("account_id", resolvedAccountID), zap.Error(err))
		} else if info != nil {
			// 同上的非空守衛。這裡特別容易踩到：買家以 OAuth 註冊時 account
			// 可能沒有登入電話，但 RecordOrder 已把 buyers.phone 寫進
			// crm_customers.phone，無條件覆寫會讓列表顯示號碼、詳情顯示空白。
			if info.Phone != "" {
				result.Phone = info.Phone
			}
			// email 目前恆為守衛前的空字串（crm_customers 沒有 email 欄，
			// toCustomerDTO 不會填），所以這層守衛現在是 no-op；保留是為了讓
			// 三處守衛逐字相同，日後若加了 email 快取欄不會漏。
			if info.Email != "" {
				result.Email = info.Email
			}
		}
	}

	if uc.buyerClient != nil && resolvedAccountID != 0 {
		addr, err := uc.buyerClient.GetDefaultAddress(ctx, buyerID)
		if err != nil {
			logger.Warn(ctx, "failed to fetch buyer address", zap.Int64("buyer_id", buyerID), zap.Error(err))
		} else {
			result.Address = addr
		}
	}

	if uc.couponClient != nil {
		count, err := uc.couponClient.GetBuyerCouponCount(ctx, storeID, buyerID)
		if err != nil {
			logger.Warn(ctx, "failed to fetch coupon count", zap.Int64("store_id", storeID), zap.Int64("buyer_id", buyerID), zap.Error(err))
		} else {
			result.CouponCount = count
		}
	}

	return result, nil
}

func (uc *CrmUseCase) ListCustomers(ctx context.Context, req dto.ListCustomersReq) (*dto.PaginatedCustomers, error) {
	params := domain.CustomerListParams{
		StoreID:      req.StoreID,
		Page:         req.Page,
		PageSize:     req.PageSize,
		Keyword:      req.Keyword,
		CustomerType: req.CustomerType,
		TagID:        req.TagID,
		SortBy:       req.SortBy,
		SortOrder:    req.SortOrder,
	}

	total, err := uc.repo.CountCustomers(ctx, params)
	if err != nil {
		return nil, err
	}

	customers, err := uc.repo.ListCustomers(ctx, params)
	if err != nil {
		return nil, err
	}

	list := make([]*dto.Customer, 0, len(customers))
	for _, c := range customers {
		list = append(list, toCustomerDTO(c))
	}

	// Batch enrich with nickname/avatar from buyer-service
	if uc.buyerClient != nil && len(list) > 0 {
		ids := make([]int64, 0, len(list))
		for _, c := range list {
			ids = append(ids, c.BuyerID)
		}
		// ids 全是 crm_customers.buyer_id → 走 buyer_id 軸的批次 RPC。
		metaMap, err := uc.buyerClient.GetBuyerMetaBatchByBuyerIDs(ctx, ids)
		if err != nil {
			logger.Warn(ctx, "failed to batch fetch buyer meta for customer list", zap.Error(err))
		} else {
			updates := make([]domain.CustomerCacheUpdate, 0, len(metaMap))
			for _, c := range list {
				if meta, ok := metaMap[c.BuyerID]; ok {
					c.Avatar = meta.AvatarURL
					// 空的 display_name 不覆寫快取值——與
					// SearchCustomersByTags 同一條規則，也與
					// UpsertCustomerOnOrder 的 COALESCE(NULLIF(...)) 同一個
					// 取捨：買家真的清空暱稱時 crm 保留舊值（罕見、無害），
					// 好過讓顧客從賣家後台的 keyword 搜尋中消失（常見、有害，
					// ListCustomers / CountCustomers 的 keyword 條件直接吃
					// crm_customers.nickname / .phone）。
					// buyers.display_name 可為空：欄位 nullable，且
					// Buyer.UpdateProfile 接受空字串。
					if meta.Nickname != "" {
						c.Nickname = meta.Nickname
					}
					updates = append(updates, domain.CustomerCacheUpdate{
						BuyerID: c.BuyerID,
						// 用生效後的 c.Nickname 而非 meta.Nickname：這樣
						// 回應內容與寫回 DB 的內容結構上不可能再分岔。
						Nickname: c.Nickname,
						Phone:    c.Phone,
					})
				}
			}
			// Async write-back to keep DB cache fresh for keyword search
			if len(updates) > 0 {
				bgCtx := context.WithoutCancel(ctx)
				syncutil.SafeGo(bgCtx, func() {
					if err := uc.repo.BatchUpdateCustomerCache(bgCtx, req.StoreID, updates); err != nil {
						logger.Warn(bgCtx, "failed to write back customer cache", zap.Error(err))
					}
				})
			}
		}
	}

	return &dto.PaginatedCustomers{
		Total: total,
		List:  list,
	}, nil
}

func (uc *CrmUseCase) UpdateCustomerType(ctx context.Context, req dto.UpdateCustomerTypeReq) error {
	return uc.repo.UpdateCustomerType(ctx,
		req.StoreID,
		req.BuyerID,
		domain.CustomerType(req.CustomerType),
	)
}

func (uc *CrmUseCase) ListTags(ctx context.Context, storeID int64) ([]*dto.Tag, error) {
	tags, err := uc.repo.ListTags(ctx, storeID)
	if err != nil {
		return nil, err
	}
	result := make([]*dto.Tag, 0, len(tags))
	for _, t := range tags {
		result = append(result, toTagDTO(t))
	}
	return result, nil
}

func (uc *CrmUseCase) CreateTag(ctx context.Context, req dto.CreateTagReq) (*dto.Tag, error) {
	tag := &domain.CrmTag{
		ID:      uc.tsid(),
		StoreID: req.StoreID,
		Name:    req.Name,
	}
	if err := uc.repo.CreateTag(ctx, tag); err != nil {
		return nil, err
	}
	return toTagDTO(tag), nil
}

func (uc *CrmUseCase) UpdateTag(ctx context.Context, req dto.UpdateTagReq) error {
	tag := &domain.CrmTag{
		ID:      req.TagID,
		StoreID: req.StoreID,
		Name:    req.Name,
	}
	return uc.repo.UpdateTag(ctx, tag)
}

func (uc *CrmUseCase) DeleteTag(ctx context.Context, storeID, tagID int64) error {
	return uc.repo.DeleteTag(ctx, storeID, tagID)
}

func (uc *CrmUseCase) GetCustomerTags(ctx context.Context, storeID, buyerID int64) ([]*dto.Tag, error) {
	tags, err := uc.repo.GetCustomerTags(ctx, storeID, buyerID)
	if err != nil {
		return nil, err
	}
	result := make([]*dto.Tag, 0, len(tags))
	for _, t := range tags {
		result = append(result, toTagDTO(t))
	}
	return result, nil
}

func (uc *CrmUseCase) AddCustomerTags(ctx context.Context, req dto.AddCustomerTagsByNameReq) error {
	for _, name := range req.TagNames {
		tag, err := uc.repo.GetTagByName(ctx, req.StoreID, name)
		if err != nil {
			tag = &domain.CrmTag{ID: uc.tsid(), StoreID: req.StoreID, Name: name}
			if err := uc.repo.CreateTag(ctx, tag); err != nil {
				logger.Warn(ctx, "failed to create tag", zap.String("name", name), zap.Error(err))
				continue
			}
		}
		if err := uc.repo.AddCustomerTag(ctx, req.StoreID, req.BuyerID, tag.ID); err != nil {
			logger.Warn(ctx, "failed to add customer tag",
				zap.Int64("store_id", req.StoreID),
				zap.Int64("buyer_id", req.BuyerID),
				zap.Int64("tag_id", tag.ID),
				zap.Error(err),
			)
		}
	}
	return nil
}

func (uc *CrmUseCase) RemoveCustomerTag(ctx context.Context, req dto.ManageCustomerTagReq) error {
	return uc.repo.RemoveCustomerTag(ctx, req.StoreID, req.BuyerID, req.TagID)
}

func (uc *CrmUseCase) RecordOrder(ctx context.Context, eventID, orderNo string, storeID, buyerID, amount int64) (bool, error) {
	nickname, phone := "", ""
	if uc.buyerClient != nil {
		profile, err := uc.buyerClient.GetProfileByBuyerID(ctx, buyerID)
		switch {
		case err != nil:
			logger.Warn(ctx, "first-touch: failed to fetch buyer profile", zap.Int64("buyer_id", buyerID), zap.Error(err))
		case profile == nil:
			logger.Warn(ctx, "first-touch: buyer profile not found", zap.Int64("buyer_id", buyerID))
		default:
			nickname = profile.Nickname
			phone = profile.Phone
			if uc.accountClient != nil && profile.AccountID != 0 {
				info, err := uc.accountClient.GetAccountInfo(ctx, profile.AccountID)
				if err != nil {
					logger.Warn(ctx, "first-touch: failed to fetch account info, falling back to buyer profile phone", zap.Int64("account_id", profile.AccountID), zap.Error(err))
				} else if info != nil && info.Phone != "" {
					phone = info.Phone
				}
			}
		}
	}

	recorded := false
	err := uc.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := uc.repo.LockOrderForNetting(txCtx, orderNo); err != nil {
			return err
		}
		hasFullRefund, err := uc.repo.HasFullRefundEntry(txCtx, orderNo)
		if err != nil {
			return err
		}
		effectiveAmount := amount
		if hasFullRefund {
			effectiveAmount = 0
		}
		inserted, err := uc.repo.AppendLedgerEntry(txCtx, domain.LedgerEntry{
			EventID:     eventID,
			EventType:   "order_completed",
			OrderNo:     orderNo,
			StoreID:     storeID,
			BuyerID:     buyerID,
			AmountDelta: effectiveAmount,
			CountDelta:  1,
		})
		if err != nil {
			return err
		}
		if !inserted {
			logger.Info(txCtx, "order event already processed, skip", zap.String("event_id", eventID), zap.String("order_no", orderNo))
			return nil
		}
		if err := uc.repo.UpsertCustomerOnOrder(txCtx, uc.tsid(), storeID, buyerID, effectiveAmount, nickname, phone); err != nil {
			return err
		}
		recorded = true
		return nil
	})
	return recorded, err
}

// RecordRefund appends a refund's net contribution to the customer ledger
// and applies the delta to crm_customers.total_spent in one transaction,
// exactly once per event_id. order_count is never touched — a refunded
// order stays in the seller's purchase-records list (see spec Background).
//
// FULL_REFUND does not trust order-service's refund_total_amount (known
// unreliable on that path), and does not net to any order-service-reported
// fixed amount either: order.completed structurally never arrives for the
// dominant full-refund path (verified against
// OrderRepository.completeDeliveredOrderByStatus's strict
// status=DELIVERED CAS — once an order reaches FULLY_REFUNDED(70) directly
// from DELIVERED(40), that CAS can never succeed again, so order.completed
// is not delayed, it is permanently never published). Instead FULL_REFUND
// nets to exactly the ledger's own history for this order_no, negated,
// unconditionally:
//
//	amount_delta = -S      where S = SUM(amount_delta) WHERE order_no = $1
//
// S is always either the complete history for this order (order.completed
// never arrives, nothing more will ever be appended) or already includes
// order.completed's own contribution (it did land) — either way, negating
// it alone is correct. See spec §2's FULL_REFUND bullet (attempt 5) for the
// full derivation and the four earlier wrong attempts.
//
// S is read via SumLedgerAmountForOrder inside the same transaction that
// appends this refund's own ledger row, under LockOrderForNetting — a
// Postgres advisory lock keyed on order_no, taken as the transaction's
// first statement. order.completed and refund.completed are consumed by
// separate MQ subscriptions, each its own goroutine (pkg/mq/resilient.go),
// so without this lock a concurrently-committing RecordOrder or a second
// FULL_REFUND for the same order_no could interleave with this read — the
// lock serializes every event for a given order_no across
// goroutines/subscriptions, closing that race and the lost-update case
// where two concurrent FULL_REFUND events both read the same stale state.
//
// It does NOT close event *reordering*: mutual exclusion is not ordering.
// A PARTIAL_REFUND redelivered to the queue tail by MQ retry, landing
// after a FULL_REFUND has already zero-netted this order, still applies
// its unconditional -refund_amount and permanently understates
// total_spent. This requires the events to already be out of business
// order (an uncommon case-history sequence) plus a transient retry —
// accepted, documented residual risk, not fixed here (see spec §2).
//
// See RecordOrder's doc comment for the symmetric fix on the other side:
// an order.completed that arrives after a FULL_REFUND already zero-netted
// its order_no must not re-add money on top.
func (uc *CrmUseCase) RecordRefund(ctx context.Context, eventID, orderNo, refundOutcome string, refundAmount int64) (bool, error) {
	order, err := uc.orderClient.GetOrderByNo(ctx, orderNo)
	if err != nil {
		return false, fmt.Errorf("resolve order %s: %w", orderNo, err)
	}

	recorded := false
	err = uc.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := uc.repo.LockOrderForNetting(txCtx, orderNo); err != nil {
			return err
		}
		var amountDelta int64
		switch refundOutcome {
		case "PARTIAL_REFUND":
			amountDelta = -refundAmount
		case "FULL_REFUND":
			sum, err := uc.repo.SumLedgerAmountForOrder(txCtx, orderNo)
			if err != nil {
				return err
			}
			amountDelta = -sum
		default:
			return fmt.Errorf("unknown refund_outcome %q for order_no=%s", refundOutcome, orderNo)
		}

		eventType := "refund_completed_partial"
		if refundOutcome == "FULL_REFUND" {
			eventType = "refund_completed_full"
		}
		inserted, err := uc.repo.AppendLedgerEntry(txCtx, domain.LedgerEntry{
			EventID:     eventID,
			EventType:   eventType,
			OrderNo:     orderNo,
			StoreID:     order.StoreID,
			BuyerID:     order.BuyerID,
			AmountDelta: amountDelta,
			CountDelta:  0,
		})
		if err != nil {
			return err
		}
		if !inserted {
			logger.Info(txCtx, "refund event already processed, skip", zap.String("event_id", eventID), zap.String("order_no", orderNo))
			return nil
		}

		rows, err := uc.repo.ApplyRefundDelta(txCtx, uc.tsid(), order.StoreID, order.BuyerID, amountDelta)
		if err != nil {
			return err
		}
		if rows != 1 {
			return fmt.Errorf("refund for order_no=%s: expected exactly 1 row affected by upsert, got %d (store=%d customer=%d)", orderNo, rows, order.StoreID, order.BuyerID)
		}
		recorded = true
		return nil
	})
	return recorded, err
}

func (uc *CrmUseCase) SearchCustomersByTags(ctx context.Context, req dto.SearchCustomersByTagsReq) (*dto.SearchCustomersByTagsResult, error) {
	customers, err := uc.repo.SearchCustomersByTagIds(ctx, req.StoreID, req.TagIDs)
	if err != nil {
		return nil, err
	}

	result := make([]*dto.MatchedCustomer, 0, len(customers))
	for _, c := range customers {
		result = append(result, &dto.MatchedCustomer{
			BuyerID:  c.BuyerID,
			Nickname: c.Nickname,
			Phone:    c.Phone,
		})
	}

	// 批次從 buyer-service 取 avatar
	if uc.buyerClient != nil && len(result) > 0 {
		ids := make([]int64, 0, len(result))
		for _, c := range result {
			ids = append(ids, c.BuyerID)
		}
		// ids 全是 crm_customers.buyer_id → 走 buyer_id 軸的批次 RPC。
		metaMap, err := uc.buyerClient.GetBuyerMetaBatchByBuyerIDs(ctx, ids)
		if err != nil {
			logger.Warn(ctx, "SearchCustomersByTags: failed to batch fetch buyer meta", zap.Error(err))
		} else {
			for _, c := range result {
				if meta, ok := metaMap[c.BuyerID]; ok {
					c.Avatar = meta.AvatarURL
					if meta.Nickname != "" {
						c.Nickname = meta.Nickname
					}
				}
			}
		}
	}

	return &dto.SearchCustomersByTagsResult{
		Total:     int32(len(result)),
		Customers: result,
	}, nil
}

// GetBuyerVipStatuses returns customer_type for a buyer across multiple sellers.
// RedactDeletedBuyerPII 抹除某買家在所有賣家 CRM 裡的個資快取（#2847）。
//
// 由 buyer.deleted 事件驅動。該事件在買家註銷的同一個 transaction 入 outbox，
// 而註銷時若買家**還有進行中訂單**，buyer-service 會導向 PENDING_DELETION 且
// **不發**此事件（usecase/delete_buyer_account.go 的 count>0 分支）——所以收到
// 事件時，賣家對這個買家已無任何操作需求，不需要為了保留窗口而延後清除。
//
// 🔴 這與 ListCustomers 那套「上游給不出值就保留舊值」的規則是兩件事：
// 那是為了不讓賣家的 keyword 搜尋失效；這裡是買家已行使刪除權，保留即違規。
//
// 回傳實際更動列數。0 不是錯誤——事件重放或本來就乾淨都會是 0。
func (uc *CrmUseCase) RedactDeletedBuyerPII(ctx context.Context, buyerID int64) (int64, error) {
	return uc.repo.RedactCustomerPIIByBuyerID(ctx, buyerID)
}

func (uc *CrmUseCase) GetBuyerVipStatuses(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]string, error) {
	return uc.repo.GetCustomerTypesByBuyer(ctx, buyerID, storeIDs)
}

// DTO mapping helpers

func toCustomerDTO(c *domain.CrmCustomer) *dto.Customer {
	return &dto.Customer{
		ID:           c.ID,
		StoreID:      c.StoreID,
		BuyerID:      c.BuyerID,
		CustomerType: string(c.CustomerType),
		TotalSpent:   c.TotalSpent,
		OrderCount:   c.OrderCount,
		Nickname:     c.Nickname,
		Phone:        c.Phone,
	}
}

func toTagDTO(t *domain.CrmTag) *dto.Tag {
	return &dto.Tag{
		ID:      t.ID,
		StoreID: t.StoreID,
		Name:    t.Name,
	}
}
