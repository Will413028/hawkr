import assert from "node:assert/strict";
import { afterEach, test } from "node:test";
import {
	acquireBodyScrollLock,
	resetBodyScrollLocksForTest,
} from "../lib/browser/body-scroll-lock";

type FakeStyle = Record<string, string>;

function installDom(scrollY = 240) {
	const bodyStyle: FakeStyle = {
		left: "",
		overflow: "",
		overscrollBehavior: "",
		position: "",
		right: "",
		top: "",
		width: "",
	};
	const rootStyle: FakeStyle = {
		overflow: "",
		overscrollBehavior: "",
		scrollBehavior: "",
	};
	const scrollCalls: Array<[number, number]> = [];
	let actualScrollY = scrollY;
	Object.assign(globalThis, {
		document: {
			body: { style: bodyStyle },
			documentElement: { style: rootStyle },
		},
		window: {
			get scrollY() {
				return actualScrollY;
			},
			scrollTo: (x: number, y: number) => {
				scrollCalls.push([x, y]);
				actualScrollY =
					bodyStyle.overflow === "hidden" || rootStyle.overflow === "hidden"
						? 0
						: y;
			},
		},
	});
	return {
		bodyStyle,
		rootStyle,
		scrollCalls,
		readActualScrollY: () => actualScrollY,
	};
}

afterEach(() => {
	resetBodyScrollLocksForTest();
	Reflect.deleteProperty(globalThis, "document");
	Reflect.deleteProperty(globalThis, "window");
});

test("overlapping locks restore the original overflow only after the final release", () => {
	const { bodyStyle } = installDom();
	const releaseDialog = acquireBodyScrollLock();
	const releaseDrawer = acquireBodyScrollLock();

	assert.equal(bodyStyle.overflow, "hidden");
	releaseDialog();
	assert.equal(bodyStyle.overflow, "hidden");
	releaseDrawer();
	assert.equal(bodyStyle.overflow, "");
});

test("release functions are idempotent and cannot unlock another overlay", () => {
	const { bodyStyle } = installDom();
	const releaseA = acquireBodyScrollLock();
	const releaseB = acquireBodyScrollLock();

	releaseA();
	releaseA();
	assert.equal(bodyStyle.overflow, "hidden");
	releaseB();
	assert.equal(bodyStyle.overflow, "");
});

test("fixed and overflow locks can close in either order without stranding styles", () => {
	const { bodyStyle, rootStyle, scrollCalls } = installDom(360);
	const releaseDialog = acquireBodyScrollLock();
	const releaseViewer = acquireBodyScrollLock({
		preserveScrollPosition: true,
		lockRoot: true,
		preventOverscroll: true,
	});

	assert.equal(bodyStyle.position, "fixed");
	assert.equal(bodyStyle.top, "-360px");
	assert.equal(rootStyle.overflow, "hidden");

	releaseViewer();
	assert.equal(bodyStyle.position, "");
	assert.equal(bodyStyle.overflow, "hidden");
	assert.deepEqual(scrollCalls, [[0, 360]]);

	releaseDialog();
	assert.equal(bodyStyle.overflow, "");
	assert.equal(rootStyle.overflow, "");
});

test("the final fixed-lock release restores the real nonzero scroll offset", () => {
	const { bodyStyle, rootStyle, scrollCalls, readActualScrollY } =
		installDom(120);
	const releaseViewer = acquireBodyScrollLock({
		preserveScrollPosition: true,
		scrollY: 120,
		lockRoot: true,
	});

	assert.equal(bodyStyle.top, "-120px");
	assert.equal(bodyStyle.overflow, "hidden");
	assert.equal(rootStyle.overflow, "hidden");

	releaseViewer();

	assert.equal(bodyStyle.overflow, "");
	assert.equal(rootStyle.overflow, "");
	assert.deepEqual(scrollCalls, [[0, 120]]);
	assert.equal(readActualScrollY(), 120);
});

test("releasing without restore un-pins the body but leaves the new route at the top", () => {
	const { bodyStyle, scrollCalls } = installDom(2400);
	const releaseQuickView = acquireBodyScrollLock({
		preserveScrollPosition: true,
		scrollY: 2400,
	});

	assert.equal(bodyStyle.position, "fixed");
	assert.equal(bodyStyle.top, "-2400px");

	// The document moved on to another route while the overlay was open — the
	// pinned offset belongs to the page underneath, not to the arriving one.
	releaseQuickView({ restoreScrollPosition: false });

	assert.equal(bodyStyle.position, "");
	assert.equal(bodyStyle.top, "");
	assert.deepEqual(scrollCalls, []);
});

test("a non-restoring release still lets a later lock pin its own offset", () => {
	const { bodyStyle, scrollCalls } = installDom(2400);
	acquireBodyScrollLock({ preserveScrollPosition: true, scrollY: 2400 })({
		restoreScrollPosition: false,
	});

	const releaseNext = acquireBodyScrollLock({
		preserveScrollPosition: true,
		scrollY: 120,
	});
	assert.equal(bodyStyle.top, "-120px");

	releaseNext();
	assert.deepEqual(scrollCalls, [[0, 120]]);
});
