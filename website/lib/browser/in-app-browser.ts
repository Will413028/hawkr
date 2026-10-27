import InAppSpy from "inapp-spy";

export type MobileBrowserPlatform = "ios" | "android" | "other";

export interface InAppBrowserInfo {
	isInApp: boolean;
	appKey: string | null;
	appName: string | null;
	platform: MobileBrowserPlatform;
}

export type InAppBrowserGuideCorner =
	| "top-left"
	| "top-right"
	| "bottom-left"
	| "bottom-right";

export type InAppBrowserGuideArrow =
	| "left-up"
	| "right-up"
	| "left-down"
	| "right-down";

export interface InAppBrowserNativeMenuGuide {
	corner: InAppBrowserGuideCorner;
	arrow: InAppBrowserGuideArrow;
	cornerLabel: string;
	menuSymbol: "⋯" | "⋮";
	actionLabel: string;
}

export type LoginRoleHint = "buyer" | "seller";

const APP_DISPLAY_NAMES: Record<string, string> = {
	line: "LINE",
	twitter: "X",
};

export const EMPTY_IN_APP_BROWSER_INFO: InAppBrowserInfo = {
	isInApp: false,
	appKey: null,
	appName: null,
	platform: "other",
};

const DEFAULT_TOP_RIGHT_MENU_GUIDE: InAppBrowserNativeMenuGuide = {
	corner: "top-right",
	arrow: "right-up",
	cornerLabel: "右上角",
	menuSymbol: "⋯",
	actionLabel: "在外部瀏覽器開啟",
};

const THREADS_TOP_RIGHT_MENU_GUIDE: InAppBrowserNativeMenuGuide = {
	...DEFAULT_TOP_RIGHT_MENU_GUIDE,
	actionLabel: "使用外部瀏覽器開啟",
};

const LINE_BOTTOM_RIGHT_MENU_GUIDE: InAppBrowserNativeMenuGuide = {
	corner: "bottom-right",
	arrow: "right-down",
	cornerLabel: "右下角",
	menuSymbol: "⋮",
	actionLabel: "在瀏覽器中開啟",
};

/**
 * Native in-app-browser chrome moves independently of the web page and differs
 * by host app. Keep this table data-driven so an app release can be corrected
 * without changing the guide layout. Unknown versions retain a safe top-right
 * fallback instead of guessing at a different edge.
 */
const NATIVE_MENU_GUIDES: Readonly<
	Partial<
		Record<
			string,
			Readonly<
				Partial<Record<MobileBrowserPlatform, InAppBrowserNativeMenuGuide>>
			>
		>
	>
> = {
	threads: {
		ios: THREADS_TOP_RIGHT_MENU_GUIDE,
		android: THREADS_TOP_RIGHT_MENU_GUIDE,
		other: THREADS_TOP_RIGHT_MENU_GUIDE,
	},
	facebook: {
		ios: THREADS_TOP_RIGHT_MENU_GUIDE,
		android: THREADS_TOP_RIGHT_MENU_GUIDE,
		other: THREADS_TOP_RIGHT_MENU_GUIDE,
	},
	instagram: {
		ios: DEFAULT_TOP_RIGHT_MENU_GUIDE,
		android: DEFAULT_TOP_RIGHT_MENU_GUIDE,
		other: DEFAULT_TOP_RIGHT_MENU_GUIDE,
	},
	line: {
		ios: LINE_BOTTOM_RIGHT_MENU_GUIDE,
		android: LINE_BOTTOM_RIGHT_MENU_GUIDE,
		other: LINE_BOTTOM_RIGHT_MENU_GUIDE,
	},
};

function platformFromUserAgent(userAgent: string): MobileBrowserPlatform {
	if (/Android/i.test(userAgent)) return "android";
	if (/iPhone|iPad|iPod/i.test(userAgent)) return "ios";
	return "other";
}

/**
 * Detect the social-app browser that owns the current page.
 *
 * Passing a user agent keeps this safe for a Server Component. Omitting it
 * lets inapp-spy use its client-only checks as well (notably Telegram).
 */
export function detectInAppBrowser(userAgent?: string): InAppBrowserInfo {
	const result = userAgent
		? InAppSpy({ ua: userAgent })
		: typeof window === "undefined"
			? {
					...EMPTY_IN_APP_BROWSER_INFO,
					ua: "",
					skipped: false,
				}
			: InAppSpy();
	const appKey = result.appKey ?? null;

	return {
		isInApp: result.isInApp,
		appKey,
		appName: appKey
			? (APP_DISPLAY_NAMES[appKey] ?? result.appName ?? null)
			: null,
		platform: platformFromUserAgent(result.ua),
	};
}

export function getInAppBrowserNativeMenuGuide(
	browser: InAppBrowserInfo,
): InAppBrowserNativeMenuGuide {
	if (!browser.appKey) return DEFAULT_TOP_RIGHT_MENU_GUIDE;
	const appGuide = NATIVE_MENU_GUIDES[browser.appKey];
	return (
		appGuide?.[browser.platform] ??
		appGuide?.other ??
		DEFAULT_TOP_RIGHT_MENU_GUIDE
	);
}

/**
 * Preserve the requested destination while carrying the role selected on the
 * login page into the external browser. Preview-only guide controls are
 * removed so copied links remain canonical.
 */
export function buildLoginBrowserHandoffUrl(
	currentUrl: string,
	role: LoginRoleHint,
): string {
	const url = new URL(currentUrl);
	if (url.protocol !== "https:" && url.protocol !== "http:") {
		throw new TypeError("Login handoff requires an HTTP(S) URL");
	}

	const redirect = url.searchParams.get("redirect");
	url.search = "";
	if (redirect) url.searchParams.set("redirect", redirect);
	url.searchParams.set("role", role);
	url.hash = "";
	return url.toString();
}

/**
 * Android in-app browsers can hand an intent URL to the OS browser chooser.
 * The call must still happen directly inside a trusted tap handler.
 */
export function buildAndroidExternalBrowserIntent(urlValue: string): string {
	const url = new URL(urlValue);
	if (url.protocol !== "https:" && url.protocol !== "http:") {
		throw new TypeError("External browser intent requires an HTTP(S) URL");
	}
	url.hash = "";
	const fallbackUrl = url.toString();
	const target = `${url.host}${url.pathname}${url.search}`;
	const scheme = url.protocol.slice(0, -1);

	return `intent://${target}#Intent;scheme=${scheme};action=android.intent.action.VIEW;category=android.intent.category.BROWSABLE;S.browser_fallback_url=${encodeURIComponent(fallbackUrl)};end`;
}

/**
 * Chrome for iOS registers googlechrome(s) URL schemes. Safari itself exposes
 * no equivalent public web URL scheme, so this is an installed-Chrome
 * best-effort handoff and the UI must retain a manual/copy fallback.
 */
export function buildIosChromeExternalBrowserUrl(urlValue: string): string {
	const url = new URL(urlValue);
	if (url.protocol !== "https:" && url.protocol !== "http:") {
		throw new TypeError("iOS Chrome handoff requires an HTTP(S) URL");
	}
	url.hash = "";
	const chromeScheme =
		url.protocol === "https:" ? "googlechromes:" : "googlechrome:";

	return `${chromeScheme}${url.toString().slice(url.protocol.length)}`;
}
