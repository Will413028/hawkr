package usecase

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func patchRow(reply string) repository.RevisionWithPatch {
	b, err := json.Marshal(turnPayload{Reply: reply})
	if err != nil {
		panic(err)
	}
	return repository.RevisionWithPatch{PatchPayloadJSON: b}
}

func TestBuildRecentNarratives_NewestFirst(t *testing.T) {
	got := buildRecentNarratives([]repository.RevisionWithPatch{
		patchRow("第一輪"), patchRow("第二輪"), patchRow("第三輪"),
	})
	want := []string{"第三輪", "第二輪"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildRecentNarratives = %v, want %v", got, want)
	}
}

func TestBuildRecentNarratives_EmptyHistory(t *testing.T) {
	if got := buildRecentNarratives(nil); len(got) != 0 {
		t.Fatalf("空 history 應該回空：%v", got)
	}
}

func TestBuildRecentNarratives_SkipsUnparseableRow(t *testing.T) {
	// 解析失敗跳過該筆而不是整包放棄：anti-repetition 是品質加分項，
	// 不是正確性必要條件。
	bad := repository.RevisionWithPatch{PatchPayloadJSON: []byte("{not json")}
	got := buildRecentNarratives([]repository.RevisionWithPatch{patchRow("好的"), bad})
	if !reflect.DeepEqual(got, []string{"好的"}) {
		t.Fatalf("buildRecentNarratives = %v, want [好的]", got)
	}
}

func TestBuildRecentNarratives_SkipsEmptyReply(t *testing.T) {
	got := buildRecentNarratives([]repository.RevisionWithPatch{patchRow("好的"), patchRow("")})
	if !reflect.DeepEqual(got, []string{"好的"}) {
		t.Fatalf("buildRecentNarratives = %v, want [好的]", got)
	}
}

type fakeSurfaceRealizer struct {
	blocks []RealizedBlock
	err    error

	// captured 記錄 realizeOrFallback 實際傳進來的 SurfaceContext——用來驗證
	// recent []string 有沒有被正確轉發，不是只驗證回傳值（見
	// TestRealizeOrFallback_ForwardsRecentNarrativesIntoSurfaceContext）。
	captured *SurfaceContext
}

func (f *fakeSurfaceRealizer) Realize(_ context.Context, sc SurfaceContext) ([]RealizedBlock, error) {
	f.captured = &sc
	return f.blocks, f.err
}

func planForPipeline() domain.DialoguePlanV1 {
	return domain.PlanDialogue(domain.DialoguePlanInput{
		Ready:    true,
		Question: &domain.ClarifyQuestion{SlotKey: domain.SlotDeliveryDate, Prompt: "希望什麼時候送達？"},
	})
}

func TestRealizeOrFallback_ModelPathReturnsModelBlocks(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}
	realizer := &fakeSurfaceRealizer{blocks: []RealizedBlock{{
		Kind: domain.BlockNextStep, Text: "想問一下大概什麼時候要用？",
		ClaimCodes: []string{string(domain.ClaimNextQuestionOptional)},
	}}}

	blocks, mode := realizeOrFallback(context.Background(), realizer, plan, args, domain.Slots{}, nil)
	if mode != "MODEL" {
		t.Fatalf("mode = %q, want MODEL", mode)
	}
	if len(blocks) != 1 || blocks[0].Text != "想問一下大概什麼時候要用？" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestRealizeOrFallback_ProviderErrorFallsBackVerbatim(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}

	blocks, mode := realizeOrFallback(context.Background(), &fakeSurfaceRealizer{err: errBoom}, plan, args, domain.Slots{}, nil)
	if mode != "TEMPLATE" {
		t.Fatalf("mode = %q, want TEMPLATE", mode)
	}
	if !reflect.DeepEqual(blocks, domain.RenderPlan(plan, args)) {
		t.Fatalf("退場必須逐字等於 RenderPlan：\ngot  %+v\nwant %+v", blocks, domain.RenderPlan(plan, args))
	}
}

func TestRealizeOrFallback_ValidatorRejectFallsBack(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}
	// 空 claimCodes 觸發第一層 reject。
	realizer := &fakeSurfaceRealizer{blocks: []RealizedBlock{{Kind: domain.BlockNextStep, Text: "好的"}}}

	blocks, mode := realizeOrFallback(context.Background(), realizer, plan, args, domain.Slots{}, nil)
	if mode != "TEMPLATE" {
		t.Fatalf("mode = %q, want TEMPLATE", mode)
	}
	if !reflect.DeepEqual(blocks, domain.RenderPlan(plan, args)) {
		t.Fatal("validator reject 後必須逐字等於 RenderPlan")
	}
}

func TestRealizeOrFallback_NilRealizerFallsBack(t *testing.T) {
	// DI 沒接上（或本地缺 API key）時不得 panic。
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}

	blocks, mode := realizeOrFallback(context.Background(), nil, plan, args, domain.Slots{}, nil)
	if mode != "TEMPLATE" || !reflect.DeepEqual(blocks, domain.RenderPlan(plan, args)) {
		t.Fatalf("nil realizer 必須安靜退回模板：mode=%q", mode)
	}
}

// TestRealizeOrFallback_ForwardsRecentNarrativesIntoSurfaceContext 是 review
// finding 的補洞：recent 是 realizeOrFallback 的活參數，一路轉發進
// SurfaceContext.RecentNarratives（surface_realizer.go 的 buildSurfaceRealizePrompt
// 會消費它），只是資料來源從已刪除的 RecentNarrativeReader 換成呼叫端組好的
// []string，行為本身沒有消失。若漏寫 RecentNarratives: recent，所有既有測試
// 依然全綠（都傳 nil），anti-repetition 會靜默失效——這支測試專門擋這個退步。
// 刻意讓 realizer 回 error 退到 fallback：這裡只關心 SurfaceContext 有沒有被
// 正確組出，不關心 realize 成功與否。
func TestRealizeOrFallback_ForwardsRecentNarrativesIntoSurfaceContext(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}
	recent := []string{"第一輪：已記錄品項。", "第二輪：已確認到貨日。"}
	realizer := &fakeSurfaceRealizer{err: errBoom}

	_, _ = realizeOrFallback(context.Background(), realizer, plan, args, domain.Slots{}, recent)

	if realizer.captured == nil {
		t.Fatal("realizer.Realize 未被呼叫，SurfaceContext 無法驗證")
	}
	if !reflect.DeepEqual(realizer.captured.RecentNarratives, recent) {
		t.Fatalf("SurfaceContext.RecentNarratives = %v, want %v", realizer.captured.RecentNarratives, recent)
	}
}

// TestRealizeOrFallback_CommittedSlotsFeedFactEchoAllowlist 是 whole-branch
// review I2 接線的補洞：realizeOrFallback 把 committed 轉發給
// validateSurfaceBlocks（surface_validator.go 的第三層 fact-echo）這件事，此前
// 沒有任何測試守護。若有人把第五個參數改回 domain.Slots{}，所有既有測試依然
// 全綠（都傳 domain.Slots{}），fact-echo 會靜默退回誤擋狀態——症狀是「敘述又
// 每輪都一樣」，正是 I2 要修的東西。
//
// 這裡刻意讓 committed 帶著 SafeRenderArgs 完全沒有的值（模擬第 3 輪：這一輪
// recorded 的是別的欄位，committed 裡的 Quantity／DeliveryCity 是更早輪次留
// 下來的事實），斷言的是行為差異（renderMode），不是參數有沒有被傳遞：
// 接線正確 → committed 進 haystack → fact-echo 放行 → MODEL；
// 接線被改回空值 → haystack 沒有這些值 → fact_mismatch → TEMPLATE。
func TestRealizeOrFallback_CommittedSlotsFeedFactEchoAllowlist(t *testing.T) {
	plan := planForPipeline()
	committed := domain.Slots{Quantity: intPtr(50), DeliveryCity: strPtr("桃園市")}
	// SafeRenderArgs 刻意不含 50／桃園市：這一輪 recorded 的是下一題的問句，
	// 不是這兩個欄位。
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}
	realizer := &fakeSurfaceRealizer{blocks: []RealizedBlock{{
		Kind: domain.BlockUnderstood, Text: "桃園的 50 份，我已經記下囉。",
		ClaimCodes: []string{string(domain.ClaimRequestFactRecorded)},
	}}}

	_, mode := realizeOrFallback(context.Background(), realizer, plan, args, committed, nil)
	if mode != "MODEL" {
		t.Fatalf("mode = %q, want MODEL（committed 的事實值應該放行 fact-echo）", mode)
	}
}

// ctxWithLogObserver 回傳一個帶 observer logger 的 context ＋ 收集到的紀錄。
// pkglogger.Warn 走 FromContext，所以注入 context 就能攔下實際輸出——不是只
// 驗證「有呼叫」，而是驗證「欄位真的帶出去了」。
func ctxWithLogObserver(t *testing.T) (context.Context, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.WarnLevel)
	return pkglogger.WithLogger(context.Background(), zap.New(core)), logs
}

// 被擋的文字只活到 realizeOrFallback 的 reject 分支：退場之後 realized 就被丟棄，
// 回應與 metric 都只剩「退回了模板」。這支測試守住「擋了什麼」這個證據真的有被
// 記下來——沒有它，validation_reject_total 上升時只能靠推理猜原因。
func TestRealizeOrFallback_ValidatorRejectLogsBlockedText(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}
	// 空 claimCodes 觸發第一層 reject（與 TestRealizeOrFallback_ValidatorRejectFallsBack 同形）。
	realizer := &fakeSurfaceRealizer{blocks: []RealizedBlock{{Kind: domain.BlockNextStep, Text: "我們已經幫你找到 3 家供應商了"}}}

	ctx, logs := ctxWithLogObserver(t)
	if _, mode := realizeOrFallback(ctx, realizer, plan, args, domain.Slots{}, nil); mode != "TEMPLATE" {
		t.Fatalf("mode = %q, want TEMPLATE", mode)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want exactly 1 log entry, got %d: %+v", len(entries), entries)
	}
	fields := entries[0].ContextMap()
	if got := fields["reason"]; got != "claim_violation" {
		t.Errorf(`reason = %v, want "claim_violation"`, got)
	}
	if got := fields["act_code"]; got != string(plan.ActCode) {
		t.Errorf("act_code = %v, want %v", got, plan.ActCode)
	}
	// 關鍵斷言：被擋的原文要能在 log 裡找到，否則這次改動等於沒做。
	// ContextMap 會把 zap.Strings 攤成 []interface{}，不是 []string。
	raw, ok := fields["rejected_blocks"].([]interface{})
	if !ok || len(raw) != 1 {
		t.Fatalf("rejected_blocks = %#v, want 1 element", fields["rejected_blocks"])
	}
	blocked, _ := raw[0].(string)
	if !strings.Contains(blocked, "我們已經幫你找到 3 家供應商了") {
		t.Errorf("rejected_blocks[0] 不含被擋原文：%q", blocked)
	}
	if !strings.Contains(blocked, string(domain.BlockNextStep)) {
		t.Errorf("rejected_blocks[0] 不含 block kind：%q", blocked)
	}
}

// provider 失敗時 err 同樣只有這一次記錄機會——逾時／HTTP 400／JSON 解析失敗
// 塌縮成同一個 result="error" 序列，沒有 log 就分不出是哪一種（2026-08-31 復原
// 時的 C1 正是 100% 走這條且完全靜默）。
func TestRealizeOrFallback_ProviderErrorLogsError(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}

	ctx, logs := ctxWithLogObserver(t)
	if _, mode := realizeOrFallback(ctx, &fakeSurfaceRealizer{err: errBoom}, plan, args, domain.Slots{}, nil); mode != "TEMPLATE" {
		t.Fatalf("mode = %q, want TEMPLATE", mode)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want exactly 1 log entry, got %d", len(entries))
	}
	if got := entries[0].ContextMap()["error"]; got != errBoom.Error() {
		t.Errorf("error = %v, want %v", got, errBoom)
	}
}

// 成功路徑不得產生 log——reject 是例外路徑才不取樣；如果每輪都寫一筆，這個
// 「不取樣」的前提就不成立了。
func TestRealizeOrFallback_ModelPathLogsNothing(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}
	realizer := &fakeSurfaceRealizer{blocks: []RealizedBlock{{
		Kind: domain.BlockNextStep, Text: "想問一下大概什麼時候要用？",
		ClaimCodes: []string{string(domain.ClaimNextQuestionOptional)},
	}}}

	ctx, logs := ctxWithLogObserver(t)
	if _, mode := realizeOrFallback(ctx, realizer, plan, args, domain.Slots{}, nil); mode != "MODEL" {
		t.Fatalf("mode = %q, want MODEL", mode)
	}
	if n := logs.Len(); n != 0 {
		t.Fatalf("成功路徑不該寫 log，got %d 筆：%+v", n, logs.All())
	}
}

// 中文是多位元組字元：byte 截斷會切在字元中間，log 裡出現替代字元會讓人誤以為
// 模型輸出本身壞掉。這支釘住「以 rune 截斷」。
func TestRejectedBlockDigests_TruncatesOnRuneBoundary(t *testing.T) {
	long := strings.Repeat("採", rejectedBlockTextLimit+50)
	digests := rejectedBlockDigests([]RealizedBlock{{Kind: domain.BlockNextStep, Text: long}})
	if len(digests) != 1 {
		t.Fatalf("digests = %#v", digests)
	}
	text := digests[0][strings.LastIndex(digests[0], "|")+1:]
	if !strings.HasSuffix(text, "…") {
		t.Errorf("超長文字應以省略號結尾：%q", text[len(text)-10:])
	}
	if got := len([]rune(text)); got != rejectedBlockTextLimit+1 {
		t.Errorf("截斷後 rune 數 = %d, want %d（含省略號）", got, rejectedBlockTextLimit+1)
	}
	if strings.ContainsRune(text, '\uFFFD') {
		t.Error("出現 U+FFFD，表示切在多位元組字元中間")
	}
}

// ClaimCodes 要一起記：第一層的 claim_violation 是對「模型自報了什麼」的判定，
// 只看 Text 看不出它為什麼被擋。
func TestRejectedBlockDigests_IncludesClaimCodes(t *testing.T) {
	digests := rejectedBlockDigests([]RealizedBlock{{
		Kind: domain.BlockUnderstood, Text: "好的",
		ClaimCodes: []string{"REQUEST_FACT_RECORDED", "BASICS_COMPLETE"},
	}})
	if len(digests) != 1 || !strings.Contains(digests[0], "REQUEST_FACT_RECORDED,BASICS_COMPLETE") {
		t.Fatalf("digests = %#v", digests)
	}
}

// 停用實作每一輪都會回 ErrProcurementLLMDisabled——那不是非預期失敗，不該寫 log。
// 這支守住上面那三支「reject 是例外路徑所以不取樣」的前提：停用環境下若每輪一筆
// WARN，log 就被淹掉，非預期失敗再也找不到。
func TestRealizeOrFallback_DisabledRealizerLogsNothing(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}

	ctx, logs := ctxWithLogObserver(t)
	blocks, mode := realizeOrFallback(ctx, disabledSurfaceRealizer{}, plan, args, domain.Slots{}, nil)
	if mode != "TEMPLATE" {
		t.Fatalf("mode = %q, want TEMPLATE", mode)
	}
	if !reflect.DeepEqual(blocks, domain.RenderPlan(plan, args)) {
		t.Fatal("停用時必須逐字等於 RenderPlan")
	}
	if n := logs.Len(); n != 0 {
		t.Fatalf("停用是預期狀態，不該寫 log，got %d 筆：%+v", n, logs.All())
	}
}

// 對照組：一般 provider 失敗照樣要記——上面那條抑制不能誤傷真正的失敗。
func TestRealizeOrFallback_GenericErrorStillLogs(t *testing.T) {
	plan := planForPipeline()
	args := domain.SafeRenderArgs{NextQuestionPrompt: "希望什麼時候送達？"}

	ctx, logs := ctxWithLogObserver(t)
	realizeOrFallback(ctx, &fakeSurfaceRealizer{err: errBoom}, plan, args, domain.Slots{}, nil)
	if n := logs.Len(); n != 1 {
		t.Fatalf("一般 provider 失敗必須記，got %d 筆", n)
	}
}
