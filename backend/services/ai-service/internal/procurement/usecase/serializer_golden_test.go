package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/stretchr/testify/require"
)

func TestTurnAnswerRewind_SameSerializerShape(t *testing.T) {
	repo := newFakeRepo()
	checker := fakePricePreviewChecker{hasPreview: false}

	patch := readyTurnPatch()
	out, err := ClaimAndDispatchTurnViaGraph(context.Background(), TurnInput{
		BuyerID: 1234, Message: "寶島梨禮盒 50 盒，單盒800，兩週後送桃園", ClientRequestID: "cr-golden-1",
	}, GraphControllerDeps{
		RunStore:      newFakeRunStore(),
		RevisionStore: repo,
		Dispatcher:    NewRunDispatcher(),
		Runner:        newTerminalCompletionRunner(t, graphTurnProgress{Patch: patch, NextSlots: domain.Slots(patch)}, checker),
	}, TurnStreamHooks{})
	require.NoError(t, err)
	turnResult := out.Result
	require.Empty(t, turnResult.MissingRequired, "readyTurnPatch 應該一次把必填欄位填滿")
	require.Equal(t, "REVIEW_PRICE_PREVIEW", turnResult.NextAction, "READY 且尚無 price preview，turn 應該附上提示")

	answerUC := NewAnswerUseCase(repo, nil, checker, nil, nil, nil)
	answerResult, err := answerUC.composeAnswerViaGraph(context.Background(), AnswerInput{
		BuyerID: 1234, RequestID: turnResult.RequestID, SlotKey: domain.SlotDeliveryCity,
		OptionID: "city-newtaipei", ExpectedRevisionID: turnResult.RevisionID, ClientRequestID: "cr-golden-2",
	})
	require.NoError(t, err)
	require.Empty(t, answerResult.MissingRequired, "重新回答已填過的必填欄位，仍應維持 READY")
	require.Equal(t, "REVIEW_PRICE_PREVIEW", answerResult.NextAction, "answer 與 turn 必須附上同一種提示")

	rewindUC := NewRewindUseCase(repo, nil, checker, nil, nil, nil)
	rewindResult, err := rewindUC.composeRewindViaGraph(context.Background(), RewindInput{
		BuyerID: 1234, RequestID: turnResult.RequestID, SlotKey: domain.SlotDeliveryCity, ClientRequestID: "cr-golden-3",
	})
	require.NoError(t, err)
	require.Empty(t, rewindResult.MissingRequired, "rewind 回到 turn 那筆版本，delivery_city 回到「桃園市」仍非 nil，應維持 READY")
	require.Equal(t, "REVIEW_PRICE_PREVIEW", rewindResult.NextAction, "rewind 與 turn 必須附上同一種提示")

	turnKeys := jsonKeys(t, turnResult)
	answerKeys := jsonKeys(t, answerResult)
	rewindKeys := jsonKeys(t, rewindResult)

	require.ElementsMatch(t, turnKeys, answerKeys, "turns/answers 頂層鍵集合必須一致")
	require.ElementsMatch(t, turnKeys, rewindKeys, "turns/rewinds 頂層鍵集合必須一致")
	require.Contains(t, turnKeys, "next_action", "next_action 應該是三個端點共用鍵集合的一部分")
}

func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
