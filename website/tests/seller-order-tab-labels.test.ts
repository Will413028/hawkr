/**
 * 賣家工作台四桶的顯示名契約（後端 03 §2.1 SSOT）。
 *
 * 這支測試守的不是文案品味，是**桶的成員與桶的名字要對得上**：第三桶含
 * `cancelled`（order.status 80）與 `refunded`（70），所以它不能叫「已完成」——
 * 那會讓賣家把它讀成成交數。dev 實測同一家店 81 筆時這張卡顯示 75，其中只有
 * 19 筆真的成交、56 筆是已取消。
 *
 * 「已完成」在本專案是 CodeMeta `OrderStatus` 50 **單一狀態**的 Label，只在按狀態
 * 逐一計數的場景（代行工作區「賣家總覽」的訂單卡）才成立，那一側刻意不動。
 *
 * 對應的另兩支：
 *   - backend `OrderServiceSellerStatusCountsTest`（釘桶含 80/70）
 *   - seller App `test/features/order/data/order_tab_mapper_test.dart`（釘 App 標籤）
 */

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	SELLER_ORDER_TAB_LABEL,
	type SellerOrderTabKey,
} from "../features/seller-center/orders/orderTabs";

/** CodeMeta OrderStatus 50 的 Label — 只有「按單一狀態計數」才可以用這個詞。 */
const STATUS_50_LABEL = "已完成";

test("第三桶不得叫「已完成」——它含已取消/已退款，那個詞是 OrderStatus 50 專屬", () => {
	assert.notEqual(
		SELLER_ORDER_TAB_LABEL.completed,
		STATUS_50_LABEL,
		"第三桶含 cancelled(80)/refunded(70)，叫「已完成」會被讀成成交數",
	);
	assert.equal(SELLER_ORDER_TAB_LABEL.completed, "已結束");
});

test("四個桶各有標籤且互不重複（重複＝畫面上兩張卡同名，無法分辨）", () => {
	const keys: ReadonlyArray<SellerOrderTabKey> = [
		"pending",
		"in_transit",
		"completed",
		"aftersale",
	];
	for (const key of keys) {
		assert.ok(SELLER_ORDER_TAB_LABEL[key]?.length > 0, `${key} 缺少顯示名`);
	}
	const labels = keys.map((k) => SELLER_ORDER_TAB_LABEL[k]);
	assert.equal(new Set(labels).size, keys.length, "桶的顯示名重複");
});

/**
 * 說明文字裡的分頁清單也要跟著改名。
 *
 * 上一輪改了 `SELLER_ORDER_TAB_LABEL`，但**同一組分頁在
 * 另外幾處還有手抄本**，其中 `lib/content/seller-guide-data.ts` 是 `/seller-guide`
 * 這個活的賣家頁 —— 改了名字卻沒回灌到引用它的每一處，等於一邊的畫面寫「已結束」、
 * 另一邊的說明書教賣家去找「已完成」。這一條就是把那個回灌變成機械檢查。
 *
 * 判準只認**分頁列舉**（`待出貨 … 配送中 … 第三格`），不是「這一行有沒有出現
 * 已完成」——後者會誤判三種正當用法：買家訂單頁的分頁真的有「已完成」
 * （`docs/content-copy-audit.md`）、狀態流裡的
 * `50 已完成`（`docs/transaction-policy.md`），以及本輪刻意加的括號說明
 * （「已結束」＝…含已完成…）。
 */
const TAB_PROSE_COPIES = [
	"lib/content/seller-guide-data.ts",
	"features/seller/landing/components/SellerShowcaseCarousel.tsx",
] as const;

/**
 * `待出貨` → `配送中` → 第三格；分隔符涵蓋 ASCII `/`、全角 `／` 與頓號 `、`。
 *
 * 第三格只吃 2–6 個中文／英數字元：招募頁那句是「…、已結束清楚分流，出貨…」，
 * 標籤後面直接接下去沒有分隔符，所以用**定長取樣＋`startsWith`** 而不是取到分隔符
 * （後者會把整句話都當成標籤，斷言訊息也會變成一整行）。
 */
const TAB_ENUMERATION =
	/待出貨\s*[/／、]\s*配送中\s*[/／、]\s*([一-鿿A-Za-z0-9]{2,6})/g;

test("說明文字列舉分頁時，第三格必須是 SELLER_ORDER_TAB_LABEL.completed", () => {
	const root = fileURLToPath(new URL("..", import.meta.url));
	for (const relativePath of TAB_PROSE_COPIES) {
		const text = readFileSync(join(root, relativePath), "utf8");
		const slots = [...text.matchAll(TAB_ENUMERATION)].map((m) => m[1]);
		// 鑑別力：這一份手抄本必須真的還在列舉分頁。改寫成完全不列舉時要轉紅，
		// 而不是「零命中 ⇒ 通過」——那正是這條要防的失明。
		assert.ok(
			slots.length > 0,
			`${relativePath} 找不到分頁列舉；若它真的不再列舉，請從 TAB_PROSE_COPIES 移除並說明`,
		);
		for (const slot of slots) {
			assert.ok(
				slot.startsWith(SELLER_ORDER_TAB_LABEL.completed),
				`${relativePath}：分頁列舉的第三格是「${slot}」，應為「${SELLER_ORDER_TAB_LABEL.completed}」`,
			);
		}
	}
});
