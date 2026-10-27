import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { canBuyerCancelOrder } from "@/lib/orders/repo";
import { STATUS_LABEL } from "@/lib/orders/types";

/**
 * #3018｜「或選擇取消」這句話必須對應到一顆真的存在、按下去真的會取消的鍵。
 *
 * ── 這個 bug 為什麼能活著 ────────────────────────────────────────────────
 * 取消鍵的閘門寫的是 `order.canCancel`，而 **gateway 的訂單 DTO 根本沒有那一欄**
 * （實查 `gateway/internal/handler/order.go` 的 `OrderDetailResponse` /
 * `OrderSummaryResponse`）。讀一個不存在的欄位在 TypeScript 是合法的（型別上是
 * `boolean | undefined`）、在執行期是 `undefined` ⇒ 恆 falsy ⇒ 那顆鍵**從來沒有
 * 出現過**，而畫面上的文案照樣寫著「可使用『再次付款』完成這筆訂單，或選擇取消」。
 * 全程 HTTP 200、零錯誤、lint／tsc／既有測試全綠。
 *
 * ⇒ 本檔守三件事，缺一都會讓同一個形狀復活：
 *   ① 判準與 order-service 一致（只有待付款可取消）
 *   ② 買家訂單詳情裡**沒有任何**按鈕的閘門是 `order.canCancel`
 *   ③ 文案與入口共存：說了「或選擇取消」，就要有一顆走 `cancelOrder` 的鍵
 */

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");
const stripComments = (source: string) =>
	source.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");

const DETAIL = "features/account/screens/orders/OrderDetailClient.tsx";

test("只有待付款訂單可由買家取消（對齊 order-service 的 cancelOrder）", () => {
	// order-service `OrderService.cancelOrder`：非 PENDING_PAYMENT 一律
	// InvalidStatusException("僅待付款訂單可取消")。
	assert.equal(canBuyerCancelOrder({ status: "pending_payment" }), true);

	// 對**全部**已知狀態逐一比對，不是只測兩三個：漏測的那個狀態一旦被放行，
	// 買家會拿到一句「僅待付款訂單可取消」而不知道自己做錯了什麼。
	for (const status of Object.keys(STATUS_LABEL)) {
		if (status === "pending_payment") continue;
		assert.equal(
			canBuyerCancelOrder({ status }),
			false,
			`狀態 ${status} 不該出現取消鍵——後端只接受 PENDING_PAYMENT`,
		);
	}

	// 沒有訂單時不得樂觀放行（載入中就渲染一顆鍵，按下去打的是空 orderNo）。
	assert.equal(canBuyerCancelOrder(null), false);
	assert.equal(canBuyerCancelOrder(undefined), false);
});

test("買家訂單詳情不得再用 order.canCancel 當閘門", () => {
	const source = stripComments(read(DETAIL));
	// 🔴 錨在「有沒有讀這個欄位」，不是「有沒有這顆鍵」：症狀是鍵消失，
	// 而消失的東西沒有字串可以 grep。
	assert.ok(
		!/\.canCancel\b/.test(source),
		"訂單詳情又讀了後端不存在的 canCancel ⇒ 取消鍵會靜默消失（#3018）",
	);
});

test("說了「或選擇取消」，就要有一顆真的會取消的鍵", () => {
	const source = stripComments(read(DETAIL));
	assert.ok(
		read(DETAIL).includes("或選擇取消"),
		"文案改了就把這一條一起改——本測試守的是文案與入口共存，不是那句話本身",
	);
	assert.ok(
		/canBuyerCancelOrder\(order\)\s*\?\s*\(?\s*<Button/.test(source) ||
			/canCancel\s*\?\s*\(?\s*<Button/.test(source),
		"待付款橫幅裡沒有以 canBuyerCancelOrder 閘門的取消鍵",
	);
	assert.ok(/onClick=\{onCancel\}/.test(source), "取消鍵沒有接上 onCancel");
	// `cancelOrder` 真的被呼叫——不是只有 import。
	assert.ok(
		/await cancelOrder\(order\.orderNo\)/.test(source),
		"handleCancel 沒有呼叫 cancelOrder ⇒ 按鈕按下去什麼都不會發生",
	);
});
