#!/usr/bin/env node

/**
 * Transport gate: every gateway call must go through `apiClient` (lib/api/client.ts),
 * or, for the Connect API, through the transports in lib/connect/transport.ts.
 * Bans hand-built gateway fetches — `fetch(`${API_BASE | env.*ApiBase | gatewayHost}…`)`
 * — outside a small allowlist, so the timeout / retry / envelope / credentials policy
 * stays in one place and the "server fetch with no timeout hangs the SSR stream"
 * bug class (React #419) cannot regrow.
 */

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const root = process.cwd();
const failures: string[] = [];

// Files allowed to build a gateway URL by hand: the transport itself and the
// SSE long-poll streams (which must NOT carry a fixed fetch timeout — they are
// kept alive by their own heartbeat watchdog).
const ALLOWLIST = new Set([
	"lib/api/client.ts",
	"lib/sse/client.ts",
	"lib/seller/ai-listing-chat.ts",
	"lib/ai-chat/stream-client.ts",
	// 代行 BFF 的唯一上游轉送點（56 號 §4.5）。與 lib/api/client.ts 同性質：逾時／
	// header allow-list／envelope policy 集中在這一支，而且它是本 repo 唯一會轉送
	// operation credential 的模組。刻意用單檔 allowlist 而不是
	// `app/(delegated)/adm/api/` 目錄前綴 —— 目錄前綴會讓底下每一支 route 都能自己
	// 組 gateway URL。
	// ⚠️ 現況它其實掃不到那支檔案：下面的述詞只認得「fetch( 之後 120 字內出現
	// base 識別字」的行內寫法，而 _upstream.ts 先把 URL 存進變數（簽章要用同一個
	// 字串）。「只有一個轉送點」的實際保證在 tests/delegated-bff-boundary.test.ts；
	// 這條登記是為了日後有人改回行內寫法時不會突然轉紅。
	"app/(delegated)/adm/api/_upstream.ts",
]);
// CDN / OSS image helpers never touch the gateway base.
const ALLOW_PREFIXES = ["lib/media/"];

const BASE_TOKEN = "(?:serverApiBase|apiBaseUrl|gatewayHost|API_BASE)";
// A `fetch(` whose URL argument references a gateway base within ~120 chars.
const BARE_GATEWAY_FETCH = new RegExp(`fetch\\([\\s\\S]{0,120}?${BASE_TOKEN}`);
// The deleted constant must never be imported again.
const API_BASE_IMPORT = /import\b[^;]*\bAPI_BASE\b/;
// A Connect transport built outside lib/connect/transport.ts would skip its
// credentials, origin-auth, timeout, retry (ADR 2026-09-25-api-error-model D7)
// and reporting policy.
const CONNECT_TRANSPORT = /\bcreate(?:Connect|GrpcWeb)Transport\s*\(/;
const CONNECT_TRANSPORT_OWNER = "lib/connect/transport.ts";

function walk(dir: string): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(join(root, dir));
	} catch {
		return out;
	}
	for (const entry of entries) {
		if (
			entry === "node_modules" ||
			entry === ".next" ||
			entry === ".git" ||
			entry === ".worktrees"
		) {
			continue;
		}
		const rel = relative(root, join(root, dir, entry));
		if (statSync(join(root, rel)).isDirectory()) out.push(...walk(rel));
		else if (/\.(ts|tsx)$/.test(rel)) out.push(rel);
	}
	return out;
}

function isAllowed(path: string): boolean {
	return ALLOWLIST.has(path) || ALLOW_PREFIXES.some((p) => path.startsWith(p));
}

const files = ["lib", "features", "components", "app"].flatMap((d) => walk(d));
for (const path of files) {
	if (isAllowed(path)) continue;
	const text = readFileSync(join(root, path), "utf8");
	if (BARE_GATEWAY_FETCH.test(text)) {
		failures.push(
			`${path}: hand-built gateway fetch — route it through apiClient.public/authed (lib/api/client.ts).`,
		);
	}
	if (path !== CONNECT_TRANSPORT_OWNER && CONNECT_TRANSPORT.test(text)) {
		failures.push(
			`${path}: builds its own Connect transport — use the transports in ${CONNECT_TRANSPORT_OWNER}.`,
		);
	}
	if (API_BASE_IMPORT.test(text)) {
		failures.push(
			`${path}: imports the removed API_BASE constant — use apiClient instead.`,
		);
	}
}

// Self-check: the rules must actually fire / not fire on known inputs. (Fixtures
// avoid a literal template placeholder so they don't trip noTemplateCurlyInString;
// the regex only cares that a base token follows `fetch(` within range.)
if (
	!BARE_GATEWAY_FETCH.test("await fetch(env.apiBaseUrl + '/public/x')") ||
	!BARE_GATEWAY_FETCH.test("fetch(getEnv().serverApiBase + '/public/x')") ||
	BARE_GATEWAY_FETCH.test("await fetch('/api/public/posts/' + id)") ||
	!CONNECT_TRANSPORT.test("const t = createConnectTransport({ baseUrl });") ||
	CONNECT_TRANSPORT.test("const t = createRouterTransport((r) => r);") ||
	!API_BASE_IMPORT.test('import { API_BASE } from "@/lib/api";')
) {
	console.error("check-transport self-test failed — the gate regex is broken.");
	process.exit(1);
}

if (failures.length > 0) {
	console.error(`Transport gate failed:\n${failures.join("\n")}`);
	process.exit(1);
}

console.log(`Transport gate passed (${files.length} files scanned).`);
