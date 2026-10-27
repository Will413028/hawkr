// services/ai-service/internal/procurement/cropidentity/crop_gate_test.go
package cropidentity

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/stretchr/testify/require"
)

func item(v string) *string { return &v }

// TestComputeCropGate_Unavailable_MarksPendingWithNoCandidates 釘住 Cut 1
// （品項別名檢索死路解鎖）：查無候選以前是硬擋（Item 退回舊值、回一個被拒欄位、
// 不 Pending），現在改成待處理——Item 是買家這輪打的原字串（不是退回舊值）、
// 候選是空的非 nil slice（不是 nil：BuildCropDisambiguationQuestion 的迴圈邏輯
// 依賴它可迭代，也讓 golden fixture／JSON 序列化穩定輸出 []而非 null）、
// PendingReason 落庫成 UNAVAILABLE，且不落 OptionsJSON（沒有候選內容可存）。
func TestComputeCropGate_Unavailable_MarksPendingWithNoCandidates(t *testing.T) {
	resolver := &testkit.FakeResolver[DisambiguationResult]{ResolveResult: DisambiguationResult{Unavailable: true}}

	g := ComputeGate(context.Background(), resolver,
		domain.Slots{Item: item("舊品項")}, domain.Slots{Item: item("查無此物")})

	require.Equal(t, "查無此物", *g.Item, "待處理狀態顯示買家剛打的字，不是悄悄復原成舊值")
	require.True(t, g.Pending)
	require.NotNil(t, g.Candidates)
	require.Empty(t, g.Candidates)
	require.True(t, g.Evaluated(), "查無候選也是一種判定，買家看得到結果")

	// PendingReason 隨這一輪的判定往下游走，最後跟中斷本體同一次 INSERT——
	// 「是不是待處理、為什麼」完全由它承載，不靠候選清單長度推斷。
	require.Equal(t, domain.CropPendingReasonUnavailable, g.PendingReason)
	require.False(t, g.FactsAsOf.IsZero(), "真的查過 Pricebook，要記下查詢時刻")
}

func TestComputeCropGate_Ambiguous_KeepsItemAndMarksPending(t *testing.T) {
	resolver := &testkit.FakeResolver[DisambiguationResult]{ResolveResult: DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates:          []domain.CropDisambiguationOption{{Label: "有機小白菜"}},
	}}

	g := ComputeGate(context.Background(), resolver,
		domain.Slots{}, domain.Slots{Item: item("小白菜")})

	require.Equal(t, "小白菜", *g.Item)
	require.True(t, g.Pending)
	require.Len(t, g.Candidates, 1)
	require.Equal(t, domain.CropPendingReasonAmbiguous, g.PendingReason)
	require.False(t, g.FactsAsOf.IsZero())
}

func TestComputeCropGate_Matched_KeepsItemNoPending(t *testing.T) {
	cropCode := "CROP-X"
	resolver := &testkit.FakeResolver[DisambiguationResult]{ResolveResult: DisambiguationResult{CropCode: &cropCode}}

	g := ComputeGate(context.Background(), resolver,
		domain.Slots{}, domain.Slots{Item: item("芭樂")})

	require.Equal(t, "芭樂", *g.Item)
	require.False(t, g.Pending)
}

func TestComputeCropGate_ResolverError_DegradesWithoutBlocking(t *testing.T) {
	resolver := &testkit.FakeResolver[DisambiguationResult]{ResolveErr: errors.New("pricebook unavailable")}

	g := ComputeGate(context.Background(), resolver,
		domain.Slots{}, domain.Slots{Item: item("芭樂")})

	require.Equal(t, "芭樂", *g.Item)
	require.False(t, g.Pending)
}

// TestComputeCropGate_NilResolver_DoesNotResolve 釘住 nil-safe 慣例：
// turn／answer／rewind 的絕大多數既有單元測試都傳 nil cropResolver（沒有
// pricebook 替身，也不驗證品項解析），ComputeGate 不得因此 panic。
func TestComputeCropGate_NilResolver_DoesNotResolve(t *testing.T) {
	g := ComputeGate(context.Background(), nil,
		domain.Slots{}, domain.Slots{Item: item("芭樂")})

	require.Equal(t, "芭樂", *g.Item)
	require.False(t, g.Pending)
	require.False(t, g.Evaluated(), "沒有 resolver 就沒有判定")
}

// TestGateResult_Evaluated 釘住「這一輪 gate 有沒有做出新判定」的判準。
// 這個述詞同時決定兩件事，兩者必須永遠一致：steps[] 要不要有 CROP_GATE 這一格，
// 以及要不要為它打一次 LLM 敘述。
func TestGateResult_Evaluated(t *testing.T) {
	ctx := context.Background()

	t.Run("品項變動並解析成功＝有判定", func(t *testing.T) {
		code := "moa-group:abc"
		resolver := &testkit.FakeResolver[DisambiguationResult]{ResolveResult: DisambiguationResult{GroupCode: &code}}
		g := ComputeGate(ctx, resolver,
			domain.Slots{}, domain.Slots{Item: item("高麗菜")})
		require.True(t, g.Evaluated())
	})

	t.Run("品項變動但查無＝有判定（硬擋也是一種判定）", func(t *testing.T) {
		resolver := &testkit.FakeResolver[DisambiguationResult]{ResolveResult: DisambiguationResult{Unavailable: true}}
		g := ComputeGate(ctx, resolver,
			domain.Slots{Item: item("舊")}, domain.Slots{Item: item("紫蘇")})
		require.True(t, g.Evaluated(), "Unavailable 會產生 Rejected，買家看得到結果，就該佔一格")
	})

	t.Run("Pricebook 查詢失敗＝有判定，且標記 ResolveFailed", func(t *testing.T) {
		resolver := &testkit.FakeResolver[DisambiguationResult]{ResolveErr: errors.New("pricebook unavailable")}
		g := ComputeGate(ctx, resolver,
			domain.Slots{Item: item("高麗菜")}, domain.Slots{Item: item("番茄")})
		require.True(t, g.Evaluated())
		require.True(t, g.ResolveFailed,
			"降級路徑必須跟「成功解析但沒有 code」區分，否則敘述會對買家說「判斷已完成」")
	})

	t.Run("🔴 品項沒變動且已解析＝沒有判定", func(t *testing.T) {
		resolved := domain.Slots{Item: item("甘藍"),
			ItemResolution: domain.NewSystemItemResolution(
				item("LA1"), nil, nil, nil, "甘藍-初秋", domain.CategoryFromEntity("category-cabbage"))}
		g := ComputeGate(ctx, &testkit.FakeResolver[DisambiguationResult]{}, resolved, resolved)

		require.False(t, g.Pending, "沒有開著的中斷（三個進入點都有 guard），這一輪不待消歧")
		require.False(t, g.Evaluated(),
			"這一輪 gate 什麼都沒做。算成有判定的話，每一輪都會多出一格內容逐字相同的步驟——正是這次要修掉的重複")
	})

	// 上一條的前提是「已經有解析結果」。品項沒變動但**還沒有**解析結果時 gate 這
	// 一輪真的去查了 Pricebook，那就是一次新判定，該有它自己的那一格；重複步驟的
	// 疑慮不成立——解析成功之後就回到上一條的 no-op。
	t.Run("品項沒變動但尚未解析＝有判定", func(t *testing.T) {
		resolver := &testkit.FakeResolver[DisambiguationResult]{ResolveResult: DisambiguationResult{
			CropCode: item("LA1"), Label: "甘藍-初秋",
			CategoryOrigin: domain.CategoryFromEntity("category-cabbage"),
		}}
		g := ComputeGate(ctx, resolver,
			domain.Slots{Item: item("甘藍")}, domain.Slots{Item: item("甘藍")})

		require.True(t, g.Evaluated(), "重試成功是一次新判定")
		require.NotNil(t, g.Resolution)
	})
}

// countingResolver 記錄 Resolve 被呼叫幾次。「不得重查」這類斷言只看結果值是證
// 不出來的——結果一樣也可能是查了一次又得到同樣答案。
type countingResolver struct {
	inner Resolver
	calls int
}

func (c *countingResolver) Resolve(ctx context.Context, itemText string) (DisambiguationResult, error) {
	c.calls++
	return c.inner.Resolve(ctx, itemText)
}

// TestComputeCropGate_RetriesAfterResolverFailure：Pricebook 這一輪掛了，下一輪買家
// 只補了城市、品項一個字都沒改——解析必須重試。
//
// 現行 itemChanged 只比品項文字，所以失敗之後同一個品項永遠不會再查一次；而失敗
// 時既有解析已經被清掉（見 ComputeGate 裡那行 log），於是這張需求就卡在「沒有品項
// 分類」直到買家自己去改品項。設計書與 ADR 都沒有規定過這個行為。
func TestComputeCropGate_RetriesAfterResolverFailure(t *testing.T) {
	inner := &testkit.FakeResolver[DisambiguationResult]{ResolveErr: errors.New("temporary unavailable")}
	resolver := &countingResolver{inner: inner}

	slots := domain.Slots{Item: item("芭樂"), Unit: item("KG")}
	first := ComputeGate(context.Background(), resolver, domain.Slots{}, slots)
	require.True(t, first.ResolveFailed)
	require.Nil(t, first.Resolution)
	require.Equal(t, 1, resolver.calls)

	// 第二輪：resolver 恢復，買家只補了配送城市。
	inner.ResolveErr = nil
	inner.ResolveResult = DisambiguationResult{
		CropCode: item("CROP-GUAVA"), Label: "芭樂",
		CategoryOrigin: domain.CategoryFromEntity("category-guava"),
	}
	after := slots
	after.DeliveryCity = item("桃園市")

	second := ComputeGate(context.Background(), resolver, slots, after)
	require.Equal(t, 2, resolver.calls, "依賴恢復後必須重試，不能等買家去改品項")
	require.NotNil(t, second.Resolution, "重試成功必須產生解析結果")
	require.False(t, second.ResolveFailed)
}

// TestComputeCropGate_ResolvedItemIsNotReResolved 是上一條的負控制：已經有解析結果
// 的品項不得每一輪重查一次 Pricebook。
func TestComputeCropGate_ResolvedItemIsNotReResolved(t *testing.T) {
	inner := &testkit.FakeResolver[DisambiguationResult]{ResolveResult: DisambiguationResult{
		CropCode: item("CROP-GUAVA"), Label: "芭樂",
	}}
	resolver := &countingResolver{inner: inner}

	resolved := domain.Slots{
		Item: item("芭樂"),
		ItemResolution: domain.NewSystemItemResolution(
			item("CROP-GUAVA"), nil, nil, nil, "芭樂", domain.CategoryFromEntity("category-guava")),
	}
	after := resolved
	after.DeliveryCity = item("桃園市")

	g := ComputeGate(context.Background(), resolver, resolved, after)
	require.Zero(t, resolver.calls, "已解析的品項不得重查")
	require.Nil(t, g.Resolution)
	require.False(t, g.Evaluated())
}

// TestComputeCropGate_AwaitingBuyerAnswer_DoesNotReResolve：買家正在挑候選的時候又
// 補了一句別的，不得重查——重查會換掉他眼前那份候選清單，等於把他正在回答的問題
// 換掉。待選內容由既有的 interrupt 持有，這一輪不碰。
func TestComputeCropGate_AwaitingBuyerAnswer_DoesNotReResolve(t *testing.T) {
	inner := &testkit.FakeResolver[DisambiguationResult]{ResolveResult: DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates:          []domain.CropDisambiguationOption{{Label: "台灣芭樂"}},
	}}
	resolver := &countingResolver{inner: inner}

	pending := domain.Slots{Item: item("芭樂")}
	after := pending
	after.DeliveryCity = item("桃園市")

	g := ComputeGate(context.Background(), resolver, pending, after, AwaitingBuyerAnswer())
	require.Zero(t, resolver.calls, "買家還在挑候選時不得重查")
	require.False(t, g.Pending)
	require.Empty(t, g.Candidates)
}
