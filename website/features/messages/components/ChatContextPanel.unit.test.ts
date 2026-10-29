/** @vitest-environment jsdom */

import { create } from "@bufbuild/protobuf";
import {
	StoreCardSchema,
	StoreCardStatus,
} from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type AppLocale, messagesFor } from "@/lib/intl/config";

const mocks = vi.hoisted(() => ({ cards: new Map<string, unknown>() }));

vi.mock("@/lib/seller/fetchStoreCards", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/seller/fetchStoreCards")>()),
	fetchStoreCards: vi.fn(async () => mocks.cards),
}));
vi.mock("@/components/shared/Link", () => ({
	default: ({
		href,
		children,
		className,
	}: {
		href: string;
		children: React.ReactNode;
		className?: string;
	}) => React.createElement("a", { href, className }, children),
}));
vi.mock("@/lib/api", () => ({ fetchSellerProducts: vi.fn(async () => null) }));

import ChatContextPanel from "./ChatContextPanel";

function renderPanel(
	status: StoreCardStatus,
	locale: AppLocale = "zh-TW",
	over: Record<string, unknown> = {},
) {
	mocks.cards = new Map([
		[
			"7",
			create(StoreCardSchema, {
				id: "7",
				storeCode: "shop7",
				name: "小店",
				status,
				...over,
			}),
		],
	]);
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	const tree = React.createElement(
		QueryClientProvider,
		{ client },
		React.createElement(ChatContextPanel, { sellerId: "7" }),
	);
	return render(
		React.createElement(NextIntlClientProvider, {
			locale,
			messages: messagesFor(locale),
			children: tree,
		}),
	);
}

afterEach(cleanup);

describe("ChatContextPanel store states (B11)", () => {
	it("links an open store", async () => {
		renderPanel(StoreCardStatus.OPEN);
		const link = await screen.findByRole("link", { name: "前往賣場" });
		expect(link.getAttribute("href")).toBe("/sellers/shop7");
	});

	it("shows 暫停營業 without a link for a paused store", async () => {
		renderPanel(StoreCardStatus.PAUSED);
		expect(await screen.findByText("暫停營業")).toBeTruthy();
		expect(screen.queryByRole("link", { name: "前往賣場" })).toBeNull();
	});

	it("shows 已關閉 without a link for a closed store", async () => {
		renderPanel(StoreCardStatus.CLOSED);
		expect(await screen.findByText("已關閉")).toBeTruthy();
		expect(screen.queryByRole("link", { name: "前往賣場" })).toBeNull();
	});

	it("names a closed store without a name 已關閉的店家, in English too", async () => {
		renderPanel(StoreCardStatus.CLOSED, "zh-TW", { name: undefined });
		expect(await screen.findByText("已關閉的店家")).toBeTruthy();
		cleanup();
		renderPanel(StoreCardStatus.PAUSED, "en");
		expect(await screen.findByText("Temporarily closed")).toBeTruthy();
	});
});
