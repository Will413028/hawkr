import type { ProductDescription } from "@/lib/products/seller-repo";
import type { DelegatedProductDetail } from "./repo";

/**
 * 版本衝突（9266）的合流規則（§3.4 版本衝突契約／設計書 L5346）。
 *
 * 契約要求兩個出口，而**兩個出口的差別全在這一支**：
 * - 「重新載入最新版本」＝ 丟掉我的輸入，整張表單 hydrate 成賣家最新的內容
 * - 「複製我的內容」＝ **以賣家最新版本為基底**，把我剛才打的字留下來
 *
 * ⚠️ 「複製我的內容」**不是**「用我的內容覆蓋賣家的」：它只是不要讓操作者白打一次，
 * 送不送出仍要走一次確認抽屜，而那個抽屜上顯示的是**賣家最新的版本號**。
 */

/** 表單上一個規格的可編輯欄位（其餘欄位代行 UI 唯讀，由 Gateway 以 before 回填）。 */
export interface DelegatedSkuDraft {
	spec: string;
	price: string;
	salePrice: string;
	stockQty: string;
	unit: string;
}

export interface DelegatedProductFormDraft {
	name: string;
	/** 分類的葉節點 id（`categoryLv3Id`）。分類樹未載完時為空字串。 */
	categoryId: string;
	origin: string;
	description: ProductDescription;
	/** `name` 存的是 `CertificationType` 碼值，不是中文名（見 `./certifications`）。 */
	skus: Record<string, DelegatedSkuDraft>;
}

/** 最新版本的內容 → 表單初值（＝「重新載入最新版本」那個出口）。 */
export function draftFromDetail(
	detail: DelegatedProductDetail,
): DelegatedProductFormDraft {
	const skus: Record<string, DelegatedSkuDraft> = {};
	for (const sku of detail.skus) {
		skus[sku.skuId] = {
			spec: sku.spec,
			price: sku.price,
			salePrice: sku.salePrice,
			stockQty: sku.stockQty,
			unit: sku.unit,
		};
	}
	return {
		name: detail.name,
		categoryId: detail.categoryId,
		origin: detail.origin,
		description: detail.description,
		// 複製一份：表單會就地增刪，共用同一個陣列會讓「有沒有改過」的比較恆為 true
		// （detail 那一份跟著一起變）⇒ 送出鍵永遠可按、送出去卻是一筆空變更。
		skus,
	};
}

/**
 * 「複製我的內容」：以 `latest` 為基底，保留 `mine` 打過的欄位。
 *
 * 四條規則，每一條都對應一個「不這樣做就會壞資料」的理由：
 * 1. **我自己填的欄位保留我的**（`name`／`categoryId`／`origin`／`description`）—— 那正是操作者不想白打的東西；差異會出現在下一次確認抽屜的
 *    三欄表裡，由他自己判斷要不要蓋過去。
 * 2. **規格以 `latest` 為準集合**：賣家在這段期間新增的規格要出現（否則我送出的
 *    `skus` 陣列少一筆，而 product-service 對非空 `skus` 是**整份對齊** ⇒ 那一筆
 *    規格會被移除，請求合法、回 `code: 0`）；賣家刪掉的規格則不再帶回來。
 * 3. **圖片一律採 `latest`，不在這裡回傳** —— 保留既有圖的憑據是 `assetId`，而它
 *    必須來自**這一次** GET（Gateway 拿 intent 之前讀到的原始資源比對，不在其中
 *    即 9275）。賣家在這段期間換過圖時，舊 `assetId` 會讓整次送出被拒。
 * 4. **`categoryId` 為空時退回 `latest` 的**：分類樹還沒載完就撞上衝突時，`mine` 的
 *    分類是空字串，照抄會把一個必填欄位清成空 —— 而空的 `categoryId` 不會被送出
 *    （`buildProductPatch`），於是畫面上分類欄空著、送出後分類卻沒變，兩邊對不上。
 */
export function keepMyEdits(
	latest: DelegatedProductDetail,
	mine: DelegatedProductFormDraft,
): DelegatedProductFormDraft {
	const base = draftFromDetail(latest);
	const skus: Record<string, DelegatedSkuDraft> = {};
	for (const sku of latest.skus) {
		skus[sku.skuId] = mine.skus[sku.skuId] ?? base.skus[sku.skuId];
	}
	return {
		name: mine.name,
		categoryId: mine.categoryId || base.categoryId,
		origin: mine.origin,
		description: mine.description,
		skus,
	};
}
