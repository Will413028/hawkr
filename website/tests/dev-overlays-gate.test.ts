import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";
import { devOverlaysEnabled } from "../lib/config/env";

/**
 * dev-only 浮動工具的總開關（web#425）。
 *
 * 背景：development 下有兩顆右下角 viewport-fixed 浮動鈕（`RoleSwitcher` 與
 * React Query Devtools），production build 兩顆都不 render。於是本機 `next dev`
 * 量到的 DOM 與 CI（production build）不是同一份 —— 而那兩顆實測整個蓋在手機底部
 * 導覽列第 5 格上，讓本機 mobile E2E 3/3 紅、CI 同一支卻一直是綠的。整輪調查就是
 * 花在分辨「這是產品壞了還是環境多了東西」。
 *
 * 本檔釘住的是**下一次**：
 *   1. 開關的真值表（production 一定關、`off` 一定關）
 *   2. 掛載條件只有一個持有者 —— 沒有人可以再自己寫一次 `NODE_ENV === "development"`
 *      來掛第三顆浮動元件（那正是本題會重演的方式）
 *   3. Playwright 真的有把開關關掉（不然本機又會量到與 CI 不同的 DOM）
 *   4. 手機停靠樣式所依賴的第三方 class 名還存在（devtools 升級改名不會靜靜地壞掉）
 *
 * ⚠️ 本檔是純文字＋純函式層（無瀏覽器）。「兩顆真的沒壓在導覽列上」屬於幾何，
 * 由真瀏覽器那側負責（`tests/mobile-bottom-nav-geometry.spec.ts` 釘導覽列本身的
 * 幾何；dev 浮動工具只在 development 存在，E2E 跑的環境已由第 3 條關掉）。
 */

const repoFile = (path: string) => new URL(`../${path}`, import.meta.url);

test("開關真值表：只有 development 且未被關掉時才開", () => {
	// 預設開：人工開發時這些工具本來就有用。
	assert.equal(devOverlaysEnabled("development", undefined), true);
	assert.equal(devOverlaysEnabled("development", ""), true);

	// 明確關掉（Playwright 注入的值）。
	assert.equal(devOverlaysEnabled("development", "off"), false);

	// production build 一律關 —— 這是「CI 量到的 DOM 裡沒有這兩顆」的來源。
	assert.equal(devOverlaysEnabled("production", undefined), false);
	assert.equal(devOverlaysEnabled("production", "off"), false);
	assert.equal(devOverlaysEnabled("test", undefined), false);
});

/**
 * 掛載條件的唯一持有者。
 *
 * 判準寫成「`NODE_ENV === "development"` 後面接 JSX 的開頭」而不是「出現
 * NODE_ENV」：zustand 的 `devtools({ enabled: process.env.NODE_ENV === ... })`
 * 是 redux devtools 中介層，不渲染任何 DOM，掃進來只會製造假陽性讓閘門被關掉。
 */
const JSX_DEV_GATE = /NODE_ENV\s*===\s*"development"\s*&&\s*\(?\s*</;

async function readSourceFiles(): Promise<Array<[string, string]>> {
	const roots = ["app", "components", "features"];
	const out: Array<[string, string]> = [];
	async function walk(dir: string): Promise<void> {
		let entries: Awaited<ReturnType<typeof readdir>>;
		try {
			entries = await readdir(repoFile(`${dir}/`), { withFileTypes: true });
		} catch {
			return;
		}
		for (const entry of entries) {
			const path = `${dir}/${entry.name}`;
			if (entry.isDirectory()) {
				if (entry.name === "node_modules" || entry.name === "__tests__")
					continue;
				await walk(path);
				continue;
			}
			if (!/\.tsx?$/.test(entry.name)) continue;
			out.push([path, await readFile(repoFile(path), "utf8")]);
		}
	}
	await Promise.all(roots.map(walk));
	return out;
}

test("dev-only 元件的掛載條件一律走 DEV_OVERLAYS_ENABLED，不得各自寫 NODE_ENV", async () => {
	// 自我保護：掃描器若因重構而抓不到東西，本測試會靜靜地永遠綠。先用一段已知
	// 違規的字串證明它答得出來（拼接而成，避免自己成為被掃到的那一個）。
	assert.match(
		`{process.env.${"NODE_ENV"} === "development" && (<Foo />)}`,
		JSX_DEV_GATE,
		"掃描器失效（連已知違規的寫法都掃不到）",
	);

	const files = await readSourceFiles();
	assert.ok(files.length > 50, `只掃到 ${files.length} 個檔 —— 掃描器壞了`);

	const offenders = files
		// 註解裡會提到這個寫法（說明為何不要用），先抹掉再掃。
		.filter(([, source]) =>
			JSX_DEV_GATE.test(
				source
					.replaceAll(/\/\*[\s\S]*?\*\//g, "")
					.replaceAll(/^\s*\/\/.*$/gm, ""),
			),
		)
		.map(([path]) => path);

	assert.deepEqual(
		offenders,
		[],
		"dev-only 浮動元件的掛載條件必須用 lib/config/env.ts 的 DEV_OVERLAYS_ENABLED：" +
			"各自寫 NODE_ENV 的話，本機 E2E 沒辦法一次把它們全關掉，" +
			"就會再一次量到與 CI 不同的 DOM（web#425）",
	);
});

test("兩個已知掛載點確實引用共用開關", async () => {
	for (const path of [
		"components/shared/RootDocument.tsx",
		"components/shared/AppProviders.tsx",
	]) {
		const source = await readFile(repoFile(path), "utf8");
		assert.match(
			source,
			/import \{ DEV_OVERLAYS_ENABLED \} from "@\/lib\/config\/env"/,
			`${path} 應從 lib/config/env 匯入 DEV_OVERLAYS_ENABLED`,
		);
		assert.match(
			source,
			/\{DEV_OVERLAYS_ENABLED &&/,
			`${path} 的 dev-only 元件應以 DEV_OVERLAYS_ENABLED 作為掛載條件`,
		);
	}
});

test("Playwright 的 webServer 會把 dev 浮動工具關掉", async () => {
	const config = await readFile(repoFile("playwright.config.ts"), "utf8");
	assert.match(
		config.replaceAll(/^\s*\/\/.*$/gm, ""),
		/NEXT_PUBLIC_DEV_OVERLAYS:\s*"off"/,
		"playwright.config.ts 的 webServer.env 必須設 NEXT_PUBLIC_DEV_OVERLAYS=off，" +
			"否則本機 next dev 會多渲染兩顆浮動鈕，量到的 DOM 與 CI 不同",
	);
});

/**
 * 手機停靠樣式耦合到第三方 class 名，這裡把那個耦合顯性化。
 *
 * devtools 的 public API（`buttonPosition`）只有四個角落＋relative，而手機寬度下
 * 四個角落都壓到自家固定 chrome，所以只能從外面壓樣式。既然只能這樣，就要讓
 * 「套件升級把 class 改名」變成一個會轉紅的事件，而不是「鈕悄悄跑回右下角」。
 */
test("devtools 停靠樣式依賴的 class 名，在安裝的套件裡仍然存在", async () => {
	const css = await readFile(
		repoFile("styles/features/feed-components.css"),
		"utf8",
	);
	assert.match(
		css,
		/body \.tsqd-open-btn-container \{/,
		"feed-components.css 應以 `body .tsqd-open-btn-container` 覆寫 devtools 開關鈕的位置",
	);

	// 套件產物裡必須找得到同一個 class 字面值。class 是由框架無關的核心套件
	// `@tanstack/query-devtools` 產生的（react 那層只是 wrapper），所以從 wrapper
	// 的解析起點往下解，pnpm 的巢狀結構才找得到。
	const require = createRequire(import.meta.url);
	const core = createRequire(require.resolve("@tanstack/react-query-devtools"));
	const bundle = await readFile(
		core.resolve("@tanstack/query-devtools"),
		"utf8",
	);
	assert.ok(
		bundle.includes("tsqd-open-btn-container"),
		"@tanstack/react-query-devtools 的產物裡找不到 `tsqd-open-btn-container` —— " +
			"套件大概改了 class 名，feed-components.css 的手機停靠覆寫已經失效，" +
			"開關鈕會跑回右下角壓住底部導覽列第 5 格（web#425）",
	);
});

test("手機停靠帶讓開底部導覽列，且引用導覽列變數而非抄數字", async () => {
	const css = await readFile(
		repoFile("styles/features/feed-components.css"),
		"utf8",
	);
	const nav = await readFile(
		repoFile("components/shared/MobileBottomNav.tsx"),
		"utf8",
	);
	const breakpoint = nav.match(/max-\[(\d+)px\]:grid/);
	assert.ok(breakpoint, "MobileBottomNav 應以任意 max-width 斷點宣告 grid");

	// 停靠帶必須關在「導覽列存在」的那個斷點裡，否則桌機也會被收合。
	assert.ok(
		css.includes(`@media not (min-width: ${breakpoint[1]}px) {`),
		`feed-components.css 的 dev 停靠帶應在 \`@media not (min-width: ${breakpoint[1]}px)\` 內` +
			"（與 MobileBottomNav 同真同假）",
	);

	const dock = css.match(/--hk-dev-dock-bottom:\s*([^;]+);/);
	assert.ok(dock, "應宣告 --hk-dev-dock-bottom（dev 停靠帶的下緣）");
	for (const name of [
		"--mobile-bottom-nav-height",
		"--mobile-bottom-nav-bottom",
	]) {
		assert.ok(
			dock[1].includes(`var(${name},`),
			`--hk-dev-dock-bottom 必須以 var(${name}, …) 取得導覽列幾何，不得寫死 px。` +
				`實際為 ${dock[1].replaceAll(/\s+/g, " ").trim()}`,
		);
	}
});
