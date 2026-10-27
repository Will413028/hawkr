import assert from "node:assert/strict";
import test from "node:test";
import {
	buildAndroidExternalBrowserIntent,
	buildIosChromeExternalBrowserUrl,
	buildLoginBrowserHandoffUrl,
	detectInAppBrowser,
	getInAppBrowserNativeMenuGuide,
} from "../lib/browser/in-app-browser";

const IOS_BASE =
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148";

test("detects Threads, Instagram, and LINE embedded browsers", () => {
	const threads = detectInAppBrowser(`${IOS_BASE} Barcelona 388.0.0.0.0`);
	const instagram = detectInAppBrowser(`${IOS_BASE} Instagram 388.0.0.0.0`);
	const line = detectInAppBrowser(
		"Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/131.0 Mobile Safari/537.36 Line/15.10.1",
	);

	assert.deepEqual(
		{
			isInApp: threads.isInApp,
			appKey: threads.appKey,
			appName: threads.appName,
			platform: threads.platform,
		},
		{
			isInApp: true,
			appKey: "threads",
			appName: "Threads",
			platform: "ios",
		},
	);
	assert.equal(instagram.appKey, "instagram");
	assert.equal(instagram.appName, "Instagram");
	assert.equal(line.appKey, "line");
	assert.equal(line.appName, "LINE");
	assert.equal(line.platform, "android");
});

test("does not classify ordinary Safari as an in-app browser", () => {
	const safari = detectInAppBrowser(
		`${IOS_BASE} Version/18.5 Mobile/15E148 Safari/604.1`,
	);
	assert.equal(safari.isInApp, false);
	assert.equal(safari.appKey, null);
	assert.equal(safari.platform, "ios");
});

test("maps each supported host browser to its current native menu corner", () => {
	const cases = [
		{
			appKey: "threads",
			platform: "ios",
			userAgent: `${IOS_BASE} Barcelona 388.0.0.0.0`,
			corner: "top-right",
			arrow: "right-up",
		},
		{
			appKey: "threads",
			platform: "android",
			userAgent:
				"Mozilla/5.0 (Linux; Android 14; SM-S921B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/136.0.7103.125 Mobile Safari/537.36 Barcelona 382.0.0.51.85 Android",
			corner: "top-right",
			arrow: "right-up",
		},
		{
			appKey: "instagram",
			platform: "ios",
			userAgent: `${IOS_BASE} Instagram 388.0.0.0.0`,
			corner: "top-right",
			arrow: "right-up",
		},
		{
			appKey: "instagram",
			platform: "android",
			userAgent:
				"Mozilla/5.0 (Linux; Android 15; Pixel 9 Build/AP3A.241105.007; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/136.0.7103.125 Mobile Safari/537.36 Instagram 388.0.0.0.0 Android",
			corner: "top-right",
			arrow: "right-up",
		},
		{
			appKey: "facebook",
			platform: "ios",
			userAgent: `${IOS_BASE} FBIOS/500.0.0.0.0`,
			corner: "top-right",
			arrow: "right-up",
		},
		{
			appKey: "facebook",
			platform: "android",
			userAgent:
				"Mozilla/5.0 (Linux; Android 15; Pixel 9 Build/AP3A.241105.007; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/136.0.7103.125 Mobile Safari/537.36 Facebook/500.0.0.0.0",
			corner: "top-right",
			arrow: "right-up",
		},
		{
			appKey: "line",
			platform: "ios",
			userAgent: `${IOS_BASE} Line/15.10.1`,
			corner: "bottom-right",
			arrow: "right-down",
		},
		{
			appKey: "line",
			platform: "android",
			userAgent:
				"Mozilla/5.0 (Linux; Android 15; Pixel 9 Build/AP3A.241105.007; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/136.0.7103.125 Mobile Safari/537.36 Line/15.10.1",
			corner: "bottom-right",
			arrow: "right-down",
		},
	] as const;

	for (const browserCase of cases) {
		const browser = detectInAppBrowser(browserCase.userAgent);
		const guide = getInAppBrowserNativeMenuGuide(browser);
		assert.equal(browser.appKey, browserCase.appKey);
		assert.equal(browser.platform, browserCase.platform);
		assert.equal(guide.corner, browserCase.corner);
		assert.equal(guide.arrow, browserCase.arrow);
	}

	assert.deepEqual(
		getInAppBrowserNativeMenuGuide(
			detectInAppBrowser(`${IOS_BASE} Line/15.10.1`),
		),
		{
			corner: "bottom-right",
			arrow: "right-down",
			cornerLabel: "右下角",
			menuSymbol: "⋮",
			actionLabel: "在瀏覽器中開啟",
		},
	);
	assert.equal(
		getInAppBrowserNativeMenuGuide(
			detectInAppBrowser(`${IOS_BASE} Barcelona 388.0.0.0.0`),
		).actionLabel,
		"使用外部瀏覽器開啟",
	);
	assert.equal(
		getInAppBrowserNativeMenuGuide(
			detectInAppBrowser(`${IOS_BASE} Instagram 388.0.0.0.0`),
		).actionLabel,
		"在外部瀏覽器開啟",
	);
	assert.equal(
		getInAppBrowserNativeMenuGuide(
			detectInAppBrowser(`${IOS_BASE} FBIOS/500.0.0.0.0`),
		).actionLabel,
		"使用外部瀏覽器開啟",
	);
});

test("unknown and generic embedded browsers use the safe top-right fallback", () => {
	assert.equal(
		getInAppBrowserNativeMenuGuide({
			isInApp: true,
			appKey: "future-social-app",
			appName: "Future App",
			platform: "ios",
		}).corner,
		"top-right",
	);
	assert.equal(
		getInAppBrowserNativeMenuGuide({
			isInApp: true,
			appKey: null,
			appName: null,
			platform: "android",
		}).arrow,
		"right-up",
	);
});

test("external handoff preserves redirect and selected role but removes preview state", () => {
	assert.equal(
		buildLoginBrowserHandoffUrl(
			"https://example.com/login?redirect=%2Fcheckout&role=buyer&webviewGuide=sheet#error",
			"seller",
		),
		"https://example.com/login?redirect=%2Fcheckout&role=seller",
	);
});

test("builds an Android browser intent from the canonical handoff URL", () => {
	assert.equal(
		buildAndroidExternalBrowserIntent(
			"https://example.com/login?redirect=%2Fcheckout&role=buyer",
		),
		"intent://example.com/login?redirect=%2Fcheckout&role=buyer#Intent;scheme=https;action=android.intent.action.VIEW;category=android.intent.category.BROWSABLE;S.browser_fallback_url=https%3A%2F%2Fexample.com%2Flogin%3Fredirect%3D%252Fcheckout%26role%3Dbuyer;end",
	);
});

test("Android intent strips fragments before the Intent parameter section", () => {
	const intent = buildAndroidExternalBrowserIntent(
		"https://example.com/login?role=buyer#Intent;scheme=evil;end",
	);

	assert.equal(intent.match(/#Intent/g)?.length, 1);
	assert.doesNotMatch(intent, /scheme=evil/);
});

test("builds the documented Chrome for iOS URL scheme", () => {
	assert.equal(
		buildIosChromeExternalBrowserUrl(
			"https://example.com/login?redirect=%2Fcheckout&role=buyer#ignored",
		),
		"googlechromes://example.com/login?redirect=%2Fcheckout&role=buyer",
	);
	assert.equal(
		buildIosChromeExternalBrowserUrl("http://localhost:3107/login?role=seller"),
		"googlechrome://localhost:3107/login?role=seller",
	);
});

test("handoff helpers reject non-web protocols", () => {
	assert.throws(
		() => buildLoginBrowserHandoffUrl("javascript:alert(1)", "buyer"),
		/HTTP\(S\)/,
	);
	assert.throws(
		() => buildAndroidExternalBrowserIntent("file:///tmp/login"),
		/HTTP\(S\)/,
	);
	assert.throws(
		() => buildIosChromeExternalBrowserUrl("javascript:alert(1)"),
		/HTTP\(S\)/,
	);
});
