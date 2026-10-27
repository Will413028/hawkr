import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

function blockAt(text: string, markerAt: number, label: string): string {
	assert.notEqual(markerAt, -1, `missing source marker: ${label}`);
	const openAt = text.indexOf("{", markerAt);
	assert.notEqual(openAt, -1, `missing opening brace after: ${label}`);

	let depth = 0;
	for (let index = openAt; index < text.length; index += 1) {
		if (text[index] === "{") depth += 1;
		if (text[index] !== "}") continue;
		depth -= 1;
		if (depth === 0) return text.slice(openAt + 1, index);
	}

	assert.fail(`missing closing brace after: ${label}`);
}

const shell = source("features/seller-center/components/SellerCenterShell.tsx");
const notificationsClient = source(
	"features/account/screens/notifications/NotificationsClient.tsx",
);
const css = source("styles/features/seller-center.css");

test("手機快速入口選取色使用跨亮暗模式的品牌 action tokens", () => {
	const mobileAt = css.indexOf("@media (max-width: 960px)");
	const activeAt = css.indexOf(
		".seller-shell__mobile-quick-tab.is-active {",
		mobileAt,
	);
	const active = blockAt(css, activeAt, "mobile quick-tab active state");

	assert.match(active, /background:\s*var\(--theme-action-primary\)/);
	assert.match(active, /color:\s*var\(--theme-text-inverse\)/);
	assert.match(active, /box-shadow:\s*var\(--shadow-sm\)/);
	assert.doesNotMatch(active, /--theme-primary-subtle/);
});

test("賣家通知分類的選取 tab 不再沿用共用亮綠 pill", () => {
	assert.match(
		notificationsClient,
		/className="seller-notifications__filter-tab"[\s\S]*role="tab"/,
	);
	assert.match(
		css,
		/\.seller-shell\s+\.seller-notifications__filter-tab\[aria-selected="true"\]\s*\{[^}]*background:\s*var\(--theme-action-primary\)[^}]*color:\s*var\(--theme-text-inverse\)/s,
	);
});

test("手機側欄是明確的 elevated panel 而不是讓內容穿透的玻璃", () => {
	const drawerCommentAt = css.indexOf("/* ---- Off-canvas drawer ---- */");
	const drawerAt = css.indexOf(".seller-sidebar {", drawerCommentAt);
	const drawer = blockAt(css, drawerAt, "mobile seller drawer");

	assert.match(drawer, /background:\s*var\(--theme-surface-elevated\)/);
	assert.match(drawer, /border-color:\s*var\(--theme-border-strong\)/);
	assert.match(drawer, /box-shadow:\s*var\(--shadow-2xl\)/);
	assert.match(drawer, /backdrop-filter:\s*none/);

	assert.match(
		css,
		/@media \(max-width: 960px\)\s*\{\s*html\[data-theme="dark"\] body \.seller-shell \.seller-sidebar\s*\{[^}]*background:\s*var\(--theme-surface-elevated\)[^}]*box-shadow:\s*var\(--shadow-2xl\)/s,
	);
});

test("打開手機側欄時使用主題遮罩並保留可辨識的關閉控制", () => {
	const mobileAt = css.indexOf("@media (max-width: 960px)");
	const backdropAt = css.indexOf(".seller-shell__backdrop {", mobileAt);
	const backdrop = blockAt(css, backdropAt, "mobile drawer backdrop");

	assert.match(backdrop, /background:\s*var\(--theme-overlay\)/);
	assert.match(backdrop, /-webkit-backdrop-filter:\s*blur\(3px\)/);
	assert.match(backdrop, /backdrop-filter:\s*blur\(3px\)/);
	assert.doesNotMatch(backdrop, /rgba\(38,\s*150,\s*116/);
	assert.match(
		shell,
		/className="seller-shell__backdrop"[\s\S]*aria-label="關閉側邊選單"/,
	);
});

test("手機側欄關閉鍵與導覽列右側數字使用同一條靠右軸線", () => {
	const drawerCommentAt = css.indexOf("/* ---- Off-canvas drawer ---- */");
	const closeAt = css.indexOf(".seller-sidebar__close {", drawerCommentAt);
	const close = blockAt(css, closeAt, "mobile seller drawer close button");

	assert.match(close, /margin-left:\s*auto/);
	assert.match(close, /margin-right:\s*2px/);
	assert.match(css, /\.seller-sidebar__badge\s*\{[^}]*margin-left:\s*auto/s);
	assert.match(
		css,
		/\.seller-sidebar__close,[\s\S]*?width:\s*44px[^}]*height:\s*44px/s,
	);
});
