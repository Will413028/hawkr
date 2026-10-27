import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const session = readFileSync(
	path.join(root, "features/procurement-assistant/useProcurementSession.ts"),
	"utf8",
);
const hook = readFileSync(
	path.join(root, "features/procurement-assistant/useAgentConversations.ts"),
	"utf8",
);
const sidebar = readFileSync(
	path.join(root, "features/messages/components/ConversationSidebar.tsx"),
	"utf8",
);
const keys = readFileSync(path.join(root, "lib/query/keys.ts"), "utf8");
const layout = readFileSync(
	path.join(root, "app/[locale]/(account)/account/messages/layout.tsx"),
	"utf8",
);

/* ── 側欄切對話要瞬切 ────────────────────────────────────────────
 * 實測（Playwright，transcript 250ms／snapshot 120ms 的 mock 延遲）：
 *   改之前：切換 502ms 才有內容，中間閃約 370ms 全版骨架
 *   改之後：切回看過的 88ms／0 骨架；沒看過但滑過停 450ms 的 103ms／0 骨架
 * 右側 workspace 因 Next 路由參數變更一定 remount（側欄不會），所以關鍵是
 * 「掛載的第一個 frame 就能同步讀到快取」。 */

test("transcript／snapshot 的 query key 是 identity-scoped（切帳號不得沿用他人對話）", () => {
	assert.match(
		keys,
		/"assistant", \.\.\.idPrefix\(id\), "transcript", conversationId/,
	);
	assert.match(
		keys,
		/"assistant", \.\.\.idPrefix\(id\), "snapshot", requestId/,
	);
});

test("掛載時同步讀快取，命中就不進骨架態", () => {
	// 純函式，同步讀 cache，兩份都要有才算命中。
	assert.match(session, /function readCachedRestore\(/);
	assert.match(session, /queryClient\.getQueryData<AgentTranscript>\(/);
	assert.match(session, /queryClient\.getQueryData<RawSnapshot>\(/);
	assert.match(session, /if \(!snapshot\) return null;/);
	// state 的 lazy initializer 直接套用 → 第一個 frame 就有內容。
	assert.match(session, /const cached = readCachedRestore\(/);
	// restoring 由「有沒有 requestId」推導，而不是無條件 true。
	assert.match(session, /state\.requestId === null &&/);
	assert.doesNotMatch(
		session,
		/React\.useState\(\s*Boolean\(initialRequestId \|\| initialConversationId\)/,
	);
});

test("同步路徑與 effect 路徑共用同一支組裝函式（不得各寫一份）", () => {
	assert.match(session, /function restoredStateFrom\(/);
	// 兩個呼叫點：readCachedRestore 與還原 effect。
	assert.equal(
		(session.match(/restoredStateFrom\(/g) ?? []).length,
		3,
		"應為 1 個定義 + 2 個呼叫點",
	);
});

test("snapshot 一律回後端拿（stage 的權威來源），transcript 才吃 staleTime", () => {
	const snapshotFn = session.slice(session.indexOf("const fetchSnapshot ="));
	assert.match(snapshotFn.slice(0, 700), /staleTime: 0/);
	const transcriptFn = session.slice(
		session.indexOf("const fetchTranscript ="),
	);
	assert.match(transcriptFn.slice(0, 500), /staleTime: 60_000/);
});

test("每輪回應後作廢該對話的 transcript——切走再切回不得少掉新訊息", () => {
	assert.match(
		session,
		/queryKey: queryKeys\.assistant\.transcript\(\s*\n?\s*queryIdentity,\s*\n?\s*conversationId,\s*\n?\s*\)/,
	);
});

test("只有 ?requestId= 那條路沒有 transcript，此時不得把本地 turns 清空", () => {
	assert.match(
		session,
		/turns: restored\.turns\?\.length \? restored\.turns : s\.turns/,
	);
});

test("滑過／聚焦某一列就預抓，讓沒看過的對話也接近瞬切", () => {
	assert.match(
		sidebar,
		/onMouseEnter=\{\(\) => onAgentRowPrefetch\?\.\(row\)\}/,
	);
	assert.match(sidebar, /onFocus=\{\(\) => onAgentRowPrefetch\?\.\(row\)\}/);
	assert.match(
		layout,
		/onAgentRowPrefetch=\{\(row\) => void agent\.prefetch\(row\.id\)\}/,
	);
});

test("預抓要抓齊 transcript＋snapshot（少一份就讀不到快取、照樣閃骨架）", () => {
	const prefetch = hook.slice(hook.indexOf("const prefetch ="));
	assert.match(prefetch, /queryKeys\.assistant\.transcript\(/);
	assert.match(prefetch, /queryKeys\.assistant\.snapshot\(/);
	// hover 反覆觸發不得洗 API。
	assert.match(prefetch, /staleTime: 15_000/);
	// 預抓失敗是無聲的——它只是加速，不能變成錯誤畫面。
	assert.match(prefetch, /\.catch\(\(\) => \{\}\)/);
	assert.match(prefetch, /if \(!shouldFetch\) return;/);
});
