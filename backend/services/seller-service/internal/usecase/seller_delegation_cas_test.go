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

// 56 §5.4：代行寫入一律帶 expectedVersion，rows = 0 即版本衝突（9266）。
//
// Go 端沒有框架級樂觀鎖：CAS 述詞是逐句手寫的，「影響筆數 0」是唯一訊號。所以真正
// 要守的不變式是「expectedVersion 有沒有走到 repository」——它一旦在中途被丟掉，
// 寫入照樣成功、照樣回 code 0，賣家在同一秒改的東西被無聲蓋掉。因此下列每個 case
// 都斷言 repository 實際收到的參數，而不是「呼叫有沒有回錯」。
func TestUpdateLogoCover_PassesExpectedVersionToRepository(t *testing.T) {
	expected := int64(4)

	cases := []struct {
		name   string
		invoke func(uc *SellerUseCase, target dto.SellerTarget, ev *int64) error
	}{
		{
			name: "UpdateLogo",
			invoke: func(uc *SellerUseCase, target dto.SellerTarget, ev *int64) error {
				_, err := uc.UpdateLogo(context.Background(), target, "staged-token", ev)
				return err
			},
		},
		{
			name: "UpdateCover",
			invoke: func(uc *SellerUseCase, target dto.SellerTarget, ev *int64) error {
				_, err := uc.UpdateCover(context.Background(), target, "staged-token", ev)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/delegated", func(t *testing.T) {
			repo, uc := newCASTestUseCase(nil)
			if err := tc.invoke(uc, dto.TargetByStoreID(casStoreID), &expected); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.LastExpectedVersion == nil {
				t.Fatal("expectedVersion never reached the repository — the CAS predicate was disabled, so the write would overwrite a concurrent seller edit with no error at all")
			}
			if *repo.LastExpectedVersion != expected {
				t.Fatalf("expectedVersion = %d, want %d", *repo.LastExpectedVersion, expected)
			}
			assertWroteToStore(t, repo, casStoreID)
		})

		t.Run(tc.name+"/seller-own-path", func(t *testing.T) {
			repo, uc := newCASTestUseCase(nil)
			if err := tc.invoke(uc, dto.TargetByAccountID(casAccountID), nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.LastExpectedVersion != nil {
				t.Fatalf("expectedVersion = %d, want nil — the seller's own path has no version to compare",
					*repo.LastExpectedVersion)
			}
			// 賣家自己的路徑也必須落在同一列：account 軸只是「怎麼找到那一列」，
			// 找到之後的寫入鍵與代行路徑是同一個。
			assertWroteToStore(t, repo, casStoreID)
		})

		t.Run(tc.name+"/version-conflict-propagates", func(t *testing.T) {
			_, uc := newCASTestUseCase(domain.ErrVersionConflict)
			err := tc.invoke(uc, dto.TargetByStoreID(casStoreID), &expected)
			if !errors.Is(err, domain.ErrVersionConflict) {
				t.Fatalf("err = %v, want ErrVersionConflict — a conflict must reach the caller as 9266, "+
					"not be swallowed into a success the client reads as 'saved'", err)
			}
		})
	}
}

// casAccountID / casStoreID 刻意不相等，且 casAccountID 同時是「另一家店」
// otherStoreID 的 store_id —— 這正是 grandfathered 資料掩蓋掉的那個形狀。
const (
	casAccountID int64 = 2685914682
	casStoreID   int64 = 700700700
)

func assertWroteToStore(t *testing.T, repo *mockSellerRepo, want int64) {
	t.Helper()
	if repo.LastWriteStoreID != want {
		t.Fatalf("write keyed on store_id = %d, want %d — a storefront write must land on the "+
			"targeted stint, not on whichever row shares the id on the other axis",
			repo.LastWriteStoreID, want)
	}
}

// newCASTestUseCase wires a use case whose logo/cover writes fail with writeErr
// (nil = succeed) and whose repo records the CAS argument it was handed.
func newCASTestUseCase(writeErr error) (*mockSellerRepo, *SellerUseCase) {
	mediaClient := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, _ []string, _ string) ([]AssetInfo, error) {
			return []AssetInfo{{AssetId: "new-asset", ObjectKey: "public/seller_logo/new.png"}}, nil
		},
		LinkImagesFn:        func(_ context.Context, _ string, _ string, _ []string) error { return nil },
		ReleaseReferencesFn: func(_ context.Context, _ string, _ []string, _ []string) error { return nil },
	}
	seller := func() *domain.Seller {
		return &domain.Seller{
			AccountID: casAccountID, StoreID: casStoreID, StoreCode: 2600000001,
			Status: domain.SellerStatusActive, Version: 4,
		}
	}
	repo := &mockSellerRepo{
		// 兩個 resolver 各自只認自己那根軸的值。餵錯軸就查無此列，而不是像
		// grandfathered fixture 那樣兩邊都命中同一列、把 bug 藏起來。
		GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != casAccountID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return seller(), nil
		},
		GetCurrentByStoreIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != casStoreID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return seller(), nil
		},
		UpdateFn:     func(_ context.Context, _ *domain.Seller) error { return nil },
		UpdateLogoFn: func(_ context.Context, _ int64, _ string) error { return writeErr },
		UpdateCoverFn: func(_ context.Context, _ int64, _ string) error {
			return writeErr
		},
	}
	uc := NewSellerUseCase(repo, nil, nil, testAccountClient, mediaClient,
		&config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil,
		testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)
	return repo, uc
}

// UpdateProfile 的 CAS 走 repo.Update，與 logo/cover 是不同的查詢，必須各自守。
func TestUpdateProfile_PassesExpectedVersionToRepository(t *testing.T) {
	expected := int64(7)

	var gotExpected *int64
	var updateCalls int
	seller := func() *domain.Seller {
		return &domain.Seller{
			AccountID: casAccountID, StoreID: casStoreID, StoreCode: 2600000001,
			ContactPhone: "0912345678", // 已有值，避免讀取路徑的 backfill 再寫一次
			Status:       domain.SellerStatusActive, Version: 7,
		}
	}
	repo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != casAccountID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return seller(), nil
		},
		GetCurrentByStoreIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
			if id != casStoreID {
				return nil, pkgerrors.NewNotFound("seller not found")
			}
			return seller(), nil
		},
		UpdateFn: func(_ context.Context, _ *domain.Seller) error {
			updateCalls++
			return nil
		},
	}
	uc := NewSellerUseCase(repo, nil, nil, testAccountClient, &mockMediaClient{},
		&config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil,
		testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)

	if err := uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
		Target:          dto.TargetByStoreID(casStoreID),
		StoreName:       "梨山盛花園",
		ExpectedVersion: &expected,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotExpected = repo.LastExpectedVersion
	if updateCalls != 1 {
		t.Fatalf("repo.Update called %d times, want 1", updateCalls)
	}
	if gotExpected == nil || *gotExpected != expected {
		t.Fatalf("expectedVersion = %v, want %d", gotExpected, expected)
	}
	assertWroteToStore(t, repo, casStoreID)
}

// 這一支是本批修復的迴歸鎖：代行寫入的目標是一家店，而「另一家店的 account_id
// 恰好等於這個 store_id」在 TSID 值域裡完全可能發生。修復前的實作以 account_id
// 為鍵，於是 PATCH store=casStoreID 會靜默改到 otherStore 並回 code: 0。
//
// fixture 刻意讓 otherStore.AccountID == casStoreID —— 這正是本機 seed（全是
// store_id == account_id 的首任賣家）永遠造不出來的那個形狀。
func TestStorefrontWrites_TargetTheNamedStore_NotTheAccountIDTwin(t *testing.T) {
	const otherStoreID int64 = 900900900

	newRepo := func() *mockSellerRepo {
		return &mockSellerRepo{
			// account 軸：casStoreID 這個「值」在此軸上屬於 otherStore。
			GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
				if id == casStoreID {
					return &domain.Seller{
						AccountID: casStoreID, StoreID: otherStoreID, StoreCode: 2600000002,
						Status: domain.SellerStatusActive, Version: 1,
					}, nil
				}
				return nil, pkgerrors.NewNotFound("seller not found")
			},
			// store 軸：casStoreID 是目標店的 PK。
			GetCurrentByStoreIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
				if id != casStoreID {
					return nil, pkgerrors.NewNotFound("seller not found")
				}
				return &domain.Seller{
					AccountID: casAccountID, StoreID: casStoreID, StoreCode: 2600000001,
					ContactPhone: "0912345678",
					Status:       domain.SellerStatusActive, Version: 1,
				}, nil
			},
			UpdateFn:      func(_ context.Context, _ *domain.Seller) error { return nil },
			UpdateLogoFn:  func(_ context.Context, _ int64, _ string) error { return nil },
			UpdateCoverFn: func(_ context.Context, _ int64, _ string) error { return nil },
		}
	}
	mediaClient := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, _ []string, _ string) ([]AssetInfo, error) {
			return []AssetInfo{{AssetId: "new-asset", ObjectKey: "public/seller_logo/new.png"}}, nil
		},
		LinkImagesFn:        func(_ context.Context, _ string, _ string, _ []string) error { return nil },
		ReleaseReferencesFn: func(_ context.Context, _ string, _ []string, _ []string) error { return nil },
	}
	newUC := func(repo *mockSellerRepo) *SellerUseCase {
		return NewSellerUseCase(repo, nil, nil, testAccountClient, mediaClient,
			&config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil,
			testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)
	}
	version := int64(1)

	cases := []struct {
		name   string
		invoke func(uc *SellerUseCase) error
	}{
		{"UpdateProfile", func(uc *SellerUseCase) error {
			return uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
				Target:          dto.TargetByStoreID(casStoreID),
				StoreName:       "代行改的店名",
				ExpectedVersion: &version,
			})
		}},
		{"UpdateLogo", func(uc *SellerUseCase) error {
			_, err := uc.UpdateLogo(context.Background(), dto.TargetByStoreID(casStoreID), "staged-token", &version)
			return err
		}},
		{"UpdateCover", func(uc *SellerUseCase) error {
			_, err := uc.UpdateCover(context.Background(), dto.TargetByStoreID(casStoreID), "staged-token", &version)
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo()
			if err := tc.invoke(newUC(repo)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo.LastWriteStoreID == otherStoreID {
				t.Fatalf("%s wrote to store %d — that is a DIFFERENT seller, reached only because its "+
					"account_id equals the targeted store_id", tc.name, otherStoreID)
			}
			assertWroteToStore(t, repo, casStoreID)
		})
	}
}
