import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

test("mounts ViewTransitions inside body around all shared Link consumers", () => {
	const layout = readFileSync(
		new URL("../components/shared/RootDocument.tsx", import.meta.url),
		"utf8",
	);
	const bodyStart = layout.indexOf("<body");
	const providerStart = layout.indexOf("<ViewTransitions>", bodyStart);
	const providers = layout.indexOf("<AppProviders", providerStart);
	const providerEnd = layout.indexOf("</ViewTransitions>", providers);
	const bodyEnd = layout.indexOf("</body>", providerEnd);

	assert.ok(bodyStart >= 0);
	assert.ok(providerStart > bodyStart);
	assert.ok(providers > providerStart);
	assert.ok(providerEnd > providers);
	assert.ok(bodyEnd > providerEnd);
	assert.equal(layout.indexOf("<ViewTransitions>"), providerStart);
});

test("shared Link keeps viewport prefetch disabled and preserves atomic data warming", () => {
	const [transitionLink, sharedLink] = [
		readFileSync(
			new URL("../components/shared/view-transitions.tsx", import.meta.url),
			"utf8",
		),
		readFileSync(
			new URL("../components/shared/Link.tsx", import.meta.url),
			"utf8",
		),
	];

	assert.match(transitionLink, /prefetch=\{false\}/);
	assert.match(transitionLink, /router\.prefetch\(target\)/);
	assert.match(transitionLink, /onFocus=\{\(event\) =>/);
	assert.match(transitionLink, /onMouseEnter=\{\(event\) =>/);
	assert.match(transitionLink, /onPointerDown=\{\(event\) =>/);
	assert.match(
		sharedLink,
		/props\.hardNavigation \|\|\s*\(targetRole && activeRole !== targetRole\)/,
	);
	assert.match(sharedLink, /navigationIntent !== latestNavigationIntent/);
	assert.match(sharedLink, /armProgrammaticRouteMotion\(href\)/);
	assert.match(
		sharedLink,
		/armProgrammaticRouteMotion\(href\);\s*if \(props\.replace\)/,
	);
});
