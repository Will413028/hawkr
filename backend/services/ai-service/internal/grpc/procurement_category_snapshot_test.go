package grpc

import (
	"encoding/json"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/stretchr/testify/require"
)

func TestGetSnapshot_RestoresCategoryStageFromOpenInterrupt(t *testing.T) {
	for _, stage := range []string{domain.CropSelectionGroup, domain.CropSelectionItem, domain.CropSelectionDescribe} {
		t.Run(stage, func(t *testing.T) {
			candidates := []domain.CropDisambiguationOption{{CategoryID: "24", CategoryLevel: 3, Label: "五花肉"}}
			if stage == domain.CropSelectionDescribe {
				candidates = nil
			}
			payload, err := json.Marshal(usecase.CropDisambiguationInterruptPayload{
				Kind: string(domain.SlotCropDisambiguation), Candidates: candidates,
				Selection:           &domain.CropSelection{Stage: stage, ParentCategoryID: "8", ParentLabel: "豬肉", CategoryID: "24", Label: "五花肉"},
				WorkflowInterruptID: "internal-workflow-stage-id",
			})
			require.NoError(t, err)
			store := &fakeRunStore{
				conversationIDForRequest: 42,
				runsByConversation:       []repository.Run{{ID: 901, RunType: domain.RunTypeClarificationResume, Status: domain.RunStatusWaiting()}},
				hasOpenInterruptForRun:   true,
				openInterruptForRun:      repository.Interrupt{ID: "ledger-stage-id", RunID: 901, InterruptKind: "CLARIFICATION", Status: "PENDING", PayloadJSON: payload},
			}
			s := newSnapshotTestServer(emptySlotsSnapshot(1, 2), store)
			got, err := s.GetSnapshot(buyerActorContext("1234"), &aipb.GetSnapshotRequest{RequestId: "1"})
			require.NoError(t, err)
			require.Equal(t, "ledger-stage-id", got.GetOpenConfirmation().GetInterruptId())
			q := got.GetOpenConfirmation().GetQuestion()
			require.NotNil(t, q)
			require.Equal(t, string(domain.SlotCropDisambiguation), q.GetSlotKey())
			require.Equal(t, stage != domain.CropSelectionDescribe, q.GetAllowFreeText())
			if stage != domain.CropSelectionDescribe {
				require.Equal(t, "category-24", q.GetOptions()[0].GetOptionId())
			}
		})
	}
}
