// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	GetMyStoreResponseSchema,
	SellerCapability,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	store: null as unknown,
	closeMyStore: vi.fn(),
}));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: () => ({
		data: mocks.store,
		loading: false,
		error: null,
		refresh: async () => {},
	}),
}));
vi.mock("@/lib/seller/repo", () => ({ closeMyStore: mocks.closeMyStore }));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		children,
		onClick,
		disabled,
	}: {
		children: React.ReactNode;
		onClick?: () => void;
		disabled?: boolean;
	}) =>
		React.createElement(
			"button",
			{ type: "button", onClick, disabled },
			children,
		),
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

import { activeStatuses } from "@/lib/seller/capability-fixtures";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import CloseStoreSection from "./CloseStoreSection";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

function withCapabilities(capabilities: SellerCapability[]) {
	mocks.store = create(GetMyStoreResponseSchema, {
		store: { id: "1", etag: "e1" },
		capabilities: activeStatuses(capabilities),
	});
}

const replace = vi.fn();

beforeEach(() => {
	mocks.closeMyStore.mockReset();
	replace.mockReset();
	vi.stubGlobal("location", { ...window.location, replace });
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe("CloseStoreSection", () => {
	it("shows nothing when the store has no close capability", () => {
		withCapabilities([SellerCapability.ACCOUNT_SELF_READ]);
		render(React.createElement(CloseStoreSection));
		expect(screen.queryByText("關閉商店")).toBeNull();
	});

	it("opens a confirmation first and does not call CloseMyStore until confirmed", () => {
		withCapabilities([SellerCapability.ACCOUNT_CLOSE]);
		render(React.createElement(CloseStoreSection));

		fireEvent.click(screen.getByRole("button", { name: "關閉商店" }));
		expect(screen.getByRole("dialog", { name: "關閉商店" })).toBeTruthy();
		expect(mocks.closeMyStore).not.toHaveBeenCalled();

		fireEvent.click(screen.getByRole("button", { name: "取消" }));
		expect(screen.queryByRole("dialog")).toBeNull();
		expect(mocks.closeMyStore).not.toHaveBeenCalled();
	});

	it("confirming calls CloseMyStore once and leaves the workspace", async () => {
		withCapabilities([SellerCapability.ACCOUNT_CLOSE]);
		mocks.closeMyStore.mockResolvedValue({});
		render(React.createElement(CloseStoreSection));

		fireEvent.click(screen.getByRole("button", { name: "關閉商店" }));
		fireEvent.click(screen.getByRole("button", { name: "確認關閉" }));

		await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
		expect(mocks.closeMyStore).toHaveBeenCalledOnce();
	});

	it("shows the server's reason when the store cannot be closed and stays put", async () => {
		withCapabilities([SellerCapability.ACCOUNT_CLOSE]);
		mocks.closeMyStore.mockRejectedValue(
			new ConnectError("has open orders", Code.FailedPrecondition, undefined, [
				{
					desc: LocalizedMessageSchema,
					value: { locale: "zh-TW", message: "還有未結訂單，無法關閉店家" },
				},
			]),
		);
		render(React.createElement(CloseStoreSection));

		fireEvent.click(screen.getByRole("button", { name: "關閉商店" }));
		fireEvent.click(screen.getByRole("button", { name: "確認關閉" }));

		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toContain("還有未結訂單"),
		);
		expect(replace).not.toHaveBeenCalled();
	});
});
