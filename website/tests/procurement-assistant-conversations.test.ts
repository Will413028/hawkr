import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { stepIcon, stepLabel } from "../features/procurement-assistant/steps";

const root = fileURLToPath(new URL("..", import.meta.url));
const hook = readFileSync(
	path.join(root, "features/procurement-assistant/useAgentConversations.ts"),
	"utf8",
);
const keys = readFileSync(path.join(root, "lib/query/keys.ts"), "utf8");

/* ── steps.ts 純函式 ─────────────────────────────────────────── */

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
