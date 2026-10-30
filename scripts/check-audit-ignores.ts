#!/usr/bin/env node

/**
 * `pnpm audit --prod` 的忽略清單與 OSV-Scanner 基準的一致性檢查。
 *
 * pnpm-workspace.yaml 的 auditConfig.ignoreGhsas 讓 `pnpm audit --prod` 略過已審查的基準，
 * 但期限只有 osv-scanner.toml 的 IgnoredVulns（ignoreUntil）會執行。所以規定：
 *   1. ignoreGhsas 的每一筆都必須也在 osv-scanner.toml 的 IgnoredVulns 裡，而且是 npm 的基準
 *      （reason 以「基準：npm」開頭）——這裡不能有 OSV 看不到的私下放行。
 *   2. 沒有重複。
 * 期限到了：OSV-Scanner 失敗，修好或延期都在 osv-scanner.toml；這裡的項目隨之刪除。
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");

const workspace = readFileSync(join(root, "pnpm-workspace.yaml"), "utf8").split("\n");
const start = workspace.findIndex((l) => /^\s{2}ignoreGhsas:\s*$/.test(l));
if (start < 0) {
	console.error("check-audit-ignores: pnpm-workspace.yaml 沒有 auditConfig.ignoreGhsas。");
	process.exit(1);
}
const ignored: string[] = [];
for (const line of workspace.slice(start + 1)) {
	const m = /^\s{4}-\s+(GHSA-[a-z0-9-]+)\s*$/.exec(line);
	if (!m) break;
	ignored.push(m[1]);
}

const toml = readFileSync(join(root, "osv-scanner.toml"), "utf8");
const npmBaseline = new Set<string>();
for (const block of toml.split("[[IgnoredVulns]]").slice(1)) {
	const id = /^id\s*=\s*"([^"]+)"/m.exec(block)?.[1];
	const reason = /^reason\s*=\s*"([^"]*)"/m.exec(block)?.[1] ?? "";
	if (id && reason.startsWith("基準：npm")) npmBaseline.add(id);
}

const errors: string[] = [];
if (ignored.length === 0) errors.push("ignoreGhsas 是空的（解析失敗或誤刪）。");
for (const id of new Set(ignored.filter((v, i) => ignored.indexOf(v) !== i))) {
	errors.push(`${id} 重複。`);
}
for (const id of ignored) {
	if (!npmBaseline.has(id)) {
		errors.push(`${id} 不在 osv-scanner.toml 的 npm 基準；期限由 OSV 執行，請先在那裡登記（或從這裡刪除）。`);
	}
}
if (errors.length > 0) {
	console.error(`check-audit-ignores: ${errors.length} 個問題\n- ${errors.join("\n- ")}`);
	process.exit(1);
}
console.log(`check-audit-ignores: ${ignored.length} 筆 ignoreGhsas 都在 osv-scanner.toml 的 npm 基準內。`);
