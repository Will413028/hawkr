// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	KYCField,
	KYCFlowSchema,
	KYCFlowStatus,
	PayUniState,
	SellerPayUniSchema,
	StoreField,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
	cert: {} as Record<string, unknown>,
}));

vi.mock("@/lib/seller/cert-gate", () => ({
	CERT_PAGE_HREF: "/seller-center/me/certifications",
	useCertGate: () => state.cert,
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "1", activeRole: "seller" }),
}));
vi.mock("@/lib/orders/repo", () => ({
	fetchOrders: async () => ({ records: [] }),
}));
vi.mock("@/features/seller-center", () => ({
	SellerCertGate: ({ status }: { status: string }) =>
		React.createElement("div", { "data-testid": "cert-gate" }, status),
}));
vi.mock(
	"@/features/seller-center/components/SellerMobileDisclosureList",
	() => ({
		SellerMobileDisclosureIcon: () => null,
		SellerMobileDisclosureList: () => null,
	}),
);
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({ children }: { children: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/Link", () => ({
	default: ({ children, href }: { children: React.ReactNode; href: string }) =>
		React.createElement("a", { href }, children),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		children,
		href,
	}: {
		children: React.ReactNode;
		href?: string;
	}) =>
		href
			? React.createElement("a", { href }, children)
			: React.createElement("button", { type: "button" }, children),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import WalletClient from "./WalletClient";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

function payuniOf(
	state: PayUniState,
	init: {
		explanation?: string;
		reason?: string;
		fixFields?: KYCField[];
		fixStoreFields?: StoreField[];
	} = {},
) {
	return create(SellerPayUniSchema, {
		state,
		explanation: init.explanation
			? create(LocalizedMessageSchema, {
					locale: "zh-TW",
					message: init.explanation,
				})
			: undefined,
		reason: init.reason ?? "",
		fixFields: init.fixFields ?? [],
		fixStoreFields: init.fixStoreFields ?? [],
	});
}

function mount() {
	return render(
		React.createElement(
			QueryClientProvider,
			{ client: new QueryClient() },
			React.createElement(WalletClient),
		),
	);
}

function setCert(
	status: string,
	flowStatus: KYCFlowStatus,
	canResubmit = false,
	payuni?: ReturnType<typeof payuniOf>,
) {
	state.cert = {
		loading: false,
		status,
		flowStatus,
		canResubmit,
		flow: create(KYCFlowSchema, { status: flowStatus, canResubmit, payuni }),
		seller: { capabilities: [{ capability: 2, active: true, missing: [] }] },
		refresh: () => {},
	};
}

afterEach(() => {
	cleanup();
});

describe("WalletClient 的 PayUni 狀態", () => {
	it("已核准：沒有面板、閘門，也沒有「可提領」「成交餘額」與刷新按鈕", () => {
		setCert(
			"approved",
			KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
			false,
			payuniOf(PayUniState.APPROVED),
		);
		mount();
		const text = document.body.textContent ?? "";
		expect(text).not.toMatch(/可提領|成交餘額/);
		expect(text).not.toMatch(/上次同步|重新整理狀態|同步失敗/);
		expect(screen.queryByTestId("cert-gate")).toBeNull();
		expect(screen.queryByRole("button", { name: /重新整理|同步/ })).toBeNull();
	});

	it("審核中：標題是目錄名詞、說明是伺服器的句子，沒有刷新按鈕與上次同步", () => {
		setCert(
			"forwarded",
			KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW,
			false,
			payuniOf(PayUniState.REVIEWING, { explanation: "伺服器獨特說明-審核中" }),
		);
		mount();
		expect(screen.getByText("PayUni 審核中")).toBeTruthy();
		expect(screen.getByText("伺服器獨特說明-審核中")).toBeTruthy();
		const text = document.body.textContent ?? "";
		expect(text).not.toMatch(/上次同步|重新整理狀態/);
		// 認證核准前不顯示「帳戶與認證」區塊，畫面上沒有任何按鈕。
		expect(screen.queryAllByRole("button")).toHaveLength(0);
	});

	it("PayUni 拒絕時整個錢包不會被認證閘取代；審核意見在 zh-TW 的元素裡，原碼不顯示", () => {
		setCert(
			"rejected",
			KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
			false,
			payuniOf(PayUniState.CONTACT_SUPPORT, {
				explanation: "伺服器獨特說明-客服",
				reason: "PayUni 的原文意見",
			}),
		);
		mount();
		expect(screen.queryByTestId("cert-gate")).toBeNull();
		expect(screen.getByText("PayUni 申請未通過")).toBeTruthy();
		expect(screen.getByText("伺服器獨特說明-客服")).toBeTruthy();
		expect(screen.getByText("PayUni 的審核意見")).toBeTruthy();
		const reason = screen.getByText("PayUni 的原文意見");
		expect(reason.closest("[lang]")?.getAttribute("lang")).toBe("zh-TW");
		expect(screen.queryByText("請修正以下資料")).toBeNull();
	});

	it("「前往補件」只跟 canResubmit：FIXABLE 但不能重送時不出現", () => {
		setCert(
			"rejected",
			KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
			false,
			payuniOf(PayUniState.FIXABLE),
		);
		mount();
		expect(screen.queryByRole("link", { name: "前往補件" })).toBeNull();
	});

	it("可自行重送：給補件入口，列出伺服器指出的欄位", () => {
		setCert(
			"rejected",
			KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
			true,
			payuniOf(PayUniState.FIXABLE, {
				fixFields: [KYCField.KYC_FIELD_BANK_NAME, KYCField.KYC_FIELD_BANK_CODE],
			}),
		);
		mount();
		expect(
			screen.getByRole("link", { name: "前往補件" }).getAttribute("href"),
		).toBe("/seller-center/me/certifications");
		expect(screen.getByText("請修正以下資料")).toBeTruthy();
		expect(screen.queryByRole("link", { name: "前往店家設定" })).toBeNull();
	});

	it("問題指向店家欄位：給店家設定的入口，只跟 fixStoreFields", () => {
		setCert(
			"rejected",
			KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
			false,
			payuniOf(PayUniState.FIX_VIA_SUPPORT, {
				fixStoreFields: [StoreField.SUPPORT_EMAIL],
			}),
		);
		mount();
		expect(
			screen.getByRole("link", { name: "前往店家設定" }).getAttribute("href"),
		).toBe("/seller-center/me");
		expect(document.body.textContent).toContain("客服電子信箱");
		expect(screen.queryByRole("link", { name: "前往補件" })).toBeNull();
	});

	it("MerID 終止：顯示終止狀態", () => {
		setCert(
			"rejected",
			KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
			false,
			payuniOf(PayUniState.TERMINATED),
		);
		mount();
		expect(screen.getByText("PayUni 收款帳戶已終止")).toBeTruthy();
	});

	it("不認得的狀態：標題是「未知」並顯示說明，入口照常讀 canResubmit", () => {
		setCert(
			"rejected",
			KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
			true,
			payuniOf(PayUniState.UNSPECIFIED, { explanation: "伺服器獨特說明-未知" }),
		);
		mount();
		expect(screen.getByText("未知")).toBeTruthy();
		expect(screen.getByText("伺服器獨特說明-未知")).toBeTruthy();
		expect(screen.getByRole("link", { name: "前往補件" })).toBeTruthy();
	});

	it("沒有 payuni、流程也還沒到 PayUni：顯示認證閘", () => {
		setCert("submitted", KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED);
		mount();
		expect(screen.getByTestId("cert-gate")).toBeTruthy();
	});

	it("商家認證核准前不顯示「帳戶與認證」", () => {
		setCert("submitted", KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED);
		mount();
		expect(screen.queryByText("帳戶與認證")).toBeNull();
		expect(screen.queryByText("商家認證")).toBeNull();
	});

	it("認證核准後顯示「商家認證」列", () => {
		setCert("approved", KYCFlowStatus.KYC_FLOW_STATUS_APPROVED);
		state.cert = { ...state.cert, approved: true };
		mount();
		expect(screen.getByText("商家認證")).toBeTruthy();
	});
});
