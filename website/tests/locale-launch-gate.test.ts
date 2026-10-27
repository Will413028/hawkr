import assert from "node:assert/strict";
import test from "node:test";
import {
	defaultLocale,
	LAUNCH_BLOCKERS,
	publishedLocales,
	supportedLocales,
} from "../lib/intl/locales";

const surfaces = Object.keys(publishedLocales) as Array<
	keyof typeof publishedLocales
>;

test("a locale other than the default is published only after every launch blocker is cleared", () => {
	for (const surface of surfaces) {
		const added = publishedLocales[surface].filter(
			(locale) => locale !== defaultLocale,
		);
		if (added.length === 0) continue;
		assert.deepEqual(
			LAUNCH_BLOCKERS,
			[],
			`${surface} publishes ${added.join(", ")} while lib/intl/locales.ts LAUNCH_BLOCKERS is not empty: finish them first (plan 2026-09-29-a3-buyer-seller.md, I-web-0 "發佈 en 之前").`,
		);
	}
});

test("the default locale is always published and only supported locales are listed", () => {
	for (const surface of surfaces) {
		assert.ok(publishedLocales[surface].includes(defaultLocale), surface);
		for (const locale of publishedLocales[surface]) {
			assert.ok(supportedLocales.includes(locale), `${surface}: ${locale}`);
		}
	}
});

test("every launch blocker says what is missing", () => {
	for (const blocker of LAUNCH_BLOCKERS) {
		assert.ok(blocker.trim().length > 10, blocker);
	}
});
