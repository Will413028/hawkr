import assert from "node:assert/strict";
import { type ChildProcess, spawn } from "node:child_process";
import http from "node:http";
import type { AddressInfo } from "node:net";
import test, { after, before } from "node:test";
import { fileURLToPath } from "node:url";
import { hermeticEnv } from "../helpers/hermetic-env";

/**
 * The HTTP status of an unknown id, from `next start` on the build in `.next` (`pnpm run test:build-output`,
 * run after the build). With the session read in a hole instead of a root `<Suspense>`, `notFound()` must
 * still run before the first byte is streamed, or the 404 body would ship with a 200.
 *
 * The upstream is a loopback fake: unknown products and sellers answer 404, everything else an empty list.
 */
const ROOT = fileURLToPath(new URL("../../", import.meta.url));

let upstream: http.Server;
let site: ChildProcess;
let base = "";

function listen(server: http.Server): Promise<number> {
	return new Promise((resolve) =>
		server.listen(0, "127.0.0.1", () =>
			resolve((server.address() as AddressInfo).port),
		),
	);
}

before(async () => {
	upstream = http.createServer((req, res) => {
		const path = (req.url ?? "").split("?")[0];
		const send = (code: number, body: unknown) => {
			res.writeHead(code, { "content-type": "application/json" });
			res.end(JSON.stringify(body));
		};
		if (/\/public\/(products|sellers)\/[^/]+$/.test(path)) {
			return send(404, { code: 404, message: "not found" });
		}
		if (/categories\/tree|tags|hashtags/.test(path)) {
			return send(200, { code: 0, data: [] });
		}
		send(200, {
			code: 0,
			data: { records: [], total: 0, page: 1, pageSize: 8 },
		});
	});
	const upstreamPort = await listen(upstream);
	const probe = http.createServer();
	const port = await listen(probe);
	await new Promise((resolve) => probe.close(resolve));
	base = `http://127.0.0.1:${port}`;
	const apiBase = `http://127.0.0.1:${upstreamPort}/api/v1`;
	site = spawn(
		process.execPath,
		[`${ROOT}node_modules/next/dist/bin/next`, "start", "-p", String(port)],
		{
			cwd: ROOT,
			env: hermeticEnv({
				API_BASE_URL: apiBase,
				NEXT_PUBLIC_API_BASE_URL: apiBase,
			}) as NodeJS.ProcessEnv,
			stdio: "ignore",
		},
	);
	for (let i = 0; i < 100; i++) {
		try {
			await fetch(`${base}/faq`);
			return;
		} catch {
			await new Promise((resolve) => setTimeout(resolve, 300));
		}
	}
	throw new Error("next start did not come up");
});

after(async () => {
	site?.kill();
	await new Promise((resolve) => upstream?.close(resolve));
});

async function statusOf(path: string): Promise<number> {
	const res = await fetch(`${base}${path}`, { redirect: "manual" });
	await res.arrayBuffer();
	return res.status;
}

test("an unknown product id answers 404", async () => {
	assert.equal(await statusOf("/products/999999999"), 404);
	assert.equal(await statusOf("/products/doesnotexist"), 404);
});

test("an invalid delegated storeId answers 404 on every page under it", async () => {
	assert.equal(await statusOf("/adm/abc/orders"), 404);
	assert.equal(await statusOf("/adm/99999999999999999999/orders"), 404);
});

test("a known page still answers 200 with the header in the document", async () => {
	const res = await fetch(`${base}/`);
	const html = await res.text();
	assert.equal(res.status, 200);
	assert.ok(html.includes("site-header"));
});
