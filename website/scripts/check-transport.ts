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

const ALLOWLIST = new Set([
	"lib/api/client.ts",
	"lib/sse/client.ts",
	"app/(delegated)/adm/api/_upstream.ts",
]);
const ALLOW_PREFIXES = ["lib/media/"];

const BASE_TOKEN = "(?:serverApiBase|apiBaseUrl|gatewayHost|API_BASE)";
const BARE_GATEWAY_FETCH = new RegExp(`fetch\\([\\s\\S]{0,120}?${BASE_TOKEN}`);
const API_BASE_IMPORT = /import\b[^;]*\bAPI_BASE\b/;
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
