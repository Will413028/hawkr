import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { EventSchemas } from "@ag-ui/core";
import { describe, expect, it } from "vitest";

// 這些 .sse 是 gateway 產生的 canonical frame，直接讀 gateway 的 golden fixture，不另存拷貝。
// 這支測試的價值不在覆蓋率，而在於：後端改了 wire 格式 ⇒ gateway 的 golden test 要求重新產生
// fixture；新格式不合 AG-UI spec ⇒ 這裡擋下。
const FIXTURE_DIR = join(
	__dirname,
	"../../../../backend/gateway/internal/proxy/testdata/agui-frames",
);

describe("AG-UI fixture 合規性", () => {
	const files = readdirSync(FIXTURE_DIR).filter((f) => f.endsWith(".sse"));

	it("fixture 目錄不是空的", () => {
		expect(files.length).toBeGreaterThan(0);
	});

	for (const file of files) {
		it(`${file} 的每個 frame 都通過官方 schema`, () => {
			const raw = readFileSync(join(FIXTURE_DIR, file), "utf8");
			const blocks = raw.split("\n\n").filter((b) => b.trim().length > 0);
			expect(blocks.length).toBeGreaterThan(0);

			for (const block of blocks) {
				const dataLines = block
					.split("\n")
					.filter((l) => l.startsWith("data:"))
					.map((l) => l.slice(5).replace(/^\s/, ""));
				expect(dataLines.length).toBeGreaterThan(0);
				expect(block).not.toMatch(/^event:/m);

				const parsed = EventSchemas.safeParse(JSON.parse(dataLines.join("\n")));
				expect(
					parsed.success,
					parsed.success ? "" : JSON.stringify(parsed.error.issues),
				).toBe(true);
			}
		});
	}
});
