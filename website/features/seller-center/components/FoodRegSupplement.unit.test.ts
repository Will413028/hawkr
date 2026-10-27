// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BadRequestSchema,
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	AgreementKind,
	FoodRegistrationState,
	GetMyStoreResponseSchema,
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
	submit: vi.fn(),
	cancel: vi.fn(),
	clear: vi.fn(),
}));

vi.mock("@/lib/seller/repo", () => ({
	submitFoodRegistration: mocks.submit,
	withdrawFoodRegistration: mocks.cancel,
	clearFoodRegistration: mocks.clear,
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
vi.mock("@/components/ui/Input", () => ({
	Input: ({
		label,
		value,
		onChange,
	}: {
		label: string;
		value: string;
		onChange: React.ChangeEventHandler<HTMLInputElement>;
	}) => React.createElement("input", { "aria-label": label, value, onChange }),
}));

import { SELLER_AGREEMENTS } from "@/features/legal/documents/sellerAgreements.gen";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import { FoodRegSupplement } from "./FoodRegSupplement";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

const NOTICE_ACCEPTANCES = [
	{
		kind: AgreementKind.FOOD_REGISTRATION_NOTICE,
		version: SELLER_AGREEMENTS.FOOD_REGISTRATION_NOTICE.version,
	},
];

function readNotice() {
	fireEvent.click(
		screen.getByRole("checkbox", {
			name: /我已閱讀〈食品業者登錄個人資料蒐集告知事項〉/,
		}),
	);
}

type StoreInit = NonNullable<
	Parameters<typeof create<typeof GetMyStoreResponseSchema>>[1]
>["store"];

function seller(store: StoreInit = {}) {
	return create(GetMyStoreResponseSchema, { store: { id: "1", ...store } });
}

const SUBMITTED = timestampFromDate(new Date("2026-10-15T04:00:00Z"));

beforeEach(() => {
	mocks.submit.mockReset().mockResolvedValue({});
	mocks.cancel.mockReset().mockResolvedValue({});
	mocks.clear.mockReset().mockResolvedValue({});
});
afterEach(() => cleanup());

describe("FoodRegSupplement", () => {
	it("未登錄：顯示說明與「送出審核」，字號正規化後送出", async () => {
		const onSaved = vi.fn();
		render(
			React.createElement(FoodRegSupplement, { seller: seller(), onSaved }),
		);
		expect(
			screen.getByText(
				/上架加工食品前須完成食品業者登錄。送出後平台會核對食藥署的登錄資料/,
			),
		).toBeTruthy();
		expect(screen.getByText(/審核期間沿用目前的設定。/)).toBeTruthy();
		const send = screen.getByRole("button", {
			name: "送出審核",
		}) as HTMLButtonElement;
		expect(send.disabled).toBe(true);
		fireEvent.change(screen.getByLabelText("食品業者登錄字號"), {
			target: { value: " a-1 " },
		});
		expect(send.disabled).toBe(true);
		expect(
			screen
				.getByRole("link", { name: "食品業者登錄個人資料蒐集告知事項" })
				.getAttribute("href"),
		).toBe("/privacy#seller-food-registration-notice");
		readNotice();
		expect(send.disabled).toBe(false);
		fireEvent.click(send);
		await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
		expect(mocks.submit).toHaveBeenCalledWith("A-1", NOTICE_ACCEPTANCES);
	});

	it("沒勾告知事項不能送出", () => {
		render(React.createElement(FoodRegSupplement, { seller: seller() }));
		fireEvent.change(screen.getByLabelText("食品業者登錄字號"), {
			target: { value: "A-1" },
		});
		const send = screen.getByRole("button", {
			name: "送出審核",
		}) as HTMLButtonElement;
		expect(send.disabled).toBe(true);
		fireEvent.click(send);
		expect(mocks.submit).not.toHaveBeenCalled();
	});

	it("告知事項的版本已不是現行：取消勾選並提示重新整理", async () => {
		mocks.submit.mockRejectedValueOnce(
			new ConnectError("文件已更新", Code.FailedPrecondition, undefined, [
				{
					desc: ErrorInfoSchema,
					value: {
						reason: ErrorReason[ErrorReason.AGREEMENT_OUTDATED],
						domain: "hawkr",
					},
				},
			]),
		);
		render(React.createElement(FoodRegSupplement, { seller: seller() }));
		fireEvent.change(screen.getByLabelText("食品業者登錄字號"), {
			target: { value: "A-1" },
		});
		readNotice();
		fireEvent.click(screen.getByRole("button", { name: "送出審核" }));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toBe(
				"條款已更新，請重新整理頁面後再勾選。",
			),
		);
		expect(
			(
				screen.getByRole("checkbox", {
					name: /食品業者登錄個人資料蒐集告知事項/,
				}) as HTMLInputElement
			).checked,
		).toBe(false);
	});

	it("字號欄位違規：優先顯示欄位訊息，不顯示整體訊息", async () => {
		mocks.submit.mockRejectedValueOnce(
			new ConnectError("資料有誤", Code.InvalidArgument, undefined, [
				{
					desc: BadRequestSchema,
					value: {
						fieldViolations: [
							{
								field: "registration_number",
								description: "required",
								localizedMessage: create(LocalizedMessageSchema, {
									locale: "zh-TW",
									message: "請輸入食品業者登錄字號",
								}),
							},
						],
					},
				},
			]),
		);
		render(React.createElement(FoodRegSupplement, { seller: seller() }));
		fireEvent.change(screen.getByLabelText("食品業者登錄字號"), {
			target: { value: "A-1" },
		});
		readNotice();
		fireEvent.click(screen.getByRole("button", { name: "送出審核" }));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toBe(
				"請輸入食品業者登錄字號",
			),
		);
	});

	it("被平台撤銷：顯示字號與理由，按鈕是「重新送出」", async () => {
		render(
			React.createElement(FoodRegSupplement, {
				seller: seller({
					revokedFoodRegistration: {
						registrationNumber: "A-9",
						endReason: "字號與食藥署資料不符",
					},
				}),
			}),
		);
		expect(
			screen.getByText(
				"已被平台撤銷：A-9。理由：字號與食藥署資料不符。請確認字號與食藥署登錄資料一致後重新送出審核。",
			),
		).toBeTruthy();
		fireEvent.change(screen.getByLabelText("食品業者登錄字號"), {
			target: { value: "A-10" },
		});
		readNotice();
		fireEvent.click(screen.getByRole("button", { name: "重新送出" }));
		await waitFor(() =>
			expect(mocks.submit).toHaveBeenCalledWith("A-10", NOTICE_ACCEPTANCES),
		);
	});

	// 撤銷的說明由伺服器決定要不要帶：撤銷時就已存在的同字號待審件（核准的 A 被撤銷、同字號的待審 B 還在），兩者伺服器都帶，
	// client 原樣顯示，不自己判斷哪一個該藏。
	it("伺服器同時帶撤銷與同字號的待審：兩個都顯示", () => {
		render(
			React.createElement(FoodRegSupplement, {
				seller: seller({
					revokedFoodRegistration: {
						registrationNumber: "A-9",
						endReason: "不符",
					},
					pendingFoodRegistration: {
						state: FoodRegistrationState.PENDING,
						registrationNumber: "A-9",
						submitTime: SUBMITTED,
					},
				}),
			}),
		);
		expect(screen.getByText(/已被平台撤銷：A-9。理由：不符。/)).toBeTruthy();
		expect(screen.getByText(/審核中：A-9/)).toBeTruthy();
	});

	it("伺服器同時帶撤銷與被退回的請求：兩個都顯示，按鈕是「重新送出」", () => {
		render(
			React.createElement(FoodRegSupplement, {
				seller: seller({
					revokedFoodRegistration: {
						registrationNumber: "A-9",
						endReason: "不符",
					},
					rejectedFoodRegistration: {
						state: FoodRegistrationState.REJECTED,
						registrationNumber: "A-10",
						rejectReason: "查無此字號",
						submitTime: SUBMITTED,
					},
				}),
			}),
		);
		expect(screen.getByText(/已被平台撤銷：A-9。/)).toBeTruthy();
		expect(screen.getByText(/未通過：查無此字號/)).toBeTruthy();
		expect(screen.getByRole("button", { name: "重新送出" })).toBeTruthy();
	});

	it("伺服器沒有帶撤銷：不顯示撤銷的說明，按鈕是「送出審核」", () => {
		render(React.createElement(FoodRegSupplement, { seller: seller() }));
		expect(screen.queryByText(/已被平台撤銷/)).toBeNull();
		expect(screen.getByRole("button", { name: "送出審核" })).toBeTruthy();
		// 兩個欄位都沒有：沒有審核中或未通過的狀態，也沒有撤回。
		expect(screen.queryByText(/審核中|未通過/)).toBeNull();
		expect(screen.queryByRole("button", { name: "撤回" })).toBeNull();
	});

	it("審核中：顯示字號與送出日期，認證未核准時多一句說明，可以撤回", async () => {
		const onSaved = vi.fn();
		render(
			React.createElement(FoodRegSupplement, {
				seller: seller({
					pendingFoodRegistration: {
						state: FoodRegistrationState.PENDING,
						registrationNumber: "A-2",
						submitTime: SUBMITTED,
					},
				}),
				kycApproved: false,
				onSaved,
			}),
		);
		expect(screen.getByText(/審核中：A-2（2026\/10\/15 送出）/)).toBeTruthy();
		expect(screen.getByText("平台會在商家認證通過後進行審核。")).toBeTruthy();
		expect(screen.queryByLabelText("食品業者登錄字號")).toBeNull();
		fireEvent.click(screen.getByRole("button", { name: "撤回" }));
		await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
		expect(mocks.cancel).toHaveBeenCalledOnce();
	});

	it("審核中且認證已核准：沒有認證說明", () => {
		render(
			React.createElement(FoodRegSupplement, {
				seller: seller({
					pendingFoodRegistration: {
						state: FoodRegistrationState.PENDING,
						registrationNumber: "A-2",
						submitTime: SUBMITTED,
					},
				}),
				kycApproved: true,
			}),
		);
		expect(screen.queryByText(/商家認證通過後進行審核/)).toBeNull();
	});

	it("未通過：顯示理由並可以重新送出", async () => {
		render(
			React.createElement(FoodRegSupplement, {
				seller: seller({
					rejectedFoodRegistration: {
						state: FoodRegistrationState.REJECTED,
						registrationNumber: "A-3",
						rejectReason: "字號查無資料",
					},
				}),
			}),
		);
		expect(
			screen.getByText(
				"未通過：字號查無資料。請確認字號與食藥署登錄資料一致後重新送出。",
			),
		).toBeTruthy();
		// 撤回只屬於待審的登錄：被退回的沒有撤回按鈕。
		expect(screen.queryByRole("button", { name: "撤回" })).toBeNull();
		fireEvent.change(screen.getByLabelText("食品業者登錄字號"), {
			target: { value: "A-4" },
		});
		readNotice();
		fireEvent.click(screen.getByRole("button", { name: "送出審核" }));
		await waitFor(() =>
			expect(mocks.submit).toHaveBeenCalledWith("A-4", NOTICE_ACCEPTANCES),
		);
	});

	it("已登錄：顯示字號，更換字號再次送出，清除前先確認", async () => {
		const onSaved = vi.fn();
		render(
			React.createElement(FoodRegSupplement, {
				seller: seller({
					approvedFoodRegistration: { registrationNumber: "A-5" },
				}),
				onSaved,
			}),
		);
		expect(screen.getByText("已登錄：A-5")).toBeTruthy();
		expect(screen.queryByLabelText("食品業者登錄字號")).toBeNull();

		fireEvent.click(screen.getByRole("button", { name: "更換字號" }));
		fireEvent.change(screen.getByLabelText("食品業者登錄字號"), {
			target: { value: "A-6" },
		});
		readNotice();
		fireEvent.click(screen.getByRole("button", { name: "送出審核" }));
		await waitFor(() =>
			expect(mocks.submit).toHaveBeenCalledWith("A-6", NOTICE_ACCEPTANCES),
		);

		fireEvent.click(screen.getByRole("button", { name: "清除" }));
		expect(
			screen.getByText(
				"清除後，店面與商品頁不再顯示食品業者登錄字號，也無法上架加工食品類商品。",
			),
		).toBeTruthy();
		expect(mocks.clear).not.toHaveBeenCalled();
		fireEvent.click(screen.getByRole("button", { name: "確認清除" }));
		await waitFor(() => expect(mocks.clear).toHaveBeenCalledOnce());
	});

	it("審核中的件不影響已登錄的顯示", () => {
		render(
			React.createElement(FoodRegSupplement, {
				seller: seller({
					approvedFoodRegistration: { registrationNumber: "A-5" },
					pendingFoodRegistration: {
						state: FoodRegistrationState.PENDING,
						registrationNumber: "A-7",
						submitTime: SUBMITTED,
					},
				}),
				kycApproved: true,
			}),
		);
		expect(screen.getByText("已登錄：A-5")).toBeTruthy();
		expect(screen.getByText(/審核中：A-7/)).toBeTruthy();
	});
});
