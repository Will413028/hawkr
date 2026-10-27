package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
)

// ADM 貼文審核佇列必須顯示得出「已軟刪賣家」的店名（#2818 同 class）。
//
// 觸發鏈是日常的：賣家刪帳號 → seller.deleted → SoftDeleteBySellerID 把該賣家全部貼文軟刪
// → 那些貼文正好落在 AdminListPosts 的 deleted 分頁 → 審核者要看的就是「這是誰的貼文」。
//
// 🔴 fixture 的形狀本身就是陰性保證：getSellerInfo 回 nil（模擬 seller-service 那支帶
// `AND deleted_at IS NULL` 的公開視角查詢把軟刪賣家濾掉），只有 GetSellerMetaBatch 回得出名字。
// ⇒ 只有真的走 batch 這條路才會過。把 resolveSellerDisplay 改回 GetSellerInfo，這支必轉紅。
//
// gateway 對 AdminListPosts **不做** enrichPostsWithSellerInfo（公開 feed 的三個端點才做），
// 所以這裡是 ADM 賣家名的唯一來源，沒有下游兜底。
type adminSellerNameRepo struct {
	stubPostRepo
	rows []gen.AdminListPostsRow
}

func (r *adminSellerNameRepo) AdminList(_ context.Context, _ string, _, _ int64) ([]gen.AdminListPostsRow, error) {
	return r.rows, nil
}

func (r *adminSellerNameRepo) AdminCount(_ context.Context, _ string) (int64, error) {
	return int64(len(r.rows)), nil
}

func newAdminSellerNameServer(repo *adminSellerNameRepo, seller usecase.SellerClient) *ContentServer {
	postUC := usecase.NewPostUseCase(
		repo,
		usecase.NewHashtagUseCase(nil, func() int64 { return 1 }),
		nil, nil,
		nil,
		seller,
		&stubBuyerClient{},
		nil,
		nil,
		&config.Config{},
		func() int64 { return 1 },
		nil,
	)
	return NewContentServer(postUC, nil, nil, nil, nil, nil, &stubBuyerClient{}, seller, nil, zap.NewNop())
}

func TestAdminListPosts_SoftDeletedSeller_StillShowsSellerName(t *testing.T) {
	const storeID int64 = 2685914682
	content := "已刪除賣家的貼文"
	deletedAt := time.Unix(1787800000, 0)

	repo := &adminSellerNameRepo{rows: []gen.AdminListPostsRow{{
		ID:         857045288972403572,
		SellerID:   storeID,
		Content:    &content,
		DeletedAt:  &deletedAt,
		CreateTime: pgtype.Timestamptz{Time: time.Unix(1787700000, 0), Valid: true},
	}}}

	seller := &mockSellerClientH{
		// 公開視角查詢：軟刪賣家被 `AND deleted_at IS NULL` 濾掉 ⇒ 查無
		getSellerInfoFn: func(_ context.Context, _ int64) (*usecase.AccountInfo, error) {
			return nil, nil
		},
		// PK lookup，無可見性過濾 ⇒ 名字還在
		getSellerMetaBatchFn: func(_ context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			out := make(map[int64]*usecase.AccountInfo, len(ids))
			for _, id := range ids {
				out[id] = &usecase.AccountInfo{Nickname: "已刪除的測試賣家", Avatar: "logo/abc.png"}
			}
			return out, nil
		},
	}

	srv := newAdminSellerNameServer(repo, seller)
	resp, err := srv.AdminListPosts(context.Background(), &pb.AdminListPostsRequest{
		Status: "deleted", Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("AdminListPosts error = %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d, message = %q", resp.GetCode(), resp.GetMessage())
	}
	if len(resp.GetData()) != 1 {
		t.Fatalf("records = %d, want 1", len(resp.GetData()))
	}
	if got := resp.GetData()[0].GetSellerName(); got != "已刪除的測試賣家" {
		t.Errorf("sellerName = %q, want %q——空字串代表又走回帶可見性過濾的那支查詢，"+
			"ADM 的「已刪除」分頁會只剩一串 id", got, "已刪除的測試賣家")
	}
}
