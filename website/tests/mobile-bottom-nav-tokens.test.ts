import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";

/**
 * 底部導覽列的幾何在本 repo 有三個持有者，而且**沒有任何機制**讓它們自動一致：
 *
 *   components/shared/MobileBottomNav.tsx   ← 真正被渲染的那個（唯一真實來源）
 *   styles/components/navigation-final.css  ← 鏡像成 CSS 變數，供 `.page-content` 淨空使用
 *   components/shared/AppToaster.tsx        ← 再抄一份當 calc() fallback + 停靠斷點
 *
 * 元件已遷 Tailwind（高度由內容決定、`bottom-2.5`、`max-[840px]:grid`），CSS 那側
 * 無法反過來決定它的尺寸，所以三者只能靠「有人記得同步」—— 而 2026-07-30 的實測顯示
 * 這條防線早就破了（宣告 74px、實際 67.5px，靜靜漂了 6.5px）。
 *
 * 本檔是純文字層的閘門（無瀏覽器，跑在 CI 的 `pnpm run test:unit`），釘住兩件事：
 *   1. **斷點**：以元件宣告的任意 max-width 斷點為準，CSS 區塊與 AppToaster 的 matchMedia
 *      條件必須是由它推導出來的同一句（見下方 `expectedMediaCondition`）。
 *   2. **數值**：CSS 宣告值必須是單純 px 常數，且 AppToaster 的 fallback 逐字相同。
 *
 * 另一半鏈條在 `tests/mobile-bottom-nav-geometry.spec.ts`：那支用真實瀏覽器把
 * 「CSS 宣告值」釘到元件的**實際渲染幾何**上。兩支合起來才是完整的防漂移鏈：
 *
 *   實際渲染 ←(geometry.spec)→ CSS 變數 ←(本檔)→ AppToaster fallback
 *
 * ⚠️ 本檔單獨看是「兩份字面值互相比對」，兩邊一起改就會過 —— 它擋的是**單邊**漂移。
 * 「宣告值 ≠ 實際渲染」只有 geometry.spec 抓得到，缺了那支這裡再綠也沒有意義。
 */

const cssPath = new URL(
	"../styles/components/navigation-final.css",
	import.meta.url,
);
const appToasterPath = new URL(
	"../components/shared/AppToaster.tsx",
	import.meta.url,
);
const mobileBottomNavPath = new URL(
	"../components/shared/MobileBottomNav.tsx",
	import.meta.url,
);
const cartSnackbarPath = new URL(
	"../components/shared/CartSnackbar.tsx",
	import.meta.url,
);
const messagesCssPath = new URL(
	"../styles/features/messages.css",
	import.meta.url,
);
const sellerRecruitmentCssPath = new URL(
	"../styles/features/seller-recruitment-scroll-world.css",
	import.meta.url,
);

/**
 * 導覽列淨空的唯一合法寫法：引用那兩個變數，不再抄數字。
 * 任何要「讓開底部導覽列」的規則都必須是這一串（web#386 的第四份常數就是因為
 * 自己寫了 `84px` 才漂掉的）。
 *
 * 比對前會先把空白壓成單一空格：CSS 的 calc() 換行由 biome formatter 決定，
 * 逐字比對會在下次 `pnpm format` 之後假紅。
 */
const NAV_CLEARANCE_EXPR =
	"bottom: calc( var(--mobile-bottom-nav-height) + var(--mobile-bottom-nav-bottom) );";

/** 把 CSS 片段的連續空白壓成單一空格，讓斷言與 formatter 的換行決定脫鉤。 */
function collapseWhitespace(css: string): string {
	return css.replaceAll(/\s+/g, " ");
}

/**
 * 取出 `@media <condition> { ... }` 的區塊內容（以大括號配對掃描，不用會誤配的 regex）。
 * 斷言要「限定在這個區塊裡」才有意義：全檔搜尋會讓宣告搬到別的斷點下也照樣綠。
 */
function mediaBlock(css: string, condition: string): string {
	const header = `@media ${condition} {`;
	const start = css.indexOf(header);
	assert.notEqual(
		start,
		-1,
		`navigation-final.css 應有 \`${header}\` 區塊（底部導覽列的斷點）`,
	);
	let depth = 0;
	for (let i = start + header.length - 1; i < css.length; i += 1) {
		if (css[i] === "{") depth += 1;
		else if (css[i] === "}") {
			depth -= 1;
			if (depth === 0) return css.slice(start + header.length, i);
		}
	}
	throw new Error(`\`${header}\` 區塊沒有配對的結尾大括號`);
}

function declaredVar(block: string, name: string): string {
	const match = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
	assert.ok(match, `底部導覽列的斷點區塊內應宣告 --${name}`);
	return match[1].trim();
}

/**
 * 斷點的唯一真實來源是元件自己。Tailwind v4 為 `max-[840px]:` 產生的 media query
 * 是 `not (min-width: 840px)`（**不含** 840，實測編譯輸出），不是 `(max-width: 840px)`。
 * 由元件的字面值推導出期望條件，改斷點時只要改元件，其餘兩處會被本檔逼著跟上。
 */
async function expectedMediaCondition(): Promise<string> {
	return `not (min-width: ${await navBreakpointPx()}px)`;
}

/** 導覽列顯示斷點的唯一真實來源：元件自己的任意 max-width grid 變體字面值。 */
async function navBreakpointPx(): Promise<string> {
	const nav = await readFile(mobileBottomNavPath, "utf8");
	const match = nav.match(/max-\[(\d+)px\]:grid/);
	assert.ok(
		match,
		"MobileBottomNav 應以任意 max-width 斷點宣告 grid（本檔由它推導其餘幾處的條件）",
	);
	return match[1];
}

test("CSS 變數區塊與 AppToaster 的停靠斷點，都等於元件宣告的任意 max-width 斷點", async () => {
	const [css, toaster, condition] = await Promise.all([
		readFile(cssPath, "utf8"),
		readFile(appToasterPath, "utf8"),
		expectedMediaCondition(),
	]);

	// CSS 側：宣告那兩個變數（與 .page-content 淨空）的區塊必須是同一個條件。
	// 用 `(max-width: 840px)` 會多含正好 840px 這一個像素寬的區間 —— 那裡導覽列
	// 已經 display:none，卻仍會替它保留 94px 空位。
	assert.ok(
		css.includes(`@media ${condition} {`),
		`navigation-final.css 宣告底部導覽列變數的區塊應為 \`@media ${condition}\`（與元件同真同假）`,
	);

	// AppToaster 側：底部停靠與否必須跟導覽列在不在同進退，否則會為一根不存在的
	// 導覽列讓開 90px。
	const query = toaster.match(/const MOBILE_DOCK_QUERY\s*=\s*"([^"]+)"/);
	assert.ok(query, "AppToaster 應定義 MOBILE_DOCK_QUERY");
	assert.equal(
		query[1],
		condition,
		`AppToaster 的 MOBILE_DOCK_QUERY 必須與元件斷點同真同假（期望 \`${condition}\`）`,
	);
});

/**
 * 具名斷點的變體（`md:` / `max-md:` / `lg:` …）——CartSnackbar 不得使用任何一個。
 * 抓「前面是字串開頭或空白」是為了不誤傷 `max-[840px]:` 這種任意值寫法。
 *
 * 刻意**不帶** `g`：帶 `g` 的 regex 常數會被 `assert.match`（走 `test()`）推進
 * `lastIndex`，後面的 `matchAll` 就會從半路開始掃 —— 自我保護那一行反而會製造漏掃。
 * 要掃全部時在使用端複製一份帶 `g` 的。
 */
const NAMED_BREAKPOINT_VARIANT = /(?:^|[\s"'`])((?:max-)?(?:sm|md|lg|xl|2xl)):/;

/**
 * CartSnackbar 的手機／桌機分界必須是**導覽列的斷點**，不是 Tailwind 的 `md`。
 *
 * 這是 web#417 留下的落差（當時只加註解沒修）：位置樣式走 `md:`（768px），而導覽列
 * 顯示到 839px 為止（`MobileBottomNav.tsx` 的 `max-[840px]:grid`）——768–839px，
 * 也就是平板直立的常見寬度，snackbar 拿桌機的 24px 位移，正好被導覽列蓋住。
 *
 * 上面那支「viewport-fixed 不得寫死 px」的閘門對這個完全無感：CartSnackbar 的
 * `bottom-[ calc(var(…)+var(…)) ]` 本來就合規，錯的是**它被關在哪個斷點之下**。
 * 數字對、斷點錯，一樣是壞的。
 *
 * 修法是兩側都用導覽列那個數字：`max-[840px]:` 編成 `not (min-width: 840px)`、
 * `min-[840px]:` 編成 `(min-width: 840px)`，互為補集 —— 正好 840px 那一個像素寬的
 * 區間不重疊也不落空（這正是本檔開頭那條「不含 840」的陷阱的另一面）。
 */
test("CartSnackbar 的響應式分界等於導覽列斷點，不得使用具名斷點", async () => {
	const [raw, px] = await Promise.all([
		readFile(cartSnackbarPath, "utf8"),
		navBreakpointPx(),
	]);
	// 註解裡會提到 `md:`（說明為何移除），先抹掉再掃，否則會被自己的說明文字打敗。
	const source = stripCommentsAndVars(raw);

	// 自我保護：掃描器若因重構而抓不到東西，本測試會靜靜地永遠綠 —— 先用一段
	// 已知違規的字串證明它答得出來，再拿去掃真檔案。
	//
	// ⚠️ 這個 fixture 必須拼接而成：Tailwind v4 的 candidate 掃描**包含 tests/**，
	// 而且是純文字掃描（**連註解都算**）—— 只要這個檔案裡任何一處出現完整的
	// 具名斷點 class 字面值，它就會被真的編進 production CSS bundle 變成死樣式。
	// 實測過：拼接前 bundle 裡多了一條無人使用的桌機 bottom 規則。
	// 所以連這段說明也不能把那個 class 寫全。
	assert.match(
		`"left-4 ${"md"}:bottom-6 group"`,
		NAMED_BREAKPOINT_VARIANT,
		"具名斷點掃描器失效（連已知違規的字串都掃不到）",
	);

	const named = [
		...new Set(
			[...source.matchAll(new RegExp(NAMED_BREAKPOINT_VARIANT, "g"))].map(
				(m) => m[1],
			),
		),
	];
	assert.deepEqual(
		named,
		[],
		`CartSnackbar 不得使用具名斷點變體（實際出現 ${named.join(", ")}）：` +
			`它的手機／桌機分界是「導覽列在不在」，只有 ${px}px 這一個數字，` +
			"用 md(768px) 會讓 768–839px 拿到桌機位移、被導覽列蓋住",
	);

	// 桌機位移必須被關在導覽列斷點**之上**：導覽列還在的時候絕不能套用。
	assert.match(
		source,
		new RegExp(`min-\\[${px}px\\]:bottom-6`),
		`CartSnackbar 的桌機底部位移應為 \`min-[${px}px]:bottom-6\`（導覽列消失之後才套用）`,
	);

	// 手機側的排版 utility 也必須用同一個斷點，否則 768–839px 會出現
	// 「容器是手機的整寬、內容卻是桌機的置中」這種各走各的錯位。
	//
	// 這裡刻意**不**釘某一支 utility 名稱：原本釘的是 `truncate`，而它後來因為
	// 「訊息過長在任何寬度都會溢出」被改成無條件套用（不再帶斷點前綴），閘門就跟著
	// 假紅了。真正要守的不變量是「這個檔案裡出現的每一個任意值斷點都必須是導覽列
	// 那個數字」——只要還有任何一支帶前綴的 utility，它就跑得動；一支都不剩時
	// 底下的桌機 bottom-6 斷點斷言仍會逼出至少一個。
	const breakpoints = [
		...new Set(
			[...source.matchAll(/(?:max|min)-\[(\d+)px\]:/g)].map((m) => m[1]),
		),
	];
	assert.deepEqual(
		breakpoints,
		[px],
		`CartSnackbar 的所有斷點都應是 ${px}px（實際出現 ${breakpoints.join(", ")}）：` +
			"位置樣式與內容 utility 分家會讓 768–839px 出現「容器是手機的整寬、" +
			"內容卻是桌機的置中」這種各走各的錯位",
	);
});

test("AppToaster 的 mobile dock fallback 與 navigation-final.css 宣告值一致", async () => {
	const [css, toaster, condition] = await Promise.all([
		readFile(cssPath, "utf8"),
		readFile(appToasterPath, "utf8"),
		expectedMediaCondition(),
	]);
	const block = mediaBlock(css, condition);

	const height = declaredVar(block, "mobile-bottom-nav-height");
	const bottom = declaredVar(block, "mobile-bottom-nav-bottom");

	// 宣告值必須是單純的 px 常數：一旦有人改成 calc()/env()/max()，本測試的字面比對就不再
	// 足以保證 fallback 正確，會在這裡先失敗，逼人重新設計同步方式而不是默默漂移。
	assert.match(
		height,
		/^\d+(\.\d+)?px$/,
		`--mobile-bottom-nav-height 應為單純 px 常數，實際為 ${height}`,
	);
	assert.match(
		bottom,
		/^\d+(\.\d+)?px$/,
		`--mobile-bottom-nav-bottom 應為單純 px 常數，實際為 ${bottom}`,
	);

	// 特別點名 env()：這兩個值曾經是 `max(10px, env(safe-area-inset-bottom))`，而
	// `MobileBottomNav` 用的是 `bottom-2.5`，**從未套用 safe-area** —— env() 會讓變數
	// 在有安全區的機種上比實際多讓 24px 以上。這種回歸在瀏覽器模擬環境永遠量不到
	// （env(safe-area-inset-bottom) 恆為 0），geometry.spec 抓不到，只能在這裡擋。
	for (const [name, value] of [
		["--mobile-bottom-nav-height", height],
		["--mobile-bottom-nav-bottom", bottom],
	] as const) {
		assert.ok(
			!value.includes("env("),
			`${name} 不得使用 env()：元件並未套用 safe-area（bottom-2.5 是固定值），` +
				`寫 env() 會讓淨空在真機上多讓 24px 以上，而模擬器永遠量到 0 測不出來。實際為 ${value}`,
		);
	}

	const offset = toaster.match(
		/const MOBILE_DOCK_BOTTOM_OFFSET\s*=\s*\n?\s*"([^"]+)"/,
	);
	assert.ok(offset, "AppToaster 應定義 MOBILE_DOCK_BOTTOM_OFFSET");

	assert.equal(
		offset[1],
		`calc(var(--mobile-bottom-nav-height, ${height}) + var(--mobile-bottom-nav-bottom, ${bottom}) + 12px)`,
		"AppToaster 的 fallback 值必須與 navigation-final.css 的宣告逐字相同",
	);
});

test("page-content 的淨空與變數宣告在同一個斷點區塊內", async () => {
	const css = await readFile(cssPath, "utf8");
	const block = mediaBlock(css, await expectedMediaCondition());

	// 淨空是靠 `.page-content` 的 padding-bottom 撐出來的。若有人把它改成寫死數字，
	// 變數就失去唯一宣告處的地位；若有人把它留在 `(max-width: 840px)` 那個區塊，
	// 正好 840px 就會替一根 display:none 的導覽列保留空位 —— 兩種都在這裡擋下。
	assert.match(
		block,
		/body \.page-content \{\s*padding-bottom: calc\(\s*var\(--mobile-bottom-nav-height\) \+\s*var\(--mobile-bottom-nav-bottom\) \+\s*16px\s*\);\s*\}/,
		"body .page-content 的 padding-bottom 應為 calc(高度 + 離底距離 + 16px)，且與變數宣告同區塊",
	);
});

test("聊天附件選單的底部位移引用導覽列變數，且不得再套 safe-area", async () => {
	const css = await readFile(messagesCssPath, "utf8");
	// 註解裡本來就會提到 `env(safe-area-*)`（說明為何移除），先抹掉再比對，
	// 否則②會被自己的說明文字打敗。
	const block = mediaBlock(css, "(max-width: 540px)").replaceAll(
		/\/\*[\s\S]*?\*\//g,
		"",
	);

	// ① 位移必須是「導覽列高度 + 離底距離」的變數運算。
	// 這裡曾是 `calc(84px + env(safe-area-inset-bottom))`——84px = 遷 Tailwind 前的
	// 74px 高 + 10px 離底距離，是本 repo 第四份互相漂移的底部導覽列常數（web#386）。
	// 引用變數之後，「位移 == 實際幾何」由 mobile-bottom-nav-geometry.spec.ts 那一端
	// 保證（它把變數釘在元件真實渲染高度上），這裡只需擋住「有人又抄一份數字」。
	assert.ok(
		collapseWhitespace(block).includes(NAV_CLEARANCE_EXPR),
		"messages.css 的 @media (max-width: 540px) .attachment-picker 應以 " +
			"`bottom: calc(var(--mobile-bottom-nav-height) + var(--mobile-bottom-nav-bottom))` " +
			"讓開底部導覽列，不得寫死 px",
	);

	// ② 同一個區塊內不得出現 safe-area：MobileBottomNav 用 `bottom-2.5`，從未套用
	// safe-area（實測 env(safe-area-inset-bottom) 探針 = 0），加上去只會讓真機多讓
	// 24px 以上，而模擬器永遠量到 0 —— 這種回歸只有純文字比對擋得住。
	assert.ok(
		!block.includes("env(safe-area"),
		".attachment-picker 的 ≤540px 區塊不得使用 env(safe-area-*)：元件並未套用 safe-area，" +
			"多讓出來的 24px 在瀏覽器模擬環境量不到，只有真機才看得見",
	);

	// ③ 高度上限扣的也必須是同一組變數（原本一樣寫死 84px，會與 ① 各自漂）。
	assert.match(
		block,
		/max-height: calc\([\s\S]*?var\(--mobile-bottom-nav-height\)[\s\S]*?var\(--mobile-bottom-nav-bottom\)[\s\S]*?\);/,
		".attachment-picker 的 max-height 也應扣掉同一組導覽列變數，不得另寫一份數字",
	);
});

test("seller 招募頁的底部淨空引用導覽列變數，且不得再套 safe-area", async () => {
	// 註解裡本來就會提到 `env(safe-area-*)`（說明為何不能加），先抹掉再比對。
	const css = (await readFile(sellerRecruitmentCssPath, "utf8")).replaceAll(
		/\/\*[\s\S]*?\*\//g,
		"",
	);
	const match = css.match(/--srw-mobile-safe-bottom:\s*([^;]+);/);
	assert.ok(
		match,
		"seller-recruitment-scroll-world.css 應宣告 --srw-mobile-safe-bottom（手機底部淨空）",
	);
	const value = collapseWhitespace(match[1]).trim();

	// 這裡曾是 `calc(高度 + 離底距離 + env(safe-area-inset-bottom))`（web#409，
	// 與 messages.css 的第四份同一個 root cause）：MobileBottomNav 用 `bottom-2.5`，
	// 從未套用 safe-area，補一個不存在的位移只會讓真機多讓出去；而瀏覽器模擬環境
	// 永遠量到 0，只有純文字比對擋得住。
	assert.ok(
		!value.includes("env("),
		"--srw-mobile-safe-bottom 不得使用 env()：元件並未套用 safe-area（bottom-2.5 是固定值），" +
			`多讓出來的間距在瀏覽器模擬環境量不到。實際為 ${value}`,
	);

	// 淨空必須引用導覽列那兩個變數（fallback 值本身由「全樹 fallback」那支釘住），
	// 不得再抄一份數字。
	for (const name of [
		"--mobile-bottom-nav-height",
		"--mobile-bottom-nav-bottom",
	]) {
		assert.ok(
			value.includes(`var(${name},`),
			`--srw-mobile-safe-bottom 應以 var(${name}, …) 取得導覽列幾何，不得寫死 px。實際為 ${value}`,
		);
	}
});

/** 掃描來源樹（不含測試、建置產物與 node_modules），回傳所有樣式與元件檔內容。 */
async function readSourceFiles(): Promise<Array<[string, string]>> {
	const roots = ["styles", "components", "features", "app", "lib"];
	const out: Array<[string, string]> = [];
	async function walk(dir: string): Promise<void> {
		let entries: Awaited<ReturnType<typeof readdir>>;
		try {
			entries = await readdir(new URL(`../${dir}/`, import.meta.url), {
				withFileTypes: true,
			});
		} catch {
			return; // 目錄不存在就跳過，不讓本測試綁死目錄結構
		}
		for (const entry of entries) {
			const path = `${dir}/${entry.name}`;
			if (entry.isDirectory()) {
				if (entry.name === "node_modules" || entry.name === "__tests__")
					continue;
				await walk(path);
			} else if (/\.(css|ts|tsx)$/.test(entry.name)) {
				out.push([
					path,
					await readFile(new URL(`../${path}`, import.meta.url), "utf8"),
				]);
			}
		}
	}
	await Promise.all(roots.map(walk));
	return out;
}

/**
 * 全樹閘門：`var(--mobile-bottom-nav-*, <fallback>)` 的 fallback 逐字等於宣告值。
 *
 * 上面那幾支各自釘住一個**已知**持有者（AppToaster、messages.css）。它們擋不住的是
 * web#386 真正的失效模式：有人在第五個檔案裡再抄一份數字，而沒有任何測試知道它存在。
 * `--mobile-bottom-nav-height` 只宣告在 `@media not (min-width: 840px)` 之內，所以每個
 * fallback 在 ≥840px 都**真的會生效**，不是裝飾——寫錯就是真的錯。
 *
 * 實際命中（2026-08-02）：`styles/features/seller-recruitment-scroll-world.css` 的
 * `--srw-mobile-safe-bottom` 就是第五份，值目前正確、但過去沒有任何東西釘著它。
 */
test("全樹的 --mobile-bottom-nav-* fallback 都等於唯一宣告值", async () => {
	const [css, condition] = await Promise.all([
		readFile(cssPath, "utf8"),
		expectedMediaCondition(),
	]);
	const block = mediaBlock(css, condition);
	const declared: Record<string, string> = {
		"--mobile-bottom-nav-height": declaredVar(
			block,
			"mobile-bottom-nav-height",
		),
		"--mobile-bottom-nav-bottom": declaredVar(
			block,
			"mobile-bottom-nav-bottom",
		),
	};

	const offenders: string[] = [];
	let seen = 0;
	for (const [path, source] of await readSourceFiles()) {
		// 註解裡引用這些變數是說明用途（本 repo 多處如此），不是生效的 fallback。
		const code = source.replaceAll(/\/\*[\s\S]*?\*\//g, "");
		for (const m of code.matchAll(
			/var\(\s*(--mobile-bottom-nav-(?:height|bottom))\s*,\s*([^)]+?)\s*\)/g,
		)) {
			seen += 1;
			const [, name, fallback] = m;
			if (fallback !== declared[name]) {
				offenders.push(
					`${path}: var(${name}, ${fallback}) ≠ 宣告值 ${declared[name]}`,
				);
			}
		}
	}

	// 自我保護：比對邏輯若因重構而掃不到任何東西，這條測試會靜靜地永遠綠。
	assert.ok(
		seen > 0,
		"掃不到任何 --mobile-bottom-nav-* fallback —— 掃描器壞了，不是 repo 乾淨了",
	);
	assert.deepEqual(
		offenders,
		[],
		"fallback 必須與 navigation-final.css 的宣告值逐字相同，否則就是新的一份漂移常數",
	);
});

/**
 * 全樹閘門②：viewport-fixed 元素的底部位移，不得寫死導覽列高度量級的字面值。
 *
 * 上面那支（fallback 比對）只看得見**已經引用**變數的地方——它的視野裡沒有
 * 「根本沒引用那個變數、自己抄一個數字」的檔案，而那正是漂移的來源。web#417 就是
 * 這樣抓到第六、七份的：`messages.css` 的 `.attachment-picker` 寫 80px、
 * `CartSnackbar.tsx` 也寫 80px，導覽列實際只佔 78px，三個數字（80/72/80）互不相同，
 * 而 #386 的閘門對它們完全無感。
 *
 * **判準不是數字本身，是「誰會跟導覽列相撞」**：只有 `position: fixed` 的元素活在
 * viewport 座標系、跟同樣 fixed 的導覽列共用底部空間。`absolute` 的元素貼在自己的
 * 容器裡（例：`.post-viewer__notice` 的 72px 其實是它上方動作列的高度，與導覽列無關），
 * 一般的 `padding-bottom` 也只是頁面留白——把它們一起掃進來會產出大量假陽性，
 * 閘門就會被當噪音關掉。實測：只加「數字落在導覽列量級」這一條會命中 20 處、
 * 其中 18 處無辜；加上「fixed + 底部位移」後正好命中 2 處真兇、零假陽性。
 *
 * 合法寫法只有一種：`calc(var(--mobile-bottom-nav-height, …) + var(--mobile-bottom-nav-bottom, …))`
 * （fallback 的正確性由上一支釘住），所以比對前先把 `var(…)` 整段抹掉——
 * fallback 裡的 68px 是宣告值的複本，不是抄來的第二份常數。
 */
const NAV_SCALE_PX = /\b(6[0-9]|7[0-9]|8[0-9]|9[0-9])(\.\d+)?px\b/;

/** 去掉註解與 `var(…)`（含 fallback），只留下「真的自己寫的數字」。 */
function stripCommentsAndVars(source: string): string {
	return source
		.replaceAll(/\/\*[\s\S]*?\*\//g, "")
		.replaceAll(/^\s*\/\/.*$/gm, "")
		.replaceAll(/var\([^()]*\)/g, "");
}

/**
 * 把相鄰字面值的字串接合（`"a " + "b"` → `"a b"`）後再掃。
 *
 * 長的 className 在本 repo 慣例上會拆成多行相接（見 CartSnackbar），而
 * `fixed` 與 `bottom-[ … ]` 很容易落在不同段。逐個字面值掃的話，這個純粹為了
 * 換行而做的拆分就會變成閘門的旁路——實測就是這樣漏掉 CartSnackbar 的：
 * 只要有人把字串折行，寫死的數字就隱形了。接合後掃的是執行期真正的那一串。
 */
function joinAdjacentStringLiterals(code: string): string {
	let prev: string;
	let out = code;
	do {
		prev = out;
		out = out.replaceAll(/(["'`])\s*\+\s*\1/g, "");
	} while (out !== prev);
	return out;
}

test("viewport-fixed 元素的底部位移不得寫死導覽列量級的 px", async () => {
	const offenders: string[] = [];
	let fixedBlocksSeen = 0;

	for (const [path, source] of await readSourceFiles()) {
		const code = stripCommentsAndVars(source);

		if (path.endsWith(".css")) {
			// 逐個「最內層」規則區塊掃（`[^{}]*` 保證不跨區塊，@media 外殼自然被略過）。
			for (const match of code.matchAll(/\{[^{}]*\}/g)) {
				const body = match[0];
				if (!/position\s*:\s*fixed/.test(body)) continue;
				fixedBlocksSeen += 1;
				const decl = body.match(
					/(?:^|[\s;{])(bottom|inset-block-end)\s*:\s*([^;}]+)/,
				);
				if (decl && NAV_SCALE_PX.test(decl[2])) {
					const line = code.slice(0, match.index).split("\n").length;
					offenders.push(
						`${path}:~${line}: ${decl[1]}: ${decl[2].trim().replace(/\s+/g, " ")}`,
					);
				}
			}
		} else {
			// Tailwind：同一個 class 字串裡同時有 `fixed` 與 `bottom-[ … ]`。
			// 接合相鄰字面值會抹掉換行，行號因此是近似值（標 `~`）——offender 訊息
			// 已含檔案與那一段 utility，足以定位。
			const joined = joinAdjacentStringLiterals(code);
			for (const strMatch of joined.matchAll(/"([^"]*)"|`([^`]*)`/g)) {
				const cls = strMatch[1] ?? strMatch[2] ?? "";
				if (!/(?:^|\s)fixed(?:\s|$)/.test(cls)) continue;
				fixedBlocksSeen += 1;
				for (const util of cls.matchAll(
					/(?:^|\s)((?:[\w-]+:)*bottom-\[[^\]]*\])/g,
				)) {
					if (NAV_SCALE_PX.test(util[1])) {
						const line = joined.slice(0, strMatch.index).split("\n").length;
						offenders.push(`${path}:~${line}: ${util[1]}`);
					}
				}
			}
		}
	}

	// 自我保護：掃描器若因重構而看不到任何 fixed 元素，這條測試會靜靜地永遠綠——
	// 那比沒有閘門更危險（大家以為有人在看）。
	assert.ok(
		fixedBlocksSeen > 10,
		`只掃到 ${fixedBlocksSeen} 個 fixed 元素 —— 掃描器壞了，不是 repo 乾淨了`,
	);

	assert.deepEqual(
		offenders,
		[],
		"viewport-fixed 元素要讓開底部導覽列時，必須寫 " +
			"`calc(var(--mobile-bottom-nav-height, …) + var(--mobile-bottom-nav-bottom, …))`，" +
			"不得自己抄一個 px 數字（那就是下一份漂移常數）",
	);
});

test("已遷 Tailwind 的 .mobile-bottom-nav class 不得再出現於樣式表", async () => {
	const css = await readFile(cssPath, "utf8");

	// `MobileBottomNav.tsx` 已無此 class（Tailwind utilities），任何針對它的規則都是
	// 死規則：不生效、卻讓讀者誤以為有效（web#378 的 z-index 意圖就是這樣失效的）。
	// 只允許出現在註解裡（說明為何刪除）。
	const withoutComments = css.replaceAll(/\/\*[\s\S]*?\*\//g, "");
	assert.doesNotMatch(
		withoutComments,
		/\.mobile-bottom-nav/,
		"navigation-final.css 不應再有 .mobile-bottom-nav 選擇器（該 class 已不存在於 DOM）",
	);
});

test("底部導覽列的量測不得採信沒有版面的讀數", async () => {
	const source = await readFile(
		new URL("../components/shared/MobileBottomNav.tsx", import.meta.url),
		"utf8",
	);

	// nav 在 ≥840px 是 `hidden`，`display: none` 子樹裡每個 tab 的 offsetWidth
	// 都是 0。舊版把這組全零寫進 slots，而 `measured` 只比對「數量」，5 個零照樣
	// 算量到 → 指示塊以 opacity:1 + width:0 渲染：DOM 裡在、畫面上不見，而且因為
	// 尺寸沒再變過，ResizeObserver 也不會重量。
	assert.match(
		source,
		/if \(el\.offsetWidth === 0 \|\| el\.offsetHeight === 0\) return;/,
		"measure() 必須拒絕零尺寸讀數，而不是把它當成有效量測寫進 state",
	);

	// 拒絕的位置必須在 setSlots 之前，否則零值還是會進 state。
	const guard = source.indexOf("el.offsetHeight === 0");
	const commit = source.indexOf("setSlots(next)");
	assert.ok(
		guard !== -1 && commit !== -1 && guard < commit,
		"零尺寸守衛必須擋在 setSlots 之前",
	);
});
