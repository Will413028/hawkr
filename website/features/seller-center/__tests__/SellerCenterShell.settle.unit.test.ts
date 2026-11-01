// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { NuqsTestingAdapter } from "nuqs/adapters/testing";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const router = vi.hoisted(() => ({
	push: vi.fn(),
	replace: vi.fn(),
	refresh: vi.fn(),
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => router,
}));
vi.mock("@/lib/intl/navigation", () => ({
	usePathname: () => "/seller-center/orders",
	useRouter: () => router,
}));
vi.mock("next/navigation", () => ({
	useSearchParams: () => new URLSearchParams(),
	usePathname: () => "/seller-center/orders",
	useRouter: () => router,
}));
vi.mock("@/components/shared/Link", () => ({
	default: (props: { children?: React.ReactNode }) =>
		React.createElement("a", null, props.children),
}));
vi.mock("@sentry/nextjs", () => ({ captureMessage: vi.fn() }));
vi.mock("@/lib/navigation/document-location", () => ({
	commitDocumentNavigation: vi.fn(),
}));
vi.mock("@/lib/seller/repo", async (importOriginal) => {
	const original = await importOriginal<typeof import("@/lib/seller/repo")>();
	return {
		...original,
		sellerStorefrontRepo: {
			...original.sellerStorefrontRepo,
			fetchMyStore: async () => ({
				store: { id: "9", storeName: "S", onboardingCompleted: true },
				capabilities: [],
			}),
		},
	};
});

import WorkspaceAlign from "@/components/shared/WorkspaceAlign";
import { SellerCenterShell } from "@/features/seller-center";
import { SellerOrdersPage } from "@/features/seller-center/orders";
import { AuthProvider } from "@/lib/auth/context";
import type { SessionState } from "@/lib/auth/session-state";
import { messagesFor } from "@/lib/intl/config";

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

const h = React.createElement as unknown as (
	type: unknown,
	props: unknown,
	...children: unknown[]
) => React.ReactElement;

function memoryStorage(seed: Record<string, string>) {
	const store = new Map(Object.entries(seed));
	return {
		getItem: (key: string) => store.get(key) ?? null,
		setItem: (key: string, value: string) => void store.set(key, value),
		removeItem: (key: string) => void store.delete(key),
	};
}

function Lifecycle({ events }: { events: string[] }) {
	React.useEffect(() => {
		events.push("mount");
		return () => void events.push("unmount");
	}, [events]);
	return React.createElement(SellerOrdersPage);
}

describe("seller center while the server's session answer is pending", () => {
	it("mounts the page once, after the answer, instead of mounting and tearing it down", async () => {
		vi.stubGlobal(
			"localStorage",
			memoryStorage({ "hkui:auth:active-role": "seller" }),
		);
		vi.stubGlobal("sessionStorage", memoryStorage({}));
		const events: string[] = [];
		let answer!: (state: SessionState) => void;
		const session = new Promise<SessionState>((resolve) => {
			answer = resolve;
		});
		const { container } = render(
			h(
				QueryClientProvider,
				{ client: new QueryClient() },
				h(
					NextIntlClientProvider,
					{ locale: "zh-TW", messages: messagesFor("zh-TW") },
					h(
						NuqsTestingAdapter,
						null,
						h(
							AuthProvider,
							{ session },
							h(
								WorkspaceAlign,
								{ required: "seller", workspace: "seller" },
								h(SellerCenterShell, null, h(Lifecycle, { events })),
							),
						),
					),
				),
			),
		);
		await act(async () => {
			await new Promise((done) => setTimeout(done, 50));
		});
		expect(container.querySelector(".seller-orders")).toBeNull();
		expect(events).toEqual([]);

		await act(async () => {
			answer({
				status: "signedIn",
				identity: { accountId: "1", isSeller: true },
			});
		});
		await waitFor(() =>
			expect(container.querySelector(".seller-orders")).not.toBeNull(),
		);
		await act(async () => {
			await new Promise((done) => setTimeout(done, 300));
		});
		expect(events).toEqual(["mount"]);
		expect(container.querySelector(".seller-orders")).not.toBeNull();
	});
});
