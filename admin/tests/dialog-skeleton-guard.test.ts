import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";
import {
	type Finding,
	scanCss,
	scanSource,
} from "./fixtures/dialog-skeleton-scan";

/**
 * 後台的對話框只有一個出處：src/components/UI/Dialog.tsx。
 *
 * 任何其他檔案出現對話框的骨架（`role` 為 dialog／alertdialog、`aria-modal`、`*-modal-overlay`、
 * `*-modal-content`、`adm-operation-dialog*` class，或直接呼叫 focus trap／Escape／背景隔離的 hooks）＝又有一份
 * 複製品，它的 focus trap、Escape、背景 inert 與還焦就沒有人看管。新增對話框用 `<Dialog>`，不要改這份清單。
 *
 * 偵測用 TypeScript compiler API 解析 .ts／.tsx（字串字面值、`{"…"}`、樣板字串、`clsx("…")`、條件式、
 * 字串相接、物件屬性、`setAttribute` 都看得到），不是掃文字；.css 掃 class 選擇器。
 * 這是「掃全 src/」而不是檔名清單：舊的清單測試在新增對話框卻忘了登記時照樣全綠（58A 陷阱 T26）。
 */
const SHARED = "src/components/UI/Dialog.tsx";

/**
 * 骨架字樣出現在這些檔案是正當的，而且**只允許列出的種類**（其餘種類照樣算違規）：
 * 整個檔案豁免會讓這些檔案日後多長出一份自己的對話框也看不見。
 */
const EXEMPT_SOURCE: Record<string, { why: string; kinds: Finding["kind"][] }> =
	{
		[SHARED]: {
			why: "唯一的對話框元件",
			kinds: ["role", "aria-modal", "class", "hook", "exempt"],
		},
		"src/components/Layout/Sidebar.tsx": {
			why: "手機版導覽抽屜（role=dialog 的導覽，不是確認框），a11y 由 admin-visual-system.test.ts 守",
			kinds: ["role", "aria-modal"],
		},
		"src/app/admin/layout.tsx": {
			why: "導覽抽屜呼叫 useEscapeKey／useFocusTrap／useModalStack",
			kinds: ["hook"],
		},
		"src/components/UI/Toast.tsx": {
			why: "toast 的 live region 跟著最上層 modal，並是 data-modal-exempt 的唯一使用處",
			kinds: ["hook", "exempt"],
		},
		"src/lib/a11y.ts": {
			why: "hooks 與豁免屬性的定義處",
			kinds: ["exempt"],
		},
	};

/** 這些樣式表定義對話框骨架 class 是正當的（Dialog 自己 import 的那幾份）。 */
const EXEMPT_CSS: Record<string, string> = {
	"src/styles/AdminVisualRefresh.css":
		"共用 Dialog 的 standard／side 骨架（.modal-*）樣式的唯一出處，含 Dialog 根字型",
	"src/components/UI/Dialog.css":
		"共用 Dialog 的 operation 變體骨架（.adm-operation-dialog-*），Dialog.tsx 自己 import",
};

async function filesUnder(root: string, ext: RegExp): Promise<string[]> {
	const entries = await readdir(root, { withFileTypes: true, recursive: true });
	return entries
		.filter((e) => e.isFile() && ext.test(e.name))
		.map((e) => `${e.parentPath}/${e.name}`.replaceAll("\\", "/"));
}

const describe = (f: Finding) => `${f.kind}:${f.detail}@${f.line}`;

test("src/ 內只有共用 Dialog 畫對話框骨架（.ts、.tsx 與 .css）", async () => {
	const sources = await filesUnder("src", /\.tsx?$/u);
	const styles = await filesUnder("src", /\.css$/u);
	assert.ok(sources.length > 100, "掃描範圍不對：src 底下的 ts／tsx 太少");
	assert.ok(
		sources.includes(SHARED),
		"找不到共用 Dialog，路徑改了就要同步改這支測試",
	);

	const offenders: string[] = [];
	for (const file of sources) {
		const allowed = EXEMPT_SOURCE[file]?.kinds ?? [];
		const found = scanSource(file, await readFile(file, "utf8")).filter(
			(f) => !allowed.includes(f.kind),
		);
		if (found.length)
			offenders.push(`${file}: ${found.map(describe).join(", ")}`);
	}
	for (const file of styles) {
		if (file in EXEMPT_CSS) continue;
		const found = scanCss(await readFile(file, "utf8"));
		if (found.length) offenders.push(`${file}: ${found.join(", ")}`);
	}
	assert.deepEqual(
		offenders,
		[],
		`這些檔案自己畫了對話框骨架或呼叫 dialog hooks，改用 <Dialog>（${SHARED}）：\n${offenders.join("\n")}`,
	);
});

test("對照組：共用 Dialog 與豁免項真的命中偵測，過期的豁免要拿掉", async () => {
	for (const file of ["src/components/UI/Dialog.tsx"]) {
		const kinds = new Set(
			scanSource(file, await readFile(file, "utf8")).map((f) => f.kind),
		);
		for (const kind of ["role", "aria-modal", "class", "hook"] as const) {
			assert.ok(kinds.has(kind), `${file} 應命中 ${kind}`);
		}
	}
	for (const [file, { kinds }] of Object.entries(EXEMPT_SOURCE)) {
		if (file === SHARED) continue;
		const found = new Set(
			scanSource(file, await readFile(file, "utf8")).map((f) => f.kind),
		);
		for (const kind of kinds) {
			assert.ok(
				found.has(kind),
				`${file} 已不含 ${kind}，從 EXEMPT_SOURCE 拿掉`,
			);
		}
	}
	for (const file of Object.keys(EXEMPT_CSS)) {
		assert.ok(
			scanCss(await readFile(file, "utf8")).length > 0,
			`${file} 已不含骨架，從 EXEMPT_CSS 拿掉`,
		);
	}
});

/** 審查列出的繞過寫法，每一種都必須被抓到。 */
const EVASIONS: Record<
	string,
	{ file: string; code: string; kind: Finding["kind"] }
> = {
	"role 字串": {
		file: "a.tsx",
		code: `const A = () => <div role="dialog" />;`,
		kind: "role",
	},
	"role alertdialog": {
		file: "a.tsx",
		code: `const A = () => <div role="alertdialog" />;`,
		kind: "role",
	},
	"role 運算式": {
		file: "a.tsx",
		code: `const A = () => <div role={"dialog"} />;`,
		kind: "role",
	},
	"role 樣板字串": {
		file: "a.tsx",
		code: "const A = () => <div role={`dialog`} />;",
		kind: "role",
	},
	"role 條件式": {
		file: "a.tsx",
		code: `const A = ({x}:{x:boolean}) => <div role={x ? "dialog" : undefined} />;`,
		kind: "role",
	},
	"role 物件屬性": {
		file: "a.ts",
		code: `export const p = { role: "dialog" };`,
		kind: "role",
	},
	"role setAttribute": {
		file: "a.ts",
		code: `el.setAttribute("role", "alertdialog");`,
		kind: "role",
	},
	"aria-modal 屬性": {
		file: "a.tsx",
		code: `const A = () => <section aria-modal="true" />;`,
		kind: "aria-modal",
	},
	"aria-modal 運算式": {
		file: "a.tsx",
		code: `const A = () => <section aria-modal={true} />;`,
		kind: "aria-modal",
	},
	"aria-modal 物件": {
		file: "a.ts",
		code: `export const p = { "aria-modal": "true" };`,
		kind: "aria-modal",
	},
	"class 字串": {
		file: "a.tsx",
		code: `const A = () => <div className="modal-overlay" />;`,
		kind: "class",
	},
	"class 運算式": {
		file: "a.tsx",
		code: `const A = () => <div className={"modal-content"} />;`,
		kind: "class",
	},
	"class 樣板字串": {
		file: "a.tsx",
		code: `const A = ({x}:{x:string}) => <div className={\`modal-overlay \${x}\`} />;`,
		kind: "class",
	},
	"class clsx": {
		file: "a.tsx",
		code: `const A = () => <div className={clsx("modal-overlay")} />;`,
		kind: "class",
	},
	"class 條件式": {
		file: "a.tsx",
		code: `const A = ({x}:{x:boolean}) => <div className={x ? "adm-operation-dialog-overlay" : ""} />;`,
		kind: "class",
	},
	"class 相接": {
		file: "a.tsx",
		code: `const c = "modal-" + "overlay";`,
		kind: "class",
	},
	"class 帶前綴": {
		file: "a.tsx",
		code: `const c = "cat-modal-overlay";`,
		kind: "class",
	},
	"class modifier": {
		file: "a.tsx",
		code: `const c = "modal-overlay--side";`,
		kind: "class",
	},
	"class 在 .ts": {
		file: "a.ts",
		code: `export const c = ["x", "adm-operation-dialog"];`,
		kind: "class",
	},
	"hook useEscapeKey": {
		file: "a.tsx",
		code: `function A(){ useEscapeKey(() => {}); return null; }`,
		kind: "hook",
	},
	"hook useFocusTrap": {
		file: "a.tsx",
		code: `function A(){ useFocusTrap(ref); return null; }`,
		kind: "hook",
	},
	"hook useModalStack": {
		file: "a.tsx",
		code: `function A(){ useModalStack(ref); return null; }`,
		kind: "hook",
	},
	"hook 命名空間呼叫": {
		file: "a.tsx",
		code: `import * as a11y from "@/lib/a11y"; function A(){ a11y.useFocusTrap(ref); return null; }`,
		kind: "hook",
	},
	"hook import 改名": {
		file: "a.tsx",
		code: `import { useFocusTrap as trap } from "@/lib/a11y"; function A(){ trap(ref); return null; }`,
		kind: "hook",
	},
	"hook ModalParentContext": {
		file: "a.tsx",
		code: `import { ModalParentContext } from "@/lib/a11y";`,
		kind: "hook",
	},
	"class modal-body": {
		file: "a.tsx",
		code: `const A = () => <div className="modal-body" />;`,
		kind: "class",
	},
	"class modal-footer": {
		file: "a.tsx",
		code: `const c = "modal-footer";`,
		kind: "class",
	},
	"class modal-header": {
		file: "a.tsx",
		code: `const c = "modal-header";`,
		kind: "class",
	},
	"class modal-close": {
		file: "a.tsx",
		code: `const c = "modal-close";`,
		kind: "class",
	},
	"class adm-operation-dialog-body": {
		file: "a.tsx",
		code: `const c = "adm-operation-dialog-body";`,
		kind: "class",
	},
	"原生 <dialog>": {
		file: "a.tsx",
		code: `const A = () => <dialog open />;`,
		kind: "native-dialog",
	},
	"原生 showModal()": {
		file: "a.tsx",
		code: `function f(r: { current: HTMLDialogElement }){ r.current.showModal(); }`,
		kind: "native-dialog",
	},
	"role 經 const 間接": {
		file: "a.tsx",
		code: `const R = "dialog"; const A = () => <div role={R} />;`,
		kind: "role",
	},
	"class 經 const 間接": {
		file: "a.tsx",
		code: `const C = "modal-overlay"; const A = () => <div className={C} />;`,
		kind: "class",
	},
	"exempt 屬性（JSX）": {
		file: "a.tsx",
		code: `const A = () => <div data-modal-exempt="" />;`,
		kind: "exempt",
	},
	"exempt 屬性（字串）": {
		file: "a.ts",
		code: `el.setAttribute("data-modal-exempt", "");`,
		kind: "exempt",
	},
	"exempt 屬性（常數）": {
		file: "a.tsx",
		code: `import { MODAL_EXEMPT_ATTRIBUTE } from "@/lib/a11y"; const p = { [MODAL_EXEMPT_ATTRIBUTE]: "" };`,
		kind: "exempt",
	},
};

for (const [name, { file, code, kind }] of Object.entries(EVASIONS)) {
	test(`繞過寫法被抓到：${name}`, () => {
		const found = scanSource(file, code);
		assert.ok(
			found.some((f) => f.kind === kind),
			`沒偵測到 ${kind}：${JSON.stringify(found)}`,
		);
	});
}

test("不誤報：註解、一般文字、無關 class 與 role", () => {
	const code = `
// role="dialog" aria-modal modal-overlay
/* <div className="modal-overlay" /> */
const a = "dialog";
const b = <div className="create-measure-modal-warning hcal-modal-error post-moderate-modal adm-operation-dialog-actions--preflight" role="button" />;
const c = { role: "status" };
const showModal = true;
const d = <div role="status" data-other="" />;
`;
	assert.deepEqual(scanSource("a.tsx", code), []);
});

test("CSS：class 選擇器命中，註解與無關選擇器不算", () => {
	assert.deepEqual(scanCss(".x .modal-overlay { }"), ["modal-overlay"]);
	assert.deepEqual(
		scanCss(".adm-operation-dialog-overlay{}\n.cat-modal-overlay{}"),
		["adm-operation-dialog-overlay", "cat-modal-overlay"],
	);
	assert.deepEqual(scanCss("/* .modal-overlay */ .post-moderate-modal {}"), []);
});

test("CSS：整組 .modal-* 與 .adm-operation-dialog-* 都只能在 Dialog 的樣式表", () => {
	for (const cls of [
		"modal-header",
		"modal-body",
		"modal-footer",
		"modal-close",
		"modal-content--side",
		"modal",
		"adm-operation-dialog",
		"adm-operation-dialog-actions",
		"adm-operation-dialog-body",
		"adm-operation-dialog--audit",
	]) {
		assert.deepEqual(scanCss(`.${cls} {}`), [cls], cls);
		assert.deepEqual(scanCss(`.page .${cls} h3 {}`), [cls], `後代 ${cls}`);
	}
	assert.deepEqual(
		scanCss(
			".modal-header{} .modal-body{} .modal-footer{} .modal-close{} .x .modal-header h3{}",
		),
		["modal-body", "modal-close", "modal-footer", "modal-header"],
	);
});
