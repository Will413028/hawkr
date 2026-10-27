import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

/**
 * 三個認證輸入面收斂成一份共用元件（19 號設計書 §14.2）。
 *
 * 這一組守的是型別與純函式測試看不到的三件事：
 * ① **只有一份實作**。收斂之前，商品表單與匯入編輯各帶一份逐字相同的 `CertRow`，
 *    代行工作區又另有一套；「認證現在算什麼狀態」因此有三個各自演化的答案。
 * ② **每個面都宣告了自己的 mode**，而 mode 決定它能讀哪個資料來源。漏宣告不會編譯錯，
 *    但會讓草稿宣稱被標成已通過（本次要修掉的 bug 之一）。
 * ③ **代行那一面不做即時驗證**：它的瀏覽器在 www 上沒有賣家登入態，拿不到申請清單，
 *    也不得呼叫任何認證端點。
 *
 * ⚠️ 以「整段元素」而不是全檔 `includes` 比對 mode：全檔搜尋在「兩個 `ProductCertificationRow`
 * 只有一個掛了 mode」的情況下照樣是綠的。
 */

const read = (relativePath: string): string =>
	readFileSync(new URL(relativePath, import.meta.url), "utf8");

const SHARED = "../../../components/shared/ProductCertificationRow.tsx";

const shared = read(SHARED);
const sellerForm = read("./components/ProductFormSections.tsx");
const sellerHook = read("./hooks/useProductForm.ts");
const importForm = read("../products-import/components/ImportItemEditForm.tsx");
const importHook = read("../products-import/hooks/useImportItemForm.ts");
const delegatedEditor = read(
	"../../delegated-operations/products/CertificationEditor.tsx",
);

/**
 * 逐個吐出某個 JSX 元素的完整原始碼。以「自成一行的 `/>`」當結束記號：屬性裡的箭頭
 * 函式含 `>`，用第一個 `>` 當結束會把元素切一半，於是後半段的屬性永遠找不到、斷言恆綠。
 */
function elements(source: string, name: string): string[] {
	return [
		...source.matchAll(new RegExp(`<${name}[\\s\\S]*?\\n\\s*/>`, "g")),
	].map((m) => m[0]);
}

describe("認證列只有一份實作", () => {
	it("三個面都不再自帶 CertRow copy", () => {
		for (const [surface, source] of [
			["seller product form", sellerForm],
			["seller import form", importForm],
			["delegated editor", delegatedEditor],
		] as const) {
			expect(source, surface).not.toContain("function CertRow(");
		}
	});

	it("商品表單與匯入編輯都掛共用元件（鑑別力：解析真的抓得到）", () => {
		for (const [surface, source] of [
			["seller product form", sellerForm],
			["seller import form", importForm],
		] as const) {
			expect(source, surface).toContain(
				'from "@/components/shared/ProductCertificationRow"',
			);
			expect(elements(source, "ProductCertificationRow"), surface).toHaveLength(
				1,
			);
		}
	});

	it("每一個共用元件的使用點都宣告了 mode", () => {
		for (const [surface, source] of [
			["seller product form", sellerForm],
			["seller import form", importForm],
		] as const) {
			for (const tag of elements(source, "ProductCertificationRow")) {
				expect(tag, surface).toContain("mode=");
			}
		}
	});
});

describe("共用元件自己不打端點", () => {
	it("不 import 傳輸層，也不呼叫任何認證端點函式", () => {
		// 送出與撤回一律由宿主注入：只有宿主知道自己有沒有 productId、有沒有登入態。
		// 這一條擋的是「日後有人為了方便在元件裡直接 fetch」——那會讓代行那一面在
		// 沒有登入態的瀏覽器上開始打 /app 端點。
		expect(shared).not.toContain('from "@/lib/api/client"');
		expect(shared).not.toContain("authedFetch");
		expect(shared).not.toContain("verifyProductCertification");
		expect(shared).not.toContain("fetchProductCertificationApplications");
		expect(shared).not.toContain("submitProductCertificationApplication");
		expect(shared).not.toContain("withdrawProductCertificationApplication");
	});

	it("只從 seller-repo 取型別（`import type`），不取執行期函式", () => {
		const imports = [
			...shared.matchAll(/import\s+(type\s+)?\{[\s\S]*?\}\s+from\s+"([^"]+)"/g),
		];
		const fromRepo = imports.filter((m) =>
			m[2].endsWith("products/seller-repo"),
		);
		expect(fromRepo).toHaveLength(1);
		expect(fromRepo[0][1]).toBe("type ");
	});
});

describe("商品表單（建立 vs 編輯）", () => {
	it("編輯才走 review，建立只能是 draft —— 建立頁沒有 productId 就打不到申請端點", () => {
		const tag = elements(sellerForm, "ProductCertificationRow")[0];
		expect(tag).toContain('form.mode === "edit" ? "review" : "draft"');
	});

	it("編輯頁把申請清單餵進去，否則賣家看不到自己剛送的那一筆", () => {
		const tag = elements(sellerForm, "ProductCertificationRow")[0];
		expect(tag).toContain("applications={form.certApplications}");
		expect(tag).toContain("onSubmit=");
	});

	it("申請清單與商品 detail 分開抓：投影裡沒有待審那一筆", () => {
		expect(sellerHook).toContain("fetchProductCertificationApplications");
	});

	it("取消勾選走 withdraw 端點，不是把它從商品 payload 拿掉", () => {
		// 全量取代的「沒送」以 certification_type 為粒度，而待審申請在那條路徑上是
		// 保留的（19 §6.4）——只從 payload 移除的話，賣家看到勾選消失、申請還在佇列。
		expect(sellerHook).toContain("withdrawProductCertificationApplication");
		expect(sellerHook).toContain("view.withdrawable");
	});
});

describe("匯入逐筆編輯", () => {
	it("宣告 draft：CSV 帶進來的認證還沒送審，不得顯示任何審核狀態", () => {
		const tag = elements(importForm, "ProductCertificationRow")[0];
		expect(tag).toContain('mode="draft"');
		expect(tag).not.toContain("applications=");
		expect(tag).not.toContain("onSubmit=");
	});

	it("匯入 hook 不碰申請端點（那裡還沒有商品可掛）", () => {
		expect(importHook).not.toContain("CertificationApplication");
	});
});

describe("代營運工作區不做即時驗證", () => {
	it("以 readonly 呼叫共用判讀", () => {
		expect(delegatedEditor).toContain(
			'from "@/lib/products/certification-status"',
		);
		expect(delegatedEditor).toContain('mode: "readonly"');
	});

	it("不呼叫任何需要賣家登入態的認證端點", () => {
		expect(delegatedEditor).not.toContain("verifyProductCertification");
		expect(delegatedEditor).not.toContain("CertificationApplication");
		expect(delegatedEditor).not.toContain("authedFetch");
	});

	it("狀態文字不是在這一面寫死的", () => {
		// 寫死「已通過」會讓判讀規則改動時只有兩個面跟著改。
		expect(delegatedEditor).not.toContain('"已通過"');
		expect(delegatedEditor).toContain("resolveCertTypeView");
	});

	it("既有 chip 仍可移除，但這一面不新增也不改號碼", () => {
		expect(delegatedEditor).toContain("removeCertification(value, index)");
		expect(delegatedEditor).not.toContain("addCertification");
		expect(delegatedEditor).not.toContain("<Input");
	});
});

describe("沒有任何面自行推導「已驗證」", () => {
	it("兩個 hook 都不帶本地驗證狀態", () => {
		for (const hook of [sellerHook, importHook]) {
			expect(hook).not.toContain("markCertVerified");
			expect(hook).not.toContain("verified");
		}
	});

	it("共用元件不自己寫碼值到中文的對照表", () => {
		// 徽章文字一律走 CodeMeta 生成表，見 lib/products/certification-status.ts。
		for (const label of ["審核中", "已通過", "未通過", "已撤回", "已撤銷"]) {
			expect(shared).not.toContain(`"${label}"`);
		}
	});
});
