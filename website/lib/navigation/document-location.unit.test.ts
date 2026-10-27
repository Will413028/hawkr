// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from "vitest";
import { setActiveLocale } from "@/lib/intl/active-locale";
import { commitDocumentNavigation } from "./document-location";

const assign = vi.fn();
const replace = vi.fn();

function stubLocation() {
	vi.stubGlobal("location", { ...window.location, assign, replace });
}

afterEach(() => {
	vi.unstubAllGlobals();
	assign.mockClear();
	replace.mockClear();
	setActiveLocale("zh-TW");
});

describe("commitDocumentNavigation", () => {
	it("a full-page navigation stays in the page's language", () => {
		stubLocation();
		setActiveLocale("en");
		commitDocumentNavigation("/");
		commitDocumentNavigation("/login?role=seller&redirect=%2Fseller-center");
		expect(assign).toHaveBeenNthCalledWith(1, "/en");
		expect(assign).toHaveBeenNthCalledWith(
			2,
			"/en/login?role=seller&redirect=%2Fseller-center",
		);
	});

	it("replace goes through location.replace", () => {
		stubLocation();
		setActiveLocale("en");
		commitDocumentNavigation("/seller/onboarding", true);
		expect(replace).toHaveBeenCalledExactlyOnceWith("/en/seller/onboarding");
		expect(assign).not.toHaveBeenCalled();
	});

	it("the default language has no prefix", () => {
		stubLocation();
		commitDocumentNavigation("/");
		expect(assign).toHaveBeenCalledExactlyOnceWith("/");
	});

	it("an absolute URL (a payment gateway, another origin), an already-prefixed path and an API path are not touched", () => {
		stubLocation();
		setActiveLocale("en");
		commitDocumentNavigation("https://pay.example.com/checkout?id=1");
		commitDocumentNavigation("/en/account");
		commitDocumentNavigation("/zh-TW/account");
		commitDocumentNavigation("/api/v1/file.pdf");
		expect(assign.mock.calls.map((call) => call[0])).toEqual([
			"https://pay.example.com/checkout?id=1",
			"/en/account",
			"/account",
			"/api/v1/file.pdf",
		]);
	});
});
