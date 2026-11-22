// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BlockedBuyerSchema,
	BlockedStoreSchema,
	ListBlockedBuyersResponseSchema,
	ListBlockedStoresResponseSchema,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { StoreCardStatus } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
	waitFor,
	within,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
	listBlockedStores: vi.fn(),
	listBlockedBuyers: vi.fn(),
	unblockStore: vi.fn(),
	unblockBuyer: vi.fn(),
}));
vi.mock("@/lib/connect/buyer", () => ({ buyerAppClient: () => rpc }));
vi.mock("@/lib/auth/use-account-status", () => ({
	useAccountStatus: () => "signedIn",
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "7", activeRole: "buyer" }),
}));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/Avatar", () => ({
	default: ({ name }: { name?: string | null }) =>
		React.createElement("span", { "data-testid": "avatar", "data-name": name }),
}));
vi.mock("@/components/ui/Dialog", () => ({
	Dialog: ({
		open,
		title,
		children,
		footer,
	}: {
		open: boolean;
		title: string;
		children: React.ReactNode;
		footer: React.ReactNode;
	}) =>
		open
			? React.createElement(
					"div",
					{ role: "dialog", "aria-label": title },
					children,
					footer,
				)
			: null,
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import BlockedListClient from "./BlockedListClient";

const AT = timestampFromDate(new Date("2026-11-01T00:30:00Z"));

function store(id: string, name?: string, status = StoreCardStatus.OPEN) {
	return create(BlockedStoreSchema, {
		storeId: id,
		store: name === undefined ? undefined : { id, name, status },
		createTime: AT,
	});
}
function buyer(id: string, name: string, deleted = false) {
	return create(BlockedBuyerSchema, {
		buyerId: id,
		displayName: name,
		deleted,
		createTime: AT,
	});
}
const storePage = (items: ReturnType<typeof store>[], next = "") =>
	create(ListBlockedStoresResponseSchema, {
		blockedStores: items,
		nextPageToken: next,
	});
const buyerPage = (items: ReturnType<typeof buyer>[], next = "") =>
	create(ListBlockedBuyersResponseSchema, {
		blockedBuyers: items,
		nextPageToken: next,
	});

function render() {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return renderWithoutIntl(React.createElement(BlockedListClient), {
		wrapper: ({ children }: { children: React.ReactNode }) =>
			React.createElement(
				QueryClientProvider,
				{ client },
				React.createElement(IntlTestWrapper, null, children),
			),
	});
}

beforeEach(() => {
	for (const fn of Object.values(rpc)) fn.mockReset();
	rpc.listBlockedStores.mockResolvedValue(storePage([]));
	rpc.listBlockedBuyers.mockResolvedValue(buyerPage([]));
});
afterEach(cleanup);

describe("BlockedListClient", () => {
	it("lists blocked stores and buyers in separate sections with the block date", async () => {
		rpc.listBlockedStores.mockResolvedValue(storePage([store("5", "好店")]));
		rpc.listBlockedBuyers.mockResolvedValue(
			buyerPage([buyer("11", "小明"), buyer("12", "", true)]),
		);
		render();
		expect(await screen.findByText("好店")).toBeTruthy();
		expect(screen.getByText("小明")).toBeTruthy();
		expect(screen.getByText("已刪除的帳號")).toBeTruthy();
		expect(screen.getAllByText("2026/11/01").length).toBe(3);
		expect(
			within(screen.getByTestId("blocked-stores")).getByText("好店"),
		).toBeTruthy();
		expect(
			within(screen.getByTestId("blocked-buyers")).getByText("小明"),
		).toBeTruthy();
		expect(rpc.listBlockedStores).toHaveBeenCalledWith({
			pageSize: 20,
			pageToken: "",
		});
	});

	it("shows a closed store with its status and a store without a card as a closed store", async () => {
		rpc.listBlockedStores.mockResolvedValue(
			storePage([store("5", "舊店", StoreCardStatus.CLOSED), store("6")]),
		);
		render();
		expect(await screen.findByText("舊店")).toBeTruthy();
		expect(screen.getAllByRole("button", { name: "解除封鎖" })).toHaveLength(2);
	});

	it("both sections empty shows the empty state, not an error", async () => {
		render();
		expect(await screen.findByText("尚無封鎖的使用者")).toBeTruthy();
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("both reads failing shows the error with retry, distinct from empty", async () => {
		rpc.listBlockedStores.mockRejectedValue(
			new ConnectError("壞了", Code.Internal),
		);
		rpc.listBlockedBuyers.mockRejectedValue(
			new ConnectError("壞了", Code.Internal),
		);
		render();
		expect(await screen.findByRole("alert")).toBeTruthy();
		expect(screen.queryByText("尚無封鎖的使用者")).toBeNull();
		rpc.listBlockedStores.mockResolvedValue(storePage([store("5", "好店")]));
		fireEvent.click(screen.getByRole("button", { name: "重試" }));
		expect(await screen.findByText("好店")).toBeTruthy();
	});

	it("one failing section keeps the other and offers a retry for that section", async () => {
		rpc.listBlockedStores.mockRejectedValue(
			new ConnectError("店家讀取失敗", Code.Internal),
		);
		rpc.listBlockedBuyers.mockResolvedValue(buyerPage([buyer("11", "小明")]));
		render();
		expect(await screen.findByText("小明")).toBeTruthy();
		const section = screen.getByTestId("blocked-stores");
		expect(within(section).getByRole("alert")).toBeTruthy();
		expect(within(section).getByRole("button", { name: "重試" })).toBeTruthy();
	});

	it("see more sends the section's next_page_token and appends", async () => {
		rpc.listBlockedBuyers
			.mockResolvedValueOnce(buyerPage([buyer("11", "小明")], "tok-2"))
			.mockResolvedValueOnce(buyerPage([buyer("12", "小華")]));
		render();
		fireEvent.click(await screen.findByRole("button", { name: "查看更多" }));
		expect(await screen.findByText("小華")).toBeTruthy();
		expect(screen.getByText("小明")).toBeTruthy();
		expect(rpc.listBlockedBuyers).toHaveBeenNthCalledWith(2, {
			pageSize: 20,
			pageToken: "tok-2",
		});
	});

	it("unblocking a store asks first, then sends the store id and removes the row", async () => {
		rpc.listBlockedStores
			.mockResolvedValueOnce(
				storePage([store("5", "好店"), store("6", "別店")]),
			)
			.mockReturnValue(new Promise(() => {}));
		rpc.unblockStore.mockResolvedValue({ changed: true });
		render();
		await screen.findByText("好店");
		fireEvent.click(screen.getAllByRole("button", { name: "解除封鎖" })[0]);
		const dialog = screen.getByRole("dialog", { name: "解除封鎖「好店」？" });
		expect(rpc.unblockStore).not.toHaveBeenCalled();
		fireEvent.click(within(dialog).getByRole("button", { name: "解除封鎖" }));
		await waitFor(() => expect(screen.queryByText("好店")).toBeNull());
		expect(rpc.unblockStore).toHaveBeenCalledWith({ storeId: "5" });
		expect(rpc.unblockBuyer).not.toHaveBeenCalled();
		expect(screen.getByText("別店")).toBeTruthy();
	});

	it("unblocking a buyer sends the buyer id", async () => {
		rpc.listBlockedBuyers
			.mockResolvedValueOnce(buyerPage([buyer("11", "小明")]))
			.mockReturnValue(new Promise(() => {}));
		rpc.unblockBuyer.mockResolvedValue({ changed: true });
		render();
		await screen.findByText("小明");
		fireEvent.click(screen.getByRole("button", { name: "解除封鎖" }));
		fireEvent.click(
			within(screen.getByRole("dialog")).getByRole("button", {
				name: "解除封鎖",
			}),
		);
		await waitFor(() =>
			expect(rpc.unblockBuyer).toHaveBeenCalledWith({ buyerId: "11" }),
		);
		expect(rpc.unblockStore).not.toHaveBeenCalled();
	});

	it("a failed unblock shows the error on the list and keeps the row", async () => {
		rpc.listBlockedBuyers.mockResolvedValue(buyerPage([buyer("11", "小明")]));
		rpc.unblockBuyer.mockRejectedValue(
			new ConnectError("暫時無法解除", Code.Internal),
		);
		render();
		await screen.findByText("小明");
		fireEvent.click(screen.getByRole("button", { name: "解除封鎖" }));
		fireEvent.click(
			within(screen.getByRole("dialog")).getByRole("button", {
				name: "解除封鎖",
			}),
		);
		expect((await screen.findByRole("alert")).textContent).toBeTruthy();
		expect(screen.getByText("小明")).toBeTruthy();
		expect(screen.queryByRole("dialog")).toBeNull();
	});
});
