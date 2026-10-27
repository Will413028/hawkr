import { HttpResponse, http } from "msw";
import { describe, expect, it } from "vitest";
import { getEnv } from "@/lib/config/env";
import { server } from "@/tests/mocks/server";
import { confirmCheckout } from "./repo";

/**
 * `POST /app/checkout/confirm` 的 wire body。
 *
 * 少送或多送一個選填欄位在型別上完全合法 —— TS 不叫、lint 不叫、畫面正常，
 * 只有真的按下付款才會知道（同檔 `invoice` 欄位 2026-09-01 踩過的形狀）。
 */
const BASE = getEnv().serverApiBase;

const ITEMS = [{ skuId: "101", productId: "843856384063471755", quantity: 2 }];

/** 收下一次請求的 body，並回一個合法的成功 envelope。 */
function captureConfirmBody() {
	const seen: Array<Record<string, unknown>> = [];
	server.use(
		http.post(`${BASE}/app/checkout/confirm`, async ({ request }) => {
			seen.push((await request.json()) as Record<string, unknown>);
			return HttpResponse.json({
				code: 0,
				message: "success",
				data: { orderNos: ["O250912001"], totalPayAmount: "980" },
			});
		}),
	);
	return seen;
}

describe("confirm 的 wire body", () => {
	it("3001 的 confirm body 逐鍵固定", async () => {
		const seen = captureConfirmBody();

		await confirmCheckout({
			checkoutId: "chk-1",
			items: ITEMS,
			shippingMethod: "3001",
			deliveryTemp: "3202",
			receiverZipCode: "330",
			recipientName: "王小明",
			recipientPhone: "0912345678",
			shippingAddress: "桃園市",
		});

		expect(Object.keys(seen[0]).sort()).toEqual([
			"checkoutId",
			"deliveryTemp",
			"items",
			"orderRemark",
			"receiverZipCode",
			"recipientName",
			"recipientPhone",
			"shippingAddress",
			"shippingMethod",
			"shippingNote",
		]);
	});
});
