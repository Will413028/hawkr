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
	assert.match(page, /provider="line"/);
	assert.ok(
		page.indexOf('{lineLoginAction("page")}') <
			page.indexOf("{showBrowserGuide && ("),
	);
	assert.match(page, /alternativeAction=/);
	assert.match(
		button,
		/data-webview-guide-trigger=\{guided \? provider : undefined\}/,
	);
	assert.match(button, /aria-haspopup=\{guided \? "dialog" : undefined\}/);
});
