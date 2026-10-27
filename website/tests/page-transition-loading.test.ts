import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import test from "node:test";
import {
	isScrollManagedHistoryState,
	isScrollManagedSamePathHistoryState,
	SCROLL_MANAGED_HISTORY_PATH_KEY,
	scheduleLocationChangeEvent,
} from "../lib/navigation/location-change";

const pageTransitionPath = new URL(
	"../components/shared/PageTransition.tsx",
	import.meta.url,
);
const rootLayoutPath = new URL(
	"../components/shared/RootDocument.tsx",
	import.meta.url,
);
const appDirectory = new URL("../app/", import.meta.url);
const routeLoadingSkeletonCssPath = new URL(
	"../styles/components/route-loading-skeleton.css",
	import.meta.url,
);

async function findRouteLoadingFiles(
	directory: URL,
	prefix = "",
): Promise<string[]> {
	const entries = await readdir(directory, { withFileTypes: true });
	const nested = await Promise.all(
		entries.map(async (entry) => {
			const relativePath = `${prefix}${entry.name}`;
			if (entry.isDirectory()) {
				return findRouteLoadingFiles(
					new URL(`${encodeURIComponent(entry.name)}/`, directory),
					`${relativePath}/`,
				);
			}
			return entry.name === "loading.tsx" ? [relativePath] : [];
		}),
	);
	return nested.flat().sort();
}

test("PageTransition controller never paints a navigation fallback", async () => {
	const source = await readFile(pageTransitionPath, "utf8");

	assert.match(source, /getAnchorNavigationUrl/);
	assert.match(source, /document\.addEventListener\("click"/);
	assert.match(source, /a\[data-hk-hard-navigation\]/);
	assert.match(source, /window\.history\.pushState = patchedPushState/);
	assert.equal(
		[...source.matchAll(/scheduleLocationChangeEvent\(window\)/g)].length,
		2,
		"pushState and replaceState must both leave React's insertion-effect callstack",
	);
	assert.doesNotMatch(
		source,
		/window\.dispatchEvent\(new Event\(LOCATION_CHANGE_EVENT\)\)/,
	);
	assert.match(source, /isScrollManagedHistoryState/);
	assert.match(source, /if \(!scrollManaged\) handleHistoryUrl/);
	assert.match(source, /window\.addEventListener\("popstate"/);
	assert.doesNotMatch(source, /<RouteLoadingSkeleton/);
	assert.doesNotMatch(source, /captureRouteLoadingScope/);
	assert.match(source, /controllerOnly/);
});

test("programmatic routes animate only after commit without moving page geometry", async () => {
	const source = await readFile(pageTransitionPath, "utf8");
	const start = source.indexOf("Direct router.push/replace calls");
	const end = source.indexOf("const endNavPendingRef", start);
	const arrivalEffect = source.slice(start, end);

	assert.ok(start >= 0);
	assert.ok(end > start);
	assert.match(arrivalEffect, /consumeProgrammaticRouteMotion\(pathname\)/);
	assert.match(arrivalEffect, /main\.page-content > \.page-transition/);
	assert.match(arrivalEffect, /\[\{ opacity: 0\.92 \}, \{ opacity: 1 \}\]/);
	assert.match(arrivalEffect, /duration: 200/);
	assert.match(arrivalEffect, /prefers-reduced-motion: reduce/);
	assert.doesNotMatch(arrivalEffect, /transform:\s/);
});

test("history commit notification leaves insertion-effect callstacks and coalesces", async () => {
	const target = new EventTarget();
	let notifications = 0;
	target.addEventListener("hk:locationchange", () => {
		notifications += 1;
	});

	scheduleLocationChangeEvent(target);
	scheduleLocationChangeEvent(target);
	assert.equal(
		notifications,
		0,
		"subscribers must not run synchronously inside history.pushState",
	);

	await new Promise<void>((resolve) => queueMicrotask(resolve));
	assert.equal(
		notifications,
		1,
		"same-tick history commits should notify once",
	);

	scheduleLocationChangeEvent(target);
	await new Promise<void>((resolve) => queueMicrotask(resolve));
	assert.equal(
		notifications,
		2,
		"a completed notification must release the target for the next navigation",
	);
});

test("scroll-managed history state is restricted to its owning pathname", () => {
	const state = { [SCROLL_MANAGED_HISTORY_PATH_KEY]: "/seller-guide" };
	assert.equal(isScrollManagedHistoryState(state, "/seller-guide"), true);
	assert.equal(isScrollManagedHistoryState(state, "/market/products"), false);
	assert.equal(isScrollManagedHistoryState(null, "/seller-guide"), false);
	assert.equal(
		isScrollManagedSamePathHistoryState(
			state,
			"/seller-guide",
			"/seller-guide",
		),
		true,
	);
	assert.equal(
		isScrollManagedSamePathHistoryState(state, "/about", "/seller-guide"),
		false,
	);
});

test("root keeps streamed children server-owned without route loading boundaries", async () => {
	const [layout, loadingFiles] = await Promise.all([
		readFile(rootLayoutPath, "utf8"),
		findRouteLoadingFiles(appDirectory),
	]);

	assert.match(layout, /<PageTransition controllerOnly \/>/);
	assert.doesNotMatch(
		layout,
		/<Suspense fallback=\{null\}>\s*<div className="page-transition">\{children\}<\/div>\s*<\/Suspense>/,
	);
	assert.match(layout, /<div className="page-transition">\{children\}<\/div>/);
	assert.deepEqual(loadingFiles, []);
});

test("route skeleton has visible animated styles", async () => {
	const source = await readFile(routeLoadingSkeletonCssPath, "utf8");

	assert.match(source, /\.route-loading-skeleton/);
	assert.match(source, /\.route-loading-skeleton__grid/);
	assert.match(source, /@keyframes routeLoadingSkeletonShimmer/);
	assert.match(source, /prefers-reduced-motion/);
});
