/**
 * Strict TS types projected from 預設模組.openapi.json
 * Source of truth: /app/buyers/me; the social post types are the client-side view models over the Connect `hawkr.social.v1alpha1` messages (`lib/posts/repo.ts`)
 *
 * Naming: every JSON body field is camelCase (backend-payuni#2235 收斂 — 10 檔 §2:
 * URL/Query snake_case, JSON body camelCase). Query-string params are the sole
 * snake_case survivors and are built at the call site, never typed here.
 */

export const ACCOUNT_TYPE = {
	BUYER: 1,
	SELLER: 2,
	BOTH: 3,
	ADMIN: 4,
} as const;

export type AccountTypeValue = (typeof ACCOUNT_TYPE)[keyof typeof ACCOUNT_TYPE];

export type Role = "guest" | "buyer" | "seller" | "admin";

export function roleFromAccountType(t?: number | null): Role {
	if (t === ACCOUNT_TYPE.ADMIN) return "admin";
	if (t === ACCOUNT_TYPE.SELLER) return "seller";
	if (t === ACCOUNT_TYPE.BUYER) return "buyer";
	if (t === ACCOUNT_TYPE.BOTH) return "seller";
	return "guest";
}

/** website view model mapped from the Connect post product message in `lib/posts/repo.ts` */
export interface PostProductData {
	id: string;
	/** objectKey of the product image. */
	imageKey: string;
	name: string;
	price: string;
}

/** 觀看者在貼文留言時的身分（伺服器依觀看者算）：店主在自己店的貼文是店家，其他有買家身分的人是買家。 */
export interface PostViewerActor {
	kind: "buyer" | "store";
	id: string;
	/** 讀不到名稱（店家卡片讀不到、買家已刪除）時沒有。 */
	name?: string;
	avatarKey?: string;
}

/** 貼文上依觀看者算的留言權限；匿名讀取的貼文沒有這一組。 */
export interface PostViewerState {
	canComment: boolean;
	actor?: PostViewerActor;
}

/** website view model of a post list item, mapped from the Connect post message in `lib/posts/repo.ts` */
export interface PostListItem {
	/** 樂觀鎖版本；只有店主、代行讀自己的貼文時才有意義，寫入時帶回去。 */
	etag?: string;
	id: string;
	sellerId: string;
	/**
	 * 賣家店面的 canonical 定址碼 — 貼文卡片的賣家連結一律用它組
	 * `/sellers/{storeCode}`（#2098 階段三）。空字串代表 gateway 查不到該賣家
	 * （已關店 / 批次逾時）；此時不要退回 `sellerId`，那條路徑正在被收掉。
	 */
	storeCode: string;
	sellerName: string;
	/** §231 objectKey SSOT (`logoKey`); the wire also carries the Gateway-composed
	 *  `logoUrl` twin, which the UI no longer reads. */
	logoKey: string;
	content: string;
	/**
	 * 貼文圖 objectKey 陣列（無圖為 `[]`，欄位永遠存在）。
	 *
	 * 單數 `imageKey` / `imageUrl` 已於 backend#2235 從所有貼文端點移除：兩組並存時
	 * 編輯貼文讀單數欄、回送複數欄，複數那份會被當成「使用者刪掉了」而清空 DB 圖片。
	 * social 的 `posts.image` 是單一 TEXT 欄，所以這個陣列恆為 0–1 element。
	 */
	imageKeys: string[];
	tags: string[];
	product?: PostProductData | null;
	commentCount: number;
	likeCount: number;
	shareCount: number;
	viewCount: number;
	isLiked: boolean;
	createdAt: number;
	hiddenAt?: number;
	hiddenReason?: string;
	/** 下架這則貼文的措施 ID（只對店主；有值就是被平台下架，理由經措施讀）。 */
	takedownMeasureId?: string;
	/** 依觀看者的讀取才有（匿名讀取不設）：能不能留言與留言時的身分。 */
	viewer?: PostViewerState;
	/**
	 * PostPublishStatus 數字碼字串（4401 未發布／4402 已發布／4403 已下架）。
	 *
	 * 代行清單（`/admin/sellers/{id}/posts`）與賣家自己的清單
	 * （`/app/sellers/me/posts`）都帶它。**不可選代表「這條路徑的上游沒給」而不是
	 * 「這則貼文沒有狀態」** —— 渲染端因此只在有值時畫 pill，缺值時什麼都不畫
	 * （畫「已發布」會是一個沒有根據的宣稱）。可選性不因為兩條路徑都補上了就收掉：
	 * 那是述詞的防線，不是某一條路徑的暫時性補丁。
	 */
	publishStatus?: string;
	/**
	 * 代行者排定、尚未執行的發布時間（ISO 8601 UTC）；沒有排程時欄位不出現。
	 * 排程不是發布狀態：貼文仍是未發布（4401），到點才轉已發布。只有賣家自己的清單
	 * （`/app/sellers/me/posts`）帶它。
	 */
	scheduledPublishAt?: string;
}

/** website view model of post detail, mapped from the Connect post message in `lib/posts/repo.ts` */
export interface PostDetail {
	id: string;
	sellerId: string;
	sellerName: string;
	/** §231 objectKey SSOT; the `logoUrl` / `imageUrls` twins are unread. */
	logoKey: string;
	content: string;
	/** 見 PostListItem.imageKeys —— 無圖為 `[]`，恆為 0–1 element。 */
	imageKeys: string[];
	tags: string[];
	products: PostProductData[];
	commentCount: number;
	likeCount: number;
	shareCount: number;
	viewCount: number;
	isLiked: boolean;
	createdAt: number;
	/** 依觀看者的讀取才有（匿名讀取不設）。 */
	viewer?: PostViewerState;
}

/**
 * website view model 的店主視角（`lib/posts/owner.ts` 由 Connect 貼文訊息映射）：店主讀自己的單篇貼文。
 * 發布狀態與平台下架資訊同 `PostListItem` 的同名欄位；只有店主路徑帶它們。
 */
export interface MyPostDetail extends PostDetail {
	/** 樂觀鎖版本；修改、發布、下架與刪除時帶回。 */
	etag?: string;
	publishStatus?: string;
	hiddenAt?: number;
	hiddenReason?: string;
	takedownMeasureId?: string;
}

/** matches handler.BuyerLookupResponse */
export interface BuyerInfo {
	uid: string;
	buyerName: string;
	avatarUrl: string;
	phone: string;
}

export interface ApiEnvelope<T> {
	code: number;
	data: T;
	message?: string;
}

export interface Paginated<T> {
	page: string | number;
	pageSize: string | number;
	total: string | number;
	records: T[];
}
