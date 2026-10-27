import assert from "node:assert/strict";
import { createServer, type Server } from "node:http";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

// Reproduces the React #419 read-hang: an upstream that sends response headers
// (so fetch() resolves) but then stalls mid-body forever. The transport's
// AbortSignal.timeout must fire and a softFail GET must degrade to a null-data
// envelope — NOT throw and NOT hang the SSR stream.

let server: Server;
const prevApiBase = process.env.API_BASE_URL;

function startHangServer(): Promise<number> {
	return new Promise((resolve) => {
		server = createServer((_req, res) => {
			res.writeHead(200, { "content-type": "application/json" });
			res.write('{"code":0,"data"'); // partial body, then hang — never res.end()
		});
		server.listen(0, "127.0.0.1", () => {
			const addr = server.address();
			resolve(typeof addr === "object" && addr ? addr.port : 0);
		});
	});
}

beforeEach(async () => {
	const port = await startHangServer();
	process.env.API_BASE_URL = `http://127.0.0.1:${port}`;
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	delete (globalThis as { window?: unknown }).window; // server path → serverApiBase
});

afterEach(() => {
	if (prevApiBase === undefined) delete process.env.API_BASE_URL;
	else process.env.API_BASE_URL = prevApiBase;
	__setEnvForTest(null);
	__setFlavorForTest(null);
	server.closeAllConnections?.();
	server.close();
});

test("softFail GET degrades a header-then-body-hang upstream to null (no throw, no hang)", async () => {
	const { apiClient } = await import("../lib/api/client");
	const started = Date.now();
	const { data } = await apiClient.public.get("/marketplace", {
		softFail: true,
		timeoutMs: 700,
	});
	const elapsed = Date.now() - started;
	assert.equal(data, null, "should soft-fail to null data");
	assert.ok(elapsed < 3000, `should abort near the timeout, took ${elapsed}ms`);
});

test("non-softFail GET surfaces the timeout as an error (caller owns it)", async () => {
	const { apiClient } = await import("../lib/api/client");
	await assert.rejects(() =>
		apiClient.public.get("/marketplace", { timeoutMs: 700 }),
	);
});
