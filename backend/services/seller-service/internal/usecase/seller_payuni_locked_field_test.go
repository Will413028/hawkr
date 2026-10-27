package usecase

import (
	"context"
	"errors"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

const (
	lockAccountID = int64(910000001)
	lockStoreID   = int64(910000002)
)

type fakeCertRepoForLock struct {
	domain.CertificationRepository
	status domain.CertificationStatus
	absent bool
	err    error
}

func (f *fakeCertRepoForLock) GetCurrentApplication(_ context.Context, _ int64) (*domain.CertificationApplication, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.absent {
		return nil, nil
	}
	return &domain.CertificationApplication{Status: f.status}, nil
}

func newSellerUseCaseForLockTest(t *testing.T, certRepo domain.CertificationRepository) (*SellerUseCase, *mockSellerRepo) {
	t.Helper()
	seller := func() *domain.Seller {
		return &domain.Seller{
			AccountID: lockAccountID, StoreID: lockStoreID, StoreCode: 2600000009,
			StoreName: "原始店名", Description: "原始介紹",
			City: "台北市", District: "中正區", AddressDetail: "重慶南路一段1號",
			ContactPhone: "0912345678",
			Status:       domain.SellerStatusActive, Version: 3,
		}
	}
	repo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != lockAccountID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return seller(), nil
		},
		GetCurrentByStoreIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != lockStoreID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return seller(), nil
		},
		UpdateFn: func(_ context.Context, _ *domain.Seller) error { return nil },
	}
	uc := NewSellerUseCase(repo, nil, certRepo, testAccountClient, &mockMediaClient{},
		&config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil,
		testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)
	return uc, repo
}

func TestUpdateProfile_StoreNameLockByCertStatus(t *testing.T) {
	cases := []struct {
		name    string
		status  domain.CertificationStatus
		wantErr error
	}{
		{"submitted", domain.CertStatusSubmitted, domain.ErrPayuniLockedFieldUnderReview},
		{"forwarded", domain.CertStatusForwarded, domain.ErrPayuniLockedField},
		{"draft", domain.CertStatusDraft, nil},
		{"rejected", domain.CertStatusRejected, nil},
		{"forward_failed", domain.CertStatusForwardFailed, nil},
		{"cancelled", domain.CertStatusCancelled, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, _ := newSellerUseCaseForLockTest(t, &fakeCertRepoForLock{status: tc.status})
			err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
				Target:    dto.TargetByStoreID(lockStoreID),
				StoreName: "改過的店名",
				City:      "台北市",
				District:  "中正區",
			})
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("應放行，得到 %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestPayuniLockedErrors_AreDistinct(t *testing.T) {
	if errors.Is(domain.ErrPayuniLockedFieldUnderReview, domain.ErrPayuniLockedField) {
		t.Error("審核中與已送 PayUni 必須是可區分的兩個錯誤")
	}
}

func TestUpdateProfile_DescriptionLockedToo(t *testing.T) {
	uc, _ := newSellerUseCaseForLockTest(t, &fakeCertRepoForLock{status: domain.CertStatusForwarded})
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target:      dto.TargetByStoreID(lockStoreID),
		Description: "改過的自我介紹",
		City:        "台北市",
		District:    "中正區",
	})
	if !errors.Is(err, domain.ErrPayuniLockedField) {
		t.Errorf("description 也必須鎖，得到 %v", err)
	}
}

func TestUpdateProfile_SameValueIsNotAChange(t *testing.T) {
	uc, _ := newSellerUseCaseForLockTest(t, &fakeCertRepoForLock{status: domain.CertStatusForwarded})
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target:        dto.TargetByStoreID(lockStoreID),
		StoreName:     "原始店名", // 與現值相同
		Description:   "原始介紹", // 與現值相同
		City:          "台北市",
		District:      "信義區", // 只有地址真的變了
		AddressDetail: "松高路1號",
	})
	if err != nil {
		t.Fatalf("送相同的值不該被擋: %v", err)
	}
}

func TestUpdateProfile_EmptyStoreNameIsNotAChange(t *testing.T) {
	uc, _ := newSellerUseCaseForLockTest(t, &fakeCertRepoForLock{status: domain.CertStatusForwarded})
	err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target:      dto.TargetByStoreID(lockStoreID),
		StoreName:   "",     // 沒送
		Description: "原始介紹", // 與現值相同
		City:        "台北市",
		District:    "中正區",
	})
	if err != nil {
		t.Fatalf("空 storeName 代表沒送，不該被擋: %v", err)
	}
}

func TestUpdateProfile_CertLookupFailureDoesNotBlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		repo *fakeCertRepoForLock
	}{
		{"查詢出錯", &fakeCertRepoForLock{err: errors.New("boom")}},
		{"沒有申請紀錄", &fakeCertRepoForLock{absent: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, _ := newSellerUseCaseForLockTest(t, tc.repo)
			err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
				Target:    dto.TargetByStoreID(lockStoreID),
				StoreName: "改過的店名",
				City:      "台北市",
				District:  "中正區",
			})
			if err != nil {
				t.Errorf("應放行，得到 %v", err)
			}
		})
	}
}
