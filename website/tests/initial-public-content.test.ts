import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";
import { ACTIVE_ROLE_STORAGE_KEY } from "../lib/auth/active-role";
import { INITIAL_LOADER_WATCHDOG_MS } from "../lib/loading/initial-loader-watchdog";
import {
	INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
	INITIAL_PUBLIC_CONTENT_READY_AT,
	INITIAL_PUBLIC_CONTENT_READY_CLASS,
	INITIAL_PUBLIC_CONTENT_SCRIPT,
} from "../lib/loading/initial-public-content";

interface ScheduledTimer {
	callback: () => void;
	delay: number;
}

function createHarness({
	pathname = "/",
	search = "",
	body = "",
	readyState = "interactive",
}: {
	pathname?: string;
	search?: string;
	body?: string;
	readyState?: DocumentReadyState;
} = {}) {
	const dom = new JSDOM(
		`<!doctype html><html class="initial-loader-active"><body>${body}</body></html>`,
		{
			pretendToBeVisual: true,
			runScripts: "outside-only",
			url: `https://hawkr.test${pathname}${search}`,
		},
	);
	const { window } = dom;
	const { document } = window;
	let currentReadyState = readyState;
	let nextHandle = 1;
	const frames = new Map<number, FrameRequestCallback>();
	const timers = new Map<number, ScheduledTimer>();

	Object.defineProperty(document, "readyState", {
		configurable: true,
		get: () => currentReadyState,
	});
	Object.defineProperty(window, "requestAnimationFrame", {
		configurable: true,
		value: (callback: FrameRequestCallback) => {
			const handle = nextHandle++;
			frames.set(handle, callback);
			return handle;
		},
	});
	Object.defineProperty(window, "cancelAnimationFrame", {
		configurable: true,
		value: (handle: number) => {
			frames.delete(handle);
		},
	});
	Object.defineProperty(window, "setTimeout", {
		configurable: true,
		value: (callback: TimerHandler, delay = 0) => {
			assert.equal(typeof callback, "function");
			const handle = nextHandle++;
			timers.set(handle, {
				callback: callback as () => void,
				delay: Number(delay),
			});
			return handle;
		},
	});
	Object.defineProperty(window, "clearTimeout", {
		configurable: true,
		value: (handle: number) => {
			timers.delete(handle);
		},
	});

	const flushFrames = () => {
		const pending = [...frames.entries()];
		frames.clear();
		for (const [, callback] of pending) {
			callback(window.performance.now());
		}
	};
	const deliverMutations = async () => {
		await new Promise<void>((resolve) => queueMicrotask(resolve));
	};
	const fireTimersAt = (delay: number) => {
		for (const [handle, timer] of [...timers.entries()]) {
			if (timer.delay !== delay) continue;
			timers.delete(handle);
			timer.callback();
		}
	};

	return {
		dom,
		window,
		document,
		frames,
		timers,
		flushFrames,
		deliverMutations,
		fireTimersAt,
		setReadyState(next: DocumentReadyState) {
			currentReadyState = next;
		},
		run() {
			window.eval(INITIAL_PUBLIC_CONTENT_SCRIPT);
		},
	};
}

test("guest SSR marker starts the visual handoff without releasing the React barrier", () => {
	const harness = createHarness({
		body: '<span data-hk-initial-content-ready="home" hidden></span>',
	});
	try {
		harness.run();
		harness.flushFrames();

		const classes = harness.document.documentElement.classList;
		assert.equal(classes.contains(INITIAL_PUBLIC_CONTENT_ENABLED_CLASS), true);
		assert.equal(classes.contains(INITIAL_PUBLIC_CONTENT_READY_CLASS), true);
		assert.equal(classes.contains("initial-loader-active"), true);
		const readyAt = harness.document.documentElement.getAttribute(
			INITIAL_PUBLIC_CONTENT_READY_AT,
		);
		assert.notEqual(readyAt, null);
		assert.equal(Number.isFinite(Number(readyAt)), true);
		assert.equal(harness.timers.size, 0, "ready handoff clears its deadline");
	} finally {
		harness.dom.window.close();
	}
});

test("a protected route never opts into the public visual handoff", () => {
	const harness = createHarness({
		pathname: "/account/notifications",
		body: '<span data-hk-page-content-ready="notifications" hidden></span>',
	});
	try {
		harness.run();
		harness.flushFrames();
		const classes = harness.document.documentElement.classList;
		assert.equal(classes.contains(INITIAL_PUBLIC_CONTENT_ENABLED_CLASS), false);
		assert.equal(classes.contains(INITIAL_PUBLIC_CONTENT_READY_CLASS), false);
		assert.equal(classes.contains("initial-loader-active"), true);
		assert.equal(harness.timers.size, 0);
	} finally {
		harness.dom.window.close();
	}
});

test("a committed public terminal state can replace the normal ready marker", () => {
	const harness = createHarness({
		body: '<main data-hk-route-terminal-ready="error"></main>',
	});
	try {
		harness.run();
		harness.flushFrames();
		const classes = harness.document.documentElement.classList;
		assert.equal(classes.contains(INITIAL_PUBLIC_CONTENT_READY_CLASS), true);
		assert.equal(classes.contains("initial-loader-active"), true);
	} finally {
		harness.dom.window.close();
	}
});

test("a client-owned query keeps the canonical SSR snapshot behind the React gate", () => {
	const harness = createHarness({
		pathname: "/search",
		search: "?q=%E7%95%AA%E8%8C%84",
		body: '<main data-hk-page-content-ready="search"></main>',
	});
	try {
		harness.run();
		harness.flushFrames();
		const classes = harness.document.documentElement.classList;
		assert.equal(classes.contains(INITIAL_PUBLIC_CONTENT_ENABLED_CLASS), false);
		assert.equal(classes.contains(INITIAL_PUBLIC_CONTENT_READY_CLASS), false);
		assert.equal(classes.contains("initial-loader-active"), true);
	} finally {
		harness.dom.window.close();
	}
});

test("session and storage signals fail closed before enabling the public fast path", async (t) => {
	await t.test("stored active role", () => {
		const harness = createHarness({
			body: '<span data-hk-initial-content-ready="home" hidden></span>',
		});
		try {
			harness.window.localStorage.setItem(ACTIVE_ROLE_STORAGE_KEY, "seller");
			harness.run();
			harness.flushFrames();
			assert.equal(
				harness.document.documentElement.classList.contains(
					INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
				),
				false,
			);
		} finally {
			harness.dom.window.close();
		}
	});

	await t.test("storage access failure", () => {
		const harness = createHarness({
			body: '<span data-hk-initial-content-ready="home" hidden></span>',
		});
		try {
			Object.defineProperty(harness.window, "localStorage", {
				configurable: true,
				get: () => {
					throw new Error("storage unavailable");
				},
			});
			harness.run();
			harness.flushFrames();
			assert.equal(
				harness.document.documentElement.classList.contains(
					INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
				),
				false,
			);
			assert.equal(
				harness.document.documentElement.classList.contains(
					"initial-loader-active",
				),
				true,
			);
		} finally {
			harness.dom.window.close();
		}
	});
});

test("a hidden Flight ancestor blocks readiness while the intentionally hidden marker does not", async () => {
	const harness = createHarness({
		body: '<div id="flight" hidden><span data-hk-initial-content-ready="home" hidden></span></div>',
	});
	try {
		harness.run();
		harness.flushFrames();
		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			false,
		);

		harness.document.getElementById("flight")?.removeAttribute("hidden");
		await harness.deliverMutations();
		harness.flushFrames();
		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			true,
		);
	} finally {
		harness.dom.window.close();
	}
});

test("seller storefront waits for both independently streamed markers", async () => {
	const harness = createHarness({
		pathname: "/sellers/farm-42",
		body: "<header data-hk-seller-profile-ready></header>",
	});
	try {
		harness.run();
		harness.flushFrames();
		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			false,
		);

		const marker = harness.document.createElement("span");
		marker.hidden = true;
		marker.setAttribute("data-hk-page-content-ready", "seller-storefront");
		harness.document.body.append(marker);
		await harness.deliverMutations();
		harness.flushFrames();
		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			true,
		);
	} finally {
		harness.dom.window.close();
	}
});

test("product handoff rejects another product's marker and accepts the decoded route id", async () => {
	const harness = createHarness({
		pathname: "/products/item%2042",
		body: '<section data-hk-page-content-ready="product-detail" data-hk-product-detail-id="item 41"></section>',
	});
	try {
		harness.run();
		harness.flushFrames();
		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			false,
		);

		const correct = harness.document.createElement("section");
		correct.setAttribute("data-hk-page-content-ready", "product-detail");
		correct.setAttribute("data-hk-product-detail-id", "item 42");
		harness.document.body.append(correct);
		await harness.deliverMutations();
		harness.flushFrames();
		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			true,
		);
	} finally {
		harness.dom.window.close();
	}
});

test("content present while the document is loading is rechecked on readiness", () => {
	const harness = createHarness({
		body: '<span data-hk-initial-content-ready="home" hidden></span>',
		readyState: "loading",
	});
	try {
		harness.run();
		harness.flushFrames();
		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			false,
		);

		harness.setReadyState("interactive");
		harness.document.dispatchEvent(
			new harness.window.Event("readystatechange"),
		);
		harness.flushFrames();
		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			true,
		);
	} finally {
		harness.dom.window.close();
	}
});

test("the public observer stops at the watchdog deadline and cannot release later", async () => {
	const harness = createHarness();
	try {
		harness.run();
		harness.flushFrames();
		assert.equal(harness.timers.size, 1);

		harness.fireTimersAt(INITIAL_LOADER_WATCHDOG_MS);
		assert.equal(harness.timers.size, 0);
		const marker = harness.document.createElement("span");
		marker.hidden = true;
		marker.setAttribute("data-hk-initial-content-ready", "home");
		harness.document.body.append(marker);
		await harness.deliverMutations();
		harness.document.dispatchEvent(
			new harness.window.Event("readystatechange"),
		);
		harness.flushFrames();

		assert.equal(
			harness.document.documentElement.classList.contains(
				INITIAL_PUBLIC_CONTENT_READY_CLASS,
			),
			false,
		);
		assert.equal(
			harness.document.documentElement.classList.contains(
				"initial-loader-active",
			),
			true,
		);
	} finally {
		harness.dom.window.close();
	}
});

test("an English URL takes the same public fast path as its zh-TW twin, and a protected one still does not", async (t) => {
	await t.test("/en is the home page", () => {
		const harness = createHarness({
			pathname: "/en",
			body: '<span data-hk-initial-content-ready="home" hidden></span>',
		});
		try {
			harness.run();
			harness.flushFrames();
			const classes = harness.document.documentElement.classList;
			assert.equal(
				classes.contains(INITIAL_PUBLIC_CONTENT_ENABLED_CLASS),
				true,
			);
			assert.equal(classes.contains(INITIAL_PUBLIC_CONTENT_READY_CLASS), true);
		} finally {
			harness.dom.window.close();
		}
	});

	await t.test(
		"/en/account/notifications is still behind the React gate",
		() => {
			const harness = createHarness({
				pathname: "/en/account/notifications",
				body: '<span data-hk-page-content-ready="notifications" hidden></span>',
			});
			try {
				harness.run();
				harness.flushFrames();
				const classes = harness.document.documentElement.classList;
				assert.equal(
					classes.contains(INITIAL_PUBLIC_CONTENT_ENABLED_CLASS),
					false,
				);
				assert.equal(classes.contains("initial-loader-active"), true);
			} finally {
				harness.dom.window.close();
			}
		},
	);
});
