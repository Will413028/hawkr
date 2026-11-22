// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { messagesFor } from "@/lib/intl/config";

const mocks = vi.hoisted(() => ({
	status: "guest",
	openLogin: vi.fn(),
	submit: vi.fn(),
}));
vi.mock("@tanstack/react-query", async (importOriginal) => ({
	...(await importOriginal<typeof import("@tanstack/react-query")>()),
	useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "1", activeRole: "buyer" }),
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: mocks.status, activeRole: null }),
}));
vi.mock("@/lib/state/use-login-required-overlay", () => ({
	useLoginRequiredOverlay: () => mocks.openLogin,
}));
vi.mock("./api", () => ({ submitCommunityReport: mocks.submit }));

import CommunityReportDialog from "./CommunityReportDialog";
import ReportActionMenu from "./ReportActionMenu";
import { validateCommunityReport } from "./report-logic";

function wrap(node: React.ReactNode) {
	return React.createElement(NextIntlClientProvider, {
		locale: "zh-TW",
		messages: messagesFor("zh-TW"),
		children: node,
	});
}

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
	mocks.status = "guest";
});

describe("validateCommunityReport", () => {
	it("needs a category; description is required only for OTHER and capped at 500 characters", () => {
		expect(
			validateCommunityReport({ category: undefined, description: "" }),
		).toEqual({
			category: "categoryRequired",
		});
		expect(
			validateCommunityReport({
				category: PolicyCategory.SPAM,
				description: "",
			}),
		).toEqual({});
		expect(
			validateCommunityReport({
				category: PolicyCategory.OTHER,
				description: "  ",
			}),
		).toEqual({ description: "descriptionRequired" });
		expect(
			validateCommunityReport({
				category: PolicyCategory.SPAM,
				description: "字".repeat(501),
			}),
		).toEqual({ description: "descriptionTooLong" });
		expect(
			validateCommunityReport({
				category: PolicyCategory.SPAM,
				description: "字".repeat(500),
			}),
		).toEqual({});
	});

	it("rejects a category outside accepted_policy_categories", () => {
		expect(
			validateCommunityReport({
				category: PolicyCategory.COUNTERFEIT,
				description: "",
			}),
		).toEqual({ category: "categoryRequired" });
	});
});

describe("CommunityReportDialog", () => {
	const open = () =>
		render(
			wrap(
				React.createElement(CommunityReportDialog, {
					open: true,
					onClose: () => {},
					target: { kind: "chatMessage", id: "m1" },
				}),
			),
		);
	const pick = (c: PolicyCategory) =>
		fireEvent.change(screen.getByLabelText(/類別/), {
			target: { value: String(c) },
		});

	it("does not submit OTHER without a description", () => {
		open();
		pick(PolicyCategory.OTHER);
		fireEvent.click(screen.getByRole("button", { name: "送出檢舉" }));
		expect(mocks.submit).not.toHaveBeenCalled();
		expect(screen.getByText("請填寫說明")).toBeTruthy();
	});

	it("submits the message id as a CHAT_MESSAGE target and shows the received text", async () => {
		mocks.submit.mockResolvedValue({ reportId: "r1" });
		open();
		pick(PolicyCategory.HARASSMENT);
		fireEvent.click(screen.getByRole("button", { name: "送出檢舉" }));
		await waitFor(() => expect(mocks.submit).toHaveBeenCalledTimes(1));
		const input = mocks.submit.mock.calls[0][0];
		expect(input.target).toEqual({ kind: "chatMessage", id: "m1" });
		expect(input.category).toBe(PolicyCategory.HARASSMENT);
		expect(input.requestId).toMatch(/^[0-9a-f-]{36}$/);
		expect(await screen.findByText(/已收到檢舉/)).toBeTruthy();
	});

	it("retries with the same request_id when nothing changed, a new one when the content changed", async () => {
		mocks.submit.mockRejectedValue(new ConnectError("x", Code.Unavailable));
		open();
		pick(PolicyCategory.SPAM);
		const submit = () =>
			fireEvent.click(screen.getByRole("button", { name: "送出檢舉" }));
		submit();
		await waitFor(() => expect(mocks.submit).toHaveBeenCalledTimes(1));
		await screen.findByRole("alert");
		submit();
		await waitFor(() => expect(mocks.submit).toHaveBeenCalledTimes(2));
		expect(mocks.submit.mock.calls[1][0].requestId).toBe(
			mocks.submit.mock.calls[0][0].requestId,
		);
		await screen.findByRole("alert");
		pick(PolicyCategory.FRAUD);
		submit();
		await waitFor(() => expect(mocks.submit).toHaveBeenCalledTimes(3));
		expect(mocks.submit.mock.calls[2][0].requestId).not.toBe(
			mocks.submit.mock.calls[0][0].requestId,
		);
	});
});

describe("ReportActionMenu", () => {
	const menu = (block?: Parameters<typeof ReportActionMenu>[0]["block"]) =>
		render(
			wrap(
				React.createElement(ReportActionMenu, {
					target: { kind: "review", id: "rv1" },
					block,
				}),
			),
		);

	it("review menu has report only, and a guest goes through login first", () => {
		menu();
		fireEvent.click(screen.getByRole("button", { name: "更多動作" }));
		expect(screen.queryByRole("menuitem", { name: "封鎖此人" })).toBeNull();
		fireEvent.click(screen.getByRole("menuitem", { name: "檢舉" }));
		expect(mocks.openLogin).toHaveBeenCalledTimes(1);
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("block asks for confirmation, runs once, then shows the blocked notice", async () => {
		mocks.status = "signedIn";
		const run = vi.fn().mockResolvedValue({ changed: true });
		const onBlocked = vi.fn();
		menu({ name: "小店", description: "封鎖後的後果", run, onBlocked });
		fireEvent.click(screen.getByRole("button", { name: "更多動作" }));
		fireEvent.click(screen.getByRole("menuitem", { name: "封鎖此人" }));
		expect(run).not.toHaveBeenCalled();
		expect(screen.getByText("封鎖後的後果")).toBeTruthy();
		fireEvent.click(screen.getByRole("button", { name: "封鎖" }));
		await waitFor(() => expect(run).toHaveBeenCalledTimes(1));
		expect(await screen.findByText("已封鎖「小店」")).toBeTruthy();
		expect(onBlocked).toHaveBeenCalledTimes(1);
	});

	it("a failed block shows the error and does not report success", async () => {
		mocks.status = "signedIn";
		const run = vi
			.fn()
			.mockRejectedValue(new ConnectError("x", Code.Unavailable));
		const onBlocked = vi.fn();
		menu({ name: "小店", description: "d", run, onBlocked });
		fireEvent.click(screen.getByRole("button", { name: "更多動作" }));
		fireEvent.click(screen.getByRole("menuitem", { name: "封鎖此人" }));
		fireEvent.click(screen.getByRole("button", { name: "封鎖" }));
		expect(await screen.findByRole("alert")).toBeTruthy();
		expect(onBlocked).not.toHaveBeenCalled();
		expect(screen.queryByText("已封鎖「小店」")).toBeNull();
	});
});
