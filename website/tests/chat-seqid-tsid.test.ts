import assert from "node:assert/strict";
import test from "node:test";
import { parseWSMessage } from "../lib/chat/types";

// 聊天訊息的 seqId / messageId / from / to 都是 64-bit TSID，超過 JS
// Number.MAX_SAFE_INTEGER (2^53-1)。WebSocket 訊息若把 seqId 當 number 解析，
// full-width TSID 末位被四捨五入，使去重 key `${conversationId}_${seqId}` 不穩定
// → 同一則訊息可能重複推送或漏去重；又與 REST 歷史（保字串）型別不一致。
//
// 本測試鎖住 parseWSMessage 一律把這些 id 正規化成字串、且 > 2^53 不丟精度。
const TSID = "7234567890123456789"; // > 2^53

test("parseWSMessage：seqId 為 number 時轉成字串（去重 key 穩定）", () => {
	const m = parseWSMessage({
		type: "chat",
		id: 123,
		from: 456,
		to: 789,
		seqId: 100,
	});
	assert.equal(m.seqId, "100");
	assert.equal(m.id, "123");
	assert.equal(m.senderId, "456");
	assert.equal(m.receiverId, "789");
	assert.equal(typeof m.seqId, "string");
});

test("parseWSMessage：seqId / id 為 string TSID 時精確保留", () => {
	const m = parseWSMessage({
		type: "chat",
		id: TSID,
		messageId: TSID,
		from: TSID,
		to: TSID,
		seqId: TSID,
	});
	assert.equal(m.seqId, TSID);
	assert.equal(m.id, TSID);
	assert.equal(m.senderId, TSID);
	assert.equal(m.receiverId, TSID);
});

test("parseWSMessage：缺 seqId 時為 undefined（不可變成 'undefined' / '0'）", () => {
	const m = parseWSMessage({ type: "typing", from: TSID, to: TSID });
	assert.equal(m.seqId, undefined);
});
