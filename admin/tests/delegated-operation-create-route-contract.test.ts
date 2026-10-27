import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("create route derives authority from server preflight and reconciles before handing over the code", async () => {
	const source = await readFile(
		"src/app/api/admin/delegated-operations/route.ts",
		"utf8",
	);
	const handler = source.slice(source.indexOf("export async function POST"));
	const auth = handler.indexOf("adminSession.token(request)");
	const bodyRead = handler.indexOf("readBoundedForm(request)");
	const preflight = handler.indexOf("DELEGATED_OPERATION_UPSTREAM.preflight");
	const create = handler.indexOf("DELEGATED_OPERATION_UPSTREAM.create");
	const summary = handler.indexOf(
		"DELEGATED_OPERATION_UPSTREAM.summary(sessionId)",
	);
	const handoff = handler.indexOf("buildHandoffLocation");

	assert.ok(auth >= 0 && auth < bodyRead, "authenticate before reading body");
	assert.ok(
		preflight >= 0 && preflight < create,
		"authoritative preflight before create",
	);
	assert.ok(
		summary > create && summary < handoff,
		"reconcile before the 303 Location",
	);
	assert.match(handler, /capabilities:\s*preflight\.capabilities/u);
	assert.doesNotMatch(handler, /operationSession:\s*operationCredential/u);
	assert.match(handler, /"operationCredential" in data/u);
	assert.doesNotMatch(handler, /startRequest\.capabilities/u);
	// A browser-supplied TTL may only narrow the window, never widen it: the
	// preflight value stays the ceiling on both the outbound body and the
	// echoed-context check.
	assert.match(
		handler,
		/const requestedTtlSeconds = Math\.min\(\s*startRequest\.ttlSeconds \?\? preflight\.ttlSeconds,\s*preflight\.ttlSeconds,\s*\)/u,
	);
	assert.match(handler, /ttlSeconds:\s*requestedTtlSeconds/u);
	assert.doesNotMatch(handler, /preflight\.ttlSeconds \* 1000/u);
});

test("credential 不落 cookie，兌換去向不得由請求的 Host 推導", async () => {
	const source = await readFile(
		"src/app/api/admin/delegated-operations/route.ts",
		"utf8",
	);
	// 兩個 host 各存一份就必然有一份清不掉（56 號 §2.7）。
	assert.doesNotMatch(source, /cookies\.set/u);
	assert.doesNotMatch(source, /Set-Cookie/iu);
	// Host 可被呼叫端控制；由它推導等於讓對方指定 credential 的兌換去向。
	assert.doesNotMatch(source, /headers\.get\("host"\)/u);
	assert.match(source, /WEB_ORIGIN/u);
});
