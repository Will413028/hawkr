// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { KYCApplicationSchema } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";
import { queryKeys } from "@/lib/query/keys";

const identity = { accountId: "1", activeRole: "seller" } as const;

const rpc = vi.hoisted(() => ({ refresh: vi.fn() }));
vi.mock("@/lib/seller/onboarding-summary", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/seller/onboarding-summary")>()),
	refreshMyPaymentAccount: rpc.refresh,
}));
vi.mock("./useQueryIdentity", () => ({ useQueryIdentity: () => identity }));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ hasSeller: true, status: "authenticated" }),
}));
vi.mock("@/lib/seller-data/scope", () => ({
	useSellerDataScope: () => ({
		mode: "seller",
		keys: { storefront: () => ["seller", "storefront"] },
		repos: {},
		canRead: () => false,
	}),
}));

import {
	useInvalidateSellerSelf,
	useRefreshPaymentAccount,
	useSetMyKyc,
} from "./useSellerSelf";

function setup() {
	const client = new QueryClient();
	client.setQueryData(queryKeys.seller.onboarding(identity), { tasks: [] });
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(QueryClientProvider, { client }, children);
	const isStale = () =>
		client.getQueryState(queryKeys.seller.onboarding(identity))?.isInvalidated;
	return { wrapper, isStale };
}

describe("開通摘要在申請與店家資料改動後失效", () => {
	it("useInvalidateSellerSelf 讓開通摘要失效", async () => {
		const { wrapper, isStale } = setup();
		const { result } = renderHook(() => useInvalidateSellerSelf(), { wrapper });
		expect(isStale()).toBe(false);
		await result.current();
		expect(isStale()).toBe(true);
	});

	it("useSetMyKyc 寫入申請後讓開通摘要失效", () => {
		const { wrapper, isStale } = setup();
		const { result } = renderHook(() => useSetMyKyc(), { wrapper });
		expect(isStale()).toBe(false);
		result.current(create(KYCApplicationSchema, {}));
		expect(isStale()).toBe(true);
	});
});

describe("useRefreshPaymentAccount", () => {
	it("重讀成功：重新讀開通摘要與收款帳戶", async () => {
		rpc.refresh.mockResolvedValue(undefined);
		const { wrapper, isStale } = setup();
		const { result } = renderHook(() => useRefreshPaymentAccount(), {
			wrapper,
		});
		await result.current();
		expect(rpc.refresh).toHaveBeenCalledTimes(1);
		expect(isStale()).toBe(true);
	});

	it("重讀失敗：不丟出，仍讓開通摘要失效（快取不留舊的）", async () => {
		rpc.refresh.mockRejectedValue(new Error("refresh boom"));
		const { wrapper, isStale } = setup();
		const { result } = renderHook(() => useRefreshPaymentAccount(), {
			wrapper,
		});
		await expect(result.current()).resolves.toBeUndefined();
		expect(isStale()).toBe(true);
	});
});
