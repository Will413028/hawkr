import {
	mkdirSync,
	mkdtempSync,
	readFileSync,
	rmSync,
	writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
	buildReport,
	type ConstResolver,
	forbidden,
	loadManifest,
	main,
	scanDir,
	scanFallbackDir,
	scanFallbacks,
	scanSource,
} from "../tools/check-enum-text";

const manifest = loadManifest();
const SELLER = "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
const MEASURE_KIND = "hawkr.seller.v1alpha1.MeasureKind";

// 單端的 enum：不在目錄、不是伺服器擁有的。
const single = manifest.find(
	(e) => !e.inCatalog && !e.serverOwned && e.tsModule && e.tsName,
);

const temps: string[] = [];
afterEach(() => {
	for (const d of temps.splice(0)) rmSync(d, { recursive: true, force: true });
});

function tempDir(): string {
	const d = mkdtempSync(join(tmpdir(), "enum-text-"));
	temps.push(d);
	return d;
}

/** 一行 import 接程式本體（行號不位移）；E 是別名，回傳 `form:成員@行`。 */
const PRE = `import { MeasureKind as E } from "${SELLER}";`;
function scan(body: string, pre = PRE, consts?: ConstResolver): string[] {
	return scanSource("a.tsx", `${pre} ${body}`, manifest, consts).map(
		(h) => `${h.form}:${h.member}@${h.line}`,
	);
}

describe("規則 E：if／else", () => {
	it("then 在 E.M 成立時產生文字：if；!== 的 else 也是", () => {
		expect(
			scan(`export function f(k: E) {
	if (k === E.WARNING) return "警告";
	return "";
}`),
		).toEqual(["if:WARNING@2"]);
		expect(
			scan(`export function f(k: E) {
	let t = "";
	if (k !== E.WARNING) {} else { t = "警告"; }
	if (k == E.SUSPENSION) { t = "停權"; }
	return t;
}`),
		).toEqual(["else:WARNING@3", "if:SUSPENSION@4"]);
	});

	it("補集不算：=== 的 else、&& 為假的 else、|| 為真", () => {
		expect(
			scan(`export function f(k: E, ok: boolean) {
	let t = "";
	if (k === E.WARNING) {} else { t = "其他"; }
	if (k === E.SUSPENSION && ok) {} else { t = "其他"; }
	if (k !== E.TERMINATION || ok) { t = "其他"; }
	if (k !== E.WARNING && ok) {} else { t = "其他"; }
	if (k === E.SUSPENSION || ok) { t = "其他"; }
	return t;
}`),
		).toEqual([]);
	});

	it("蘊含：|| 為假、!( !== ) 為真才確定相等", () => {
		expect(
			scan(`export function f(k: E, ok: boolean) {
	let t = "";
	if (k !== E.WARNING || ok) {} else { t = "警告"; }
	if (!(k !== E.SUSPENSION)) t = "停權";
	return t;
}`),
		).toEqual(["else:WARNING@3", "if:SUSPENSION@4"]);
	});

	it("沒有漢字、不是 api 的 enum 不報", () => {
		expect(
			scan(`export function f(k: E, o: Other) {
	if (k === E.WARNING) return "warning";
	if (o === Other.X) return "警告";
	return "";
}`),
		).toEqual([]);
	});
});

describe("規則 E：guard（提早離開）", () => {
	it("then 一定離開、沒有 else：之後的敘述在 !== 時才是確定相等", () => {
		expect(
			scan(`export function f(k: E) {
	if (k !== E.WARNING) return null;
	return "警告";
}`),
		).toEqual(["guard:WARNING@2"]);
		expect(
			scan(`export function f(k: E) {
	if (k !== E.WARNING) throw new Error("x");
	return "警告";
}`),
		).toEqual(["guard:WARNING@2"]);
	});

	it("=== 的 guard 之後是補集，不報；then 沒有離開也不報", () => {
		expect(
			scan(`export function f(k: E) {
	if (k === E.WARNING) return null;
	return "其他";
}`),
		).toEqual([]);
		expect(
			scan(`export function f(k: E) {
	let t = "";
	if (k !== E.WARNING) { t = "x"; }
	return t + "警告";
}`),
		).toEqual([]);
	});
});

describe("規則 E：三元式與短路", () => {
	it("三元式的兩支：=== 的真支、!== 與反向比較的假支", () => {
		expect(
			scan(`export const f = (k: E) => (k === E.WARNING ? "警告" : "");`),
		).toEqual(["ternary:WARNING@1"]);
		expect(
			scan(`export const f = (k: E) => (k == E.WARNING ? "警告" : "");`),
		).toEqual(["ternary:WARNING@1"]);
		expect(
			scan(`export const f = (k: E) => (E.WARNING !== k ? "" : "警告");`),
		).toEqual(["ternary:WARNING@1"]);
	});

	it("補集那一支不報：只有 else 有文字的 ===、只有 then 有文字的 !==", () => {
		expect(
			scan(`export const f = (k: E) => (k === E.WARNING ? null : "其他");`),
		).toEqual([]);
		expect(
			scan(`export const f = (k: E) => (k !== E.WARNING ? "其他" : null);`),
		).toEqual([]);
	});

	it("&& 與 ||（含 JSX）", () => {
		expect(
			scan(`export const a = (k: E) => k === E.WARNING && "警告";
export const b = (k: E) => k !== E.SUSPENSION || "停權";
export const c = (k: E) => <p>{k === E.TERMINATION && <b>終止</b>}</p>;`),
		).toEqual(["and:WARNING@1", "or:SUSPENSION@2", "and:TERMINATION@3"]);
		// 補集：=== 的 ||、!== 的 &&。
		expect(
			scan(`export const a = (k: E) => k === E.WARNING || "其他";
export const b = (k: E) => k !== E.SUSPENSION && "其他";`),
		).toEqual([]);
	});

	it("集合成員檢查：為真才算；!includes 不算", () => {
		expect(
			scan(`export const a = (k: E) => ([E.WARNING, E.SUSPENSION].includes(k) ? "限制" : "");
export const b = (k: E) => new Set([E.WARNING]).has(k) && "限制";
export const c = (k: E) => (![E.WARNING].includes(k) ? "限制" : "");`),
		).toEqual(["ternary:WARNING@1", "ternary:SUSPENSION@1", "and:WARNING@2"]);
	});
});

describe("規則 E：具名的成員集合（K5 的作用域查找）", () => {
	it("const 的 Set／陣列當接收者：has、includes 為真才算", () => {
		expect(
			scan(`const S = new Set([E.WARNING, E.SUSPENSION]);
const L = [E.TERMINATION] as const;
const S2 = new Set(L);
export function f(k: E) {
	return [S.has(k) ? "限制" : "", L.includes(k) && "終止", S2.has(k) ? "終止" : "", !S.has(k) ? "其他" : ""];
}`),
		).toEqual([
			"ternary:WARNING@1",
			"ternary:SUSPENSION@1",
			"and:TERMINATION@2",
			"ternary:TERMINATION@2",
		]);
	});

	it("guard 不用成員檢查（選項驗證）；if 用", () => {
		expect(
			scan(`const OPTIONS = [E.WARNING, E.SUSPENSION];
export function f(k: E, n: string) {
	if (!OPTIONS.includes(k)) return "請選擇種類。";
	if (!n) return "請填寫名稱。";
	return "";
}
export function g(k: E) {
	if (OPTIONS.includes(k)) { return "限制"; }
	return "";
}`),
		).toEqual(["if:WARNING@1", "if:SUSPENSION@1"]);
	});

	it("參數或 let 遮住的名稱、內容不是 enum 成員的集合不算", () => {
		expect(
			scan(`const S = new Set([E.WARNING]);
const T = new Set(["a", "b"]);
export function f(k: E, S: Set<E>) {
	let L = [E.SUSPENSION];
	return [S.has(k) ? "限制" : "", T.has(k) ? "其他" : "", L.includes(k) ? "停權" : ""];
}`),
		).toEqual([]);
	});
});

describe("規則 E：switch 與 case", () => {
	it("return、賦值、throw、widget；fall-through 的前幾個 case 一起算", () => {
		expect(
			scan(`export function f(k: E) {
	let label = "";
	switch (k) {
		case E.WARNING:
		case E.SUSPENSION:
			return "限制";
		case E.TERMINATION:
			label = "終止合作";
			break;
	}
	return label;
}
export function g(k: E) {
	switch (k) {
		case E.WARNING:
			throw new Error("限制");
		case E.SUSPENSION:
			return <b>停權</b>;
	}
}`),
		).toEqual([
			"case:WARNING@4",
			"case:SUSPENSION@5",
			"case:TERMINATION@7",
			"case:WARNING@15",
			"case:SUSPENSION@17",
		]);
	});

	it("default 是補集不算；case 沒有漢字不報", () => {
		expect(
			scan(`export function f(k: E) {
	switch (k) {
		case E.WARNING:
			return null;
		default:
			return "其他";
	}
}
export function g(k: E) {
	switch (k) {
		case E.WARNING:
			return "warning";
	}
}`),
		).toEqual([]);
	});

	it("fall-through 進 default：不算 default 的文字，之後的 case 只算自己的成員", () => {
		expect(
			scan(`export function f(k: E) {
	switch (k) {
		case E.WARNING:
		default:
			return "其他";
		case E.SUSPENSION:
			return "停權";
	}
}`),
		).toEqual(["case:SUSPENSION@6"]);
	});

	it("switch (true) 的 case 是比較", () => {
		expect(
			scan(`export function f(k: E) {
	switch (true) {
		case k === E.WARNING:
			return "警告";
		case k !== E.SUSPENSION:
			return "其他";
	}
}`),
		).toEqual(["case:WARNING@3"]);
	});
});

describe("規則 E：key 與 pair", () => {
	it("computed key：值（含巢狀、template）有漢字", () => {
		expect(
			scan(`export const L: Record<E, unknown> = {
	[E.WARNING]: "警告",
	[E.SUSPENSION]: { label: \`停權\` },
	[E.TERMINATION]: 3,
};`),
		).toEqual(["key:WARNING@2", "key:SUSPENSION@3"]);
		const ns = scanSource(
			"a.ts",
			`import * as s from "${SELLER}";
export const L = { [s.MeasureKind.WARNING]: \`警告\` };`,
			manifest,
		);
		expect(ns.map((h) => h.form)).toEqual(["key"]);
	});

	it("配對：物件、元組、Map、引數、<option>", () => {
		expect(
			scan(`export const a = { value: E.WARNING, label: "警告" };
export const b = [[E.SUSPENSION, "停權"]];
export const c = new Map([[E.TERMINATION, "終止"]]);
export const d = f(E.WARNING, "警告");
export const e = <option value={E.SUSPENSION}>停權</option>;
export const g = <Item kind={E.TERMINATION} label="終止" />;`),
		).toEqual([
			"pair:WARNING@1",
			"pair:SUSPENSION@2",
			"pair:TERMINATION@3",
			"pair:WARNING@4",
			"pair:SUSPENSION@5",
			"pair:TERMINATION@6",
		]);
	});

	it("配對不進入巢狀函式與子元素", () => {
		expect(
			scan(`export const a = f(E.WARNING, () => "警告");
export const b = <X kind={E.WARNING}><p>警告</p></X>;
export const c = { value: E.WARNING, render: () => "警告" };`),
		).toEqual([]);
	});

	it("值號 0（UNSPECIFIED）不是選擇子", () => {
		expect(
			scan(`export const a = <option value={E.UNSPECIFIED}>請選擇</option>;
export const b = { [E.UNSPECIFIED]: "請選擇" };
export const c = (k: E) => (k === E.UNSPECIFIED ? "請選擇" : "");`),
		).toEqual([]);
	});
});

describe("規則 E：別名（一層一層代入）", () => {
	it("const 別名是比較或 cond ? x : null；let 不代入", () => {
		expect(
			scan(`export function f(k: E, o: object) {
	const isW = k === E.WARNING;
	const p = k === E.SUSPENSION ? o : null;
	const q = isW;
	return [isW ? "警告" : "", p && "停權", q && "x警告"];
}
export function g(k: E) {
	let t = k === E.WARNING;
	return t ? "警告" : "";
}`),
		).toEqual(["ternary:WARNING@2", "and:SUSPENSION@3", "and:WARNING@2"]);
	});

	it("alias != null、alias === undefined 的假支；非條件式的別名不當成有值", () => {
		expect(
			scan(`export function f(k: E, o: object) {
	const p = k === E.WARNING ? o : undefined;
	const b = k === E.SUSPENSION;
	return [p != null && "撤回", p === undefined ? "" : "警告", b != null && "其他", p == null && "沒有"];
}`),
		).toEqual(["and:WARNING@2", "ternary:WARNING@2"]);
	});

	it("參數遮住同名的別名", () => {
		expect(
			scan(`export function f(k: E) {
	const t = k === E.WARNING;
	return ((t: boolean) => (t ? "警告" : ""))(true);
}`),
		).toEqual([]);
	});
});

describe("規則 E：產生文字（常數）", () => {
	it("頂層與函式內的 const，引用它的區域算有文字", () => {
		expect(
			scan(`const T = "警告";
export function f(k: E) {
	const u = "停權";
	return [k === E.WARNING ? T : "", k === E.SUSPENSION ? u : ""];
}`),
		).toEqual(["ternary:WARNING@4", "ternary:SUSPENSION@4"]);
	});

	it("不含漢字的常數、屬性名、let 不算", () => {
		expect(
			scan(`const T = "warning";
export function f(k: E, o: { T: string }) {
	let u = "停權";
	return [k === E.WARNING ? T : "", k === E.SUSPENSION ? u : "", k === E.TERMINATION ? o.T : ""];
}`),
		).toEqual([]);
	});

	it("跨檔匯入的常數（resolver 給的名稱）", () => {
		const consts: ConstResolver = (spec) =>
			spec === "./text" ? new Set(["T"]) : undefined;
		const pre = `${PRE} import { T, N as U } from "./text"; import { M } from "./other";`;
		expect(
			scan(
				`export const a = (k: E) => (k === E.WARNING ? T : "");
export const b = (k: E) => (k === E.SUSPENSION ? U : "");
export const c = (k: E) => (k === E.TERMINATION ? M : "");`,
				pre,
				consts,
			),
		).toEqual(["ternary:WARNING@1"]);
		// 沒有 resolver：只看同檔，匯入的不算。
		expect(
			scan(`export const a = (k: E) => (k === E.WARNING ? T : "");`, pre),
		).toEqual([]);
	});

	it('同檔的物件常數：TEXT.key、TEXT["key"] 的值含漢字才算', () => {
		expect(
			scan(`const TEXT = { term: "合作已終止", en: "ended", fn: () => "停權" } as const;
export function f(k: E) {
	return [k === E.WARNING ? TEXT.term : "", k === E.SUSPENSION ? TEXT["fn"]() : "", k === E.TERMINATION ? TEXT.en : "", k === E.WARNING ? TEXT.none : ""];
}`),
		).toEqual(["ternary:WARNING@3", "ternary:SUSPENSION@3"]);
	});

	it("解構的參數、catch 變數與解構宣告遮住匯入的常數", () => {
		const consts: ConstResolver = () => new Set(["T"]);
		const pre = `${PRE} import { T } from "./text";`;
		expect(
			scan(
				`export const a = ({ T }: { T: string }) => (E.WARNING === (0 as unknown as E) ? T : "");
export const b = (k: E, [T]: string[]) => (k === E.WARNING ? T : "");
export function c(k: E, o: { T: string }) { const { T } = o; return k === E.SUSPENSION ? T : ""; }
export function d(k: E) { try { f(); } catch (T) { return k === E.TERMINATION ? T : ""; } }
export const e = (k: E) => (k === E.WARNING ? T : "");`,
				pre,
				consts,
			),
		).toEqual(["ternary:WARNING@5"]);
	});

	it("export { U } 的常數（宣告在別處）算跨檔常數", () => {
		const dir = tempDir();
		writeFileSync(
			join(dir, "text.ts"),
			`const U = "警告";\nconst V = "停權";\nconst PRIVATE = "私有";\nexport { U, V as W };\n`,
		);
		writeFileSync(
			join(dir, "use.ts"),
			`${PRE}\nimport { U, W, PRIVATE } from "./text";\nexport const a = (k: E) => (k === E.WARNING ? U : "");\nexport const b = (k: E) => (k === E.SUSPENSION ? W : "");\nexport const c = (k: E) => (k === E.TERMINATION ? PRIVATE : "");\n`,
		);
		const hits = scanDir(dir, manifest).filter((h) => h.file === "use.ts");
		expect(hits.map((h) => `${h.form}:${h.member}@${h.line}`)).toEqual([
			"ternary:WARNING@3",
			"ternary:SUSPENSION@4",
		]);
	});

	it("scanDir 解析相對路徑與 tsconfig 別名，只追樹內的、含漢字的 export const", () => {
		const base = tempDir();
		const dir = join(base, "client");
		mkdirSync(join(dir, "src", "lib"), { recursive: true });
		writeFileSync(
			join(dir, "tsconfig.json"),
			`{ // 註解也要能解析
	"compilerOptions": { "paths": { "@/*": ["./src/*"] } } }`,
		);
		writeFileSync(
			join(dir, "src", "lib", "text.ts"),
			`export const T = "警告";
export const EN = "warning";
const PRIVATE = "私有";
export { PRIVATE };`,
		);
		writeFileSync(
			join(dir, "src", "use.ts"),
			`${PRE}
import { T, EN, PRIVATE } from "./lib/text";
import { T as T2 } from "@/lib/text";
import { T as OUT } from "../../outside";
export const a = (k: E) => (k === E.WARNING ? T : "");
export const b = (k: E) => (k === E.SUSPENSION ? EN : "");
export const c = (k: E) => (k === E.TERMINATION ? PRIVATE : "");
export const d = (k: E) => (k === E.WARNING ? T2 : "");
export const e = (k: E) => (k === E.WARNING ? OUT : "");
`,
		);
		writeFileSync(join(base, "outside.ts"), `export const T = "樹外";`);
		const hits = scanDir(dir, manifest).filter((h) =>
			h.file.endsWith("use.ts"),
		);
		expect(hits.map((h) => `${h.form}:${h.member}@${h.line}`)).toEqual([
			"ternary:WARNING@5",
			"ternary:TERMINATION@7",
			"ternary:WARNING@8",
		]);
	});
});

describe("manifest 與回報", () => {
	it("單端 enum：任何形式都不失敗；只有對應表（case、key、pair）進 report", () => {
		expect(single).toBeDefined();
		const e = single;
		if (!e?.tsModule || !e.tsName) return;
		const pre = `import { ${e.tsName} as S } from "${e.tsModule}";`;
		const src = (body: string) =>
			scanSource("a.ts", `${pre} ${body}`, manifest);
		const ternary = src(
			`export const f = (k: S) => (k === S.X ? "文字" : "");`,
		);
		expect(ternary).toHaveLength(1);
		expect(forbidden(ternary, manifest)).toEqual([]);
		expect(buildReport(ternary).enums).toEqual({});
		const key = src(`export const L = { [S.X]: "文字" };`);
		expect(forbidden(key, manifest)).toEqual([]);
		expect(buildReport(key).enums).toEqual({ [e.name]: ["a.ts:1"] });
		for (const body of [
			`export const f = (k: S) => { switch (k) { case S.X: return "文字"; } };`,
			`export const L = { value: S.X, label: "文字" };`,
		]) {
			expect(buildReport(src(body)).enums).toEqual({ [e.name]: ["a.ts:1"] });
		}
	});

	it("目錄與伺服器擁有的 enum：條件形式也擋", () => {
		const hits = scanSource(
			"a.ts",
			`${PRE} export const f = (k: E) => (k === E.WARNING ? "警告" : "");`,
			manifest,
		);
		expect(forbidden(hits, manifest)).toHaveLength(1);
		const owned = manifest.find((e) => e.serverOwned && e.tsModule && e.tsName);
		if (!owned?.tsModule || !owned.tsName)
			throw new Error("manifest 沒有伺服器擁有的 enum");
		const ownedHits = scanSource(
			"a.ts",
			`import { ${owned.tsName} as S } from "${owned.tsModule}"; export const f = (k: S) => (k === S.X ? "文字" : "");`,
			manifest,
		);
		expect(forbidden(ownedHits, manifest)).toHaveLength(1);
	});

	it("只算從 @hawkr/api 生成檔引入的 enum：同名但來自別處（CodeMeta）的不報", () => {
		const body = `export const L = { [E.WARNING]: "警告" };`;
		expect(
			scan(
				body,
				`import { MeasureKind as E } from "./core/codemeta/code_meta";`,
			),
		).toEqual([]);
		expect(scan(body)).toEqual(["key:WARNING@1"]);
	});
});

describe("R3：userMessage 後面的寫死後備句", () => {
	it('`userMessage(…) ?? "…"` 與 `||`、括號、template 都擋', () => {
		const src = `import { userMessage } from "@hawkr/api/connect/errors";
export const a = userMessage(e, d) ?? "系統錯誤";
export const b = (userMessage(e, d) || \`失敗\`);
export const c = <p>{userMessage(e, d) ?? "無法連接到 API"}</p>;
export const ok1 = userMessage(e, d) ?? other;
export const ok2 = errorText(e, d);
export const ok3 = userMessage(e, d);
`;
		expect(scanFallbacks("a.tsx", src).map((h) => h.line)).toEqual([2, 3, 4]);
	});

	it('`(cond && userMessage(…)) ?? "…"` 與 kind 三元式的寫死後備句都擋', () => {
		const src = `export const a = (err && userMessage(err, d)) ?? "失敗";
export const b = failure.kind === "message" ? failure.message : "系統錯誤";
export const c = failure?.kind === "message"
	? failure.message
	: \`載入失敗\`;
export const d1 = failure.kind !== "message" ? "儲存失敗" : failure.message;
export const e1 = (err && userMessage(err, d)) || "失敗";
`;
		expect(scanFallbacks("a.ts", src).map((h) => h.line)).toEqual([
			1, 2, 3, 6, 7,
		]);
	});

	it("目錄查詢與參數當後備不擋", () => {
		const src = `export const a = userMessage(e, d) ?? t("x");
export const b = (err && userMessage(err, d)) ?? ut.common.error;
export const c = failure.kind === "message" ? failure.message : t("x");
export const d1 = failure.kind === "message" ? failure.message : fallback;
export const e1 = failure.kind === "fields" ? failure.message : "x";
export const f = failure.kind === "message" ? failure.message : errorText(e, d);
`;
		expect(scanFallbacks("a.ts", src)).toEqual([]);
	});

	it("掃目錄略過測試；main 對命中回 1", () => {
		const dir = tempDir();
		writeFileSync(
			join(dir, "bad.ts"),
			`export const a = userMessage(e, d) ?? "系統錯誤";\n`,
		);
		writeFileSync(
			join(dir, "bad.test.ts"),
			`export const a = userMessage(e, d) ?? "系統錯誤";\n`,
		);
		expect(scanFallbackDir(dir).map((h) => h.file)).toEqual(["bad.ts"]);
		const err = vi.spyOn(console, "error").mockImplementation(() => {});
		expect(main([dir])).toBe(1);
		err.mockRestore();
	});
});

describe("命令列", () => {
	it("掃目錄：略過測試、gen、node_modules；寫出 report；有禁止的表就非零結束", () => {
		const dir = tempDir();
		mkdirSync(join(dir, "lib"));
		mkdirSync(join(dir, "node_modules"));
		mkdirSync(join(dir, "lib", "gen"));
		const bad = `import { MeasureKind } from "${SELLER}";
export const L = { [MeasureKind.WARNING]: "警告" };`;
		writeFileSync(join(dir, "lib", "labels.ts"), bad);
		writeFileSync(join(dir, "lib", "labels.test.ts"), bad);
		writeFileSync(join(dir, "lib", "gen", "x.ts"), bad);
		writeFileSync(join(dir, "node_modules", "x.ts"), bad);
		expect(scanDir(dir, manifest).map((h) => h.file)).toEqual([
			join("lib", "labels.ts"),
		]);

		const report = join(tempDir(), "report.json");
		const err = vi.spyOn(console, "error").mockImplementation(() => {});
		expect(main(["--report", report, dir])).toBe(1);
		err.mockRestore();
		expect(JSON.parse(readFileSync(report, "utf8"))).toEqual({
			enums: { [MEASURE_KIND]: [`${join("lib", "labels.ts")}:2`] },
		});
	});

	it("乾淨的目錄回 0，缺參數回 2", () => {
		const dir = tempDir();
		writeFileSync(join(dir, "ok.ts"), `export const x = "警告";\n`);
		expect(main([dir])).toBe(0);
		const err = vi.spyOn(console, "error").mockImplementation(() => {});
		expect(main([])).toBe(2);
		err.mockRestore();
	});
});
