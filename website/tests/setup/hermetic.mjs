/**
 * Preloaded into every test process: `pnpm run test:unit` (`--import ./tests/setup/hermetic.mjs`), every
 * Vitest worker (`setupFiles` in vitest.config.ts) and every subprocess a test spawns (`hermeticEnv()` in
 * tests/helpers/hermetic-env.ts puts it in `NODE_OPTIONS`).
 *
 * A test must never reach a real host. Unset, the dev flavor's server API base is the local monolith
 * (lib/config/env.ts DEV_GATEWAY_HOST), so a request a test forgot to stub would go there.
 *
 * 1. `API_BASE_URL` and `NEXT_PUBLIC_API_BASE_URL` are pinned to a loopback discard port before any module
 *    reads the env, unless the test already chose a loopback (or same-origin relative) value.
 * 2. `NEXT_PUBLIC_WEB_ORIGIN`, `NEXT_PUBLIC_ADM_ORIGIN` and `NEXT_PUBLIC_MEDIA_BASE_URL` get fixed reserved-name (RFC 2606) values when
 *    unset, so assertions on canonical and media URLs do not depend on the developer's `.env.local`.
 * 3. Every way out of the process refuses a host that is not loopback, and says so by name:
 *      - global `fetch` and `WebSocket`,
 *      - `http.request/get`, `https.request/get`, `http2.connect`,
 *      - and, underneath all of them (undici, ws, jsdom's XHR, msw's pass-through, a bare `net.connect`),
 *        `net.Socket.prototype.connect`.
 *    A test that needs an answer installs its own stub (msw, a `fetch` stub, a loopback server).
 *
 * Plain JavaScript on purpose: a subprocess loads it through `NODE_OPTIONS` without any TypeScript loader.
 */
import http from "node:http";
import http2 from "node:http2";
import https from "node:https";
import { syncBuiltinESMExports } from "node:module";
import net from "node:net";
import { isLoopbackBase, isLoopbackHost } from "./loopback.mjs";

const LOOPBACK_API_BASE = "http://127.0.0.1:9/api/v1";

for (const name of ["API_BASE_URL", "NEXT_PUBLIC_API_BASE_URL"]) {
	const current = process.env[name];
	if (!current || !isLoopbackBase(current)) {
		process.env[name] = LOOPBACK_API_BASE;
	}
}

const TEST_ORIGINS = {
	NEXT_PUBLIC_WEB_ORIGIN: "https://web-dev.example.com",
	NEXT_PUBLIC_ADM_ORIGIN: "https://admin.example.com",
	NEXT_PUBLIC_MEDIA_BASE_URL: "https://test-media.example.com",
};

for (const [name, value] of Object.entries(TEST_ORIGINS)) {
	if (!process.env[name]?.trim()) process.env[name] = value;
}

function refusal(what, host) {
	return new Error(
		`hermetic test: refusing ${what} to ${host} (stub it in the test, or use a loopback address)`,
	);
}

/** The host a `http(s).request(url|options, options?, cb?)` call is about, or null for a unix socket. */
function requestHost(args) {
	const [first, second] = args;
	if (typeof first === "string" || first instanceof URL) {
		const url = new URL(first);
		const options = second && typeof second === "object" ? second : {};
		if (options.socketPath) return null;
		return options.hostname ?? options.host ?? url.hostname;
	}
	if (first && typeof first === "object") {
		if (first.socketPath) return null;
		return first.hostname ?? first.host ?? "localhost";
	}
	return "localhost";
}

function guardRequest(owner, name) {
	const real = owner[name];
	owner[name] = function guarded(...args) {
		const host = requestHost(args);
		if (host !== null && !isLoopbackHost(host)) {
			throw refusal(`${owner === https ? "https" : "http"}.${name}`, host);
		}
		return real.apply(this, args);
	};
}

guardRequest(http, "request");
guardRequest(http, "get");
guardRequest(https, "request");
guardRequest(https, "get");

const realHttp2Connect = http2.connect;
http2.connect = function guarded(authority, ...rest) {
	const host = new URL(
		typeof authority === "string" ? authority : authority.href,
	).hostname;
	if (!isLoopbackHost(host)) throw refusal("http2.connect", host);
	return realHttp2Connect.call(this, authority, ...rest);
};

/** The host a `Socket.prototype.connect` call is about, or null for a unix socket. */
function connectHost(args) {
	let [first, second] = args;
	// net.connect() hands over its normalized `[options, callback]` array
	if (Array.isArray(first)) [first, second] = first;
	if (first !== null && typeof first === "object") {
		if (first.path) return null;
		return first.host ?? first.hostname ?? "localhost";
	}
	if (typeof first === "string" && !/^\d+$/.test(first)) return null;
	return typeof second === "string" ? second : "localhost";
}

const realConnect = net.Socket.prototype.connect;
net.Socket.prototype.connect = function guarded(...args) {
	const host = connectHost(args);
	if (host !== null && !isLoopbackHost(host)) {
		const error = refusal("a TCP connection", host);
		process.nextTick(() => this.destroy(error));
		return this;
	}
	return realConnect.apply(this, args);
};

const realFetch = globalThis.fetch;
if (typeof realFetch === "function") {
	globalThis.fetch = async function guarded(input, init) {
		const raw = input instanceof Request ? input.url : String(input);
		const { hostname } = new URL(raw, "http://localhost");
		if (!isLoopbackHost(hostname)) throw refusal("a fetch", hostname);
		return realFetch.call(this, input, init);
	};
}

const RealWebSocket = globalThis.WebSocket;
if (typeof RealWebSocket === "function") {
	globalThis.WebSocket = class GuardedWebSocket extends RealWebSocket {
		constructor(url, ...rest) {
			const { hostname } = new URL(String(url), "http://localhost");
			if (!isLoopbackHost(hostname)) throw refusal("a WebSocket", hostname);
			super(url, ...rest);
		}
	};
}

// `import { request } from "node:http"` binds to the ESM view of the builtin; bring it up to date.
syncBuiltinESMExports();
