// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BlockedBuyerSchema,
	ListBlockedBuyersResponseSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	act,
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
	listBlockedBuyers: vi.fn(),
	unblockBuyer: vi.fn(),
}));
vi.mock("@/lib/connect/seller", () => ({ sellerAppClient: () => rpc }));
vi.mock("@/lib/auth/use-account-status", () => ({
	useAccountStatus: () => "signedIn",
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "7", activeRole: "seller" }),
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
import BlockedBuyersPage from "./BlockedBuyersPage";

function blocked(id: string, name: string, at = "2026-11-01T00:30:00Z") {
	return create(BlockedBuyerSchema, {
		buyer: { id, displayName: name },
		createTime: timestampFromDate(new Date(at)),
	});
}
function page(items: ReturnType<typeof blocked>[], nextPageToken = "") {
	return create(ListBlockedBuyersResponseSchema, {
		blockedBuyers: items,
		nextPageToken,
	});
}

let client: QueryClient;
function render() {
	client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return renderWithoutIntl(React.createElement(BlockedBuyersPage), {
		wrapper: ({ children }: { children: React.ReactNode }) =>
			React.createElement(
				QueryClientProvider,
				{ client },
				React.createElement(IntlTestWrapper, null, children),
			),
	});
}

beforeEach(() => {
	rpc.listBlockedBuyers.mockReset();
	rpc.unblockBuyer.mockReset();
});
afterEach(cleanup);

describe("BlockedBuyersPage", () => {
	it("lists each blocked buyer with name and block date", async () => {
		rpc.listBlockedBuyers.mockResolvedValue(
			page([
				blocked("11", "小明"),
				blocked("12", "小華", "2026-10-02T00:00:00Z"),
			]),
		);
		render();
		expect(await screen.findByText("小明")).toBeTruthy();
		expect(screen.getByText("小華")).toBeTruthy();
		expect(screen.getByText("2026/11/01")).toBeTruthy();
		expect(screen.getByText("2026/10/02")).toBeTruthy();
		expect(rpc.listBlockedBuyers).toHaveBeenCalledWith({
			pageSize: 20,
			pageToken: "",
		});
	});

	it("an empty list shows the empty state", async () => {
		rpc.listBlockedBuyers.mockResolvedValue(page([]));
		render();
		expect(await screen.findByText("尚無封鎖名單")).toBeTruthy();
		expect(screen.getByText("你封鎖的使用者會顯示在這裡。")).toBeTruthy();
	});

	it("load more sends the previous next_page_token and appends the rows", async () => {
		rpc.listBlockedBuyers
			.mockResolvedValueOnce(page([blocked("11", "小明")], "tok-2"))
			.mockResolvedValueOnce(page([blocked("12", "小華")]));
		render();
		fireEvent.click(await screen.findByRole("button", { name: "載入更多" }));
		expect(await screen.findByText("小華")).toBeTruthy();
		expect(screen.getByText("小明")).toBeTruthy();
		expect(rpc.listBlockedBuyers).toHaveBeenNthCalledWith(2, {
			pageSize: 20,
			pageToken: "tok-2",
		});
		expect(screen.queryByRole("button", { name: "載入更多" })).toBeNull();
	});

	it("unblock asks first, then sends the buyer id and removes the row", async () => {
		rpc.listBlockedBuyers
			.mockResolvedValueOnce(
				page([blocked("11", "小明"), blocked("12", "小華")]),
			)
			.mockReturnValue(new Promise(() => {}));
		rpc.unblockBuyer.mockResolvedValue({ changed: true });
		render();
		await screen.findByText("小明");
		fireEvent.click(screen.getAllByRole("button", { name: "解除封鎖" })[0]);
		const dialog = screen.getByRole("dialog", { name: "解除封鎖「小明」？" });
		expect(rpc.unblockBuyer).not.toHaveBeenCalled();
		fireEvent.click(within(dialog).getByRole("button", { name: "解除封鎖" }));
		await waitFor(() => expect(screen.queryByText("小明")).toBeNull());
		expect(rpc.unblockBuyer).toHaveBeenCalledWith({ buyerId: "11" });
		expect(screen.getByText("小華")).toBeTruthy();
	});

	it("a failed unblock shows the error on that row and keeps the list", async () => {
		rpc.listBlockedBuyers.mockResolvedValue(
			page([blocked("11", "小明"), blocked("12", "小華")]),
		);
		rpc.unblockBuyer.mockRejectedValue(
			new ConnectError("暫時無法解除", Code.Internal),
		);
		render();
		await screen.findByText("小明");
		fireEvent.click(screen.getAllByRole("button", { name: "解除封鎖" })[0]);
		const dialog = screen.getByRole("dialog");
		fireEvent.click(within(dialog).getByRole("button", { name: "解除封鎖" }));
		const alert = await screen.findByRole("alert");
		expect(alert.textContent).toBeTruthy();
		expect(screen.getAllByRole("alert")).toHaveLength(1);
		expect(screen.getByText("小明")).toBeTruthy();
		expect(screen.getByText("小華")).toBeTruthy();
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("unblocking every loaded row while another page exists keeps load more instead of the empty state", async () => {
		rpc.listBlockedBuyers
			.mockResolvedValueOnce(page([blocked("11", "小明")], "tok-2"))
			.mockReturnValue(new Promise(() => {}));
		rpc.unblockBuyer.mockResolvedValue({ changed: true });
		render();
		await screen.findByText("小明");
		fireEvent.click(screen.getByRole("button", { name: "解除封鎖" }));
		const dialog = screen.getByRole("dialog");
		fireEvent.click(within(dialog).getByRole("button", { name: "解除封鎖" }));
		await waitFor(() => expect(screen.queryByText("小明")).toBeNull());
		expect(screen.queryByText("尚無封鎖名單")).toBeNull();
		expect(screen.getByRole("button", { name: "載入更多" })).toBeTruthy();
	});

	it("activating the confirm button twice in a row sends one unblock", async () => {
		rpc.listBlockedBuyers.mockResolvedValue(page([blocked("11", "小明")]));
		rpc.unblockBuyer.mockReturnValue(new Promise(() => {}));
		render();
		await screen.findByText("小明");
		fireEvent.click(screen.getByRole("button", { name: "解除封鎖" }));
		const confirm = within(screen.getByRole("dialog")).getByRole("button", {
			name: "解除封鎖",
		});
		act(() => {
			fireEvent.click(confirm);
			fireEvent.click(confirm);
		});
		await waitFor(() => expect(rpc.unblockBuyer).toHaveBeenCalled());
		await new Promise((resolve) => setTimeout(resolve, 20));
		expect(rpc.unblockBuyer).toHaveBeenCalledTimes(1);
	});

	it("labels a deleted buyer with the seller-app wording", async () => {
		rpc.listBlockedBuyers.mockResolvedValue(
			page([
				create(BlockedBuyerSchema, {
					buyer: { id: "13", deleted: true },
					createTime: timestampFromDate(new Date("2026-11-01T00:30:00Z")),
				}),
			]),
		);
		render();
		expect(await screen.findByText("已刪除帳號")).toBeTruthy();
	});
});
