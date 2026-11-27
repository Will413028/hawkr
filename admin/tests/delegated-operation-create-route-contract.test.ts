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
	assert.doesNotMatch(source, /cookies\.set/u);
	assert.doesNotMatch(source, /Set-Cookie/iu);
	assert.doesNotMatch(source, /headers\.get\("host"\)/u);
	assert.match(source, /WEB_ORIGIN/u);
});
