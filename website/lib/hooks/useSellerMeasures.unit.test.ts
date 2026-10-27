// @vitest-environment jsdom

import { MeasureState } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "@/lib/query/keys";
import type { SellerMeasure } from "@/lib/seller/measures";

const mocks = vi.hoisted(() => ({
	auth: { hydrated: true, isLoggedIn: true, hasSeller: false },
	fetchMyMeasures: vi.fn(),
	fetchHasMeasures: vi.fn(),
	fetchMyMeasure: vi.fn(),
	submitMeasureAppeal: vi.fn(),
	submitMeasureRemediation: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({ useAuth: () => mocks.auth }));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "acc-1", activeRole: "buyer" }),
}));
vi.mock("@/lib/seller/measures", () => ({
	fetchMyMeasures: mocks.fetchMyMeasures,
	fetchHasMeasures: mocks.fetchHasMeasures,
	fetchMyMeasure: mocks.fetchMyMeasure,
	submitMeasureAppeal: mocks.submitMeasureAppeal,
	submitMeasureRemediation: mocks.submitMeasureRemediation,
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import {
	useHasMeasures,
	useSellerMeasure,
	useSellerMeasures,
	useSubmitSellerAppeal,
	useSubmitSellerRemediation,
} from "./useSellerMeasures";

const identity = { accountId: "acc-1", activeRole: "buyer" } as const;
const listKey = () =>
	queryKeys.seller.measureList(identity, { state: MeasureState.UNSPECIFIED });
const measure = (over: Partial<SellerMeasure> = {}) =>
	({ id: "m1", etag: "etag-1", canAppeal: true, ...over }) as SellerMeasure;

let client: QueryClient;
const wrapper = ({ children }: { children: React.ReactNode }) =>
	React.createElement(
		IntlTestWrapper,
		null,
		React.createElement(QueryClientProvider, { client }, children),
	);

beforeEach(() => {
	client = new QueryClient({
		defaultOptions: { queries: { retry: false, gcTime: Infinity } },
	});
	mocks.auth.hydrated = true;
	mocks.auth.isLoggedIn = true;
	mocks.fetchMyMeasures.mockReset().mockResolvedValue([measure()]);
	mocks.fetchHasMeasures.mockReset().mockResolvedValue(true);
	mocks.fetchMyMeasure.mockReset().mockResolvedValue(measure());
	mocks.submitMeasureAppeal.mockReset();
	mocks.submitMeasureRemediation.mockReset();
});
afterEach(cleanup);

describe("措施查詢的啟用條件", () => {
	it("登入的非賣家（店家全關的帳號，沒有賣家角色）也會讀取", async () => {
		const list = renderHook(() => useSellerMeasures(), { wrapper });
		const exists = renderHook(() => useHasMeasures(), { wrapper });
		await waitFor(() => expect(list.result.current.records).toHaveLength(1));
		await waitFor(() => expect(exists.result.current).toBe(true));
		expect(mocks.fetchMyMeasures).toHaveBeenCalledTimes(1);
		expect(mocks.fetchHasMeasures).toHaveBeenCalledTimes(1);
	});

	it.each([
		["尚未 hydrate", { hydrated: false, isLoggedIn: true }],
		["未登入", { hydrated: true, isLoggedIn: false }],
	])("%s 不讀取", async (_name, state) => {
		Object.assign(mocks.auth, state);
		const list = renderHook(() => useSellerMeasures(), { wrapper });
		const exists = renderHook(() => useHasMeasures(), { wrapper });
		const one = renderHook(() => useSellerMeasure("m1"), { wrapper });
		await act(async () => {});
		expect(mocks.fetchMyMeasures).not.toHaveBeenCalled();
		expect(mocks.fetchHasMeasures).not.toHaveBeenCalled();
		expect(mocks.fetchMyMeasure).not.toHaveBeenCalled();
		expect(list.result.current.records).toEqual([]);
		expect(exists.result.current).toBe(false);
		expect(one.result.current.measure).toBeNull();
	});

	it("列表與 exists 各用自己的 key", async () => {
		mocks.fetchMyMeasures.mockResolvedValue([measure(), measure({ id: "m2" })]);
		renderHook(() => useSellerMeasures(), { wrapper });
		renderHook(() => useHasMeasures(), { wrapper });
		await waitFor(() => expect(client.getQueryData(listKey())).toHaveLength(2));
		await waitFor(() =>
			expect(
				client.getQueryData(queryKeys.seller.measureExists(identity)),
			).toBe(true),
		);
	});
});

describe("useSellerMeasure", () => {
	it("列表已讀過就用列表裡的那筆當初值，不再讀單筆", async () => {
		client.setQueryData(listKey(), [measure({ etag: "from-list" })]);
		const { result } = renderHook(() => useSellerMeasure("m1"), { wrapper });
		expect(result.current.measure?.etag).toBe("from-list");
		expect(result.current.loading).toBe(false);
	});

	it("not_found 與 permission_denied 是 notFound，其餘錯誤不是", async () => {
		const { ConnectError, Code } = await import("@connectrpc/connect");
		mocks.fetchMyMeasure.mockRejectedValue(
			new ConnectError("x", Code.NotFound),
		);
		const missing = renderHook(() => useSellerMeasure("gone"), { wrapper });
		await waitFor(() => expect(missing.result.current.notFound).toBe(true));
		mocks.fetchMyMeasure.mockRejectedValue(
			new ConnectError("x", Code.Unavailable),
		);
		const broken = renderHook(() => useSellerMeasure("down"), { wrapper });
		await waitFor(() => expect(broken.result.current.error).not.toBeNull());
		expect(broken.result.current.notFound).toBe(false);
	});
});

describe.each([
	["useSubmitSellerAppeal", useSubmitSellerAppeal, "submitMeasureAppeal"],
	[
		"useSubmitSellerRemediation",
		useSubmitSellerRemediation,
		"submitMeasureRemediation",
	],
] as const)("%s", (_name, useSubmit, submitMock) => {
	it("成功後把回傳的措施寫回單筆與列表，並讓整個措施 family 失效", async () => {
		const stale = measure();
		const result = measure({ etag: "etag-2", canAppeal: false });
		client.setQueryData(listKey(), [stale, measure({ id: "m2" })]);
		client.setQueryData(queryKeys.seller.measureExists(identity), true);
		client.setQueryData(queryKeys.seller.measure(identity, "m1"), stale);
		mocks[submitMock].mockResolvedValue(result);
		const invalidate = vi.spyOn(client, "invalidateQueries");
		const { result: hook } = renderHook(() => useSubmit(), { wrapper });

		let returned: SellerMeasure | undefined;
		await act(async () => {
			returned = await hook.current(stale, {
				reason: "理由",
				evidenceStagedTokens: [],
			});
		});

		expect(returned).toBe(result);
		expect(mocks[submitMock]).toHaveBeenCalledWith(stale, {
			reason: "理由",
			evidenceStagedTokens: [],
		});
		expect(
			client.getQueryData(queryKeys.seller.measure(identity, "m1")),
		).toEqual(result);
		const list = client.getQueryData<SellerMeasure[]>(listKey());
		expect(list?.map((item) => item.id)).toEqual(["m1", "m2"]);
		expect(list?.[0]).toEqual(result);
		expect(invalidate).toHaveBeenCalledWith({
			queryKey: queryKeys.seller.measures(identity),
		});
		expect(client.getQueryState(listKey())?.isInvalidated).toBe(true);
		expect(
			client.getQueryState(queryKeys.seller.measureExists(identity))
				?.isInvalidated,
		).toBe(true);
	});

	it("送出失敗時不動快取、不失效", async () => {
		const stale = measure();
		client.setQueryData(listKey(), [stale]);
		mocks[submitMock].mockRejectedValue(new Error("boom"));
		const invalidate = vi.spyOn(client, "invalidateQueries");
		const { result: hook } = renderHook(() => useSubmit(), { wrapper });
		await act(async () => {
			await expect(
				hook.current(stale, { reason: "理由", evidenceStagedTokens: [] }),
			).rejects.toThrow("boom");
		});
		expect(invalidate).not.toHaveBeenCalled();
		expect(client.getQueryData(listKey())).toEqual([stale]);
	});
});
