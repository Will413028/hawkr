// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import {
	KYCApplicationSchema,
	KYCStatus,
	PaymentAccountSchema,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/ui/Button", () => ({
	default: ({ children, href }: { children: React.ReactNode; href?: string }) =>
		React.createElement("a", { href }, children),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import { SellerCertGate } from "./SellerCertGate";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

afterEach(cleanup);

describe("SellerCertGate", () => {
	it("沒有申請：未送出，動作去商家認證頁，內容帶 subject", () => {
		render(React.createElement(SellerCertGate, { subject: "設定物流與退貨" }));
		expect(screen.getByText("尚未通過商家認證")).toBeTruthy();
		expect(screen.getByText(/設定物流與退貨/)).toBeTruthy();
		expect(screen.getByText("目前狀態：未送出")).toBeTruthy();
		expect(
			screen.getByText("前往商家認證").closest("a")?.getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});

	it("平台審核中：狀態是目錄的徽章文字", () => {
		render(
			React.createElement(SellerCertGate, {
				application: create(KYCApplicationSchema, {
					status: KYCStatus.KYC_STATUS_SUBMITTED,
				}),
			}),
		);
		expect(screen.getByText("目前狀態：審核中")).toBeTruthy();
	});

	it("平台審核核准、收款帳戶待設定：說明與動作是收款帳戶的目錄文字", () => {
		render(
			React.createElement(SellerCertGate, {
				application: create(KYCApplicationSchema, {
					status: KYCStatus.KYC_STATUS_APPROVED,
				}),
				paymentAccount: create(PaymentAccountSchema, {
					status: PaymentAccountStatus.ACTION_REQUIRED,
				}),
			}),
		);
		expect(screen.getByText("目前狀態：待設定")).toBeTruthy();
		expect(
			screen.getByText(
				"提供身分與銀行資料後即可開始收款。資料由金流商收集，平台不經手。",
			),
		).toBeTruthy();
		expect(screen.getByText("前往設定收款帳戶")).toBeTruthy();
		expect(document.body.textContent).not.toMatch(/逾期|停用/);
	});

	it("收款帳戶無法啟用：動作是客服連結", () => {
		render(
			React.createElement(SellerCertGate, {
				application: create(KYCApplicationSchema, {
					status: KYCStatus.KYC_STATUS_APPROVED,
				}),
				paymentAccount: create(PaymentAccountSchema, {
					status: PaymentAccountStatus.DISABLED,
				}),
			}),
		);
		expect(
			screen.getByText("聯絡客服").closest("a")?.getAttribute("href"),
		).toBe("/contact");
	});

	it("收款帳戶準備中：不顯示「重新整理」，動作去商家認證頁", () => {
		render(
			React.createElement(SellerCertGate, {
				application: create(KYCApplicationSchema, {
					status: KYCStatus.KYC_STATUS_APPROVED,
				}),
				paymentAccount: create(PaymentAccountSchema, {
					status: PaymentAccountStatus.NOT_CREATED,
				}),
			}),
		);
		expect(screen.queryByText("重新整理")).toBeNull();
		expect(
			screen.getByText("前往商家認證").closest("a")?.getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});
});
