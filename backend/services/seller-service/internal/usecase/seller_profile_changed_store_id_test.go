package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

type mockProfileEventPublisher struct {
	published chan int64
}

func newMockProfileEventPublisher() *mockProfileEventPublisher {
	return &mockProfileEventPublisher{published: make(chan int64, 1)}
}

func (m *mockProfileEventPublisher) PublishSellerProfileChanged(_ context.Context, storeID int64) error {
	m.published <- storeID
	return nil
}

func TestSellerUC_ProfileChangedEvent_PublishesStoreID_NotAccountID(t *testing.T) {
	const accountID = int64(1001)
	const storeID = int64(2001) // deliberately != accountID

	paths := []struct {
		name   string
		invoke func(*SellerUseCase) error
	}{
		{
			name: "UpdateProfile",
			invoke: func(uc *SellerUseCase) error {
				return uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{Target: dto.TargetByAccountID(accountID), StoreName: "新店名"})
			},
		},
		{
			name: "UpdateLogo",
			invoke: func(uc *SellerUseCase) error {
				_, err := uc.UpdateLogo(context.Background(), dto.TargetByAccountID(accountID), "staged-token", nil)
				return err
			},
		},
		{
			name: "UpdateCover",
			invoke: func(uc *SellerUseCase) error {
				_, err := uc.UpdateCover(context.Background(), dto.TargetByAccountID(accountID), "staged-token", nil)
				return err
			},
		},
	}

	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
					if id != accountID {
						t.Errorf("GetByAccountID(%d), want %d", id, accountID)
					}
					return &domain.Seller{
						AccountID: accountID, StoreID: storeID,
						Status: domain.SellerStatusActive,
					}, nil
				},
				UpdateFn: func(_ context.Context, _ *domain.Seller) error { return nil },
			}
			mediaClient := &mockMediaClient{
				FinalizeImagesFn: func(_ context.Context, _ []string, _ string) ([]AssetInfo, error) {
					return []AssetInfo{{AssetId: "asset-1", ObjectKey: "sellers/logo.png"}}, nil
				},
			}
			publisher := newMockProfileEventPublisher()
			uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, mediaClient,
				&config.Config{MediaBaseURL: "https://cdn.test"},
				revalidate.New(revalidate.Config{}, nil), publisher,
				testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)

			if err := p.invoke(uc); err != nil {
				t.Fatalf("%s: unexpected error: %v", p.name, err)
			}

			select {
			case got := <-publisher.published:
				if got != storeID {
					t.Errorf("%s emitted profile-changed id = %d, want store_id %d (accountID was %d)", p.name, got, storeID, accountID)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("%s: emitProfileChanged never published (goroutine didn't run or profileEvents nil)", p.name)
			}
		})
	}
}
