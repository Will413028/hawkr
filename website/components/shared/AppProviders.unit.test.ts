// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { activeLocale, setActiveLocale } from "@/lib/intl/active-locale";
import { useRoutedLocale } from "@/lib/intl/routed-locale";

const mocks = vi.hoisted(() => ({ browserTransport: vi.fn(() => ({})) }));

vi.mock("@/lib/connect/transport", () => ({
	browserTransport: mocks.browserTransport,
}));
vi.mock("@/components/shared/AppOverlayHost", () => ({ default: () => null }));
vi.mock("@/components/shared/AppToaster", () => ({ default: () => null }));
vi.mock("@/components/shared/SiteGrainientBackground", () => ({
	default: () => null,
}));
vi.mock("next-themes", () => ({
	ThemeProvider: ({ children }: { children: React.ReactNode }) => children,
}));
vi.mock("nuqs/adapters/next/app", () => ({
	NuqsAdapter: ({ children }: { children: React.ReactNode }) => children,
}));

import AppProviders from "./AppProviders";

afterEach(() => {
	cleanup();
	setActiveLocale("zh-TW");
	mocks.browserTransport.mockClear();
});

function Probe({ seen }: { seen: { routed?: string; active?: string } }) {
	seen.routed = useRoutedLocale();
	seen.active = activeLocale();
	return null;
}

function mount(locale: string) {
	const seen: { routed?: string; active?: string } = {};
	render(
		React.createElement(AppProviders, {
			locale,
			messages: {},
			children: React.createElement(Probe, { seen }),
		}),
	);
	return seen;
}

describe("AppProviders and the page's language", () => {
	it("an English route sets the language of the tree, of the browser-side code outside React and of the Connect transport", () => {
		const seen = mount("en");
		expect(seen).toEqual({ routed: "en", active: "en" });
		expect(mocks.browserTransport).toHaveBeenCalledWith({ locale: "en" });
	});

	it("the language is the route's, not the document's: a translated <html lang> changes nothing", () => {
		document.documentElement.lang = "zh-TW";
		expect(mount("en").active).toBe("en");
		document.documentElement.lang = "";
	});

	it("an unknown route value falls back to the default language", () => {
		const seen = mount("fr");
		expect(seen).toEqual({ routed: "zh-TW", active: "zh-TW" });
		expect(mocks.browserTransport).toHaveBeenCalledWith({ locale: "zh-TW" });
	});
});
