package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

type fakeConversationStore struct {
	conv    repository.Conversation
	status  string
	convErr error

	patches    []gen.ProcurementAnswerPatch
	patchesErr error

	getForMessagesCalls []struct {
		conversationID int64
		buyerID        identity.BuyerID
	}
}

func (f *fakeConversationStore) GetConversationForMessages(_ context.Context, conversationID int64, buyerID identity.BuyerID) (repository.Conversation, string, error) {
	f.getForMessagesCalls = append(f.getForMessagesCalls, struct {
		conversationID int64
		buyerID        identity.BuyerID
	}{conversationID, buyerID})
	if f.convErr != nil {
		return repository.Conversation{}, "", f.convErr
	}
	return f.conv, f.status, nil
}

func (f *fakeConversationStore) ListAnswerPatchesByRequestID(context.Context, int64) ([]gen.ProcurementAnswerPatch, error) {
	if f.patchesErr != nil {
		return nil, f.patchesErr
	}
	return f.patches, nil
}

func (f *fakeConversationStore) ListConversations(context.Context, identity.BuyerID, *repository.ConversationCursor, int) ([]repository.Conversation, error) {
	panic("ListConversations not used by GetRuntimeSnapshot")
}

func (f *fakeConversationStore) RenameConversation(context.Context, int64, identity.BuyerID, string, string) (bool, bool, repository.RenameRecord, error) {
	panic("RenameConversation not used by GetRuntimeSnapshot")
}

func (f *fakeConversationStore) DeleteConversation(context.Context, int64, identity.BuyerID, string) (bool, bool, error) {
	panic("DeleteConversation not used by GetRuntimeSnapshot")
}

func newRuntimeSnapshotTestServer(convStore usecase.ConversationStore, runStore RunStore) *AIProcurementGrpcServer {
	return NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil,
		usecase.NewConversationUseCase(convStore), nil, nil, nil, nil,
		DurableRunDeps{}, runStore, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func TestGetRuntimeSnapshot_AssemblesAllFiveSections(t *testing.T) {
	createdAt := time.Date(2026, 9, 13, 1, 2, 3, 0, time.UTC)
	convStore := &fakeConversationStore{
		conv:   repository.Conversation{ID: 42, RequestID: 7, BuyerID: 1234, Title: "高麗菜 200 箱"},
		status: "CLARIFYING",
		patches: []gen.ProcurementAnswerPatch{{
			ID: 900, RequestID: 7,
			PayloadJson: []byte(`{"message":"我要高麗菜","reply":"好的"}`),
			CreatedAt:   ts(createdAt),
		}},
	}
	runStore := &fakeRunStore{
		runsByConversation: []repository.Run{
			{ID: 901, RunType: domain.RunTypeTurn, Status: domain.RunStatusWaiting(), LastEventSeq: 5, MinAvailableSeq: 1},
			{ID: 900, RunType: domain.RunTypeTurn, Status: domain.RunStatusSucceeded(), LastEventSeq: 3, MinAvailableSeq: 1},
		},
		hasOpenInterruptForRun: true,
		openInterruptForRun: repository.Interrupt{
			ID: "int-1", InterruptKind: "CLARIFICATION", PayloadJSON: []byte(`{}`),
		},
		artifactsByConversation: []repository.RunArtifact{{
			ArtifactID: 555, RunID: 900, ArtifactType: string(domain.ArtifactPricingPreview),
			Version: 2, Status: "READY", PublicDataJSON: []byte(`{"p50":"38"}`),
			Checksum: "abc", DomainRefType: "CANDIDATE_SET", DomainRefID: "77",
			AsOf: createdAt, CreatedAt: createdAt,
		}},
	}
	s := newRuntimeSnapshotTestServer(convStore, runStore)

	got, err := s.GetRuntimeSnapshot(buyerActorContext("1234"), &aipb.GetRuntimeSnapshotRequest{ConversationId: "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetConversationId() != "42" || got.GetRequestId() != "7" ||
		got.GetStatus() != "CLARIFYING" || got.GetTitle() != "高麗菜 200 箱" {
		t.Fatalf("header = %q/%q/%q/%q", got.GetConversationId(), got.GetRequestId(), got.GetStatus(), got.GetTitle())
	}

	if len(got.GetMessages()) != 2 {
		t.Fatalf("messages = %d, want 2", len(got.GetMessages()))
	}
	if got.GetMessages()[0].GetRole() != "buyer" || got.GetMessages()[0].GetText() != "我要高麗菜" {
		t.Fatalf("messages[0] = %+v", got.GetMessages()[0])
	}

	if len(got.GetRuns()) != 2 {
		t.Fatalf("runs = %d, want 2", len(got.GetRuns()))
	}
	if got.GetRuns()[0].GetTerminal() {
		t.Fatalf("runs[0] (WAITING_FOR_INPUT) terminal = true, want false")
	}
	if !got.GetRuns()[1].GetTerminal() {
		t.Fatalf("runs[1] (SUCCEEDED) terminal = false, want true")
	}
	if got.GetRuns()[0].GetLastSeqId() != "5" || got.GetRuns()[0].GetMinAvailableSeqId() != "1" {
		t.Fatalf("runs[0] replay boundary = %q/%q", got.GetRuns()[0].GetLastSeqId(), got.GetRuns()[0].GetMinAvailableSeqId())
	}

	if got.GetActiveRunId() != "901" {
		t.Fatalf("activeRunId = %q, want 901", got.GetActiveRunId())
	}
	if oc := got.GetOpenConfirmation(); oc == nil || oc.GetInterruptId() != "int-1" || oc.GetConfirmationKind() != "CLARIFICATION" {
		t.Fatalf("openConfirmation = %+v", oc)
	}

	if len(got.GetArtifacts()) != 1 {
		t.Fatalf("artifacts = %d, want 1", len(got.GetArtifacts()))
	}
	a := got.GetArtifacts()[0]
	if a.GetArtifactId() != "555" || a.GetRunId() != "900" || a.GetVersion() != "2" ||
		a.GetDataJson() != `{"p50":"38"}` || a.GetDomainRefId() != "77" {
		t.Fatalf("artifacts[0] = %+v", a)
	}
	if a.GetAsOf() != "2026-09-13T01:02:03Z" || a.GetCreatedAt() != "2026-09-13T01:02:03Z" {
		t.Fatalf("artifact timestamps = %q/%q", a.GetAsOf(), a.GetCreatedAt())
	}
}

func TestGetRuntimeSnapshot_PassesBuyerIDToEveryQuery(t *testing.T) {
	convStore := &fakeConversationStore{conv: repository.Conversation{ID: 42, RequestID: 7}, status: "CLARIFYING"}
	runStore := &fakeRunStore{}
	s := newRuntimeSnapshotTestServer(convStore, runStore)

	if _, err := s.GetRuntimeSnapshot(buyerActorContext("1234"), &aipb.GetRuntimeSnapshotRequest{ConversationId: "42"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(convStore.getForMessagesCalls) != 1 ||
		convStore.getForMessagesCalls[0].conversationID != 42 ||
		convStore.getForMessagesCalls[0].buyerID != identity.BuyerID(1234) {
		t.Fatalf("GetConversationForMessages calls = %+v", convStore.getForMessagesCalls)
	}
	if len(runStore.listRunsCalls) != 1 ||
		runStore.listRunsCalls[0].conversationID != 42 || runStore.listRunsCalls[0].buyerID != identity.BuyerID(1234) {
		t.Fatalf("ListRunsByConversation calls = %+v", runStore.listRunsCalls)
	}
	if len(runStore.listArtifactsCalls) != 1 ||
		runStore.listArtifactsCalls[0].conversationID != 42 || runStore.listArtifactsCalls[0].buyerID != identity.BuyerID(1234) {
		t.Fatalf("ListLatestArtifactsByConversation calls = %+v", runStore.listArtifactsCalls)
	}
}

func TestGetRuntimeSnapshot_OtherBuyersConversationIs404(t *testing.T) {
	convStore := &fakeConversationStore{convErr: repository.ErrNotFound}
	runStore := &fakeRunStore{}
	s := newRuntimeSnapshotTestServer(convStore, runStore)

	_, err := s.GetRuntimeSnapshot(buyerActorContext("1234"), &aipb.GetRuntimeSnapshotRequest{ConversationId: "42"})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("code = %v, want NotFound (err=%v)", got, err)
	}
	if len(runStore.listRunsCalls) != 0 || len(runStore.listArtifactsCalls) != 0 {
		t.Fatalf("queried runs/artifacts after ownership failure: %+v %+v", runStore.listRunsCalls, runStore.listArtifactsCalls)
	}
}

func TestGetRuntimeSnapshot_InvalidConversationIDIsInvalidArgument(t *testing.T) {
	s := newRuntimeSnapshotTestServer(&fakeConversationStore{}, &fakeRunStore{})

	_, err := s.GetRuntimeSnapshot(buyerActorContext("1234"), &aipb.GetRuntimeSnapshotRequest{ConversationId: "not-a-number"})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument (err=%v)", got, err)
	}
}

func TestGetRuntimeSnapshot_ArtifactQueryFailureIsNotSwallowed(t *testing.T) {
	convStore := &fakeConversationStore{conv: repository.Conversation{ID: 42, RequestID: 7}, status: "CLARIFYING"}
	runStore := &fakeRunStore{listArtifactsErr: errors.New("boom")}
	s := newRuntimeSnapshotTestServer(convStore, runStore)

	if _, err := s.GetRuntimeSnapshot(buyerActorContext("1234"), &aipb.GetRuntimeSnapshotRequest{ConversationId: "42"}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetRuntimeSnapshot_NoActiveRunSkipsInterruptQuery(t *testing.T) {
	convStore := &fakeConversationStore{conv: repository.Conversation{ID: 42, RequestID: 7}, status: "READY"}
	runStore := &fakeRunStore{runsByConversation: []repository.Run{
		{ID: 900, RunType: domain.RunTypeTurn, Status: domain.RunStatusSucceeded(), LastEventSeq: 3, MinAvailableSeq: 1},
	}}
	s := newRuntimeSnapshotTestServer(convStore, runStore)

	got, err := s.GetRuntimeSnapshot(buyerActorContext("1234"), &aipb.GetRuntimeSnapshotRequest{ConversationId: "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetActiveRunId() != "" || got.GetOpenConfirmation() != nil {
		t.Fatalf("activeRunId = %q, openConfirmation = %+v", got.GetActiveRunId(), got.GetOpenConfirmation())
	}
	if len(runStore.getOpenInterruptForRunCalls) != 0 {
		t.Fatalf("queried interrupt with no active run: %+v", runStore.getOpenInterruptForRunCalls)
	}
	if !got.GetRuns()[0].GetTerminal() {
		t.Fatal("SUCCEEDED run terminal = false, want true")
	}
}

func TestGetRuntimeSnapshot_CandidateSearchRunNeverBecomesActive(t *testing.T) {
	convStore := &fakeConversationStore{conv: repository.Conversation{ID: 42, RequestID: 7}, status: "CLARIFYING"}
	runStore := &fakeRunStore{runsByConversation: []repository.Run{
		{ID: 910, RunType: domain.RunTypeCandidateSearch, Status: domain.RunStatusQueued(), LastEventSeq: 1, MinAvailableSeq: 1},
	}}
	s := newRuntimeSnapshotTestServer(convStore, runStore)

	got, err := s.GetRuntimeSnapshot(buyerActorContext("1234"), &aipb.GetRuntimeSnapshotRequest{ConversationId: "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetActiveRunId() != "" {
		t.Fatalf("activeRunId = %q, want empty", got.GetActiveRunId())
	}
	if len(got.GetRuns()) != 1 {
		t.Fatalf("runs = %d, want the candidate search still listed", len(got.GetRuns()))
	}
}

// TestGetRuntimeSnapshot_RunVersionIsTheCancelCASToken 守的是「停止鈕在契約上
// 做不做得出來」。cancel 的 expected_run_version 是必填 CAS token，而在這個欄位
// 存在之前，瀏覽器沒有任何讀路徑拿得到它——version 只出現在 cancel 自己的回應裡。
//
// 鑑別力來自三件事一起驗，少一件都會放過一種壞實作：
//   - 兩個 run 給**不同**的 version：寫死常數或把第一筆的值套用到全部的實作會紅。
//   - 值逐一對回各自的 run：只驗「非空」的話，填錯 run 的版本照樣過。
//   - 明確排除 "0"：proto 的 string 零值與「版本尚未知」長得一樣，而
//     procurement_agent_runs.version 是 NOT NULL DEFAULT 1，真實資料永遠 ≥1。
//     漏填欄位時得到的正是 ""，補成 "0" 則會讓 CAS 必然落空——兩種都必須紅。
func TestGetRuntimeSnapshot_RunVersionIsTheCancelCASToken(t *testing.T) {
	convStore := &fakeConversationStore{conv: repository.Conversation{ID: 42, RequestID: 7}, status: "CLARIFYING"}
	runStore := &fakeRunStore{runsByConversation: []repository.Run{
		{ID: 901, RunType: domain.RunTypeTurn, Status: domain.RunStatusRunning(), LastEventSeq: 5, MinAvailableSeq: 1, Version: 7},
		{ID: 900, RunType: domain.RunTypeTurn, Status: domain.RunStatusSucceeded(), LastEventSeq: 3, MinAvailableSeq: 1, Version: 3},
	}}
	s := newRuntimeSnapshotTestServer(convStore, runStore)

	got, err := s.GetRuntimeSnapshot(buyerActorContext("1234"), &aipb.GetRuntimeSnapshotRequest{ConversationId: "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.GetRuns()) != 2 {
		t.Fatalf("runs = %d, want 2", len(got.GetRuns()))
	}
	for i, want := range []struct {
		runID   string
		version string
	}{{"901", "7"}, {"900", "3"}} {
		run := got.GetRuns()[i]
		if run.GetRunId() != want.runID {
			t.Fatalf("runs[%d] runId = %q, want %q", i, run.GetRunId(), want.runID)
		}
		if run.GetVersion() != want.version {
			t.Fatalf("runs[%d] (run %s) version = %q, want %q — cancel 的 CAS token 必須逐一對回自己的 run",
				i, want.runID, run.GetVersion(), want.version)
		}
		if run.GetVersion() == "" || run.GetVersion() == "0" {
			t.Fatalf("runs[%d] version = %q：空字串＝欄位沒填，\"0\"＝CAS 必然落空，兩者都讓停止鈕做不出來",
				i, run.GetVersion())
		}
	}
}
