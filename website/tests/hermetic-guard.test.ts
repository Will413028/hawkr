import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readdir, readFile } from "node:fs/promises";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { hermeticEnv } from "./helpers/hermetic-env";

const root = fileURLToPath(new URL("../", import.meta.url));

/**
 * tests/setup/hermetic.mjs is preloaded into this process by `pnpm run test:unit`; these tests pin what it
 * promises, because a guard nobody checks turns into one that quietly stopped working.
 */

test("the API base is pinned to a loopback address", () => {
	for (const name of ["API_BASE_URL", "NEXT_PUBLIC_API_BASE_URL"]) {
		const value = process.env[name] ?? "";
		assert.ok(
			value.startsWith("/") ||
				/^https?:\/\/(?:127\.|localhost|\[::1\])/.test(value),
			`${name}=${value}`,
		);
	}
});

test("fetch, node:http, node:https and a bare TCP connection refuse a host that is not loopback", async () => {
	await assert.rejects(fetch("https://api.hawkr.example/x"), /hermetic test/);
	assert.throws(
		() => http.request("http://api.hawkr.example/x"),
		/hermetic test: refusing http\.request to api\.hawkr\.example/,
	);
	assert.throws(
		() => http.get({ hostname: "api.hawkr.example", path: "/" }),
		/hermetic test/,
	);
	assert.throws(
		() => https.request("https://api.hawkr.example/x"),
		/hermetic test: refusing https\.request/,
	);
	assert.throws(
		() => https.get("https://api.hawkr.example/x"),
		/hermetic test/,
	);
	// underneath everything else: a socket opened without any of the wrappers
	const error = await new Promise<Error>((resolve) => {
		const socket = net.connect({ host: "api.hawkr.example", port: 443 });
		socket.once("error", resolve);
		socket.once("connect", () => resolve(new Error("connected")));
	});
	assert.match(error.message, /hermetic test: refusing a TCP connection/);
});

test("a WebSocket to a host that is not loopback is refused", () => {
	assert.equal(typeof globalThis.WebSocket, "function");
	assert.throws(
		() => new WebSocket("wss://api.hawkr.example/ws"),
		/hermetic test: refusing a WebSocket/,
	);
});

test("loopback still works, over fetch and over node:http", async () => {
	const server = http.createServer((_request, response) => {
		response.end("ok");
	});
	await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
	try {
		const { port } = server.address() as net.AddressInfo;
		const viaFetch = await fetch(`http://127.0.0.1:${port}/`);
		assert.equal(await viaFetch.text(), "ok");
		const viaHttp = await new Promise<string>((resolve, reject) => {
			http
				.get({ hostname: "localhost", port, path: "/" }, (response) => {
					let body = "";
					response.on("data", (chunk) => {
						body += chunk;
					});
					response.on("end", () => resolve(body));
				})
				.on("error", reject);
		});
		assert.equal(viaHttp, "ok");
	} finally {
		server.close();
	}
});

test("a subprocess started with hermeticEnv() carries the guard, a loopback API base and the stub the test chose", () => {
	const script =
		"const out = {api: process.env.API_BASE_URL, pub: process.env.NEXT_PUBLIC_API_BASE_URL}; " +
		'try { require("node:http").request("http://api.hawkr.example/x"); out.http = "allowed"; } catch (e) { out.http = String(e.message); } ' +
		"fetch('https://api.hawkr.example/x').then(() => { out.fetch = 'allowed'; }, (e) => { out.fetch = String(e.message); }).then(() => process.stdout.write(JSON.stringify(out)));";
	const run = (env: NodeJS.ProcessEnv) => {
		const child = spawnSync(process.execPath, ["-e", script], {
			cwd: root,
			env,
			encoding: "utf8",
			timeout: 30_000,
		});
		assert.equal(child.status, 0, child.stderr);
		return JSON.parse(child.stdout) as Record<string, string>;
	};

	// the parent's own pins are removed first, so what the child shows is the preload's work
	const base = { ...process.env };
	delete base.API_BASE_URL;
	delete base.NEXT_PUBLIC_API_BASE_URL;
	const pinned = run(hermeticEnv({}, base) as NodeJS.ProcessEnv);
	assert.equal(pinned.api, "http://127.0.0.1:9/api/v1");
	assert.equal(pinned.pub, "http://127.0.0.1:9/api/v1");
	assert.match(pinned.http, /hermetic test: refusing http\.request/);
	assert.match(pinned.fetch, /hermetic test: refusing a fetch/);

	// a loopback stub the test names is kept; a real host is replaced
	const stub = run(
		hermeticEnv(
			{ API_BASE_URL: "http://127.0.0.1:41999/api/v1" },
			base,
		) as NodeJS.ProcessEnv,
	);
	assert.equal(stub.api, "http://127.0.0.1:41999/api/v1");
	const real = run(
		hermeticEnv(
			{ API_BASE_URL: "https://api.hawkr.example/api/v1" },
			base,
		) as NodeJS.ProcessEnv,
	);
	assert.equal(real.api, "http://127.0.0.1:9/api/v1");
});

const SPAWN_CALL =
	/\b(?:spawn|spawnSync|exec|execSync|execFile|execFileSync|fork|execFileAsync|execAsync)\s*\(/g;
const CHILD_PROCESS_IMPORT =
	/(?:from\s+|import\s*\(\s*|require\s*\(\s*)["'](?:node:)?child_process["']/;

/** The text of the call whose "(" is at `open`, up to its matching ")". */
function callText(code: string, open: number): string {
	let depth = 0;
	for (let i = open; i < code.length; i++) {
		if (code[i] === "(") depth++;
		else if (code[i] === ")" && --depth === 0) return code.slice(open, i + 1);
	}
	return code.slice(open);
}

test("every process a test file spawns gets the guard, call by call", async () => {
	const offenders: string[] = [];
	const visit = async (dir: string) => {
		for (const entry of await readdir(path.join(root, dir), {
			withFileTypes: true,
		})) {
			const relative = `${dir}/${entry.name}`;
			if (entry.isDirectory()) {
				if (["node_modules", "e2e", "seo", "visual"].includes(entry.name))
					continue;
				await visit(relative);
			} else if (
				/\.(test|unit\.test)\.(ts|tsx)$/.test(entry.name) &&
				// this file spawns on purpose, with the environments it is checking
				relative !== "tests/hermetic-guard.test.ts"
			) {
				const code = await readFile(path.join(root, relative), "utf8");
				if (!CHILD_PROCESS_IMPORT.test(code)) continue;
				for (const match of code.matchAll(SPAWN_CALL)) {
					const open = match.index + match[0].length - 1;
					if (!/\bhermeticEnv\s*\(/.test(callText(code, open))) {
						const line = code.slice(0, match.index).split("\n").length;
						offenders.push(`${relative}:${line}`);
					}
				}
			}
		}
	};
	for (const dir of ["tests", "lib", "features", "components", "app"]) {
		await visit(dir);
	}
	assert.deepEqual(
		offenders,
		[],
		"這些 spawn／exec 呼叫沒有在呼叫裡用 hermeticEnv()（tests/helpers/hermetic-env.ts）：子程序不會繼承預載的守衛。",
	);
});
