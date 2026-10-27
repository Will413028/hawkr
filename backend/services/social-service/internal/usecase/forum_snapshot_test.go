package usecase

import (
	"context"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"testing"
)

// 鏡射 comment_snapshot_test.go 的 TestResolveCommentIdentity_* 系列——
// resolveForumIdentity 是同一套邏輯的討論區版本，寫路徑的三個核心不變量
// （查不到重試一次／命中不重試／賣家走 store surrogate）需要同等覆蓋。

// 第一次查不到就重試一次：涵蓋 transient 失敗。
func TestResolveForumIdentity_RetriesOnceOnMiss(t *testing.T) {
	calls := 0
	buyer := &mockBuyerClient{
		getBuyerMetaBatchFn: func(_ context.Context, ids []int64) (map[int64]*AccountInfo, error) {
			calls++
			if calls == 1 {
				return map[int64]*AccountInfo{}, nil // 第一次：transient 失敗 / 查不到
			}
			return map[int64]*AccountInfo{ids[0]: {Nickname: "王小明", Avatar: "public/a.jpg"}}, nil
		},
	}
	uc := NewForumUseCase(&mockForumRepo{}, buyer, &mockSellerClient{}, func() int64 { return 1 })

	name, avatar := uc.resolveForumIdentity(context.Background(), Actor{Kind: domain.ActorKindBuyer, ID: 100})
	if calls != 2 {
		t.Errorf("lookup 次數 = %d, want 2（查不到要重試一次）", calls)
	}
	if name != "王小明" || avatar != "public/a.jpg" {
		t.Errorf("resolveForumIdentity = (%q, %q), want (王小明, public/a.jpg)", name, avatar)
	}
}

// 查到但沒設頭像 ≠ 查不到：命中就直接用，不得再多打一次。
func TestResolveForumIdentity_NoRetryWhenFoundWithEmptyAvatar(t *testing.T) {
	calls := 0
	buyer := &mockBuyerClient{
		getBuyerMetaBatchFn: func(_ context.Context, ids []int64) (map[int64]*AccountInfo, error) {
			calls++
			return map[int64]*AccountInfo{ids[0]: {Nickname: "沒頭像的人", Avatar: ""}}, nil
		},
	}
	uc := NewForumUseCase(&mockForumRepo{}, buyer, &mockSellerClient{}, func() int64 { return 1 })

	name, avatar := uc.resolveForumIdentity(context.Background(), Actor{Kind: domain.ActorKindBuyer, ID: 100})
	if calls != 1 {
		t.Errorf("lookup 次數 = %d, want 1（命中就不該重試，否則每則無頭像留言都多打一次 RPC）", calls)
	}
	if name != "沒頭像的人" || avatar != "" {
		t.Errorf("resolveForumIdentity = (%q, %q), want (沒頭像的人, \"\")", name, avatar)
	}
}

// 賣家留言走 seller 的 store surrogate，不是 userID——這正是本次修復的核心：
// 修復前 forum.go 無條件呼叫 buyerClient，這條測試鎖定「role=2 時查表用的是
// storeID 不是 userID」，防止日後回歸成無條件查買家。
func TestResolveForumIdentity_SellerUsesStoreID(t *testing.T) {
	const storeID int64 = 50
	var gotIDs []int64
	seller := &mockSellerClient{
		getSellerMetaBatchFn: func(_ context.Context, ids []int64) (map[int64]*AccountInfo, error) {
			gotIDs = ids
			return map[int64]*AccountInfo{ids[0]: {Nickname: "陳記", Avatar: "public/seller_logo.jpg"}}, nil
		},
	}
	uc := NewForumUseCase(&mockForumRepo{}, &mockBuyerClient{}, seller, func() int64 { return 1 })

	name, avatar := uc.resolveForumIdentity(context.Background(), Actor{Kind: domain.ActorKindStore, ID: storeID})
	if len(gotIDs) != 1 || gotIDs[0] != storeID {
		t.Errorf("seller lookup ids = %v, want [%d]（不能拿 raw account_id/userID 去查）", gotIDs, storeID)
	}
	if name != "陳記" || avatar != "public/seller_logo.jpg" {
		t.Errorf("resolveForumIdentity = (%q, %q), want (陳記, public/seller_logo.jpg)", name, avatar)
	}
}
