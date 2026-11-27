package delegation

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/config"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	commonpb "github.com/Will413028/hawkr/go-pkg/proto/common"
)

// 每一列稽核 intent 都必須抵達終態。留在 PENDING 的列不是「還沒好」——三步序跑完之後
// 沒有人會再回來看它，而 delegation_audit_pending_total 的告警窗口是 24 小時，一列卡住
// 就響滿一天再自行 resolve，於是「真的需要人看的 PENDING」（domain write 成立但 confirm
// 沒回來）與「永遠不會關的 PENDING」在告警上長得一模一樣，鑑別力歸零。
//
// 本檔守三條離開路徑，它們的共同點是**寫入已經成立、只有稽核沒關**，所以全部無聲：
// 回應是成功的、log 只有一行 Warn、指標照常。
//
// ⚠️ 這裡刻意直接驅動 Orchestrator 而不是打 HTTP：要斷言的是「Gateway 送了什麼給
// delegation 模組」，而三條路徑在 HTTP 層的回應全部正常，從外面看不出差別。

// closureAuditStub 記下送出的 intent／confirm，並可指定 intent 回 replayed 與既有 result。
type closureAuditStub struct {
	replayed      bool
	existingState string

	intents  []*accountpb.RecordAuditIntentRequest
	confirms []*accountpb.ConfirmAuditResultRequest
	// confirmCtxErr 記錄 confirm 當下 ctx 的狀態。detached 之後它必須是 nil，即使
	// 呼叫端的 ctx 早就被取消。
	confirmCtxErr error
}

func (s *closureAuditStub) RecordAuditIntent(_ context.Context, in *accountpb.RecordAuditIntentRequest, _ int) (*accountpb.RecordAuditIntentResponse, error) {
	s.intents = append(s.intents, in)
	return &accountpb.RecordAuditIntentResponse{
		Data: &accountpb.RecordAuditIntentData{
			AuditLogId: "9001",
			Replayed:   s.replayed,
			Result:     s.existingState,
		},
	}, nil
}

func (s *closureAuditStub) ConfirmAuditResult(ctx context.Context, in *accountpb.ConfirmAuditResultRequest, _ int) (*accountpb.ConfirmAuditResultResponse, error) {
	s.confirms = append(s.confirms, in)
	s.confirmCtxErr = ctx.Err()
	return &accountpb.ConfirmAuditResultResponse{}, nil
}

func closureOrchestrator(stub *closureAuditStub) *Orchestrator {
	return &Orchestrator{Audit: stub, Cfg: config.DelegationConfig{}}
}

func closureSession() *accountpb.OperationSessionSummary {
	return &accountpb.OperationSessionSummary{SessionId: "5001"}
}

// TestOrchestrate_ReplayedPendingRowGetsClosed：重放時若既有列還是 PENDING，必須補
// confirm。
//
// 這條是三條裡最隱蔽的：重放本來就是「上一次已經寫過了」，所以早退是對的；但**上一次
// 之所以留下 PENDING，正是因為它的 confirm 沒送到**，而重試——那個本該治好它的動作——
// 走的偏偏就是這條跳過 confirm 的路。於是每一次重試都讓那列繼續留在 PENDING。
func TestOrchestrate_ReplayedPendingRowGetsClosed(t *testing.T) {
	stub := &closureAuditStub{replayed: true, existingState: "PENDING"}
	o := closureOrchestrator(stub)

	writeCalled := false
	_, denial := o.Orchestrate(context.Background(), Spec{
		Session:      closureSession(),
		Action:       "PRODUCT_UPDATE",
		ResourceType: "product",
		ResourceID:   "7001",
		ReadBefore: func(context.Context, *commonpb.DelegationContext) (*Resource, *Denial) {
			return &Resource{ID: "7001", Name: "測試商品", Version: "3", Fields: map[string]any{"name": "測試商品"}}, nil
		},
		Write: func(context.Context, *commonpb.DelegationContext) (*Resource, *Denial) {
			writeCalled = true
			return &Resource{ID: "7001", Version: "4"}, nil
		},
		SnapshotFields: []string{"name"},
	})
	if denial != nil {
		t.Fatalf("重放不該被拒絕: %+v", denial)
	}
	if writeCalled {
		t.Fatal("重放必須跳過 domain write，否則同一個變更被套用兩次")
	}
	if len(stub.confirms) != 1 {
		t.Fatalf("既有列是 PENDING，必須補一次 confirm；實際 %d 次", len(stub.confirms))
	}
	got := stub.confirms[0]
	if got.GetResult() != "SUCCEEDED" {
		t.Fatalf("重放代表寫入已成立，補記必須是 SUCCEEDED；實際 %q", got.GetResult())
	}
	if got.GetAuditLogId() != "9001" {
		t.Fatalf("補記必須指向既有那一列；實際 %q", got.GetAuditLogId())
	}
	if got.GetAfterSnapshotJson() == "" {
		t.Fatal("補記要帶 after 快照——這次讀到的就是那次寫入之後的狀態，留空等於放棄舉證素材")
	}
	if len(got.GetChangedFields()) != 0 {
		t.Fatalf("寫入前的快照已經沒了，changed_fields 只能留空；實際 %v", got.GetChangedFields())
	}
}

// TestOrchestrate_ReplayedTerminalRowStaysUntouched：既有列已經是終態時，不得再送
// confirm。
//
// 這是上一條的反向護欄。confirm 的述詞是 `WHERE result='PENDING'` 先到先贏，對已經
// SUCCEEDED 的列再送一次會被靜默丟棄（delegation 模組把 ErrNotFound 吞掉回 nil），
// 所以「多送一次沒差」——差在多副本：若另一個 pod 正要寫 FAILED，多送的這次會贏，
// 把稽核記成成功。不送才是對的。
func TestOrchestrate_ReplayedTerminalRowStaysUntouched(t *testing.T) {
	for _, state := range []string{"SUCCEEDED", "FAILED"} {
		t.Run(state, func(t *testing.T) {
			stub := &closureAuditStub{replayed: true, existingState: state}
			o := closureOrchestrator(stub)
			_, denial := o.Orchestrate(context.Background(), Spec{
				Session:      closureSession(),
				Action:       "PRODUCT_UPDATE",
				ResourceType: "product",
				ResourceID:   "7001",
				ReadBefore: func(context.Context, *commonpb.DelegationContext) (*Resource, *Denial) {
					return &Resource{ID: "7001", Version: "3"}, nil
				},
				Write: func(context.Context, *commonpb.DelegationContext) (*Resource, *Denial) {
					t.Fatal("重放不得重跑 domain write")
					return nil, nil
				},
				SnapshotFields: []string{"name"},
			})
			if denial != nil {
				t.Fatalf("不該被拒絕: %+v", denial)
			}
			if len(stub.confirms) != 0 {
				t.Fatalf("既有列已是 %s，不得再送 confirm；實際送了 %d 次", state, len(stub.confirms))
			}
		})
	}
}

// TestOrchestrate_ConfirmSurvivesCallerCancellation：呼叫端斷線不得決定稽核列關不關。
//
// 到達 confirm 的那一刻，domain write 已經落地了。若沿用 handler 的
// `c.Request.Context()`，管理員早一毫秒關掉分頁就會讓這次 RPC 立刻 context canceled ——
// 而那正是「從外面最看不出結果」的一群請求：寫入是持久的、回應早就送出去了、稽核列
// 永遠寫著 PENDING，沒有任何跡象。
func TestOrchestrate_ConfirmSurvivesCallerCancellation(t *testing.T) {
	stub := &closureAuditStub{}
	o := closureOrchestrator(stub)

	ctx, cancel := context.WithCancel(context.Background())
	_, denial := o.Orchestrate(ctx, Spec{
		Session:      closureSession(),
		Action:       "PRODUCT_UPDATE",
		ResourceType: "product",
		ResourceID:   "7001",
		ReadBefore: func(context.Context, *commonpb.DelegationContext) (*Resource, *Denial) {
			return &Resource{ID: "7001", Version: "3", Fields: map[string]any{"name": "before"}}, nil
		},
		Write: func(context.Context, *commonpb.DelegationContext) (*Resource, *Denial) {
			// 寫入已經成立，之後呼叫端才斷線 —— 這是真實時序，不是人造的邊角。
			cancel()
			return &Resource{ID: "7001", Version: "4", Fields: map[string]any{"name": "after"}}, nil
		},
		SnapshotFields: []string{"name"},
	})
	if denial != nil {
		t.Fatalf("不該被拒絕: %+v", denial)
	}
	if len(stub.confirms) != 1 {
		t.Fatalf("寫入已成立，confirm 必須照送；實際 %d 次", len(stub.confirms))
	}
	if stub.confirmCtxErr != nil {
		t.Fatalf("confirm 的 ctx 必須與呼叫端解耦；實際帶著 %v", stub.confirmCtxErr)
	}
}
