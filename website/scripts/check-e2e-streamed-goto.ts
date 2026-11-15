#!/usr/bin/env node

/**
 * e2e streamed-navigation gate: specs under tests/e2e must navigate through
 * gotoStreamed / reloadStreamed (tests/e2e/helpers/streamed-navigation.ts),
 * never with a raw `waitUntil: "domcontentloaded"`. Pages contain streamed Suspense boundaries, and
 * before React reveals the streamed content a hidden `body > div[hidden]
 * [id^="S:"]` copy coexists with the hydrated page, so locators can hit
 * strict-mode duplicates. A spec that must assert during streaming opts out
 * with a `// streamed-goto-allow: <reason>` line comment placed on the line
 * directly above the line that starts the call (the `.goto(` / `.reload(` line
 * owning the `waitUntil`), so one comment covers exactly one navigation.
 */

import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();
const E2E_DIR = "tests/e2e";
const HELPER = "tests/e2e/helpers/streamed-navigation.ts";
const RAW = /waitUntil:\s*["']domcontentloaded["']/;
const CALL =
	/\.(goto|reload|goBack|goForward|waitForURL|waitForNavigation|setContent)\s*\(/;
const ALLOW = /^\s*\/\/\s*streamed-goto-allow:\s*\S+/;

function findViolations(path: string, text: string): string[] {
	const lines = text.split("\n");
	const out: string[] = [];
	lines.forEach((line, i) => {
		if (!RAW.test(line)) return;
		let callLine = -1;
		for (let j = i; j >= Math.max(0, i - 5); j--) {
			if (j < i && /;\s*$/.test(lines[j] ?? "")) break;
			if (CALL.test(lines[j] ?? "")) {
				callLine = j;
				break;
			}
		}
		if (callLine > 0 && ALLOW.test(lines[callLine - 1] ?? "")) return;
		out.push(
			`${path}:${i + 1}: raw waitUntil "domcontentloaded" — use gotoStreamed/reloadStreamed from helpers/streamed-navigation (or add a "// streamed-goto-allow: <reason>" line directly above the call).`,
		);
	});
	return out;
}

function walk(dir: string): string[] {
	const out: string[] = [];
	for (const entry of readdirSync(join(root, dir), { withFileTypes: true })) {
		const path = join(dir, entry.name);
		if (entry.isDirectory()) out.push(...walk(path));
		else if (/\.tsx?$/.test(entry.name)) out.push(path);
	}
	return out;
}

// Self-check: fires on a raw goto, quiet on a call with an allow comment
// directly above it, and one allow comment never covers a second call.
const RAW_GOTO = 'await page.goto("/", { waitUntil: "domcontentloaded" });';
const ALLOW_LINE = "// streamed-goto-allow: asserts loader";
const selfChecks: Array<[string, number]> = [
	[RAW_GOTO, 1],
	[`${ALLOW_LINE}\n${RAW_GOTO}`, 0],
	[
		`${ALLOW_LINE}\nawait page.goto("/", {\n  waitUntil: "domcontentloaded",\n});`,
		0,
	],
	[`${ALLOW_LINE}\n${RAW_GOTO}\n${RAW_GOTO}`, 1],
	[
		`${ALLOW_LINE}\nawait page.goto("/", {\n  waitUntil: "domcontentloaded",\n});\nawait page.goto("/b", {\n  waitUntil: "domcontentloaded",\n});`,
		1,
	],
];
if (selfChecks.some(([src, n]) => findViolations("x", src).length !== n)) {
	console.error("check-e2e-streamed-goto self-test failed.");
	process.exit(1);
}

const files = walk(E2E_DIR).filter((p) => p !== HELPER);
const failures = files.flatMap((p) =>
	findViolations(p, readFileSync(join(root, p), "utf8")),
);

if (failures.length > 0) {
	console.error(`e2e streamed-goto gate failed:\n${failures.join("\n")}`);
	process.exit(1);
}
console.log(`e2e streamed-goto gate passed (${files.length} files scanned).`);
