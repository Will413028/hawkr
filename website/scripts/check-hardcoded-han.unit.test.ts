import { describe, expect, it } from "vitest";
import {
	buildCounts,
	type Counts,
	canUpdate,
	collectHanLiterals,
	compare,
	isExcluded,
	run,
} from "./check-hardcoded-han";

const PH = "$" + "{}";
const src = (entries: Record<string, string>) =>
	buildCounts(new Map(Object.entries(entries)));

describe("collectHanLiterals", () => {
	it("字串、無插值樣板、屬性字串、JSX 文字都算；沒有漢字的不算", () => {
		const text = `export const a = "你好"; export const b = \`樣板\`; export const c = "plain";
export const E = () => <div title="標題">內文 <b>粗體</b>{" "}</div>;`;
		expect(collectHanLiterals("a.tsx", text)).toEqual([
			"你好",
			"樣板",
			"標題",
			"內文",
			"粗體",
		]);
	});

	it("樣板字串的各段合併成一筆，插值以佔位表示", () => {
		expect(
			collectHanLiterals(
				"a.ts",
				["export const a = `共 ", "{n} 件，", "{m} 筆`;"].join("$"),
			),
		).toEqual([`共 ${PH} 件，${PH} 筆`]);
	});

	it("註解不算", () => {
		expect(
			collectHanLiterals("a.ts", "// 註解\n/* 中文 */\nexport const a = 1;"),
		).toEqual([]);
	});
});

describe("isExcluded", () => {
	it("代行工作區、測試、生成檔不算；一般檔算", () => {
		expect(isExcluded("app/(delegated)/x/page.tsx")).toBe(true);
		expect(isExcluded("features/a/b.test.tsx")).toBe(true);
		expect(isExcluded("features/a/b.unit.test.ts")).toBe(true);
		expect(isExcluded("lib/__tests__/x.ts")).toBe(true);
		expect(isExcluded("lib/x.gen.ts")).toBe(true);
		expect(isExcluded("lib/x.g.ts")).toBe(true);
		expect(isExcluded("features/a/b.tsx")).toBe(false);
		expect(isExcluded("app/(commerce)/page.tsx")).toBe(false);
	});

	it("buildCounts 略過排除的檔", () => {
		expect(
			src({
				"app/(delegated)/a.tsx": 'export const a = "中";',
				"lib/a.test.ts": 'export const a = "中";',
				"lib/a.gen.ts": 'export const a = "中";',
			}),
		).toEqual({});
	});
});

describe("run（檢查模式）", () => {
	const base: Counts = src({
		"lib/a.ts": 'export const a = ["中", "中", "文"];',
	});

	it("與 baseline 相同則通過", () => {
		expect(run(base, base, false).code).toBe(0);
	});

	it("新檔任何一筆都失敗", () => {
		const cur = src({
			"lib/a.ts": 'export const a = ["中", "中", "文"];',
			"lib/new.ts": 'export const n = "新";',
		});
		const r = run(cur, base, false);
		expect(r.code).toBe(1);
		expect(r.lines.join("\n")).toContain("lib/new.ts");
	});

	it("既有檔同一字面值次數加一失敗", () => {
		const cur = src({
			"lib/a.ts": 'export const a = ["中", "中", "中", "文"];',
		});
		const r = run(cur, base, false);
		expect(r.code).toBe(1);
		expect(r.lines.join("\n")).toContain("lib/a.ts");
	});

	it("刪一句加一句（總數不變）也失敗", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "中", "別句"];' });
		expect(run(cur, base, false).code).toBe(1);
	});

	it("次數減少失敗並要求 --update", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "文"];' });
		const r = run(cur, base, false);
		expect(r.code).toBe(1);
		expect(r.lines.join("\n")).toContain("--update");
	});
});

describe("run（--update）", () => {
	const base: Counts = src({ "lib/a.ts": 'export const a = ["中", "文"];' });

	it("搬移到別的檔放行並寫出新 baseline", () => {
		const cur = src({ "lib/b.ts": 'export const a = ["中", "文"];' });
		const r = run(cur, base, true);
		expect(r.code).toBe(0);
		expect(JSON.parse(r.write ?? "")).toEqual(cur);
	});

	it("減少放行", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中"];' });
		expect(run(cur, base, true).code).toBe(0);
	});

	it("新增一筆拒絕並列出增加的字面值，不寫檔", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "文", "新"];' });
		const r = run(cur, base, true);
		expect(r.code).toBe(1);
		expect(r.write).toBeUndefined();
		expect(r.lines.join("\n")).toContain("新");
	});

	it("總數不增加但某字面值的全域次數增加也拒絕", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "中"];' });
		const r = canUpdate(cur, base);
		expect(r.ok).toBe(false);
		expect(r.increases).toEqual([{ literal: "中", from: 1, to: 2 }]);
	});
});

describe("compare", () => {
	it("次數相同沒有差異", () => {
		const c = src({ "lib/a.ts": 'export const a = "中";' });
		expect(compare(c, c)).toEqual({ over: [], under: [] });
	});
});
