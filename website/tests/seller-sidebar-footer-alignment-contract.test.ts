import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

const shell = source("features/seller-center/components/SellerCenterShell.tsx");
const css = source("styles/features/seller-center.css");

test("側欄登出列與相鄰導覽列共用同一組外層幾何", () => {
	assert.match(
		shell,
		/className="seller-sidebar__item seller-sidebar__item--signout h-auto justify-start"/,
	);
	assert.match(
		css,
		/\.seller-sidebar__item\s*\{[^}]*min-height:\s*44px[^}]*padding:\s*9px var\(--hkui-space-3\)[^}]*width:\s*100%/s,
	);
});

test("登出按鈕的 loading 預留層不會再把 idle 內容置中推移", () => {
	assert.match(
		css,
		/\.seller-sidebar__item--signout:where\(\[data-hkui-button\]\)\s*>\s*:where\(span\)\s*\{[^}]*width:\s*100%[^}]*justify-items:\s*stretch/s,
	);
	assert.match(
		css,
		/\.seller-sidebar__item--signout:where\(\[data-hkui-button\]\)\s*>\s*:where\(span\)\s*>\s*:where\(span\)\s*\{[^}]*width:\s*100%[^}]*justify-content:\s*flex-start/s,
	);
	assert.match(
		css,
		/\.seller-sidebar__item--signout:where\(\[data-hkui-button\]\)\s*>\s*:where\(span\)\s*>\s*:where\(span:first-child\),[^{]*\{[^}]*gap:\s*var\(--hkui-space-3\)/s,
	);
	assert.match(
		css,
		/:where\(\[role="status"\]\s*>\s*span:first-child\)\s*\{[^}]*width:\s*20px[^}]*height:\s*20px/s,
	);
});
