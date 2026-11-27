import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import {
	clearEndPendingSessionId,
	DELEGATED_END_PENDING_STORAGE_KEY,
	isEndLatched,
	readEndPendingSessionId,
	writeEndPendingSessionId,
} from "../features/delegated-operations/state/end-pending-latch";

/**
 * 「結束結果不明」的封鎖必須活過一次 document load。
 *
 * 伺服器在結束未獲證明時刻意不清 credential、不終止工作階段，所以封鎖若只是一個
 * React state，操作者按 F5 就會回到可寫入的工作區 —— fail-closed 保證整個消失。
 * 本檔守兩件事：① 旗標的語意（什麼情況鎖、什麼情況放）② provider 真的接了它
 * （沒有執行環境可以渲染 provider，所以第二件靠原始碼契約守）。
 */

function fakeStore(initial: Record<string, string> = {}) {
	const data = new Map(Object.entries(initial));
	return {
		data,
		getItem: (key: string) => data.get(key) ?? null,
		setItem: (key: string, value: string) => {
			data.set(key, value);
		},
		removeItem: (key: string) => {
			data.delete(key);
		},
	};
}

test("F5 之後封鎖仍然成立（本檔的核心）", () => {
	const store = fakeStore();

	writeEndPendingSessionId("SESSION-1", store);

	const pendingSessionId = readEndPendingSessionId(store);
	assert.equal(
		isEndLatched({
			blockedInMemory: false,
			pendingSessionId,
			currentSessionId: "SESSION-1",
		}),
		true,
	);
});

test("換一個工作階段就解除封鎖（唯一需要的出路）", () => {
	const store = fakeStore();
	writeEndPendingSessionId("SESSION-1", store);

	assert.equal(
		isEndLatched({
			blockedInMemory: false,
			pendingSessionId: readEndPendingSessionId(store),
			currentSessionId: "SESSION-2",
		}),
		false,
	);
});

test("記憶體旗標單獨成立；沒有旗標或還沒拿到 summary 時不鎖", () => {
	assert.equal(
		isEndLatched({
			blockedInMemory: true,
			pendingSessionId: null,
			currentSessionId: null,
		}),
		true,
	);
	assert.equal(
		isEndLatched({
			blockedInMemory: false,
			pendingSessionId: null,
			currentSessionId: "SESSION-1",
		}),
		false,
	);
	assert.equal(
		isEndLatched({
			blockedInMemory: false,
			pendingSessionId: "SESSION-1",
			currentSessionId: null,
		}),
		false,
	);
});

test("結束被證明後清得掉", () => {
	const store = fakeStore();
	writeEndPendingSessionId("SESSION-1", store);
	clearEndPendingSessionId(store);
	assert.equal(readEndPendingSessionId(store), null);
	assert.equal(store.data.has(DELEGATED_END_PENDING_STORAGE_KEY), false);
});

test("沒有 storage 時三支都不得 throw（SSR／私密模式）", () => {
	assert.equal(readEndPendingSessionId(null), null);
	assert.doesNotThrow(() => writeEndPendingSessionId("SESSION-1", null));
	assert.doesNotThrow(() => clearEndPendingSessionId(null));

	const hostile = {
		getItem: () => {
			throw new Error("SecurityError");
		},
		setItem: () => {
			throw new Error("QuotaExceededError");
		},
		removeItem: () => {
			throw new Error("SecurityError");
		},
	};
	assert.equal(readEndPendingSessionId(hostile), null);
	assert.doesNotThrow(() => writeEndPendingSessionId("SESSION-1", hostile));
	assert.doesNotThrow(() => clearEndPendingSessionId(hostile));
});

/**
 * 上面全是純函式，provider 把它們全部拔掉照樣綠 —— 而那正是這個 bug 的原樣。
 * 沒有 DOM 測試環境可以渲染 provider（vitest 跑 node、tests/ 只有
 * renderToStaticMarkup），所以接線由原始碼契約守。
 */
test("provider 真的接了持久化封鎖", () => {
	const source = readFileSync(
		new URL(
			"../features/delegated-operations/state/DelegatedSessionContext.tsx",
			import.meta.url,
		),
		"utf8",
	);

	for (const needle of [
		"readEndPendingSessionId()",
		"writeEndPendingSessionId(",
		"clearEndPendingSessionId()",
		"isEndLatched({",
	]) {
		assert.ok(
			source.includes(needle),
			`DelegatedSessionContext 少了 ${needle} ⇒ 封鎖又只活在記憶體裡`,
		);
	}

	const blockedOnlyUses = source.match(/writesBlockedByEnd/g) ?? [];
	assert.equal(
		blockedOnlyUses.length,
		2,
		"writesBlockedByEnd 只該出現在宣告與 isEndLatched 的入參兩處（其餘一律讀 endLatched）",
	);
	assert.match(
		source,
		/endLatched && canRenderDelegatedWorkspace\(state\)/,
		"end_unknown 的推導沒有走 endLatched",
	);
	assert.match(source, /!endLatched/, "writesAllowed 沒有讀 endLatched");
});
