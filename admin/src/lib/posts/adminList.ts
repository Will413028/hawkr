/**
 * `GET /api/v1/admin/posts` 的 wire 契約，與貼文管理頁對它做的取值運算。
 *
 * 形狀權威 = backend-payuni `gateway/internal/handler/post.go` `AdminPostListItemData`
 * （貼文圖只有 `imageKeys` / `imageUrls` 兩個欄位，**無 omitempty**：無圖時是 `[]`，
 * 不是 undefined）。
 *
 * 從頁面拆出來是為了讓形狀能被實打回應釘住（tests/admin-post-list-media-contract.test.ts）。
 * 頁面原本宣告 `images: string[]` —— gateway 從來沒送過這個欄位，於是
 * `post.images?.length > 0` 恆為 false，111 則貼文的縮圖一張都沒渲染
 * （hawkr-website#430 B1）。TypeScript 攔不到這種錯：介面是我們自己宣告的，
 * 宣告錯了就一路錯到畫面。
 *
 * 媒體欄位一律讀 `imageKeys`（objectKey ＝ SSOT，設計書 49 §6.1），顯示 URL 交給
 * `buildImageUrl(key, preset)` 組；不讀 gateway 併送的 `imageUrls`（那是固定
 * `/i/original/` 的原圖，40×40 縮圖位置用它等於拉原圖）。
 */

/** 貼文狀態（social-service 送字面值，非數字碼）。 */
export type AdminPostStatus = "published" | "hidden" | "deleted";

export interface AdminPostListItem {
	// postId / storeId 為 64-bit TSID（> Number.MAX_SAFE_INTEGER），必須用
	// string，否則原生 JSON.parse 會失精導致 moderate 打錯貼文。後端 gateway 已對
	// AdminPostListItemData 加 ,string（PR #967）。
	postId: string;
	storeId: string;
	sellerName: string;
	content: string;
	/** 貼文圖 objectKey 陣列 — SSOT，顯示走 buildImageUrl。無圖時為 `[]`。 */
	imageKeys: string[];
	/** gateway 併送的 `/i/original/` 顯示 URL（本頁不讀，留著標示契約全貌）。無圖時為 `[]`。 */
	imageUrls: string[];
	status: string;
	reportCount: number;
	latestReportReason?: string;
	/** Unix 秒（非毫秒）；顯示前要 ×1000。 */
	latestReportAt?: number;
	hiddenReason?: string;
	/** Unix 秒（非毫秒）；顯示前要 ×1000。 */
	createdAt: number;
}

/**
 * 列表縮圖要用的 objectKey：第一張有值的貼文圖，沒有就回空字串。
 *
 * 空 key 直接篩掉而不是交給呼叫端判斷 —— 渲染成 `src=""` 的空框正是本次要修掉的
 * 症狀，缺圖該讓那格只剩文字，而不是掛一個破圖框。
 *
 * 仍走可選鏈是對「陣列整個缺席」的防禦（例如部署錯配），不是舊欄名的 fallback。
 */
export function selectPostThumbKey(post: AdminPostListItem): string {
	return post.imageKeys?.find((key) => Boolean(key?.trim())) ?? "";
}
