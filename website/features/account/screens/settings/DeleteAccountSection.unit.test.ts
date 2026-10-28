// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { ErrorInfoSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { DeleteAccountResponse_DeletionState } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
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
	deleteAccount: vi.fn(),
	signOut: vi.fn(async () => {}),
	externalAccounts: [] as { provider: string }[],
}));
vi.mock("@clerk/nextjs", () => ({
	useUser: () => ({ user: { externalAccounts: mocks.externalAccounts } }),
}));
vi.mock("@/lib/api/auth", () => ({ deleteAccount: mocks.deleteAccount }));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ signOut: mocks.signOut }),
}));
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

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import DeleteAccountSection from "./DeleteAccountSection";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

const assign = vi.fn();

function openDialog() {
	render(React.createElement(DeleteAccountSection));
	fireEvent.click(screen.getByRole("button", { name: "刪除帳號" }));
	return screen.getByRole("dialog", { name: "刪除帳號" });
}

beforeEach(() => {
	mocks.deleteAccount.mockReset();
	mocks.signOut.mockClear();
	mocks.externalAccounts = [];
	assign.mockReset();
	vi.stubGlobal("location", { ...window.location, assign });
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe("DeleteAccountSection", () => {
	it("asks for confirmation and does not call DeleteAccount before it", () => {
		openDialog();
		expect(mocks.deleteAccount).not.toHaveBeenCalled();
		fireEvent.click(screen.getByRole("button", { name: "取消" }));
		expect(screen.queryByRole("dialog")).toBeNull();
		expect(mocks.deleteAccount).not.toHaveBeenCalled();
	});

	it("a deleted account signs out and leaves", async () => {
		mocks.deleteAccount.mockResolvedValue(
			DeleteAccountResponse_DeletionState.DELETED,
		);
		openDialog();
		fireEvent.click(screen.getByRole("button", { name: "確認刪除" }));

		await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
		expect(mocks.signOut).toHaveBeenCalledOnce();
	});

	it("unfinished orders start the deletion flow and keep the session", async () => {
		mocks.deleteAccount.mockResolvedValue(
			DeleteAccountResponse_DeletionState.PENDING_ORDERS,
		);
		openDialog();
		fireEvent.click(screen.getByRole("button", { name: "確認刪除" }));

		await waitFor(() =>
			expect(screen.getByRole("status").textContent).toContain(
				"帳號刪除處理中",
			),
		);
		expect(mocks.signOut).not.toHaveBeenCalled();
		expect(assign).not.toHaveBeenCalled();
	});

	it("a failed precondition carrying a different reason is a generic failure", async () => {
		mocks.deleteAccount.mockRejectedValue(
			new ConnectError("other", Code.FailedPrecondition, undefined, [
				{
					desc: ErrorInfoSchema,
					value: {
						reason: ErrorReason[ErrorReason.VERSION_CONFLICT],
						domain: "hawkr",
					},
				},
			]),
		);
		openDialog();
		fireEvent.click(screen.getByRole("button", { name: "確認刪除" }));

		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(mocks.signOut).not.toHaveBeenCalled();
	});

	it("a failed precondition without the store reason is a generic failure", async () => {
		mocks.deleteAccount.mockRejectedValue(
			new ConnectError("no pending deletion", Code.FailedPrecondition),
		);
		openDialog();
		fireEvent.click(screen.getByRole("button", { name: "確認刪除" }));

		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(mocks.signOut).not.toHaveBeenCalled();
	});

	it("any other failure is shown and the account stays signed in", async () => {
		mocks.deleteAccount.mockRejectedValue(
			new ConnectError("down", Code.Unavailable),
		);
		openDialog();
		fireEvent.click(screen.getByRole("button", { name: "確認刪除" }));

		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(mocks.signOut).not.toHaveBeenCalled();
	});

	it("an unspecified state is not treated as deleted", async () => {
		mocks.deleteAccount.mockResolvedValue(
			DeleteAccountResponse_DeletionState.UNSPECIFIED,
		);
		openDialog();
		fireEvent.click(screen.getByRole("button", { name: "確認刪除" }));

		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(mocks.signOut).not.toHaveBeenCalled();
		expect(assign).not.toHaveBeenCalled();
	});

	it("an account with an Apple external account is told to revoke the authorization", () => {
		mocks.externalAccounts = [{ provider: "google" }, { provider: "apple" }];
		const dialog = openDialog();
		expect(dialog.textContent).toContain(
			"刪除後請到 Apple ID 設定的「使用 Apple 登入」移除 Hawkr 的授權。",
		);
	});

	it("accounts without an Apple external account see no Apple guidance", () => {
		mocks.externalAccounts = [{ provider: "google" }];
		const dialog = openDialog();
		expect(dialog.textContent).not.toContain("Apple");
		cleanup();
		mocks.externalAccounts = [];
		expect(openDialog().textContent).not.toContain("Apple");
	});
});
