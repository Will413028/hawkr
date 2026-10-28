import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const source = readFileSync(
	path.join(root, "features/procurement-assistant/ProcurementClarifyChat.tsx"),
	"utf8",
);
const rail = readFileSync(
	path.join(root, "features/procurement-assistant/AgentActivityRail.tsx"),
	"utf8",
);
const taskPanel = readFileSync(
	path.join(root, "features/procurement-assistant/AgentTaskPanel.tsx"),
	"utf8",
);
const steps = readFileSync(
	path.join(root, "features/procurement-assistant/steps.ts"),
	"utf8",
);
const css = readFileSync(
	path.join(root, "styles/features/procurement-assistant.css"),
	"utf8",
);

test("原始模型推理展示已下架（doc 55 §8.1 上線阻斷項）——不得再渲染 reasoning 或「查看思考過程」", () => {
	assert.doesNotMatch(source, /查看思考過程/);
	assert.doesNotMatch(source, /turn\.reasoning/);
});

test("同一時間只有一個即時處理 surface，完成後沒有快照或回放", () => {
	// 泡泡裡不得再有收合的步驟摘要；元件本身也已下架。
	assert.doesNotMatch(source, /AgentStepSummary/);
	// 沒有回放路徑：沒有 replayLines／replayTurn／revealed 集合。
	assert.doesNotMatch(source, /replay/i);
	assert.doesNotMatch(source, /revealed/);
	// 也不留判斷「這輪是不是串流來的」的旗標——兩條路徑落地行為一致。
	assert.doesNotMatch(source, /streamed/);
	// 主對話區只有一個處理 slot；正式對話不另外渲染相同進度的一行摘要。
	assert.match(source, /processing\?: React\.ReactNode/);
	assert.match(
		source,
		/\{pending && processing \? \(\s*<li className="hawkr-agent-processing-row">\{processing\}<\/li>\s*\) : pending \? \(/,
	);
	assert.doesNotMatch(source, /AgentActivityRail/);
	// 即時 surface 只存在於 pending turn 或 searching，完成後直接移除。
	assert.match(
		taskPanel,
		/if \(stage !== "searching" && !pending\) return null;/,
	);
	assert.doesNotMatch(taskPanel, /RailSnapshot|snapshotRail|setSnapshot/);
	assert.doesNotMatch(source, /turn\.steps/);
});

test("思考步驟只存在於 Live Activity Rail，不借用 facts pill 變成完成後氣泡", () => {
	assert.doesNotMatch(source, /turn\.steps/);
	assert.doesNotMatch(source, /hawkr-agent-message__steps/);
	assert.doesNotMatch(css, /\.hawkr-agent-message__steps/);
	// 後端 guard 過的逐步敘述仍可在 pending rail 顯示。
	assert.match(rail, /stepLabel\(step\)/);
});

test("allowlisted public steps 逐列呈現，並把完整 liveSteps 交給狀態軌", () => {
	// normalizeSteps 已先 fail closed；這裡把完整 liveSteps 交給狀態軌，保留
	// §V.10.6-3 所需的 status。
	assert.match(taskPanel, /<AgentActivityRail[\s\S]*?steps=\{liveSteps\}/);
	assert.match(rail, /data-status=\{step\.status\}/);
	// 停留多久由下一個 step 何時抵達決定，前端不排程。
	assert.doesNotMatch(source, /holdMs/);
	assert.doesNotMatch(rail, /holdMs/);
	assert.match(source, /<CropDisambiguationCard[\s\S]*?<ClarifyQuestionCard/);
});

test("onSkipOption 未傳時不得讓 ClarifyQuestionCard 長出跳過按鈕——傳給 onSkip 的必須是條件式 undefined，不是恆真的 wrapper function", () => {
	assert.match(
		source,
		/onSkipOption\?: \(payload: \{ slotKey: string; skip: true \}\) => void;/,
	);
	assert.match(source, /onSkip=\{\s*onSkipOption\s*\?/);
});

test("跳過跟點 chip 一樣算『用按鈕作答』，下一題掛載要把焦點還給第一顆選項", () => {
	assert.match(
		source,
		/answeredViaChipRef\.current = true;\s*onSkipOption\(payload\);/,
	);
});

test("狀態軌只在等待中渲染；問題卡在等待結束後才出現", () => {
	// legacy（liveSteps）與 graph 路徑（liveActivity）任一有內容都要開閘——
	// 兩條路徑互斥，但閘門本身得認得兩種輪子。
	assert.match(
		taskPanel,
		/const hasLiveRows =\s*liveSteps\.length > 0 \|\| liveActivity\.length > 0;/,
	);
	assert.match(
		taskPanel,
		/<AgentActivityRail[\s\S]*?steps=\{liveSteps\}[\s\S]*?activity=\{liveActivity\}[\s\S]*?motion=\{motion\}[\s\S]*?\/>/,
	);
	// graph 路徑的列身分鍵是後端指定的 rowId，不是（clientRequestId, code）。
	// T23 起分成 top-level 列與 lane 子列兩層，兩層都必須用後端的 rowId 當 key。
	assert.match(rail, /key=\{row\.rowId\}/);
	assert.match(rail, /key=\{child\.rowId\}/);
	assert.match(source, /!pending && question && onPickOption/);
	assert.doesNotMatch(source, /onFreeText={focusComposer}/);
});

test("狀態軌不自己計時——節奏是伺服器的，不得出現前進用的 timer", () => {
	assert.doesNotMatch(rail, /setTimeout/);
	assert.doesNotMatch(rail, /onComplete/);
});

test("設計書 §V.10.6-1：收到幾個 step 就顯示幾列，不是只留最新一行", () => {
	// 舊版只渲染 lines.at(-1)，前面做過什麼一換行就消失。§V.10.6-1 要求
	// 「收到幾個不同 step 就顯示幾列，不設固定兩列上限」。
	assert.match(rail, /steps\.map\(\(step\) =>/);
	assert.doesNotMatch(rail, /\.at\(-1\)/);
	// Slice A 的列身分鍵是（clientRequestId, code）——liveSteps 已按 code 收斂，
	// 所以 heartbeat 會 upsert 原列而不是長出重複的一行。
	assert.match(rail, /key=\{step\.code\}/);
	// §V.10.6-3：step 完成只完成那一列，狀態掛在該列上。
	assert.match(rail, /data-status=\{step\.status\}/);
});

test("尚未收到公開步驟時中性等待，真實步驟到達後呈現完整清單", () => {
	assert.match(taskPanel, /\{hasLiveRows \? \(/);
	assert.match(taskPanel, /className="hawkr-agent-processing__waiting"/);
	assert.match(taskPanel, /等待回應…/);
	assert.doesNotMatch(taskPanel, /setTimeout|setInterval|holdMs/);
});

test("§V.10.10-2：尚未 ACK 的等待列只能用不循環的 SUBMITTING，不得靠 motion 猜", () => {
	// 無列等待分支沿用 canonical Sprite 的 SUBMITTING；不把整個 run 的動態
	// 表情套到尚未收到權威事件的等待狀態。
	const waiting = taskPanel.slice(
		taskPanel.indexOf('className="hawkr-agent-processing__waiting"'),
	);
	assert.match(waiting, /<HawkrAgentSprite[\s\S]*?state="SUBMITTING"/);
	assert.doesNotMatch(waiting, /state=\{motion\}/);
});

test("Activity Rail 的長句在窄畫面完整換行，不用 ellipsis 截掉思考內容", () => {
	const rowTextRule = css.match(/\.hawkr-agent-rail__copy \{[\s\S]*?\n\}/)?.[0];
	assert.ok(rowTextRule);
	assert.match(rail, /className="hawkr-agent-rail__copy"/);
	assert.match(rowTextRule, /overflow-wrap: anywhere;/);
	assert.match(rowTextRule, /white-space: normal;/);
	assert.doesNotMatch(rowTextRule, /text-overflow:\s*ellipsis/);
});

test("composer 字數超限與欄位驗證都連到同一個可讀錯誤", () => {
	assert.match(
		source,
		/const hasError = overLimit \|\| Boolean\(validationError\)/,
	);
	assert.match(source, /aria-invalid=\{hasError \? "true" : undefined\}/);
	assert.match(
		source,
		/aria-describedby=\{\s*hasError \? "procurement-clarify-error" : undefined\s*\}/,
	);
});

test("問題卡送出鍵維持 44px 觸控尺寸，並和輸入框右端保持平行弧線", () => {
	assert.match(
		css,
		/\.hawkr-agent-question-card__free-text-send\.thread-page__send \{[\s\S]*?width: 44px;[\s\S]*?height: 44px;[\s\S]*?flex: 0 0 44px;/,
	);
	assert.match(
		css,
		/\.hawkr-agent-question-card__free-text-form:not\([\s\S]*?padding-right: 1px;/,
	);
});

test("伺服器 label 缺席時的備援字串表守 brand voice：動詞開頭，不是「正在X」", () => {
	const labels = [...steps.matchAll(/^\t([A-Z_]+): "([^"]+)",$/gm)].map(
		(m) => m[2],
	);
	assert.ok(
		labels.length >= 15,
		`expected the label table, saw ${labels.length}`,
	);
	const stateStyle = labels.filter((l) => l.startsWith("正在"));
	assert.deepEqual(
		stateStyle,
		[],
		`brand voice 規則 3（動詞開頭）：這些還是狀態描述 ${stateStyle.join("／")}`,
	);
});

test("狀態軌對 screen reader 隱藏——逐行冒出不得進 aria-live", () => {
	// 播報由 workspace 的 atomic polite status 負責，只播當下這一步一次
	//（§V.11.0 Accessibility：視覺列表本身不設 live）。
	assert.match(rail, /aria-hidden="true"/);
	assert.doesNotMatch(rail, /aria-live/);
});

test("未知步驟碼降級成 clock icon＋原樣 label（forward-compat）", () => {
	assert.match(steps, /STEP_ICON\[code\] \?\? "clock"/);
});

test("問題卡不是需求快照卡的復活——不得使用 assistant-snapshot class", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.doesNotMatch(questionCard, /assistant-snapshot/);
});

test("當前問題卡的小精靈是靜態 Hawkr 標誌，不播放動畫也不冒充仍在處理中的狀態", () => {
	const mark = readFileSync(
		path.join(root, "features/procurement-assistant/HawkrAgentMark.tsx"),
		"utf8",
	);
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	const cropCard = readFileSync(
		path.join(
			root,
			"features/procurement-assistant/CropDisambiguationCard.tsx",
		),
		"utf8",
	);

	assert.match(mark, /<BrandLogo variant="mark" size="fill" \/>/);
	assert.doesNotMatch(mark, /animateOnce|pending|autoplay/);

	for (const [name, card] of [
		["clarify", questionCard],
		["crop-disambiguation", cropCard],
	] as const) {
		const avatar = card.match(/<HawkrAgentMark[\s\S]*?\/>/)?.[0];
		assert.ok(avatar, `${name} question card must render the Agent avatar`);
		assert.match(avatar, /label="Hawkr Agent"/);
		assert.doesNotMatch(avatar, /pending=|animateOnce/);
	}
});

test("跳過按鈕：無選項的自由輸入型問題卡（多數選填欄位屬此類）也要能顯示，不能被舊的 options.length>0 gate 卡住", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.match(
		questionCard,
		/question\.options\.length > 0 \|\|\s*question\.allowFreeText \|\|\s*onSkip/,
	);
});

test("跳過按鈕：帶 slotKey 與 skip:true 回呼給呼叫端，pending 時 disabled", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.match(
		questionCard,
		/onClick=\{\(\) =>\s*onSkip\(\{ slotKey: question\.slotKey, skip: true \}\)\s*\}/,
	);
	assert.match(questionCard, />\s*跳過\s*</);
});

test("買家訊息使用登入身分的大頭貼；真實圖片缺席時才走既有 buyer fallback", () => {
	assert.match(source, /src=\{identity\?\.avatarKey\}/);
	assert.match(source, /name=\{buyerName\}/);
	assert.match(source, /entity="buyer"/);
	assert.match(
		source,
		/className="hawkr-agent-message-avatar hawkr-agent-message-avatar--buyer"/,
	);
	assert.doesNotMatch(source, /turn\.role === "buyer" \? "你"/);
});

test("無固定選項的問題卡提供可輸入欄位，並以 slotKey + freeText 走 answers 回呼", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.match(questionCard, /question\.options\.length === 0/);
	assert.match(
		questionCard,
		/\(showCustomAnswer \|\| canRetryCropDescription\)\s*&&\s*question\.allowFreeText/,
	);
	assert.match(questionCard, /placeholder=\{inputHint\.placeholder\}/);
	assert.match(questionCard, /maxLength=\{200\}/);
	assert.match(
		questionCard,
		/validateFreeTextAnswer\(question\.slotKey, answer\)/,
	);
	assert.match(
		questionCard,
		/canRetryCropDescription \? "重新搜尋品項" : "送出答案"/,
	);
	assert.match(
		source,
		/onSubmitFreeText\(\{ slotKey: question\.slotKey, freeText \}\)/,
	);
});

test("「其他…」按鈕維持只在有選項時出現——放寬跳過按鈕的 gate 不能連帶讓純自由輸入題多長出這顆按鈕", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.match(
		questionCard,
		/question\.options\.length > 0\s*&&\s*question\.allowFreeText\s*&&\s*!isCropDisambiguation/,
	);
	assert.match(questionCard, /onClick=\{openCustomAnswer\}/);
	assert.match(questionCard, /aria-expanded=\{showCustomAnswer\}/);
	assert.doesNotMatch(questionCard, /onFreeText/);
});

test("後端 rejected 欄位用固定原因就近呈現，不把是否說明錯誤交給 LLM reply 運氣", () => {
	assert.match(source, /turn\.rejectedThisTurn/);
	assert.match(source, /有些條件需要重新確認/);
	assert.match(source, /slotLabel\(field\.key\)/);
});

test("已記下的條件以不可互動的霧面摘要呈現，snake/camel key 使用相同語意圖示", () => {
	for (const [key, icon] of [
		["item", "goods"],
		["quantity", "cubes"],
		["unit", "reorder"],
		["packaging", "box"],
		["budgetPerUnit", "wallet"],
		["budget_per_unit", "wallet"],
		["deliveryCity", "location"],
		["delivery_city", "location"],
		["deliveryDate", "calendar"],
		["delivery_date", "calendar"],
		["exactMatchRequired", "scan"],
		["exact_match_required", "scan"],
	] as const) {
		assert.match(source, new RegExp(`${key}: "${icon}"`));
	}
	assert.match(source, /RECORDED_FIELD_ICON\[key\] \?\? "paper"/);
	assert.match(source, /className="hawkr-agent-message__fact-icon"/);
	assert.match(source, /aria-hidden="true"/);
	assert.match(source, /data-slot-key=\{field\.key\}/);
	assert.doesNotMatch(
		source,
		/<button[^>]*className="hawkr-agent-message__fact/,
	);
	assert.match(
		css,
		/\.hawkr-agent-message__facts li \{[\s\S]*?min-width: 0;[\s\S]*?max-width: 100%;[\s\S]*?background: var\(--hkui-content1\);[\s\S]*?backdrop-filter: blur\(14px\)/,
	);
	assert.match(
		css,
		/\.hawkr-agent-message__fact-icon \{[\s\S]*?flex: 0 0 30px;[\s\S]*?width: 30px;[\s\S]*?height: 30px;[\s\S]*?border-radius: var\(--radius-circle\);/,
	);
	assert.match(
		css,
		/\.hawkr-agent-message__fact-value \{[\s\S]*?font-size: var\(--hkui-text-small, 0\.875rem\);[\s\S]*?overflow-wrap: anywhere;/,
	);
	assert.match(
		css,
		/\.hawkr-agent-message__fact-label \{[\s\S]*?min-width: 0;[\s\S]*?flex: 0 1 auto;[\s\S]*?overflow-wrap: anywhere;/,
	);
	assert.match(
		css,
		/@media \(prefers-reduced-transparency: reduce\) \{[\s\S]*?\.hawkr-agent-message__facts li \{[\s\S]*?background: var\(--hkui-content1\);[\s\S]*?backdrop-filter: none;/,
	);
});

test("草稿是受控的，composer 自己不持有 draft state——entry 與 thread 才可能是同一個 composer（設計書 v6 §V.11.0）", () => {
	// 舊版把 draft 放在 composer 內部的 useState，再靠 initialDraft 從外面塞值
	// 進去。那個結構撐不住 §V.11.0 的兩條要求：入口收合時元件換位置會重掛
	//（草稿蒸發），而且送出後立刻清空會讓首送失敗的 prompt 消失。
	assert.match(
		source,
		/value: string;\s*onValueChange: \(next: string\) => void;/,
	);
	assert.doesNotMatch(source, /const \[draft, setDraft\] = React\.useState/);
	assert.doesNotMatch(source, /initialDraft/);
	// 送出後不由 composer 自己清空——清空時機屬於持有草稿的父層。
	assert.match(source, /setValidationError\(null\);\s*onSubmit\(message\);/);
	// 外部可拿到 textarea ref，entry 的 chips／起手卡才能把焦點移進來。
	assert.match(source, /const textareaRef = inputRef \?\? fallbackRef;/);
});

test("onPickUnit 走跟 onPick/onSkip 一樣的『用按鈕作答』焦點回歸規則，往下傳給 ClarifyQuestionCard", () => {
	assert.match(
		source,
		/onPickUnit\?: \(payload: \{\s*slotKey: string;\s*unitOptionId: string;\s*unitLabel: string;\s*freeText\?: string;\s*\}\) => void;/,
	);
	assert.match(
		source,
		/answeredViaChipRef\.current = true;\s*onPickUnit\(payload\);/,
	);
});

test("quantity 複合問題卡：單位 chip 在數字欄驗證通過前 disabled，點擊送出 freeText+unitOptionId，不覆用 onPick 的 optionId 語意", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.match(
		questionCard,
		/import \{ questionHasUnitOptions \} from "\.\/session-logic";/,
	);
	assert.match(
		questionCard,
		/const hasUnitOptions = questionHasUnitOptions\(question\);/,
	);
	assert.match(
		questionCard,
		/isCompoundQuantity = hasUnitOptions && question\.allowFreeText/,
	);
	assert.match(questionCard, /disabled=\{pending \|\| !quantityReady\}/);
	assert.match(
		questionCard,
		/onPickUnit\(\{\s*slotKey: question\.slotKey,\s*unitOptionId: option\.optionId,\s*unitLabel: option\.label,/,
	);
	// 複合問題卡不透過 submitFreeText 送出——只有點單位 chip 才送出一次；
	// 沒有獨立的「送出答案」按鈕，Enter 也不觸發送出。
	assert.match(questionCard, /if \(isCompoundQuantity\) return;/);
});

test("quantity 單位追問：allowFreeText:false 沿用既有 gate 隱藏數字欄，只渲染單位 chips", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.match(
		questionCard,
		/isUnitFollowUp = hasUnitOptions && !question\.allowFreeText/,
	);
	assert.match(
		questionCard,
		/question\.options\.length > 0 \|\|\s*question\.allowFreeText \|\|\s*onSkip \|\|\s*hasUnitOptions/,
	);
});

test("品項分類消歧 gate：D「其他」chip 不直接 onPick，而是展開本地子表單", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.match(
		questionCard,
		/const CROP_DISAMBIGUATION_SLOT_KEY = "crop_disambiguation";/,
	);
	assert.match(questionCard, /const CROP_OTHER_OPTION_ID = "crop-other";/);
	// D chip 的 onClick 先判斷是不是 crop-other，是的話只開表單、不呼叫 onPick。
	assert.match(
		questionCard,
		/if \(isCropOtherOption\) \{\s*openCropOtherForm\(\);\s*return;\s*\}\s*onPick\(\{/,
	);
	// 真候選（非 crop-other）維持原本直接 onPick 的行為，沒有被這個判斷擋住。
	assert.match(
		questionCard,
		/isCropOtherOption =\s*isCropDisambiguation &&\s*option\.optionId === CROP_OTHER_OPTION_ID;/,
	);
});

test("品項分類消歧 gate：D 子表單描述填了才能送出，未傳 onSubmitCropOther 時送出鍵也 disabled", () => {
	const questionCard = readFileSync(
		path.join(root, "features/procurement-assistant/ClarifyQuestionCard.tsx"),
		"utf8",
	);
	assert.match(
		questionCard,
		/const canSubmitCropOther =\s*isCropDisambiguation &&\s*Boolean\(cropOtherDescription\.trim\(\)\) &&\s*!pending;/,
	);
	assert.match(
		questionCard,
		/const submitCropOther = \(\) => \{\s*if \(!canSubmitCropOther \|\| !onSubmitCropOther\) return;\s*onSubmitCropOther\(\{\s*description: cropOtherDescription\.trim\(\),\s*\}\);/,
	);
	assert.match(
		questionCard,
		/disabled=\{!canSubmitCropOther \|\| !onSubmitCropOther\}/,
	);
	// 送出後回到未展開狀態並清空欄位，跟其他問題卡的自由文字表單同一套收尾慣例。
	assert.match(
		questionCard,
		/setShowCropOtherForm\(false\);\s*setCropOtherDescription\(""\);/,
	);
});

test("品項分類消歧 gate：onSubmitCropOther 沿用 onSubmitFreeText 同一種『在 ProcurementClarifyChat 補上 slotKey 再往下傳』慣例，不新造一條平行資料流", () => {
	assert.match(
		source,
		/onSubmitCropOther\?: \(payload: \{\s*slotKey: string;\s*description: string;\s*\}\) => void;/,
	);
	assert.match(
		source,
		/onSubmitCropOther=\{\s*onSubmitCropOther\s*\?\s*\(payload\) =>\s*onSubmitCropOther\(\{\s*slotKey: question\.slotKey,\s*description: payload\.description,\s*\}\)\s*:\s*undefined\s*\}/,
	);
});
