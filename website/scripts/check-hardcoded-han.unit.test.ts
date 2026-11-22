import { describe, expect, it } from "vitest";
import {
	buildCounts,
	type Counts,
	canUpdate,
	collectHanLiterals,
	compare,
	isExcluded,
	parseArgs,
	run,
	runBaselineAgainst,
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

	it("改名、拆檔、抽元件（原檔仍在）、交叉改名都放行", () => {
		expect(
			canUpdate(
				src({ "lib/renamed.ts": 'export const a = ["中", "文"];' }),
				base,
			).ok,
		).toBe(true);
		const split = src({
			"lib/a.ts": 'export const a = ["中"];',
			"lib/b.ts": 'export const b = "文";',
		});
		expect(canUpdate(split, base).ok).toBe(true);
		const two: Counts = src({
			"lib/a.ts": 'export const a = "甲";',
			"lib/b.ts": 'export const b = "乙";',
		});
		const swapped = src({
			"lib/a.ts": 'export const a = "乙";',
			"lib/b.ts": 'export const b = "甲";',
		});
		expect(canUpdate(swapped, two).ok).toBe(true);
		expect(run(split, base, "update").code).toBe(0);
	});

	it("新檔帶新句子或複製既有句子（全域增加）拒絕", () => {
		const withNew = src({
			"lib/a.ts": 'export const a = ["中", "文"];',
			"lib/new.ts": 'export const n = "新";',
		});
		expect(canUpdate(withNew, base).ok).toBe(false);
		const copied = src({
			"lib/a.ts": 'export const a = ["中", "文"];',
			"lib/new.ts": 'export const n = "文";',
		});
		const r = run(copied, base, "update");
		expect(r.code).toBe(1);
		expect(r.write).toBeUndefined();
		expect(r.lines.join("\n")).toContain("文");
	});

	it("總數不增加但某字面值的全域次數增加也拒絕", () => {
		const cur = src({ "lib/a.ts": 'export const a = ["中", "中"];' });
		const r = canUpdate(cur, base);
		expect(r.ok).toBe(false);
		expect(r.increases).toEqual([{ literal: "中", from: 1, to: 2 }]);
	});
});

describe("init 模式與 baseline 不存在", () => {
	const cur = src({ "lib/a.ts": 'export const a = "中";' });

	it("init 模式：baseline 不存在時建立", () => {
		const r = run(cur, null, "init");
		expect(r.code).toBe(0);
		expect(JSON.parse(r.write ?? "")).toEqual(cur);
	});

	it("init 模式：baseline 已存在時拒絕且不寫檔", () => {
		const r = run(cur, cur, "init");
		expect(r.code).toBe(1);
		expect(r.write).toBeUndefined();
	});

	it("update 與檢查模式：baseline 不存在時失敗", () => {
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

describe("parseArgs", () => {
	it("合法組合", () => {
		expect(parseArgs([])).toEqual({ mode: "check" });
		expect(parseArgs(["--init"])).toEqual({ mode: "init" });
		expect(parseArgs(["--update"])).toEqual({ mode: "update" });
		expect(parseArgs(["--check-baseline-against", "x.json"])).toEqual({
			mode: "against",
			path: "x.json",
		});
	});

	it("未知參數、--init 與 --update 同時、缺路徑都報錯", () => {
		for (const argv of [
			["--bogus"],
			["--update", "--bogus"],
			["--init", "--update"],
			["--update", "--init"],
			["--check-baseline-against"],
			["--check-baseline-against", "--update"],
		])
			expect(parseArgs(argv), argv.join(" ")).toHaveProperty("error");
	});
});

describe("runBaselineAgainst", () => {
	const prev: Counts = src({ "lib/a.ts": 'export const a = ["中", "文"];' });

	it("相同、減少、檔案之間搬移都通過", () => {
		expect(runBaselineAgainst(prev, prev).code).toBe(0);
		expect(
			runBaselineAgainst(src({ "lib/a.ts": 'export const a = "中";' }), prev)
				.code,
		).toBe(0);
		expect(
			runBaselineAgainst(
				src({ "lib/b.ts": 'export const a = ["中", "文"];' }),
				prev,
			).code,
		).toBe(0);
	});

	it("任一字面值全域次數變大（含新句子、次數加一）失敗", () => {
		const added = runBaselineAgainst(
			src({ "lib/a.ts": 'export const a = ["中", "文", "新"];' }),
			prev,
		);
		expect(added.code).toBe(1);
		expect(added.lines.join("\n")).toContain("新");
		expect(
			runBaselineAgainst(
				src({ "lib/a.ts": 'export const a = ["中", "中", "文"];' }),
				prev,
			).code,
		).toBe(1);
	});
});
