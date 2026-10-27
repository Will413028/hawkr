package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/review-service/internal/domain"
)

// A seller's storefront (and the review stats / review list rendered on it) is
// cached by URL path segment, and that segment is store_code (canonical) or
// store_id (legacy links). A review write holds store_id only, so busting that
// alone leaves the canonical page stale for any seller whose ids diverge.
func TestReviewUC_sellerCacheTags(t *testing.T) {
	const storeID = int64(700700700)

	tests := []struct {
		name       string
		storeID    int64
		noClient   bool
		storeCode  string
		resolveErr error
		want       []string
	}{
		{
			name:      "diverged ids: store_id + store_code both busted",
			storeID:   storeID,
			storeCode: "2600000001",
			want:      []string{"seller:700700700", "seller:2600000001"},
		},
		{
			name:      "store_code backfilled from the bound MerID: one tag, no duplicates",
			storeID:   storeID,
			storeCode: "700700700",
			want:      []string{"seller:700700700"},
		},
		{
			name:       "resolution fails: fail-soft to the store_id tag",
			storeID:    storeID,
			resolveErr: errors.New("connection refused"),
			want:       []string{"seller:700700700"},
		},
		{
			name:      "no store_code on the wire: store_id tag still goes out",
			storeID:   storeID,
			storeCode: "",
			want:      []string{"seller:700700700"},
		},
		{
			name:      "store_code zero (pre-backfill row): no seller:0 tag",
			storeID:   storeID,
			storeCode: "0",
			want:      []string{"seller:700700700"},
		},
		{
			name:      "non-numeric store_code: no panic, store_id tag still goes out",
			storeID:   storeID,
			storeCode: "26abcdefgh",
			want:      []string{"seller:700700700"},
		},
		{
			name:     "no seller client wired: store_id tag still goes out",
			storeID:  storeID,
			noClient: true,
			want:     []string{"seller:700700700"},
		},
		{
			name:      "unknown store_id: nothing to bust",
			storeID:   0,
			storeCode: "2600000001",
			want:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sellerClient SellerClient
			if !tt.noClient {
				sellerClient = &mockSellerClient{
					ResolveStoreCodeFn: func(_ context.Context, gotStoreID int64) (string, error) {
						if gotStoreID != tt.storeID {
							t.Errorf("ResolveStoreCode(%d), want store_id %d", gotStoreID, tt.storeID)
						}
						return tt.storeCode, tt.resolveErr
					},
				}
			}
			uc := NewReviewUseCase(&mockReviewRepo{}, &mockOrderClient{}, nil, nil, sellerClient, nil,
				newTestConfig(), fixedTSID(), nil)

			got := uc.sellerCacheTags(context.Background(), tt.storeID)
			if !slices.Equal(got, tt.want) {
				t.Errorf("sellerCacheTags = %v, want %v", got, tt.want)
			}
		})
	}
}

// Every review write path that moves a rating aggregate must bust all the id
// spaces the seller's pages are cached under, on the wire, in the documented
// {"tags": [...]} shape — one stale page is the whole bug (#2033).
func TestReviewUC_WritePathsBustEveryIDSpace(t *testing.T) {
	const (
		buyerID   = int64(100)
		storeID   = int64(700700700)
		storeCode = "2600000001"
		productID = int64(825045450150940300)
		itemID    = int64(999)
		reviewID  = int64(123)
	)
	wantTags := []string{"product:825045450150940300", "seller:700700700", "seller:2600000001"}

	paths := []struct {
		name   string
		invoke func(*ReviewUseCase) error
	}{
		{
			name: "SubmitItemReview",
			invoke: func(uc *ReviewUseCase) error {
				_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
					OrderNo:     "ORD-001",
					OrderItemID: itemID,
					BuyerID:     buyerID,
					Rating:      5,
					Content:     "很新鮮",
				})
				return err
			},
		},
		{
			name: "HideReview",
			invoke: func(uc *ReviewUseCase) error {
				return uc.HideReview(context.Background(), reviewID, "moderation")
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

			repo := &mockReviewRepo{
				GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) { return nil, nil },
				InsertFn:           func(_ context.Context, r *domain.Review) (*domain.Review, error) { return r, nil },
				GetByIDFn: func(_ context.Context, id int64) (*domain.Review, error) {
					return &domain.Review{ID: id, StoreID: storeID, ProductID: productID}, nil
				},
			}
			orderClient := &mockOrderClient{
				GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
					return completedOrder(buyerID, storeID, time.Now().Add(-24*time.Hour)),
						[]*domain.OrderItemInfo{{OrderItemID: itemID, ProductID: productID, SkuName: "蘋果", Price: 100}}, nil
				},
			}

			sellerClient := &mockSellerClient{
				ResolveStoreCodeFn: func(_ context.Context, gotStoreID int64) (string, error) {
					if gotStoreID != storeID {
						t.Errorf("ResolveStoreCode(%d), want store_id %d", gotStoreID, storeID)
					}
					return storeCode, nil
				},
			}
			uc := NewReviewUseCase(repo, orderClient, &mockMediaClient{}, nil, sellerClient, nil,
				newTestConfig(), fixedTSID(),
				revalidate.New(revalidate.Config{BaseURL: srv.URL, Secret: "test-secret"}, nil))

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

// An unwired (nil) or unconfigured client means the bust would be dropped
// anyway (local dev without WEB_REVALIDATE_*): short-circuit before the
// store_code lookup so a disabled bust costs no RPC.
func TestReviewUC_revalidateDisabledSkipsStoreCodeLookup(t *testing.T) {
	clients := map[string]*revalidate.Client{
		"nil client":                nil,
		"missing base url / secret": revalidate.New(revalidate.Config{}, nil),
	}
	for name, client := range clients {
		t.Run(name, func(t *testing.T) {
			sellerClient := &mockSellerClient{
				ResolveStoreCodeFn: func(_ context.Context, _ int64) (string, error) {
					t.Error("store_code lookup ran although revalidate is disabled")
					return "", nil
				},
			}
			uc := NewReviewUseCase(&mockReviewRepo{}, &mockOrderClient{}, nil, nil, sellerClient, nil,
				newTestConfig(), fixedTSID(), client)

			uc.revalidateProductAndSeller(context.Background(), 825045450150940300, 700700700)
		})
	}
}
