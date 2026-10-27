import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { JSDOM } from "jsdom";
import {
	isQuickViewOpener,
	PRODUCT_QUICK_VIEW_PENDING_BOOT_SCRIPT,
	setQuickViewOpener,
} from "../lib/product/quickViewOrigin";

const realWindow = globalThis.window;

function installWindow(href: string) {
	Object.defineProperty(globalThis, "window", {
		configurable: true,
		value: { location: { href } },
	});
}

beforeEach(() => {
	installWindow("https://shop.test/market/products");
	setQuickViewOpener(null);
});

afterEach(() => {
	setQuickViewOpener(null);
	if (realWindow === undefined) Reflect.deleteProperty(globalThis, "window");
	else
		Object.defineProperty(globalThis, "window", {
			configurable: true,
			value: realWindow,
		});
});

test("no recorded opener means no page owns the quick-view scroll", () => {
	assert.equal(isQuickViewOpener("https://shop.test/market/products"), false);
	assert.equal(isQuickViewOpener("/market/products"), false);
});

test("only a return to the opener counts, search params included", () => {
	setQuickViewOpener("/market/products?category=fruit");

	assert.equal(isQuickViewOpener("/market/products?category=fruit"), true);
	assert.equal(
		isQuickViewOpener("https://shop.test/market/products?category=fruit"),
		true,
	);
	// A different filter is a different list state — it starts fresh.
	assert.equal(isQuickViewOpener("/market/products?category=leaf"), false);
	assert.equal(isQuickViewOpener("/market/products"), false);
});

test("navigating onwards out of the quick-view is never a return", () => {
	setQuickViewOpener("/market/products");

	// The seller storefront reached from inside the quick-view, plus the other
	// exits a product offers. None of them may inherit the list's scroll.
	for (const forward of ["/sellers/abc", "/cart", "/checkout", "/"]) {
		assert.equal(isQuickViewOpener(forward), false, forward);
	}
});

test("a malformed href cannot be mistaken for the opener", () => {
	setQuickViewOpener("/market/products");
	assert.equal(isQuickViewOpener("http://"), false);
});

test("the pre-hydration capture records the opener alongside the card geometry", () => {
	// The boot script runs before hydration, so a native click on a product card
	// must already carry the page it was launched from.
	assert.match(
		PRODUCT_QUICK_VIEW_PENDING_BOOT_SCRIPT,
		/openerUrl:location\.pathname\+location\.search/,
	);
});

test("the pre-hydration capture recognises a product link of any language and ignores the rest", () => {
	const click = (href: string) => {
		const dom = new JSDOM(
			`<!doctype html><body><a id="link" href="${href}">p</a></body>`,
			{
				pretendToBeVisual: true,
				runScripts: "outside-only",
				url: "https://shop.test/en/market/products",
			},
		);
		const { window } = dom;
		window.eval(PRODUCT_QUICK_VIEW_PENDING_BOOT_SCRIPT);
		// the capture listener has already run when this one stops jsdom from navigating
		window.document.addEventListener("click", (event) =>
			event.preventDefault(),
		);
		window.document.getElementById("link")?.dispatchEvent(
			new window.MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			}),
		);
		const controller = (
			window as unknown as {
				__hkProductQuickViewPending: {
					origin: { openerUrl: string } | null;
				};
			}
		).__hkProductQuickViewPending;
		const origin = controller.origin;
		window.close();
		return origin;
	};

	assert.notEqual(click("/products/9"), null);
	assert.equal(click("/en/products/9")?.openerUrl, "/en/market/products");
	assert.notEqual(click("/en/products/9/"), null);
	assert.equal(click("/en/sellers/9"), null);
	assert.equal(click("/products"), null);
});
