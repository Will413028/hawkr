import assert from "node:assert/strict";
import { after, before, beforeEach, test } from "node:test";
import {
	armProgrammaticRouteMotion,
	cancelProgrammaticRouteMotion,
	consumeProgrammaticRouteMotion,
	resetProgrammaticRouteMotionForTests,
} from "../lib/navigation/programmatic-route-motion";

const originalWindow = globalThis.window;
const fakeWindow = Object.assign(new EventTarget(), {
	location: new URL("https://example.com/about"),
});

function setLocation(path: string) {
	fakeWindow.location = new URL(path, "https://example.com");
}

before(() => {
	Object.defineProperty(globalThis, "window", {
		configurable: true,
		value: fakeWindow,
	});
});

beforeEach(() => {
	setLocation("/about");
	resetProgrammaticRouteMotionForTests();
});

after(() => {
	if (originalWindow === undefined) {
		Reflect.deleteProperty(globalThis, "window");
	} else {
		Object.defineProperty(globalThis, "window", {
			configurable: true,
			value: originalWindow,
		});
	}
});

test("an ordinary intent is consumed exactly once by its destination commit", () => {
	assert.equal(armProgrammaticRouteMotion("/cart"), 1);
	assert.equal(consumeProgrammaticRouteMotion("/cart"), true);
	assert.equal(consumeProgrammaticRouteMotion("/cart"), false);
});

test("same-path, app-tab, shell and cross-origin targets fail closed", () => {
	assert.equal(armProgrammaticRouteMotion("/about?section=team"), null);
	setLocation("/");
	assert.equal(armProgrammaticRouteMotion("/feed"), null);
	setLocation("/account");
	assert.equal(armProgrammaticRouteMotion("/account/orders"), null);
	assert.equal(
		armProgrammaticRouteMotion("https://example.org/elsewhere"),
		null,
	);
});

test("superseded, mismatched and cancelled intents never animate another page", () => {
	const first = armProgrammaticRouteMotion("/cart");
	assert.equal(first, 1);
	const second = armProgrammaticRouteMotion("/checkout");
	assert.equal(second, 2);
	cancelProgrammaticRouteMotion(first);
	assert.equal(consumeProgrammaticRouteMotion("/cart"), false);
	assert.equal(consumeProgrammaticRouteMotion("/checkout"), false);

	const third = armProgrammaticRouteMotion("/orders");
	cancelProgrammaticRouteMotion(third);
	assert.equal(consumeProgrammaticRouteMotion("/orders"), false);
});

test("expired and page-hidden intents are cleared", () => {
	const originalNow = Date.now;
	let now = 1000;
	Date.now = () => now;
	try {
		armProgrammaticRouteMotion("/cart");
		now = 6000;
		assert.equal(consumeProgrammaticRouteMotion("/cart"), false);

		armProgrammaticRouteMotion("/checkout");
		fakeWindow.dispatchEvent(new Event("pagehide"));
		assert.equal(consumeProgrammaticRouteMotion("/checkout"), false);
	} finally {
		Date.now = originalNow;
	}
});
