import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const projectFile = (path: string) => new URL(`../${path}`, import.meta.url);

test("desktop and mobile primary navigation share the intent coordinator", async () => {
	const [navbar, megaMenu, mobile] = await Promise.all([
		readFile(projectFile("components/shared/Navbar.tsx"), "utf8"),
		readFile(projectFile("components/shared/MegaMenu.tsx"), "utf8"),
		readFile(projectFile("components/shared/MobileBottomNav.tsx"), "utf8"),
	]);

	for (const source of [navbar, megaMenu, mobile]) {
		assert.match(source, /prefetchMainNavOnIntent/);
	}
	assert.match(navbar, /prefetch=\{false\}/);
	assert.match(megaMenu, /prefetch=\{false\}/);
	assert.match(
		mobile,
		/prefetch=\{index < baseItems\.length \? false : undefined\}/,
	);
	assert.match(navbar, /href="\/"[\s\S]*?prefetch=\{false\}/);
	assert.match(megaMenu, /cancelMainNavPointerIntent/);
	const idleWarmCalls = [
		...mobile.matchAll(/scheduleMainNavIdleWarm\(\{([\s\S]*?)\}\)/g),
	];
	assert.equal(idleWarmCalls.length, 1, "the global shell owns one idle batch");
	assert.match(idleWarmCalls[0][1], /\brouter\s*(?:,|:\s*router\s*,)/);
	assert.match(idleWarmCalls[0][1], /currentPathname:\s*pathname\b/);
	for (const source of [navbar, megaMenu]) {
		assert.doesNotMatch(
			source,
			/scheduleMainNavIdleWarm/,
			"idle warming must have one global owner across buyer and seller chrome",
		);
	}
});

test("guest account navigation keeps the hard auth boundary", async () => {
	const mobile = await readFile(
		projectFile("components/shared/MobileBottomNav.tsx"),
		"utf8",
	);

	assert.match(
		mobile,
		/hardNavigation=\{!isLoggedIn && item\.icon === "login"\}/,
	);
});
