import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 履約禁區的第一層在**前端**只有一個機械保證：代行訂單面的整條 import 鏈上不存在
 * 出貨模組（56 號 §8.1.1 層 1、mockup B19）。
 *
 * 設計書要求代行訂單頁「不渲染 row checkbox、批次列與任何出貨 CTA」，而且明說
 * **不是 `disabled`**。用一個 `if` 去關掉那些元素的話，禁區就退化成一個布林值：
 * 算錯、或有人日後在另一個分支多畫一顆按鈕，畫面就會出現一個打不到端點的出貨鍵，
 * 而沒有任何測試會轉紅。這一支測試把「不存在」變成可執行的斷言。
 *
 * ⚠️ **雙向都驗，只驗一邊等於沒驗**：代行側零命中的斷言，在「這些字串其實已經改名
 * 了」的情況下會空過。所以每一條禁令都同時斷言**賣家自己的頁面必須命中** —— 那一頁
 * 是本案唯一不准動的東西，它一旦不再命中，代表禁令的措辭已經與真實程式碼脫節。
 */

// fileURLToPath 而不是 .pathname：後者會把路徑裡的空白留成 %20，於是在
// 「Hawkr Web」這種含空白的 checkout 路徑下 walk() 全部讀不到檔案，
// delegatedFiles 變成空陣列——整份禁令看起來是紅的，其實是路徑解錯。
// CI 的 checkout 路徑沒有空白，所以只在本機炸。
const ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const DELEGATED_ORDER_DIRS = [
	"features/delegated-operations/orders",
	"app/(delegated)/adm/[storeId]/orders",
	"app/(delegated)/adm/api/orders",
];
const SELLER_ORDERS_PAGE = "features/seller-center/orders/SellerOrdersPage.tsx";
const SELLER_ORDERS_HOOK = "lib/hooks/useOrders.ts";

/**
 * 註解要先剝掉再比對：本目錄的檔頭刻意寫出 `nextShipmentAction`／`ACTION_LABEL`
 * 這些名字來解釋「為什麼不沿用那一頁」，那是說明不是程式碼。剝法與
 * `scripts/check-no-emoji.ts` 同一支（區塊註解 ＋ 整行雙斜線）。JSX 註解也吃得到，
 * 因為它的內層就是一個區塊註解。
 */
function stripComments(source: string): string {
	return source
		.replace(/\/\*[\s\S]*?\*\//g, "")
		.split("\n")
		.filter((line) => !line.trimStart().startsWith("//"))
		.join("\n");
}

function walk(dir: string): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(join(ROOT, dir));
	} catch {
		return out;
	}
	for (const entry of entries) {
		const rel = join(dir, entry);
		if (statSync(join(ROOT, rel)).isDirectory()) out.push(...walk(rel));
		else if (/\.tsx?$/.test(entry) && !entry.endsWith(".unit.test.ts")) {
			out.push(rel);
		}
	}
	return out;
}

function read(file: string): string {
	return stripComments(readFileSync(join(ROOT, file), "utf8"));
}

const delegatedFiles = DELEGATED_ORDER_DIRS.flatMap((dir) => walk(dir));

/** 出貨／履約與「代行下不掛」的資料層。命中任一個都代表禁區被接回來了。 */
const FORBIDDEN_IMPORT_TOKENS = [
	"lib/seller/shipment",
	"lib/seller/shipping",
	"lib/shipping/anomalies",
	"lib/after-sales",
	"shipmentOperationStore",
	"useSellerShipmentAnomalies",
	"useSellerDashboard",
	"useSellerWorkspaceRealtime",
	"useSellerAfterSales",
	"useSellerSelf",
	"useQueryIdentity",
	// 賣家訂單資料層：它打的是 /app/orders，代行必須走 /adm/api/orders/{storeId}。
	"lib/orders/repo",
];

/** 這些一定要在賣家頁上命中，否則上面的清單只是一串沒有意義的字串。 */
const SELLER_PAGE_CONTROL_TOKENS = [
	"lib/seller/shipment",
	"lib/shipping/anomalies",
	"shipmentOperationStore",
	"useSellerShipmentAnomalies",
	"useSellerDashboard",
	"useSellerAfterSales",
	"lib/orders/repo",
];

/** 即時同步的訂閱者掛在賣家中心外框（代行用的是 `DelegatedWorkspaceFrame`，不會接回）。 */
const SELLER_SHELL = "features/seller-center/components/SellerCenterShell.tsx";

/** 畫面上的履約元素。`批次` 只出現在商品頁，訂單頁一顆都不准有。 */
const FORBIDDEN_MARKUP_TOKENS = [
	'type="checkbox"',
	"批次",
	"預約出貨",
	"呼叫黑貓取件",
	"下載託運單 PDF",
	"預約取號",
	"出貨確認",
];

describe("代行訂單面：履約禁區", () => {
	it("掃描範圍不是空的（否則底下每一條都會空過）", () => {
		expect(delegatedFiles.length).toBeGreaterThanOrEqual(6);
	});

	for (const token of FORBIDDEN_IMPORT_TOKENS) {
		it(`代行訂單面不 import ${token}`, () => {
			const offenders = delegatedFiles.filter((file) =>
				read(file).includes(token),
			);
			expect(offenders).toEqual([]);
		});
	}

	for (const token of SELLER_PAGE_CONTROL_TOKENS) {
		it(`鑑別力：賣家訂單頁本來就有 ${token}`, () => {
			expect(read(SELLER_ORDERS_PAGE)).toContain(token);
		});
	}

	it("鑑別力：賣家中心外框本來就有 useSellerWorkspaceRealtime", () => {
		expect(read(SELLER_SHELL)).toContain("useSellerWorkspaceRealtime");
	});

	for (const token of FORBIDDEN_MARKUP_TOKENS) {
		it(`代行訂單面不渲染 ${token}`, () => {
			const offenders = delegatedFiles.filter((file) =>
				read(file).includes(token),
			);
			expect(offenders).toEqual([]);
		});
	}

	it("鑑別力：賣家訂單頁本來就有 checkbox 與批次列", () => {
		const seller = read(SELLER_ORDERS_PAGE);
		for (const token of FORBIDDEN_MARKUP_TOKENS) {
			expect(seller).toContain(token);
		}
	});

	it("代行訂單面只有 GET：沒有任何寫入方法", () => {
		const offenders = delegatedFiles.filter((file) =>
			/method:\s*"(POST|PUT|PATCH|DELETE)"/.test(read(file)),
		);
		expect(offenders).toEqual([]);
	});

	it("鑑別力：同一個判定式在商品的 publish route 上會命中", () => {
		const publishRoute =
			"app/(delegated)/adm/api/products/[storeId]/[productId]/publish/route.ts";
		expect(/method:\s*"(POST|PUT|PATCH|DELETE)"/.test(read(publishRoute))).toBe(
			true,
		);
	});
});

describe("代行訂單面：快取鍵隔離（定案第 4 條）", () => {
	const FORBIDDEN_KEY_TOKENS = ["queryKeys.seller.", "queryKeys.buyer."];

	for (const token of FORBIDDEN_KEY_TOKENS) {
		it(`不使用 ${token}`, () => {
			const offenders = delegatedFiles.filter((file) =>
				read(file).includes(token),
			);
			expect(offenders).toEqual([]);
		});
	}

	it("鑑別力：賣家訂單 hook 本來就用登入身分的鍵", () => {
		const hook = read(SELLER_ORDERS_HOOK);
		expect(hook).toContain("queryKeys.buyer.");
		expect(hook).toContain("useQueryIdentity");
	});

	it("代行訂單面用的是 delegated 鍵家族", () => {
		const usesDelegatedKeys = delegatedFiles.some((file) =>
			read(file).includes("queryKeys.delegated."),
		);
		expect(usesDelegatedKeys).toBe(true);
	});
});
