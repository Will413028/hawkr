package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/stretchr/testify/require"
)

func testDeliveryDate() string {
	return time.Now().UTC().AddDate(0, 0, 14).Format(domain.DateLayout)
}

func TestOptionalStreak_CapPersistsAcrossImmediateFollowingTurn(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeRunStore()
	graphTurnDeps := func(progress graphTurnProgress) GraphControllerDeps {
		return GraphControllerDeps{
			RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
			Runner: newTerminalCompletionRunner(t, progress, nil),
		}
	}

	turn1Slots := domain.Slots{Item: ptr("水果禮盒"), DeliveryCity: ptr("桃園市"), Unit: ptr("KG")}
	out1, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, Message: "我要水果禮盒，送桃園市，用公斤計價", ClientRequestID: "cr-cap-1",
	}, graphTurnDeps(graphTurnProgress{Patch: domain.SlotPatch(turn1Slots), NextSlots: turn1Slots}), TurnStreamHooks{})
	require.NoError(t, err)
	turn1 := out1.Result
	require.NotNil(t, turn1.Question)
	require.Equal(t, domain.SlotQuantity, turn1.Question.SlotKey, "選填優先序第一個未填欄位（unit 已知，只追問數量）")

	answerUC := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)

	turn2, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotQuantity,
		FreeText: "100", ExpectedRevisionID: turn1.RevisionID, ClientRequestID: "cr-cap-2",
	})
	require.NoError(t, err)
	require.NotNil(t, turn2.Question)
	require.Equal(t, domain.SlotDeliveryDate, turn2.Question.SlotKey)

	turn3, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotDeliveryDate,
		FreeText: testDeliveryDate(), ExpectedRevisionID: turn2.RevisionID, ClientRequestID: "cr-cap-3",
	})
	require.NoError(t, err)
	require.NotNil(t, turn3.Question)
	require.Equal(t, domain.SlotPackaging, turn3.Question.SlotKey)

	turn4, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotPackaging,
		FreeText: "紙箱", ExpectedRevisionID: turn3.RevisionID, ClientRequestID: "cr-cap-4",
	})
	require.NoError(t, err)
	require.Nil(t, turn4.Question,
		"連問 3 題選填（quantity/delivery_date/packaging）已達 MaxConsecutiveOptionalQuestions，第 4 輪不再主動發問")

	quantity, deliveryDate, packaging := 100, testDeliveryDate(), "紙箱"
	idleSlots := domain.Slots{
		Item: ptr("水果禮盒"), DeliveryCity: ptr("桃園市"), Unit: ptr("KG"),
		Quantity: &quantity, DeliveryDate: &deliveryDate, Packaging: &packaging,
	}
	out5, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, RequestID: turn1.RequestID, Message: "謝謝", ClientRequestID: "cr-cap-5",
	}, graphTurnDeps(graphTurnProgress{Patch: domain.SlotPatch{}, NextSlots: idleSlots}), TurnStreamHooks{})
	require.NoError(t, err)
	require.Nil(t, out5.Result.Question,
		"cap 觸發後緊接的下一輪，cap 狀態必須繼續生效——這是本輪修復刻意保留、"+
			"不能因為 streak 重新設計而退回去的行為")
}

func TestOptionalStreak_RewindReopeningOptionalSlot_ResetsStreakForFollowingTurn(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeRunStore()
	graphTurnDeps := func(progress graphTurnProgress) GraphControllerDeps {
		return GraphControllerDeps{
			RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
			Runner: newTerminalCompletionRunner(t, progress, nil),
		}
	}

	turn1Slots := domain.Slots{Item: ptr("水果禮盒"), DeliveryCity: ptr("桃園市"), Unit: ptr("KG")}
	out1, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, Message: "我要水果禮盒，送桃園市，用公斤計價", ClientRequestID: "cr-rw-1",
	}, graphTurnDeps(graphTurnProgress{Patch: domain.SlotPatch(turn1Slots), NextSlots: turn1Slots}), TurnStreamHooks{})
	require.NoError(t, err)
	turn1 := out1.Result
	require.Equal(t, domain.SlotQuantity, turn1.Question.SlotKey)

	answerUC := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)

	turn2, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotQuantity,
		FreeText: "100", ExpectedRevisionID: turn1.RevisionID, ClientRequestID: "cr-rw-2",
	})
	require.NoError(t, err)
	require.Equal(t, domain.SlotDeliveryDate, turn2.Question.SlotKey)

	turn3, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotDeliveryDate,
		FreeText: testDeliveryDate(), ExpectedRevisionID: turn2.RevisionID, ClientRequestID: "cr-rw-3",
	})
	require.NoError(t, err)
	require.Equal(t, domain.SlotPackaging, turn3.Question.SlotKey)

	turn4, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotPackaging,
		FreeText: "紙箱", ExpectedRevisionID: turn3.RevisionID, ClientRequestID: "cr-rw-4",
	})
	require.NoError(t, err)
	require.Nil(t, turn4.Question, "cap 已打滿（quantity/delivery_date/packaging 三題選填）")

	quantity, deliveryDate, packaging := 100, testDeliveryDate(), "紙箱"
	idleSlots := domain.Slots{
		Item: ptr("水果禮盒"), DeliveryCity: ptr("桃園市"), Unit: ptr("KG"),
		Quantity: &quantity, DeliveryDate: &deliveryDate, Packaging: &packaging,
	}
	out5, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, RequestID: turn1.RequestID, Message: "謝謝", ClientRequestID: "cr-rw-5",
	}, graphTurnDeps(graphTurnProgress{Patch: domain.SlotPatch{}, NextSlots: idleSlots}), TurnStreamHooks{})
	require.NoError(t, err)
	require.Nil(t, out5.Result.Question)
	out6, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, RequestID: turn1.RequestID, Message: "好的", ClientRequestID: "cr-rw-6",
	}, graphTurnDeps(graphTurnProgress{Patch: domain.SlotPatch{}, NextSlots: idleSlots}), TurnStreamHooks{})
	require.NoError(t, err)
	require.Nil(t, out6.Result.Question)

	rewindUC := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	rewindResult, err := rewindUC.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotPackaging, ClientRequestID: "cr-rw-7",
	})
	require.NoError(t, err)
	require.Nil(t, rewindResult.Slots.Packaging, "packaging 應該被回退掉")
	require.NotNil(t, rewindResult.Question)
	require.Equal(t, domain.SlotPackaging, rewindResult.Question.SlotKey, "rewind 永遠重問買家指定的欄位，不管 cap")

	turn8, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotPackaging,
		FreeText: "紙箱", ExpectedRevisionID: rewindResult.RevisionID, ClientRequestID: "cr-rw-8",
	})
	require.NoError(t, err)
	require.NotNil(t, turn8.Question,
		"rewind 之後的下一輪不該被 rewind 前、完全無關的舊 cap 狀態卡住")
	require.Equal(t, domain.SlotBudgetPerUnit, turn8.Question.SlotKey,
		"unit 在 turn1 已隨必填欄位一起給出，quantity 在 turn2 單獨答過，"+
			"delivery_date／packaging 在 cap streak 三題內已經問過（packaging 並透過 rewind 重新確認），"+
			"reset 後的 streak 必須讓 PickNextQuestion 願意問優先序上第一個還沒問過的選填欄位——budget_per_unit")
}

func TestOptionalStreak_NothingLeftToAsk_ResetsToZero_NotStaleAccumulatedValue(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeRunStore()
	graphTurnDeps := func(progress graphTurnProgress) GraphControllerDeps {
		return GraphControllerDeps{
			RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
			Runner: newTerminalCompletionRunner(t, progress, nil),
		}
	}

	turn1Slots := domain.Slots{Item: ptr("水果禮盒"), DeliveryCity: ptr("桃園市")}
	out1, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, Message: "我要水果禮盒，送桃園市", ClientRequestID: "cr-done-1",
	}, graphTurnDeps(graphTurnProgress{Patch: domain.SlotPatch(turn1Slots), NextSlots: turn1Slots}), TurnStreamHooks{})
	require.NoError(t, err)
	turn1 := out1.Result
	require.Equal(t, domain.SlotQuantity, turn1.Question.SlotKey)

	answerUC := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)
	turn2, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turn1.RequestID, SlotKey: domain.SlotQuantity,
		FreeText: "100", UnitOptionID: "unit-kg", ExpectedRevisionID: turn1.RevisionID, ClientRequestID: "cr-done-2",
	})
	require.NoError(t, err)
	require.Equal(t, domain.SlotDeliveryDate, turn2.Question.SlotKey)
	require.NotNil(t, turn2.Slots.Quantity, "sanity check: turn2 的複合問答必須真的填了 quantity")
	require.NotNil(t, turn2.Slots.Unit, "sanity check: turn2 的複合問答必須真的隨 UnitOptionID 填了 unit")

	turn3Patch := domain.SlotPatch{
		Packaging: ptr("禮盒"), BudgetPerUnit: ptr(800), DeliveryDate: ptr(testDeliveryDate()),
	}
	turn3Slots := domain.Slots{
		Item: turn2.Slots.Item, DeliveryCity: turn2.Slots.DeliveryCity,
		Quantity: turn2.Slots.Quantity, Unit: turn2.Slots.Unit,
		Packaging: turn3Patch.Packaging, BudgetPerUnit: turn3Patch.BudgetPerUnit, DeliveryDate: turn3Patch.DeliveryDate,
	}
	out3, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, RequestID: turn1.RequestID, Message: "一次補齊剩下的資訊", ClientRequestID: "cr-done-3",
	}, graphTurnDeps(graphTurnProgress{Patch: turn3Patch, NextSlots: turn3Slots}), TurnStreamHooks{})
	require.NoError(t, err)
	require.Nil(t, out3.Result.Question, "所有欄位都填齊了，沒有東西可問")

	stored, ok := repo.patches[fakePatchKey{1234, "cr-done-3"}]
	require.True(t, ok)
	var payload turnPayload
	require.NoError(t, json.Unmarshal(stored.PayloadJSON, &payload))
	require.Equal(t, 0, payload.OptionalStreak,
		"「真的沒東西可問」的 nil 必須讓 streak 歸零，不能延續累積到一半的舊值（2）")
}

func TestOptionalStreak_CropDisambiguationPending_DoesNotStarveQuoteRequiredOptionalSlots(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(context.Background(), 1234)
	require.NoError(t, err)
	item, city, unit := "小白菜", "桃園市", "KG"
	seedRev, err := repo.AppendRevision(context.Background(), repository.AppendInput{
		RequestID: req.ID, BuyerID: 1234,
		Slots:           domain.Slots{Item: &item, DeliveryCity: &city, Unit: &unit},
		Source:          "LLM_EXTRACT",
		ClientRequestID: "cr-cropstreak-seed",
		PayloadJSON:     mustPayload(t, turnPayload{OptionalStreak: 2}),
		RequestStatus:   "REQUIREMENT_READY",
	})
	require.NoError(t, err)

	store := newFakeRunStore()
	seededSlots := domain.Slots{Item: &item, DeliveryCity: &city, Unit: &unit}
	cropCandidates := []domain.CropDisambiguationOption{
		{Label: "有機小白菜", CropCode: ptr("CROP-X")},
		{Label: "小白菜（一般）", CropCode: ptr("CROP-Y")},
	}
	lastRevisionID := seedRev.ID
	for i := 0; i < 3; i++ {
		clientRequestID := fmt.Sprintf("cr-cropstreak-idle-%d", i)
		out, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
			BuyerID: 1234, RequestID: req.ID, Message: "嗯…我再想想", ClientRequestID: clientRequestID,
		}, GraphControllerDeps{
			RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
			Runner: newTerminalCompletionRunner(t, graphTurnProgress{
				Patch: domain.SlotPatch{}, NextSlots: seededSlots,
				CropPending: true, CropCandidates: cropCandidates,
			}, nil),
		}, TurnStreamHooks{})
		require.NoError(t, err)
		turn := out.Result
		require.NotNil(t, turn.Question)
		require.Equal(t, domain.SlotCropDisambiguation, turn.Question.SlotKey,
			"品項身份未定，消歧問句必須持續蓋過其他問句")

		stored, ok := repo.patches[fakePatchKey{1234, clientRequestID}]
		require.True(t, ok)
		var payload turnPayload
		require.NoError(t, json.Unmarshal(stored.PayloadJSON, &payload))
		require.Equal(t, 2, payload.OptionalStreak,
			"卡在消歧卡片的每一輪都不該把 streak 往上疊，必須原樣延續進入這段卡住期之前的值（2）")
		lastRevisionID = turn.RevisionID
	}

	_ = lastRevisionID
	out, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, RequestID: req.ID, Message: "就選有機小白菜", ClientRequestID: "cr-cropstreak-resolve",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
		Runner: newTerminalCompletionRunner(t, graphTurnProgress{
			Patch: domain.SlotPatch{}, NextSlots: seededSlots, CropPending: false,
		}, nil),
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.NotNil(t, out.Result.Question,
		"消歧解開後，quantity 這個報價必填的選填欄位必須還問得到——卡在消歧卡片的那 3 輪不該把 streak 推過 cap")
	require.Equal(t, domain.SlotQuantity, out.Result.Question.SlotKey)
}
