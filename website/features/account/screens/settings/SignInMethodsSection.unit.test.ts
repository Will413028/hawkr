// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	cleanup,
	render,
	screen,
	waitFor,
	within,
} from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	search: "",
	fetchLinkedProviders: vi.fn(),
	locale: "zh-TW" as "zh-TW" | "en",
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useSearchParams: () => new URLSearchParams(mocks.search),
}));
vi.mock("@/lib/api/auth", () => ({
	fetchLinkedProviders: mocks.fetchLinkedProviders,
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		hydrated: true,
		isLoggedIn: true,
		identity: { accountId: "7", isSeller: false },
		activeRole: "buyer",
	}),
}));

import { messagesFor } from "@/lib/intl/config";
import { RoutedLocaleProvider } from "@/lib/intl/routed-locale";
import SignInMethodsSection from "./SignInMethodsSection";

function renderSection() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	const section = React.createElement(
		RoutedLocaleProvider,
		{ value: mocks.locale },
		React.createElement(SignInMethodsSection),
	);
	return render(
		React.createElement(QueryClientProvider, {
			client,
			children: React.createElement(NextIntlClientProvider, {
				locale: mocks.locale,
				messages: messagesFor(mocks.locale),
				children: section,
			}),
		}),
	);
}

const row = (name: string) =>
	screen.getByText(name).closest("li") as HTMLElement;

beforeEach(() => {
	mocks.search = "";
	mocks.locale = "zh-TW";
	mocks.fetchLinkedProviders.mockReset();
});
afterEach(cleanup);

describe("SignInMethodsSection", () => {
	it("Google linked, LINE not: one 已連結, one 連結 link", async () => {
		mocks.fetchLinkedProviders.mockResolvedValue(["google"]);
		renderSection();
		await waitFor(() =>
			expect(within(row("Google")).getByText("已連結")).toBeTruthy(),
		);
		expect(within(row("Google")).queryByRole("link")).toBeNull();
		expect(within(row("LINE")).queryByText("已連結")).toBeNull();
		expect(
			within(row("LINE")).getByRole("link", { name: "連結" }),
		).toBeTruthy();
		expect(screen.getAllByText("已連結")).toHaveLength(1);
	});

	it("the link goes to the start URL with link=1 and the encoded, localized return_to", async () => {
		mocks.fetchLinkedProviders.mockResolvedValue(["google"]);
		renderSection();
		const a = await within(row("LINE")).findByRole("link", { name: "連結" });
		const href = a.getAttribute("href") as string;
		expect(href.startsWith("/api/auth/line/start?")).toBe(true);
		const q = new URL(href, "http://x").searchParams;
		expect(q.get("link")).toBe("1");
		expect(q.get("return_to")).toBe("/account/settings");
		expect(href).toContain("return_to=%2Faccount%2Fsettings");
	});

	it("en: labels and return_to carry the locale prefix", async () => {
		mocks.locale = "en";
		mocks.fetchLinkedProviders.mockResolvedValue([]);
		renderSection();
		expect(
			screen.getByRole("heading", { name: "Sign-in methods" }),
		).toBeTruthy();
		const a = await within(row("Google")).findByRole("link", { name: "Link" });
		expect(
			new URL(a.getAttribute("href") as string, "http://x").searchParams.get(
				"return_to",
			),
		).toBe("/en/account/settings");
	});

	it("link_error=identity_already_linked shows its sentence", async () => {
		mocks.search = "link_error=identity_already_linked";
		mocks.fetchLinkedProviders.mockResolvedValue([]);
		renderSection();
		expect(screen.getByRole("alert").textContent).toBe(
			"這個登入帳號已連結到別的帳號",
		);
		await screen.findAllByRole("link");
	});

	it("link_error=access_denied reuses the login sentence; unknown shows nothing", async () => {
		mocks.search = "link_error=access_denied";
		mocks.fetchLinkedProviders.mockResolvedValue([]);
		const first = renderSection();
		expect(screen.getByRole("alert").textContent).toBe(
			"你取消了授權，請再試一次",
		);
		await screen.findAllByRole("link");
		first.unmount();
		mocks.search = "link_error=whatever";
		renderSection();
		await screen.findAllByRole("link");
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("RPC failure: no button and no 已連結", async () => {
		mocks.fetchLinkedProviders.mockRejectedValue(new Error("boom"));
		renderSection();
		await waitFor(() =>
			expect(screen.getAllByText("發生錯誤，請稍後再試").length).toBe(2),
		);
		expect(screen.queryByRole("link")).toBeNull();
		expect(screen.queryByText("已連結")).toBeNull();
	});

	it("while loading: no button and no 已連結", () => {
		mocks.fetchLinkedProviders.mockReturnValue(new Promise(() => {}));
		renderSection();
		expect(screen.queryByRole("link")).toBeNull();
		expect(screen.queryByText("已連結")).toBeNull();
	});
});
