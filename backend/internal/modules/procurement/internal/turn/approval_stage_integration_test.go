//go:build integration

package turn_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"uuid"

	adkmodel "google.golang.org/adk/v2/model"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func (f *fixture) runTurn(patch string) string {
	f.t.Helper()
	id := f.submit("m", uitext.ZhTW)
	if err := f.runWith(context.Background(), id, f.executor(llmExtractor(&scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(patch)}}))); err != nil {
		f.t.Fatal(err)
	}
	last, err := store.New(f.d.Conn(context.Background())).GetLatestTurnByRequest(context.Background(), store.GetLatestTurnByRequestParams{RequestID: f.request, BuyerID: buyer})
	if err != nil {
		f.t.Fatal(err)
	}
	return last.AssistantReply
}

func (f *fixture) approve(ttl time.Duration, state string) uuid.UUID {
	f.t.Helper()
	ctx := context.Background()
	q := store.New(f.d.Conn(ctx))
	active, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: f.request, BuyerID: ptr(buyer)})
	if err != nil {
		f.t.Fatal(err)
	}
	id := uuid.NewV7()
	setID := uuid.NewV7()
	if err := q.InsertCandidateSet(ctx, store.InsertCandidateSetParams{ID: setID, RequestID: f.request, RequestRevisionID: active.ID, BuyerID: buyer, Now: now}); err != nil {
		f.t.Fatal(err)
	}
	if n, err := q.InsertApproval(ctx, store.InsertApprovalParams{
		ID: id, BuyerID: ptr(buyer), RequestID: f.request, RequestRevisionID: active.ID, RangeLowerAmount: 100, RangeUpperAmount: 200, CandidateSetID: setID,
		SchemaVersion: "v1", ClientRequestID: uuid.NewV7(), Now: now, ExpiresAt: now.Add(ttl),
	}); err != nil || n != 1 {
		f.t.Fatalf("InsertApproval n=%d err=%v", n, err)
	}
	if state != "PENDING" {
		if _, err := f.d.Pool().Exec(ctx, `UPDATE procurement.rfq_publication_approvals SET state = $2 WHERE id = $1`, id, state); err != nil {
			f.t.Fatal(err)
		}
	}
	return id
}

// A9-D35：planner 的詢價階段讀「現行版本上未過期的待執行核准」。
func TestPlannerStageFollowsTheUnexpiredPendingApproval(t *testing.T) {
	approved := text.ReplyApprovalComplete(uitext.ZhTW).Text
	reconfirm := text.ReplyReconfirmWarning(uitext.ZhTW).Text
	for _, tc := range []struct {
		name      string
		ttl       time.Duration
		state     string
		patch     string
		wantReply string
	}{
		{"pending and unexpired on the current revision, nothing changed", time.Hour, "PENDING", `{}`, approved},
		{"pending but expiring exactly now is expired", 0, "PENDING", `{}`, ""},
		{"expired", -time.Hour, "PENDING", `{}`, ""},
		{"executing is no longer a pending approval", time.Hour, "EXECUTING", `{}`, ""},
		{"succeeded is no longer a pending approval", time.Hour, "SUCCEEDED", `{}`, ""},
		{"content changed this turn makes the approval stale", time.Hour, "PENDING", `{"packaging":"箱"}`, reconfirm},
	} {
		f := newFixture(t)
		f.runTurn(ok)
		f.approve(tc.ttl, tc.state)
		reply := f.runTurn(tc.patch)
		if tc.wantReply != "" {
			if !strings.Contains(reply, tc.wantReply) {
				t.Errorf("%s：回覆 %q 要包含 %q", tc.name, reply, tc.wantReply)
			}
			continue
		}
		if strings.Contains(reply, approved) || strings.Contains(reply, reconfirm) {
			t.Errorf("%s：沒有有效的核准，回覆不得是核准階段：%q", tc.name, reply)
		}
	}
}

func TestPlannerStageIsStaleWhenTheApprovalIsOnAnEarlierRevision(t *testing.T) {
	f := newFixture(t)
	f.runTurn(ok)
	f.approve(time.Hour, "PENDING")
	f.runTurn(`{"packaging":"箱"}`)
	reply := f.runTurn(`{}`)
	if !strings.Contains(reply, text.ReplyReconfirmWarning(uitext.ZhTW).Text) {
		t.Errorf("核准在舊版上，要請買家重新確認：%q", reply)
	}
}
