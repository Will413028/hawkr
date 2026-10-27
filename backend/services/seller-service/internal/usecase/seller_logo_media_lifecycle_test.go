package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

// 換 logo / cover 的媒體生命週期契約（backend#2215 / #2222）。
//
// 不變式：**任一瞬間，「有 asset_reference 的資產集合」必須是「業務列指著的資產集合」的超集**。
// 這個關係一旦反過來，media GC 就會把一張正在顯示的圖連同 R2 物件實體刪除，而且事後完全
// 無跡可循（media_asset 列一併被刪）——2026-08 dev 上 16 個賣家 logo 就是這樣變成 404 的。
//
// 由此推出唯一安全的順序，本測試逐項釘住：
//
//	① LinkImages(新資產)  —— 必須在寫業務列之前，失敗即中止（不得吞掉、不得丟進 goroutine）
//	② sellerRepo.Update*  —— 業務列改指新 key
//	③ ReleaseReferences(keepAssetIDs=[新資產]) —— 舊資產這時才交還給 GC
//
// 三步的任一種失敗都只會「殘留可回收的引用」（洩漏），不會「刪掉正在用的圖」。
// 舊版順序是 release → 寫業務列 → best-effort link：第一步就把舊圖交給了 GC，
// 而最後一步失敗時新圖也沒有引用——兩張圖同時暴露在 GC 之下。
func TestUpdateLogoAndCover_LinkBeforeWriteThenReleaseSuperseded(t *testing.T) {
	const accountID int64 = 2685914682

	cases := []struct {
		name        string
		wantRefType string
		invoke      func(uc *SellerUseCase) error
	}{
		{
			name:        "UpdateLogo",
			wantRefType: media.RefTypeSellerLogo,
			invoke: func(uc *SellerUseCase) error {
				_, err := uc.UpdateLogo(context.Background(), dto.TargetByAccountID(accountID), "staged-token", nil)
				return err
			},
		},
		{
			name:        "UpdateCover",
			wantRefType: media.RefTypeSellerCover,
			invoke: func(uc *SellerUseCase) error {
				_, err := uc.UpdateCover(context.Background(), dto.TargetByAccountID(accountID), "staged-token", nil)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			var releasedRefType, linkedRefType string
			var releasedRefIDs, releasedKeep, linkedAssetIDs []string

			mediaClient := &mockMediaClient{
				FinalizeImagesFn: func(_ context.Context, _ []string, _ string) ([]AssetInfo, error) {
					calls = append(calls, "finalize")
					return []AssetInfo{{AssetId: "new-asset", ObjectKey: "public/seller_logo/new.png"}}, nil
				},
				ReleaseReferencesFn: func(_ context.Context, refType string, refIDs []string, keepAssetIDs []string) error {
					calls = append(calls, "release")
					releasedRefType, releasedRefIDs, releasedKeep = refType, refIDs, keepAssetIDs
					return nil
				},
				LinkImagesFn: func(_ context.Context, refType string, _ string, assetIDs []string) error {
					calls = append(calls, "link")
					linkedRefType, linkedAssetIDs = refType, assetIDs
					return nil
				},
			}

			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
					return &domain.Seller{
						AccountID: id, StoreID: 700700700, StoreCode: 2600000001,
						Status: domain.SellerStatusActive,
					}, nil
				},
				UpdateFn: func(_ context.Context, _ *domain.Seller) error { return nil },
				UpdateLogoFn: func(_ context.Context, _ int64, _ string) error {
					calls = append(calls, "write")
					return nil
				},
				UpdateCoverFn: func(_ context.Context, _ int64, _ string) error {
					calls = append(calls, "write")
					return nil
				},
			}

			uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, mediaClient,
				&config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil,
				testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)

			if err := tc.invoke(uc); err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.name, err)
			}

			wantOrder := []string{"finalize", "link", "write", "release"}
			if len(calls) != len(wantOrder) {
				t.Fatalf("%s call order = %v, want %v", tc.name, calls, wantOrder)
			}
			for i := range wantOrder {
				if calls[i] != wantOrder[i] {
					t.Fatalf("%s call order = %v, want %v —— link 必須在寫業務列之前、release 必須在其後；"+
						"順序一錯就會有一張正在顯示的圖沒有引用，7 天後被 media GC 連同 R2 物件實體刪除",
						tc.name, calls, wantOrder)
				}
			}

			if linkedRefType != tc.wantRefType {
				t.Errorf("linked refType = %q, want %q", linkedRefType, tc.wantRefType)
			}
			if len(linkedAssetIDs) != 1 || linkedAssetIDs[0] != "new-asset" {
				t.Errorf("linked assetIDs = %v, want [new-asset]", linkedAssetIDs)
			}
			if releasedRefType != tc.wantRefType {
				t.Errorf("released refType = %q, want %q", releasedRefType, tc.wantRefType)
			}
			// Scoped to this seller: a wider refID set would release other sellers'
			// logos and hand their live images to the GC.
			//
			// refID 是 store_id（700700700）而非 account_id（2685914682）：資產屬於
			// 一段 stint。以 account_id 為 refID 時，re-onboard 後的新 stint 換 logo
			// 會 release 掉舊 stint 仍在顯示的那張圖。fixture 的兩個 id 刻意不相等，
			// 否則這條斷言分不出兩者。
			if len(releasedRefIDs) != 1 || releasedRefIDs[0] != "700700700" {
				t.Errorf("released refIDs = %v, want exactly [700700700] (the store_id, not the account_id)", releasedRefIDs)
			}
			if len(releasedKeep) != 1 || releasedKeep[0] != "new-asset" {
				t.Errorf("released keepAssetIDs = %v, want [new-asset] —— 沒帶 keep 名單，release 會把"+
					"上一步剛建立的新資產引用一起刪掉，換上去的新圖反而變成 GC 目標", releasedKeep)
			}
		})
	}
}

// Link 失敗必須中止且不寫業務列。舊版把 link 錯誤降級成 Warn，於是 sellers.logo 指向一個
// 永遠不會有 asset_reference 的物件——GC 的孤兒規則看不出差別，7 天後實體刪除。
func TestUpdateLogoAndCover_LinkFailureAbortsWithoutWritingBusinessRow(t *testing.T) {
	const accountID int64 = 2685914682

	cases := []struct {
		name   string
		invoke func(uc *SellerUseCase) error
	}{
		{"UpdateLogo", func(uc *SellerUseCase) error {
			_, err := uc.UpdateLogo(context.Background(), dto.TargetByAccountID(accountID), "staged-token", nil)
			return err
		}},
		{"UpdateCover", func(uc *SellerUseCase) error {
			_, err := uc.UpdateCover(context.Background(), dto.TargetByAccountID(accountID), "staged-token", nil)
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrote := false
			released := false
			mediaClient := &mockMediaClient{
				FinalizeImagesFn: func(_ context.Context, _ []string, _ string) ([]AssetInfo, error) {
					return []AssetInfo{{AssetId: "new-asset", ObjectKey: "public/seller_logo/new.png"}}, nil
				},
				LinkImagesFn: func(_ context.Context, _ string, _ string, _ []string) error {
					return errors.New("media-service unavailable")
				},
				ReleaseReferencesFn: func(_ context.Context, _ string, _ []string, _ []string) error {
					released = true
					return nil
				},
			}
			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
					return &domain.Seller{AccountID: id, StoreID: 700700700, Status: domain.SellerStatusActive}, nil
				},
				UpdateLogoFn:  func(_ context.Context, _ int64, _ string) error { wrote = true; return nil },
				UpdateCoverFn: func(_ context.Context, _ int64, _ string) error { wrote = true; return nil },
			}

			uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, mediaClient,
				&config.Config{MediaBaseURL: "https://cdn.test"}, nil, nil,
				testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)

			if err := tc.invoke(uc); err == nil {
				t.Fatalf("%s: link 失敗時必須回錯，不得吞掉", tc.name)
			}
			if wrote {
				t.Errorf("%s: link 失敗仍寫入業務列 —— 這正是「業務列指著、卻沒有引用」的成因", tc.name)
			}
			if released {
				t.Errorf("%s: link 失敗仍釋放舊引用 —— 舊圖還被 sellers 指著，會被 GC 實體刪除", tc.name)
			}
		})
	}
}
