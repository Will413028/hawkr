import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { PRODUCT_STATUS_LABEL, ProductStatus } from "@/lib/codemeta/enums";
import { shelfStatusChange, targetStoreLabel } from "./confirm-content";

/**
 * 確認抽屜是不可逆寫入前的最後一道人為關卡，設計書對它下了兩條逐字要求：
 * §6.5「顯示『以 ADM 身分替梨山盛花園上架商品』、理由、before／after、目前版本…」，
 * §4.1 對 `ADMIN_PRODUCT_PUBLISH` 的確認要求是「diff＋二次確認＋通知」。
 */

const ROOT = fileURLToPath(new URL("../..", import.meta.url));
const EDIT_PAGE =
	"features/delegated-operations/products/DelegatedProductEditPage.tsx";
const POST_COMPOSE = "features/seller-center/posts/PostComposeClient.tsx";

function read(file: string): string {
	return readFileSync(join(ROOT, file), "utf8");
}

describe("targetStoreLabel", () => {
	it("有店名就用店名", () => {
		expect(targetStoreLabel("梨山盛花園", "2685914682")).toBe("梨山盛花園");
	});

	it("空白店名不算有店名", () => {
		expect(targetStoreLabel("   ", "2685914682")).toBe("店家 2685914682");
	});

	/**
	 * ⚠️ 這一條是本節的重點：退回「本店」等於沒修 —— 在一個替別人操作的畫面上，
	 * 「本店」既沒指出是哪一家，讀起來還像是管理員自己的店。
	 */
	it("拿不到店名時退店家編號，而不是「本店」", () => {
		const label = targetStoreLabel(null, "2685914682");
		expect(label).toBe("店家 2685914682");
		expect(label).not.toContain("本店");
	});

	it("兩者都沒有時明說無法確認，不給一句讀得通的空話", () => {
		const label = targetStoreLabel(undefined, "");
		expect(label).toContain("無法確認");
		expect(label).not.toContain("本店");
	});
});

describe("shelfStatusChange", () => {
	it("上架：未啟用 → 啟用", () => {
		expect(
			shelfStatusChange(
				{ status: ProductStatus.INACTIVE, statusLabel: "" },
				ProductStatus.ACTIVE,
			),
		).toEqual({
			field: "發布狀態",
			before: PRODUCT_STATUS_LABEL[ProductStatus.INACTIVE],
			after: PRODUCT_STATUS_LABEL[ProductStatus.ACTIVE],
		});
	});

	it("下架方向的 before／after 相反", () => {
		const row = shelfStatusChange(
			{ status: ProductStatus.ACTIVE, statusLabel: "" },
			ProductStatus.INACTIVE,
		);
		expect(row.before).toBe(PRODUCT_STATUS_LABEL[ProductStatus.ACTIVE]);
		expect(row.after).toBe(PRODUCT_STATUS_LABEL[ProductStatus.INACTIVE]);
		expect(row.before).not.toBe(row.after);
	});

	it("before 優先用後端下發的 statusLabel（與稽核列同一份字串）", () => {
		expect(
			shelfStatusChange(
				{ status: ProductStatus.DRAFT, statusLabel: "草稿（後端）" },
				ProductStatus.ACTIVE,
			).before,
		).toBe("草稿（後端）");
	});
});

describe("商品編輯頁：上／下架確認抽屜", () => {
	const source = read(EDIT_PAGE);

	it("上／下架抽屜帶 before／after 變更列（§4.1 高風險：diff 不因單欄而免除）", () => {
		expect(source).toContain("changes: [shelfStatusChange(detail, target)]");
	});

	it("提示卡指名目標店家，不寫「本店」", () => {
		expect(source).toContain("targetStoreLabel(");
		expect(source).toMatch(/替「\$\{targetStore\}」/);
		// 註解裡照樣不准出現：這個字串是本項要消滅的對象，留在檔內只會被下一個人抄走。
		expect(source).not.toContain("替本店");
	});
});

describe("貼文代發：署名列", () => {
	const source = read(POST_COMPOSE);

	/**
	 * 設計書 §6.6 的「固定文案」＝不論拿不拿得到店名都要有這一行。原本是
	 * `sellerName ? … : undefined`，退化時消失的偏偏是承載歸屬的那一行。
	 */
	it("代行分支不再以店名的有無決定整行的存在", () => {
		expect(source).toMatch(
			/以「\$\{targetStoreLabel\(sellerName, scope\.ownerStoreId\)\}」名義，由 ADM 代行發布/,
		);
	});

	it("賣家自己那條分支原樣保留（/seller-center 行為不變）", () => {
		expect(source).toMatch(/以「\$\{sellerName\}」名義發布/);
	});
});
