// @vitest-environment jsdom
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { StoreCardSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import {
	AcceptSellerQuoteResponseSchema,
	BuyerFacingSellerResponseSchema,
	ListSellerResponsesResponseSchema,
	OrderProvisioningState,
	QuoteAcceptability,
	SellerResponseType,
	TaxInclusionBasis,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useBuyerQuoteComparison } from "./useBuyerQuoteComparison";

/**
 * 買家端報價比較與接受的傳輸契約。元件行為在 `__tests__/BuyerQuoteComparison.test.ts`，
 * 這裡釘住請求形狀（哪一支帶 client_request_id、有沒有夾帶金額）與回應轉換。
 */

const rpc = vi.hoisted(() => ({
	handlers: {} as Record<string, (request: never) => unknown>,
}));

vi.mock("@/lib/connect/procurement", async () => {
	const { fakeProcurementClient } = await import("./testing/fake-procurement");
	return { buyerProcurementClient: () => fakeProcurementClient(rpc.handlers) };
});
vi.mock("@/lib/i18n/ui-text", async () => {
	const { uiTextStub } = await import("./testing/fake-procurement");
	return uiTextStub;
});

const REQUEST_ID = "req-1";
const PUBLICATION_ID = "pub-1";
const RESPONSE_ID = "resp-1";
const VERSION_ID = "ver-1";

function protoRow(init: { shortlisted?: boolean; payloadHash?: string } = {}) {
	return create(BuyerFacingSellerResponseSchema, {
		sellerResponseId: RESPONSE_ID,
		sellerResponseVersion: VERSION_ID,
		payloadHash: init.payloadHash ?? "a".repeat(64),
		submitTime: timestampFromDate(new Date("2026-09-13T02:00:00Z")),
		store: create(StoreCardSchema, {
			id: "s1",
			storeCode: "greenfield",
			name: "範例鮮果合作社",
		}),
		quote: {
			responseType: SellerResponseType.MANUAL_QUOTE,
			specNote: "抽驗三箱",
			unitBasis: { quantity: "1", unit: "KG" },
			offeredQuantity: "500",
			offeredQuantityUnit: "KG",
			goodsAmount: BigInt(18000),
			shippingAmount: BigInt(0),
			taxAmount: BigInt(900),
			taxInclusionBasis: TaxInclusionBasis.TAX_EXCLUDED,
			totalAmount: BigInt(18900),
		},
		shortlisted: init.shortlisted ?? false,
		acceptability: QuoteAcceptability.ACCEPTABLE,
	});
}

afterEach(() => {
	rpc.handlers = {};
	vi.clearAllMocks();
});

async function mountLoaded(rows = [protoRow()]) {
	rpc.handlers.listSellerResponses = vi.fn(() =>
		create(ListSellerResponsesResponseSchema, { sellerResponses: rows }),
	);
	const hook = renderHook(() =>
		useBuyerQuoteComparison(REQUEST_ID, PUBLICATION_ID),
	);
	await waitFor(() => {
		expect(hook.result.current.list.status).not.toBe("loading");
	});
	return hook;
}

const target = {
	sellerResponseId: RESPONSE_ID,
	sellerResponseVersion: VERSION_ID,
};

describe("比較清單讀取", () => {
	it("以需求與發布讀清單，並把金額與店家轉成畫面模型", async () => {
		const { result } = await mountLoaded();
		const list = result.current.list;
		expect(list.status).toBe("success");
		if (list.status !== "success") return;
		expect(list.responses[0]).toMatchObject({
			sellerResponseId: RESPONSE_ID,
			seller: { displayName: "範例鮮果合作社", storeCode: "greenfield" },
			quote: {
				totalAmount: "18900",
				specNote: "抽驗三箱",
				taxInclusionBasis: "TAX_EXCLUDED",
				leadTimeDays: "",
			},
			acceptability: "ACCEPTABLE",
		});
		const handler = rpc.handlers.listSellerResponses as ReturnType<
			typeof vi.fn
		>;
		expect(handler.mock.calls[0][0]).toMatchObject({
			requestId: REQUEST_ID,
			publicationId: PUBLICATION_ID,
		});
	});

	it("沒有發布編號時不發請求，回「目前沒有可比較的詢價」", async () => {
		const handler = vi.fn();
		rpc.handlers.listSellerResponses = handler;
		const { result } = renderHook(() =>
			useBuyerQuoteComparison(REQUEST_ID, null),
		);
		await waitFor(() => expect(result.current.list.status).toBe("error"));
		expect(handler).not.toHaveBeenCalled();
	});

	it("伺服器錯誤翻成畫面用的失敗，文字取預設文字而不是內部訊息", async () => {
		rpc.handlers.listSellerResponses = () => {
			throw new ConnectError("upstream exploded", Code.Unavailable);
		};
		const { result } = renderHook(() =>
			useBuyerQuoteComparison(REQUEST_ID, PUBLICATION_ID),
		);
		await waitFor(() => expect(result.current.list.status).toBe("error"));
		if (result.current.list.status === "error") {
			expect(result.current.list.error.code).toBe("SERVER_ERROR");
			expect(result.current.list.error.detail).not.toContain("upstream");
		}
	});
});

describe("加入與移出比較", () => {
	it("加入帶需求與版本，不帶 client_request_id", async () => {
		const { result } = await mountLoaded();
		const put = vi.fn((_request: Record<string, unknown>) => ({}));
		rpc.handlers.putShortlistEntry = put;
		await result.current.setShortlisted(target, true);
		expect(put.mock.calls[0][0]).toMatchObject({
			requestId: REQUEST_ID,
			...target,
		});
		expect(put.mock.calls[0][0]).not.toHaveProperty("clientRequestId");
	});

	it("移除走 deleteShortlistEntry", async () => {
		const { result } = await mountLoaded([protoRow({ shortlisted: true })]);
		const del = vi.fn(() => ({}));
		rpc.handlers.deleteShortlistEntry = del;
		await result.current.setShortlisted(target, false);
		expect(del).toHaveBeenCalledTimes(1);
	});

	it("失敗時把那一列還原，不留下伺服器不承認的狀態", async () => {
		const { result } = await mountLoaded();
		rpc.handlers.putShortlistEntry = () => {
			throw new ConnectError("nope", Code.Internal);
		};
		const outcome = await result.current.setShortlisted(target, true);
		expect(outcome.ok).toBe(false);
		await waitFor(() => {
			const list = result.current.list;
			expect(list.status === "success" && list.responses[0].shortlisted).toBe(
				false,
			);
		});
	});
});

describe("接受一個 exact version", () => {
	const command = {
		clientRequestId: "6f1d3c1e-2a40-4a5e-9d0b-1f2e3d4c5b6a",
		sellerResponseId: RESPONSE_ID,
		expectedSellerResponseVersion: VERSION_ID,
		expectedSellerFacingHash: "b".repeat(64),
	};

	it("請求只有版本、內容雜湊與 client_request_id，不夾帶金額", async () => {
		const { result } = await mountLoaded();
		const accept = vi.fn((_request: Record<string, unknown>) =>
			create(AcceptSellerQuoteResponseSchema, {
				buyerAcceptanceId: "acc-1",
				sellerResponseId: RESPONSE_ID,
				sellerResponseVersion: VERSION_ID,
				acceptedDealPrice: BigInt(18900),
				orderProvisioningState: OrderProvisioningState.CREATING_ORDER,
			}),
		);
		rpc.handlers.acceptSellerQuote = accept;
		const outcome = await result.current.acceptQuote(command);
		const request = accept.mock.calls[0][0] as Record<string, unknown>;
		expect(request).toMatchObject({
			requestId: REQUEST_ID,
			clientRequestId: command.clientRequestId,
			expectedSellerFacingHash: command.expectedSellerFacingHash,
		});
		expect(Object.keys(request).filter((k) => /amount|price/i.test(k))).toEqual(
			[],
		);
		expect(outcome.ok).toBe(true);
		if (outcome.ok) {
			expect(outcome.data).toMatchObject({
				acceptedDealPrice: "18900",
				orderProvisioningState: "CREATING_ORDER",
				orderNo: "",
				paymentDeadline: "",
			});
		}
	});

	it("版本衝突原樣傳給畫面，不在資料層自動改接受最新版", async () => {
		const { result } = await mountLoaded();
		const accept = vi.fn(() => {
			throw new ConnectError("conflict", Code.Aborted);
		});
		rpc.handlers.acceptSellerQuote = accept;
		const outcome = await result.current.acceptQuote(command);
		expect(accept).toHaveBeenCalledTimes(1);
		expect(outcome.ok).toBe(false);
	});
});

describe("client_request_id 產生器", () => {
	it("每次一個新的 UUID", async () => {
		const { result } = await mountLoaded();
		const first = result.current.newClientRequestId();
		const second = result.current.newClientRequestId();
		expect(first).not.toBe(second);
		expect(first).toMatch(/^[0-9a-f-]{36}$/);
	});
});
