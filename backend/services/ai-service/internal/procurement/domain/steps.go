package domain

import (
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// stepStatusSucceeded 綁到 CodeMeta 生成常數（ProcurementStepStatus 的成功終態，舊值 DONE）。
// 取區域名字純粹是為了讓下面 10 個 struct literal 不必各寫一次完整路徑——它不是第二份定義：
// CodeMeta 改了這裡自動跟著變，編譯期就綁死。真正的第二份定義是把 "SUCCEEDED" 寫成字串
// 字面值，那正是本次要消滅的東西。
var stepStatusSucceeded = codemeta.ProcurementStepStatusSucceeded.Value

// StepCode 是白名單步驟碼——POST /turns 回應、Activity rail 與候選搜尋階段共用
// 同一份列舉，避免兩邊各寫一份字面值而 drift。
//
// clarify 階段（8 碼會被 emit）：ComputeTurnSteps 產生 UNDERSTAND_INPUT／
// EXTRACT_SLOTS／VALIDATE_SLOTS 三碼必發，CROP_GATE／DEGRADED_FALLBACK／
// SLOTS_REJECTED 三碼條件性發，末碼是 PICK_NEXT_QUESTION 或 REQUIREMENT_READY
// 二擇一（ComputeAnswerSteps 是它的子集，少前兩碼）。
//
// 候選搜尋階段（T23，2026-09-05）：只有 SEARCH_CANDIDATES 與它底下的
// SEARCH_PLATFORM／SEARCH_EXTERNAL 兩條子列、加上末碼 SUMMARIZE 共 4 碼會被
// emit，清單見 CandidateSearchRailPlan。**READ_SELLER／COMPARE／VERIFY_CERT／
// VERIFY_ORIGIN／CHECK_STOCK／RANK_CANDIDATES 這 6 碼刻意維持零 emit**：
// usecase.CandidateSearchUseCase.run 全部的工作就是 claim → 兩條 lane 併發搜尋
// → finalize，沒有逐家讀賣家、規格比對、履歷／產地／供應量查核，排序也是在各
// lane 的 Search 內部就做完（candidate_platform_lane.go 的 relevance＋tier 加權
// 排序），不是跨 lane 的獨立階段。為這些不存在的工作發步驟等於替買家編造進度，
// 正是 doc57 §1.1 evidence contract 要擋的事——等它們命名的工作真的存在再接。
//
// FAILED 已於 T23 移除：失敗表達在 status 軸（codemeta.ProcurementStepStatusFailed），
// 不在 code 軸。ledger 的 STEP_STARTED／STEP_FINISHED 與 Activity payload 的
// status 欄位本來就在表達狀態，再讓 code 軸也有一個 FAILED 會使同一件事橫跨兩
// 個 enum、各自演化。刻意整個拿掉而不是留一個 deprecated 常數：重新引入同名
// code 會變成編譯錯誤，比註解可靠（同 ai.proto MessageEvent 的 reserved 手法）。
//
// 🔴 CROP_GATE 曾經是「只走 T8 ADK graph 的 Activity event 路徑、不是
// Compute*Steps 的 live path」（見 usecase.NewCropGateNode／emitActivityEvent）。
// 那條規則已作廢：即時軌只在該輪進行中掛載，回覆一落地就整軌卸載，於是
// 「核對品項分類」這個真的跑過的步驟在靜止的對話紀錄裡從來不存在。更糟的是
// crop_gate node／answer.go／rewind.go 三處都已經呼叫 narrateStepOutcome 算好
// 敘述並塞進 narration map，而 applyStepNarration 只會把文字貼到「已存在的
// step」上——沒有 CROP_GATE 這一格，那些 LLM 呼叫的產物每輪都被靜默丟棄。
// 現在三支 Compute*Steps 都會在 gate 真的做過判定時（cropidentity.GateResult
// .Evaluated()）補上這一碼，Activity event 照舊發，兩條路徑並存不衝突。
type StepCode string

const (
	StepUnderstandInput  StepCode = "UNDERSTAND_INPUT"
	StepExtractSlots     StepCode = "EXTRACT_SLOTS"
	StepValidateSlots    StepCode = "VALIDATE_SLOTS"
	StepCropGate         StepCode = "CROP_GATE"
	StepSlotsRejected    StepCode = "SLOTS_REJECTED"
	StepPickNextQuestion StepCode = "PICK_NEXT_QUESTION"
	StepRequirementReady StepCode = "REQUIREMENT_READY"
	// StepSearchCandidates 是候選搜尋階段的父列（設計稿 01「搜尋候選供應商」），
	// 兩條 lane 子列掛在它底下。它有自己真實可觀測的生命週期——從
	// MarkCandidateSetRunning 認領成功到兩條 lane 都 join 回來——不是由子列
	// 推導出來的裝飾列，所以由 server 直接宣告狀態，而不是讓前端自己算。
	StepSearchCandidates StepCode = "SEARCH_CANDIDATES"
	StepSearchPlatform   StepCode = "SEARCH_PLATFORM"
	StepSearchExternal   StepCode = "SEARCH_EXTERNAL"
	StepReadSeller       StepCode = "READ_SELLER"
	StepCompare          StepCode = "COMPARE"
	StepVerifyCert       StepCode = "VERIFY_CERT"
	StepVerifyOrigin     StepCode = "VERIFY_ORIGIN"
	StepCheckStock       StepCode = "CHECK_STOCK"
	StepRankCandidates   StepCode = "RANK_CANDIDATES"
	StepSummarize        StepCode = "SUMMARIZE"
	StepDegradedFallback StepCode = "DEGRADED_FALLBACK"
)

// StepLabels 是伺服器固定字串表——label 永遠查這張表，結構上不可能回填模型
// 原文。字面值照 issue #2343 2026-08-10 comment 給的文案，不改動。
var StepLabels = map[StepCode]string{
	StepUnderstandInput:  "讀你寫的需求",
	StepExtractSlots:     "抓出品項、數量和交期",
	StepValidateSlots:    "核對條件合不合理",
	StepCropGate:         "核對品項分類",
	StepSlotsRejected:    "有幾個條件先擱著",
	StepPickNextQuestion: "看還缺什麼關鍵資訊",
	StepRequirementReady: "需求齊了",
	StepSearchCandidates: "搜尋候選供應商",
	StepSearchPlatform:   "翻平台上的賣家",
	StepSearchExternal:   "找平台外的業者",
	StepReadSeller:       "讀這家賣家的資料",
	StepCompare:          "把規格和報價擺一起比",
	StepVerifyCert:       "查產銷履歷",
	StepVerifyOrigin:     "查產地對不對",
	StepCheckStock:       "看供應量夠不夠",
	StepRankCandidates:   "排出最合適的順序",
	StepSummarize:        "整理成給你的建議",
	StepDegradedFallback: "解析不穩，改用簡化方式",
}

// Step 是 API 回應與 DB 持久化（answer_patches.payload_json.steps）共用的形狀。
type Step struct {
	Code   StepCode `json:"code"`
	Status string   `json:"status"`
	Label  string   `json:"label"`
	// Text 是這一步「實際發生了什麼」的一句話，由 StepNarrator（LLM）就該步驟
	// 的結構化結果產生——例如「甘藍條件有效，無拒絕欄位。」而不是 Label 那種
	// 固定的「核對條件合不合理」。空字串代表這一步沒有敘述（該路徑還沒接上
	// narrator，或 narrator 逾時／被 guard 擋下而退回 Label）。
	//
	// ⚠️ 這個欄位是 2026-09-03 由產品決策明確要求加入的，它**推翻**了本檔
	// slotSafeArgs 原本記載的「後端只出資料不出文字」邊界，以及
	// AgentActivityRail 的 §V.10.6-3「raw step 不進對話紀錄」。原規則的動機是
	// 前端只渲染自己的固定模板；改動後前端會直接顯示這裡的自由文字，且它會
	// 隨 turn 持久化進對話紀錄。兩處原本記載舊規則的註解已於同一次改動更新，
	// 不要依據任何仍宣稱「不出文字」的敘述回推行為。
	Text       string            `json:"text,omitempty"`
	DurationMs int64             `json:"durationMs"`
	SafeArgs   map[string]string `json:"safeArgs,omitempty"`
}

// StepNarration 是「這一輪各步驟的敘述」——key 是 StepCode，value 是
// StepNarrator 產出的一句話。Compute*Steps 用它填 Step.Text；沒有對應 key 的
// 步驟 Text 留空。
type StepNarration map[StepCode]string

// slotSafeArgs 只挑出 EXTRACT_SLOTS／PICK_NEXT_QUESTION／REQUIREMENT_READY 前端
// 動態文案模板會用到的一小撮欄位，鍵集合固定、值直接來自 domain.Slots 既有的
// 結構化欄位——不帶 wire label。
//
// 這個 map 本身仍然只出資料不出文字；但「後端只出資料不出文字」已經不再是
// Step 整體的邊界——2026-09-03 起 Step.Text 會帶 LLM 產生的自由文字（見該欄位
// 的說明）。safeArgs 維持結構化，是因為前端仍有幾個固定模板要吃它，不是因為
// 那條邊界還在。
func slotSafeArgs(slots Slots) map[string]string {
	args := map[string]string{}
	if slots.Item != nil {
		args["item"] = *slots.Item
	}
	if slots.Quantity != nil {
		args["quantity"] = strconv.Itoa(*slots.Quantity)
	}
	if slots.Unit != nil {
		args["unit"] = *slots.Unit
	}
	if slots.DeliveryCity != nil {
		args["city"] = *slots.DeliveryCity
	}
	if len(args) == 0 {
		return nil
	}
	return args
}

// TurnStepTimings 是 usecase.Handle 用 5 個 time.Now() 時間點量出的 4 段真實
// 耗時（毫秒）。ComputeTurnSteps 本身不呼叫 time.Now()，時間全部用參數傳入
// ——保持純函式，單元測試餵固定數字即可，不摸真實時間、不會 flaky。
type TurnStepTimings struct {
	UnderstandMs       int64 // 冪等檢查確認為新 turn 之後 → CreateRequest/GetSlots 完成
	ExtractMs          int64 // extractor.Extract() 呼叫本身
	ValidateMs         int64 // current.Apply(patch, now()) 呼叫本身
	DecideAndComposeMs int64 // MissingRequired() 判斷 + RenderPlan() 呼叫；
	// 欄位名稱沿用舊版自然語回覆機制年代的既有命名（#2549 Phase 1 起這一段改呼叫
	// domain.RenderPlan，是純函式、無 I/O，耗時遠低於舊版可到 30 秒的 LLM 呼叫）；
	// 白名單沒有對應的「生成回覆」code，併入這一步才能讓 steps 總時長貼近真實
	// 等待時間（見
	// docs/superpowers/specs/2026-08-11-procurement-turns-steps-whitelist-design.md §2）。
}

// ComputeTurnSteps 決定 POST /turns 這一輪回傳的 steps[]（doc 55 白名單的
// clarify 階段子集）。純函式：同樣輸入必得同樣輸出，無 I/O。
//
// degraded=true 時呼叫端傳進來的 hasRejected 恆為 false（見 Handle() 那時
// 送進 Apply() 的 patch 是空的）——這個函式本身不驗證這個不變式，呼叫端
// 負責保證輸入合理。
func ComputeTurnSteps(timings TurnStepTimings, slots Slots, hasRejected, ready, degraded, cropGateEvaluated bool, narration StepNarration) []Step {
	args := slotSafeArgs(slots)
	steps := []Step{
		{Code: StepUnderstandInput, Status: stepStatusSucceeded, Label: StepLabels[StepUnderstandInput], DurationMs: timings.UnderstandMs},
		{Code: StepExtractSlots, Status: stepStatusSucceeded, Label: StepLabels[StepExtractSlots], DurationMs: timings.ExtractMs, SafeArgs: args},
	}
	if degraded {
		steps = append(steps, Step{Code: StepDegradedFallback, Status: stepStatusSucceeded, Label: StepLabels[StepDegradedFallback], DurationMs: 0})
	}
	steps = append(steps, Step{Code: StepValidateSlots, Status: stepStatusSucceeded, Label: StepLabels[StepValidateSlots], DurationMs: timings.ValidateMs})
	// crop_gate 在圖上排在 validate_slots 之後、決定下一題之前，這裡的順序跟真正
	// 的執行順序一致。條件性發：gate 沒做新判定的那些輪（品項沒變動、只是把上一輪
	// 的狀態帶著走）不該生出一格內容每輪都一樣的步驟。
	if cropGateEvaluated {
		steps = append(steps, Step{Code: StepCropGate, Status: stepStatusSucceeded, Label: StepLabels[StepCropGate], DurationMs: 0})
	}
	if hasRejected {
		steps = append(steps, Step{Code: StepSlotsRejected, Status: stepStatusSucceeded, Label: StepLabels[StepSlotsRejected], DurationMs: 0})
	}
	finalCode := StepPickNextQuestion
	if ready {
		finalCode = StepRequirementReady
	}
	steps = append(steps, Step{Code: finalCode, Status: stepStatusSucceeded, Label: StepLabels[finalCode], DurationMs: timings.DecideAndComposeMs, SafeArgs: args})
	return applyStepNarration(steps, narration)
}

// ComputeAnswerSteps 決定 POST /answers 這一輪回傳的 steps[]——比 ComputeTurnSteps
// 少 UNDERSTAND_INPUT／EXTRACT_SLOTS 兩碼：/answers 是直接寫值，沒有 LLM 抽取階段，
// 也不會 degraded（沒有 LLM 呼叫可失敗）。
func ComputeAnswerSteps(validateMs, decideMs int64, slots Slots, hasRejected, ready, cropGateEvaluated bool, narration StepNarration) []Step {
	steps := []Step{
		{Code: StepValidateSlots, Status: stepStatusSucceeded, Label: StepLabels[StepValidateSlots], DurationMs: validateMs},
	}
	if cropGateEvaluated {
		steps = append(steps, Step{Code: StepCropGate, Status: stepStatusSucceeded, Label: StepLabels[StepCropGate], DurationMs: 0})
	}
	if hasRejected {
		steps = append(steps, Step{Code: StepSlotsRejected, Status: stepStatusSucceeded, Label: StepLabels[StepSlotsRejected], DurationMs: 0})
	}
	finalCode := StepPickNextQuestion
	if ready {
		finalCode = StepRequirementReady
	}
	steps = append(steps, Step{Code: finalCode, Status: stepStatusSucceeded, Label: StepLabels[finalCode], DurationMs: decideMs, SafeArgs: slotSafeArgs(slots)})
	return applyStepNarration(steps, narration)
}

// ComputeRewindSteps 決定 POST /rewinds 這一輪回傳的 steps[]——rewind 是讀取既有
// target 版本的內容、不驗證新值，所以沒有 VALIDATE_SLOTS。但它確實會跑 crop
// gate（rewind.go:122——還原到舊版 slots 後，舊的解析結果描述的是 rewind 前那個
// 品項，必須重新判定），gate 真的做過判定時就補上 CROP_GATE 這一碼。
func ComputeRewindSteps(decideMs int64, slots Slots, ready, cropGateEvaluated bool, narration StepNarration) []Step {
	finalCode := StepPickNextQuestion
	if ready {
		finalCode = StepRequirementReady
	}
	steps := []Step{}
	if cropGateEvaluated {
		steps = append(steps, Step{Code: StepCropGate, Status: stepStatusSucceeded, Label: StepLabels[StepCropGate], DurationMs: 0})
	}
	steps = append(steps, Step{Code: finalCode, Status: stepStatusSucceeded, Label: StepLabels[finalCode], DurationMs: decideMs, SafeArgs: slotSafeArgs(slots)})
	return applyStepNarration(steps, narration)
}

// applyStepNarration 把 narration 的句子填進對應 code 的 Step.Text。三支
// Compute*Steps 共用，確保「有敘述就一定填得進去」不必各自記得做一次。
// narration 為 nil（該路徑還沒接 narrator）時原樣回傳，Text 全空。
func applyStepNarration(steps []Step, narration StepNarration) []Step {
	if len(narration) == 0 {
		return steps
	}
	for i := range steps {
		if text, ok := narration[steps[i].Code]; ok {
			steps[i].Text = text
		}
	}
	return steps
}

// ---- Activity rail 的前瞻步驟清單（T23）----

// RailRow 是 Activity rail 上一列的靜態形狀：「這個 run 會做哪一步、它掛在
// 哪一步底下」。刻意不含 rowId 與 status——前者由 usecase 在 run 當下鑄造
// （每次執行都是新的一批 opaque id），後者隨執行推進而變。
type RailRow struct {
	Code StepCode
	// Parent 空字串代表 top-level 列；非空時這一列是該 code 的子列
	// （設計稿 01 的巢狀 lane 子列）。目前只支援一層巢狀——rail 本身就是
	// 給人一眼掃過的進度列，再深一層就不是進度而是 trace 了。
	Parent StepCode
}

// CandidateSearchRailPlan 是候選搜尋 run 一開始就能誠實宣告的完整步驟清單，
// 順序即顯示順序。usecase 在 run 起頭用它送出一批 status=PENDING 的列，讓
// 買家看得到「還沒跑到的步驟」（設計稿 01 的「等待中」），之後每一步的真實
// 事件再就地更新對應的列。
//
// 這件事只對**固定 pipeline** 成立：候選搜尋的 run() 從頭到尾就是 claim →
// 兩條 lane 併發 → finalize，開跑前就知道會做哪幾步。LLM controller 的 turn
// run 做不到——工具是模型逐步決定的，預先宣告清單等於預言模型的選擇——所以
// 那條 run 不套用這份機制，而設計稿也只在候選搜尋這一段畫「等待中」。
//
// 純函式、無狀態：同樣的輸入（沒有輸入）永遠回同一份清單，測試直接比對。
func CandidateSearchRailPlan() []RailRow {
	return []RailRow{
		{Code: StepSearchCandidates},
		{Code: StepSearchPlatform, Parent: StepSearchCandidates},
		{Code: StepSearchExternal, Parent: StepSearchCandidates},
		// SUMMARIZE 對應 FinalizeCandidateSet——把兩條 lane 的結果寫成這一批
		// 候選的終態（設計稿 01 的「整理候選清單」）。沿用既有 code 與既有
		// label「整理成給你的建議」，不為了貼合稿上的字面另開一碼：StepLabels
		// 的字面值是 issue #2343 定案後凍結的，用詞差異不值得動那張表。
		{Code: StepSummarize},
	}
}
