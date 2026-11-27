import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

/**
 * 字體大小（小 / 中 / 大）的來源契約閘門。
 *
 * 這個設定的正確性靠**五個檔案同時成立**，而它們之間沒有任何型別或建置關係：
 *
 *   lib/theme/text-scale.ts              ← 唯一真實來源：key / attribute / boot script
 *   components/shared/RootDocument.tsx                       ← boot script 必須在 hydration 前跑
 *   styles/foundation/02-reset-layout.css← attribute → 根 font-size + --hk-text-scale
 *   styles/globals.css                   ← @theme --spacing 反向補償（沒有它＝整頁縮放）
 *   components/shared/*                  ← 入口（導覽列 / 頁尾 / 設定）
 *
 * 少了任何一條，功能不會報錯，只會**安靜地壞掉**：boot script 沒掛就是每次重整
 * 閃一次中字級；`--spacing` 沒補償就是連 44px 觸控目標一起縮到 38.5px。
 * 這支測試只讀原始碼字串，不需要瀏覽器，跑在 `node --test`。
 */

const read = (relative: string) =>
	readFile(new URL(`../${relative}`, import.meta.url), "utf8");

test("text-scale module keeps the storage / attribute contract", async () => {
	const source = await read("lib/theme/text-scale.ts");
	assert.match(source, /^"use client";/);
	assert.match(source, /TEXT_SCALE_STORAGE_KEY = "hk:text-scale"/);
	assert.match(source, /TEXT_SCALE_ATTRIBUTE = "data-text-scale"/);
	assert.match(source, /export type TextScale = "sm" \| "md" \| "lg"/);
	assert.match(source, /sm: "小"/);
	assert.match(source, /md: "中"/);
	assert.match(source, /lg: "大"/);
	assert.match(source, /removeAttribute\(TEXT_SCALE_ATTRIBUTE\)/);
	assert.match(source, /useSyncExternalStore/);
	assert.match(
		source,
		/function getServerTextScale\(\): TextScale \{\s*return "md";/,
	);
	assert.match(source, /const scale = mounted \? stored : "md"/);
});

test("boot script only applies sm / lg and is mounted in the root layout", async () => {
	const source = await read("lib/theme/text-scale.ts");
	const boot = source.slice(source.indexOf("TEXT_SCALE_BOOT_SCRIPT"));
	assert.match(boot, /localStorage\.getItem/);
	assert.match(boot, /v === 'sm' \|\| v === 'lg'/);
	assert.match(boot, /setAttribute/);
	assert.match(boot, /removeAttribute/);

	const layout = await read("components/shared/RootDocument.tsx");
	assert.match(
		layout,
		/import \{ TEXT_SCALE_BOOT_SCRIPT \} from "@\/lib\/theme\/text-scale"/,
	);
	assert.match(layout, /id="hk-text-scale-boot"/);
	assert.match(layout, /__html: TEXT_SCALE_BOOT_SCRIPT/);
	const themeIndex = layout.indexOf('id="hk-theme-boot"');
	const textIndex = layout.indexOf('id="hk-text-scale-boot"');
	const loaderIndex = layout.indexOf('id="hk-initial-loader-watchdog"');
	assert.ok(
		themeIndex > 0 && textIndex > themeIndex && textIndex < loaderIndex,
	);
});

test("root font-size rules exist for sm / lg only", async () => {
	const css = await read("styles/foundation/02-reset-layout.css");
	assert.match(css, /:root \{\s*--hk-text-scale: 1;\s*\}/);
	assert.match(
		css,
		/html\[data-text-scale="sm"\] \{\s*font-size: 87\.5%;\s*--hk-text-scale: 0\.875;\s*\}/,
	);
	assert.match(
		css,
		/html\[data-text-scale="lg"\] \{\s*font-size: 112\.5%;\s*--hk-text-scale: 1\.125;\s*\}/,
	);
	assert.ok(!css.includes('html[data-text-scale="md"]'));
});

test("Tailwind spacing is compensated so only type scales", async () => {
	const css = await read("styles/globals.css");
	assert.match(
		css,
		/--spacing:\s*calc\(0\.25rem \/ var\(--hk-text-scale, 1\)\);/,
	);
	const themeStart = css.indexOf("@theme {");
	assert.ok(themeStart >= 0);
	assert.ok(css.indexOf("--spacing:") > themeStart);
});

test("TextScaleToggle is the ThemeToggle's twin and draws the document icon", async () => {
	const source = await read("components/shared/TextScaleToggle.tsx");
	assert.match(
		source,
		/export default function TextScaleToggle\(\{[\s\S]*?\}: TextScaleToggleProps\)/,
	);
	assert.match(source, /className\?: string;/);
	assert.match(source, /verbose\?: boolean;/);
	assert.match(
		source,
		/import \{ THEME_TOGGLE_CHROME_CLASS \} from "@\/components\/shared\/ThemeToggle"/,
	);
	assert.match(
		source,
		/variant === "footer" \? FOOTER_ICON_LINK : THEME_TOGGLE_CHROME_CLASS/,
	);
	assert.match(source, /<LazyAppIcon[\s\S]*?name="document"/);
	assert.match(source, /size=\{18\}/);
	assert.ok(!/aria-pressed=/.test(source));
	assert.match(source, /aria-label=\{`字級：\$\{label\}，點擊切換`\}/);
	assert.match(source, /role="status"/);
	assert.match(source, /aria-live="polite"/);

	const themeToggle = await read("components/shared/ThemeToggle.tsx");
	assert.match(themeToggle, /export const THEME_TOGGLE_CHROME_CLASS = cn\(/);
	assert.match(themeToggle, /aria-pressed=\{isDark\}/);
});

test("TextScaleControl is a radiogroup with 44px targets", async () => {
	const source = await read("components/shared/TextScaleControl.tsx");
	assert.match(source, /role="radiogroup"/);
	assert.match(source, /aria-label="字體大小"/);
	assert.match(source, /type="radio"/);
	assert.match(source, /checked=\{selected\}/);
	assert.match(source, /onChange=\{\(\) => setScale\(option\)\}/);
	assert.match(source, /className="sr-only"/);
	assert.match(source, /React\.useId\(\)/);
	assert.match(source, /h-11 min-w-11/);
	assert.ok(!source.includes("ArrowRight"));
	assert.ok(!/#[0-9a-fA-F]{3,8}\b/.test(source));
	assert.ok(!/\brgba?\(/.test(source));
});

test("navbar only offers the toggle to signed-in readers", async () => {
	const navbar = await read("components/shared/Navbar.tsx");
	assert.match(
		navbar,
		/import TextScaleToggle from "@\/components\/shared\/TextScaleToggle"/,
	);
	assert.match(
		navbar,
		/status === "signedIn" && \(\s*<TextScaleToggle className="size-11 \[@media\(max-width:840px\)\]:hidden" \/>/,
	);
	const themeIndex = navbar.indexOf('<ThemeToggle className="size-11" />');
	const textIndex = navbar.indexOf("<TextScaleToggle");
	assert.ok(themeIndex > 0 && textIndex > themeIndex);
	assert.ok(navbar.slice(themeIndex, textIndex).indexOf("<CartBubble") === -1);
});

test("footers expose the toggle next to the background switch", async () => {
	const sellerFooter = await read("components/shared/SellerFooter.tsx");
	assert.match(sellerFooter, /<TextScaleToggle/);
	const bgIndex = sellerFooter.indexOf("<SiteBackgroundToggle />");
	const toggleIndex = sellerFooter.indexOf("<TextScaleToggle");
	assert.ok(bgIndex > 0 && toggleIndex > bgIndex);
});

test("buyer footer exposes the toggle (owned by the Footer workstream)", async () => {
	const footer = await read("components/shared/Footer.tsx");
	assert.match(footer, /<TextScaleToggle/);
	const bgIndex = footer.indexOf('<SiteBackgroundToggle variant="icon" />');
	const toggleIndex = footer.indexOf("<TextScaleToggle");
	assert.ok(bgIndex > 0 && toggleIndex > bgIndex);
});

test("settings page carries the 顯示 section with both controls", async () => {
	const page = await read("features/account/screens/settings/SettingsPage.tsx");
	assert.match(page, /<h2>顯示<\/h2>/);
	assert.match(page, /<ThemeToggle verbose \/>/);
	assert.match(page, /<TextScaleControl \/>/);
	assert.ok(!page.includes('"use client"'));
	assert.ok(page.indexOf("<h2>顯示</h2>") < page.indexOf("SECTIONS.map"));
});
