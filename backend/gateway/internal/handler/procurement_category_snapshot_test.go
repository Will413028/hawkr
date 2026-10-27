package handler

import (
	"encoding/json"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/stretchr/testify/require"
)

func TestProcurementSnapshot_PreservesAuthoritativeCategoryQuestion(t *testing.T) {
	dto := snapshotResponseFromProto(&pb.SnapshotResponse{OpenConfirmation: &pb.AgentRunOpenConfirmation{
		InterruptId: "ledger-stage-id", ConfirmationKind: "CLARIFICATION",
		Question: &pb.ClarifyQuestion{SlotKey: "crop_disambiguation", Prompt: "請選豬肉細項", AllowFreeText: true, Source: "FIXED", Options: []*pb.QuestionOption{{OptionId: "category-24", Label: "五花肉", Value: "五花肉"}}},
	}})
	raw, err := json.Marshal(dto)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	oc := decoded["openConfirmation"].(map[string]any)
	q := oc["question"].(map[string]any)
	require.Equal(t, true, q["allowFreeText"])
	require.Equal(t, "category-24", q["options"].([]any)[0].(map[string]any)["optionId"])
	require.NotContains(t, string(raw), "workflowInterruptId")
}
