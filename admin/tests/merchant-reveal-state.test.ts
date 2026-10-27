import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { RevealKYCSensitiveResponseSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import {
	type KycReveal,
	revealFrom,
} from "../src/app/admin/merchants/[id]/kyc-application";
import {
	closePreview,
	EMPTY_REVEAL_STATE,
	type ExpiryTimers,
	expireReveal,
	hideReveal,
	openPreview,
	type RevealState,
	scheduleRevealExpiry,
	showReveal,
} from "../src/app/admin/merchants/[id]/reveal-state";

/**
 * 顯示全文的授權與放大中的證件影像一起出現、一起消失：到期、收起時兩者都清掉；到期的計時器可能提早或延後觸發，
 * 觸發時還沒到期要重排。時間全部是注入的固定值，不讀系統時鐘、不用真的計時器。
 */

const EXPIRE = new Date("2026-10-11T05:00:00Z");

function aReveal(): KycReveal {
	const reveal = revealFrom(
		"app1",
		create(RevealKYCSensitiveResponseSchema, {
			idCardNo: "A123456789",
			expireTime: timestampFromDate(EXPIRE),
		}),
	);
	assert.ok(reveal);
	return reveal;
}

const IMAGE = { src: "/api/bff/api/files/admin/x", alt: "身分證(正)" };

function opened(): RevealState {
	return openPreview(showReveal(aReveal()), IMAGE);
}

test("放大圖只在授權還在時能開；顯示全文成功時沒有放大圖", () => {
	assert.deepEqual(openPreview(EMPTY_REVEAL_STATE, IMAGE), EMPTY_REVEAL_STATE);
	const state = showReveal(aReveal());
	assert.equal(state.preview, null);
	assert.deepEqual(openPreview(state, IMAGE).preview, IMAGE);
	assert.equal(closePreview(opened()).preview, null);
	assert.ok(closePreview(opened()).reveal, "關掉放大圖不影響授權");
});

test("到期：授權與已打開的放大圖一起清掉；到期前原封不動", () => {
	const state = opened();
	assert.equal(
		expireReveal(state, new Date(EXPIRE.getTime() - 1)),
		state,
		"到期前同一個物件",
	);
	const expired = expireReveal(state, EXPIRE);
	assert.equal(expired.reveal, null);
	assert.equal(expired.preview, null, "放大圖也要關掉");
	assert.equal(expireReveal(EMPTY_REVEAL_STATE, EXPIRE), EMPTY_REVEAL_STATE);
});

test("收起：授權與放大圖一起清掉", () => {
	const hidden = hideReveal();
	assert.equal(hidden.reveal, null);
	assert.equal(hidden.preview, null);
});

/** 手動推進的計時器：記下排了什麼，由測試決定什麼時候、在哪個時間觸發。 */
function fakeTimers(startMs: number) {
	let nowMs = startMs;
	let nextId = 0;
	const pending = new Map<number, { at: number; callback: () => void }>();
	const timers: ExpiryTimers = {
		now: () => new Date(nowMs),
		setTimer: (callback, ms) => {
			nextId += 1;
			pending.set(nextId, { at: nowMs + ms, callback });
			return nextId;
		},
		clearTimer: (handle) => {
			pending.delete(handle as number);
		},
	};
	return {
		timers,
		pending,
		/** 把時鐘撥到 ms，並觸發所有排定的計時器（不管它們排定的時間，模擬提早觸發）。 */
		fireAllAt(ms: number) {
			nowMs = ms;
			for (const [id, t] of [...pending]) {
				pending.delete(id);
				t.callback();
			}
		},
	};
}

test("到期計時器：排在授權到期的那一刻，觸發時已到期才呼叫 onExpire（一次）", () => {
	const clock = fakeTimers(EXPIRE.getTime() - 15 * 60_000);
	let expired = 0;
	scheduleRevealExpiry(aReveal(), () => expired++, clock.timers);
	assert.equal(clock.pending.size, 1);
	assert.equal([...clock.pending.values()][0].at, EXPIRE.getTime());
	clock.fireAllAt(EXPIRE.getTime());
	assert.equal(expired, 1);
	assert.equal(clock.pending.size, 0);
});

test("計時器提早觸發時重排，不是排一次就算數；之後到期才呼叫 onExpire", () => {
	const clock = fakeTimers(EXPIRE.getTime() - 15 * 60_000);
	let expired = 0;
	scheduleRevealExpiry(aReveal(), () => expired++, clock.timers);
	// 提早 5 分鐘觸發：還沒到期，要重排到剩餘的時間。
	clock.fireAllAt(EXPIRE.getTime() - 5 * 60_000);
	assert.equal(expired, 0, "還沒到期不能清掉");
	assert.equal(clock.pending.size, 1, "重排了");
	assert.equal([...clock.pending.values()][0].at, EXPIRE.getTime());
	clock.fireAllAt(EXPIRE.getTime());
	assert.equal(expired, 1);
});

test("recheck（分頁回到前景）：已到期立刻處理，還沒到期不動；cancel 之後不再呼叫", () => {
	const clock = fakeTimers(EXPIRE.getTime() - 15 * 60_000);
	let expired = 0;
	const schedule = scheduleRevealExpiry(
		aReveal(),
		() => expired++,
		clock.timers,
	);
	schedule.recheck();
	assert.equal(expired, 0);
	// 背景分頁的計時器被延後：時鐘已過到期，計時器還沒觸發，回到前景時 recheck。
	clock.fireAllAt(EXPIRE.getTime() + 30_000); // 計時器此刻才觸發
	assert.equal(expired, 1);
	schedule.recheck();
	assert.equal(expired, 1, "只呼叫一次");

	const clock2 = fakeTimers(EXPIRE.getTime() - 60_000);
	let called = 0;
	const s2 = scheduleRevealExpiry(aReveal(), () => called++, clock2.timers);
	s2.cancel();
	assert.equal(clock2.pending.size, 0);
	clock2.fireAllAt(EXPIRE.getTime() + 1000);
	s2.recheck();
	assert.equal(called, 0);
});

test("頁面接線：到期、收起都走 reveal-state（授權與放大圖一起清）並 reset 顯示全文的 mutation；放大圖只在授權還在時顯示", async () => {
	const { readFile } = await import("node:fs/promises");
	const page = await readFile("src/app/admin/merchants/[id]/page.tsx", "utf8");
	assert.match(page, /scheduleRevealExpiry\(/u);
	assert.match(page, /expireReveal\(cur, new Date\(\)\)/u);
	assert.match(
		page,
		/revealMutationReset\(\)/u,
		"到期時 reset mutation（明文不留在 mutation data）",
	);
	assert.match(
		page,
		/setSecret\(hideReveal\(\)\);\s*revealMutation\.reset\(\);/u,
	);
	assert.match(
		page,
		/const previewImage = liveReveal \? secret\.preview : null;/u,
	);
	assert.doesNotMatch(
		page,
		/setPreviewImage|useState<\{\s*src: string/u,
		"放大圖不再是獨立的 state",
	);
});
