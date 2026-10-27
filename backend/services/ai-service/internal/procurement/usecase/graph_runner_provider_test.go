package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/stretchr/testify/require"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// TestNewProcurementGraphRunner_BuildsRunner proves the provider wires
// NewProcurementGraph into a real *runner.Runner without error, given
// fakes for its two usecase-level dependencies (SlotExtractor/cropidentity.Resolver
// already have package-level fakes from other tests, per CLAUDE.md §15 —
// this reuses fakeExtractor/testkit.FakeResolver, not new doubles) and a real
// in-memory session.Service (session.InMemoryService() — this test is not
// exercising which session.Service production actually injects, just that
// NewProcurementGraphRunner's own wiring produces a valid *runner.Runner
// for whatever session.Service value fx hands it).
func TestNewProcurementGraphRunner_BuildsRunner(t *testing.T) {
	r, _, err := NewProcurementGraphRunner(
		fakeExtractor{}, &testkit.FakeResolver[cropidentity.DisambiguationResult]{}, seqIDs(5000), session.InMemoryService(), nil,
		&fakeStepNarrator{text: "narrated"}, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, r)
}

// TestNewProcurementGraphRunner_ResumeRunnerRejectsMissingSession 釘住兩個 runner
// 的分工：新的一輪可以建立 session，resume 不行。
//
// AutoCreateSession 是 runner 的私有欄位，所以這裡驗的是行為而不是欄位：對一個
// 不存在的 session，resume runner 必須回錯，turn runner 必須照常跑起來。
func TestNewProcurementGraphRunner_ResumeRunnerRejectsMissingSession(t *testing.T) {
	ctx := t.Context()
	svc := session.InMemoryService()
	turnRunner, resumeRunner, err := NewProcurementGraphRunner(
		fakeExtractor{patch: domain.SlotPatch{Item: item("小白菜")}},
		&testkit.FakeResolver[cropidentity.DisambiguationResult]{},
		seqIDs(9300), svc, nil, nil, nil,
	)
	require.NoError(t, err)

	const userID, sessionID = "u-missing", "s-missing"
	var resumeErr error
	for _, err := range resumeRunner.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要小白菜"}}},
		agent.RunConfig{}) {
		if err != nil {
			resumeErr = err
		}
	}
	require.Error(t, resumeErr, "resume 不得對不存在的 session 靜默 AutoCreate")

	// 新的一輪：graph 本身可能因為缺 seed state 而失敗，那不是這裡要驗的；
	// 要驗的是 session 真的被建出來了——也就是 AutoCreateSession 仍然開著。
	for _, err := range turnRunner.Run(ctx, userID, sessionID,
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "我要小白菜"}}},
		agent.RunConfig{}, runner.WithStateDelta(map[string]any{})) {
		_ = err
	}
	_, getErr := svc.Get(ctx, &session.GetRequest{
		AppName: string(ProcurementAppName), UserID: userID, SessionID: sessionID,
	})
	require.NoError(t, getErr, "新的一輪仍然要能建立 session")
}
