// @vitest-environment jsdom
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import { useBuyerQuoteComparison } from "./useBuyerQuoteComparison";

/**
 * 買家端報價比較與接受的**傳輸契約**。
 *
 * 元件行為在 `__tests__/BuyerQuoteComparison.test.ts`，這裡只釘住四件會在接線
 * 當天才看得出對錯的事：端點路徑、哪一支帶冪等鍵、body 有沒有夾帶金額、
 * 回應解析不過時要 fail closed 而不是只顯示解析得出來的那幾筆。
 */

const { getMock, putMock, deleteMock, postMock } = vi.hoisted(() => ({
	getMock: vi.fn(),
	putMock: vi.fn(),
	deleteMock: vi.fn(),
	postMock: vi.fn(),
}));

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			authed: {
				...actual.apiClient.authed,
				get: getMock,
				put: putMock,
				delete: deleteMock,
				post: postMock,
			},
		},
	};
});

const REQUEST_ID = "1234567890123456788";
const RESPONSE_ID = "1234567890123456789";
const VERSION_ID = "1234567890123456791";

function wireRow(overrides: Record<string, unknown> = {}) {
	return {
		sellerResponseId: RESPONSE_ID,
		sellerResponseVersion: VERSION_ID,
		status: "SUBMITTED",
		payloadHash: "a".repeat(64),
		submittedAt: "2026-09-13T02:00:00Z",
		seller: {
			displayName: "範例鮮果合作社",
			partyType: "PLATFORM_SELLER",
			storeCode: "greenfield",
		},
		quote: {
			responseType: "MANUAL_QUOTE",
			offeredQuality: { grade: "特級", inspectionNotes: "" },
			unitBasis: { quantity: "1", unit: "KG" },
			offeredQuantity: "500",
			offeredQuantityUnit: "KG",
			conversionBasis: "",
			currency: "TWD",
			goodsAmount: "18000",
			shippingAmount: "0",
			taxAmount: "900",
			taxInclusionBasis: "TAX_EXCLUDED",
			totalAmount: "18900",
			leadTimeDays: "3",
			promisedDeliveryAt: "",
			validUntil: "",
			termsNote: "",
			deviationDisclosure: "",
		},
		outsideBuyerRequestedRange: false,
		shortlisted: false,
		acceptability: "ACCEPTABLE",
		...overrides,
	};
}

function ackData(overrides: Record<string, unknown> = {}) {
	return {
		buyerAcceptanceId: "1234567890123456805",
		sellerResponseId: RESPONSE_ID,
		sellerResponseVersion: VERSION_ID,
		acceptedDealPrice: "18900",
		currency: "TWD",
		acceptedAt: "2026-09-13T03:00:00Z",
		orderProvisioningState: "CREATING_ORDER",
		orderNo: "",
		paymentDeadline: "",
		replayed: false,
		...overrides,
	};
}

afterEach(() => {
	vi.clearAllMocks();
});

async function mountLoaded(data: unknown = [wireRow()]) {
	getMock.mockResolvedValue({ code: 0, data });
	const hook = renderHook(() => useBuyerQuoteComparison(REQUEST_ID));
	await waitFor(() => {
		expect(hook.result.current.list.status).not.toBe("loading");
	});
	return hook;
}

describe("比較清單讀取", () => {
	it("打的是 seller-responses 端點", async () => {
		await mountLoaded();
		expect(getMock).toHaveBeenCalledWith(
			`/buyers/me/procurement/requests/${REQUEST_ID}/seller-responses`,
		);
	});

	it("解析不過時整份 fail closed，不呈現半份清單", async () => {
		const { result } = await mountLoaded([
			wireRow(),
			wireRow({ payloadHash: "nope" }),
		]);
		expect(result.current.list.status).toBe("error");
		if (result.current.list.status === "error") {
			expect(result.current.list.error.code).toBe("INVALID_RESPONSE");
		}
	});

	it("伺服器錯誤翻成畫面用的失敗，不外洩內部訊息", async () => {
		getMock.mockRejectedValue(
			new ApiClientError("upstream exploded", 503, 503, null, ""),
		);
		const { result } = renderHook(() => useBuyerQuoteComparison(REQUEST_ID));
		await waitFor(() => expect(result.current.list.status).toBe("error"));
		if (result.current.list.status === "error") {
			expect(result.current.list.error.code).toBe("SERVER_ERROR");
			expect(result.current.list.error.detail).not.toContain("upstream");
		}
	});
});

describe("加入與移出比較", () => {
	it("PUT 到帶版本的 shortlist 路徑，且不帶冪等鍵", async () => {
		const { result } = await mountLoaded();
		putMock.mockResolvedValue({ code: 0, data: {} });
		await result.current.setShortlisted(
			{ sellerResponseId: RESPONSE_ID, sellerResponseVersion: VERSION_ID },
			true,
		);
		expect(putMock).toHaveBeenCalledWith(
			`/buyers/me/procurement/requests/${REQUEST_ID}/shortlist/${RESPONSE_ID}/versions/${VERSION_ID}`,
			{},
		);
		expect(JSON.stringify(putMock.mock.calls[0])).not.toContain(
			"Idempotency-Key",
		);
	});

	it("移除走 DELETE，路徑同一條", async () => {
		const { result } = await mountLoaded();
		deleteMock.mockResolvedValue({ code: 0, data: {} });
		await result.current.setShortlisted(
			{ sellerResponseId: RESPONSE_ID, sellerResponseVersion: VERSION_ID },
			false,
		);
		expect(deleteMock).toHaveBeenCalledWith(
			`/buyers/me/procurement/requests/${REQUEST_ID}/shortlist/${RESPONSE_ID}/versions/${VERSION_ID}`,
			{},
		);
	});

	it("失敗時把那一列還原，不留下伺服器不承認的狀態", async () => {
		const { result } = await mountLoaded();
		putMock.mockRejectedValue(new ApiClientError("nope", 500, 500, null, ""));
		const outcome = await result.current.setShortlisted(
			{ sellerResponseId: RESPONSE_ID, sellerResponseVersion: VERSION_ID },
			true,
		);
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
	it("POST 到 buyer-quote-acceptances，冪等鍵走 header、body 只有三個 CAS 欄位", async () => {
		const { result } = await mountLoaded();
		postMock.mockResolvedValue({ code: 0, data: ackData() });
		await result.current.acceptQuote({
			idempotencyKey: "bqa-test",
			sellerResponseId: RESPONSE_ID,
			expectedSellerResponseVersion: VERSION_ID,
			expectedRfqSnapshotHash: "b".repeat(64),
		});
		const [path, options] = postMock.mock.calls[0];
		expect(path).toBe(
			`/buyers/me/procurement/requests/${REQUEST_ID}/buyer-quote-acceptances`,
		);
		expect(options.headers).toEqual({ "Idempotency-Key": "bqa-test" });
		expect(Object.keys(options.json).sort()).toEqual([
			"expectedRfqSnapshotHash",
			"expectedSellerResponseVersion",
			"sellerResponseId",
		]);
		expect(JSON.stringify(options.json)).not.toContain("Amount");
	});

	it("ACK 缺欄位時不宣稱已接受", async () => {
		const { result } = await mountLoaded();
		postMock.mockResolvedValue({
			code: 0,
			data: { buyerAcceptanceId: RESPONSE_ID },
		});
		const outcome = await result.current.acceptQuote({
			idempotencyKey: "bqa-test",
			sellerResponseId: RESPONSE_ID,
			expectedSellerResponseVersion: VERSION_ID,
			expectedRfqSnapshotHash: "b".repeat(64),
		});
		expect(outcome.ok).toBe(false);
		if (!outcome.ok) expect(outcome.error.code).toBe("INVALID_RESPONSE");
	});

	it("STALE_SELLER_RESPONSE 原樣傳給畫面，不在資料層自動改接受最新版", async () => {
		const { result } = await mountLoaded();
		postMock.mockRejectedValue(
			new ApiClientError(
				"seller response changed",
				409,
				409,
				null,
				"STALE_SELLER_RESPONSE",
			),
		);
		const outcome = await result.current.acceptQuote({
			idempotencyKey: "bqa-test",
			sellerResponseId: RESPONSE_ID,
			expectedSellerResponseVersion: VERSION_ID,
			expectedRfqSnapshotHash: "b".repeat(64),
		});
		expect(postMock).toHaveBeenCalledTimes(1);
		expect(outcome.ok).toBe(false);
		if (!outcome.ok) {
			expect(outcome.error.code).toBe("STALE_SELLER_RESPONSE");
		}
	});

	it("結帳中會把錯誤 payload 上的既有訂單編號帶出來", async () => {
		const { result } = await mountLoaded();
		postMock.mockRejectedValue(
			new ApiClientError(
				"checkout active",
				409,
				409,
				{ orderNo: "HK20260913001" },
				"RFQ_CHECKOUT_ALREADY_ACTIVE",
			),
		);
		const outcome = await result.current.acceptQuote({
			idempotencyKey: "bqa-test",
			sellerResponseId: RESPONSE_ID,
			expectedSellerResponseVersion: VERSION_ID,
			expectedRfqSnapshotHash: "b".repeat(64),
		});
		expect(outcome.ok).toBe(false);
		if (!outcome.ok) expect(outcome.error.orderNo).toBe("HK20260913001");
	});
});

describe("冪等鍵產生器", () => {
	it("每次一把新的，格式符合 Gateway 的字元與長度限制", async () => {
		const { result } = await mountLoaded();
		const first = result.current.newIdempotencyKey();
		const second = result.current.newIdempotencyKey();
		expect(first).not.toBe(second);
		for (const key of [first, second]) {
			expect(key).toMatch(/^[A-Za-z0-9._:-]+$/);
			expect(key.length).toBeGreaterThanOrEqual(8);
			expect(key.length).toBeLessThanOrEqual(128);
		}
	});
});
