/** @vitest-environment jsdom */

import { SellerCapability } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { type QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { HttpResponse, http } from "msw";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import { createAppQueryClient } from "@/lib/query/client";
import { server } from "@/tests/mocks/server";
import { useSellerInvoiceOverview } from "./useSellerInvoices";

vi.mock("@sentry/nextjs", () => ({
	captureException: vi.fn(),
	addBreadcrumb: vi.fn(),
}));
vi.mock("@/lib/config/env", () => ({
	getEnv: () => ({
		apiBaseUrl: "https://invoice.test/api/v1",
		apiTimeoutMs: 15_000,
	}),
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		isLoggedIn: true,
		activeRole: "seller",
		identity: { accountId: "seller-test" },
	}),
}));
const sellerSelf = vi.hoisted(() => ({
	current: null as {
		capabilities: { capability: number; active: boolean; missing: number[] }[];
	} | null,
}));
vi.mock("./useSellerSelf", () => ({
	useSellerSelf: () => ({ data: sellerSelf.current }),
}));

const HEALTHY = [
	SellerCapability.ACCOUNT_SELF_READ,
	SellerCapability.ORDER_FULFILL,
	SellerCapability.INVOICE_MANAGE,
];
// 停權或等待刪除：沒有 INVOICE_MANAGE。
const RESTRICTED = [
	SellerCapability.ACCOUNT_SELF_READ,
	SellerCapability.ORDER_FULFILL,
];

const active = (capabilities: SellerCapability[]) =>
	capabilities.map((capability) => ({ capability, active: true, missing: [] }));

const endpoint = "https://invoice.test/api/v1/app/seller/invoices/overview";
const overview = {
	taxReviewStatus: "APPROVED",
	invoiceMode: "SELF_ISSUE",
	providerStatus: "INACTIVE",
	pendingDocumentCount: "3",
	openComplaintCount: "0",
	overdueComplaintCount: "0",
	deadlineAt: "",
	extensionCount: "0",
	canRequestExtension: false,
	selfIssueEffective: true,
	intakeLimit: {
		orderCountUsed: "0",
		orderCountLimit: "",
		amountUsed: "0",
		amountLimit: "",
		blocked: false,
	},
	recentDocuments: [],
};
const clients: QueryClient[] = [];

function unavailable() {
	return HttpResponse.json(
		{
			code: 503,
			message: "invoice service unavailable",
			traceId: "test-trace",
		},
		{ status: 503 },
	);
}

function mountOverview() {
	// Exercise the real app retry policy, hook, repository and HTTP transport.
	// Only shorten retry delays so the regression settles without real waiting.
	const client = createAppQueryClient();
	client.setDefaultOptions({
		...client.getDefaultOptions(),
		queries: { ...client.getDefaultOptions().queries, retryDelay: 0 },
	});
	clients.push(client);
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
	return renderHook(() => useSellerInvoiceOverview(), { wrapper });
}

beforeEach(() => {
	vi.clearAllMocks();
	sellerSelf.current = { capabilities: active(HEALTHY) };
	Object.assign(window, {
		Clerk: { loaded: true, session: { getToken: async () => "test-token" } },
	});
});

afterEach(() => {
	cleanup();
	for (const client of clients.splice(0)) client.clear();
	delete (window as Window & { Clerk?: unknown }).Clerk;
});

describe("useSellerInvoiceOverview retry ownership", () => {
	it("limits an outage to the transport retry, exposes the error, and allows manual recovery", async () => {
		const handler = vi.fn<() => Response>(unavailable);
		server.use(http.get(endpoint, handler));
		const { result } = mountOverview();

		await waitFor(() =>
			expect(result.current.error).toBeInstanceOf(ApiClientError),
		);
		expect(handler).toHaveBeenCalledTimes(2);
		expect(result.current.error).toMatchObject({
			status: 503,
			payload: { traceId: "test-trace" },
		});
		expect(result.current.overview).toBeNull();
		expect(result.current.loading).toBe(false);

		handler.mockImplementation(() =>
			HttpResponse.json({ code: 0, data: overview }),
		);
		await act(async () => {
			await result.current.refresh();
		});
		await waitFor(() => expect(result.current.overview).toEqual(overview));
		expect(result.current.error).toBeNull();
		expect(handler).toHaveBeenCalledTimes(3);
	});

	it("still recovers from a transient 503 through the existing transport retry", async () => {
		const handler = vi
			.fn<() => Response>(() => HttpResponse.json({ code: 0, data: overview }))
			.mockImplementationOnce(unavailable);
		server.use(http.get(endpoint, handler));
		const { result } = mountOverview();

		await waitFor(() => expect(result.current.overview).toEqual(overview));
		expect(handler).toHaveBeenCalledTimes(2);
		expect(result.current.error).toBeNull();
	});
});

describe("BLOCK queries follow the store's capabilities", () => {
	it("does not call the invoice endpoint once the store lost INVOICE_MANAGE", async () => {
		sellerSelf.current = { capabilities: active(RESTRICTED) };
		const handler = vi.fn<() => Response>(() =>
			HttpResponse.json({ code: 0, data: overview }),
		);
		server.use(http.get(endpoint, handler));
		const { result } = mountOverview();

		await waitFor(() => expect(result.current.blockedBySuspension).toBe(true));
		expect(result.current.overview).toBeNull();
		expect(result.current.loading).toBe(false);
		expect(handler).not.toHaveBeenCalled();
	});

	it("does not treat a store that has not been read yet as blocked", async () => {
		sellerSelf.current = null;
		const handler = vi.fn<() => Response>(() =>
			HttpResponse.json({ code: 0, data: overview }),
		);
		server.use(http.get(endpoint, handler));
		const { result } = mountOverview();

		await waitFor(() => expect(result.current.overview).toEqual(overview));
		expect(result.current.blockedBySuspension).toBe(false);
	});
});
