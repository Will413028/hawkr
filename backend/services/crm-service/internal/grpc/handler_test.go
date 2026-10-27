package grpc_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"go.uber.org/zap"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
	crmgrpc "github.com/Will413028/hawkr/crm-service/internal/grpc"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/crm"
)

// ── Mock repos ────────────────────────────────────────────────────────────────

type mockCrmRepo struct {
	GetCustomerFn             func(ctx context.Context, storeID, buyerID int64) (*domain.CrmCustomer, error)
	ListCustomersFn           func(ctx context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error)
	CountCustomersFn          func(ctx context.Context, params domain.CustomerListParams) (int64, error)
	CreateCustomerFn          func(ctx context.Context, customer *domain.CrmCustomer) error
	UpdateCustomerTypeFn      func(ctx context.Context, storeID, buyerID int64, customerType domain.CustomerType) error
	UpsertCustomerOnOrderFn   func(ctx context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error
	GetTagFn                  func(ctx context.Context, id int64) (*domain.CrmTag, error)
	GetTagByNameFn            func(ctx context.Context, storeID int64, name string) (*domain.CrmTag, error)
	ListTagsFn                func(ctx context.Context, storeID int64) ([]*domain.CrmTag, error)
	CreateTagFn               func(ctx context.Context, tag *domain.CrmTag) error
	UpdateTagFn               func(ctx context.Context, tag *domain.CrmTag) error
	DeleteTagFn               func(ctx context.Context, storeID, tagID int64) error
	GetCustomerTagsFn         func(ctx context.Context, storeID, buyerID int64) ([]*domain.CrmTag, error)
	AddCustomerTagFn          func(ctx context.Context, storeID, buyerID, tagID int64) error
	RemoveCustomerTagFn       func(ctx context.Context, storeID, buyerID, tagID int64) error
	GetCustomerTypesByBuyerFn func(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]string, error)
}

func (m *mockCrmRepo) GetCustomer(ctx context.Context, storeID, buyerID int64) (*domain.CrmCustomer, error) {
	return m.GetCustomerFn(ctx, storeID, buyerID)
}
func (m *mockCrmRepo) ListCustomers(ctx context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
	return m.ListCustomersFn(ctx, params)
}
func (m *mockCrmRepo) CountCustomers(ctx context.Context, params domain.CustomerListParams) (int64, error) {
	return m.CountCustomersFn(ctx, params)
}
func (m *mockCrmRepo) CreateCustomer(ctx context.Context, customer *domain.CrmCustomer) error {
	return m.CreateCustomerFn(ctx, customer)
}
func (m *mockCrmRepo) UpdateCustomerType(ctx context.Context, storeID, buyerID int64, customerType domain.CustomerType) error {
	return m.UpdateCustomerTypeFn(ctx, storeID, buyerID, customerType)
}
func (m *mockCrmRepo) GetTag(ctx context.Context, id int64) (*domain.CrmTag, error) {
	return m.GetTagFn(ctx, id)
}
func (m *mockCrmRepo) ListTags(ctx context.Context, storeID int64) ([]*domain.CrmTag, error) {
	return m.ListTagsFn(ctx, storeID)
}
func (m *mockCrmRepo) CreateTag(ctx context.Context, tag *domain.CrmTag) error {
	return m.CreateTagFn(ctx, tag)
}
func (m *mockCrmRepo) UpdateTag(ctx context.Context, tag *domain.CrmTag) error {
	return m.UpdateTagFn(ctx, tag)
}
func (m *mockCrmRepo) DeleteTag(ctx context.Context, storeID, tagID int64) error {
	return m.DeleteTagFn(ctx, storeID, tagID)
}
func (m *mockCrmRepo) GetTagByName(ctx context.Context, storeID int64, name string) (*domain.CrmTag, error) {
	return m.GetTagByNameFn(ctx, storeID, name)
}
func (m *mockCrmRepo) GetCustomerTags(ctx context.Context, storeID, buyerID int64) ([]*domain.CrmTag, error) {
	if m.GetCustomerTagsFn == nil {
		return nil, nil
	}
	return m.GetCustomerTagsFn(ctx, storeID, buyerID)
}
func (m *mockCrmRepo) AddCustomerTag(ctx context.Context, storeID, buyerID, tagID int64) error {
	return m.AddCustomerTagFn(ctx, storeID, buyerID, tagID)
}
func (m *mockCrmRepo) RemoveCustomerTag(ctx context.Context, storeID, buyerID, tagID int64) error {
	return m.RemoveCustomerTagFn(ctx, storeID, buyerID, tagID)
}
func (m *mockCrmRepo) UpsertCustomerOnOrder(ctx context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error {
	if m.UpsertCustomerOnOrderFn == nil {
		return nil
	}
	return m.UpsertCustomerOnOrderFn(ctx, id, storeID, buyerID, amount, nickname, phone)
}
func (m *mockCrmRepo) BatchUpdateCustomerCache(_ context.Context, _ int64, _ []domain.CustomerCacheUpdate) error {
	return nil
}
func (m *mockCrmRepo) AppendLedgerEntry(_ context.Context, _ domain.LedgerEntry) (bool, error) {
	return true, nil
}
func (m *mockCrmRepo) ApplyRefundDelta(_ context.Context, _, _, _, _ int64) (int64, error) {
	return 1, nil
}
func (m *mockCrmRepo) RedactCustomerPIIByBuyerID(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (m *mockCrmRepo) SumLedgerAmountForOrder(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (m *mockCrmRepo) HasFullRefundEntry(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (m *mockCrmRepo) LockOrderForNetting(_ context.Context, _ string) error {
	return nil
}
func (m *mockCrmRepo) GetCustomerTypesByBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]string, error) {
	if m.GetCustomerTypesByBuyerFn == nil {
		return nil, nil
	}
	return m.GetCustomerTypesByBuyerFn(ctx, buyerID, storeIDs)
}
func (m *mockCrmRepo) SearchCustomersByTagIds(ctx context.Context, storeID int64, tagIDs []int64) ([]*domain.CrmCustomer, error) {
	return nil, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newTestServer(t *testing.T, crmRepo domain.CrmRepository) (pb.CrmServiceClient, func()) {
	t.Helper()
	logger := zap.NewNop()
	crmUC := usecase.NewCrmUseCase(crmRepo, nil, nil, nil, nil, func() int64 { return 1 }, nil)
	srv := crmgrpc.NewCrmServer(crmUC, logger)

	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := googlegrpc.NewServer(
		googlegrpc.UnaryInterceptor(interceptor.ErrorInterceptor()),
	)
	pb.RegisterCrmServiceServer(grpcSrv, srv)
	go func() { _ = grpcSrv.Serve(lis) }()

	conn, err := googlegrpc.NewClient("passthrough:///bufnet",
		googlegrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}
	return pb.NewCrmServiceClient(conn), func() {
		_ = conn.Close()
		grpcSrv.Stop()
	}
}

// ── Customer tests ────────────────────────────────────────────────────────────

func TestHandler_GetCustomerList(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 3, nil
			},
			ListCustomersFn: func(_ context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
				return []*domain.CrmCustomer{
					{ID: 1, StoreID: params.StoreID, BuyerID: 201},
					{ID: 2, StoreID: params.StoreID, BuyerID: 202},
				}, nil
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.GetCustomerList(ctx, &pb.GetCustomerListRequest{
			StoreId: "100", Page: 1, PageSize: 20, CustomerType: "",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("want code=0, got %d", resp.Code)
		}
		if resp.Data.Total != "3" {
			t.Errorf("want total='3', got %s", resp.Data.Total)
		}
		if len(resp.Data.List) != 2 {
			t.Errorf("want 2 customers, got %d", len(resp.Data.List))
		}
	})

	t.Run("usecase error returns gRPC error", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			CountCustomersFn: func(_ context.Context, _ domain.CustomerListParams) (int64, error) {
				return 0, errors.New("db connection failed")
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		_, err := client.GetCustomerList(ctx, &pb.GetCustomerListRequest{StoreId: "100"})
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})
}

func TestHandler_GetCustomerStats(t *testing.T) {
	ctx := context.Background()

	t.Run("success - includes tags", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			GetCustomerFn: func(_ context.Context, storeID, buyerID int64) (*domain.CrmCustomer, error) {
				return &domain.CrmCustomer{
					ID:           1,
					StoreID:      storeID,
					BuyerID:      buyerID,
					CustomerType: domain.CustomerTypeVIP,
					TotalSpent:   12000,
					OrderCount:   5,
				}, nil
			},
			GetCustomerTagsFn: func(_ context.Context, _, _ int64) ([]*domain.CrmTag, error) {
				return []*domain.CrmTag{{ID: 1, Name: "VIP"}}, nil
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.GetCustomerStats(ctx, &pb.GetCustomerStatsRequest{
			StoreId: "100", BuyerId: "200",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Data.TotalSpent != "12000" {
			t.Errorf("want TotalSpent='12000', got %s", resp.Data.TotalSpent)
		}
		if len(resp.Data.Tags) != 1 {
			t.Errorf("want 1 tag, got %d", len(resp.Data.Tags))
		}
		if resp.Data.Tags[0].Name != "VIP" {
			t.Errorf("want tag name='VIP', got '%s'", resp.Data.Tags[0].Name)
		}
	})

	t.Run("tag fetch error is ignored - still returns customer", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			GetCustomerFn: func(_ context.Context, storeID, buyerID int64) (*domain.CrmCustomer, error) {
				return &domain.CrmCustomer{ID: 1, StoreID: storeID, BuyerID: buyerID}, nil
			},
			GetCustomerTagsFn: func(_ context.Context, _, _ int64) ([]*domain.CrmTag, error) {
				return nil, errors.New("tags unavailable")
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.GetCustomerStats(ctx, &pb.GetCustomerStatsRequest{StoreId: "100", BuyerId: "200"})
		if err != nil {
			t.Fatalf("tag error should be ignored, got: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("want code=0, got %d", resp.Code)
		}
	})
}

func TestHandler_UpdateCustomerStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		var gotType domain.CustomerType
		crmRepo := &mockCrmRepo{
			UpdateCustomerTypeFn: func(_ context.Context, _, _ int64, customerType domain.CustomerType) error {
				gotType = customerType
				return nil
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.UpdateCustomerStatus(ctx, &pb.UpdateCustomerStatusRequest{
			StoreId: "100", BuyerId: "200", CustomerType: "blocklist",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("want code=0, got %d", resp.Code)
		}
		if gotType != domain.CustomerTypeBlocklist {
			t.Errorf("want Blocklist customer_type, got %s", gotType)
		}
	})
}

// ── Tag tests ─────────────────────────────────────────────────────────────────
func TestHandler_CreateTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			CreateTagFn: func(_ context.Context, tag *domain.CrmTag) error {
				tag.ID = 99
				return nil
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.CreateTag(ctx, &pb.CreateTagRequest{
			StoreId: "100", Name: "高消費",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Data.Id != "99" {
			t.Errorf("want tag id='99', got %s", resp.Data.Id)
		}
		if resp.Data.Name != "高消費" {
			t.Errorf("want name='高消費', got '%s'", resp.Data.Name)
		}
	})

	t.Run("repo error returns gRPC error", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			CreateTagFn: func(_ context.Context, _ *domain.CrmTag) error {
				return errors.New("insert failed")
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		_, err := client.CreateTag(ctx, &pb.CreateTagRequest{StoreId: "100", Name: "test"})
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})
}

func TestHandler_ListTags(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			ListTagsFn: func(_ context.Context, _ int64) ([]*domain.CrmTag, error) {
				return []*domain.CrmTag{
					{ID: 1, Name: "A"},
					{ID: 2, Name: "B"},
				}, nil
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.ListTags(ctx, &pb.ListTagsRequest{StoreId: "100"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Data) != 2 {
			t.Errorf("want 2 tags, got %d", len(resp.Data))
		}
	})
}

func TestHandler_UpdateTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			UpdateTagFn: func(_ context.Context, _ *domain.CrmTag) error { return nil },
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.UpdateTag(ctx, &pb.UpdateTagRequest{StoreId: "100", TagId: "5", Name: "新名稱"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("want code=0, got %d", resp.Code)
		}
	})
}

func TestHandler_DeleteTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			DeleteTagFn: func(_ context.Context, _, _ int64) error { return nil },
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.DeleteTag(ctx, &pb.DeleteTagRequest{StoreId: "100", TagId: "5"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("want code=0, got %d", resp.Code)
		}
	})
}

// ── Customer tag tests ────────────────────────────────────────────────────────

func TestHandler_GetCustomerTags(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			GetCustomerTagsFn: func(_ context.Context, _, _ int64) ([]*domain.CrmTag, error) {
				return []*domain.CrmTag{{ID: 1, Name: "VIP"}}, nil
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.GetCustomerTags(ctx, &pb.GetCustomerTagsRequest{StoreId: "100", BuyerId: "200"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Data) != 1 {
			t.Errorf("want 1 tag, got %d", len(resp.Data))
		}
		if resp.Data[0].Name != "VIP" {
			t.Errorf("want name='VIP', got '%s'", resp.Data[0].Name)
		}
	})
}

func TestHandler_AddCustomerTags(t *testing.T) {
	ctx := context.Background()

	t.Run("success - existing tags found by name", func(t *testing.T) {
		var addCount int
		crmRepo := &mockCrmRepo{
			GetTagByNameFn: func(_ context.Context, _ int64, name string) (*domain.CrmTag, error) {
				return &domain.CrmTag{ID: 1, Name: name}, nil
			},
			AddCustomerTagFn: func(_ context.Context, _, _, _ int64) error {
				addCount++
				return nil
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.AddCustomerTags(ctx, &pb.AddCustomerTagsRequest{
			StoreId: "100", BuyerId: "200", TagNames: []string{"VIP", "奧客", "高消費"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("want code=0, got %d", resp.Code)
		}
		if addCount != 3 {
			t.Errorf("want 3 tags added, got %d", addCount)
		}
	})

	t.Run("success - tag not found, auto-created", func(t *testing.T) {
		var createCount, addCount int
		crmRepo := &mockCrmRepo{
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
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.AddCustomerTags(ctx, &pb.AddCustomerTagsRequest{
			StoreId: "100", BuyerId: "200", TagNames: []string{"新標籤A", "新標籤B"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("want code=0, got %d", resp.Code)
		}
		if createCount != 2 {
			t.Errorf("want 2 tags created, got %d", createCount)
		}
		if addCount != 2 {
			t.Errorf("want 2 tags added, got %d", addCount)
		}
	})
}

func TestHandler_RemoveCustomerTag(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			RemoveCustomerTagFn: func(_ context.Context, _, _, _ int64) error { return nil },
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		resp, err := client.RemoveCustomerTag(ctx, &pb.RemoveCustomerTagRequest{
			StoreId: "100", BuyerId: "200", TagId: "10",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("want code=0, got %d", resp.Code)
		}
	})

	t.Run("repo error returns gRPC error", func(t *testing.T) {
		crmRepo := &mockCrmRepo{
			RemoveCustomerTagFn: func(_ context.Context, _, _, _ int64) error {
				return errors.New("not found")
			},
		}
		client, stop := newTestServer(t, crmRepo)
		defer stop()

		_, err := client.RemoveCustomerTag(ctx, &pb.RemoveCustomerTagRequest{
			StoreId: "100", BuyerId: "200", TagId: "10",
		})
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})
}

// ── ErrorInterceptor 映射測試 ────────────────────────────────────────────────

func TestHandler_ErrorInterceptor_MapsNotFound(t *testing.T) {
	crmRepo := &mockCrmRepo{
		GetCustomerFn: func(_ context.Context, _, _ int64) (*domain.CrmCustomer, error) {
			return nil, pkgerrors.NewNotFound("customer not found")
		},
	}
	client, stop := newTestServer(t, crmRepo)
	defer stop()

	resp, err := client.GetCustomerStats(context.Background(), &pb.GetCustomerStatsRequest{
		StoreId: "100", BuyerId: "999",
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for missing customer, got 0")
	}
}

func TestHandler_ErrorInterceptor_MapsConflict(t *testing.T) {
	crmRepo := &mockCrmRepo{
		CreateTagFn: func(_ context.Context, _ *domain.CrmTag) error {
			return pkgerrors.NewConflict("tag already exists")
		},
	}
	client, stop := newTestServer(t, crmRepo)
	defer stop()

	resp, err := client.CreateTag(context.Background(), &pb.CreateTagRequest{
		StoreId: "100", Name: "重複標籤",
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for duplicate tag, got 0")
	}
}

func TestHandler_ErrorInterceptor_PlainErrorMapsToInternal(t *testing.T) {
	crmRepo := &mockCrmRepo{
		GetCustomerFn: func(_ context.Context, _, _ int64) (*domain.CrmCustomer, error) {
			return nil, errors.New("db connection failed")
		},
	}
	client, stop := newTestServer(t, crmRepo)
	defer stop()

	_, err := client.GetCustomerStats(context.Background(), &pb.GetCustomerStatsRequest{StoreId: "100", BuyerId: "200"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("want gRPC status error, got %T", err)
	}
	if st.Code() != codes.Internal {
		t.Errorf("want codes.Internal, got %v", st.Code())
	}
}
