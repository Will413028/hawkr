package usecase

import (
	"context"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// SSE 新留言頭像（issue #2223）。
//
// 症狀：`comment.created` / `comment.replied` 的 avatarKey 是空字串，重拉一次列表才有頭像。
// 成因不是「欄位從未被寫入」（實測 dev 125 列中 101 列有值），而是**兩條路徑的資料來源不同**：
//   - REST 列表：social gRPC 層 enrichCommentUsers 於**讀取時**現查並覆蓋 DB 的反正規化值
//   - SSE 快照：buildCommentSnapshot 讀 `cms_comment.user_avatar` 這個**建立當下寫死**的欄位
//
// 於是只要建立那一刻解析失敗（單筆 client 把 transient error 攤成空字串、且只記 Debug log），
// 那一幀的頭像就永久是空的，而列表在下一次讀取自癒。本機實測坐實過同一則留言
// SSE avatarKey="" 而 REST 回 objectKey。
//
// 修法採 issue 待辦的第二個選項「與 REST 一致」：身分由 resolveCommentIdentity 解析
// （*MetaBatch，與 enrichCommentUsers 同一組 RPC，查不到會重試一次），快照直接用解析結果，
// 不再碰那個可為 NULL 的欄位。

// 快照必須用傳入的解析結果，不得回頭讀 row 的反正規化欄位——
// 這裡刻意讓 row 帶「舊值」而傳入「新值」，讀錯來源就會轉紅。
func TestBuildCommentSnapshot_UsesResolvedIdentityNotRow(t *testing.T) {
	row := &gen.CmsComment{
		ID: 1, PostID: 10, UserID: 100, Role: 1,
		UserName:   testStringPointer("row 裡的舊名字"),
		UserAvatar: testStringPointer("row/stale_avatar.jpg"),
		Content:    testStringPointer("hi"),
	}
	snap := buildCommentSnapshot(row, "解析出的名字", "resolved/avatar.jpg")
	if snap.UserName != "解析出的名字" {
		t.Errorf("snap.UserName = %q, want 解析出的名字（讀到 row 的值＝來源錯了）", snap.UserName)
	}
	if snap.AvatarKey != "resolved/avatar.jpg" {
		t.Errorf("snap.AvatarKey = %q, want resolved/avatar.jpg（讀到 row 的值＝來源錯了）", snap.AvatarKey)
	}
}

// row 的欄位是 NULL 時，快照仍應帶解析出的頭像——這正是 #2223 的使用者可見症狀。
func TestBuildCommentSnapshot_NullColumnStillCarriesAvatar(t *testing.T) {
	row := &gen.CmsComment{ID: 1, PostID: 10, UserID: 100, Role: 1} // user_name / user_avatar 皆 NULL
	snap := buildCommentSnapshot(row, "王小明", "public/buyer_avatar/2001/a.jpg")
	if snap.AvatarKey != "public/buyer_avatar/2001/a.jpg" {
		t.Errorf("snap.AvatarKey = %q，NULL 欄位又把頭像吃掉了（#2223 回歸）", snap.AvatarKey)
	}
	if snap.UserName != "王小明" {
		t.Errorf("snap.UserName = %q, want 王小明", snap.UserName)
	}
}

// 第一次查不到就重試一次：涵蓋 transient 失敗。單筆 client 做不到這件事，
// 因為它把「查不到」與「查到但沒設頭像」都攤成空字串，重試會變成每則留言都多打一次。
func TestResolveCommentIdentity_RetriesOnceOnMiss(t *testing.T) {
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
	uc := NewCommentUseCase(&mockCommentRepo{}, &mockPostRepo{}, buyer, &mockSellerClient{}, nil, func() int64 { return 1 })

	name, avatar := uc.resolveCommentIdentity(context.Background(), Actor{Kind: domain.ActorKindBuyer, ID: 100})
	if calls != 2 {
		t.Errorf("lookup 次數 = %d, want 2（查不到要重試一次）", calls)
	}
	if name != "王小明" || avatar != "public/a.jpg" {
		t.Errorf("resolveCommentIdentity = (%q, %q), want (王小明, public/a.jpg)", name, avatar)
	}
}

// 查到但沒設頭像 ≠ 查不到：命中就直接用，不得再多打一次。
func TestResolveCommentIdentity_NoRetryWhenFoundWithEmptyAvatar(t *testing.T) {
	calls := 0
	buyer := &mockBuyerClient{
		getBuyerMetaBatchFn: func(_ context.Context, ids []int64) (map[int64]*AccountInfo, error) {
			calls++
			return map[int64]*AccountInfo{ids[0]: {Nickname: "沒頭像的人", Avatar: ""}}, nil
		},
	}
	uc := NewCommentUseCase(&mockCommentRepo{}, &mockPostRepo{}, buyer, &mockSellerClient{}, nil, func() int64 { return 1 })

	name, avatar := uc.resolveCommentIdentity(context.Background(), Actor{Kind: domain.ActorKindBuyer, ID: 100})
	if calls != 1 {
		t.Errorf("lookup 次數 = %d, want 1（命中就不該重試，否則每則無頭像留言都多打一次 RPC）", calls)
	}
	if name != "沒頭像的人" || avatar != "" {
		t.Errorf("resolveCommentIdentity = (%q, %q), want (沒頭像的人, \"\")", name, avatar)
	}
}

// 賣家留言走 seller 的 store surrogate，不是 userID。
func TestResolveCommentIdentity_SellerUsesStoreID(t *testing.T) {
	const storeID int64 = 50
	var gotIDs []int64
	seller := &mockSellerClient{
		getSellerMetaBatchFn: func(_ context.Context, ids []int64) (map[int64]*AccountInfo, error) {
			gotIDs = ids
			return map[int64]*AccountInfo{ids[0]: {Nickname: "陳記", Avatar: "public/seller_logo.jpg"}}, nil
		},
	}
	uc := NewCommentUseCase(&mockCommentRepo{}, &mockPostRepo{}, &mockBuyerClient{}, seller, nil, func() int64 { return 1 })

	name, avatar := uc.resolveCommentIdentity(context.Background(), Actor{Kind: domain.ActorKindStore, ID: storeID})
	if len(gotIDs) != 1 || gotIDs[0] != storeID {
		t.Errorf("seller lookup ids = %v, want [%d]（不能拿 raw account_id 去查）", gotIDs, storeID)
	}
	if name != "陳記" || avatar != "public/seller_logo.jpg" {
		t.Errorf("resolveCommentIdentity = (%q, %q), want (陳記, public/seller_logo.jpg)", name, avatar)
	}
}
