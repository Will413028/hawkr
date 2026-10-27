package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestRunDispatcher_Dispatch_RunsInBackground(t *testing.T) {
	d := NewRunDispatcher()
	var wg sync.WaitGroup
	wg.Add(1)
	ran := false
	d.Dispatch(context.Background(), func(bgCtx context.Context) {
		defer wg.Done()
		ran = true
	})

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatched work did not run within timeout")
	}
	if !ran {
		t.Fatal("work was not executed")
	}
}

// TestRunDispatcher_Dispatch_SurvivesCallerCancelButKeepsCallerValues 釘住
// issue #3519 slice B Task 5 修的那個缺陷本身：Dispatch 從 context.Background()
// 換成 context.WithoutCancel(ctx) 之後，(1) 呼叫端 ctx 取消／逾時不得連帶
// 中止背景 work 拿到的 bgCtx（這是 WithoutCancel 存在的理由），且 (2) 掛在
// 呼叫端 ctx 上的值（trace span、request-scoped logger 的具體實作就是這種
// context value）必須原封不動地穿透到 bgCtx——這是「trace 不再消失」這句話
// 唯一能在單元測試層級直接斷言的部分；trace 是否真的出現在後端（span 匯出、
// 收集）不在這個測試範圍內，需要跑起來的服務才能驗證。
func TestRunDispatcher_Dispatch_SurvivesCallerCancelButKeepsCallerValues(t *testing.T) {
	d := NewRunDispatcher()
	type ctxKey struct{}
	callerCtx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "trace-value"))

	// 呼叫端 ctx 在呼叫 Dispatch 之前就取消——模擬 request 已經回應完畢／逾時
	// 這個時間點，同時讓下面 bgCtx.Err() 的斷言變成決定性的，不是競速。Dispatch
	// 內部才會對 callerCtx 呼叫 context.WithoutCancel，所以把 cancel() 挪到
	// Dispatch 呼叫之前，可以保證不論背景 goroutine 何時真正執行到
	// bgCtx.Err()，callerCtx 這時都已經確定是 Canceled 狀態；這樣「bgCtx.Err()
	// 必須是 nil」才是真的在驗證 WithoutCancel 生效，而不是剛好在背景 goroutine
	// 讀到 Err() 之前贏了一場競速（final review Finding 10：原本 cancel() 排在
	// Dispatch 之後呼叫，即使把 Dispatch 錯改回直接傳 callerCtx、完全不呼叫
	// WithoutCancel，這個斷言也可能剛好在 cancel() 生效前讀到 nil 而假綠——對
	// 那個回歸沒有鑑別力，真正抓到 Task 5 那個缺陷的只有下面的 value-passthrough
	// 斷言）。
	cancel()

	seenValue := make(chan any, 1)
	seenErr := make(chan error, 1)
	done := make(chan struct{})
	d.Dispatch(callerCtx, func(bgCtx context.Context) {
		defer close(done)
		seenValue <- bgCtx.Value(ctxKey{})
		seenErr <- bgCtx.Err()
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatched work did not run within timeout")
	}

	if got := <-seenValue; got != "trace-value" {
		t.Fatalf("bgCtx lost the caller's context value: got %v, want %q", got, "trace-value")
	}
	if err := <-seenErr; err != nil {
		t.Fatalf("caller ctx cancellation leaked into bgCtx: bgCtx.Err() = %v, want nil", err)
	}
}

func TestRunDispatcher_Dispatch_PanicDoesNotCrashCaller(t *testing.T) {
	d := NewRunDispatcher()
	done := make(chan struct{})
	d.Dispatch(context.Background(), func(bgCtx context.Context) {
		defer close(done)
		panic("boom")
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("panicking work did not complete within timeout")
	}
	// 測試 process 走到這裡代表 panic 被 SafeGo 接住，沒有帶走整個 test binary。
}

// interruptOutcomeFor 判斷的是「這一輪最終要問買家的問題是不是那張消歧卡」
// （result.Question.SlotKey）。Task 7 接線時發現原本的
// `DialoguePlan.ActCode == ActItemNeedsDisambiguation` 判準是錯的，理由見
// run_dispatch.go 該函式的 doc comment；以下三個測試釘住修正後的行為，其中
// 第三個直接重現原本會漏判的情境。
func TestInterruptOutcomeFor_ClarificationWhenQuestionIsCropDisambiguation(t *testing.T) {
	result := TurnResult{Question: &domain.ClarifyQuestion{SlotKey: domain.SlotCropDisambiguation}}

	kind, ok := interruptOutcomeFor(result)

	if !ok {
		t.Fatal("want ok=true when the turn's question is the crop disambiguation card")
	}
	if kind != "CLARIFICATION" {
		t.Fatalf("got interruptKind=%q, want CLARIFICATION", kind)
	}
}

func TestInterruptOutcomeFor_NotOkForOrdinaryQuestions(t *testing.T) {
	cases := []struct {
		name   string
		result TurnResult
	}{
		{"一般缺項問句", TurnResult{Question: &domain.ClarifyQuestion{SlotKey: domain.SlotQuantity}}},
		{"這一輪沒有問句", TurnResult{Question: nil}},
		{
			// 光有 act code 不算數：act code 是「這一輪要講什麼話」，
			// interrupt 要看的是「有沒有一張非答不可的消歧卡」。
			name:   "act code 是消歧但沒有消歧問句卡",
			result: TurnResult{DialoguePlan: domain.DialoguePlanV1{ActCode: domain.ActItemNeedsDisambiguation}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kind, ok := interruptOutcomeFor(tc.result)
			if ok {
				t.Fatal("want ok=false")
			}
			if kind != "" {
				t.Fatalf("got interruptKind=%q, want empty string", kind)
			}
		})
	}
}

// TestInterruptOutcomeFor_DisambiguationPendingWithHigherPriorityActCode 是這次
// 修正的迴歸哨兵，重現舊判準會漏掉的那個真實情境：買家在同一輪裡既觸發了品項
// 消歧、又修正了一個已填欄位（或輸入被 reject／抽取降級）。
//
// selectActCode（domain/dialogue_planner.go）把 CropDisambiguationPending 排在
// Degraded／HasRejected／computeHasCorrection **之後**，所以 act code 會是那三個
// 之一，看不出待消歧；但 turn.go 在 cropGate.Pending 時**無條件**把 question 換成
// 消歧卡，兩者互相獨立。舊判準（看 act code）會讓這種 turn 以 SUCCEEDED 收尾、
// 不建 interrupt，買家因此卡在一張永遠無法 resume 的消歧卡上。
func TestInterruptOutcomeFor_DisambiguationPendingWithHigherPriorityActCode(t *testing.T) {
	// 這三個 act code 在 selectActCode 裡都排在 CropDisambiguationPending 之前。
	for _, act := range []domain.ActCode{
		domain.ActExtractionDegraded,
		domain.ActInputRejected,
		domain.ActCorrectionApplied,
	} {
		t.Run(string(act), func(t *testing.T) {
			result := TurnResult{
				DialoguePlan: domain.DialoguePlanV1{ActCode: act},
				Question:     &domain.ClarifyQuestion{SlotKey: domain.SlotCropDisambiguation},
			}

			kind, ok := interruptOutcomeFor(result)

			if !ok {
				t.Fatalf("act code %q 蓋過消歧判斷了——消歧卡還在，run 卻不會建 interrupt", act)
			}
			if kind != "CLARIFICATION" {
				t.Fatalf("got interruptKind=%q, want CLARIFICATION", kind)
			}
		})
	}
}

// turnPipelineOutcome 本身沒有行為，Task 7/8 會把它當作 executeTurnPipeline／
// executeAnswerPipeline 的回傳形狀使用。這裡只驗證欄位確實存在且可賦值——
// 主要目的是讓 Task 7/8 若不慎改動欄位名稱時有一個立即失敗的訊號，而不是
// 靠使用端間接發現。
func TestTurnPipelineOutcome_FieldsAssignable(t *testing.T) {
	outcome := turnPipelineOutcome{
		result:        TurnResult{RequestID: 1},
		interruptID:   "int-1",
		interruptKind: "CLARIFICATION",
	}

	if outcome.result.RequestID != 1 || outcome.interruptID != "int-1" || outcome.interruptKind != "CLARIFICATION" {
		t.Fatal("turnPipelineOutcome fields did not round-trip as assigned")
	}
}

func TestCanonicalHash_SameInputSameHash(t *testing.T) {
	type payload struct {
		BuyerID string
		Message string
	}
	in := payload{BuyerID: "buyer-1", Message: "hello"}

	if got, want := canonicalHash(in), canonicalHash(in); got != want {
		t.Fatalf("canonicalHash not deterministic: %q != %q", got, want)
	}
}

func TestCanonicalHash_DifferentInputDifferentHash(t *testing.T) {
	type payload struct {
		BuyerID string
		Message string
	}
	a := canonicalHash(payload{BuyerID: "buyer-1", Message: "hello"})
	b := canonicalHash(payload{BuyerID: "buyer-1", Message: "goodbye"})

	if a == b {
		t.Fatal("canonicalHash produced the same hash for different inputs")
	}
}

func TestCanonicalHash_UnmarshalableInputFallsBackToSentinel(t *testing.T) {
	// func 值無法被 json.Marshal，觸發 canonicalHash 的 sentinel fallback 路徑
	// （見 canonicalHash 內的錯誤處理註解）。sentinel 是直接對字面 byte slice
	// sha256（不經過 json.Marshal，否則 []byte 會被 base64 編碼成別的位元組），
	// 所以這裡用 sha256 直接算期望值，而不是遞迴呼叫 canonicalHash。
	sum := sha256.Sum256([]byte("canonical-hash-marshal-error"))
	want := hex.EncodeToString(sum[:])

	got := canonicalHash(func() {})

	if got != want {
		t.Fatalf("unmarshalable input did not hash to the documented sentinel: got %q, want %q", got, want)
	}
}

// TestCanonicalResumePayloadHash_MatchesCanonicalHash 釘住
// CanonicalResumePayloadHash 是 canonicalHash 的匯出 wrapper（Task 14，Task 14b
// M4 把簽章擴充成 (buyerID, interruptID, payload)——匯出給 internal/grpc 的
// ResumeRun handler 用，見該函式文件）——沒有另外重新實作一份雜湊邏輯，對同一組
// 輸入必須跟直接呼叫 canonicalHash（用同一個匿名 struct 形狀）逐字相同。
func TestCanonicalResumePayloadHash_MatchesCanonicalHash(t *testing.T) {
	buyerID := identity.BuyerID(8001)
	interruptID := "int-1"
	payload := []byte(`{"optionId":"opt-1"}`)

	want := canonicalHash(struct {
		BuyerID     identity.BuyerID
		InterruptID string
		Payload     []byte
	}{buyerID, interruptID, payload})

	if got := CanonicalResumePayloadHash(buyerID, interruptID, payload); got != want {
		t.Fatalf("CanonicalResumePayloadHash(...) = %q, want %q", got, want)
	}
}

func TestCanonicalResumePayloadHash_DifferentPayloadDifferentHash(t *testing.T) {
	a := CanonicalResumePayloadHash(8001, "int-1", []byte(`{"optionId":"opt-1"}`))
	b := CanonicalResumePayloadHash(8001, "int-1", []byte(`{"optionId":"opt-2"}`))

	if a == b {
		t.Fatal("CanonicalResumePayloadHash produced the same hash for different payloads")
	}
}

func TestCanonicalResumePayloadHash_SamePayloadSameHash(t *testing.T) {
	payload := []byte(`{"optionId":"opt-1"}`)

	if got, want := CanonicalResumePayloadHash(8001, "int-1", payload), CanonicalResumePayloadHash(8001, "int-1", payload); got != want {
		t.Fatalf("CanonicalResumePayloadHash not deterministic: %q != %q", got, want)
	}
}

// TestCanonicalResumePayloadHash_DifferentInterruptIDDifferentHash 釘住 Task 14b
// M4 修的那個洞：同一把 clientRequestID + 同一串 payload bytes，但用在**另一個**
// interrupt 上，必須產生不同的 hash——否則買家對 I1 用 key X 成功後，對 I2 送
// 同一把 key、同一串 bytes，會命中 I1 的冪等鍵、拿到 I1 的 child，而 I2 從未被
// 真正解析。
func TestCanonicalResumePayloadHash_DifferentInterruptIDDifferentHash(t *testing.T) {
	payload := []byte(`{"optionId":"opt-1"}`)
	a := CanonicalResumePayloadHash(8001, "int-1", payload)
	b := CanonicalResumePayloadHash(8001, "int-2", payload)

	if a == b {
		t.Fatal("CanonicalResumePayloadHash produced the same hash for different interrupt IDs")
	}
}

// TestCanonicalResumePayloadHash_DifferentBuyerIDDifferentHash 釘住 M4 的另一半：
// 不同買家不得共用同一個雜湊值。
func TestCanonicalResumePayloadHash_DifferentBuyerIDDifferentHash(t *testing.T) {
	payload := []byte(`{"optionId":"opt-1"}`)
	a := CanonicalResumePayloadHash(8001, "int-1", payload)
	b := CanonicalResumePayloadHash(8002, "int-1", payload)

	if a == b {
		t.Fatal("CanonicalResumePayloadHash produced the same hash for different buyer IDs")
	}
}

// ---------------------------------------------------------------------------
// GuardNoOpenInterrupt（Task 14b Blocking 3）
// ---------------------------------------------------------------------------

type fakeOpenInterruptStore struct {
	hasOpen   bool
	interrupt repository.Interrupt
	err       error
}

func (f *fakeOpenInterruptStore) FindOpenInterruptForRequest(context.Context, int64, identity.BuyerID) (repository.Interrupt, error) {
	if f.err != nil {
		return repository.Interrupt{}, f.err
	}
	if f.hasOpen {
		return f.interrupt, nil
	}
	return repository.Interrupt{}, repository.ErrNotFound
}

func TestGuardNoOpenInterrupt_NoOpenInterrupt_PassesThrough(t *testing.T) {
	store := &fakeOpenInterruptStore{}
	if err := GuardNoOpenInterrupt(context.Background(), store, 1, identity.BuyerID(1234)); err != nil {
		t.Fatalf("expected nil error when there is no open interrupt, got %v", err)
	}
}

func TestGuardNoOpenInterrupt_OpenInterrupt_ReturnsErrInterruptPending(t *testing.T) {
	store := &fakeOpenInterruptStore{hasOpen: true, interrupt: repository.Interrupt{ID: "int-1", Status: "PENDING"}}
	err := GuardNoOpenInterrupt(context.Background(), store, 1, identity.BuyerID(1234))
	if !errors.Is(err, ErrInterruptPending) {
		t.Fatalf("expected ErrInterruptPending, got %v", err)
	}
}

// TestGuardNoOpenInterrupt_StoreError_PropagatesRaw 釘住：查詢本身失敗（非
// ErrNotFound）必須原樣往上傳，不得被吞成「沒有 open interrupt」而誤放行。
func TestGuardNoOpenInterrupt_StoreError_PropagatesRaw(t *testing.T) {
	boom := errors.New("db exploded")
	store := &fakeOpenInterruptStore{err: boom}
	err := GuardNoOpenInterrupt(context.Background(), store, 1, identity.BuyerID(1234))
	if !errors.Is(err, boom) {
		t.Fatalf("expected the raw store error to propagate, got %v", err)
	}
}
