package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

const (
	syncAccountID = int64(920000001)
	syncStoreID   = int64(920000002)
)

type fakePayuniProfileOutboxRepo struct {
	domain.PayuniMerchantProfileOutboxRepository
	inserted []*domain.PayuniMerchantProfileOutbox
	err      error
}

func (f *fakePayuniProfileOutboxRepo) Insert(_ context.Context, row *domain.PayuniMerchantProfileOutbox) error {
	if f.err != nil {
		return f.err
	}
	cp := *row
	f.inserted = append(f.inserted, &cp)
	return nil
}

func newSellerUseCaseForOutboxTest(t *testing.T, regStatus domain.PayuniStatus, sellerType string, outbox domain.PayuniMerchantProfileOutboxRepository) (*SellerUseCase, *mockSellerRepo) {
	t.Helper()
	seller := func() *domain.Seller {
		return &domain.Seller{
			AccountID: syncAccountID, StoreID: syncStoreID, StoreCode: 2600000010,
			StoreName: "測試店",
			City:      "台北市", District: "中正區", AddressDetail: "重慶南路一段1號",
			PostalCode:   "100",
			ContactPhone: "0912345678",
			Status:       domain.SellerStatusActive, Version: 3,
		}
	}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != syncAccountID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return seller(), nil
		},
		UpdateFn: func(_ context.Context, _ *domain.Seller) error { return nil },
	}
	certRepo := &mockCertificationRepo{
		GetCurrentApplicationFn: func(_ context.Context, _ int64) (*domain.CertificationApplication, error) {
			return &domain.CertificationApplication{Status: domain.CertStatusForwarded, SellerType: sellerType}, nil
		},
	}
	payuniRepo := &mockPayuniRepo{
		GetByStoreIDFn: func(_ context.Context, _ int64) (*domain.PayuniRegistration, error) {
			return &domain.PayuniRegistration{StoreID: syncStoreID, Status: regStatus, PayuniMerID: "HAWK2600000010"}, nil
		},
	}
	uc := NewSellerUseCase(sellerRepo, payuniRepo, certRepo, testAccountClient, &mockMediaClient{},
		&config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil,
		testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, outbox, testStoreCode, nil)
	return uc, sellerRepo
}

func TestUpdateProfile_PayuniOutboxEnqueueMatrix(t *testing.T) {
	cases := []struct {
		name         string
		newDistrict  string
		payuniStatus domain.PayuniStatus
		sellerType   string
		wantEnqueued bool
	}{
		{"地址變動+active+individual → 投遞", "信義區", domain.PayuniStatusActive, "individual", true},
		{"地址未變 → 不投遞", "中正區", domain.PayuniStatusActive, "individual", false},
		{"registration 非 active → 不投遞", "信義區", domain.PayuniStatusPending, "individual", false},
		{"企業賣家 → 不投遞", "信義區", domain.PayuniStatusActive, "business", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outbox := &fakePayuniProfileOutboxRepo{}
			uc, _ := newSellerUseCaseForOutboxTest(t, tc.payuniStatus, tc.sellerType, outbox)
			addressDetail := "重慶南路一段1號"
			if tc.newDistrict != "中正區" {
				addressDetail = "松高路1號"
			}
			err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
				Target:        dto.TargetByAccountID(syncAccountID),
				StoreName:     "測試店",
				City:          "台北市",
				District:      tc.newDistrict,
				AddressDetail: addressDetail,
				ContactPhone:  "0912345678",
			})
			if err != nil {
				t.Fatalf("UpdateProfile: %v", err)
			}
			want := 0
			if tc.wantEnqueued {
				want = 1
			}
			if got := len(outbox.inserted); got != want {
				t.Fatalf("outbox rows = %d, want %d", got, want)
			}
			if !tc.wantEnqueued {
				return
			}
			row := outbox.inserted[0]
			if row.StoreID != syncStoreID || row.AccountID != syncAccountID {
				t.Errorf("row ids = %d/%d, want %d/%d", row.StoreID, row.AccountID, syncStoreID, syncAccountID)
			}
			if row.Status != domain.PayuniMerchantProfileOutboxPending {
				t.Errorf("Status = %q, want pending", row.Status)
			}
		})
	}
}

func TestUpdateProfile_PayuniOutboxCarriesTheNewAddress(t *testing.T) {
	outbox := &fakePayuniProfileOutboxRepo{}
	uc, _ := newSellerUseCaseForOutboxTest(t, domain.PayuniStatusActive, "individual", outbox)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target:        dto.TargetByAccountID(syncAccountID),
		StoreName:     "測試店",
		City:          "台北市",
		District:      "信義區",
		AddressDetail: "松高路1號",
	})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if len(outbox.inserted) != 1 {
		t.Fatalf("outbox rows = %d, want 1", len(outbox.inserted))
	}
	var p domain.PayuniMerchantProfilePayload
	if err := json.Unmarshal(outbox.inserted[0].Payload, &p); err != nil {
		t.Fatalf("payload not valid json: %v", err)
	}
	if p.MerAddr != "台北市信義區松高路1號" {
		t.Errorf("MerAddr = %q, want the address written in this call", p.MerAddr)
	}
	if p.MerAddrPost != "110" {
		t.Errorf("MerAddrPost = %q, want 110 (信義區)", p.MerAddrPost)
	}
}

func TestUpdateProfile_PayuniOutboxInsertFailurePropagates(t *testing.T) {
	insertErr := errors.New("outbox insert exploded")
	outbox := &fakePayuniProfileOutboxRepo{err: insertErr}
	uc, _ := newSellerUseCaseForOutboxTest(t, domain.PayuniStatusActive, "individual", outbox)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target:        dto.TargetByAccountID(syncAccountID),
		StoreName:     "測試店",
		City:          "台北市",
		District:      "信義區",
		AddressDetail: "松高路1號",
	})
	if !errors.Is(err, insertErr) {
		t.Fatalf("err = %v, want the outbox insert error to propagate", err)
	}
}

func TestUpdateProfile_NilOutboxRepoDoesNotBlockTheWrite(t *testing.T) {
	uc, _ := newSellerUseCaseForOutboxTest(t, domain.PayuniStatusActive, "individual", nil)
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target:        dto.TargetByAccountID(syncAccountID),
		StoreName:     "測試店",
		City:          "台北市",
		District:      "信義區",
		AddressDetail: "松高路1號",
	})
	if err != nil {
		t.Fatalf("UpdateProfile with nil outbox repo: %v", err)
	}
}

func TestUpdateProfile_PayuniLookupFailureDoesNotBlockTheWrite(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sabotage func(cert *mockCertificationRepo, payuni *mockPayuniRepo)
	}{
		{"認證查詢出錯", func(cert *mockCertificationRepo, _ *mockPayuniRepo) {
			cert.GetCurrentApplicationFn = func(_ context.Context, _ int64) (*domain.CertificationApplication, error) {
				return nil, errors.New("cert boom")
			}
		}},
		{"PayUni 註冊查詢出錯", func(_ *mockCertificationRepo, payuni *mockPayuniRepo) {
			payuni.GetByStoreIDFn = func(_ context.Context, _ int64) (*domain.PayuniRegistration, error) {
				return nil, errors.New("payuni boom")
			}
		}},
		{"沒有註冊紀錄", func(_ *mockCertificationRepo, payuni *mockPayuniRepo) {
			payuni.GetByStoreIDFn = func(_ context.Context, _ int64) (*domain.PayuniRegistration, error) {
				return nil, nil
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outbox := &fakePayuniProfileOutboxRepo{}
			uc, _ := newSellerUseCaseForOutboxTest(t, domain.PayuniStatusActive, "individual", outbox)
			tc.sabotage(uc.certRepo.(*mockCertificationRepo), uc.payuniRepo.(*mockPayuniRepo))
			err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
				Target:        dto.TargetByAccountID(syncAccountID),
				StoreName:     "測試店",
				City:          "台北市",
				District:      "信義區",
				AddressDetail: "松高路1號",
			})
			if err != nil {
				t.Fatalf("查詢失敗應放行，得到 %v", err)
			}
			if got := len(outbox.inserted); got != 0 {
				t.Errorf("outbox rows = %d, want 0（查不到就不投遞）", got)
			}
		})
	}
}
