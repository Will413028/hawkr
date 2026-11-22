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
		expect(run(base, base, "check").code).toBe(0);
	});

	it("新檔任何一筆都失敗", () => {
		const cur = src({
			"lib/a.ts": 'export const a = ["中", "中", "文"];',
			"lib/new.ts": 'export const n = "新";',
		});
		const r = run(cur, base, "check");
		expect(r.code).toBe(1);
		expect(r.lines.join("\n")).toContain("lib/new.ts");
	});

	it("既有檔同一字面值次數加一失敗", () => {
		const cur = src({
			"lib/a.ts": 'export const a = ["中", "中", "中", "文"];',
		});
		const r = run(cur, base, "check");
		expect(r.code).toBe(1);
		expect(r.lines.join("\n")).toContain("lib/a.ts");
	});

	it("刪一句加一句（總數不變）也失敗", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "中", "別句"];' });
		expect(run(cur, base, "check").code).toBe(1);
	});

	it("次數減少失敗並要求 --update", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "文"];' });
		const r = run(cur, base, "check");
		expect(r.code).toBe(1);
		expect(r.lines.join("\n")).toContain("--update");
	});
});

describe("run（--update）", () => {
	const base: Counts = src({ "lib/a.ts": 'export const a = ["中", "文"];' });

	it("搬移到別的檔放行並寫出新 baseline", () => {
		const cur = src({ "lib/b.ts": 'export const a = ["中", "文"];' });
		const r = run(cur, base, "update");
		expect(r.code).toBe(0);
		expect(JSON.parse(r.write ?? "")).toEqual(cur);
	});

	it("減少放行", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中"];' });
		expect(run(cur, base, "update").code).toBe(0);
	});

	it("新增一筆拒絕並列出增加的字面值，不寫檔", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "文", "新"];' });
		const r = run(cur, base, "update");
		expect(r.code).toBe(1);
		expect(r.write).toBeUndefined();
		expect(r.lines.join("\n")).toContain("新");
	});

	it("改名放行：舊檔消失、新檔字面值是舊檔的子集", () => {
		const cur = src({ "lib/renamed.ts": 'export const a = ["中", "文"];' });
		expect(canUpdate(cur, base).ok).toBe(true);
		const cur2 = src({ "lib/renamed.ts": 'export const a = ["中"];' });
		expect(canUpdate(cur2, base).ok).toBe(true);
	});

	it("既有檔刪掉的字面值塞進新檔被拒並列出新檔", () => {
		const two: Counts = src({
			"lib/a.ts": 'export const a = ["中", "文"];',
			"lib/keep.ts": 'export const k = "留";',
		});
		const cur = src({
			"lib/a.ts": 'export const a = ["中"];',
			"lib/keep.ts": 'export const k = "留";',
			"lib/new.ts": 'export const n = "文";',
		});
		const r = canUpdate(cur, two);
		expect(r.ok).toBe(false);
		expect(r.unmatchedNewFiles).toEqual(["lib/new.ts"]);
		const out = run(cur, two, "update");
		expect(out.code).toBe(1);
		expect(out.lines.join("\n")).toContain("lib/new.ts");
	});

	it("兩個新檔不能共用同一個舊檔", () => {
		const cur = src({
			"lib/x.ts": 'export const a = "中";',
			"lib/y.ts": 'export const a = "文";',
		});
		expect(canUpdate(cur, base).unmatchedNewFiles).toHaveLength(1);
	});

	it("總數不增加但某字面值的全域次數增加也拒絕", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "中"];' });
		const r = canUpdate(cur, base);
		expect(r.ok).toBe(false);
		expect(r.increases).toEqual([{ literal: "中", from: 1, to: 2 }]);
	});
});

describe("--init 與 baseline 不存在", () => {
	const cur = src({ "lib/a.ts": 'export const a = "中";' });

	it("--init 在 baseline 不存在時建立", () => {
		const r = run(cur, null, "init");
		expect(r.code).toBe(0);
		expect(JSON.parse(r.write ?? "")).toEqual(cur);
	});

	it("--init 在 baseline 已存在時拒絕且不寫檔", () => {
		const r = run(cur, cur, "init");
		expect(r.code).toBe(1);
		expect(r.write).toBeUndefined();
	});

	it("--update 與檢查在 baseline 不存在時失敗", () => {
		for (const mode of ["update", "check"] as const) {
			const r = run(cur, null, mode);
			expect(r.code).toBe(1);
			expect(r.write).toBeUndefined();
			expect(r.lines.join("\n")).toContain("--init");
		}
	});
});

describe("漢字與副檔名", () => {
	it("只有標點或注音的字面值不算", () => {
		expect(
			collectHanLiterals(
				"a.ts",
				'export const a = ["、。", "ㄅㄆ", "「」", "漢"];',
			),
		).toEqual(["漢"]);
	});

	it("js、jsx、mjs、cjs、mts、cts 都掃；.d.ts 不掃", () => {
		for (const f of ["a.js", "a.jsx", "a.mjs", "a.cjs", "a.mts", "a.cts"])
			expect(isExcluded(`lib/${f}`)).toBe(false);
		expect(
			collectHanLiterals("a.jsx", "export const E = () => <b>中</b>;"),
		).toEqual(["中"]);
		expect(collectHanLiterals("a.js", 'export const a = "中";')).toEqual([
			"中",
		]);
		expect(isExcluded("lib/x.d.ts")).toBe(true);
		expect(isExcluded("lib/x.d.mts")).toBe(true);
	});
});

describe("compare", () => {
	it("次數相同沒有差異", () => {
		const c = src({ "lib/a.ts": 'export const a = "中";' });
		expect(compare(c, c)).toEqual({ over: [], under: [] });
	});
});
