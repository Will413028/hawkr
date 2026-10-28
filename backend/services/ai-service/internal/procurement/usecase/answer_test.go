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

func TestAnswerUseCase_OptionID_AppliesWhitelistedValue(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "水果禮盒"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "seed", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taoyuan", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-1",
	})

	require.NoError(t, err)
	require.Equal(t, "桃園市", *got.Slots.DeliveryCity)
	require.Equal(t, []domain.SlotKey{domain.SlotUnit}, got.MissingRequired)
	require.NotNil(t, got.Question, "unit 還缺，PickNextQuestion 必填分支必須給出問題卡")
	require.Equal(t, domain.SlotQuantity, got.Question.SlotKey,
		"unit 缺項的問題卡折進 quantity 複合問題卡（見 domain.buildQuestion 的路由），"+
			"不是 /answers 答不了的 slotKey \"unit\"")
}

func TestAnswerUseCase_ReadyButOptionalPending_ActCodeIsAckAndAskOptional(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, unit := "水果禮盒", "KG"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, Unit: &unit},
		Source: "LLM_EXTRACT", ClientRequestID: "seed", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taoyuan", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-ready-followup-1",
	})

	require.NoError(t, err)
	require.Empty(t, got.MissingRequired, "必填欄位應該都到齊")
	require.NotNil(t, got.Question, "選填欄位（quantity）還沒填，這一輪應該還會附上一題")
	require.Equal(t, domain.ActAckAndAskOptional, got.DialoguePlan.ActCode,
		"ready 但選填問題還在，PlanDialogue 必須選 ACK_AND_ASK_OPTIONAL，不能誤選成 "+
			"REQUEST_BASICS_READY 宣稱已開始找候選")
}

func TestAnswerUseCase_ModelBlocksSetRenderModeModel(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, unit := "水果禮盒", "KG"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, Unit: &unit},
		Source: "LLM_EXTRACT", ClientRequestID: "seed", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	uc.realizer = &fakeSurfaceRealizer{blocks: []RealizedBlock{{
		Kind: domain.BlockNextStep, Text: "想問一下大概什麼時候要用？",
		ClaimCodes: []string{string(domain.ClaimNextQuestionOptional)},
	}}}

	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taoyuan", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-model-1",
	})

	require.NoError(t, err)
	require.Equal(t, "MODEL", got.RenderMode)
	require.Equal(t, "想問一下大概什麼時候要用？", got.Reply, "TextFallback 必須是模型 blocks 的 join")
}

func TestAnswerUseCase_UnknownOptionID_ReturnsErrInvalidOptionID(t *testing.T) {
	repo := newFakeRepo()
	req, rev, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-does-not-exist", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-2",
	})

	require.ErrorIs(t, err, ErrInvalidOptionID)
}

func TestAnswerUseCase_UnsupportedSlotKey_ReturnsErrInvalidOptionID(t *testing.T) {
	repo := newFakeRepo()
	req, rev, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotExactMatchRequired,
		FreeText: "true", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-5",
	})

	require.ErrorIs(t, err, ErrInvalidOptionID)
}

func TestAnswerUseCase_StaleExpectedRevision_ReturnsErrStaleRevision(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taoyuan", ExpectedRevisionID: 999999, ClientRequestID: "cr-answer-3",
	})

	require.ErrorIs(t, err, ErrStaleRevision)
}

func TestAnswerUseCase_Skip_MarksSlotSkippedWithoutChangingSlots(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city := "水果禮盒", "桃園市"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "LLM_EXTRACT", ClientRequestID: "seed", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotPackaging,
		Skip: true, ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-4",
	})

	require.NoError(t, err)
	require.Nil(t, got.Slots.Packaging)
	require.NotEqual(t, domain.SlotPackaging, safeSlotKey(got.Question), "packaging 剛被跳過，下一題不該還是 packaging")
}

func TestAnswerUseCase_OptionAnswer_MessageIsBuyerReadable_NotRawSlotKeyOrOptionID(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "水果禮盒"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: "seed", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taoyuan", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-msg-1",
	})
	require.NoError(t, err)

	stored, ok := repo.patches[fakePatchKey{1234, "cr-answer-msg-1"}]
	require.True(t, ok)
	var payload turnPayload
	require.NoError(t, json.Unmarshal(stored.PayloadJSON, &payload))

	require.NotContains(t, payload.Message, string(domain.SlotDeliveryCity), "訊息不該出現內部 slotKey 字面值")
	require.NotContains(t, payload.Message, "city-taoyuan", "訊息不該出現內部 optionId 字面值")
	require.Contains(t, payload.Message, "桃園市", "訊息要包含買家看得懂的選項文字")
}

func TestAnswerUseCase_Skip_MessageUsesDisplayLabel_NotRawSlotKey(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city := "水果禮盒", "桃園市"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "LLM_EXTRACT", ClientRequestID: "seed", PayloadJSON: []byte(`{}`), RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotPackaging,
		Skip: true, ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-msg-2",
	})
	require.NoError(t, err)

	stored, ok := repo.patches[fakePatchKey{1234, "cr-answer-msg-2"}]
	require.True(t, ok)
	var payload turnPayload
	require.NoError(t, json.Unmarshal(stored.PayloadJSON, &payload))

	require.NotContains(t, payload.Message, string(domain.SlotPackaging), "訊息不該出現內部 slotKey 字面值")
	require.Contains(t, payload.Message, domain.SlotLabels[domain.SlotPackaging], "訊息要包含該欄位的中文顯示字串")
}

func TestAnswerUseCase_ConflictWhenClientRequestIDReusedOnDifferentRequest(t *testing.T) {
	repo := newFakeRepo()
	reqA, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	itemA := "水果禮盒"
	revA, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: reqA.ID, BuyerID: 1234, Slots: domain.Slots{Item: &itemA},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-a", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	reqB, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	itemB := "生鮮蔬菜"
	revB, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: reqB.ID, BuyerID: 1234, Slots: domain.Slots{Item: &itemB},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-b", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: reqA.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taoyuan", ExpectedRevisionID: revA.ID, ClientRequestID: "cr-reused",
	})
	require.NoError(t, err)

	_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: reqB.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taipei", ExpectedRevisionID: revB.ID, ClientRequestID: "cr-reused",
	})

	require.ErrorIs(t, err, domain.ErrIdempotencyKeyConflict,
		"同一個 clientRequestId 被拿去對另一個 request 答題必須報衝突，不能靜默重播前一個 request 的舊結果")
}

func TestAnswerUseCase_IdempotentRetry_DialoguePlanIsByteEquivalent(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, unit := "水果禮盒", "KG"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, Unit: &unit},
		Source: "LLM_EXTRACT", ClientRequestID: "seed", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	in := AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taoyuan", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-retry-plan",
	}

	first, err := uc.composeAnswerViaGraph(context.Background(), in)
	require.NoError(t, err)
	require.NotEqual(t, domain.DialoguePlanV1{}, first.DialoguePlan,
		"測試前提不成立：這一輪必須先算出非零值的 plan，否則後面的相等比對沒有意義")

	second, err := uc.composeAnswerViaGraph(context.Background(), in)
	require.NoError(t, err)

	require.Equal(t, first.DialoguePlan, second.DialoguePlan,
		"重播必須回第一次已保存的 plan，不得重新計算出另一個（即使兩次算出來語意相同，PlanHash／StateVersion 等欄位也必須完全一致）")
}

func safeSlotKey(q *domain.ClarifyQuestion) domain.SlotKey {
	if q == nil {
		return ""
	}
	return q.SlotKey
}

func seedAnswerableRequest(t *testing.T, repo *fakeRepo, clientRequestID string) (int64, int64) {
	t.Helper()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city := "水果禮盒", "桃園市"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city},
		Source: "LLM_EXTRACT", ClientRequestID: clientRequestID, PayloadJSON: []byte(`{}`),
		RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	return req.ID, rev.ID
}

func seedRequirementReadyRequest(t *testing.T, repo *fakeRepo, clientRequestID string) (int64, int64) {
	t.Helper()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city, unit := "水果禮盒", "桃園市", "KG"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item, DeliveryCity: &city, Unit: &unit},
		Source: "LLM_EXTRACT", ClientRequestID: clientRequestID, PayloadJSON: []byte(`{}`),
		RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	return req.ID, rev.ID
}

func TestAnswerUseCase_Skip_QuoteRequiredSlots_ReturnsErrCannotSkip(t *testing.T) {
	for _, slotKey := range []domain.SlotKey{domain.SlotQuantity, domain.SlotDeliveryDate} {
		t.Run(string(slotKey), func(t *testing.T) {
			repo := newFakeRepo()
			requestID, revID := seedAnswerableRequest(t, repo, "seed-"+string(slotKey))

			uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
			_, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
				BuyerID: 1234, RequestID: requestID, SlotKey: slotKey, Skip: true,
				ExpectedRevisionID: revID, ClientRequestID: "cr-skip-" + string(slotKey),
			})

			require.ErrorIs(t, err, ErrCannotSkipQuoteRequiredSlot)
			_, wrote := repo.patches[fakePatchKey{1234, "cr-skip-" + string(slotKey)}]
			require.False(t, wrote, "被拒絕的 skip 不得留下任何版本或 patch")
		})
	}
}

func TestAnswerUseCase_Skip_OtherOptionalSlots_StillAllowed(t *testing.T) {
	for _, slotKey := range []domain.SlotKey{
		domain.SlotPackaging, domain.SlotBudgetPerUnit,
	} {
		t.Run(string(slotKey), func(t *testing.T) {
			repo := newFakeRepo()
			requestID, revID := seedAnswerableRequest(t, repo, "seed-ok-"+string(slotKey))

			uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
			got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
				BuyerID: 1234, RequestID: requestID, SlotKey: slotKey, Skip: true,
				ExpectedRevisionID: revID, ClientRequestID: "cr-skip-ok-" + string(slotKey),
			})

			require.NoError(t, err)
			require.NotEqual(t, slotKey, safeSlotKey(got.Question), "剛跳過的欄位不該又被問一次")
		})
	}
}

func TestAnswerUseCase_Skip_RequiredSlots_ReturnsErrCannotSkip(t *testing.T) {
	for _, slotKey := range []domain.SlotKey{domain.SlotItem, domain.SlotDeliveryCity} {
		t.Run(string(slotKey), func(t *testing.T) {
			repo := newFakeRepo()
			req, rev, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
			require.NoError(t, err)

			uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
			_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
				BuyerID: 1234, RequestID: req.ID, SlotKey: slotKey, Skip: true,
				ExpectedRevisionID: rev.ID, ClientRequestID: "cr-skip-required-" + string(slotKey),
			})

			require.ErrorIs(t, err, ErrCannotSkipQuoteRequiredSlot)
			_, wrote := repo.patches[fakePatchKey{1234, "cr-skip-required-" + string(slotKey)}]
			require.False(t, wrote, "被拒絕的 skip 不得留下任何版本或 patch")
		})
	}
}

func TestAnswerUseCase_ItemAnswer_TriggersCropResolution(t *testing.T) {
	repo := newFakeRepo()
	tree := testkit.VegetableTree()
	req, rev, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, cropidentity.NewDisambiguationUseCase(tree, nil), nil, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotItem,
		FreeText: "高麗菜", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-crop-1",
	})

	require.NoError(t, err)
	require.Equal(t, "高麗菜", *got.Slots.Item)
	require.NotZero(t, tree.Calls, "透過問題卡填入 item 一樣要解析一次")
	require.NotNil(t, got.Slots.ItemResolution, "分類樹已確認的品項直接落解析結果")
	require.Equal(t, domain.ItemResolvedCategoryConfirmed, got.Slots.ItemResolution.Kind)
	require.NotEmpty(t, got.Slots.ItemResolution.CategoryID)
	if got.Question != nil {
		require.NotEqual(t, domain.SlotCropDisambiguation, got.Question.SlotKey, "已確認分類不得再出消歧卡片")
	}
}

func TestAnswerUseCase_NonItemAnswer_DoesNotResolve(t *testing.T) {
	repo := newFakeRepo()
	tree := testkit.VegetableTree()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "高麗菜"
	// ⛔ 種子必須帶 ItemResolution：「不重新解析」的前提是**已經解析過**。
	// 只有 item、沒有 resolution 的狀態在真實流程裡只會出現在「上一次解析失敗」
	// 之後，而那個狀態現在要重試（見 cropidentity.needsResolution）。
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item,
			ItemResolution: domain.NewCategoryConfirmedItemResolution(
				&domain.CropSelection{Stage: domain.CropSelectionDescribe, CategoryID: "17", Label: "高麗菜"})},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-crop", PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	uc := NewAnswerUseCase(repo, cropidentity.NewDisambiguationUseCase(tree, nil), nil, nil, nil, nil)
	_, err = uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-taoyuan", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-crop-2",
	})

	require.NoError(t, err)
	require.Zero(t, tree.Calls, "item 沒變且已解析，就不該重新解析")
}

func seedPendingDisambiguationRequest(t *testing.T, repo *fakeRepo, clientRequestID string) (int64, int64) {
	t.Helper()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item := "小白菜"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234, Slots: domain.Slots{Item: &item},
		Source: "LLM_EXTRACT", ClientRequestID: clientRequestID, PayloadJSON: []byte(`{}`),
		RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	return req.ID, rev.ID
}

func pendingCropFixture() *CropDisambiguationInterruptPayload {
	return &CropDisambiguationInterruptPayload{
		Kind:          cropDisambiguationInterruptKind,
		PendingReason: domain.CropPendingReasonAmbiguous,
		Candidates:    []domain.CropDisambiguationOption{{Label: "有機小白菜", CategoryID: "18", CategoryLevel: 3}},
	}
}

func TestCropDisambiguationAnswer_DescribedThenItemChanges_ResolvesNormally(t *testing.T) {
	repo := newFakeRepo()
	reqID, revID := seedPendingDisambiguationRequest(t, repo, "seed-crop-described-then-change")
	tree := testkit.VegetableTree()
	cropUC := cropidentity.NewDisambiguationUseCase(tree, nil)

	patch, err := buildCropDisambiguationPatch(AnswerInput{
		BuyerID: 1234, RequestID: reqID, SlotKey: domain.SlotCropDisambiguation,
		PendingCrop: pendingCropFixture(),
		OptionID:    domain.CropOtherOptionID, FreeText: "野生烏殼綠竹筍",
		ExpectedRevisionID: revID, ClientRequestID: "cr-crop-described-then-change-1",
	})
	require.NoError(t, err)
	described, _ := domain.Slots{}.Apply(patch, time.Now().UTC())
	require.Equal(t, "野生烏殼綠竹筍", *described.Item)
	require.Zero(t, tree.Calls, "選逃生艙這一輪不得讀分類樹")
	require.Equal(t, domain.ItemResolvedBuyerDescribed, described.ItemResolution.Kind, "前置條件：逃生艙把買家的描述記成解析結果")

	before := domain.Slots{Item: described.Item}
	after := domain.Slots{Item: ptr("高麗菜")}
	node := NewCropGateNode(cropUC, seqIDs(9700))
	ctx := newTestAgentContext(t, map[string]any{
		"request_id":    reqID,
		"current_slots": before,
	})
	got, err := runFunctionNode[CropGateOutput](t, node, ctx, ValidateSlotsOutput{Slots: after})

	require.NoError(t, err)
	require.NotNil(t, got.Item)
	require.Equal(t, "高麗菜", *got.Item)
	require.NotZero(t, tree.Calls, "品項變了要重新解析")
	require.False(t, got.Pending, "分類確認後不再出卡片")
	require.NotNil(t, got.Resolution)
	require.Equal(t, "17", got.Resolution.CategoryID)
}

func TestHandle_ItemAnswer_AmbiguousItem_OverridesQuestionWithDisambiguation(t *testing.T) {
	repo := newFakeRepo()
	req, rev, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{Label: "有機小白菜", CategoryID: "18", CategoryLevel: 3},
			{Label: "小白菜（一般）", CategoryID: "19", CategoryLevel: 3},
		},
	}}

	uc := NewAnswerUseCase(repo, resolver, nil, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotItem,
		FreeText: "小白菜", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-crop-ambiguous-1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.ActItemNeedsDisambiguation, got.DialoguePlan.ActCode)
	require.NotNil(t, got.Question)
	require.Equal(t, domain.SlotCropDisambiguation, got.Question.SlotKey)
	require.Len(t, got.Question.Options, 3, "2 個真候選 + 固定附加的 D")
}

func seedQuantityKnownRequest(t *testing.T, repo *fakeRepo, clientRequestID string, qty int) (int64, int64) {
	t.Helper()
	req, _, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	item, city := "水果禮盒", "桃園市"
	rev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234,
		Slots:  domain.Slots{Item: &item, DeliveryCity: &city, Quantity: &qty},
		Source: "LLM_EXTRACT", ClientRequestID: clientRequestID, PayloadJSON: []byte(`{}`),
		RequestStatus: "REQUIREMENT_READY",
	})
	require.NoError(t, err)
	return req.ID, rev.ID
}

func TestTurnThenAnswer_UnitRequired_EndToEnd_ReachesRequirementReady(t *testing.T) {
	repo := newFakeRepo()
	turn1Slots := domain.Slots{Item: ptr("水果禮盒"), DeliveryCity: ptr("桃園市")}
	out1, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, Message: "我要水果禮盒，送桃園市", ClientRequestID: "cr-unit-required-1",
	}, GraphControllerDeps{
		RunStore:      newFakeRunStore(),
		RevisionStore: repo,
		Dispatcher:    NewRunDispatcher(),
		Runner:        newTerminalCompletionRunner(t, graphTurnProgress{Patch: domain.SlotPatch(turn1Slots), NextSlots: turn1Slots}, nil),
	}, TurnStreamHooks{})
	require.NoError(t, err)
	turn1 := out1.Result
	require.Equal(t, []domain.SlotKey{domain.SlotUnit}, turn1.MissingRequired,
		"item/city 都給了，unit 現在是唯一還缺的必填欄位")
	require.NotNil(t, turn1.Question)
	require.Equal(t, domain.SlotQuantity, turn1.Question.SlotKey,
		"unit 缺項的問題卡必須折進 quantity 複合問題卡，不能是 /answers 永遠答不了的 slotKey \"unit\"")
	require.True(t, turn1.Question.AllowFreeText, "quantity 也還沒填，這一題要能收數字")
	require.Len(t, turn1.Question.UnitOptions, 3)

	answerUC := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	got, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotQuantity,
		FreeText: "50", UnitOptionID: "unit-kg",
		ExpectedRevisionID: turn1.RevisionID, ClientRequestID: "cr-unit-required-2",
	})

	require.NoError(t, err)
	require.Empty(t, got.MissingRequired, "unit 補上後三個必填欄位都到齊，必須真的能到 REQUIREMENT_READY")
	require.NotNil(t, got.Slots.Unit)
	require.Equal(t, "KG", *got.Slots.Unit)
	require.NotNil(t, got.Slots.Quantity)
	require.Equal(t, 50, *got.Slots.Quantity)
}

func TestAnswerUseCase_Quantity_CompoundAnswer_SetsQuantityAndUnit(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-compound")

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotQuantity,
		FreeText: "100", UnitOptionID: "unit-kg",
		ExpectedRevisionID: revID, ClientRequestID: "cr-quantity-compound",
	})

	require.NoError(t, err)
	require.NotNil(t, got.Slots.Quantity)
	require.Equal(t, 100, *got.Slots.Quantity)
	require.NotNil(t, got.Slots.Unit)
	require.Equal(t, "KG", *got.Slots.Unit)

	stored, ok := repo.patches[fakePatchKey{1234, "cr-quantity-compound"}]
	require.True(t, ok)
	var payload turnPayload
	require.NoError(t, json.Unmarshal(stored.PayloadJSON, &payload))
	require.Contains(t, payload.Message, "100", "訊息要包含買家剛填的數量")
	require.Contains(t, payload.Message, "公斤", "訊息要包含買家剛選的單位顯示字串")
}

func TestAnswerUseCase_Quantity_FollowUpAnswer_SetsUnitOnly(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedQuantityKnownRequest(t, repo, "seed-followup", 50)

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotQuantity,
		UnitOptionID: "unit-catty", ExpectedRevisionID: revID, ClientRequestID: "cr-quantity-followup",
	})

	require.NoError(t, err)
	require.NotNil(t, got.Slots.Quantity)
	require.Equal(t, 50, *got.Slots.Quantity, "quantity 應維持原值")
	require.NotNil(t, got.Slots.Unit)
	require.Equal(t, "CATTY", *got.Slots.Unit)

	stored, ok := repo.patches[fakePatchKey{1234, "cr-quantity-followup"}]
	require.True(t, ok)
	var payload turnPayload
	require.NoError(t, json.Unmarshal(stored.PayloadJSON, &payload))
	require.NotEqual(t, "數量：", payload.Message, "單位追問不該渲染成空值的數量訊息")
	require.Contains(t, payload.Message, "台斤", "訊息要包含買家剛選的單位顯示字串")
}

func TestAnswerUseCase_Quantity_MissingUnitWhenUnitUnknown_ReturnsErrInvalidOptionID(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-missing-unit")

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotQuantity,
		FreeText: "100", ExpectedRevisionID: revID, ClientRequestID: "cr-quantity-missing-unit",
	})

	require.ErrorIs(t, err, ErrInvalidOptionID)
	_, wrote := repo.patches[fakePatchKey{1234, "cr-quantity-missing-unit"}]
	require.False(t, wrote)
}

func TestAnswerUseCase_Quantity_MissingAmountWhenQuantityUnknown_ReturnsErrInvalidOptionID(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-missing-amount")

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotQuantity,
		UnitOptionID: "unit-kg", ExpectedRevisionID: revID, ClientRequestID: "cr-quantity-missing-amount",
	})

	require.ErrorIs(t, err, ErrInvalidOptionID)
}

func TestAnswerUseCase_Quantity_InvalidUnitOptionID_ReturnsErrInvalidOptionID(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-bad-unit")

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	_, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotQuantity,
		FreeText: "100", UnitOptionID: "unit-does-not-exist",
		ExpectedRevisionID: revID, ClientRequestID: "cr-quantity-bad-unit",
	})

	require.ErrorIs(t, err, ErrInvalidOptionID)
}

func TestAnswerUseCase_SlotKeyUnit_Rejected_ReturnsErrInvalidOptionID(t *testing.T) {
	t.Run("freeText", func(t *testing.T) {
		repo := newFakeRepo()
		requestID, revID := seedAnswerableRequest(t, repo, "seed-unit-direct")

		uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
		_, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
			BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotUnit,
			FreeText: "KG", ExpectedRevisionID: revID, ClientRequestID: "cr-unit-direct-1",
		})

		require.ErrorIs(t, err, ErrInvalidOptionID)
	})

	t.Run("skip", func(t *testing.T) {
		repo := newFakeRepo()
		requestID, revID := seedAnswerableRequest(t, repo, "seed-unit-direct-skip")

		uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
		_, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
			BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotUnit,
			Skip: true, ExpectedRevisionID: revID, ClientRequestID: "cr-unit-direct-2",
		})

		require.ErrorIs(t, err, ErrInvalidOptionID)
	})
}

func TestAnswerUseCase_ReadyWithoutPricePreview_SetsNextActionReviewPricePreview(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedRequirementReadyRequest(t, repo, "seed-next-action-1")

	uc := NewAnswerUseCase(repo, nil, fakePricePreviewChecker{hasPreview: false}, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotBudgetPerUnit,
		FreeText: "800", ExpectedRevisionID: revID, ClientRequestID: "cr-next-action-1",
	})

	require.NoError(t, err)
	require.Empty(t, got.MissingRequired)
	require.Equal(t, "REVIEW_PRICE_PREVIEW", got.NextAction)
}

func TestAnswerUseCase_ReadyWithExistingPricePreview_NextActionIsEmpty(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedRequirementReadyRequest(t, repo, "seed-next-action-2")

	uc := NewAnswerUseCase(repo, nil, fakePricePreviewChecker{hasPreview: true}, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotBudgetPerUnit,
		FreeText: "800", ExpectedRevisionID: revID, ClientRequestID: "cr-next-action-2",
	})

	require.NoError(t, err)
	require.Empty(t, got.NextAction)
}

func TestAnswerUseCase_ReadyWithNilPricePreviewChecker_NextActionIsEmpty(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedRequirementReadyRequest(t, repo, "seed-next-action-3")

	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	got, err := uc.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotBudgetPerUnit,
		FreeText: "800", ExpectedRevisionID: revID, ClientRequestID: "cr-next-action-3",
	})

	require.NoError(t, err)
	require.Empty(t, got.NextAction)
}

type countingAnswerStore struct {
	AnswerStore
	appendCalls int
}

func (c *countingAnswerStore) AppendRevision(ctx context.Context, in repository.AppendInput) (repository.Revision, error) {
	c.appendCalls++
	return c.AnswerStore.AppendRevision(ctx, in)
}

func TestClaimAndDispatchAnswer_DuplicateClientRequestID_DoesNotRerunPipeline(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-claim-answer-1")
	store := &countingAnswerStore{AnswerStore: repo}
	uc := NewAnswerUseCase(store, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	in := AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
		ExpectedRevisionID: revID, ClientRequestID: "cr-claim-answer-dup-1",
	}

	first, err := uc.ClaimAndDispatchAnswer(context.Background(), in, runStore, NewRunDispatcher())
	require.NoError(t, err)
	require.Equal(t, 1, store.appendCalls, "背景 pipeline 必須真的跑過一次")
	require.NotZero(t, first.RunID, "回傳值必須帶 ledger 產生的 run_id")
	require.NotZero(t, first.Result.RevisionID, "呼叫端拿到的是終態結果，不是空殼")

	second, err := uc.ClaimAndDispatchAnswer(context.Background(), in, runStore, NewRunDispatcher())
	require.NoError(t, err)

	require.Equal(t, 1, store.appendCalls, "重送不得重跑 pipeline／不得再寫一次 revision")
	require.Equal(t, first.RunID, second.RunID, "重送要對到同一個 run，不得另建一個")
	require.Equal(t, first.Result.RequestID, second.Result.RequestID)
	require.Equal(t, first.Result.RevisionID, second.Result.RevisionID)
	require.Equal(t, first.Result.Reply, second.Result.Reply)
	require.Equal(t, first.Result.Slots, second.Result.Slots)

	require.Len(t, runStore.terminals, 1, "重播不得重新終態化這個 run")
}

type panickingAnswerStore struct {
	AnswerStore
}

func (panickingAnswerStore) GetSnapshot(context.Context, int64, identity.BuyerID) (repository.Request, repository.Revision, error) {
	panic("boom inside the answer pipeline")
}

func TestClaimAndDispatchAnswer_BackgroundPanic_DoesNotHangCaller(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-panic-answer-1")
	uc := NewAnswerUseCase(panickingAnswerStore{AnswerStore: repo}, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()

	type outcome struct {
		res SubmitTurnResult
		err error
	}
	ch := make(chan outcome, 1)
	go func() {
		res, err := uc.ClaimAndDispatchAnswer(context.Background(), AnswerInput{
			BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
			ExpectedRevisionID: revID, ClientRequestID: "cr-answer-panic-1",
		}, runStore, NewRunDispatcher())
		ch <- outcome{res, err}
	}()

	select {
	case got := <-ch:
		require.Error(t, got.err, "panic 之後呼叫端必須拿到錯誤，不是零值結果")
	case <-time.After(5 * time.Second):
		t.Fatal("背景 panic 之後 ClaimAndDispatchAnswer 沒有返回——呼叫端被卡住了")
	}

	require.Len(t, runStore.terminals, 1)
	require.Equal(t, domain.RunStatusFailed(), runStore.terminals[0].Status)
	require.Equal(t, codemeta.ProcurementRunFailureReasonTurnPipelineError.Value, runStore.terminals[0].FailureReason)
}

func TestExecuteAnswerPipeline_CropDisambiguationPending_TransitionsToWaitingForInput(t *testing.T) {
	repo := newFakeRepo()
	req, rev, err := repo.CreateRequest(context.Background(), identity.BuyerID(1234))
	require.NoError(t, err)
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{Label: "有機小白菜", CategoryID: "18", CategoryLevel: 3},
			{Label: "小白菜（一般）", CategoryID: "19", CategoryLevel: 3},
		},
	}}
	uc := NewAnswerUseCase(repo, resolver, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9101, BuyerID: 1234, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}
	in := AnswerInput{
		BuyerID: 1234, RequestID: req.ID, SlotKey: domain.SlotItem,
		FreeText: "小白菜", ExpectedRevisionID: rev.ID, ClientRequestID: "cr-answer-crop-interrupt-1",
	}

	outcome, err := uc.executeAnswerPipeline(context.Background(), run, in, runStore)
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

func TestExecuteAnswerPipeline_NormalCompletion_DoesNotCreateInterrupt(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-normal-answer-1")
	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9102, BuyerID: 1234, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}
	in := AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
		ExpectedRevisionID: revID, ClientRequestID: "cr-answer-normal-1",
	}

	outcome, err := uc.executeAnswerPipeline(context.Background(), run, in, runStore)
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
			"ComputeAnswerSteps 產生的 step 永遠是 SUCCEEDED，因此永遠換算成 STEP_FINISHED")
	}
}

func TestExecuteAnswerPipeline_ConversationIDZeroAtClaim_BackfillsRunConversationID(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-backfill-answer-1")
	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9110, BuyerID: 1234, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}
	in := AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
		ExpectedRevisionID: revID, ClientRequestID: "cr-answer-backfill-1",
	}

	outcome, err := uc.executeAnswerPipeline(context.Background(), run, in, runStore)
	require.NoError(t, err)
	require.NotZero(t, outcome.result.ConversationID, "前置條件：pipeline 必須真的解出 conversationId")

	require.Len(t, runStore.setConversationIDCalls, 1, "run.ConversationID 在 claim 當下是 0，pipeline 跑完後必須回填")
	call := runStore.setConversationIDCalls[0]
	require.Equal(t, run.ID, call.runID)
	require.Equal(t, outcome.result.ConversationID, call.conversationID)
}

func TestExecuteAnswerPipeline_RunAlreadyHasConversationID_DoesNotBackfill(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-noop-answer-1")
	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9111, BuyerID: 1234, ConversationID: 5555, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}
	in := AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
		ExpectedRevisionID: revID, ClientRequestID: "cr-answer-noop-1",
	}

	outcome, err := uc.executeAnswerPipeline(context.Background(), run, in, runStore)
	require.NoError(t, err)
	require.NotZero(t, outcome.result.ConversationID)

	require.Empty(t, runStore.setConversationIDCalls, "run 在 claim 當下已經有 conversation_id，不該再回填")
}

func TestExecuteAnswerPipeline_PipelineError_TerminalizesAsFailed(t *testing.T) {
	repo := newFakeRepo()
	requestID, _ := seedAnswerableRequest(t, repo, "seed-fail-answer-1")
	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	run := repository.Run{ID: 9103, BuyerID: 1234, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}

	_, err := uc.executeAnswerPipeline(context.Background(), run, AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
		ExpectedRevisionID: 424242, ClientRequestID: "cr-answer-fail-1",
	}, runStore)

	require.ErrorIs(t, err, ErrStaleRevision)
	require.Len(t, runStore.terminals, 1)
	terminal := runStore.terminals[0]
	require.Equal(t, domain.RunStatusFailed(), terminal.Status)
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunFailed.Value, terminal.TerminalEventType)
	require.Equal(t, codemeta.ProcurementRunFailureReasonTurnPipelineError.Value, terminal.FailureReason)
	require.Empty(t, runStore.interrupts, "失敗的 run 不得建立 interrupt")
	require.Empty(t, runStore.setConversationIDCalls,
		"pipeline 失敗、result 是零值，沒有真正的 conversationId 可以回填")
}

func TestClaimAndDispatchAnswer_OpenInterruptExists_NewCommand_BlockedByGuard(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-guard-new-1")
	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()
	runStore.hasOpenInterrupt = true
	runStore.openInterrupt = repository.Interrupt{ID: "int-open-1", RunID: 9200, Status: "PENDING"}

	_, err := uc.ClaimAndDispatchAnswer(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
		ExpectedRevisionID: revID, ClientRequestID: "cr-guard-new-1",
	}, runStore, NewRunDispatcher())

	require.ErrorIs(t, err, ErrInterruptPending)
	require.Empty(t, runStore.runs, "guard 擋下時不得呼叫 ClaimRun，零 mutation")
}

func TestClaimAndDispatchAnswer_OpenInterruptExists_IdempotentRetry_NotBlocked(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-guard-retry-1")
	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()

	in := AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
		ExpectedRevisionID: revID, ClientRequestID: "cr-guard-retry-1",
	}
	first, err := uc.ClaimAndDispatchAnswer(context.Background(), in, runStore, NewRunDispatcher())
	require.NoError(t, err)

	runStore.hasOpenInterrupt = true
	runStore.openInterrupt = repository.Interrupt{ID: "int-open-2", RunID: 9201, Status: "PENDING"}

	second, err := uc.ClaimAndDispatchAnswer(context.Background(), in, runStore, NewRunDispatcher())
	require.NoError(t, err, "冪等重送必須放行到 ClaimRun 自己的既有回放路徑，不被 guard 擋下")
	require.Equal(t, first.RunID, second.RunID, "重送要對到同一個 run，不得被 guard 攔成錯誤")
}

func TestClaimAndDispatchAnswer_WritesControllerKindGraph(t *testing.T) {
	repo := newFakeRepo()
	requestID, revID := seedAnswerableRequest(t, repo, "seed-controllerkind-graph-1")
	uc := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	runStore := newFakeRunStore()

	_, err := uc.ClaimAndDispatchAnswer(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: requestID, SlotKey: domain.SlotPackaging, FreeText: "紙箱",
		ExpectedRevisionID: revID, ClientRequestID: "cr-controllerkind-answer-graph-1",
	}, runStore, NewRunDispatcher())
	require.NoError(t, err)

	require.Len(t, runStore.runs, 1)
	for _, run := range runStore.runs {
		require.Equal(t, domain.ControllerKindGraph, run.ControllerKind)
	}
}

func TestCanonicalAnswerHash_IncludesRequestID(t *testing.T) {
	base := AnswerInput{BuyerID: 1, RequestID: 100, SlotKey: domain.SlotPackaging, FreeText: "紙箱", ExpectedRevisionID: 1}
	otherRequest := base
	otherRequest.RequestID = 200

	require.NotEqual(t, canonicalAnswerHash(base), canonicalAnswerHash(otherRequest))

	otherFreeText := base
	otherFreeText.FreeText = "袋裝"
	require.NotEqual(t, canonicalAnswerHash(base), canonicalAnswerHash(otherFreeText))

	same := base
	require.Equal(t, canonicalAnswerHash(base), canonicalAnswerHash(same))
}

func mustMarshalSlots(t *testing.T, slots domain.Slots) []byte {
	t.Helper()
	b, err := json.Marshal(slots)
	require.NoError(t, err)
	return b
}

type fakeAnswerStore struct {
	snapshot     repository.Request
	activeRev    repository.Revision
	appendResult repository.Revision
	history      []repository.RevisionWithPatch
	idempotent   repository.IdempotentResult
	idempotentOK bool
}

func (f *fakeAnswerStore) GetSnapshot(ctx context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error) {
	return f.snapshot, f.activeRev, nil
}

func (f *fakeAnswerStore) ListBranchRevisions(ctx context.Context, requestID int64, buyerID identity.BuyerID) ([]repository.RevisionWithPatch, error) {
	return f.history, nil
}

func (f *fakeAnswerStore) AppendRevision(ctx context.Context, in repository.AppendInput) (repository.Revision, error) {
	return f.appendResult, nil
}

func (f *fakeAnswerStore) FindByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.IdempotentResult, error) {
	if f.idempotentOK {
		return f.idempotent, nil
	}
	return repository.IdempotentResult{}, repository.ErrNotFound
}

func TestComposeAnswerViaGraph_HappyPath_RecordsAnswerAndAsksNext(t *testing.T) {
	store := &fakeAnswerStore{
		snapshot:     repository.Request{ID: 1},
		activeRev:    repository.Revision{ID: 10, SlotsJSON: mustMarshalSlots(t, domain.Slots{})},
		appendResult: repository.Revision{ID: 11, ConversationID: 100},
	}
	uc := NewAnswerUseCase(store, &testkit.FakeResolver[cropidentity.DisambiguationResult]{}, nil, nil, nil, nil)
	in := AnswerInput{
		BuyerID: 1234, RequestID: 1, SlotKey: domain.SlotItem,
		FreeText: "蘋果", ExpectedRevisionID: 10, ClientRequestID: "req-1",
	}

	result, err := uc.composeAnswerViaGraph(context.Background(), in)

	require.NoError(t, err)
	require.Equal(t, "TEMPLATE", result.RenderMode, "graph path always uses domain.RenderPlan, never SurfaceRealizer")
	require.NotEmpty(t, result.Reply)
	require.Equal(t, int64(11), result.RevisionID)
	require.Equal(t, int64(100), result.ConversationID)
	require.NotNil(t, result.Slots.Item)
	require.Equal(t, "蘋果", *result.Slots.Item)
}

func TestComposeAnswerViaGraph_MissingRequired_AsksNextQuestion(t *testing.T) {
	store := &fakeAnswerStore{
		snapshot:     repository.Request{ID: 1},
		activeRev:    repository.Revision{ID: 10, SlotsJSON: mustMarshalSlots(t, domain.Slots{})},
		appendResult: repository.Revision{ID: 11},
	}
	uc := NewAnswerUseCase(store, &testkit.FakeResolver[cropidentity.DisambiguationResult]{}, nil, nil, nil, nil)
	in := AnswerInput{
		BuyerID: 1234, RequestID: 1, SlotKey: domain.SlotItem,
		FreeText: "蘋果", ExpectedRevisionID: 10, ClientRequestID: "req-2",
	}

	result, err := uc.composeAnswerViaGraph(context.Background(), in)

	require.NoError(t, err)
	require.NotEmpty(t, result.MissingRequired, "slots beyond item are still missing, status should be CLARIFYING")
	require.NotNil(t, result.Question, "must return a question card when required slots are missing")
}

func TestComposeAnswerViaGraph_SameTurnSkip_ReflectedInThisTurnsQuestion(t *testing.T) {
	item := "蘋果"
	city := "臺北市"
	unit := "KG"
	qty := 100
	deliveryDate := "2026-09-15"
	store := &fakeAnswerStore{
		snapshot: repository.Request{ID: 1},
		activeRev: repository.Revision{
			ID: 10,
			SlotsJSON: mustMarshalSlots(t, domain.Slots{
				Item:         &item,
				DeliveryCity: &city,
				Unit:         &unit,
				Quantity:     &qty,
				DeliveryDate: &deliveryDate,
			}),
		},
		appendResult: repository.Revision{ID: 11},
		history: []repository.RevisionWithPatch{{
			Revision: repository.Revision{
				ID: 10,
				SlotsJSON: mustMarshalSlots(t, domain.Slots{
					Item:         &item,
					DeliveryCity: &city,
					Unit:         &unit,
					Quantity:     &qty,
					DeliveryDate: &deliveryDate,
				}),
			},
		}},
	}
	uc := NewAnswerUseCase(store, &testkit.FakeResolver[cropidentity.DisambiguationResult]{}, nil, nil, nil, nil)
	in := AnswerInput{
		BuyerID: 1234, RequestID: 1, SlotKey: domain.SlotPackaging,
		Skip: true, ExpectedRevisionID: 10, ClientRequestID: "req-3",
	}

	result, err := uc.composeAnswerViaGraph(context.Background(), in)

	require.NoError(t, err)
	require.NotNil(t, result.Question, "should have a question for the next skippable optional slot")
	require.Equal(t, domain.SlotBudgetPerUnit, result.Question.SlotKey,
		"packaging was just skipped this turn, so the next eligible optional slot is budget_per_unit "+
			"— this assertion proves the same-turn skip (answer.go:229-231) was honored, not deferred to history")
}

func TestComposeAnswerViaGraph_CropGateUnavailable_AsksDisambiguationInsteadOfRejecting(t *testing.T) {
	store := &fakeAnswerStore{
		snapshot:     repository.Request{ID: 1},
		activeRev:    repository.Revision{ID: 10, SlotsJSON: mustMarshalSlots(t, domain.Slots{})},
		appendResult: repository.Revision{ID: 11},
	}
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{Unavailable: true}}
	uc := NewAnswerUseCase(store, resolver, nil, nil, nil, nil)
	in := AnswerInput{
		BuyerID: 1234, RequestID: 1, SlotKey: domain.SlotItem,
		FreeText: "不存在的品項", ExpectedRevisionID: 10, ClientRequestID: "req-4",
	}

	result, err := uc.composeAnswerViaGraph(context.Background(), in)

	require.NoError(t, err)
	require.Empty(t, result.Rejected, "查無候選是待處理，不是拒絕")
	require.NotNil(t, result.Question)
	require.Equal(t, domain.SlotCropDisambiguation, result.Question.SlotKey)
	require.Len(t, result.Question.Options, 1, "0 候選時只剩固定的「其他（自己輸入）」")
	require.Equal(t, domain.CropOtherOptionID, result.Question.Options[0].OptionID)
}

func TestNewAnswerUseCase_KeepsNarrator(t *testing.T) {
	n := &fakeStepNarrator{text: "narrated"}
	uc := NewAnswerUseCase(nil, nil, nil, nil, n, nil)
	if uc.narrator == nil {
		t.Fatal("narrator 沒有被存進 AnswerUseCase——steps[].text 會全空")
	}
}

func TestNewRewindUseCase_KeepsNarrator(t *testing.T) {
	n := &fakeStepNarrator{text: "narrated"}
	uc := NewRewindUseCase(nil, nil, nil, nil, n, nil)
	if uc.narrator == nil {
		t.Fatal("narrator 沒有被存進 RewindUseCase——steps[].text 會全空")
	}
}

func TestBuildCropDisambiguationPatch_DescriptionRequired(t *testing.T) {
	in := AnswerInput{
		BuyerID: 1234, SlotKey: domain.SlotCropDisambiguation,
		PendingCrop: pendingCropFixture(),
		OptionID:    domain.CropOtherOptionID, FreeText: "   ",
	}
	_, err := buildCropDisambiguationPatch(in)
	require.ErrorIs(t, err, ErrInvalidOptionID)

	in.FreeText = " 野生烏殼綠竹筍 "
	patch, err := buildCropDisambiguationPatch(in)
	require.NoError(t, err)
	require.Equal(t, "野生烏殼綠竹筍", *patch.Item)
	require.Equal(t, domain.ItemResolvedBuyerDescribed, patch.ItemResolution.Kind)
	require.Equal(t, "野生烏殼綠竹筍", patch.ItemResolution.Label)
}

func TestBuildCropDisambiguationPatch_PendingGate(t *testing.T) {
	t.Run("沒有正在回答的中斷——拒絕", func(t *testing.T) {
		_, err := buildCropDisambiguationPatch(AnswerInput{
			SlotKey: domain.SlotCropDisambiguation, OptionID: domain.CropOtherOptionID,
			FreeText: "隨便",
		})
		require.ErrorIs(t, err, ErrInvalidOptionID)
	})

	t.Run("UNAVAILABLE（0 候選）——crop-other 逃生艙可用", func(t *testing.T) {
		patch, err := buildCropDisambiguationPatch(AnswerInput{
			SlotKey: domain.SlotCropDisambiguation, OptionID: domain.CropOtherOptionID,
			FreeText: "紫蘇",
			PendingCrop: &CropDisambiguationInterruptPayload{
				Kind: cropDisambiguationInterruptKind, PendingReason: domain.CropPendingReasonUnavailable,
			},
		})
		require.NoError(t, err)
		require.Equal(t, "紫蘇", *patch.Item)
		require.Equal(t, domain.ItemResolvedBuyerDescribed, patch.ItemResolution.Kind)
	})

	t.Run("UNAVAILABLE（0 候選）、送非 crop-other 的 optionId——乾淨 422", func(t *testing.T) {
		_, err := buildCropDisambiguationPatch(AnswerInput{
			SlotKey: domain.SlotCropDisambiguation, OptionID: "category-18",
			PendingCrop: &CropDisambiguationInterruptPayload{
				Kind: cropDisambiguationInterruptKind, PendingReason: domain.CropPendingReasonUnavailable,
			},
		})
		require.ErrorIs(t, err, ErrInvalidOptionID,
			"候選清單是空的，白名單比對必然落空——要回乾淨的 422，不是原始解析錯誤")
	})

	t.Run("分類候選不走 patch：由 resolver 的 ResolveSelection 處理，這裡一律拒絕", func(t *testing.T) {
		_, err := buildCropDisambiguationPatch(AnswerInput{
			SlotKey: domain.SlotCropDisambiguation, OptionID: "category-18",
			PendingCrop: &CropDisambiguationInterruptPayload{
				Kind: cropDisambiguationInterruptKind, PendingReason: domain.CropPendingReasonAmbiguous,
				Candidates: []domain.CropDisambiguationOption{{Label: "小白菜", CategoryID: "18", CategoryLevel: 3}},
			},
		})
		require.ErrorIs(t, err, ErrInvalidOptionID)
	})
}

func TestBuildCropDisambiguationPatch_EscapeReplacesItemAndDropsStaleResolution(t *testing.T) {
	patch, err := buildCropDisambiguationPatch(AnswerInput{
		SlotKey: domain.SlotCropDisambiguation, OptionID: domain.CropOtherOptionID,
		FreeText: "紫蘇",
		PendingCrop: &CropDisambiguationInterruptPayload{
			Kind: cropDisambiguationInterruptKind, PendingReason: domain.CropPendingReasonUnavailable,
		},
	})
	require.NoError(t, err)

	before := domain.Slots{Item: item("星際迷航火星藜麥"), ItemResolution: testkit.ConfirmedResolution("火星藜麥")}
	after, rejected := before.Apply(patch, time.Now())

	require.Empty(t, rejected)
	require.Equal(t, "紫蘇", *after.Item)
	require.Equal(t, &domain.ItemResolution{
		Kind: domain.ItemResolvedBuyerDescribed, Label: "紫蘇", Provenance: domain.ItemResolutionByBuyerChoice,
	}, after.ItemResolution, "舊品項的分類不得沿用到買家改寫的描述")
}
