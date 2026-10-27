import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const source = readFileSync(join(__dirname, "TaiwanPriceMap.tsx"), "utf8");

describe("TaiwanPriceMap 的 maplibre 顏色", () => {
	it("style 物件不含 CSS var()，maplibre style-spec 只接受色值", () => {
		expect(source).not.toContain("var(");
	});

	it("墨色是字面 #rrggbb", () => {
		expect(source).toMatch(/const PRICE_MAP_INK = "#[0-9a-f]{6}";/);
	});
});
