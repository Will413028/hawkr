package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
	"github.com/Will413028/hawkr/crm-service/internal/usecase/dto"
)

func TestCrmUseCase_GetCustomer(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		repo := &mockCrmRepo{
			GetCustomerFn: func(_ context.Context, storeID, buyerID int64) (*domain.CrmCustomer, error) {
				return &domain.CrmCustomer{
					ID:           1,
					StoreID:      storeID,
					BuyerID:      buyerID,
					CustomerType: domain.CustomerTypeVIP,
					TotalSpent:   5000,
					OrderCount:   10,
				}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		got, err := uc.GetCustomer(ctx, 100, 200)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != 1 {
			t.Errorf("want ID=1, got %d", got.ID)
		}
		if got.CustomerType != string(domain.CustomerTypeVIP) {
			t.Errorf("want customer_type=%s, got %s", domain.CustomerTypeVIP, got.CustomerType)
		}
		if got.TotalSpent != 5000 {
			t.Errorf("want TotalSpent=5000, got %d", got.TotalSpent)
		}
	})

	t.Run("resolves account_id from buyer_id for account-service; address queried directly with buyer_id", func(t *testing.T) {
		const (
			buyerID = int64(7484500295840492062)
			realAcc = int64(2627714723)
		)
		repo := &mockCrmRepo{
			GetCustomerFn: func(_ context.Context, storeID, id int64) (*domain.CrmCustomer, error) {
				return &domain.CrmCustomer{ID: 1, StoreID: storeID, BuyerID: id, Phone: "cached-phone"}, nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetProfileByBuyerIDFn: func(_ context.Context, id int64) (*BuyerProfile, error) {
				if id != buyerID {
					return nil, nil
				}
				return &BuyerProfile{AccountID: realAcc, Nickname: "分身買家", AvatarURL: "avatars/a.jpg"}, nil
			},
			GetDefaultAddressFn: func(_ context.Context, _ int64) (string, error) {
				return "台北市大安區某路1號", nil
			},
		}
		accountClient := &mockAccountClient{
			GetAccountInfoFn: func(_ context.Context, id int64) (*CustomerAccountInfo, error) {
				if id != realAcc {
					return &CustomerAccountInfo{}, nil
				}
				return &CustomerAccountInfo{Phone: "0912345678", Email: "a@b.c"}, nil
			},
		}
		uc := NewCrmUseCase(repo, accountClient, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		got, err := uc.GetCustomer(ctx, 100, buyerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buyerClient.lastProfileBuyerID != buyerID {
			t.Errorf("profile queried with %d, want buyer_id %d", buyerClient.lastProfileBuyerID, buyerID)
		}
		if accountClient.lastQueriedID != realAcc {
			t.Errorf("account-service queried with %d, want resolved account_id %d", accountClient.lastQueriedID, realAcc)
		}
		if buyerClient.lastAddressQueryID != buyerID {
			t.Errorf("address queried with %d, want buyer_id %d (not the resolved account_id %d)", buyerClient.lastAddressQueryID, buyerID, realAcc)
		}
		if got.Nickname != "分身買家" {
			t.Errorf("Nickname = %q, want %q", got.Nickname, "分身買家")
		}
		if got.Avatar != "avatars/a.jpg" {
			t.Errorf("Avatar = %q, want %q", got.Avatar, "avatars/a.jpg")
		}
		if got.Phone != "0912345678" {
			t.Errorf("Phone = %q, want %q", got.Phone, "0912345678")
		}
		if got.Address != "台北市大安區某路1號" {
			t.Errorf("Address = %q, want the resolved-account address", got.Address)
		}
	})

	// 釘住：上游回空欄位時不得覆寫 DB 快取值。這個情境是本輪 RecordOrder 的
	// phone fallback（account-service 失敗時改用 buyers.phone）帶出來的——
	// 買家以 OAuth 註冊、account 沒有登入電話，但 buyer profile 有聯絡電話，
	// 於是 crm_customers.phone 有值而 account-service 回空字串。少了守衛，
	// 顧客列表（讀 DB 快取）顯示號碼、顧客詳情（被覆寫）顯示空白。
	// nickname 同理：profile.Nickname 為空時要保留快取值。
	t.Run("empty upstream fields do not overwrite cached nickname or phone", func(t *testing.T) {
		const (
			buyerID = int64(7484500295840492062)
			realAcc = int64(2627714723)
		)
		repo := &mockCrmRepo{
			GetCustomerFn: func(_ context.Context, storeID, id int64) (*domain.CrmCustomer, error) {
				return &domain.CrmCustomer{
					ID: 1, StoreID: storeID, BuyerID: id,
					Nickname: "快取的名字", Phone: "0912111111",
				}, nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetProfileByBuyerIDFn: func(_ context.Context, _ int64) (*BuyerProfile, error) {
				// display_name 可為空（欄位 nullable）；avatar 照常回值。
				return &BuyerProfile{AccountID: realAcc, Nickname: "", AvatarURL: "avatars/a.jpg"}, nil
			},
		}
		accountClient := &mockAccountClient{
			GetAccountInfoFn: func(_ context.Context, _ int64) (*CustomerAccountInfo, error) {
				// OAuth 註冊、無登入電話：account-service 正常回應但 phone 為空。
				return &CustomerAccountInfo{Phone: "", Email: ""}, nil
			},
		}
		uc := NewCrmUseCase(repo, accountClient, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		got, err := uc.GetCustomer(ctx, 100, buyerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Phone != "0912111111" {
			t.Errorf("Phone = %q, want the cached %q kept — an empty account-service phone must not blank the detail view while the list view still shows it", got.Phone, "0912111111")
		}
		if got.Nickname != "快取的名字" {
			t.Errorf("Nickname = %q, want the cached value kept (same guard as ListCustomers / SearchCustomersByTags)", got.Nickname)
		}
		// avatar 沒有守衛，照常覆寫（與 ListCustomers 一致）。
		if got.Avatar != "avatars/a.jpg" {
			t.Errorf("Avatar = %q, want it refreshed regardless", got.Avatar)
		}
	})

	t.Run("profile miss leaves cached phone and skips account_id-axis calls", func(t *testing.T) {
		repo := &mockCrmRepo{
			GetCustomerFn: func(_ context.Context, storeID, id int64) (*domain.CrmCustomer, error) {
				return &domain.CrmCustomer{ID: 1, StoreID: storeID, BuyerID: id, Nickname: "cached", Phone: "cached-phone"}, nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetProfileByBuyerIDFn: func(_ context.Context, _ int64) (*BuyerProfile, error) {
				return nil, nil
			},
		}
		accountClient := &mockAccountClient{}
		uc := NewCrmUseCase(repo, accountClient, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		got, err := uc.GetCustomer(ctx, 100, 200)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if accountClient.calls != 0 {
			t.Errorf("account-service called %d times without a resolved account_id, want 0", accountClient.calls)
		}
		// 用呼叫次數而非 lastAddressQueryID，才分得出「沒呼叫」與「用 0 呼叫了」。
		if buyerClient.addressCalls != 0 {
			t.Errorf("address queried %d times without a resolved account_id, want 0 calls", buyerClient.addressCalls)
		}
		if got.Phone != "cached-phone" {
			t.Errorf("Phone = %q, want the DB-cached value preserved", got.Phone)
		}
		if got.Nickname != "cached" {
			t.Errorf("Nickname = %q, want the DB-cached value preserved", got.Nickname)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		want := errors.New("db error")
		repo := &mockCrmRepo{
			GetCustomerFn: func(_ context.Context, _, _ int64) (*domain.CrmCustomer, error) {
				return nil, want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.GetCustomer(ctx, 100, 200)
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_ListCustomers(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 2, nil
			},
			ListCustomersFn: func(_ context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, StoreID: params.StoreID, BuyerID: 201},
					{ID: 2, StoreID: params.StoreID, BuyerID: 202},
				}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.ListCustomers(ctx, dto.ListCustomersReq{
			StoreID: 100, Page: 1, PageSize: 20, CustomerType: "",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Total != 2 {
			t.Errorf("want Total=2, got %d", result.Total)
		}
		if len(result.List) != 2 {
			t.Errorf("want 2 customers, got %d", len(result.List))
		}
	})

	t.Run("enriches nickname and avatar from buyer-service", func(t *testing.T) {
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 2, nil
			},
			ListCustomersFn: func(_ context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, StoreID: params.StoreID, BuyerID: 201, Nickname: "cached1"},
					{ID: 2, StoreID: params.StoreID, BuyerID: 202, Nickname: "cached2"},
				}, nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetBuyerMetaBatchByBuyerIDsFn: func(_ context.Context, ids []int64) (map[int64]*BuyerMeta, error) {
				result := make(map[int64]*BuyerMeta, len(ids))
				result[201] = &BuyerMeta{Nickname: "Alice", AvatarURL: "https://img/alice.jpg"}
				result[202] = &BuyerMeta{Nickname: "Bob", AvatarURL: "https://img/bob.jpg"}
				return result, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100, Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.List[0].Nickname != "Alice" {
			t.Errorf("want Nickname='Alice', got '%s'", result.List[0].Nickname)
		}
		if result.List[0].Avatar != "https://img/alice.jpg" {
			t.Errorf("want Avatar='https://img/alice.jpg', got '%s'", result.List[0].Avatar)
		}
		if result.List[1].Nickname != "Bob" {
			t.Errorf("want Nickname='Bob', got '%s'", result.List[1].Nickname)
		}
	})

	// 與 SearchCustomersByTags 的 `if meta.Nickname != ""` 守衛對齊。
	// buyers.display_name 可為空（欄位 nullable，Buyer.UpdateProfile 也接受空
	// 字串），空值不得覆寫快取——否則同一個買家在標籤搜尋看得到名字、在顧客
	// 列表卻空白，而且 DB 會被洗成 '' 導致他從 keyword 搜尋中消失。
	t.Run("empty display_name does not overwrite cached nickname or write it back", func(t *testing.T) {
		writtenCh := make(chan []domain.CustomerCacheUpdate, 1)
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 1, nil
			},
			ListCustomersFn: func(_ context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, StoreID: params.StoreID, BuyerID: 201, Nickname: "快取的名字", Phone: "0912111111"},
				}, nil
			},
			BatchUpdateCustomerCacheFn: func(_ context.Context, _ int64, updates []domain.CustomerCacheUpdate) error {
				writtenCh <- updates
				return nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetBuyerMetaBatchByBuyerIDsFn: func(_ context.Context, _ []int64) (map[int64]*BuyerMeta, error) {
				// buyer-service 成功回應，但這個買家的 display_name 是空的。
				return map[int64]*BuyerMeta{201: {Nickname: "", AvatarURL: "avatars/a.jpg"}}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100, Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.List[0].Nickname != "快取的名字" {
			t.Errorf("Nickname = %q, want the cached value kept (same guard as SearchCustomersByTags)", result.List[0].Nickname)
		}
		// avatar 沒有這層守衛（空頭像是有效狀態，不影響搜尋），照常覆寫。
		if result.List[0].Avatar != "avatars/a.jpg" {
			t.Errorf("Avatar = %q, want it refreshed regardless", result.List[0].Avatar)
		}
		// 送進 buyer-service 的必須是 buyer_id。
		if len(buyerClient.metaBatchQueriedIDs) != 1 || buyerClient.metaBatchQueriedIDs[0] != 201 {
			t.Errorf("meta batch queried with %v, want [201] (buyer_id axis)", buyerClient.metaBatchQueriedIDs)
		}

		select {
		case updates := <-writtenCh:
			if len(updates) != 1 {
				t.Fatalf("want 1 cache update, got %d", len(updates))
			}
			if updates[0].Nickname != "快取的名字" {
				t.Errorf("write-back Nickname = %q, want the cached value — an empty display_name must not be persisted", updates[0].Nickname)
			}
		case <-time.After(time.Second):
			t.Fatal("write-back goroutine did not execute within 1s")
		}
	})

	t.Run("buyer-service failure degrades to cached values", func(t *testing.T) {
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 1, nil
			},
			ListCustomersFn: func(_ context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, StoreID: params.StoreID, BuyerID: 201, Nickname: "cached"},
				}, nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetBuyerMetaBatchByBuyerIDsFn: func(_ context.Context, _ []int64) (map[int64]*BuyerMeta, error) {
				return nil, errors.New("buyer-service unavailable")
			},
		}
		uc := NewCrmUseCase(repo, nil, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100, Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.List[0].Nickname != "cached" {
			t.Errorf("want Nickname='cached' (DB fallback), got '%s'", result.List[0].Nickname)
		}
		if result.List[0].Avatar != "" {
			t.Errorf("want Avatar='' (no cache), got '%s'", result.List[0].Avatar)
		}
	})

	t.Run("empty customer list skips enrichment", func(t *testing.T) {
		batchCalled := false
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 0, nil
			},
			ListCustomersFn: func(_ context.Context, _ domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{}, nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetBuyerMetaBatchByBuyerIDsFn: func(_ context.Context, _ []int64) (map[int64]*BuyerMeta, error) {
				batchCalled = true
				return map[int64]*BuyerMeta{}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100, Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.List) != 0 {
			t.Errorf("want empty list, got %d", len(result.List))
		}
		if batchCalled {
			t.Error("GetBuyerMetaBatchByBuyerIDs should not be called for empty list")
		}
	})

	t.Run("write-back updates DB cache after enrichment", func(t *testing.T) {
		writtenCh := make(chan []domain.CustomerCacheUpdate, 1)
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 2, nil
			},
			ListCustomersFn: func(_ context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, StoreID: params.StoreID, BuyerID: 201, Nickname: "old1", Phone: "+886900000001"},
					{ID: 2, StoreID: params.StoreID, BuyerID: 202, Nickname: "old2", Phone: "+886900000002"},
				}, nil
			},
			BatchUpdateCustomerCacheFn: func(_ context.Context, _ int64, updates []domain.CustomerCacheUpdate) error {
				writtenCh <- updates
				return nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetBuyerMetaBatchByBuyerIDsFn: func(_ context.Context, _ []int64) (map[int64]*BuyerMeta, error) {
				return map[int64]*BuyerMeta{
					201: {Nickname: "Alice", AvatarURL: "https://img/alice.jpg"},
					202: {Nickname: "Bob", AvatarURL: "https://img/bob.jpg"},
				}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100, Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		select {
		case updates := <-writtenCh:
			if len(updates) != 2 {
				t.Fatalf("want 2 cache updates, got %d", len(updates))
			}
			if updates[0].Nickname != "Alice" || updates[0].Phone != "+886900000001" {
				t.Errorf("want Alice/+886900000001, got %s/%s", updates[0].Nickname, updates[0].Phone)
			}
		case <-time.After(time.Second):
			t.Fatal("write-back goroutine did not execute within 1s")
		}
	})

	t.Run("write-back skipped when enrichment fails", func(t *testing.T) {
		writeBackCalled := false
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 1, nil
			},
			ListCustomersFn: func(_ context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, StoreID: params.StoreID, BuyerID: 201},
				}, nil
			},
			BatchUpdateCustomerCacheFn: func(_ context.Context, _ int64, _ []domain.CustomerCacheUpdate) error {
				writeBackCalled = true
				return nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetBuyerMetaBatchByBuyerIDsFn: func(_ context.Context, _ []int64) (map[int64]*BuyerMeta, error) {
				return nil, errors.New("buyer-service down")
			},
		}
		uc := NewCrmUseCase(repo, nil, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100, Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Give goroutine a chance to run (it shouldn't)
		time.Sleep(50 * time.Millisecond)
		if writeBackCalled {
			t.Error("write-back should not be called when enrichment fails")
		}
	})

	t.Run("write-back skipped for empty list", func(t *testing.T) {
		writeBackCalled := false
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 0, nil
			},
			ListCustomersFn: func(_ context.Context, _ domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{}, nil
			},
			BatchUpdateCustomerCacheFn: func(_ context.Context, _ int64, _ []domain.CustomerCacheUpdate) error {
				writeBackCalled = true
				return nil
			},
		}
		uc := NewCrmUseCase(repo, nil, &mockBuyerClient{}, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100, Page: 1, PageSize: 20})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		if writeBackCalled {
			t.Error("write-back should not be called for empty list")
		}
	})

	t.Run("count error", func(t *testing.T) {
		want := errors.New("count failed")
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 0, want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100})
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})

	t.Run("list error", func(t *testing.T) {
		want := errors.New("list failed")
		repo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 5, nil
			},
			ListCustomersFn: func(_ context.Context, _ domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return nil, want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.ListCustomers(ctx, dto.ListCustomersReq{StoreID: 100})
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_SearchCustomersByTags(t *testing.T) {
	ctx := context.Background()

	t.Run("success with buyer enrichment", func(t *testing.T) {
		repo := &mockCrmRepo{
			SearchCustomersByTagIdsFn: func(_ context.Context, storeID int64, tagIDs []int64) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, StoreID: storeID, BuyerID: 201, Nickname: "cached1", Phone: "0912-111-111"},
					{ID: 2, StoreID: storeID, BuyerID: 202, Nickname: "cached2", Phone: "0912-222-222"},
				}, nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetBuyerMetaBatchByBuyerIDsFn: func(_ context.Context, ids []int64) (map[int64]*BuyerMeta, error) {
				return map[int64]*BuyerMeta{
					201: {Nickname: "王小明", AvatarURL: "https://img/wang.jpg"},
					202: {Nickname: "李小華", AvatarURL: "https://img/li.jpg"},
				}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.SearchCustomersByTags(ctx, dto.SearchCustomersByTagsReq{
			StoreID: 100,
			TagIDs:  []int64{1, 2},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Total != 2 {
			t.Errorf("want Total=2, got %d", result.Total)
		}
		if result.Customers[0].Nickname != "王小明" {
			t.Errorf("want Nickname='王小明', got '%s'", result.Customers[0].Nickname)
		}
		if result.Customers[0].Avatar != "https://img/wang.jpg" {
			t.Errorf("want Avatar enriched, got '%s'", result.Customers[0].Avatar)
		}
		if result.Customers[0].Phone != "0912-111-111" {
			t.Errorf("want Phone='0912-111-111', got '%s'", result.Customers[0].Phone)
		}
	})

	t.Run("buyer-service failure degrades to cached values", func(t *testing.T) {
		repo := &mockCrmRepo{
			SearchCustomersByTagIdsFn: func(_ context.Context, _ int64, _ []int64) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, BuyerID: 201, Nickname: "cached", Phone: "0912-000-000"},
				}, nil
			},
		}
		buyerClient := &mockBuyerClient{
			GetBuyerMetaBatchByBuyerIDsFn: func(_ context.Context, _ []int64) (map[int64]*BuyerMeta, error) {
				return nil, errors.New("buyer-service unavailable")
			},
		}
		uc := NewCrmUseCase(repo, nil, buyerClient, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.SearchCustomersByTags(ctx, dto.SearchCustomersByTagsReq{StoreID: 100, TagIDs: []int64{1}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Customers[0].Nickname != "cached" {
			t.Errorf("want Nickname='cached' (DB fallback), got '%s'", result.Customers[0].Nickname)
		}
		if result.Customers[0].Avatar != "" {
			t.Errorf("want Avatar='' (no enrichment), got '%s'", result.Customers[0].Avatar)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		repo := &mockCrmRepo{
			SearchCustomersByTagIdsFn: func(_ context.Context, _ int64, _ []int64) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.SearchCustomersByTags(ctx, dto.SearchCustomersByTagsReq{StoreID: 100, TagIDs: []int64{1, 2}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Total != 0 {
			t.Errorf("want Total=0, got %d", result.Total)
		}
		if len(result.Customers) != 0 {
			t.Errorf("want empty customers, got %d", len(result.Customers))
		}
	})

	t.Run("repo error", func(t *testing.T) {
		want := errors.New("db error")
		repo := &mockCrmRepo{
			SearchCustomersByTagIdsFn: func(_ context.Context, _ int64, _ []int64) ([]*domain.CrmCustomer, error) {
				return nil, want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.SearchCustomersByTags(ctx, dto.SearchCustomersByTagsReq{StoreID: 100, TagIDs: []int64{1}})
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_UpdateCustomerType(t *testing.T) {
	ctx := context.Background()

	t.Run("blocklist - customer_type passed correctly", func(t *testing.T) {
		var gotType domain.CustomerType
		repo := &mockCrmRepo{
			UpdateCustomerTypeFn: func(_ context.Context, _, _ int64, customerType domain.CustomerType) error {
				gotType = customerType
				return nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.UpdateCustomerType(ctx, dto.UpdateCustomerTypeReq{
			StoreID:      100,
			BuyerID:      200,
			CustomerType: string(domain.CustomerTypeBlocklist),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotType != domain.CustomerTypeBlocklist {
			t.Errorf("want customer_type=%s, got %s", domain.CustomerTypeBlocklist, gotType)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		want := errors.New("update failed")
		repo := &mockCrmRepo{
			UpdateCustomerTypeFn: func(_ context.Context, _, _ int64, _ domain.CustomerType) error {
				return want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.UpdateCustomerType(ctx, dto.UpdateCustomerTypeReq{StoreID: 100, BuyerID: 200})
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_ListTags(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mockCrmRepo{
			ListTagsFn: func(_ context.Context, _ int64) ([]*domain.CrmTag, error) {
				return []*domain.CrmTag{
					{ID: 1, Name: "VIP"},
					{ID: 2, Name: "新客"},
				}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		tags, err := uc.ListTags(ctx, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tags) != 2 {
			t.Errorf("want 2 tags, got %d", len(tags))
		}
		if tags[0].Name != "VIP" {
			t.Errorf("want Name='VIP', got '%s'", tags[0].Name)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		repo := &mockCrmRepo{
			ListTagsFn: func(_ context.Context, _ int64) ([]*domain.CrmTag, error) {
				return []*domain.CrmTag{}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		tags, err := uc.ListTags(ctx, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tags) != 0 {
			t.Errorf("want empty list, got %d tags", len(tags))
		}
	})

	t.Run("repo error", func(t *testing.T) {
		want := errors.New("db error")
		repo := &mockCrmRepo{
			ListTagsFn: func(_ context.Context, _ int64) ([]*domain.CrmTag, error) {
				return nil, want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.ListTags(ctx, 100)
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_CreateTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success - fields set correctly", func(t *testing.T) {
		var saved *domain.CrmTag
		repo := &mockCrmRepo{
			CreateTagFn: func(_ context.Context, tag *domain.CrmTag) error {
				tag.ID = 42 // DB assigned
				saved = tag
				return nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		result, err := uc.CreateTag(ctx, dto.CreateTagReq{
			StoreID: 100,
			Name:    "老客戶",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if saved.StoreID != 100 {
			t.Errorf("want StoreID=100, got %d", saved.StoreID)
		}
		if saved.Name != "老客戶" {
			t.Errorf("want Name='老客戶', got '%s'", saved.Name)
		}
		if result.ID != 42 {
			t.Errorf("want ID=42, got %d", result.ID)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		want := errors.New("insert failed")
		repo := &mockCrmRepo{
			CreateTagFn: func(_ context.Context, _ *domain.CrmTag) error {
				return want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.CreateTag(ctx, dto.CreateTagReq{StoreID: 100, Name: "test"})
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_UpdateTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success - fields mapped correctly", func(t *testing.T) {
		var saved *domain.CrmTag
		repo := &mockCrmRepo{
			UpdateTagFn: func(_ context.Context, tag *domain.CrmTag) error {
				saved = tag
				return nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.UpdateTag(ctx, dto.UpdateTagReq{
			StoreID: 100,
			TagID:   5,
			Name:    "更新標籤",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if saved.ID != 5 {
			t.Errorf("want tag ID=5, got %d", saved.ID)
		}
		if saved.StoreID != 100 {
			t.Errorf("want StoreID=100, got %d", saved.StoreID)
		}
		if saved.Name != "更新標籤" {
			t.Errorf("want Name='更新標籤', got '%s'", saved.Name)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		want := errors.New("update failed")
		repo := &mockCrmRepo{
			UpdateTagFn: func(_ context.Context, _ *domain.CrmTag) error {
				return want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.UpdateTag(ctx, dto.UpdateTagReq{StoreID: 100, TagID: 5})
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_DeleteTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success - correct args forwarded", func(t *testing.T) {
		var gotStoreID, gotTagID int64
		repo := &mockCrmRepo{
			DeleteTagFn: func(_ context.Context, storeID, tagID int64) error {
				gotStoreID = storeID
				gotTagID = tagID
				return nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		if err := uc.DeleteTag(ctx, 100, 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotStoreID != 100 || gotTagID != 5 {
			t.Errorf("want DeleteTag(100,5), got (%d,%d)", gotStoreID, gotTagID)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		want := errors.New("delete failed")
		repo := &mockCrmRepo{
			DeleteTagFn: func(_ context.Context, _, _ int64) error { return want },
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		if err := uc.DeleteTag(ctx, 100, 5); !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_GetCustomerTags(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mockCrmRepo{
			GetCustomerTagsFn: func(_ context.Context, _, _ int64) ([]*domain.CrmTag, error) {
				return []*domain.CrmTag{
					{ID: 1, Name: "VIP"},
					{ID: 2, Name: "常客"},
				}, nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		tags, err := uc.GetCustomerTags(ctx, 100, 200)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tags) != 2 {
			t.Errorf("want 2 tags, got %d", len(tags))
		}
	})

	t.Run("repo error", func(t *testing.T) {
		want := errors.New("db error")
		repo := &mockCrmRepo{
			GetCustomerTagsFn: func(_ context.Context, _, _ int64) ([]*domain.CrmTag, error) {
				return nil, want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		_, err := uc.GetCustomerTags(ctx, 100, 200)
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}

func TestCrmUseCase_AddCustomerTags(t *testing.T) {
	ctx := context.Background()

	t.Run("existing tags found by name - all added", func(t *testing.T) {
		var addedIDs []int64
		repo := &mockCrmRepo{
			GetTagByNameFn: func(_ context.Context, _ int64, name string) (*domain.CrmTag, error) {
				return &domain.CrmTag{ID: 10, Name: name}, nil
			},
			AddCustomerTagFn: func(_ context.Context, _, _, tagID int64) error {
				addedIDs = append(addedIDs, tagID)
				return nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.AddCustomerTags(ctx, dto.AddCustomerTagsByNameReq{
			StoreID:  100,
			BuyerID:  200,
			TagNames: []string{"VIP", "奧客", "高消費"},
		})
		if err != nil {
			t.Fatalf("AddCustomerTags must always return nil, got: %v", err)
		}
		if len(addedIDs) != 3 {
			t.Errorf("want 3 AddCustomerTag calls, got %d", len(addedIDs))
		}
	})

	t.Run("tag not found - auto-created then added", func(t *testing.T) {
		var createCount, addCount int
		repo := &mockCrmRepo{
			GetTagByNameFn: func(_ context.Context, _ int64, _ string) (*domain.CrmTag, error) {
				return nil, errors.New("not found")
			},
			CreateTagFn: func(_ context.Context, tag *domain.CrmTag) error {
				createCount++
				tag.ID = int64(createCount)
				return nil
			},
			AddCustomerTagFn: func(_ context.Context, _, _, _ int64) error {
				addCount++
				return nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.AddCustomerTags(ctx, dto.AddCustomerTagsByNameReq{
			StoreID:  100,
			BuyerID:  200,
			TagNames: []string{"新標籤A", "新標籤B"},
		})
		if err != nil {
			t.Fatalf("AddCustomerTags must always return nil, got: %v", err)
		}
		if createCount != 2 {
			t.Errorf("want 2 tags created, got %d", createCount)
		}
		if addCount != 2 {
			t.Errorf("want 2 AddCustomerTag calls, got %d", addCount)
		}
	})

	t.Run("add error is logged and ignored - all tags still attempted", func(t *testing.T) {
		callCount := 0
		repo := &mockCrmRepo{
			GetTagByNameFn: func(_ context.Context, _ int64, name string) (*domain.CrmTag, error) {
				return &domain.CrmTag{ID: 10, Name: name}, nil
			},
			AddCustomerTagFn: func(_ context.Context, _, _, _ int64) error {
				callCount++
				return errors.New("duplicate key")
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.AddCustomerTags(ctx, dto.AddCustomerTagsByNameReq{
			StoreID:  100,
			BuyerID:  200,
			TagNames: []string{"A", "B"},
		})
		if err != nil {
			t.Errorf("want nil even on add error, got: %v", err)
		}
		if callCount != 2 {
			t.Errorf("want both tags attempted, got %d calls", callCount)
		}
	})

	t.Run("empty tag list returns nil", func(t *testing.T) {
		repo := &mockCrmRepo{}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.AddCustomerTags(ctx, dto.AddCustomerTagsByNameReq{
			StoreID: 100, BuyerID: 200, TagNames: []string{},
		})
		if err != nil {
			t.Errorf("want nil for empty tag list, got: %v", err)
		}
	})
}

func TestCrmUseCase_RemoveCustomerTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mockCrmRepo{
			RemoveCustomerTagFn: func(_ context.Context, _, _, _ int64) error {
				return nil
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.RemoveCustomerTag(ctx, dto.ManageCustomerTagReq{
			StoreID: 100,
			BuyerID: 200,
			TagID:   10,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("remove error is returned", func(t *testing.T) {
		want := errors.New("not found")
		repo := &mockCrmRepo{
			RemoveCustomerTagFn: func(_ context.Context, _, _, _ int64) error {
				return want
			},
		}
		uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, nil)

		err := uc.RemoveCustomerTag(ctx, dto.ManageCustomerTagReq{
			StoreID: 100, BuyerID: 200, TagID: 10,
		})
		if !errors.Is(err, want) {
			t.Errorf("want %v, got %v", want, err)
		}
	})
}
