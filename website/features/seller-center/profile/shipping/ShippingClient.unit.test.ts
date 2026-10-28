/** @vitest-environment jsdom */

import { create } from "@bufbuild/protobuf";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { PostalAddressSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/postal_address_pb";
import {
	GetMyStoreResponseSchema,
	KYCFlowSchema,
	KYCFlowStatus,
	PayUniState,
	SellerPayUniSchema,
	StoreSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ShippingOnboarding } from "@/lib/seller/shipping";

const h = vi.hoisted(() => ({
	certGate: {} as Record<string, unknown>,
	onboardings: {} as Record<string, unknown>,
	validateAddress: vi.fn(),
	submitHome: vi.fn(),
	submitCvs: vi.fn(),
}));

vi.mock("@/lib/seller/cert-gate", () => ({ useCertGate: () => h.certGate }));
vi.mock("@/lib/hooks/useShippingOnboardings", () => ({
	useShippingOnboardings: () => h.onboardings,
}));
vi.mock("@/lib/seller/shipping", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/seller/shipping")>()),
	validateAddress: h.validateAddress,
	submitHomeDeliveryOnboarding: h.submitHome,
	submitCvsOnboarding: h.submitCvs,
}));
// 閘門：用真的 kycFlowGate 算文字（SellerCertGate 本身只負責版面），讓測試看得到 ShippingClient 傳給它的 flow 與 subject。
vi.mock("@/features/seller-center", async () => {
	const { kycFlowGate } = await import("@/lib/seller/kyc-flow");
	const { uiTextFor } = await import("@/lib/i18n/ui-text");
	return {
		SellerCertGate: (p: { flow?: never; subject?: string }) => {
			const { copy } = kycFlowGate(uiTextFor("zh-TW"), p.flow);
			return React.createElement(
				"div",
				{ "data-testid": "cert-gate" },
				React.createElement("strong", null, copy.heading),
				React.createElement("p", null, copy.message(p.subject ?? "繼續")),
			);
		},
	};
});
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({ children }: { children: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
vi.mock("@/components/shared/TruckSpinner", () => ({
	TruckSpinner: () => null,
}));
vi.mock("@/lib/checkout/store-pickups", () => ({
	createStoreMapSession: vi.fn(),
	openStoreMapPopup: vi.fn(),
}));
vi.mock("next/image", () => ({
	default: () => null,
}));
vi.mock("@/features/seller/components/AddressAutocompleteField", () => ({
	AddressAutocompleteField: (p: {
		value: string;
		onChange: (v: string) => void;
		onManualEdit: () => void;
	}) =>
		React.createElement("input", {
			"aria-label": "路名門牌",
			value: p.value,
			onChange: (e: React.ChangeEvent<HTMLInputElement>) => {
				p.onChange(e.target.value);
				p.onManualEdit();
			},
		}),
}));
vi.mock("@/components/ui/Dropdown", () => {
	const Dropdown = (p: {
		label: string;
		value?: string;
		items: string[];
		onChange: (v: string) => void;
		disabled?: boolean;
	}) =>
		React.createElement(
			"select",
			{
				"aria-label": p.label,
				value: p.value ?? "",
				disabled: p.disabled,
				onChange: (e: React.ChangeEvent<HTMLSelectElement>) =>
					p.onChange(e.target.value),
			},
			React.createElement("option", { value: "" }, ""),
			...p.items.map((i) =>
				React.createElement("option", { key: i, value: i }, i),
			),
		);
	return { Dropdown, default: Dropdown };
});

import ShippingClient from "./ShippingClient";

function storeOf(townCode: string, contactName = "王小明") {
	return create(GetMyStoreResponseSchema, {
		store: create(StoreSchema, {
			contactName,
			contactMobile: "0912345678",
			address: create(PostalAddressSchema, {
				townCode,
				addressDetail: "重慶南路一段 122 號",
			}),
		}),
	});
}

function setData(opts: {
	rows?: ShippingOnboarding[];
	townCode?: string;
	contactName?: string;
}) {
	h.certGate = {
		loading: false,
		shippingGate: "open",
		flowStatus: undefined,
		seller: storeOf(opts.townCode ?? "63000050", opts.contactName),
	};
	h.onboardings = {
		rows: opts.rows ?? [],
		loaded: true,
		error: null,
		refresh: vi.fn(),
	};
}

function setGate(flow: ReturnType<typeof create<typeof KYCFlowSchema>>) {
	h.certGate = { loading: false, shippingGate: "certification", flow };
	h.onboardings = { rows: [], loaded: true, error: null, refresh: vi.fn() };
}

const inputs = (label: string) =>
	screen.getAllByLabelText<HTMLInputElement>(new RegExp(`^${label}`));
const input = (label: string) => inputs(label)[0];

beforeEach(() => {
	h.validateAddress.mockReset();
	h.submitHome.mockReset();
	h.submitCvs.mockReset();
});
afterEach(cleanup);

describe("ShippingClient prefill", () => {
	it("prefills the 黑貓 form from the store when there is no row", () => {
		setData({});
		render(React.createElement(ShippingClient));
		expect(input("寄件人姓名").value).toBe("王小明");
		expect(input("聯絡電話").value).toBe("0912345678");
		expect(input("縣市").value).toBe("台北市");
		expect(input("區域").value).toBe("中正區");
		expect(input("路名門牌").value).toBe("重慶南路一段 122 號");
	});

	it("does not overwrite unsent input when the rows are refetched", () => {
		setData({});
		const view = render(React.createElement(ShippingClient));
		fireEvent.change(input("寄件人姓名"), { target: { value: "手動輸入" } });
		fireEvent.change(input("路名門牌"), { target: { value: "新路 9 號" } });
		// 重新讀取：row 出現（含不同的寄件人值），元件重新渲染。
		setData({
			rows: [
				{
					shippingType: "3652",
					goodsType: "3201",
					payuniSubmitStatus: "4304",
					senderName: "紀錄人",
					senderPhone: "0987654321",
					senderCity: "高雄市",
					senderDistrict: "左營區",
					senderAddressDetail: "博愛二路 1 號",
				},
			],
		});
		view.rerender(React.createElement(ShippingClient));
		expect(input("寄件人姓名").value).toBe("手動輸入");
		expect(input("路名門牌").value).toBe("新路 9 號");
		expect(input("縣市").value).toBe("台北市");
	});
});

describe("ShippingClient initial values wait for this mount's data", () => {
	it("a cached empty row list with a refetch in flight does not freeze the store values; the refetched 4303 row is shown locked", () => {
		setData({});
		h.onboardings = { ...h.onboardings, loading: true };
		const view = render(React.createElement(ShippingClient));
		expect(screen.queryAllByLabelText(/^寄件人姓名/)).toHaveLength(0);

		setData({
			rows: [
				{
					shippingType: "3652",
					goodsType: "3201",
					payuniSubmitStatus: "4303",
					senderName: "紀錄人",
					senderPhone: "0987654321",
					senderCity: "高雄市",
					senderDistrict: "左營區",
					senderAddressDetail: "博愛二路 1 號",
				},
			],
		});
		view.rerender(React.createElement(ShippingClient));
		const name = input("寄件人姓名");
		expect(name.value).toBe("紀錄人");
		expect(name.disabled).toBe(true);
		expect(input("縣市").value).toBe("高雄市");
	});

	it("later refetches do not bring the skeleton back or reset the form", () => {
		setData({});
		const view = render(React.createElement(ShippingClient));
		fireEvent.change(input("寄件人姓名"), { target: { value: "手動輸入" } });
		h.onboardings = { ...h.onboardings, loading: true };
		view.rerender(React.createElement(ShippingClient));
		expect(input("寄件人姓名").value).toBe("手動輸入");
	});
});

describe("ShippingClient when the first read fails", () => {
	it("leaves the loading state, shows the error with a retry, and renders no forms", () => {
		setData({});
		const refresh = vi.fn();
		h.onboardings = {
			rows: [],
			loaded: false,
			loading: false,
			error: "讀取失敗",
			refresh,
		};
		render(React.createElement(ShippingClient));
		expect(screen.queryByText("載入中…")).toBeNull();
		expect(screen.getByRole("alert").textContent).toBe("讀取失敗");
		expect(screen.queryAllByLabelText(/^寄件人姓名/)).toHaveLength(0);
		fireEvent.click(screen.getByRole("button", { name: "重新載入" }));
		expect(refresh).toHaveBeenCalledTimes(1);
	});

	it("keeps showing cached rows when only a background refetch failed", () => {
		setData({});
		h.onboardings = { ...h.onboardings, error: "背景讀取失敗" };
		render(React.createElement(ShippingClient));
		expect(input("寄件人姓名").value).toBe("王小明");
		expect(screen.getByRole("alert").textContent).toBe("背景讀取失敗");
	});
});

describe("ShippingClient 黑貓 submit", () => {
	async function submitHome() {
		fireEvent.click(
			screen.getByRole("button", { name: /^(送出開通申請|重新嘗試開通)$/ }),
		);
	}

	it("derives senderPostalCode from the selected district (台西鄉)", async () => {
		setData({ townCode: "10009160" });
		h.validateAddress.mockResolvedValue({
			valid: true,
			county: "雲林縣",
			district: "臺西鄉",
		});
		h.submitHome.mockResolvedValue({});
		render(React.createElement(ShippingClient));
		await submitHome();
		await waitFor(() => expect(h.submitHome).toHaveBeenCalledTimes(1));
		expect(h.submitHome.mock.calls[0][0]).toMatchObject({
			senderCity: "雲林縣",
			senderDistrict: "台西鄉",
			senderPostalCode: "638",
		});
	});

	it("blocks submit with 請重新選擇行政區 when the district cannot be mapped", async () => {
		setData({
			rows: [
				{
					shippingType: "3652",
					payuniSubmitStatus: "4304",
					senderName: "紀錄人",
					senderPhone: "0987654321",
					senderCity: "台北市",
					senderDistrict: "舊的不存在區",
					senderAddressDetail: "某路 1 號",
				},
			],
		});
		render(React.createElement(ShippingClient));
		await submitHome();
		expect(await screen.findByText("請重新選擇行政區")).toBeTruthy();
		expect(h.validateAddress).not.toHaveBeenCalled();
		expect(h.submitHome).not.toHaveBeenCalled();
	});

	it("has no postal-code field", () => {
		setData({});
		render(React.createElement(ShippingClient));
		expect(screen.queryByLabelText("郵遞區號")).toBeNull();
	});

	it("shows the display-width name error inline for an overlong prefilled name", () => {
		setData({ contactName: "一二三四五六七八九十一二三四五六" }); // 16 個中文字 = 32 > 30
		render(React.createElement(ShippingClient));
		expect(screen.getByText(/寄件人姓名最多 30 個字/)).toBeTruthy();
	});
});

describe("ShippingClient 7-11 prefill", () => {
	it("prefills the sender from the store, then from the row", () => {
		setData({});
		const view = render(React.createElement(ShippingClient));
		const cvsName = inputs("寄件人姓名")[1];
		expect(cvsName.value).toBe("王小明");
		view.unmount();

		setData({
			rows: [
				{
					shippingType: "3651",
					goodsType: "3201",
					senderName: "常溫人",
					senderPhone: "0911111111",
				},
			],
		});
		render(React.createElement(ShippingClient));
		expect(inputs("寄件人姓名")[1].value).toBe("常溫人");
	});

	it("takes sender and lock from the same row: a locked row without sender shows the store", () => {
		setData({
			rows: [
				{
					shippingType: "3651",
					goodsType: "3201",
					payuniSubmitStatus: "4303",
				},
			],
		});
		render(React.createElement(ShippingClient));
		const name = inputs("寄件人姓名")[1];
		expect(name.value).toBe("王小明");
		expect(name.disabled).toBe(true);
	});
});

describe("ShippingClient 的認證閘門", () => {
	it("有 PayUni 狀態：標題是 PayUni 的標題、內容是伺服器的說明，不帶 subject", () => {
		setGate(
			create(KYCFlowSchema, {
				status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
				payuni: create(SellerPayUniSchema, {
					state: PayUniState.FIXABLE,
					explanation: create(LocalizedMessageSchema, {
						locale: "zh-TW",
						message: "伺服器獨特說明-閘門",
					}),
				}),
			}),
		);
		render(React.createElement(ShippingClient));
		const gate = screen.getByTestId("cert-gate");
		expect(gate.textContent).toContain("PayUni 需要補件");
		expect(gate.textContent).toContain("伺服器獨特說明-閘門");
		expect(gate.textContent).not.toContain("設定物流與退貨");
	});

	it("沒有 PayUni 狀態：照目錄的文字，內容帶 subject", () => {
		setGate(
			create(KYCFlowSchema, {
				status: KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
			}),
		);
		render(React.createElement(ShippingClient));
		const gate = screen.getByTestId("cert-gate");
		expect(gate.textContent).toContain("尚未通過商家認證");
		expect(gate.textContent).toContain("設定物流與退貨");
	});

	it("PayUni 階段卻沒有 payuni（契約被違反）：閘門是「未知」，不是空白", () => {
		setGate(
			create(KYCFlowSchema, {
				status: KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED,
			}),
		);
		render(React.createElement(ShippingClient));
		expect(screen.getByTestId("cert-gate").textContent).toContain("未知");
	});
});
