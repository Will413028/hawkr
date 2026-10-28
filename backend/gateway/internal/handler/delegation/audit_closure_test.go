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

	batchIntentResults []*accountpb.RecordAuditIntentItemResult
	batchConfirms      []*accountpb.ConfirmAuditResultRequest
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

func (s *closureAuditStub) RecordAuditIntentBatch(_ context.Context, in *accountpb.RecordAuditIntentBatchRequest, _ int) (*accountpb.RecordAuditIntentBatchResponse, error) {
	if s.batchIntentResults != nil {
		return &accountpb.RecordAuditIntentBatchResponse{Results: s.batchIntentResults}, nil
	}
	results := make([]*accountpb.RecordAuditIntentItemResult, 0, len(in.GetItems()))
	for i, item := range in.GetItems() {
		results = append(results, &accountpb.RecordAuditIntentItemResult{
			ResourceId: item.GetResourceId(),
			Ok:         true,
			Data:       &accountpb.RecordAuditIntentData{AuditLogId: itemAuditLogID(i)},
		})
	}
	return &accountpb.RecordAuditIntentBatchResponse{Results: results}, nil
}

func (s *closureAuditStub) ConfirmAuditResultBatch(_ context.Context, in *accountpb.ConfirmAuditResultBatchRequest, _ int) (*accountpb.ConfirmAuditResultBatchResponse, error) {
	s.batchConfirms = append(s.batchConfirms, in.GetItems()...)
	return &accountpb.ConfirmAuditResultBatchResponse{}, nil
}

func itemAuditLogID(i int) string { return []string{"9101", "9102", "9103"}[i] }

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

// TestOrchestrateBatch_MissingWriteResultStillCloses：領域服務沒回報的元素也必須關。
//
// 收 confirm 的迴圈跑的是 writeResults 而不是 writeItems，所以「有 intent 但沒出現在
// 結果裡」的元素會整個掉出去 —— 沒有 confirm、沒有 log、回應裡那一格還是初始的
// OK=false/Code=0，看起來就像一個普通的失敗元素。今天 Java 端是逐 request item 迴圈、
// 保證 1:1，但這一側沒有任何東西在守那個假設。
func TestOrchestrateBatch_MissingWriteResultStillCloses(t *testing.T) {
	stub := &closureAuditStub{}
	o := closureOrchestrator(stub)

	results, denial := o.OrchestrateBatch(context.Background(), BatchSpec{
		Session:      closureSession(),
		Action:       "PRODUCT_BATCH_UNPUBLISH",
		ResourceType: "product",
		Items: []BatchItem{
			{ResourceID: "7001"},
			{ResourceID: "7002"},
			{ResourceID: "7003"},
		},
		ReadBatch: func(context.Context, *commonpb.DelegationContext) (map[string]*Resource, *Denial) {
			return map[string]*Resource{
				"7001": {ID: "7001", Version: "1", Fields: map[string]any{"name": "a"}},
				"7002": {ID: "7002", Version: "1", Fields: map[string]any{"name": "b"}},
				"7003": {ID: "7003", Version: "1", Fields: map[string]any{"name": "c"}},
			}, nil
		},
		WriteBatch: func(_ context.Context, _ *commonpb.DelegationContext, items []BatchWriteItem) ([]BatchWriteResult, *Denial) {
			// 上游只回報了第一與第三個元素。7002 憑空消失。
			out := make([]BatchWriteResult, 0, 2)
			for _, it := range items {
				if it.ResourceID == "7002" {
					continue
				}
				out = append(out, BatchWriteResult{ResourceID: it.ResourceID, OK: true, Version: "2"})
			}
			return out, nil
		},
		AfterFields:    func(before map[string]any) map[string]any { return before },
		SnapshotFields: []string{"name"},
	})
	if denial != nil {
		t.Fatalf("部分失敗不該讓整批被拒: %+v", denial)
	}
	if len(stub.batchConfirms) != 3 {
		t.Fatalf("三列 intent 就要三列 confirm；實際 %d 列", len(stub.batchConfirms))
	}
	byID := map[string]*accountpb.ConfirmAuditResultRequest{}
	for _, c := range stub.batchConfirms {
		byID[c.GetResourceId()] = c
	}
	missing, ok := byID["7002"]
	if !ok {
		t.Fatal("上游沒回報的那一列完全沒被 confirm —— 它會永遠停在 PENDING")
	}
	if missing.GetResult() != "FAILED" {
		t.Fatalf("「問了但沒得到答案」對稽核來說就是 FAILED；實際 %q", missing.GetResult())
	}
	if missing.GetBizCode() == 0 {
		t.Fatal("FAILED 必須帶得出原因碼，否則稽核列說不出為什麼失敗")
	}
	for _, id := range []string{"7001", "7003"} {
		if byID[id].GetResult() != "SUCCEEDED" {
			t.Fatalf("%s 有回報且成功，必須是 SUCCEEDED；實際 %q", id, byID[id].GetResult())
		}
	}
	// 回應那一側也不能說謊：那個元素不是「成功」。
	for _, r := range results {
		if r.ResourceID == "7002" && r.OK {
			t.Fatal("上游沒回報的元素不得回報成 OK")
		}
	}
}
