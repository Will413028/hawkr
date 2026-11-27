import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	INITIAL_LOADER_SKIP_CLASS,
	INITIAL_LOADER_TIMEOUT_CLASS,
	INITIAL_LOADER_WATCHDOG_MS,
	INITIAL_LOADER_WATCHDOG_SCRIPT,
} from "../lib/loading/initial-loader-watchdog";

const initialPageLoaderPath = new URL(
	"../components/shared/InitialPageLoader.tsx",
	import.meta.url,
);
const initialRouteReadyPath = new URL(
	"../lib/loading/initial-route-ready.ts",
	import.meta.url,
);
const homePagePath = new URL("../features/home/HomePage.tsx", import.meta.url);
const rootLayoutPath = new URL(
	"../components/shared/RootDocument.tsx",
	import.meta.url,
);
const floatingNavigationStylesPath = new URL(
	"../styles/components/floating-navigation.css",
	import.meta.url,
);
const loaderStylesPath = new URL(
	"../styles/components/decorations-overlays.css",
	import.meta.url,
);

test("home splash waits for the browser home-content marker", async () => {
	const [loaderSource, routeReadySource, homeSource] = await Promise.all([
		readFile(initialPageLoaderPath, "utf8"),
		readFile(initialRouteReadyPath, "utf8"),
		readFile(homePagePath, "utf8"),
	]);

	assert.match(loaderSource, /routeContentReadySelectors/);
	assert.match(loaderSource, /ROUTE_TERMINAL_READY_SELECTOR/);
	assert.match(loaderSource, /checkRouteChanged\(\);\s*checkRouteContent\(\);/);
	assert.match(loaderSource, /scheduleTimeout\(releaseGateAtDeadline/);
	assert.doesNotMatch(loaderSource, /目前無法載入頁面/);
	assert.doesNotMatch(loaderSource, /window\.location\.reload/);
	assert.match(loaderSource, /const timers: number\[\] = \[\]/);
	assert.match(loaderSource, /cancelAnimationFrame/);
	assert.match(
		loaderSource,
		/root\.classList\.remove\("initial-loader-active"\);\s+const revealFrame = window\.requestAnimationFrame\(\(\) => \{\s+if \(disposed\) return;\s+setLeaving\(true\)/,
	);
	assert.match(loaderSource, /document\.readyState !== "loading"/);
	assert.match(loaderSource, /addEventListener\("DOMContentLoaded"/);
	assert.doesNotMatch(loaderSource, /addEventListener\("load"/);
	assert.match(routeReadySource, /HOME_CONTENT_READY_SELECTOR/);
	assert.match(routeReadySource, /data-hk-tab-content-ready="feed"/);
	assert.match(routeReadySource, /data-hk-tab-content-ready="market"/);
	assert.match(homeSource, /data-hk-initial-content-ready="home"/);
});

test("splash releases its content gate when the route changes under it", async () => {
	const loaderSource = await readFile(initialPageLoaderPath, "utf8");

	assert.match(
		loaderSource,
		/const entryPathname = window\.location\.pathname/,
	);
	assert.match(
		loaderSource,
		/if \(window\.location\.pathname === entryPathname\) return;/,
	);
	assert.match(loaderSource, /checkRouteChanged\(\);\s*checkRouteContent\(\);/);
	assert.doesNotMatch(loaderSource, /setInterval/);

	assert.doesNotMatch(loaderSource, /from "next\/navigation"/);
});

test("every splash exit path releases the document-settled gate", async () => {
	const [loaderSource, settledSource] = await Promise.all([
		readFile(initialPageLoaderPath, "utf8"),
		readFile(
			new URL("../lib/hydration/document-settled.ts", import.meta.url),
			"utf8",
		),
	]);

	assert.match(
		loaderSource,
		/finished = true;\s*contentObserver\?\.disconnect\(\);\s*publicHandoffObserver\?\.disconnect\(\);\s*hide\(\);/,
	);
	assert.match(
		loaderSource,
		/document\.documentElement\.classList\.remove\("initial-loader-active"\)/,
	);

	assert.match(settledSource, /classList\.contains\("initial-loader-active"\)/);
	assert.match(settledSource, /attributeFilter: \["class"\]/);
});

test("document watchdog releases the splash without React hydration", () => {
	let callback: (() => void) | undefined;
	let delay = 0;
	const classes = new Set<string>();
	const window = {
		setTimeout(next: () => void, timeout: number) {
			callback = next;
			delay = timeout;
		},
	};
	const document = {
		documentElement: {
			classList: { add: (name: string) => classes.add(name) },
		},
	};

	new Function("window", "document", INITIAL_LOADER_WATCHDOG_SCRIPT)(
		window,
		document,
	);
	assert.equal(delay, INITIAL_LOADER_WATCHDOG_MS);
	assert.equal(classes.size, 0);
	callback?.();
	assert.equal(classes.has(INITIAL_LOADER_TIMEOUT_CLASS), true);
});

test("auth entry skips the document splash before first paint", () => {
	const classes = new Set(["initial-loader-active"]);
	const window = {
		location: { pathname: "/login" },
		setTimeout: () => assert.fail("must not arm watchdog"),
	};
	const document = {
		documentElement: {
			classList: {
				add: (name: string) => classes.add(name),
				remove: (name: string) => classes.delete(name),
			},
		},
	};

	new Function("window", "document", INITIAL_LOADER_WATCHDOG_SCRIPT)(
		window,
		document,
	);
	assert.equal(classes.has("initial-loader-active"), false);
	assert.equal(classes.has(INITIAL_LOADER_SKIP_CLASS), true);
});

test("layout and CSS keep independent loader hard-stop paths wired", async () => {
	const [layout, styles, floatingNavigationStyles] = await Promise.all([
		readFile(rootLayoutPath, "utf8"),
		readFile(loaderStylesPath, "utf8"),
		readFile(floatingNavigationStylesPath, "utf8"),
	]);

	assert.match(layout, /INITIAL_LOADER_WATCHDOG_SCRIPT/);
	assert.match(layout, /id="hk-initial-loader-watchdog"/);
	assert.doesNotMatch(layout, /strategy="beforeInteractive"/);
	assert.match(layout, /initial-loader-active/);
	assert.doesNotMatch(styles, /initial-loader-active \.page-wrapper/);
	assert.doesNotMatch(styles, /initial-content-hard-release/);
	assert.doesNotMatch(styles, /hk-initial-loader-terminal-error/);
	assert.match(styles, /\.hk-initial-loader-timeout \.initial-loader/);
	assert.match(styles, /\.hk-skip-initial-loader \.initial-loader/);
	assert.match(
		styles,
		/The splash is visual feedback, never an interaction lock/,
	);
	assert.match(styles, /initial-loader-hard-timeout 1ms linear 6s forwards/);
	assert.match(layout, /className="root-nav-layout-reservation"/);
	assert.match(
		floatingNavigationStyles,
		/\.root-nav-layout-reservation,\s*\.auto-nav\s*\{[\s\S]*?min-height: calc\(/,
	);
	assert.match(floatingNavigationStyles, /--floating-nav-height, 84px/);
	assert.match(floatingNavigationStyles, /--floating-nav-top-gap, 16px/);
});

test("home keeps the initial loader as the only pending surface", async () => {
	const homeSource = await readFile(homePagePath, "utf8");

	assert.match(homeSource, /^"use client";/);
	assert.match(
		homeSource,
		/if \(!homeQuery\.data\) \{[\s\S]*?if \(homeQuery\.isError\) \{[\s\S]*?<ErrorState[\s\S]*?\}[\s\S]*?return null;\s*\}/,
	);
	assert.doesNotMatch(homeSource, /HomeContentFallback/);
	assert.doesNotMatch(homeSource, /HomeShelfSkeleton/);
	assert.doesNotMatch(homeSource, /from "next\/server"/);
	assert.doesNotMatch(homeSource, /await connection\(/);
	assert.match(homeSource, /<Suspense fallback=\{null\}>\s*<HomeContent\b/);
	assert.equal(homeSource.match(/<Suspense/g)?.length, 1);
	assert.doesNotMatch(homeSource, /RouteLoadingSkeleton/);
});

test("the English auth entry skips the document splash too", () => {
	const classes = new Set(["initial-loader-active"]);
	const window = {
		location: { pathname: "/en/login" },
		setTimeout: () => assert.fail("must not arm watchdog"),
	};
	const document = {
		documentElement: {
			classList: {
				add: (name: string) => classes.add(name),
				remove: (name: string) => classes.delete(name),
			},
		},
	};

	new Function("window", "document", INITIAL_LOADER_WATCHDOG_SCRIPT)(
		window,
		document,
	);
	assert.equal(classes.has("initial-loader-active"), false);
	assert.equal(classes.has(INITIAL_LOADER_SKIP_CLASS), true);
});
