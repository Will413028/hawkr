import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const guidePath = new URL(
	"../features/auth/components/login/WebViewBrowserGuide.tsx",
	import.meta.url,
);
const oauthButtonPath = new URL(
	"../features/auth/components/login/OAuthLoginButton.tsx",
	import.meta.url,
);
const loginPagePath = new URL(
	"../features/auth/screens/login/LoginPage.tsx",
	import.meta.url,
);

test("WebView guide ships inline and full-screen spotlight designs", async () => {
	const source = await readFile(guidePath, "utf8");

	assert.match(source, /WebViewBrowserGuideVariant = "inline" \| "sheet"/);
	assert.match(source, /data-webview-guide="inline"/);
	assert.match(source, /data-webview-guide="sheet"/);
	assert.match(source, /data-webview-guide-mode="spotlight"/);
	assert.match(source, /data-webview-guide-spotlight="external-browser"/);
	assert.match(source, /data-webview-native-menu-hint="true"/);
	assert.match(source, /data-webview-guide-corner=\{guide\.corner\}/);
	assert.match(source, /data-webview-guide-arrow=\{guide\.arrow\}/);
	assert.match(source, /data-webview-guide-positioning="fixed"/);
	assert.match(source, /data-webview-guide-layout="true"/);
	assert.match(source, /aria-label="外部瀏覽器登入指引"/);
	assert.match(source, /aria-label="外部瀏覽器登入聚光指引"/);
	assert.match(source, /role="status"/);
	assert.match(source, /aria-live="polite"/);
	assert.match(source, /使用 Chrome 開啟/);
	assert.match(source, /buildIosChromeExternalBrowserUrl/);
	assert.match(source, /data-webview-external-action="true"/);
	assert.match(source, /variant !== "sheet"/);
	assert.match(source, /trapGuideFocus/);
	assert.match(source, /<Dialog/);
	assert.match(source, /open=\{open\}/);
	assert.doesNotMatch(source, /<Sheet/);
	assert.doesNotMatch(source, /data-webview-guide-trigger="sheet"/);
	assert.doesNotMatch(source, /navigator\.share/);
});

test("guide draws its arrows and entry mark with the shared static icon registry", async () => {
	const source = await readFile(guidePath, "utf8");

	assert.match(
		source,
		/import AppIcon, \{\s*type AppIconName,?\s*\} from "@\/components\/shared\/AppIcon"/,
	);
	assert.match(source, /GUIDE_ARROW_GLYPHS/);
	for (const arrow of ["left-up", "right-up", "left-down", "right-down"]) {
		assert.match(source, new RegExp(`"${arrow}": \\{ name: "arrow-(up|down)"`));
	}
	assert.match(source, /name="discovery"/);
	assert.match(source, /getInAppBrowserNativeMenuGuide/);
	assert.match(source, /isTopMenuGuide/);
	// 底部選單指引時，聚光層要留出下緣淨空讓提示卡不被自己的內容擠掉。
	// 這裡曾是 `max(6.5rem, 安全區 + 5.5rem)`：安全區那項在本站永遠是 0
	// （viewport 無 `viewport-fit: cover`），於 web#417 移除後 max() 只剩
	// 6.5rem 這一項，直接寫成常數。
	//
	// ⚠️ 上面刻意不寫出完整的 Tailwind class 字面值：Tailwind v4 的候選字掃描是
	// 純正則、不跳過註解也會掃 tests/，寫出來就會把那個已經沒人用的 utility
	// 重新編進 CSS（web#417 實測，死碼從註解裡復活）。
	assert.match(source, /pb-\[6\.5rem\]/);
	assert.doesNotMatch(source, /safe-area-inset/);
	assert.doesNotMatch(source, /點右上角/);
});

test("login sends Google to the external-browser guide in embedded browsers while keeping the other providers", async () => {
	const [page, button] = await Promise.all([
		readFile(loginPagePath, "utf8"),
		readFile(oauthButtonPath, "utf8"),
	]);

	assert.match(
		page,
		/browserGuideOpen, setBrowserGuideOpen.*useState\(false\)/s,
	);
	assert.match(page, /requiresBrowserGuide = inAppBrowser\.isInApp/);
	assert.match(
		page,
		/showBrowserGuide = requiresBrowserGuide && browserGuideOpen/,
	);
	assert.match(
		page,
		/requestedGuideVariant === "inline" \? "inline" : "sheet"/,
	);
	assert.match(
		page,
		/provider="google"[\s\S]*?guideOpen=\{requiresBrowserGuide \? showBrowserGuide : undefined\}/,
	);
	assert.match(
		page,
		/requiresBrowserGuide\s*\?\s*openBrowserGuide\s*:\s*\(\) => signInWith\("google"\)/,
	);
	assert.match(page, /open=\{showBrowserGuide\}/);
	assert.match(page, /<WebViewBrowserGuide/);
	assert.match(page, /provider="apple"/);
	assert.match(page, /provider="line"/);
	assert.ok(
		page.indexOf('{appleLoginAction("page")}') <
			page.indexOf("{showBrowserGuide && ("),
	);
	assert.match(page, /appleAction=/);
	assert.match(
		button,
		/data-webview-guide-trigger=\{guided \? provider : undefined\}/,
	);
	assert.match(button, /aria-haspopup=\{guided \? "dialog" : undefined\}/);
});
