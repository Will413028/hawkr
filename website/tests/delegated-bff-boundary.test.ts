import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

/**
 * 代行 BFF 只能有**一個**上游轉送點（56 號 §4.5 L1040）。
 *
 * ⚠️ 為什麼不是靠 `scripts/check-transport.ts`：那道 gate 的述詞是「`fetch(` 之後
 * 120 字內出現 gateway base 識別字」，只認得**行內**組 URL 的寫法。`_upstream.ts`
 * 先把 URL 存進變數（簽章要用同一個字串，行內組會變成兩份會漂移的運算式），
 * 於是它對這個形狀**系統性失明** —— allowlist 裡那一條登記因此是「日後改回行內
 * 寫法時仍然正確」的登記，不是現在的保證。真正的保證是這一支測試。
 *
 * 逾時、header allow-list、message 不透傳的政策集中在一支檔案裡才有意義；第二支
 * 自己打 gateway 的 route，會在沒有任何人發現的情況下少掉其中一項。
 */
const BFF_ROOT = "app/(delegated)/adm";
const FORWARDER = `${BFF_ROOT}/api/_upstream.ts`;
const GATEWAY_BASE_TOKENS = /serverApiBase|apiBaseUrl|gatewayHost|API_BASE/;

test("only _upstream.ts may reach the gateway from the delegated workspace", () => {
	// fileURLToPath 而不是 .pathname：後者會把路徑裡的空白留成 %20，
	// 含空白的 checkout 路徑（例如「Hawkr Web」）下 readFileSync 會 ENOENT。
	// CI 路徑沒空白所以只在本機炸。
	const root = fileURLToPath(new URL("..", import.meta.url));
	const offenders = walk(join(root, BFF_ROOT), root).filter(
		(file) =>
			file !== FORWARDER &&
			GATEWAY_BASE_TOKENS.test(readFileSync(join(root, file), "utf8")),
	);
	assert.deepEqual(
		offenders,
		[],
		"a second gateway forwarder skips the timeout / header allow-list / message policy that _upstream.ts owns",
	);
	// 反向：那一支必須真的存在且真的在組 gateway URL，否則上面的斷言會空過。
	assert.match(
		readFileSync(join(root, FORWARDER), "utf8"),
		GATEWAY_BASE_TOKENS,
		"the single forwarder no longer builds a gateway URL — this test is now vacuous",
	);
});

function walk(dir: string, root: string): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(dir);
	} catch {
		return out;
	}
	for (const entry of entries) {
		const full = join(dir, entry);
		if (statSync(full).isDirectory()) out.push(...walk(full, root));
		else if (/\.(ts|tsx)$/.test(entry)) out.push(relative(root, full));
	}
	return out;
}
