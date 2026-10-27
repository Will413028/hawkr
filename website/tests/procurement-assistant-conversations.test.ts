import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	normalizeSteps,
	stepIcon,
	stepLabel,
} from "../features/procurement-assistant/steps";

const root = fileURLToPath(new URL("..", import.meta.url));
const repo = readFileSync(
	path.join(root, "features/procurement-assistant/conversations.ts"),
	"utf8",
);
const hook = readFileSync(
	path.join(root, "features/procurement-assistant/useAgentConversations.ts"),
	"utf8",
);
const keys = readFileSync(path.join(root, "lib/query/keys.ts"), "utf8");

/* ── steps.ts 純函式 ─────────────────────────────────────────── */

test("normalizeSteps 對未知 code、wire label 與未知 status 一律 fail closed", () => {
	assert.deepEqual(normalizeSteps(undefined), []);
	assert.deepEqual(normalizeSteps("garbage"), []);
	assert.deepEqual(normalizeSteps([{ status: "SUCCEEDED" }]), []);
	assert.deepEqual(
		normalizeSteps([
			{
				code: "RAW_PRIVATE_TOOL",
				status: "RUNNING",
				label: "raw internal tool payload",
			},
		]),
		[],
	);
	assert.deepEqual(
		normalizeSteps([
			{
				code: "EXTRACT_SLOTS",
				status: "WEIRD",
				label: "raw internal tool payload",
				durationMs: "x",
			},
		]),
		[],
	);
});

test("stepIcon 未知碼降級 clock；stepLabel 只使用固定公開字串表", () => {
	assert.equal(stepIcon("NOT_A_CODE"), "clock");
	assert.equal(stepIcon("SEARCH_PLATFORM"), "search");
	assert.equal(
		stepLabel({ code: "EXTRACT_SLOTS", label: undefined }),
		"抓出品項、數量和交期",
	);
	assert.equal(stepLabel({ code: "X_UNKNOWN", label: undefined }), "處理中");
	assert.equal(
		stepLabel({ code: "EXTRACT_SLOTS", label: "raw internal tool payload" }),
		"抓出品項、數量和交期",
	);
});

test("normalizeSteps 對 safeArgs 一樣 fail closed：非物件或含非字串值一律整欄丟棄", () => {
	assert.deepEqual(
		normalizeSteps([
			{ code: "EXTRACT_SLOTS", status: "SUCCEEDED", safeArgs: "not-an-object" },
		])[0].safeArgs,
		undefined,
	);
	assert.deepEqual(
		normalizeSteps([
			{
				code: "EXTRACT_SLOTS",
				status: "SUCCEEDED",
				safeArgs: { item: "香蕉", quantity: 20 },
			},
		])[0].safeArgs,
		undefined,
	);
	assert.deepEqual(
		normalizeSteps([
			{
				code: "EXTRACT_SLOTS",
				status: "SUCCEEDED",
				safeArgs: { item: "香蕉", city: "台北市" },
			},
		])[0].safeArgs,
		{ item: "香蕉", city: "台北市" },
	);
});

test("stepLabel 用 safeArgs 產生動態文案，只限 EXTRACT_SLOTS／PICK_NEXT_QUESTION／REQUIREMENT_READY 三碼", () => {
	assert.equal(
		stepLabel({
			code: "EXTRACT_SLOTS",
			safeArgs: { item: "香蕉", city: "台北市" },
		}),
		"抓到你要送到【台北市】的【香蕉】",
	);
	assert.equal(
		stepLabel({
			code: "REQUIREMENT_READY",
			safeArgs: { item: "香蕉", city: "台北市" },
		}),
		"條件都齊了：【香蕉】送到【台北市】",
	);
	// safeArgs 缺 item（動態模板必要欄位）時退回既有固定文案，不得拼出殘句。
	assert.equal(
		stepLabel({ code: "EXTRACT_SLOTS", safeArgs: { city: "台北市" } }),
		"抓出品項、數量和交期",
	);
	assert.equal(
		stepLabel({ code: "EXTRACT_SLOTS", safeArgs: undefined }),
		"抓出品項、數量和交期",
	);
	// VALIDATE_SLOTS 沒有動態模板，即使帶了 safeArgs 也必須維持固定文案。
	assert.equal(
		stepLabel({ code: "VALIDATE_SLOTS", safeArgs: { item: "香蕉" } }),
		"核對條件合不合理",
	);
});

test("列表走 restock canonical：snake query（cursor_id/page_size）＋ camelCase body", () => {
	assert.match(repo, /cursor_id/);
	assert.match(repo, /page_size/);
	assert.match(repo, /nextCursorId/);
	assert.match(repo, /hasMore/);
});

test("端點未部署（404/405）回 available:false 空頁——優雅降級不是錯誤", () => {
	assert.match(repo, /err\.status === 404 \|\| err\.status === 405/);
	assert.match(repo, /available: false/);
});

test("端點不存在只探一次：後續呼叫短路，不再每次掛載都留一個 404 在 console", () => {
	assert.match(repo, /let endpointAvailable: boolean \| null = null;/);
	assert.match(
		repo,
		/if \(endpointAvailable === false\) return UNAVAILABLE_PAGE;/,
	);
	assert.match(repo, /endpointAvailable = false;/);
});

test("5xx／網路錯誤不得被當成「端點不存在」——下次仍要重試", () => {
	// throw 在 isEndpointMissing 分支之外，且該路徑不寫 endpointAvailable。
	const catchBlock = repo.slice(
		repo.indexOf("} catch (err) {"),
		repo.indexOf("/** 測試用：重置端點探測結果。 */"),
	);
	assert.match(catchBlock, /throw err;/);
	const beforeThrow = catchBlock.slice(0, catchBlock.indexOf("throw err;"));
	assert.equal(
		(beforeThrow.match(/endpointAvailable = false/g) ?? []).length,
		1,
		"只有 404/405 分支可以把端點標記為不存在",
	);
});

test("改名帶 clientRequestId 冪等鍵（房規：無 Idempotency-Key header）", () => {
	assert.match(repo, /clientRequestId: crypto\.randomUUID\(\)/);
	assert.doesNotMatch(repo, /Idempotency-Key/);
});

/* ── dev 實際 wire 契約（handler/conversation.go 手組的 gin.H）──────────
 * primaryCategoryId 是 additive nullable 欄位；conversationId 仍是唯一必填，
 * 避免 legacy row 沒分類時整列被前端丟掉。 */

test("列表只保證 conversationId——缺 requestId 不得整份丟掉（否則側欄全空）", () => {
	// GetConversations 的 legacy record 沒有 requestId，也可能沒有 primaryCategoryId。
	assert.match(repo, /if \(!conversationId\) return null;/);
	assert.doesNotMatch(repo, /!conversationId \|\| !requestId/);
	assert.doesNotMatch(repo, /titleSource/);
	assert.doesNotMatch(repo, /missingRequiredCount/);
});

test("transcript 的欄位名是 id（不是 turnId），role 是小寫且大小寫不敏感", () => {
	assert.match(repo, /id: str\(r\.id\) \|\| crypto\.randomUUID\(\)/);
	assert.match(repo, /str\(r\.role\)\.toLowerCase\(\) === "buyer"/);
	assert.doesNotMatch(repo, /r\.role === "BUYER"/);
});

test("transcript 會還原後端 recordedThisTurn，重開對話仍看得到已記下的條件", () => {
	assert.match(repo, /recordedThisTurn\?: RecordedField\[\]/);
	assert.match(
		repo,
		/recordedThisTurn: normalizeRecordedFields\(r\.recordedThisTurn\)/,
	);
});

test("messages 端點不分頁——不得沿游標追頁（那些參數後端根本不吃）", () => {
	const fn = repo.slice(
		repo.indexOf("export async function fetchAgentTranscript"),
	);
	// 走訪 records 的 for...of 沒問題；不得有游標／頁碼的追頁機制。
	assert.doesNotMatch(fn, /cursor_id/);
	assert.doesNotMatch(fn, /nextCursorId/);
	assert.doesNotMatch(fn, /pageIndex|MAX_PAGES/);
	assert.doesNotMatch(fn, /page_size/);
});

test("軟刪要帶 body 的 clientRequestId——缺了後端回 400 invalid request body", () => {
	const fn = repo.slice(
		repo.indexOf("export async function deleteAgentConversation"),
	);
	assert.match(fn, /json: \{ clientRequestId: crypto\.randomUUID\(\) \}/);
});

/* ── useAgentConversations hook 慣例 ────────────────────────────── */

test("query key 走 identity-scoped factory；用 placeholderData 而非 initialData", () => {
	assert.match(hook, /queryKeys\.assistant\.conversations\(queryIdentity\)/);
	assert.match(hook, /placeholderData: EMPTY_PAGE/);
	assert.doesNotMatch(hook, /initialData\s*:/);
	assert.match(keys, /"assistant", \.\.\.idPrefix\(id\), "conversations"/);
	assert.match(
		hook,
		/query\.isFetching && query\.isPlaceholderData/,
		"placeholder 第一次仍在 fetch 時必須維持 loading，不能先顯示降級空態",
	);
});

test("改名／刪除是樂觀更新＋失敗還原，成功後 invalidate 最小 key", () => {
	assert.match(hook, /setQueryData<AgentConversationPage>/);
	assert.match(hook, /queryClient\.setQueryData\(queryKey, previous\)/);
	assert.match(hook, /invalidateQueries/);
});
