// @vitest-environment jsdom

import { afterEach, describe, expect, it } from "vitest";
import { setActiveLocale } from "@/lib/intl/active-locale";
import { resolveNavigationUrl, toBrowserHref } from "./resolve-url";

afterEach(() => setActiveLocale("zh-TW"));

describe("toBrowserHref / resolveNavigationUrl", () => {
	it("an app path resolves to the page's language", () => {
		setActiveLocale("en");
		expect(toBrowserHref("/account")).toBe("/en/account");
		expect(resolveNavigationUrl("/account?tab=1")?.pathname).toBe(
			"/en/account",
		);
		expect(resolveNavigationUrl("/account?tab=1")?.search).toBe("?tab=1");
		expect(resolveNavigationUrl("/")?.pathname).toBe("/en");
	});

	it("an already-prefixed path and an absolute URL stay as they are", () => {
		setActiveLocale("en");
		expect(resolveNavigationUrl("/en/account")?.pathname).toBe("/en/account");
		expect(resolveNavigationUrl("https://example.com/x")?.href).toBe(
			"https://example.com/x",
		);
	});

	it("the default language is unprefixed", () => {
		expect(toBrowserHref("/account")).toBe("/account");
		expect(resolveNavigationUrl("/account")?.pathname).toBe("/account");
	});

	it("is null for a target that is not a URL", () => {
		expect(resolveNavigationUrl("http://")).toBeNull();
	});
});
