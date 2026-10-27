package grpc

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestUpdateMerchantProfile_VersionConflict_ReturnsBizcode(t *testing.T) {
	const accountID, storeID = int64(100), int64(999)

	sellerRepo := &testSellerRepo{
		getByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: storeID}, nil
		},
	}
	payuniRepo := &testPayuniRepo{
		getByStoreIDFn: func(_ context.Context, id int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{
				StoreID: storeID, PayuniMerID: "HAWK999",
				Status: domain.PayuniStatusActive, MerchantProfileVersion: 5,
			}, nil
		},
	}
	client, cleanup := newTestGRPCClient(t, newSellerServer(sellerRepo, payuniRepo))
	defer cleanup()

	resp, err := client.UpdateMerchantProfile(context.Background(), &pb.UpdateMerchantProfileRequest{
		AccountId: accountID, IdempotencyKey: "key-conflict01", ExpectedVersion: "3", ContactName: "王小明",
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.GetCode() != bizcode.PayuniMerchantProfileVersionConflict {
		t.Fatalf("code = %d, want %d (client sent ExpectedVersion=3, actual MerchantProfileVersion=5)",
			resp.GetCode(), bizcode.PayuniMerchantProfileVersionConflict)
	}
}
