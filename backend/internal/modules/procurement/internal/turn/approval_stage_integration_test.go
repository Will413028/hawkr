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

// runTurn 送出一輪、以替身模型回 patch 並執行，回這一輪助理的回覆。
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

// approve 為現行版本寫一份發布核准（預設 PENDING、nowAt 起算 ttl 後到期）。
func (f *fixture) approve(ttl time.Duration, state string) uuid.UUID {
	f.t.Helper()
	ctx := context.Background()
	q := store.New(f.d.Conn(ctx))
	active, err := q.GetActiveRevision(ctx, store.GetActiveRevisionParams{RequestID: f.request, BuyerID: ptr(buyer)})
	if err != nil {
		f.t.Fatal(err)
	}
	id := uuid.NewV7()
	if n, err := q.InsertApproval(ctx, store.InsertApprovalParams{
		ID: id, BuyerID: ptr(buyer), RequestID: f.request, RequestRevisionID: active.ID, RangeLowerAmount: 100, RangeUpperAmount: 200, AudienceMode: "WISH_POOL",
		AudiencePolicyID: ptr("P"), AudiencePolicyVersion: ptr(int32(1)), SchemaVersion: "v1", ClientRequestID: uuid.NewV7(), Now: now, ExpiresAt: now.Add(ttl),
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
		wantReply string // 回覆要包含的句子；空字串表示不得包含任何核准階段的句子
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
	f.runTurn(`{"packaging":"箱"}`) // 內容改了：新的一版，核准還在舊版上
	reply := f.runTurn(`{}`)       // 這一輪沒有改內容，但核准對的不是現行版本
	if !strings.Contains(reply, text.ReplyReconfirmWarning(uitext.ZhTW).Text) {
		t.Errorf("核准在舊版上，要請買家重新確認：%q", reply)
	}
}
