package grpc

import (
	"context"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// 鏡射 handler_test.go 的 TestEnrichCommentUserInfo_* 系列——enrichForumUserInfo
// 在 2026-08-08 之前無條件只查 buyerClient，這批測試把 #2223（見
// TestEnrichCommentUserInfo_NamespacesDoNotLeak）當時只修 enrichCommentUserInfo、
// 沒有連帶修這支討論區版本的缺口補上同等覆蓋率。

func makeForumEntryWithRole(id, sellerID, userID int64, role int16) *gen.CmsSellerForum {
	return &gen.CmsSellerForum{
		ID:         id,
		SellerID:   sellerID,
		UserID:     userID,
		Role:       role,
		Content:    "test",
		CreateTime: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func forumInfos(rows []*gen.CmsSellerForum) []*pb.ForumEntryInfo {
	out := make([]*pb.ForumEntryInfo, len(rows))
	for i, r := range rows {
		out[i] = toForumEntryInfo(r, usecase.Caller{})
	}
	return out
}

func TestEnrichForumUserInfo_BuyerRole(t *testing.T) {
	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				100: {Nickname: "Alice", Avatar: "alice.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: &mockSellerClientH{},
		logger:       zap.NewNop(),
	}
	rows := []*gen.CmsSellerForum{makeForumEntryWithRole(1, 10, 100, 1)}

	srv.enrichForumUserInfo(context.Background(), rows)
	data := forumInfos(rows)

	if data[0].UserName != "Alice" || data[0].UserAvatarKey != "alice.jpg" {
		t.Errorf("UserName=%q Avatar=%q, want Alice / alice.jpg", data[0].UserName, data[0].UserAvatarKey)
	}
}

func TestEnrichForumUserInfo_SellerRole(t *testing.T) {
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				300: {Nickname: "我的店", Avatar: "shop.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  &mockBuyerClientH{},
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}
	// role=2：賣家在自己討論區的回覆——這正是原本 buyer-only enrich 永遠查空、
	// 頭像/名稱恆為 DB denormalized 空值的那個症狀（page_echo_tab.dart 的註解
	// 描述的現象）。
	rows := []*gen.CmsSellerForum{makeForumEntryWithRole(2, 300, 300, 2)}

	srv.enrichForumUserInfo(context.Background(), rows)
	data := forumInfos(rows)

	if data[0].UserName != "我的店" || data[0].UserAvatarKey != "shop.jpg" {
		t.Errorf("UserName=%q Avatar=%q, want 我的店 / shop.jpg", data[0].UserName, data[0].UserAvatarKey)
	}
}

func TestEnrichForumUserInfo_MixedRoles(t *testing.T) {
	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				100: {Nickname: "BuyerAlice", Avatar: "buyer_alice.jpg"},
			}, nil
		},
	}
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				200: {Nickname: "SellerShop", Avatar: "shop.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}
	rows := []*gen.CmsSellerForum{
		makeForumEntryWithRole(1, 10, 100, 1),
		makeForumEntryWithRole(2, 10, 200, 2),
	}

	srv.enrichForumUserInfo(context.Background(), rows)
	data := forumInfos(rows)

	if data[0].UserName != "BuyerAlice" {
		t.Errorf("buyer UserName = %q, want BuyerAlice", data[0].UserName)
	}
	if data[1].UserName != "SellerShop" {
		t.Errorf("seller UserName = %q, want SellerShop", data[1].UserName)
	}
}

func TestEnrichForumUserInfo_BuyerServiceDown_Fallback(t *testing.T) {
	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return make(map[int64]*usecase.AccountInfo), nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: &mockSellerClientH{},
		logger:       zap.NewNop(),
	}

	e := makeForumEntryWithRole(1, 10, 100, 1)
	e.UserName = stringPointer("DBName")
	e.UserAvatar = stringPointer("db_avatar.jpg")
	rows := []*gen.CmsSellerForum{e}

	srv.enrichForumUserInfo(context.Background(), rows)
	data := forumInfos(rows)

	if data[0].UserName != "DBName" || data[0].UserAvatarKey != "db_avatar.jpg" {
		t.Errorf("UserName=%q Avatar=%q, want DBName / db_avatar.jpg (DB fallback)", data[0].UserName, data[0].UserAvatarKey)
	}
}

func TestEnrichForumUserInfo_SellerServiceDown_Fallback(t *testing.T) {
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(ctx context.Context, ids []int64) (map[int64]*usecase.AccountInfo, error) {
			return make(map[int64]*usecase.AccountInfo), nil
		},
	}
	srv := &ContentServer{
		buyerClient:  &mockBuyerClientH{},
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}

	e := makeForumEntryWithRole(1, 300, 300, 2)
	e.UserName = stringPointer("DBSeller")
	rows := []*gen.CmsSellerForum{e}

	srv.enrichForumUserInfo(context.Background(), rows)
	data := forumInfos(rows)

	if data[0].UserName != "DBSeller" {
		t.Errorf("UserName = %q, want DBSeller (DB fallback)", data[0].UserName)
	}
}

// TestEnrichForumUserInfo_NamespacesDoNotLeak：同一個數值 id 若一則是買家、一則
// 是賣家（buyer_id 與 store_id 是兩個獨立命名空間，同數值可以是不同的人），兩邊
// 查表結果不可互相污染。鏡射 TestEnrichCommentUserInfo_NamespacesDoNotLeak。
func TestEnrichForumUserInfo_NamespacesDoNotLeak(t *testing.T) {
	const collidingID = 100

	buyerClient := &mockBuyerClientH{
		getBuyerMetaBatchFn: func(_ context.Context, _ []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				collidingID: {Nickname: "買家小明", Avatar: "buyer.jpg"},
			}, nil
		},
	}
	sellerClient := &mockSellerClientH{
		getSellerMetaBatchFn: func(_ context.Context, _ []int64) (map[int64]*usecase.AccountInfo, error) {
			return map[int64]*usecase.AccountInfo{
				collidingID: {Nickname: "範例甲商行", Avatar: "seller.jpg"},
			}, nil
		},
	}
	srv := &ContentServer{
		buyerClient:  buyerClient,
		sellerClient: sellerClient,
		logger:       zap.NewNop(),
	}

	rows := []*gen.CmsSellerForum{
		makeForumEntryWithRole(1, 10, collidingID, 1), // role=1 買家
		makeForumEntryWithRole(2, 20, collidingID, 2), // role=2 賣家
	}

	srv.enrichForumUserInfo(context.Background(), rows)
	data := forumInfos(rows)

	if data[0].UserName != "買家小明" || data[0].UserAvatarKey != "buyer.jpg" {
		t.Errorf("買家列被賣家資料污染：UserName=%q Avatar=%q，want 買家小明 / buyer.jpg",
			data[0].UserName, data[0].UserAvatarKey)
	}
	if data[1].UserName != "範例甲商行" || data[1].UserAvatarKey != "seller.jpg" {
		t.Errorf("賣家列被買家資料污染：UserName=%q Avatar=%q，want 範例甲商行 / seller.jpg",
			data[1].UserName, data[1].UserAvatarKey)
	}
}

// TestEnrichForumUserInfo_UnresolvedRoleKeepsSnapshot：查不到就保留 DB 快照值，
// 不可因為「另一個命名空間剛好有這個 id」而套上別人的身分。
func TestEnrichForumUserInfo_UnresolvedRoleKeepsSnapshot(t *testing.T) {
	srv := &ContentServer{
		buyerClient: &mockBuyerClientH{}, // 回空 map：買家查無此人
		sellerClient: &mockSellerClientH{
			getSellerMetaBatchFn: func(_ context.Context, _ []int64) (map[int64]*usecase.AccountInfo, error) {
				return map[int64]*usecase.AccountInfo{777: {Nickname: "某商行", Avatar: "s.jpg"}}, nil
			},
		},
		logger: zap.NewNop(),
	}
	buyerRow := makeForumEntryWithRole(1, 10, 777, 1)
	sellerRow := makeForumEntryWithRole(2, 20, 777, 2)
	before := optionalString(buyerRow.UserName)

	srv.enrichForumUserInfo(context.Background(), []*gen.CmsSellerForum{buyerRow, sellerRow})

	if got := optionalString(buyerRow.UserName); got != before {
		t.Errorf("買家列查無資料卻被覆寫成 %q（原 %q）", got, before)
	}
	if got := optionalString(sellerRow.UserName); got != "某商行" {
		t.Errorf("賣家列 UserName = %q, want 某商行", got)
	}
}
