package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

func TestSellerUC_sellerCacheTags(t *testing.T) {
	tests := []struct {
		name      string
		accountID int64
		seller    *domain.Seller
		repoErr   error
		want      []string
	}{
		{
			name:      "diverged ids: account_id + store_id + store_code all busted",
			accountID: 2623900520,
			seller:    &domain.Seller{AccountID: 2623900520, StoreID: 700700700, StoreCode: 2600000001},
			want:      []string{"seller:2623900520", "seller:700700700", "seller:2600000001"},
		},
		{
			name:      "grandfathered seller whose ids coincide: one tag, no duplicates",
			accountID: 2623900520,
			seller:    &domain.Seller{AccountID: 2623900520, StoreID: 2623900520, StoreCode: 2623900520},
			want:      []string{"seller:2623900520"},
		},
		{
			name:      "store_code backfilled from the bound MerID: only store_id is distinct",
			accountID: 2623900520,
			seller:    &domain.Seller{AccountID: 2623900520, StoreID: 700700700, StoreCode: 700700700},
			want:      []string{"seller:2623900520", "seller:700700700"},
		},
		{
			name:      "missing store_code: remaining tags still go out, no seller:0",
			accountID: 2623900520,
			seller:    &domain.Seller{AccountID: 2623900520, StoreID: 700700700},
			want:      []string{"seller:2623900520", "seller:700700700"},
		},
		{
			name:      "seller lookup fails: fail-soft to the account_id tag",
			accountID: 2623900520,
			repoErr:   errors.New("connection refused"),
			want:      []string{"seller:2623900520"},
		},
		{
			name:      "seller lookup returns nothing: fail-soft to the account_id tag",
			accountID: 2623900520,
			want:      []string{"seller:2623900520"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
					if accountID != tt.accountID {
						t.Errorf("GetByAccountID(%d), want %d", accountID, tt.accountID)
					}
					return tt.seller, tt.repoErr
				},
			}
			uc := newCacheBustUseCase(sellerRepo, nil)

			got := uc.sellerCacheTags(context.Background(), tt.accountID)
			if !slices.Equal(got, tt.want) {
				t.Errorf("sellerCacheTags = %v, want %v", got, tt.want)
			}
		})
	}
}

// The tags must reach the website on the wire in the documented
// {"tags": [...]} shape — the seller's own storefront (cached under
// store_code) is the page the bust exists for.
func TestSellerUC_bustSellerCache_SendsAllIDSpaces(t *testing.T) {
	type payload struct {
		Tags []string `json:"tags"`
	}
	received := make(chan payload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode bust body: %v", err)
		}
		received <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: 700700700, StoreCode: 2600000001}, nil
		},
	}
	uc := newCacheBustUseCase(sellerRepo, revalidate.New(revalidate.Config{BaseURL: srv.URL, Secret: "test-secret"}, nil))

	uc.bustSellerCache(context.Background(), 2623900520)

	select {
	case got := <-received:
		want := []string{"seller:2623900520", "seller:700700700", "seller:2600000001"}
		if !slices.Equal(got.Tags, want) {
			t.Errorf("busted tags = %v, want %v", got.Tags, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no cache-bust request arrived")
	}
}

// An unwired (nil) or unconfigured client means the bust would be dropped
// anyway (local dev without WEB_REVALIDATE_*): short-circuit before the id
// lookup so a disabled bust costs no query.
func TestSellerUC_bustSellerCache_DisabledSkipsLookup(t *testing.T) {
	clients := map[string]*revalidate.Client{
		"nil client":                nil,
		"missing base url / secret": revalidate.New(revalidate.Config{}, nil),
	}
	for name, client := range clients {
		t.Run(name, func(t *testing.T) {
			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(ctx context.Context, accountID int64) (*domain.Seller, error) {
					t.Error("seller lookup ran although revalidate is disabled")
					return nil, nil
				},
			}
			uc := newCacheBustUseCase(sellerRepo, client)

			uc.bustSellerCache(context.Background(), 2623900520)
		})
	}
}

// Every seller write path must bust through the collection point — a path that
// keeps building its own tag list is exactly how the storefront went stale.
func TestSellerUC_WritePathsBustEveryIDSpace(t *testing.T) {
	const accountID = 2623900520
	wantTags := []string{"seller:2623900520", "seller:700700700", "seller:2600000001"}

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
			name: "CompleteOnboarding",
			invoke: func(uc *SellerUseCase) error {
				return uc.CompleteOnboarding(context.Background(), accountID)
			},
		},
		{
			name: "UpdateSEOFields",
			invoke: func(uc *SellerUseCase) error {
				return uc.UpdateSEOFields(context.Background(), accountID, "標題", "描述")
			},
		},
		{
			name: "UpdateFoodBusinessRegistration",
			invoke: func(uc *SellerUseCase) error {
				_, _, _, err := uc.UpdateFoodBusinessRegistration(context.Background(), accountID, false, "")
				return err
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
			received := make(chan []string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Tags []string `json:"tags"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode bust body: %v", err)
				}
				received <- body.Tags
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			sellerRepo := &mockSellerRepo{
				GetByAccountIDFn: func(ctx context.Context, id int64) (*domain.Seller, error) {
					return &domain.Seller{
						AccountID: id, StoreID: 700700700, StoreCode: 2600000001,
						Status: domain.SellerStatusActive,
					}, nil
				},
				UpdateFn: func(ctx context.Context, seller *domain.Seller) error { return nil },
			}
			mediaClient := &mockMediaClient{
				FinalizeImagesFn: func(ctx context.Context, stagedTokens []string, refType string) ([]AssetInfo, error) {
					return []AssetInfo{{AssetId: "asset-1", ObjectKey: "sellers/logo.png"}}, nil
				},
			}
			uc := NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, mediaClient,
				&config.Config{MediaBaseURL: "https://cdn.test"},
				revalidate.New(revalidate.Config{BaseURL: srv.URL, Secret: "test-secret"}, nil), nil,
				testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)

			if err := p.invoke(uc); err != nil {
				t.Fatalf("%s: unexpected error: %v", p.name, err)
			}

			select {
			case got := <-received:
				if !slices.Equal(got, wantTags) {
					t.Errorf("%s busted %v, want %v", p.name, got, wantTags)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("%s sent no cache-bust request", p.name)
			}
		})
	}
}

func newCacheBustUseCase(sellerRepo domain.SellerRepository, revalidateClient *revalidate.Client) *SellerUseCase {
	return NewSellerUseCase(sellerRepo, nil, nil, testAccountClient, nil,
		&config.Config{MediaBaseURL: "https://cdn.test"}, revalidateClient, nil,
		testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)
}

// ─────────────────────────────────────────────────────────────────────────────
// #2218: deletion / suspension must bust too, and deletion must not re-read.
// ─────────────────────────────────────────────────────────────────────────────

// The reason deletion needs its own tag builder at all. Once the stint is
// soft-deleted, GetSellerByAccountID (which filters `deleted_at IS NULL`) misses
// and the tag set collapses to the account id alone — losing exactly the two
// tags the storefront page is cached under. So the deleting caller must capture
// the ids while the row is still visible and hand them to sellerCacheTagsFrom.
func TestSellerCacheTags_DeletedSellerDegradesUnlessIDsCaptured(t *testing.T) {
	deletedRepo := &mockSellerRepo{
		GetByAccountIDFn: func(context.Context, int64) (*domain.Seller, error) {
			// what the reading path sees after the soft delete
			return nil, errors.New("seller not found")
		},
	}

	degraded := sellerCacheTagsByAccountID(context.Background(), deletedRepo, 2623900520)
	if want := []string{"seller:2623900520"}; !slices.Equal(degraded, want) {
		t.Fatalf("post-delete lookup tags = %v, want %v", degraded, want)
	}

	// Same seller, ids captured before the delete: the storefront tags survive.
	captured := sellerCacheTagsFrom(context.Background(), 2623900520, 700700700, 2600000001)
	want := []string{"seller:2623900520", "seller:700700700", "seller:2600000001"}
	if !slices.Equal(captured, want) {
		t.Fatalf("captured-id tags = %v, want %v", captured, want)
	}
	if len(captured) <= len(degraded) {
		t.Fatalf("capturing ids must preserve MORE tags than a post-delete lookup: %v vs %v", captured, degraded)
	}
}

// End to end on the deletion path: the bust reaches the website, and it carries
// the storefront tags rather than the degraded single tag.
func TestDeleteSellerAccount_BustsStorefrontTags(t *testing.T) {
	type payload struct {
		Tags []string `json:"tags"`
	}
	received := make(chan payload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("decode bust body: %v", err)
		}
		received <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := &fakeDelSellerRepo{seller: &domain.Seller{
		AccountID: 2623900520, StoreID: 700700700, StoreCode: 2600000001,
		Status: domain.SellerStatusActive,
	}}
	uc := NewDeleteSellerAccountUseCase(
		repo, &fakeMerchantOrderCount{count: 0}, &fakeSellerAccountSoftDelete{},
		&fakeBuyerClient{}, &fakeSellerDeletionAudit{}, &fakeSellerDeletedOutboxRepo{},
		fakeTSID, &mockTxManager{},
		revalidate.New(revalidate.Config{BaseURL: srv.URL, Secret: "test-secret"}, nil),
	)

	if err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 2623900520}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	select {
	case got := <-received:
		want := []string{"seller:2623900520", "seller:700700700", "seller:2600000001"}
		if !slices.Equal(got.Tags, want) {
			t.Errorf("deletion busted tags = %v, want %v", got.Tags, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deletion sent no cache-bust request")
	}
}

// A nil / disabled client must not panic or block the deletion — the bust is
// best-effort, exactly like every other path that hands one to the website.
func TestDeleteSellerAccount_BustDisabledStillDeletes(t *testing.T) {
	repo := &fakeDelSellerRepo{seller: &domain.Seller{
		AccountID: 1, StoreID: 1, Status: domain.SellerStatusActive,
	}}
	uc := NewDeleteSellerAccountUseCase(
		repo, &fakeMerchantOrderCount{count: 0}, &fakeSellerAccountSoftDelete{},
		&fakeBuyerClient{}, &fakeSellerDeletionAudit{}, &fakeSellerDeletedOutboxRepo{},
		fakeTSID, &mockTxManager{},
		revalidate.New(revalidate.Config{}, nil), // unconfigured → Enabled() false
	)
	if err := uc.Execute(context.Background(), DeleteSellerAccountInput{AccountID: 1}); err != nil {
		t.Fatalf("Execute with disabled bust: %v", err)
	}
	if !repo.statusCalled {
		t.Error("deletion did not run its status update")
	}
}

// Suspension / reinstatement changes sellers.status, which the website caches
// behind the same seller tags. The bust lives inside SyncRedis rather than at
// each call site, because SyncRedis IS the convergence point for status changes
// — all six Recompute callers pass through it. The first version of this change
// wired three of those six by hand and silently missed the admin suspend/lift
// endpoints; testing the convergence point is what makes that class of miss
// impossible rather than merely unlikely.
func TestStatusRecomputer_SyncRedisBustsSellerCache(t *testing.T) {
	type payload struct {
		Tags []string `json:"tags"`
	}
	received := make(chan payload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		_ = json.NewDecoder(r.Body).Decode(&p)
		received <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{
				AccountID: accountID, StoreID: 700700700, StoreCode: 2600000001,
				Status: domain.SellerStatusSuspended,
			}, nil
		},
	}
	r := newSellerStatusRecomputer(sellerRepo, &mockSellerSuspensionRepository{},
		&mockSellerViolationRepository{}, nil,
		revalidate.New(revalidate.Config{BaseURL: srv.URL, Secret: "test-secret"}, nil))

	r.SyncRedis(context.Background(), 2623900520)

	select {
	case got := <-received:
		want := []string{"seller:2623900520", "seller:700700700", "seller:2600000001"}
		if !slices.Equal(got.Tags, want) {
			t.Errorf("status-change busted tags = %v, want %v", got.Tags, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a status change sent no cache-bust request")
	}
}

// A nil redis must not stop the website from being told. Those are two
// independent side effects of the same status change, and the original guard
// returned before either could run.
func TestStatusRecomputer_BustsEvenWithoutRedis(t *testing.T) {
	received := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: 1, StoreCode: 2, Status: domain.SellerStatusActive}, nil
		},
	}
	r := newSellerStatusRecomputer(sellerRepo, &mockSellerSuspensionRepository{},
		&mockSellerViolationRepository{}, nil,
		revalidate.New(revalidate.Config{BaseURL: srv.URL, Secret: "s"}, nil))

	r.SyncRedis(context.Background(), 2623900520)

	select {
	case <-received:
	case <-time.After(3 * time.Second):
		t.Fatal("nil redis suppressed the cache bust")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 56 §5.4：代行寫入（TargetByStoreID）清掉的必須是被指名那家店的 tag。
// ─────────────────────────────────────────────────────────────────────────────

// 代行寫入指名的是一家店，而「另一家店的 account_id 恰好等於這個 store_id」在 TSID
// 值域裡完全可能發生。bust 一旦拿手上的 id 回頭走 account 軸解析一次，在這個形狀上
// 就會解析到那另一家店：被編輯的店鋪頁繼續 stale，被清掉的是陌生人的頁面——而兩件事
// 都不會有任何錯誤訊號。所以 tag 只能由剛寫入的那一列組出來。
//
// fixture 與 seller_delegation_cas_test.go 的 twin repo 同形，理由也相同：本機 seed
// 全是 store_id == account_id 的首任賣家，在那份資料上任何身分軸錯置都看起來是對的。
func TestDelegatedWrites_BustTheTargetedStoreTags(t *testing.T) {
	const (
		targetStoreCode int64 = 2_600_000_001
		twinStoreID     int64 = 900_900_900
		twinStoreCode   int64 = 2_600_000_002
	)
	// 字面值：tag 是與 website 的線上契約（lib/seo/tags.ts），拿 revalidate.SellerTag()
	// 組 want 會讓斷言跟著實作一起錯。
	wantTags := []string{"seller:2685914682", "seller:700700700", "seller:2600000001"}
	// twin 專屬的兩個 tag。它的 account_id 與目標的 store_id 同值，那個 tag 兩邊都有，
	// 不能拿來判別是誰被清掉。
	twinOnlyTags := []string{"seller:900900900", "seller:2600000002"}

	target := func() *domain.Seller {
		return &domain.Seller{
			AccountID: casAccountID, StoreID: casStoreID, StoreCode: targetStoreCode,
			ContactPhone: "0912345678", // 已有值，讀取路徑不會再 backfill
			Status:       domain.SellerStatusActive, Version: 1,
		}
	}
	twin := func() *domain.Seller {
		return &domain.Seller{
			AccountID: casStoreID, StoreID: twinStoreID, StoreCode: twinStoreCode,
			Status: domain.SellerStatusActive, Version: 1,
		}
	}

	// fixture 的形狀就是這支測試的前提：兩根軸必須分岔，且 twin 的 account_id 必須
	// 恰好等於目標的 store_id。任一條不成立，軸錯置就打不到別家店，測試恆綠。
	if casAccountID == casStoreID {
		t.Fatalf("fixture 失效：account_id == store_id (%d)，首任賣家形狀藏得住軸錯置", casAccountID)
	}
	if twin().AccountID != casStoreID {
		t.Fatalf("fixture 失效：twin.account_id = %d，必須等於目標 store_id %d", twin().AccountID, casStoreID)
	}
	if slices.Contains(wantTags, twinOnlyTags[0]) || slices.Contains(wantTags, twinOnlyTags[1]) {
		t.Fatalf("fixture 失效：twin 專屬 tag %v 出現在目標 tag %v 裡，兩者無法區分", twinOnlyTags, wantTags)
	}

	mediaClient := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, _ []string, _ string) ([]AssetInfo, error) {
			return []AssetInfo{{AssetId: "new-asset", ObjectKey: "public/seller_logo/new.png"}}, nil
		},
		LinkImagesFn:        func(_ context.Context, _ string, _ string, _ []string) error { return nil },
		ReleaseReferencesFn: func(_ context.Context, _ string, _ []string, _ []string) error { return nil },
	}
	version := int64(1)

	cases := []struct {
		name   string
		invoke func(*SellerUseCase) error
	}{
		{"storefront", func(uc *SellerUseCase) error {
			return uc.UpdateProfile(context.Background(), &dto.UpdateSellerProfile{
				Target:          dto.TargetByStoreID(casStoreID),
				StoreName:       "代行改的店名",
				ExpectedVersion: &version,
			})
		}},
		{"logo", func(uc *SellerUseCase) error {
			_, err := uc.UpdateLogo(context.Background(), dto.TargetByStoreID(casStoreID), "staged-token", &version)
			return err
		}},
		{"cover", func(uc *SellerUseCase) error {
			_, err := uc.UpdateCover(context.Background(), dto.TargetByStoreID(casStoreID), "staged-token", &version)
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan []string, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Tags []string `json:"tags"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode bust body: %v", err)
				}
				received <- body.Tags
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			accountAxis := &accountAxisProbe{}
			repo := &mockSellerRepo{
				// account 軸：casStoreID 這個「值」在此軸上屬於 twin，casAccountID 才是目標。
				GetByAccountIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
					accountAxis.record(id)
					switch id {
					case casAccountID:
						return target(), nil
					case casStoreID:
						return twin(), nil
					}
					return nil, pkgerrors.NewNotFound("seller not found")
				},
				// store 軸：casStoreID 是目標店的 PK。
				GetCurrentByStoreIDFn: func(_ context.Context, id int64) (*domain.Seller, error) {
					switch id {
					case casStoreID:
						return target(), nil
					case twinStoreID:
						return twin(), nil
					}
					return nil, pkgerrors.NewNotFound("seller not found")
				},
				UpdateFn:      func(_ context.Context, _ *domain.Seller) error { return nil },
				UpdateLogoFn:  func(_ context.Context, _ int64, _ string) error { return nil },
				UpdateCoverFn: func(_ context.Context, _ int64, _ string) error { return nil },
			}
			uc := NewSellerUseCase(repo, nil, nil, testAccountClient, mediaClient,
				&config.Config{MediaBaseURL: "https://cdn.test"},
				revalidate.New(revalidate.Config{BaseURL: srv.URL, Secret: "test-secret"}, nil), nil,
				testTSID, &mockTxManager{}, &fakeSellerDeletedOutboxRepo{}, nil, testStoreCode, nil)

			if err := tc.invoke(uc); err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.name, err)
			}

			select {
			case got := <-received:
				if !slices.Equal(got, wantTags) {
					for _, tag := range twinOnlyTags {
						if slices.Contains(got, tag) {
							t.Fatalf("%s busted %v — 那是另一家店（store_id=%d，其 account_id 恰好等於目標 store_id %d）"+
								"的 tag 集合；被編輯的那家店 %v 反而繼續 stale",
								tc.name, got, twinStoreID, casStoreID, wantTags)
						}
					}
					t.Fatalf("%s busted %v, want %v", tc.name, got, wantTags)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("%s sent no cache-bust request", tc.name)
			}

			// 代行路徑從頭到尾只走 store 軸。任何一次 account 軸解析都代表有人拿手上的
			// id 回頭重新解釋身分，而該軸上的同一個值屬於 twin —— 這個 fixture 只是剛好
			// 讓它解析得回同一列，換一筆資料就不會。
			if ids := accountAxis.seen(); len(ids) > 0 {
				t.Fatalf("%s 在代行寫入途中以 %v 走了 account 軸解析；tag 必須取自剛寫入的那一列，"+
					"該軸上 store_id %d 屬於另一家店（store_id=%d）", tc.name, ids, casStoreID, twinStoreID)
			}
		})
	}
}

// accountAxisProbe records every account-axis lookup, so a test can assert a
// write path resolved identity exactly once — at its own entry point — instead
// of re-deriving it from whatever id it happens to be holding.
type accountAxisProbe struct {
	mu  sync.Mutex
	ids []int64
}

func (p *accountAxisProbe) record(id int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ids = append(p.ids, id)
}

func (p *accountAxisProbe) seen() []int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]int64(nil), p.ids...)
}

// accountID 0 means nothing upstream resolved a seller; `seller:0` is not a tag.
func TestStatusRecomputer_SkipsUnusableAccountID(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	r := newSellerStatusRecomputer(&mockSellerRepo{}, &mockSellerSuspensionRepository{},
		&mockSellerViolationRepository{}, nil,
		revalidate.New(revalidate.Config{BaseURL: srv.URL, Secret: "s"}, nil))
	r.bustSellerCache(context.Background(), 0, 1, 2)
	time.Sleep(200 * time.Millisecond)
	if called {
		t.Error("bust fired for account id 0")
	}
}
