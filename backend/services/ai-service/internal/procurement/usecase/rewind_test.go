package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/stretchr/testify/require"
)

func TestRewindUseCase_RevertsToStateBeforeLastWrite(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "水果禮盒"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	city := "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-1",
	})

	require.NoError(t, err)
	require.Nil(t, got.Slots.DeliveryCity, "delivery_city 應該被回退掉")
	require.NotNil(t, got.Question)
	require.Equal(t, domain.SlotDeliveryCity, got.Question.SlotKey, "重新問回被 rewind 的欄位")
}

func TestRewindUseCase_ModelBlocksSetRenderModeModel(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "水果禮盒"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	city := "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	uc.realizer = &fakeSurfaceRealizer{blocks: []RealizedBlock{{
		Kind: domain.BlockNextStep, Text: "重新確認一下數量好嗎？",
		ClaimCodes: []string{string(domain.ClaimNextQuestionRequired)},
	}}}

	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-model-1",
	})

	require.NoError(t, err)
	require.Equal(t, "MODEL", got.RenderMode)
	require.Equal(t, "重新確認一下數量好嗎？", got.Reply, "TextFallback 必須是模型 blocks 的 join")
}

type slowNarrator struct {
	delay time.Duration
}

func (s *slowNarrator) Narrate(_ context.Context, _ domain.StepCode, _ string) (string, error) {
	time.Sleep(s.delay)
	return "narrated", nil
}

type slowRealizer struct {
	delay time.Duration
}

func (s *slowRealizer) Realize(_ context.Context, _ SurfaceContext) ([]RealizedBlock, error) {
	time.Sleep(s.delay)
	return []RealizedBlock{{
		Kind: domain.BlockNextStep, Text: "稍等一下我確認一下",
		ClaimCodes: []string{string(domain.ClaimNextQuestionRequired)},
	}}, nil
}

func TestRewindUseCase_NarrationRunsInParallel(t *testing.T) {
	const delay = 200 * time.Millisecond
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "水果禮盒"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	city := "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	uc.narrator = &slowNarrator{delay: delay}
	uc.realizer = &slowRealizer{delay: delay}

	start := time.Now()
	if _, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-parallel-1",
	}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*delay {
		t.Fatalf("tail 花了 %v，超過單次 %v 的兩倍——沒有真的平行", elapsed, delay)
	}
}

func TestRewindUseCase_MostRecentWriteWins_NotFirst(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "水果禮盒"
	taipei := "臺北市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &taipei},
		Source: "ANSWER", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	taoyuan := "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &taoyuan},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-2",
	})

	require.NoError(t, err)
	require.NotNil(t, got.Slots.DeliveryCity)
	require.Equal(t, "臺北市", *got.Slots.DeliveryCity,
		"target 必須是最後一次（第二次）寫入之前的版本，不是第一次寫入之前（那樣會連第一次的值也弄丟）")
}

func TestRewindUseCase_OptionalSlot_ReAsksTheRewoundSlot_NotPriorityWinner(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city := "水果禮盒", "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	unit := "KG"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Unit: &unit},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotUnit, ClientRequestID: "cr-rewind-optional-1",
	})

	require.NoError(t, err)
	require.Nil(t, got.Slots.Unit, "unit 應該被回退掉")
	require.NotNil(t, got.Question)
	require.Equal(t, domain.SlotQuantity, got.Question.SlotKey,
		"rewind unit 已經不是獨立問題型態，折進 quantity 複合問題卡；不是優先序更高、恰好也未填的其他選填欄位（spec）")
	require.True(t, got.Question.AllowFreeText, "target 狀態 quantity/unit 皆未知，應該是複合問，含數量輸入")
	require.Len(t, got.Question.UnitOptions, 3, "target 狀態 unit 未知，應該附上單位選項")
}

func TestRewindUseCase_OptionalSlotRewind_ActCodeIsAckAndAskOptional(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city, unit := "水果禮盒", "桃園市", "KG"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Unit: &unit},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	pkg := "紙箱"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Unit: &unit, Packaging: &pkg},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotPackaging, ClientRequestID: "cr-rewind-ready-followup-1",
	})

	require.NoError(t, err)
	require.Empty(t, got.MissingRequired, "item／delivery_city／unit 都不受 rewind packaging 影響，仍應 REQUIREMENT_READY")
	require.NotNil(t, got.Question, "rewind 選填欄位一定會重問被 rewind 的那格")
	require.Equal(t, domain.ActAckAndAskOptional, got.DialoguePlan.ActCode,
		"ready 但選填問題還在，PlanDialogue 必須選 ACK_AND_ASK_OPTIONAL，不能誤選成 "+
			"REQUEST_BASICS_READY 宣稱已開始找候選")
}

func TestRewindUseCase_ReAsksEvenWhenConsecutiveOptionalCapReached(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city := "水果禮盒", "桃園市"

	mustPatch := func(q *domain.ClarifyQuestion) []byte {
		b, err := json.Marshal(turnPayload{Question: q})
		require.NoError(t, err)
		return b
	}

	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: mustPatch(&domain.ClarifyQuestion{SlotKey: domain.SlotDeliveryCity}),
		RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	pkg := "紙箱"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Packaging: &pkg},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: mustPatch(&domain.ClarifyQuestion{SlotKey: domain.SlotPackaging}),
		RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	qty := 100
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Packaging: &pkg, Quantity: &qty},
		Source: "ANSWER", ClientRequestID: "seed-3", PayloadJSON: mustPatch(&domain.ClarifyQuestion{SlotKey: domain.SlotQuantity}),
		RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	unit := "KG"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Packaging: &pkg, Quantity: &qty, Unit: &unit},
		Source: "ANSWER", ClientRequestID: "seed-4", PayloadJSON: mustPatch(&domain.ClarifyQuestion{SlotKey: domain.SlotUnit}),
		RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotPackaging, ClientRequestID: "cr-rewind-cap-1",
	})

	require.NoError(t, err)
	require.NotNil(t, got.Question, "rewind 是買家明確要求重問，不該被連續選填 cap 吃成 nil")
	require.Equal(t, domain.SlotPackaging, got.Question.SlotKey)
}

func TestRewindUseCase_Message_UsesDisplayLabel_NotRawSlotKey(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "水果禮盒"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	city := "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-msg-1",
	})
	require.NoError(t, err)

	stored, ok := repo.patches[fakePatchKey{1234, "cr-rewind-msg-1"}]
	require.True(t, ok)
	var payload turnPayload
	require.NoError(t, json.Unmarshal(stored.PayloadJSON, &payload))

	require.NotContains(t, payload.Message, string(domain.SlotDeliveryCity), "訊息不該出現內部 slotKey 字面值")
	require.Contains(t, payload.Message, domain.SlotLabels[domain.SlotDeliveryCity], "訊息要包含該欄位的中文顯示字串")
}

func TestRewindUseCase_ConflictWhenClientRequestIDReusedOnDifferentRequest(t *testing.T) {
	repo := newFakeRepo()
	reqA, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city := "水果禮盒", "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: reqA.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-a1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: reqA.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "ANSWER", ClientRequestID: "seed-a2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	reqB, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: reqA.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-reused",
	})
	require.NoError(t, err)

	_, err = uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: reqB.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-reused",
	})

	require.ErrorIs(t, err, domain.ErrIdempotencyKeyConflict,
		"同一個 clientRequestId 被拿去對另一個 request 發起 rewind 必須報衝突，不能靜默重播前一個 request 的舊結果")
}

func TestRewindUseCase_NeverWrittenSlot_ReturnsErrSlotNeverWritten(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-3",
	})

	require.ErrorIs(t, err, ErrSlotNeverWritten)
}

func TestRewindUseCase_OtherBuyersRequest_ReturnsErrNotFound(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(5678))
	require.NoError(t, err)
	item := "水果禮盒"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 5678, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-b1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)

	_, err = uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotItem, ClientRequestID: "cr-rewind-cross-buyer-1",
	})

	require.ErrorIs(t, err, repository.ErrNotFound,
		"跨買家 rewind 必須看起來像不存在，不得洩漏「這個 request 存在」")
}

func TestRewindUseCase_ItemChanged_TriggersCropResolution(t *testing.T) {
	repo := newFakeRepo()
	tree := testkit.VegetableTree()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	cabbage := "高麗菜"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &cabbage},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	carrot := "地瓜"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &carrot},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, cropidentity.NewDisambiguationUseCase(tree, nil), nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotItem, ClientRequestID: "cr-rewind-crop-1",
	})

	require.NoError(t, err)
	require.NotNil(t, got.Slots.Item)
	require.Equal(t, "高麗菜", *got.Slots.Item, "item 應該被回退成 rewind 前一次寫入的值")
	require.NotZero(t, tree.Calls, "rewind 把 item 改回不同值，也要重新解析")
	require.NotNil(t, got.Slots.ItemResolution)
	require.Equal(t, "17", got.Slots.ItemResolution.CategoryID)
	if got.Question != nil {
		require.NotEqual(t, domain.SlotCropDisambiguation, got.Question.SlotKey, "分類確認後不出消歧卡片")
	}
}

func TestRewindUseCase_ItemUnchanged_DoesNotReResolve(t *testing.T) {
	repo := newFakeRepo()
	tree := testkit.VegetableTree()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "高麗菜"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item,
			ItemResolution: testkit.ConfirmedResolution("高麗菜")},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	city := "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city,
			ItemResolution: testkit.ConfirmedResolution("高麗菜")},
		Source: "ANSWER", ClientRequestID: "seed-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, cropidentity.NewDisambiguationUseCase(tree, nil), nil, nil, nil, nil)
	_, err = uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-crop-2",
	})

	require.NoError(t, err)
	// ⛔ 前提是**已經解析過**：沒有 resolution 代表上一次失敗過，那要重試。
	require.Zero(t, tree.Calls, "item 沒變且已解析，就不該重新解析")
}

func seedRewindableItemChain(t *testing.T, repo *fakeRepo, seedPrefix string) int64 {
	t.Helper()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	cabbage := "高麗菜"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &cabbage},
		Source: "LLM_EXTRACT", ClientRequestID: seedPrefix + "-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	carrot := "紅蘿蔔"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &carrot},
		Source: "ANSWER", ClientRequestID: seedPrefix + "-2", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	return req.ID
}

func TestHandle_RewindToAmbiguousItem_OverridesQuestionWithDisambiguation(t *testing.T) {
	repo := newFakeRepo()
	reqID := seedRewindableItemChain(t, repo, "seed-rewind-ambiguous")
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{Label: "高麗菜", CategoryID: "17", CategoryLevel: 3},
		},
	}}

	uc := NewRewindUseCase(repo, resolver, nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: reqID, SlotKey: domain.SlotItem, ClientRequestID: "cr-rewind-ambiguous-1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.ActItemNeedsDisambiguation, got.DialoguePlan.ActCode)
	require.NotNil(t, got.Question)
	require.Equal(t, domain.SlotCropDisambiguation, got.Question.SlotKey,
		"消歧問句要蓋掉被 rewind 的那一格的一般問題卡")
	require.Len(t, got.Question.Options, 2, "1 個真候選 + 固定附加的 D")
}

func TestHandle_RewindToUnavailableItem_AsksDisambiguationAndPersistsPendingReason(t *testing.T) {
	repo := newFakeRepo()
	reqID := seedRewindableItemChain(t, repo, "seed-rewind-unavailable")
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{Unavailable: true}}

	uc := NewRewindUseCase(repo, resolver, nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: reqID, SlotKey: domain.SlotItem, ClientRequestID: "cr-rewind-unavailable-1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.ActItemNeedsDisambiguation, got.DialoguePlan.ActCode)
	require.Empty(t, got.Rejected, "查無候選是待處理，不是拒絕")
	require.NotNil(t, got.Question)
	require.Equal(t, domain.SlotCropDisambiguation, got.Question.SlotKey)
	require.Len(t, got.Question.Options, 1, "0 候選時只剩固定的「其他（自己輸入）」")
	require.NotNil(t, got.Slots.Item)
	require.Equal(t, "高麗菜", *got.Slots.Item,
		"rewind 的持久化是 RewindTo(targetRevisionID)，回應裡的 slots 必須跟它一致——"+
			"crop_gate 不覆寫這個值，待處理狀態只反映在 Question／DialoguePlan")
}

func seedRewindableReadyRequest(t *testing.T, repo *fakeRepo) int64 {
	t.Helper()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city, unit := "水果禮盒", "桃園市", "KG"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Unit: &unit},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-ready-1", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	budget := 500
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Unit: &unit, BudgetPerUnit: &budget},
		Source: "ANSWER", ClientRequestID: "seed-ready-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	return req.ID
}

func TestRewindUseCase_ReadyWithoutPricePreview_SetsNextActionReviewPricePreview(t *testing.T) {
	repo := newFakeRepo()
	requestID := seedRewindableReadyRequest(t, repo)

	uc := NewRewindUseCase(repo, nil, fakePricePreviewChecker{hasPreview: false}, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotBudgetPerUnit, ClientRequestID: "cr-rewind-next-action-1",
	})

	require.NoError(t, err)
	require.Empty(t, got.MissingRequired)
	require.Equal(t, "REVIEW_PRICE_PREVIEW", got.NextAction)
}

func TestRewindUseCase_ReadyWithExistingPricePreview_NextActionIsEmpty(t *testing.T) {
	repo := newFakeRepo()
	requestID := seedRewindableReadyRequest(t, repo)

	uc := NewRewindUseCase(repo, nil, fakePricePreviewChecker{hasPreview: true}, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotBudgetPerUnit, ClientRequestID: "cr-rewind-next-action-2",
	})

	require.NoError(t, err)
	require.Empty(t, got.NextAction)
}

func TestRewindUseCase_ReadyWithNilPricePreviewChecker_NextActionIsEmpty(t *testing.T) {
	repo := newFakeRepo()
	requestID := seedRewindableReadyRequest(t, repo)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotBudgetPerUnit, ClientRequestID: "cr-rewind-next-action-3",
	})

	require.NoError(t, err)
	require.Empty(t, got.NextAction)
}

func TestRewindUseCase_RecomputesFreshPlan_DoesNotReuseSupersededPlan(t *testing.T) {
	repo := newFakeRepo()
	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)

	ctx := context.Background()
	store := newFakeRunStore()
	graphTurnDeps := func(progress graphTurnProgress) GraphControllerDeps {
		return GraphControllerDeps{
			RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
			Runner: newTerminalCompletionRunner(t, progress, nil),
		}
	}

	turn1Slots := domain.Slots{Item: strPtr("蘋果")}
	out1, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{BuyerID: 1, Message: "蘋果", ClientRequestID: "cr-1"},
		graphTurnDeps(graphTurnProgress{Patch: domain.SlotPatch(turn1Slots), NextSlots: turn1Slots}), TurnStreamHooks{})
	require.NoError(t, err)
	first := out1.Result
	require.NotEmpty(t, first.MissingRequired, "sanity check：turn1 只填 item，delivery_city／unit 仍缺，NOT READY")

	turn2Slots := domain.Slots{Item: strPtr("蘋果"), DeliveryCity: strPtr("桃園市"), Unit: strPtr("KG")}
	out2, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{BuyerID: 1, RequestID: first.RequestID, Message: "宅配到桃園市，單位公斤", ClientRequestID: "cr-2"},
		graphTurnDeps(graphTurnProgress{
			Patch:     domain.SlotPatch{DeliveryCity: strPtr("桃園市"), Unit: strPtr("KG")},
			NextSlots: turn2Slots,
		}), TurnStreamHooks{})
	require.NoError(t, err)
	second := out2.Result
	require.Empty(t, second.MissingRequired, "sanity check：turn2 一次把 delivery_city／unit 填齊，達成 REQUIREMENT_READY——這是被 rewind supersede 掉的狀態")

	got, err := uc.composeRewindViaGraph(ctx, RewindInput{
		BuyerID: 1, RequestID: first.RequestID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind",
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.MissingRequired, "target 狀態（turn1 之後）delivery_city／unit 仍缺，NOT READY")
	require.Equal(t, domain.ActAckAndAskRequired, got.DialoguePlan.ActCode,
		"rewind 後的 plan 必須針對還原後的 target 狀態（NOT READY）重新計算，不能延用被 supersede 的 turn2"+
			"（READY）狀態或其舊 plan——若誤用 superseded 狀態，Ready=true 會讓 ActCode 落到"+
			"ActAckAndAskOptional 而不是 ActAckAndAskRequired")
}

func seedRewindableCityChain(t *testing.T, repo *fakeRepo, seedPrefix string) int64 {
	t.Helper()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "水果禮盒"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: seedPrefix + "-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	city := "桃園市"
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "ANSWER", ClientRequestID: seedPrefix + "-2", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	return req.ID
}

type countingRewindStore struct {
	RewindStore
	rewindCalls int
}

func (c *countingRewindStore) RewindTo(ctx context.Context, in repository.RewindInput) (repository.Revision, error) {
	c.rewindCalls++
	return c.RewindStore.RewindTo(ctx, in)
}

func TestClaimAndDispatchRewind_DuplicateClientRequestID_DoesNotRerunPipeline(t *testing.T) {
	repo := newFakeRepo()
	requestID := seedRewindableCityChain(t, repo, "seed-claim-rewind-1")
	store := &countingRewindStore{RewindStore: repo}
	uc := NewRewindUseCase(store, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	in := RewindInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotDeliveryCity,
		ClientRequestID: "cr-claim-rewind-dup-1",
	}

	first, err := uc.ClaimAndDispatchRewind(context.Background(), in, runStore, NewRunDispatcher())
	require.NoError(t, err)
	require.Equal(t, 1, store.rewindCalls, "背景 pipeline 必須真的跑過一次")
	require.NotZero(t, first.RunID, "回傳值必須帶 ledger 產生的 run_id")
	require.NotZero(t, first.Result.RevisionID, "呼叫端拿到的是終態結果，不是空殼")
	require.Nil(t, first.Result.Slots.DeliveryCity, "前置條件：delivery_city 真的被回退掉")

	second, err := uc.ClaimAndDispatchRewind(context.Background(), in, runStore, NewRunDispatcher())
	require.NoError(t, err)

	require.Equal(t, 1, store.rewindCalls, "重送不得重跑 pipeline／不得再寫一次 revision")
	require.Equal(t, first.RunID, second.RunID, "重送要對到同一個 run，不得另建一個")
	require.Equal(t, first.Result.RequestID, second.Result.RequestID)
	require.Equal(t, first.Result.RevisionID, second.Result.RevisionID)
	require.Equal(t, first.Result.Reply, second.Result.Reply)
	require.Equal(t, first.Result.Slots, second.Result.Slots)

	require.Len(t, runStore.terminals, 1, "重播不得重新終態化這個 run")
}

type panickingRewindStore struct {
	RewindStore
}

func (panickingRewindStore) ListBranchRevisions(context.Context, int64, identity.BuyerID) ([]repository.RevisionWithPatch, error) {
	panic("boom inside the rewind pipeline")
}

func TestClaimAndDispatchRewind_BackgroundPanic_DoesNotHangCaller(t *testing.T) {
	repo := newFakeRepo()
	requestID := seedRewindableCityChain(t, repo, "seed-panic-rewind-1")
	uc := NewRewindUseCase(panickingRewindStore{RewindStore: repo}, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()

	type outcome struct {
		res SubmitTurnResult
		err error
	}
	ch := make(chan outcome, 1)
	go func() {
		res, err := uc.ClaimAndDispatchRewind(context.Background(), RewindInput{
			BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotDeliveryCity,
			ClientRequestID: "cr-rewind-panic-1",
		}, runStore, NewRunDispatcher())
		ch <- outcome{res, err}
	}()

	select {
	case got := <-ch:
		require.Error(t, got.err, "panic 之後呼叫端必須拿到錯誤，不是零值結果")
	case <-time.After(5 * time.Second):
		t.Fatal("背景 panic 之後 ClaimAndDispatchRewind 沒有返回——呼叫端被卡住了")
	}

	require.Len(t, runStore.terminals, 1)
	require.Equal(t, domain.RunStatusFailed(), runStore.terminals[0].Status)
	require.Equal(t, codemeta.ProcurementRunFailureReasonTurnPipelineError.Value, runStore.terminals[0].FailureReason)
}

func TestClaimAndDispatchRewind_OpenInterruptExists_NewCommand_BlockedByGuard(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(1234), Slots: domain.Slots{Item: ptr("寶島梨")},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-rewind-guard-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	runStore.hasOpenInterrupt = true
	runStore.openInterrupt = repository.Interrupt{ID: "int-open-rewind-1", RunID: 9500, Status: "PENDING"}

	_, err = uc.ClaimAndDispatchRewind(context.Background(),
		RewindInput{BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotItem, ClientRequestID: "cr-guard-rewind-new-1"},
		runStore, NewRunDispatcher())

	require.ErrorIs(t, err, ErrInterruptPending)
	require.Empty(t, runStore.runs, "guard 擋下時不得呼叫 ClaimRun，零 mutation")
}

func TestClaimAndDispatchRewind_OpenInterruptExists_IdempotentRetry_NotBlocked(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(1234), Slots: domain.Slots{Item: ptr("寶島梨")},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-rewind-guard-2", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	_, err = repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(1234), Slots: domain.Slots{Item: ptr("寶島梨"), DeliveryCity: ptr("台北市")},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-rewind-guard-3", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	in := RewindInput{BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotItem, ClientRequestID: "cr-guard-rewind-retry-1"}

	first, err := uc.ClaimAndDispatchRewind(context.Background(), in, runStore, NewRunDispatcher())
	require.NoError(t, err)

	runStore.hasOpenInterrupt = true
	runStore.openInterrupt = repository.Interrupt{ID: "int-open-rewind-2", RunID: 9501, Status: "PENDING"}

	second, err := uc.ClaimAndDispatchRewind(context.Background(), in, runStore, NewRunDispatcher())
	require.NoError(t, err, "冪等重送必須放行到 ClaimRun 自己的既有回放路徑，不被 guard 擋下")
	require.Equal(t, first.RunID, second.RunID)
}

func TestExecuteRewindPipeline_CropDisambiguationPending_TransitionsToWaitingForInput(t *testing.T) {
	repo := newFakeRepo()
	reqID := seedRewindableItemChain(t, repo, "seed-rewind-crop-interrupt")
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{Label: "高麗菜", CategoryID: "17", CategoryLevel: 3},
		},
	}}
	uc := NewRewindUseCase(repo, resolver, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9201, BuyerID: 1234, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}
	in := RewindInput{
		BuyerID: 1234, RequestID: reqID, SlotKey: domain.SlotItem, ClientRequestID: "cr-rewind-crop-interrupt-1",
	}

	outcome, err := uc.executeRewindPipeline(context.Background(), run, in, runStore)
	require.NoError(t, err)

	require.NotNil(t, outcome.result.Question)
	require.Equal(t, domain.SlotCropDisambiguation, outcome.result.Question.SlotKey)

	require.Len(t, runStore.terminals, 1)
	terminal := runStore.terminals[0]
	require.Equal(t, domain.RunStatusWaiting(), terminal.Status)
	require.NotEqual(t, domain.RunStatusSucceeded(), terminal.Status)

	require.Equal(t, codemeta.ProcurementLedgerEventTypeConfirmationRequired.Value, terminal.TerminalEventType)

	require.Len(t, runStore.interrupts, 1)
	require.Equal(t, "CLARIFICATION", runStore.interrupts[0].InterruptKind)
	require.Equal(t, run.ID, runStore.interrupts[0].RunID)

	require.Contains(t, string(terminal.PublicPayloadJSON), `"interruptId"`)
	require.Contains(t, string(terminal.PublicPayloadJSON), `"interruptKind":"CLARIFICATION"`)

	require.NotEmpty(t, outcome.interruptID)
	require.Equal(t, "CLARIFICATION", outcome.interruptKind)
}

func TestExecuteRewindPipeline_NormalCompletion_DoesNotCreateInterrupt(t *testing.T) {
	repo := newFakeRepo()
	requestID := seedRewindableCityChain(t, repo, "seed-normal-rewind-1")
	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9202, BuyerID: 1234, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}
	in := RewindInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-normal-1",
	}

	outcome, err := uc.executeRewindPipeline(context.Background(), run, in, runStore)
	require.NoError(t, err)

	require.Empty(t, runStore.interrupts, "一般完成不得建立 interrupt")
	require.Len(t, runStore.terminals, 1)
	terminal := runStore.terminals[0]
	require.Equal(t, domain.RunStatusSucceeded(), terminal.Status)
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunFinished.Value, terminal.TerminalEventType)
	require.Empty(t, terminal.FailureReason)
	require.Equal(t, outcome.result.RevisionID, terminal.ResultRevisionID,
		"終態要記下這次執行產生的 revision，重播才有錨點")
	require.Empty(t, outcome.interruptID)
	require.Empty(t, outcome.interruptKind)

	require.NotEmpty(t, outcome.result.Steps, "前置條件：這一輪必須真的產生 step 清單")
	types := runStore.eventTypes()
	require.Len(t, types, len(outcome.result.Steps),
		"每個 domain.Step 都對到一筆 ledger event")
	for _, ty := range types {
		require.Equal(t, codemeta.ProcurementLedgerEventTypeStepFinished.Value, ty,
			"ComputeRewindSteps 產生的 step 永遠是 SUCCEEDED，因此永遠換算成 STEP_FINISHED")
	}
}

func TestExecuteRewindPipeline_ConversationIDZeroAtClaim_BackfillsRunConversationID(t *testing.T) {
	repo := newFakeRepo()
	requestID := seedRewindableCityChain(t, repo, "seed-backfill-rewind-1")
	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9210, BuyerID: 1234, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}
	in := RewindInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-backfill-1",
	}

	outcome, err := uc.executeRewindPipeline(context.Background(), run, in, runStore)
	require.NoError(t, err)
	require.NotZero(t, outcome.result.ConversationID, "前置條件：pipeline 必須真的解出 conversationId")

	require.Len(t, runStore.setConversationIDCalls, 1, "run.ConversationID 在 claim 當下是 0，pipeline 跑完後必須回填")
	call := runStore.setConversationIDCalls[0]
	require.Equal(t, run.ID, call.runID)
	require.Equal(t, outcome.result.ConversationID, call.conversationID)
}

func TestExecuteRewindPipeline_RunAlreadyHasConversationID_DoesNotBackfill(t *testing.T) {
	repo := newFakeRepo()
	requestID := seedRewindableCityChain(t, repo, "seed-noop-rewind-1")
	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9211, BuyerID: 1234, ConversationID: 5555, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}
	in := RewindInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-noop-1",
	}

	outcome, err := uc.executeRewindPipeline(context.Background(), run, in, runStore)
	require.NoError(t, err)
	require.NotZero(t, outcome.result.ConversationID)

	require.Empty(t, runStore.setConversationIDCalls, "run 在 claim 當下已經有 conversation_id，不該再回填")
}

func TestExecuteRewindPipeline_PipelineError_TerminalizesAsFailed(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	uc := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9203, BuyerID: 1234, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}

	_, err = uc.executeRewindPipeline(context.Background(), run, RewindInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-rewind-fail-1",
	}, runStore)

	require.ErrorIs(t, err, ErrSlotNeverWritten)
	require.Len(t, runStore.terminals, 1)
	terminal := runStore.terminals[0]
	require.Equal(t, domain.RunStatusFailed(), terminal.Status)
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunFailed.Value, terminal.TerminalEventType)
	require.Equal(t, codemeta.ProcurementRunFailureReasonTurnPipelineError.Value, terminal.FailureReason)
	require.Empty(t, runStore.interrupts, "失敗的 run 不得建立 interrupt")
	require.Empty(t, runStore.setConversationIDCalls,
		"pipeline 失敗、result 是零值，沒有真正的 conversationId 可以回填")
}

func TestCanonicalRewindHash_IncludesRequestID(t *testing.T) {
	base := RewindInput{BuyerID: 1, RequestID: 100, SlotKey: domain.SlotDeliveryCity}
	otherRequest := base
	otherRequest.RequestID = 200

	require.NotEqual(t, canonicalRewindHash(base), canonicalRewindHash(otherRequest))

	otherSlot := base
	otherSlot.SlotKey = domain.SlotItem

	require.NotEqual(t, canonicalRewindHash(base), canonicalRewindHash(otherSlot))

	same := base
	require.Equal(t, canonicalRewindHash(base), canonicalRewindHash(same))
}

func TestComposeRewindViaGraph_RestoresHistoricalSlots(t *testing.T) {
	history := []repository.RevisionWithPatch{
		{Revision: repository.Revision{ID: 10, SlotsJSON: mustMarshalSlots(t, domain.Slots{})}},
		{Revision: repository.Revision{ID: 11, SlotsJSON: mustMarshalSlots(t, domain.Slots{Item: ptr("蘋果")})}},
		{Revision: repository.Revision{ID: 12, SlotsJSON: mustMarshalSlots(t, domain.Slots{Item: ptr("香蕉")})}},
	}
	store := &fakeRewindStore{
		history:      history,
		snapshot:     repository.Request{ID: 1},
		rewindResult: repository.Revision{ID: 13, ConversationID: 100},
	}
	uc := NewRewindUseCase(store, &testkit.FakeResolver[cropidentity.DisambiguationResult]{}, nil, nil, nil, nil)
	in := RewindInput{BuyerID: identity.BuyerID(1234), RequestID: 1, SlotKey: domain.SlotItem, ClientRequestID: "req-1"}

	result, err := uc.composeRewindViaGraph(context.Background(), in)

	require.NoError(t, err)
	require.Equal(t, "TEMPLATE", result.RenderMode)
	require.NotNil(t, result.Slots.Item)
	require.Equal(t, "蘋果", *result.Slots.Item, "rewound to the revision before item was overwritten")
	require.Equal(t, int64(13), result.RevisionID)
}

func TestComposeRewindViaGraph_ReasksTheRewoundSlot_NotPriorityOrder(t *testing.T) {
	history := []repository.RevisionWithPatch{
		{Revision: repository.Revision{ID: 10, SlotsJSON: mustMarshalSlots(t, domain.Slots{})}},
		{Revision: repository.Revision{ID: 11, SlotsJSON: mustMarshalSlots(t, domain.Slots{DeliveryCity: ptr("台北市")})}},
	}
	store := &fakeRewindStore{history: history, snapshot: repository.Request{ID: 1}, rewindResult: repository.Revision{ID: 12}}
	uc := NewRewindUseCase(store, &testkit.FakeResolver[cropidentity.DisambiguationResult]{}, nil, nil, nil, nil)
	in := RewindInput{BuyerID: identity.BuyerID(1234), RequestID: 1, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "req-2"}

	result, err := uc.composeRewindViaGraph(context.Background(), in)

	require.NoError(t, err)
	require.NotNil(t, result.Question)
	require.Equal(t, domain.SlotDeliveryCity, result.Question.SlotKey)
}

func TestComposeRewindViaGraph_NeverWrittenSlot_ReturnsError(t *testing.T) {
	store := &fakeRewindStore{history: []repository.RevisionWithPatch{}, snapshot: repository.Request{ID: 1}}
	uc := NewRewindUseCase(store, &testkit.FakeResolver[cropidentity.DisambiguationResult]{}, nil, nil, nil, nil)
	in := RewindInput{BuyerID: identity.BuyerID(1234), RequestID: 1, SlotKey: domain.SlotItem, ClientRequestID: "req-4"}

	_, err := uc.composeRewindViaGraph(context.Background(), in)

	require.ErrorIs(t, err, ErrSlotNeverWritten)
}

type fakeRewindStore struct {
	history      []repository.RevisionWithPatch
	snapshot     repository.Request
	rewindResult repository.Revision
}

func (f *fakeRewindStore) ListBranchRevisions(ctx context.Context, requestID int64, buyerID identity.BuyerID) ([]repository.RevisionWithPatch, error) {
	return f.history, nil
}

func (f *fakeRewindStore) RewindTo(ctx context.Context, in repository.RewindInput) (repository.Revision, error) {
	return f.rewindResult, nil
}

func (f *fakeRewindStore) FindByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.IdempotentResult, error) {
	return repository.IdempotentResult{}, repository.ErrNotFound
}

func (f *fakeRewindStore) GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error) {
	return f.snapshot, repository.Revision{}, nil
}
