package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/review-service/internal/config"
	"github.com/Will413028/hawkr/review-service/internal/domain"
)

// #2830：公開評論的作者名，在買家註銷之後必須仍然顯示得出來。
//
// 症狀（dev 實測）：`GET /public/products/{id}/reviews` 對已註銷買家回
// `username=”`，18 筆全空。成因是取名字走了帶 `deleted_at IS NULL` 的
// BatchGetBuyerMetaByBuyerIDs，而 order_item_reviews **沒有名字快照欄**，
// miss 就等於真的沒有名字可退。
//
// 🔴 「不會退回舊 RPC」的保證來自**編譯期**，不是本 fixture：`BuyerClient`
// 介面（buyer_client.go）只宣告 Display 一支，usecase 改回 Meta 就編不過。
// 那比 runtime 斷言強，但**成立條件是介面維持單一方法** —— 若日後有人把
// Meta 加回介面，這道保證會無聲失效，屆時本 fixture 要補一支
// BatchGetBuyerMetaByBuyerIDs 並在其中呼叫 c.t.Fatal 把它變成 runtime 保證。
//
// （2026-08-29 更正：本註解原本宣稱 fixture 已經這麼做了，實際上沒有——
//
//	fixture 從來只有 Display 一支。描述了一個不存在的機制，比沒有註解更糟。）
type displayOnlyBuyerClient struct {
	t         *testing.T
	displayFn func(ctx context.Context, ids []int64) (map[int64]*BuyerMeta, error)
}

func (c *displayOnlyBuyerClient) BatchGetBuyerDisplayByBuyerIDs(ctx context.Context, ids []int64) (map[int64]*BuyerMeta, error) {
	c.t.Helper()
	if len(ids) == 0 {
		c.t.Fatal("usecase 不該對空清單發 RPC")
	}
	return c.displayFn(ctx, ids)
}

type displayStubRepo struct {
	mockReviewRepo
	reviews []*domain.Review
}

func (r *displayStubRepo) ListByProductID(_ context.Context, _ int64, _ int32, _ bool, _, _ int32) ([]*domain.Review, error) {
	return r.reviews, nil
}
func (r *displayStubRepo) CountByProductID(_ context.Context, _ int64, _ int32, _ bool) (int64, error) {
	return int64(len(r.reviews)), nil
}
func (r *displayStubRepo) GetStatsByProductID(_ context.Context, _ int64) (*domain.ProductStats, error) {
	return &domain.ProductStats{}, nil
}

func TestGetProductReviews_ClosedBuyer_StillHasDisplayName(t *testing.T) {
	const closedBuyer int64 = 2683479278

	repo := &displayStubRepo{reviews: []*domain.Review{
		{ID: 1, BuyerID: closedBuyer, ProductID: 828988498559926931, Rating: 5},
	}}
	buyerClient := &displayOnlyBuyerClient{t: t,
		displayFn: func(_ context.Context, ids []int64) (map[int64]*BuyerMeta, error) {
			// buyer-service 對已註銷 stint 回墓碑字串（真名不離開它的 DB）
			return map[int64]*BuyerMeta{
				closedBuyer: {Nickname: "已刪除帳號", AvatarURL: "", Deleted: true},
			}, nil
		}}

	uc := NewReviewUseCase(repo, nil, nil, buyerClient, nil, nil,
		&config.Config{}, func() int64 { return 1 }, nil)

	out, err := uc.GetProductReviews(context.Background(), GetProductReviewsInput{
		ProductID: 828988498559926931, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("GetProductReviews error = %v", err)
	}
	if len(out.Reviews) != 1 {
		t.Fatalf("reviews = %d, want 1", len(out.Reviews))
	}
	if got := out.Reviews[0].BuyerName; got != "已刪除帳號" {
		t.Errorf("BuyerName = %q, want %q——空字串代表又走回帶 deleted_at 過濾的那支 RPC，"+
			"公開評論的作者欄會整片空白", got, "已刪除帳號")
	}
	// 陰性：頭像對已註銷者必須是空的（頭像是個資，不隨墓碑一起回）
	if got := out.Reviews[0].BuyerAvatar; got != "" {
		t.Errorf("BuyerAvatar = %q, want 空——已註銷買家的頭像不得外流", got)
	}
}

// 陰性：未註銷的買家必須拿到真名，不能因為修復而一律變墓碑。
func TestGetProductReviews_LiveBuyer_KeepsRealName(t *testing.T) {
	const liveBuyer int64 = 2623900520

	repo := &displayStubRepo{reviews: []*domain.Review{
		{ID: 2, BuyerID: liveBuyer, ProductID: 1, Rating: 4},
	}}
	buyerClient := &displayOnlyBuyerClient{t: t,
		displayFn: func(_ context.Context, _ []int64) (map[int64]*BuyerMeta, error) {
			return map[int64]*BuyerMeta{
				liveBuyer: {Nickname: "王小明", AvatarURL: "avatar/abc.png", Deleted: false},
			}, nil
		}}

	uc := NewReviewUseCase(repo, nil, nil, buyerClient, nil, nil,
		&config.Config{}, func() int64 { return 1 }, nil)

	out, err := uc.GetProductReviews(context.Background(), GetProductReviewsInput{
		ProductID: 1, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("GetProductReviews error = %v", err)
	}
	if got := out.Reviews[0].BuyerName; got != "王小明" {
		t.Errorf("BuyerName = %q, want 王小明——沒有這一條的話，"+
			"「一律回墓碑」的錯誤實作也會讓上面那條通過", got)
	}
	if out.Reviews[0].BuyerAvatar == "" {
		t.Error("未註銷買家的頭像不該是空的")
	}
}
