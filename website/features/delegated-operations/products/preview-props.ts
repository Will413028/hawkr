import type {
	DelegatedProductDetail,
	DelegatedProductShippingMethod,
} from "./repo";

/**
 * 代行商品預覽的資料轉接（56 號 §2.2 out-of-scope 第 13 列）。
 *
 * ⚠️ **預覽不得改打 `/api/v1/public/*`**（設計書定版）。那是最省事也最錯的做法：
 * 公開端點只看得到 `publish_status = 4402` 的商品，而代行者最需要預覽的正是**草稿**
 * 與**已下架**的那幾筆 —— 打過去不會 404、不會報錯，會回一個「查無此商品」的空殼，
 * 預覽於是**靜默空白**。所以預覽的唯一資料來源是代行自己的
 * `GET .../products/{productId}`（本頁已經為了編輯而載入的那一份）。
 *
 * 代價是兩份 DTO 的欄名不同。**三處改名**，每一處接漏的失敗形狀都是靜默的：
 *
 * | 代行 DTO | 買家 PDP 讀的名字 | 漏掉會怎樣 |
 * |---|---|---|
 * | `images[].imageKey` | `imageKeys[]` | 圖庫**整片空白**（`deriveDetailLayoutProps` 的 `thumbnails` 只讀 `imageKeys`／`imageKey`／`gallery`） |
 * | `skus[].skuId` | `skus[].id` | 規格 id 退回索引字串，選規格與加購物車的鍵對不上 |
 * | `skus[].spec` | `skus[].skuName` | 規格名變成「規格 1」「規格 2」（derive 的 `fallbackName`） |
 *
 * 剩下的欄名兩邊**逐字相同**（`price`／`salePrice`／`stockQty`／`unit`／
 * `shippingMethods[].{type,temperature,freeThreshold,feePayer}`），所以不改名 ——
 * 多寫一層映射只會多一個會漂移的地方。
 *
 * **兩處由 SKU 聚合**（公開端點是 product-service 算好才下發的，代行 DTO 沒有）：
 * `deliveryTemp` 與商品層 `shippingMethods`。算法就是「取所有 SKU 的聯集」，與
 * gateway 公開端點的 `data.DeliveryTemp`／`data.ShippingMethods` 同義，**零後端改動**。
 *
 * **三處刻意留空**：`rating`／`reviewCount`／`reviews`。代行 DTO 沒有它們，而 derive
 * 對缺席的處理正好是「尚無評價」—— 對一個還沒上架的商品來說，那比借用別的數字誠實。
 */

/** 溫層顯示順序（常溫 → 冷藏 → 冷凍），與買家 PDP 一致。 */
const TEMP_ORDER = ["3201", "3202", "3203"] as const;

function dedupeKey(method: DelegatedProductShippingMethod): string {
	return [
		method.type,
		method.temperature,
		method.freeThreshold,
		method.feePayer,
	].join("|");
}

/**
 * 代行商品 DTO → `deriveDetailLayoutProps` 吃的 `product`。
 *
 * `storeId` 另外注入：`getProductSellerStoreId` 讀的是 `product.seller.storeId`／
 * `sellerAccountId`／`storeId`／`createdBy`，代行 DTO 一個都沒有，缺它整張賣家卡不畫
 * （與匯入預覽同一招，`PreviewDetailPage.tsx:130-132`）。
 */
export function delegatedPreviewProduct(
	detail: DelegatedProductDetail,
	storeId: string,
): Record<string, unknown> {
	const methods: DelegatedProductShippingMethod[] = [];
	const seen = new Set<string>();
	for (const sku of detail.skus) {
		for (const method of sku.shippingMethods) {
			const key = dedupeKey(method);
			if (seen.has(key)) continue;
			seen.add(key);
			methods.push(method);
		}
	}

	const temps = new Set(
		methods.map((m) => m.temperature).filter((t) => t !== ""),
	);

	return {
		name: detail.name,
		origin: detail.origin,
		categoryName: detail.categoryName,
		description: detail.description,
		hashtags: detail.hashtags,
		// 認證**原樣傳碼值**（`3701`…），不在這裡套 `certLabel`：gateway
		// （`product.go:643-647`）原樣轉發碼值，碼值→顯示名的解析由
		// `ProductDetailLayout` 自己做，而預覽與公開 PDP 共用那同一支 layout。
		// 在這裡先解析等於讓預覽走一條公開頁不會走的路，兩邊就可能分岔 —— 而預覽存在
		// 的理由正是「買家會看到什麼」。（website#507 修好的就是 layout 那一側。）
		certifications: detail.certifications,
		imageKeys: detail.images.map((image) => image.imageKey).filter(Boolean),
		skus: detail.skus.map((sku) => ({
			id: sku.skuId,
			skuName: sku.spec,
			price: sku.price,
			salePrice: sku.salePrice,
			stockQty: sku.stockQty,
			unit: sku.unit,
		})),
		deliveryTemp: TEMP_ORDER.filter((code) => temps.has(code)),
		shippingMethods: methods,
		// 這兩欄是預覽存在的理由本身：「買家看不看得到、買不買得到」。
		// ⚠️ 不映射的後果是靜默且方向最壞的——deriveDetailLayoutProps 的判定是
		// `product.purchasable !== false`，欄位缺席就恆為 true ⇒ 未上架商品的預覽
		// 顯示成「可以購買」，而那正是這個畫面要回答的那個問題。
		purchasable: detail.purchasable,
		publicationState: detail.publicationState,
		storeId,
	};
}
