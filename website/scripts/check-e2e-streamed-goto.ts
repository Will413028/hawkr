#!/usr/bin/env node

/**
 * e2e streamed-navigation gate: specs under tests/e2e must navigate through
 * gotoStreamed / reloadStreamed (tests/e2e/helpers/streamed-navigation.ts),
 * never with a raw `waitUntil: "domcontentloaded"`. Every route streams, and
 * before React reveals the streamed content a hidden `body > div[hidden]
 * [id^="S:"]` copy coexists with the hydrated page, so locators can hit
 * strict-mode duplicates. A spec that must assert during streaming opts out
 * with a `// streamed-goto-allow: <reason>` line comment on the line above.
 */

import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";

const root = process.cwd();
const E2E_DIR = "tests/e2e";
const HELPER = "tests/e2e/helpers/streamed-navigation.ts";
const RAW = /waitUntil:\s*["']domcontentloaded["']/;
const ALLOW = /^\s*\/\/\s*streamed-goto-allow:\s*\S+/;

export function findViolations(path: string, text: string): string[] {
	const lines = text.split("\n");
	const out: string[] = [];
	lines.forEach((line, i) => {
		if (!RAW.test(line)) return;
		if (i > 0 && ALLOW.test(lines[i - 1] ?? "")) return;
		out.push(
			`${path}:${i + 1}: raw waitUntil "domcontentloaded" — use gotoStreamed/reloadStreamed from helpers/streamed-navigation (or add a "// streamed-goto-allow: <reason>" line above).`,
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

// Self-check: fires on a raw goto, quiet on an opted-out one.
if (
	findViolations("x", 'page.goto("/", { waitUntil: "domcontentloaded" });')
		.length !== 1 ||
	findViolations(
		"x",
		'// streamed-goto-allow: asserts loader\npage.goto("/", { waitUntil: "domcontentloaded" });',
	).length !== 0
) {
	console.error("check-e2e-streamed-goto self-test failed.");
	process.exit(1);
}

const files = walk(E2E_DIR).filter((p) => p !== HELPER);
const failures = files.flatMap((p) =>
	findViolations(
		relative(root, join(root, p)),
		readFileSync(join(root, p), "utf8"),
	),
);

if (failures.length > 0) {
	console.error(`e2e streamed-goto gate failed:\n${failures.join("\n")}`);
	process.exit(1);
}
console.log(`e2e streamed-goto gate passed (${files.length} files scanned).`);
