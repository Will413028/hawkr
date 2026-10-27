import assert from "node:assert/strict";
import test from "node:test";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { runAction } from "../src/lib/connect-action";

const CONFLICT = "已重新載入";

function prospectMatch(): ConnectError {
	return new ConnectError("match", Code.FailedPrecondition, undefined, [
		{
			desc: ErrorInfoSchema,
			value: {
				reason: "PROSPECT_MATCH_FOUND",
				domain: "hawkr",
				metadata: { prospectIds: "a,b" },
			},
		},
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message: "找到相符的名單" },
		},
	]);
}

test("認得的 reason 與 metadata 隨 message 結果回傳，呼叫端依 reason 分流", async () => {
	const result = await runAction(() => Promise.reject(prospectMatch()), {
		reload: async () => {},
		fallback: "fb",
		conflictText: CONFLICT,
	});
	assert.equal(result.ok, false);
	assert.ok(!result.ok && result.kind === "message");
	assert.equal(
		!result.ok && result.kind === "message" && result.text,
		"找到相符的名單",
	);
	assert.equal(
		!result.ok && result.kind === "message" && result.reason,
		ErrorReason.PROSPECT_MATCH_FOUND,
	);
	assert.equal(
		!result.ok && result.kind === "message" && result.metadata?.prospectIds,
		"a,b",
	);
	assert.equal(!result.ok && result.code, Code.FailedPrecondition);
	assert.equal(!result.ok && result.noResponse, false);
});

test("沒收到回應時沒有 reason 與 metadata", async () => {
	const result = await runAction(
		() => Promise.reject(ConnectError.from(new TypeError("Failed to fetch"))),
		{ reload: async () => {}, fallback: "fb", conflictText: CONFLICT },
	);
	assert.deepEqual(result, {
		ok: false,
		kind: "message",
		text: "fb",
		noResponse: true,
	});
	assert.equal(!result.ok && result.code, undefined);
});

test("沒有認得的 reason 的失敗也沒有 reason", async () => {
	const result = await runAction(
		() => Promise.reject(new ConnectError("x", Code.Unavailable)),
		{ reload: async () => {}, fallback: "fb", conflictText: CONFLICT },
	);
	assert.ok(!result.ok && result.kind === "message");
	assert.equal("reason" in result, false);
	assert.equal(result.code, Code.Unavailable);
	assert.equal(result.noResponse, false);
});

function versionConflict(): ConnectError {
	return new ConnectError("etag", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: { reason: "VERSION_CONFLICT", domain: "hawkr" },
		},
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message: "資料已被更新，請重新讀取" },
		},
	]);
}

test("成功時回傳寫入的回應", async () => {
	const result = await runAction(async () => ({ id: "p1" }), {
		fallback: "fb",
		conflictText: CONFLICT,
	});
	assert.deepEqual(result, { ok: true, value: { id: "p1" } });
});

test("版本衝突但沒有 reload：不宣稱已重新載入，回伺服器的句子", async () => {
	const result = await runAction(() => Promise.reject(versionConflict()), {
		fallback: "fb",
		conflictText: CONFLICT,
	});
	assert.deepEqual(result, {
		ok: false,
		kind: "conflict",
		text: "資料已被更新，請重新讀取",
		code: Code.Aborted,
		noResponse: false,
	});
});

test("版本衝突且重讀失敗：回伺服器的句子，不說已重新載入", async () => {
	const conflict = versionConflict();
	const result = await runAction(() => Promise.reject(conflict), {
		reload: () => Promise.reject(new Error("網路")),
		fallback: "fb",
		conflictText: CONFLICT,
	});
	assert.deepEqual(result, {
		ok: false,
		kind: "conflict",
		text: "資料已被更新，請重新讀取",
		code: Code.Aborted,
		noResponse: false,
	});
});
