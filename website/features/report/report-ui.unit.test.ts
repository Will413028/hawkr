// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { BadRequestSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	act,
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
	createReport: vi.fn(),
	sellerSelf: null as null | { store: { id: string; storeCode: string } },
}));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: () => ({
		data: mocks.sellerSelf,
		loading: false,
		error: null,
	}),
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: mocks.status, activeRole: null }),
}));
vi.mock("@/lib/state/use-login-required-overlay", () => ({
	useLoginRequiredOverlay: () => mocks.openLogin,
}));
vi.mock("./api", () => ({ createReport: mocks.createReport }));

let uploaderProps: {
	value: unknown[];
	onChange: (v: unknown[]) => void;
	onUploadingChange: (v: boolean) => void;
};
vi.mock("@/features/after-sales/components/AfterSalePhotoUploader", () => ({
	default: (props: typeof uploaderProps) => {
		uploaderProps = props;
		return null;
	},
}));

import ReportDialog from "./ReportDialog";
import ReportMenu from "./ReportMenu";

function wrap(node: React.ReactNode) {
	return React.createElement(NextIntlClientProvider, {
		locale: "zh-TW",
		messages: messagesFor("zh-TW"),
		children: node,
	});
}

const dialogEl = (open: boolean, onClose: () => void) =>
	React.createElement(ReportDialog, { open, onClose, storeId: "s1" });
const renderDialog = (onClose: () => void) =>
	render(wrap(dialogEl(true, onClose)));
function fillValid() {
	fireEvent.change(screen.getByLabelText(/類別/), {
		target: { value: String(PolicyCategory.SPAM) },
	});
	fireEvent.change(screen.getByLabelText(/說明/), { target: { value: "x" } });
}

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
	mocks.status = "guest";
	mocks.sellerSelf = null;
});

describe("ReportMenu", () => {
	it("guest clicking 檢舉 opens the login-required flow, not the form", () => {
		render(wrap(React.createElement(ReportMenu, { storeId: "s1" })));
		fireEvent.click(screen.getByRole("button", { name: "更多動作" }));
		fireEvent.click(screen.getByRole("menuitem", { name: "檢舉" }));
		expect(mocks.openLogin).toHaveBeenCalledTimes(1);
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("is hidden on the viewer's own store", () => {
		mocks.sellerSelf = { store: { id: "s1", storeCode: "code1" } };
		render(wrap(React.createElement(ReportMenu, { storeId: "s1" })));
		expect(screen.queryByRole("button", { name: "更多動作" })).toBeNull();
	});

	it("signed-in buyer gets the form", () => {
		mocks.status = "signedIn";
		render(wrap(React.createElement(ReportMenu, { storeId: "s1" })));
		fireEvent.click(screen.getByRole("button", { name: "更多動作" }));
		fireEvent.click(screen.getByRole("menuitem", { name: "檢舉" }));
		expect(mocks.openLogin).not.toHaveBeenCalled();
		expect(screen.getByRole("dialog")).toBeTruthy();
	});
});

describe("ReportDialog", () => {
	it("renders the server notice when the report was supplemented", async () => {
		mocks.createReport.mockResolvedValue({
			supplemented: true,
			notice: { message: "今天已檢舉過，內容已補充" },
		});
		render(
			wrap(
				React.createElement(ReportDialog, {
					open: true,
					onClose: () => {},
					storeId: "s1",
					product: { id: "p1", name: "蘋果" },
				}),
			),
		);
		expect(screen.getByText("檢舉商品：蘋果")).toBeTruthy();
		fireEvent.change(screen.getByLabelText(/類別/), {
			target: { value: String(PolicyCategory.SPAM) },
		});
		fireEvent.change(screen.getByLabelText(/說明/), {
			target: { value: "內容" },
		});
		fireEvent.click(screen.getByRole("button", { name: "送出檢舉" }));
		await waitFor(() =>
			expect(screen.getByText("今天已檢舉過，內容已補充")).toBeTruthy(),
		);
		expect(mocks.createReport).toHaveBeenCalledWith(
			expect.objectContaining({
				storeId: "s1",
				productId: "p1",
				category: PolicyCategory.SPAM,
				description: "內容",
				evidenceStagedTokens: [],
			}),
		);
	});

	it("shows the received text when not supplemented", async () => {
		mocks.createReport.mockResolvedValue({ supplemented: false });
		render(
			wrap(
				React.createElement(ReportDialog, {
					open: true,
					onClose: () => {},
					storeId: "s1",
				}),
			),
		);
		fireEvent.change(screen.getByLabelText(/類別/), {
			target: { value: String(PolicyCategory.SPAM) },
		});
		fireEvent.change(screen.getByLabelText(/說明/), { target: { value: "x" } });
		fireEvent.click(screen.getByRole("button", { name: "送出檢舉" }));
		await waitFor(() =>
			expect(
				screen.getByText("已收到檢舉，處理結果可在「我的檢舉」查看"),
			).toBeTruthy(),
		);
	});

	it("shows a field violation that no rendered field matches in the alert", async () => {
		mocks.createReport.mockRejectedValue(
			new ConnectError("bad", Code.InvalidArgument, undefined, [
				{
					desc: BadRequestSchema,
					value: {
						fieldViolations: [
							{
								field: "store_id",
								description: "own store",
								localizedMessage: {
									locale: "zh-TW",
									message: "不能檢舉自己的店家",
								},
							},
						],
					},
				},
			]),
		);
		renderDialog(vi.fn());
		fillValid();
		fireEvent.click(screen.getByRole("button", { name: "送出檢舉" }));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toContain(
				"不能檢舉自己的店家",
			),
		);
	});

	it("ignores a photo upload that finishes after close and reopen", async () => {
		const onClose = vi.fn();
		const view = renderDialog(onClose);
		act(() => uploaderProps.onUploadingChange(true));
		fireEvent.click(screen.getByRole("button", { name: "取消" }));
		expect(onClose).not.toHaveBeenCalled();
		view.rerender(wrap(dialogEl(false, onClose)));
		view.rerender(wrap(dialogEl(true, onClose)));
		act(() => {
			uploaderProps.onChange([{ token: "t1", previewUrl: "u" }]);
			uploaderProps.onUploadingChange(false);
		});
		expect(uploaderProps.value).toEqual([]);
	});
});
