import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const SOURCE_FILE = /\.(?:ts|tsx|css|mdx?)$/u;

async function sourceFiles(dir: string): Promise<string[]> {
	const entries = await readdir(dir, { withFileTypes: true });
	const nested = await Promise.all(
		entries.map(async (entry) => {
			const path = join(dir, entry.name);
			if (entry.isDirectory()) return sourceFiles(path);
			return SOURCE_FILE.test(entry.name) ? [path] : [];
		}),
	);
	return nested.flat();
}

test("admin/src 沒有 Remix Icon 的 ri-* 圖示 class", async () => {
	const offenders: string[] = [];
	for (const file of await sourceFiles("src")) {
		const text = await readFile(file, "utf8");
		for (const match of text.matchAll(/\bri-[a-z0-9-]+/gu)) {
			offenders.push(`${file}: ${match[0]}`);
		}
	}
	assert.deepEqual(offenders, [], offenders.join("\n"));
});

test("root layout 不載入 Remix Icon，CSP 不放行 jsDelivr", async () => {
	const [layout, nextConfig] = await Promise.all([
		readFile("src/app/layout.tsx", "utf8"),
		readFile("next.config.ts", "utf8"),
	]);
	assert.doesNotMatch(layout, /remixicon/iu);
	assert.doesNotMatch(nextConfig, /cdn\.jsdelivr\.net/u);
});
