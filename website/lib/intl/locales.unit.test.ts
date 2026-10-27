import { afterEach, describe, expect, it } from "vitest";
import {
	__setPublishedLocalesForTest,
	allRoutableLocales,
	defaultLocale,
	isAppLocale,
	publishedFor,
	publishedLocales,
	resolveRoutableLocale,
	routableLocales,
	supportedLocales,
} from "./locales";

afterEach(() => __setPublishedLocalesForTest(null));

describe("published locales", () => {
	it("every surface starts with zh-TW only (ADR 2026-10-17: en is added when the surface has no untranslated text)", () => {
		expect(publishedLocales).toEqual({ buyer: ["zh-TW"], seller: ["zh-TW"] });
	});

	it("the default locale is always published, and only supported locales are listed", () => {
		for (const surface of ["buyer", "seller"] as const) {
			const list = publishedLocales[surface];
			expect(list, surface).toContain(defaultLocale);
			for (const locale of list)
				expect(supportedLocales, surface).toContain(locale);
		}
	});

	it("a test can swap the lists and put them back", () => {
		__setPublishedLocalesForTest({ buyer: ["zh-TW", "en"], seller: ["zh-TW"] });
		expect(publishedFor("buyer")).toEqual(["zh-TW", "en"]);
		expect(publishedFor("seller")).toEqual(["zh-TW"]);
		__setPublishedLocalesForTest(null);
		expect(publishedFor("buyer")).toEqual(["zh-TW"]);
	});
});

describe("routable locales", () => {
	it("prod routes only the published locales: /en is a 404", () => {
		expect(routableLocales("buyer", "prod")).toEqual(["zh-TW"]);
		expect(routableLocales("seller", "prod")).toEqual(["zh-TW"]);
		expect(allRoutableLocales("prod")).toEqual(["zh-TW"]);
		expect(resolveRoutableLocale("en", "prod")).toBeNull();
		expect(resolveRoutableLocale("zh-TW", "prod")).toBe("zh-TW");
	});

	it("dev routes every supported locale so an unpublished locale can be tested", () => {
		expect(routableLocales("buyer", "dev")).toEqual(["zh-TW", "en"]);
		expect(allRoutableLocales("dev")).toEqual(["zh-TW", "en"]);
		expect(resolveRoutableLocale("en", "dev")).toBe("en");
	});

	it("prod follows the published lists per surface; the router knows the union", () => {
		__setPublishedLocalesForTest({ buyer: ["zh-TW", "en"], seller: ["zh-TW"] });
		expect(routableLocales("buyer", "prod")).toEqual(["zh-TW", "en"]);
		expect(routableLocales("seller", "prod")).toEqual(["zh-TW"]);
		expect(allRoutableLocales("prod")).toEqual(["zh-TW", "en"]);
		expect(resolveRoutableLocale("en", "prod")).toBe("en");
	});

	it("only exact supported tags are locales: no case folding, no region guessing", () => {
		for (const raw of [
			"zh-tw",
			"zh",
			"EN",
			"en-US",
			"zh-Hant",
			"",
			undefined,
			null,
			1,
		]) {
			expect(isAppLocale(raw), String(raw)).toBe(false);
			expect(resolveRoutableLocale(raw, "dev"), String(raw)).toBeNull();
		}
	});
});
