// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { CustomerType } from "@hawkr/api/gen/hawkr/crm/v1alpha1/crm_pb";
import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import { createElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	listCustomers: vi.fn(),
	listAllCustomers: vi.fn(),
}));
vi.mock("@/lib/seller/crm", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/seller/crm")>()),
	listCustomers: mocks.listCustomers,
	listAllCustomers: mocks.listAllCustomers,
}));
vi.mock("@/lib/hooks/useCrmStoreId", () => ({
	useCrmStoreId: () => ({ storeId: "9", loading: false }),
}));
vi.mock("@/lib/hooks/useSellerTags", () => ({
	useSellerTags: () => ({ tags: [{ id: "t1", name: "回購" }], error: null }),
}));
vi.mock("@/lib/i18n/ui-text", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/i18n/ui-text")>();
	const defaults = (code: Code) => `default-${code}`;
	return {
		...actual,
		useDefaultErrorText: () => defaults,
		useUiText: () => actual.uiTextFor("zh-TW"),
	};
});
vi.mock("@/lib/browser/body-scroll-lock", () => ({
	acquireBodyScrollLock: () => () => {},
}));
vi.mock("@/components/shared/Avatar", () => ({ default: () => null }));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import CouponBuyerPicker from "./CouponBuyerPicker";

const customer = (id: string) => ({
	id,
	name: `買家${id}`,
	deleted: false,
	customerType: CustomerType.NORMAL,
	totalSpent: 0,
	orderCount: 1,
	tags: [],
});

function open(onConfirm = vi.fn()) {
	render(
		createElement(
			IntlTestWrapper,
			null,
			createElement(CouponBuyerPicker, {
				open: true,
				initialSelected: [],
				onConfirm,
				onClose: () => {},
			}),
		),
	);
	return onConfirm;
}

describe("CouponBuyerPicker (customer list, no lookup)", () => {
	afterEach(cleanup);
	beforeEach(() => {
		mocks.listCustomers.mockReset();
		mocks.listAllCustomers.mockReset();
	});

	it("lists customers from ListCustomers without a keyword and confirms the selection", async () => {
		mocks.listCustomers.mockResolvedValue({
			customers: [customer("1"), customer("2")],
			nextPageToken: "",
		});
		const onConfirm = open();
		await screen.findByText("買家1");
		expect(screen.queryByLabelText("搜尋客戶")).toBeNull();
		expect(mocks.listCustomers).toHaveBeenCalledWith(
			"9",
			expect.not.objectContaining({ keyword: expect.anything() }),
		);
		fireEvent.click(screen.getByLabelText("選擇 買家2"));
		fireEvent.click(screen.getByText("確認（1）"));
		expect(onConfirm.mock.calls[0][0].map((c: { id: string }) => c.id)).toEqual(
			["2"],
		);
	});

	it("loads the next page with the page token", async () => {
		mocks.listCustomers
			.mockResolvedValueOnce({
				customers: [customer("1")],
				nextPageToken: "p2",
			})
			.mockResolvedValueOnce({ customers: [customer("2")], nextPageToken: "" });
		open();
		fireEvent.click(await screen.findByText("載入更多"));
		await screen.findByText("買家2");
		expect(mocks.listCustomers.mock.calls[1][1]).toMatchObject({
			pageToken: "p2",
		});
		expect(screen.queryByText("載入更多")).toBeNull();
	});

	it("shows the failure, not the empty state, when the list cannot be read", async () => {
		mocks.listCustomers.mockRejectedValue(
			new ConnectError("down", Code.Unavailable),
		);
		open();
		await screen.findByRole("alert");
		expect(screen.queryByText("目前篩選沒有客戶。")).toBeNull();
	});

	it("shows the empty state for an empty list", async () => {
		mocks.listCustomers.mockResolvedValue({ customers: [], nextPageToken: "" });
		open();
		await screen.findByText("目前篩選沒有客戶。");
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("selects the whole filter through every page", async () => {
		mocks.listCustomers.mockResolvedValue({
			customers: [customer("1")],
			nextPageToken: "p2",
		});
		mocks.listAllCustomers.mockResolvedValue([customer("1"), customer("2")]);
		open();
		await screen.findByText("買家1");
		fireEvent.click(screen.getByRole("button", { name: "VIP" }));
		await waitFor(() => expect(mocks.listCustomers).toHaveBeenCalledTimes(2));
		fireEvent.click(screen.getByText("全選此分群"));
		await screen.findByText("確認（2）");
		expect(mocks.listAllCustomers.mock.calls[0][1]).toMatchObject({
			type: CustomerType.VIP,
		});
	});
});
