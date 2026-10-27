package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// WISH_POOL 的 preimage 只有 mode ＋ policy id ＋ policy version。刻意不含賣家名單、
// 也不含任何賣家的當下狀態（停權會變，凍進去會讓任一賣家停權就作廢所有 approval）。
func TestWishPoolAudienceHash_IsStableAndPolicyOnly(t *testing.T) {
	a := WishPoolAudience()
	require.Equal(t, "WISH_POOL", a.Mode)
	require.Equal(t, "WISH_POOL_ALL_ACTIVE_SELLERS_V1", a.PolicyID)
	require.Equal(t, WishPoolPolicyVersionV1, a.PolicyVersion)
	require.Equal(t, 1, a.PolicyVersion)
	require.Empty(t, a.Recipients)

	h1, err := a.Hash()
	require.NoError(t, err)
	h2, err := WishPoolAudience().Hash()
	require.NoError(t, err)
	require.Equal(t, h1, h2, "同一個 policy 版本必須產生同一個 hash")
	require.Len(t, h1, 64, "SHA256 hex")
}

// DIRECT 的 recipient 順序是契約的一部分（#2450 Scope 2「ordered selected candidate
// IDs」）：換順序就是換受眾，hash 必須不同。
func TestDirectRecipientsAudienceHash_OrderIsSignificant(t *testing.T) {
	r1 := []PublicationRecipient{PlatformRecipient(11), PlatformRecipient(22)}
	r2 := []PublicationRecipient{PlatformRecipient(22), PlatformRecipient(11)}

	h1, err := DirectRecipientsAudience(900, 1, r1).Hash()
	require.NoError(t, err)
	h2, err := DirectRecipientsAudience(900, 1, r2).Hash()
	require.NoError(t, err)
	require.NotEqual(t, h1, h2)
}

// 兩支 pipeline 的 preimage 形狀不同，不得因為欄位剛好都空就撞成同一個 hash。
func TestAudienceHash_ModesNeverCollide(t *testing.T) {
	wish, err := WishPoolAudience().Hash()
	require.NoError(t, err)
	direct, err := DirectRecipientsAudience(0, 0, nil).Hash()
	require.NoError(t, err)
	require.NotEqual(t, wish, direct)
}

// candidate set 的版本進 preimage：同一組 store、不同 policy version 是不同受眾。
func TestDirectRecipientsAudienceHash_CandidateSetVersionIsSignificant(t *testing.T) {
	r := []PublicationRecipient{PlatformRecipient(11)}
	h1, err := DirectRecipientsAudience(900, 1, r).Hash()
	require.NoError(t, err)
	h2, err := DirectRecipientsAudience(900, 2, r).Hash()
	require.NoError(t, err)
	require.NotEqual(t, h1, h2)
}

// lane 是 recipient 身分的一部分：同一個 store 在 PLATFORM 與 EXTERNAL 兩條 lane 上
// 是兩個不同的報價對象（markup 不同、完成定義不同），不得因為 store_id 相同就撞 hash。
func TestDirectRecipientsAudienceHash_LaneIsSignificant(t *testing.T) {
	h1, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{PlatformRecipient(11)}).Hash()
	require.NoError(t, err)
	h2, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{ExternalRecipient(11)}).Hash()
	require.NoError(t, err)
	require.NotEqual(t, h1, h2)
}

// ⛔ Golden 回歸：支援 EXTERNAL 之前，這一組 PLATFORM 受眾算出來就是這個值。
// preimage 的欄位順序與 omitempty 是刻意安排的，就為了讓這個值**不會**因為新增
// ProspectID 而改變——既有 approval 的 audience_hash 是 execute 的 CAS 對象，
// 動到它等於讓所有 pending 核准作廢。
//
// 這條紅了代表 preimage 的位元組形狀變了。除非你真的打算作廢既有核准（那需要
// 一個明確的遷移決策，不是「順手整理欄位」），否則不要改這個期望值。
func TestDirectRecipientsAudienceHash_PlatformGoldenIsFrozen(t *testing.T) {
	got, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{
		PlatformRecipient(11), PlatformRecipient(22),
	}).Hash()
	require.NoError(t, err)
	require.Equal(t, "ad78a074c4fc27ca6ca562568a4a5be86f43d569ecf5123ff9e114b296cf7c1d", got)
}

// WISH_POOL 的 preimage 完全沒被這次改動碰到，同樣釘死。
// 這個值與 dev 上既有 approval 列的 audience_hash 相符（2026-09-08 核對）。
func TestWishPoolAudienceHash_GoldenIsFrozen(t *testing.T) {
	got, err := WishPoolAudience().Hash()
	require.NoError(t, err)
	require.Equal(t, "c2f6e24bc36db9ef3242279604ced6ecec90c9ae232f5dc67bcf95a9964f7e18", got)
}

// EXTERNAL 的身分是 prospect_id。⛔ 這條是整個改動的理由：修復前兩個不同的站外
// 業者都會序列化成 {"storeId":"0","lane":"EXTERNAL"} 而撞同一個 hash，買家核准
// 發給 A、execute 送給 B 也會通過 CAS。
func TestExternalRecipientsAudienceHash_DistinguishesProspects(t *testing.T) {
	h1, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{ExternalRecipient(9001)}).Hash()
	require.NoError(t, err)
	h2, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{ExternalRecipient(9002)}).Hash()
	require.NoError(t, err)
	require.NotEqual(t, h1, h2, "兩個不同的站外業者不得撞同一個 audienceHash")
}

// 兩條 lane 的 id 空間是分開的：store 42 與 prospect 42 是兩個不同的對象。
func TestAudienceHash_StoreAndProspectIDSpacesNeverCollide(t *testing.T) {
	platform, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{PlatformRecipient(42)}).Hash()
	require.NoError(t, err)
	external, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{ExternalRecipient(42)}).Hash()
	require.NoError(t, err)
	require.NotEqual(t, platform, external)
}

// 混合受眾的順序一樣有意義。
func TestAudienceHash_MixedLaneOrderIsSignificant(t *testing.T) {
	h1, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{PlatformRecipient(11), ExternalRecipient(9001)}).Hash()
	require.NoError(t, err)
	h2, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{ExternalRecipient(9001), PlatformRecipient(11)}).Hash()
	require.NoError(t, err)
	require.NotEqual(t, h1, h2)
}

// lane 與 id 欄位配錯時 Hash 必須失敗，不得算出一個看起來合法的值——
// 那種 hash 會靜默通過 execute 的 CAS。
func TestAudienceHash_RejectsIdentityMismatch(t *testing.T) {
	cases := map[string]PublicationRecipient{
		"EXTERNAL 卻填 StoreID":    {Lane: QuoteLaneExternal, StoreID: 11},
		"PLATFORM 卻填 ProspectID": {Lane: QuoteLanePlatform, ProspectID: 9001},
		"PLATFORM 但兩個都填":         {Lane: QuoteLanePlatform, StoreID: 11, ProspectID: 9001},
		"PLATFORM 但 id 是 0":      {Lane: QuoteLanePlatform},
		"EXTERNAL 但 id 是 0":      {Lane: QuoteLaneExternal},
		"未知 lane":                {Lane: QuoteLane("WHOLESALE"), StoreID: 11},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DirectRecipientsAudience(900, 1, []PublicationRecipient{r}).Hash()
			require.ErrorIs(t, err, ErrRecipientIdentityMismatch)
		})
	}
}
