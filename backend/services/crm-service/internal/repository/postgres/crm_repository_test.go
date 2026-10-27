//go:build integration

// 需要本機 Docker daemon（testcontainers 起 postgres 重放 db/migrations）。
// 跑法與 CI 接線見 setup_test.go 檔頭。
package postgres_test

import (
	"context"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
	"github.com/Will413028/hawkr/crm-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/go-pkg/database"
)

// ── Customer ──────────────────────────────────────────────────────────────────

func TestCrmRepository_CreateAndGetCustomer(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		wantID := nextTestID()
		c := &domain.CrmCustomer{
			ID:         wantID,
			StoreID:    9990001,
			BuyerID:    9990002,
			TotalSpent: 12000,
			OrderCount: 5,
		}
		if err := repo.CreateCustomer(ctx, c); err != nil {
			t.Errorf("CreateCustomer: %v", err)
			return
		}
		// NOTE: there is deliberately no assertion on c.ID here. Neither
		// `c.ID == 0` nor `c.ID != wantID` can fail: the id is supplied by
		// this test and CreateCustomer's only write to it is `c.ID = row.ID`
		// where row.ID is the same value echoed back by RETURNING. Deleting
		// that write-back line entirely would leave both assertions passing,
		// so they prove nothing. The id that IS worth asserting is the one
		// read back out of the database below.
		got, err := repo.GetCustomer(ctx, 9990001, 9990002)
		if err != nil {
			t.Errorf("GetCustomer: %v", err)
			return
		}
		if got.ID != wantID {
			t.Errorf("ID: got %d, want %d", got.ID, wantID)
		}
		if got.StoreID != 9990001 {
			t.Errorf("StoreID: got %d", got.StoreID)
		}
		if got.BuyerID != 9990002 {
			t.Errorf("BuyerID: got %d", got.BuyerID)
		}
		if got.TotalSpent != 12000 {
			t.Errorf("TotalSpent: got %d", got.TotalSpent)
		}
		if got.OrderCount != 5 {
			t.Errorf("OrderCount: got %d", got.OrderCount)
		}
	})
}

func TestCrmRepository_GetCustomer_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		_, err := repo.GetCustomer(ctx, -1, -1)
		if err == nil {
			t.Error("expected error, got nil")
			return
		}
		if !pkgerrors.IsNotFound(err) {
			t.Errorf("expected NotFound error, got %v", err)
		}
	})
}

func TestCrmRepository_ListAndCountCustomers(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID := int64(9990100)
		for _, customer := range []struct {
			buyerID    int64
			totalSpent int64
		}{
			{buyerID: 9990101, totalSpent: 100},
			{buyerID: 9990102, totalSpent: 200},
		} {
			c := &domain.CrmCustomer{ID: nextTestID(), StoreID: storeID, BuyerID: customer.buyerID, TotalSpent: customer.totalSpent}
			if err := repo.CreateCustomer(ctx, c); err != nil {
				t.Errorf("CreateCustomer(%d): %v", customer.buyerID, err)
				return
			}
		}

		params := domain.CustomerListParams{
			StoreID:      storeID,
			CustomerType: "", // all types
			Page:         1,
			PageSize:     20,
		}

		customers, err := repo.ListCustomers(ctx, params)
		if err != nil {
			t.Errorf("ListCustomers: %v", err)
			return
		}
		if len(customers) != 2 {
			t.Errorf("want 2 customers, got %d", len(customers))
		}

		defaultParams := params
		defaultParams.Page = 0
		defaultParams.PageSize = 0
		customers, err = repo.ListCustomers(ctx, defaultParams)
		if err != nil {
			t.Errorf("ListCustomers defaults: %v", err)
			return
		}
		if len(customers) != 2 {
			t.Errorf("default pagination: want 2 customers, got %d", len(customers))
		}

		pageTwoParams := params
		pageTwoParams.Page = 2
		pageTwoParams.PageSize = 1
		pageTwoParams.SortBy = "total_spent"
		pageTwoParams.SortOrder = "asc"
		customers, err = repo.ListCustomers(ctx, pageTwoParams)
		if err != nil {
			t.Errorf("ListCustomers page 2: %v", err)
			return
		}
		if len(customers) != 1 || customers[0].BuyerID != 9990102 {
			t.Errorf("page 2 pagination: got %v, want buyer 9990102", customers)
		}

		count, err := repo.CountCustomers(ctx, params)
		if err != nil {
			t.Errorf("CountCustomers: %v", err)
			return
		}
		if count != 2 {
			t.Errorf("want count=2, got %d", count)
		}
	})
}

func TestCrmRepository_ListCustomers_StatusFilter(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID := int64(9990200)

		c1 := &domain.CrmCustomer{ID: nextTestID(), StoreID: storeID, BuyerID: 9990201}
		if err := repo.CreateCustomer(ctx, c1); err != nil {
			t.Errorf("CreateCustomer normal: %v", err)
			return
		}

		c2 := &domain.CrmCustomer{ID: nextTestID(), StoreID: storeID, BuyerID: 9990202}
		if err := repo.CreateCustomer(ctx, c2); err != nil {
			t.Errorf("CreateCustomer vip: %v", err)
			return
		}
		if err := repo.UpdateCustomerType(ctx, storeID, 9990202, domain.CustomerTypeVIP); err != nil {
			t.Errorf("UpdateCustomerType: %v", err)
			return
		}

		params := domain.CustomerListParams{
			StoreID:      storeID,
			CustomerType: string(domain.CustomerTypeVIP),
			Page:         1,
			PageSize:     20,
		}

		customers, err := repo.ListCustomers(ctx, params)
		if err != nil {
			t.Errorf("ListCustomers: %v", err)
			return
		}
		if len(customers) != 1 {
			t.Errorf("want 1 VIP customer, got %d", len(customers))
			return
		}
		if customers[0].BuyerID != 9990202 {
			t.Errorf("BuyerID: got %d", customers[0].BuyerID)
		}
	})
}

func TestCrmRepository_UpdateCustomerType(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		c := &domain.CrmCustomer{ID: nextTestID(), StoreID: 9990300, BuyerID: 9990301}
		if err := repo.CreateCustomer(ctx, c); err != nil {
			t.Errorf("CreateCustomer: %v", err)
			return
		}

		if err := repo.UpdateCustomerType(ctx, 9990300, 9990301, domain.CustomerTypeBlocklist); err != nil {
			t.Errorf("UpdateCustomerType: %v", err)
			return
		}

		got, err := repo.GetCustomer(ctx, 9990300, 9990301)
		if err != nil {
			t.Errorf("GetCustomer: %v", err)
			return
		}
		if got.CustomerType != domain.CustomerTypeBlocklist {
			t.Errorf("CustomerType: want blocklist, got %s", got.CustomerType)
		}
	})
}

// ── Tags ──────────────────────────────────────────────────────────────────────

func TestCrmRepository_CreateAndGetTag(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		wantID := nextTestID()
		tag := &domain.CrmTag{
			ID:      wantID,
			StoreID: 9990500,
			Name:    "VIP",
		}
		if err := repo.CreateTag(ctx, tag); err != nil {
			t.Errorf("CreateTag: %v", err)
			return
		}
		// No assertion on tag.ID — same reasoning as
		// TestCrmRepository_CreateAndGetCustomer: the caller supplies the id
		// and RETURNING echoes it, so no assertion on the in-memory struct
		// can distinguish a working write-back from a deleted one. Assert on
		// the DB round-trip instead.
		got, err := repo.GetTag(ctx, wantID)
		if err != nil {
			t.Errorf("GetTag: %v", err)
			return
		}
		if got.ID != wantID {
			t.Errorf("ID: got %d, want %d", got.ID, wantID)
		}
		if got.Name != "VIP" {
			t.Errorf("Name: got %s", got.Name)
		}
		if got.StoreID != 9990500 {
			t.Errorf("StoreID: got %d", got.StoreID)
		}
	})
}

func TestCrmRepository_GetTag_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		_, err := repo.GetTag(ctx, -1)
		if !pkgerrors.IsNotFound(err) {
			t.Errorf("expected NotFound error, got %v", err)
		}
	})
}

// TestCrmRepository_ListTags checks that ListTags returns exactly the tags
// belonging to one store.
//
// It deliberately does NOT assert a specific order. This test used to expect
// alphabetical order with the comment "sorted by sort_order ASC", but
// crm_tags.sort_order was dropped in 20260223171304_drop_tag_unused_columns.sql
// and the query has ordered by created_at ASC ever since. All rows inserted by
// one transaction share the same created_at (Postgres NOW() is the transaction
// timestamp), so ListTags has no tiebreaker here and any positional assertion
// would be asserting an ordering the query does not actually guarantee.
func TestCrmRepository_ListTags(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID := int64(9990600)
		tags := []domain.CrmTag{
			{ID: nextTestID(), StoreID: storeID, Name: "C"},
			{ID: nextTestID(), StoreID: storeID, Name: "A"},
			{ID: nextTestID(), StoreID: storeID, Name: "B"},
		}
		for i := range tags {
			if err := repo.CreateTag(ctx, &tags[i]); err != nil {
				t.Errorf("CreateTag(%s): %v", tags[i].Name, err)
				return
			}
		}

		list, err := repo.ListTags(ctx, storeID)
		if err != nil {
			t.Errorf("ListTags: %v", err)
			return
		}
		if len(list) != 3 {
			t.Errorf("want 3 tags, got %d", len(list))
			return
		}
		got := map[string]bool{}
		for _, tag := range list {
			got[tag.Name] = true
			if tag.StoreID != storeID {
				t.Errorf("StoreID: got %d, want %d", tag.StoreID, storeID)
			}
		}
		for _, want := range []string{"A", "B", "C"} {
			if !got[want] {
				t.Errorf("tag %q missing from ListTags result", want)
			}
		}
	})
}

func TestCrmRepository_UpdateTag(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		tag := &domain.CrmTag{ID: nextTestID(), StoreID: 9990700, Name: "Old"}
		if err := repo.CreateTag(ctx, tag); err != nil {
			t.Errorf("CreateTag: %v", err)
			return
		}

		tag.Name = "New"
		if err := repo.UpdateTag(ctx, tag); err != nil {
			t.Errorf("UpdateTag: %v", err)
			return
		}

		got, err := repo.GetTag(ctx, tag.ID)
		if err != nil {
			t.Errorf("GetTag: %v", err)
			return
		}
		if got.Name != "New" {
			t.Errorf("Name: want 'New', got '%s'", got.Name)
		}
	})
}

func TestCrmRepository_DeleteTag(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		tag := &domain.CrmTag{ID: nextTestID(), StoreID: 9990800, Name: "ToDelete"}
		if err := repo.CreateTag(ctx, tag); err != nil {
			t.Errorf("CreateTag: %v", err)
			return
		}

		if err := repo.DeleteTag(ctx, 9990800, tag.ID); err != nil {
			t.Errorf("DeleteTag: %v", err)
			return
		}

		_, err := repo.GetTag(ctx, tag.ID)
		if !pkgerrors.IsNotFound(err) {
			t.Errorf("expected NotFound after delete, got %v", err)
		}
	})
}

// ── Customer-Tag relations ────────────────────────────────────────────────────

func TestCrmRepository_AddGetRemoveCustomerTag(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID := int64(9991000)
		customerID := int64(9991001)

		tag := &domain.CrmTag{ID: nextTestID(), StoreID: storeID, Name: "TestTag"}
		if err := repo.CreateTag(ctx, tag); err != nil {
			t.Errorf("CreateTag: %v", err)
			return
		}
		c := &domain.CrmCustomer{ID: nextTestID(), StoreID: storeID, BuyerID: customerID}
		if err := repo.CreateCustomer(ctx, c); err != nil {
			t.Errorf("CreateCustomer: %v", err)
			return
		}

		if err := repo.AddCustomerTag(ctx, storeID, customerID, tag.ID); err != nil {
			t.Errorf("AddCustomerTag: %v", err)
			return
		}

		tags, err := repo.GetCustomerTags(ctx, storeID, customerID)
		if err != nil {
			t.Errorf("GetCustomerTags: %v", err)
			return
		}
		if len(tags) != 1 {
			t.Errorf("want 1 tag, got %d", len(tags))
			return
		}
		if tags[0].ID != tag.ID {
			t.Errorf("tag ID: want %d, got %d", tag.ID, tags[0].ID)
		}

		if err := repo.RemoveCustomerTag(ctx, storeID, customerID, tag.ID); err != nil {
			t.Errorf("RemoveCustomerTag: %v", err)
			return
		}

		tags, err = repo.GetCustomerTags(ctx, storeID, customerID)
		if err != nil {
			t.Errorf("GetCustomerTags after remove: %v", err)
			return
		}
		if len(tags) != 0 {
			t.Errorf("want 0 tags after remove, got %d", len(tags))
		}
	})
}

func TestCrmRepository_AddCustomerTag_Idempotent(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID := int64(9991100)
		customerID := int64(9991101)

		tag := &domain.CrmTag{ID: nextTestID(), StoreID: storeID, Name: "Tag"}
		if err := repo.CreateTag(ctx, tag); err != nil {
			t.Errorf("CreateTag: %v", err)
			return
		}
		c := &domain.CrmCustomer{ID: nextTestID(), StoreID: storeID, BuyerID: customerID}
		if err := repo.CreateCustomer(ctx, c); err != nil {
			t.Errorf("CreateCustomer: %v", err)
			return
		}

		// ON CONFLICT DO NOTHING — adding twice should not error
		if err := repo.AddCustomerTag(ctx, storeID, customerID, tag.ID); err != nil {
			t.Errorf("first AddCustomerTag: %v", err)
			return
		}
		if err := repo.AddCustomerTag(ctx, storeID, customerID, tag.ID); err != nil {
			t.Errorf("second AddCustomerTag: %v", err)
			return
		}

		tags, err := repo.GetCustomerTags(ctx, storeID, customerID)
		if err != nil {
			t.Errorf("GetCustomerTags: %v", err)
			return
		}
		if len(tags) != 1 {
			t.Errorf("want 1 tag (idempotent), got %d", len(tags))
		}
	})
}

// ── Customer Ledger ───────────────────────────────────────────────────────────

func TestCrmRepository_AppendLedgerEntry_Idempotent(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		entry := domain.LedgerEntry{
			EventID:     "evt-test-001",
			EventType:   "order_completed",
			OrderNo:     "ORD-TEST-001",
			StoreID:     9992001,
			BuyerID:     9992002,
			AmountDelta: 1000,
			CountDelta:  1,
		}

		// First insert for this event_id → inserted=true.
		inserted, err := repo.AppendLedgerEntry(ctx, entry)
		if err != nil {
			t.Fatalf("first AppendLedgerEntry: %v", err)
		}
		if !inserted {
			t.Error("expected first insert to report inserted=true")
		}

		// Same event_id redelivered (MQ at-least-once) → ON CONFLICT DO NOTHING,
		// inserted=false — the signal callers use to skip duplicate accumulation.
		inserted, err = repo.AppendLedgerEntry(ctx, entry)
		if err != nil {
			t.Fatalf("second AppendLedgerEntry: %v", err)
		}
		if inserted {
			t.Error("expected duplicate event_id insert to report inserted=false")
		}
	})
}

func TestCrmRepository_ApplyRefundDelta(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID, buyerID := int64(9992001), int64(9992002)
		c := &domain.CrmCustomer{ID: nextTestID(), StoreID: storeID, BuyerID: buyerID, TotalSpent: 1000, OrderCount: 1}
		if err := repo.CreateCustomer(ctx, c); err != nil {
			t.Fatalf("CreateCustomer: %v", err)
		}

		rows, err := repo.ApplyRefundDelta(ctx, 9992099, storeID, buyerID, -400)
		if err != nil {
			t.Fatalf("ApplyRefundDelta: %v", err)
		}
		if rows != 1 {
			t.Errorf("expected 1 row affected, got %d", rows)
		}

		got, err := repo.GetCustomer(ctx, storeID, buyerID)
		if err != nil {
			t.Fatalf("GetCustomer: %v", err)
		}
		if got.TotalSpent != 600 {
			t.Errorf("expected TotalSpent=600 after refund, got %d", got.TotalSpent)
		}
	})
}

func TestCrmRepository_ApplyRefundDelta_CreatesRowWhenCustomerNotFound(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		// No prior CreateCustomer for this (storeID, buyerID) pair —
		// ApplyRefundDelta now upserts, creating a temporary negative-balance
		// row (order_count=0) instead of the old "0 rows affected = anomaly"
		// behavior. This is the real, reachable refund-before-order.completed
		// race (see spec Background); regression-tested at the usecase layer
		// by TestRefundBeforeOrderCompleted_ThenOrderArrives_ConvergesCorrectly.
		id, storeID, buyerID := int64(9992099), int64(-1), int64(-1)
		rows, err := repo.ApplyRefundDelta(ctx, id, storeID, buyerID, -400)
		if err != nil {
			t.Fatalf("ApplyRefundDelta: %v", err)
		}
		if rows != 1 {
			t.Errorf("expected 1 row affected (insert branch of upsert), got %d", rows)
		}

		got, err := repo.GetCustomer(ctx, storeID, buyerID)
		if err != nil {
			t.Fatalf("GetCustomer: %v", err)
		}
		if got.ID != id {
			t.Errorf("expected inserted row id=%d, got %d", id, got.ID)
		}
		if got.TotalSpent != -400 {
			t.Errorf("expected TotalSpent=-400 for refund-created row, got %d", got.TotalSpent)
		}
		if got.OrderCount != 0 {
			t.Errorf("expected OrderCount=0 for refund-created row, got %d", got.OrderCount)
		}
	})
}

// TestCrmRepository_UpsertCustomerOnOrder covers both branches of
// UpsertCustomerOnOrder's ON CONFLICT (store_id, buyer_id) DO UPDATE — the
// clause that accumulates total_spent/order_count for every completed order.
// The conflict target must match crm_customers_store_id_buyer_id_key; if it
// does not, Postgres rejects the statement at runtime and no compile-time
// check catches it.
func TestCrmRepository_UpsertCustomerOnOrder(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID, buyerID := int64(9993001), int64(9993002)

		// Insert branch: no existing row for this (store_id, buyer_id).
		firstID := nextTestID()
		if err := repo.UpsertCustomerOnOrder(ctx, firstID, storeID, buyerID, 1500, "小明", "0912345678"); err != nil {
			t.Fatalf("first UpsertCustomerOnOrder: %v", err)
		}
		got, err := repo.GetCustomer(ctx, storeID, buyerID)
		if err != nil {
			t.Fatalf("GetCustomer after insert: %v", err)
		}
		if got.ID != firstID {
			t.Errorf("expected inserted row id=%d, got %d", firstID, got.ID)
		}
		if got.TotalSpent != 1500 {
			t.Errorf("expected TotalSpent=1500 after first order, got %d", got.TotalSpent)
		}
		if got.OrderCount != 1 {
			t.Errorf("expected OrderCount=1 after first order, got %d", got.OrderCount)
		}

		// Conflict branch: same (store_id, buyer_id) accumulates rather than
		// inserting a second row or erroring. The id passed here is only used
		// by the insert branch, so the row must keep firstID.
		if err := repo.UpsertCustomerOnOrder(ctx, nextTestID(), storeID, buyerID, 400, "小明改", "0900000000"); err != nil {
			t.Fatalf("second UpsertCustomerOnOrder: %v", err)
		}
		got, err = repo.GetCustomer(ctx, storeID, buyerID)
		if err != nil {
			t.Fatalf("GetCustomer after upsert: %v", err)
		}
		if got.ID != firstID {
			t.Errorf("conflict branch must keep the original row id=%d, got %d", firstID, got.ID)
		}
		if got.TotalSpent != 1900 {
			t.Errorf("expected TotalSpent=1900 (1500+400) after second order, got %d", got.TotalSpent)
		}
		if got.OrderCount != 2 {
			t.Errorf("expected OrderCount=2 after second order, got %d", got.OrderCount)
		}
		if got.Nickname != "小明改" {
			t.Errorf("conflict branch should refresh nickname from EXCLUDED, got %q", got.Nickname)
		}
		// phone 也要被非空的新值刷新 — 這條與
		// TestCrmRepository_UpsertCustomerOnOrder_EmptyEnrichmentKeepsCachedValues
		// 是一對：COALESCE(NULLIF(...)) 必須擋住空值、但不能擋住真值。
		if got.Phone != "0900000000" {
			t.Errorf("conflict branch should refresh phone from EXCLUDED, got %q", got.Phone)
		}
	})
}

// TestCrmRepository_UpsertCustomerOnOrder_EmptyEnrichmentKeepsCachedValues
// 釘住 UpsertCustomerOnOrder 的 ON CONFLICT 對「空字串」的處理。
//
// nickname / phone 是 best-effort enrichment：RecordOrder 拿不到 buyer-service
// / account-service 的資料時傳空字串下來。若 ON CONFLICT 無條件
// `nickname = EXCLUDED.nickname`，一次上游短暫故障就會把賣家後台**可搜尋**的
// 顧客姓名洗掉——ListCustomers / CountCustomers 的 keyword 條件正是
// `c.nickname ILIKE ... OR c.phone ILIKE ...`（crm_customers.sql:10, :27），
// 該顧客會直接從搜尋結果消失。
//
// 這支同時斷言 total_spent / order_count 有正常累加，用來區分「保留舊值」與
// 「整句 upsert 沒生效」——只斷言舊值還在的話，一個壞掉到完全不執行的
// statement 也會過。
func TestCrmRepository_UpsertCustomerOnOrder_EmptyEnrichmentKeepsCachedValues(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID, buyerID := int64(9993101), int64(9993102)

		if err := repo.UpsertCustomerOnOrder(ctx, nextTestID(), storeID, buyerID, 1000, "小美", "0987654321"); err != nil {
			t.Fatalf("first UpsertCustomerOnOrder: %v", err)
		}

		// 第二筆訂單，enrichment 全空（模擬 buyer-service / account-service 故障）。
		if err := repo.UpsertCustomerOnOrder(ctx, nextTestID(), storeID, buyerID, 500, "", ""); err != nil {
			t.Fatalf("second UpsertCustomerOnOrder (empty enrichment): %v", err)
		}

		got, err := repo.GetCustomer(ctx, storeID, buyerID)
		if err != nil {
			t.Fatalf("GetCustomer: %v", err)
		}
		if got.Nickname != "小美" {
			t.Errorf("Nickname = %q, want %q kept — empty enrichment must not wipe the searchable name", got.Nickname, "小美")
		}
		if got.Phone != "0987654321" {
			t.Errorf("Phone = %q, want %q kept — empty enrichment must not wipe the searchable phone", got.Phone, "0987654321")
		}
		// 累加仍要發生，否則上面兩條「舊值還在」是因為整句沒跑，不是因為保留邏輯。
		if got.TotalSpent != 1500 {
			t.Errorf("TotalSpent = %d, want 1500 (1000+500) — the upsert itself must still apply", got.TotalSpent)
		}
		if got.OrderCount != 2 {
			t.Errorf("OrderCount = %d, want 2 — the upsert itself must still apply", got.OrderCount)
		}
	})
}

func TestCrmRepository_ListCustomers_TagFilter(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)

	runInTx(t, pool, func(ctx context.Context) {
		storeID := int64(9991200)

		tag := &domain.CrmTag{ID: nextTestID(), StoreID: storeID, Name: "Tagged"}
		if err := repo.CreateTag(ctx, tag); err != nil {
			t.Errorf("CreateTag: %v", err)
			return
		}

		cA := &domain.CrmCustomer{ID: nextTestID(), StoreID: storeID, BuyerID: 9991201}
		if err := repo.CreateCustomer(ctx, cA); err != nil {
			t.Errorf("CreateCustomer A: %v", err)
			return
		}
		if err := repo.AddCustomerTag(ctx, storeID, 9991201, tag.ID); err != nil {
			t.Errorf("AddCustomerTag: %v", err)
			return
		}

		cB := &domain.CrmCustomer{ID: nextTestID(), StoreID: storeID, BuyerID: 9991202}
		if err := repo.CreateCustomer(ctx, cB); err != nil {
			t.Errorf("CreateCustomer B: %v", err)
			return
		}

		params := domain.CustomerListParams{
			StoreID:      storeID,
			CustomerType: "",
			TagID:        tag.ID,
			Page:         1,
			PageSize:     20,
		}
		customers, err := repo.ListCustomers(ctx, params)
		if err != nil {
			t.Errorf("ListCustomers: %v", err)
			return
		}
		if len(customers) != 1 {
			t.Errorf("want 1 tagged customer, got %d", len(customers))
			return
		}
		if customers[0].BuyerID != 9991201 {
			t.Errorf("BuyerID: got %d", customers[0].BuyerID)
		}
	})
}

// ── Advisory Lock ─────────────────────────────────────────────────────────────

// TestCrmRepository_LockOrderForNetting_SerializesConcurrentCallers proves the
// pg_advisory_xact_lock backing LockOrderForNetting genuinely blocks a second,
// concurrent transaction for the same order_no. This is the one property this
// task's fix depends on that no mock-based test can verify — a mock cannot
// simulate a second real Postgres backend blocking on a session/transaction
// -scoped advisory lock. Deliberately does NOT use runInTx (which auto-
// rolls-back a single transaction): this test needs two independently
// committing transactions, each on its own connection, with explicit control
// over when each commits, so it drives pool.Begin/Commit directly via
// database.NewTxManager instead.
func TestCrmRepository_LockOrderForNetting_SerializesConcurrentCallers(t *testing.T) {
	pool := testPool(t)
	repo := postgres.NewCrmRepository(pool)
	txm := database.NewTxManager(pool)

	const orderNo = "O-lock-test"

	aLocked := make(chan struct{})
	releaseA := make(chan struct{})
	doneA := make(chan error, 1)

	// Transaction A: acquire the advisory lock and hold the transaction open
	// (uncommitted) until the test explicitly releases it.
	go func() {
		doneA <- txm.WithTx(context.Background(), func(ctx context.Context) error {
			if err := repo.LockOrderForNetting(ctx, orderNo); err != nil {
				return err
			}
			close(aLocked)
			<-releaseA
			return nil
		})
	}()

	select {
	case <-aLocked:
	case <-time.After(5 * time.Second):
		t.Fatal("transaction A never acquired the lock")
	}

	// Transaction B: from a separate goroutine/connection, attempt the same
	// lock while A still holds it — this call must block.
	doneB := make(chan error, 1)
	go func() {
		doneB <- txm.WithTx(context.Background(), func(ctx context.Context) error {
			return repo.LockOrderForNetting(ctx, orderNo)
		})
	}()

	select {
	case err := <-doneB:
		t.Fatalf("expected LockOrderForNetting to block while transaction A holds the lock, but it returned immediately (err=%v) — the lock is not serializing", err)
	case <-time.After(200 * time.Millisecond):
		// Expected: B is still blocked behind A's advisory lock.
	}

	// Release A (commit) — B should now be able to proceed promptly.
	close(releaseA)
	if err := <-doneA; err != nil {
		t.Fatalf("transaction A failed: %v", err)
	}

	select {
	case err := <-doneB:
		if err != nil {
			t.Fatalf("transaction B failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected transaction B to acquire the lock promptly after A released it, but it did not")
	}
}
