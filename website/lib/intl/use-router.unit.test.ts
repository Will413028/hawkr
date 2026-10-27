// @vitest-environment jsdom

import { renderHook } from "@testing-library/react";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";
import type { AppLocale } from "./locales";
import { RoutedLocaleProvider } from "./routed-locale";

const nextRouter = vi.hoisted(() => ({
	push: vi.fn(),
	replace: vi.fn(),
	prefetch: vi.fn(),
	back: vi.fn(),
	forward: vi.fn(),
	refresh: vi.fn(),
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useRouter: () => nextRouter,
}));

import { useLocalizedHref, useRouter } from "./use-router";

function inLocale(locale: AppLocale) {
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(RoutedLocaleProvider, { value: locale }, children);
}

describe("useRouter (the locale layer's)", () => {
	it("an English page navigates to the English URL", () => {
		const { result } = renderHook(() => useRouter(), {
			wrapper: inLocale("en"),
		});
		result.current.push("/account/orders?tab=1", { scroll: false });
		result.current.replace("/cart");
		result.current.prefetch("/market/products");
		expect(nextRouter.push).toHaveBeenLastCalledWith(
			"/en/account/orders?tab=1",
			{ scroll: false },
		);
		expect(nextRouter.replace).toHaveBeenLastCalledWith("/en/cart");
		expect(nextRouter.prefetch).toHaveBeenLastCalledWith("/en/market/products");
	});

	it("the default language has no prefix", () => {
		const { result } = renderHook(() => useRouter(), {
			wrapper: inLocale("zh-TW"),
		});
		result.current.push("/account/orders");
		expect(nextRouter.push).toHaveBeenLastCalledWith("/account/orders");
	});

	it("can be applied twice, and leaves external and non-page targets alone", () => {
		const { result } = renderHook(() => useRouter(), {
			wrapper: inLocale("en"),
		});
		result.current.push("/en/account");
		expect(nextRouter.push).toHaveBeenLastCalledWith("/en/account");
		result.current.push("https://example.com/pay");
		expect(nextRouter.push).toHaveBeenLastCalledWith("https://example.com/pay");
		result.current.push("/api/v1/export.csv");
		expect(nextRouter.push).toHaveBeenLastCalledWith("/api/v1/export.csv");
	});

	it("passes through the methods that are not navigations", () => {
		const { result } = renderHook(() => useRouter(), {
			wrapper: inLocale("en"),
		});
		result.current.back();
		result.current.forward();
		result.current.refresh();
		expect(nextRouter.back).toHaveBeenCalledOnce();
		expect(nextRouter.forward).toHaveBeenCalledOnce();
		expect(nextRouter.refresh).toHaveBeenCalledOnce();
	});

	it("keeps its identity across renders, so effects that depend on the router do not re-run", () => {
		const { result, rerender } = renderHook(() => useRouter(), {
			wrapper: inLocale("en"),
		});
		const first = result.current;
		rerender();
		expect(result.current).toBe(first);
	});
});

describe("useLocalizedHref", () => {
	it("gives a form action the page's language", () => {
		const english = renderHook(() => useLocalizedHref("/search"), {
			wrapper: inLocale("en"),
		});
		const chinese = renderHook(() => useLocalizedHref("/search"), {
			wrapper: inLocale("zh-TW"),
		});
		expect(english.result.current).toBe("/en/search");
		expect(chinese.result.current).toBe("/search");
	});

	it("falls back to the default language without a provider (isolated renders)", () => {
		const { result } = renderHook(() => useLocalizedHref("/search"));
		expect(result.current).toBe("/search");
	});
});
