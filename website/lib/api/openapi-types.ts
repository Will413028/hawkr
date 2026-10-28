/**
 * Strict TS types projected from 預設模組.openapi.json
 * Source of truth: /public/posts, /app/buyers/me, /app/posts/{id}/*
 *
 * Naming: every JSON body field is camelCase (backend-payuni#2235 收斂 — 10 檔 §2:
 * URL/Query snake_case, JSON body camelCase). Query-string params are the sole
 * snake_case survivors and are built at the call site, never typed here.
 */

/* ---------- Account ---------- */
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

/* ---------- Post (Feed) ---------- */
/** matches handler.PostProductData */
export interface PostProductData {
	// 64-bit TSID（> Number.MAX_SAFE_INTEGER）：api/json.ts 的 preserveBigInts 在
	// parse 時保字串，後端亦已對貼文 DTO 的 id 加 ,string（gateway PR #967），故
	// canonical 型別一律 string，避免精度丟失。
	id: string;
	/** §231: objectKey SSOT (`PostProductData.ImageKey`); the wire also carries the
	 *  Gateway-composed `imageUrl` twin, which the UI no longer reads. */
	imageKey: string;
	name: string;
	price: string;
}

/** matches handler.PostListItemData / handler.MyPostListItemData */
export interface PostListItem {
	// 64-bit TSID，canonical string（見 PostProductData 註解）。
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
	hiddenAt?: number; // unix；0/缺 = 未下架（僅 /app/sellers/me/posts 回傳）
	hiddenReason?: string; // 平台下架原因（同上）
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

/** matches handler.PostDetailData */
export interface PostDetail {
	// 64-bit TSID，canonical string（見 PostProductData 註解）。
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
}

/**
 * matches handler.MyPostDetailData：店主讀自己的單篇貼文（`/app/sellers/posts/{id}`）。
 * 發布狀態與平台下架資訊同 `PostListItem` 的同名欄位；只有店主路徑帶它們。
 */
export interface MyPostDetail extends PostDetail {
	publishStatus?: string;
	hiddenAt?: number;
	hiddenReason?: string;
}

/* ---------- Buyer profile ---------- */
/** matches handler.BuyerLookupResponse */
export interface BuyerInfo {
	uid: string;
	buyerName: string;
	avatarUrl: string;
	phone: string;
}

/* ---------- Pagination envelope ---------- */
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

/* ---------- Procurement（Hawkr 幫找採購助手候選比對） ---------- */

export type ProcurementLane = "PLATFORM" | "EXTERNAL";

/** 對應後端 domain.Slots；後端 json tag 是 snake_case，這是這組端點唯一的
 * 例外——轉換只發生在 features/procurement-assistant/session-logic.ts 的
 * mapSlots，這個型別本身維持全 repo camelCase 慣例。 */
export interface ProcurementSlots {
	item?: string;
	quantity?: number;
	unit?: string;
	packaging?: string;
	budgetPerUnit?: number;
	deliveryCity?: string;
	deliveryDate?: string;
	exactMatchRequired?: boolean;
	/** 品項的解析結果——買家原話（item）被判定成 Pricebook 分類體系裡的哪一層。
	 *  它決定報價的價格基準（同一大類內品種實測差 2.96 倍），跟 item 原話同一個
	 *  revision，所以住在 slots 裡而不是另一個頂層欄位。尚未解析時不帶。 */
	itemResolution?: ProcurementItemResolution;
}

/** 對應後端 domain.ItemResolution（ADR 2026-09-04-crop-resolution-into-slots）。 */
export interface ProcurementItemResolution {
	/** CATEGORY_CONFIRMED＝已確認平台分類；BUYER_DESCRIBED＝買家自行描述、沒有分類。 */
	kind: "CATEGORY_CONFIRMED" | "BUYER_DESCRIBED";
	/** 分類名稱（「甘藍-初秋」「甘藍」），BUYER_DESCRIBED 時為買家的描述。 */
	label?: string;
	/** 目前只有 BUYER_CHOICE＝買家在消歧卡上親自挑的（含自行描述）。 */
	provenance: "BUYER_CHOICE";
	/** 伺服器組好的顯示字串。**不要**自己用 kind＋label 重拼——recordedThisTurn
	 *  的「品項分類」chip 用的就是這個值，兩邊各拼一次會漂成兩套文案，而它們在
	 *  畫面上相鄰。 */
	displayValue: string;
}

/** matches handler.GetSnapshot 回應 */
export interface ProcurementSnapshot {
	requestId: string;
	revisionId: string;
	status: "CLARIFYING" | "REQUIREMENT_READY" | "MATCHING" | "CANDIDATES_READY";
	slots: ProcurementSlots;
	missingRequired: string[];
}

/** matches handler.candidateSetResponse 候選卡結構（含 PR #2338 的 selected 欄位） */
export interface ProcurementCandidate {
	candidateId: string;
	sourceId: string;
	lane: ProcurementLane;
	displayName: string;
	score: number;
	matchedFacts: Array<{ key: string; label: string }>;
	unverifiableFacts: Array<{ key: string; reason: string }>;
	/** RFC3339 UTC（`...Z`）。 */
	factsAsOf: string;
	selected: boolean;

	// ---- B-246 / backend #2377：賣家身分快照 ----
	//
	// 全部 optional：後端一律 omitempty，EXTERNAL lane 的候選（站外業者，沒有平台賣家）
	// 這些 key 根本不會出現。**不要把缺值補成空字串**——「沒有商家」與「商家叫做空字串」
	// 必須分得開，分組邏輯就是靠 `storeId` 在不在來決定要不要分組。
	//
	// 同理，backend 未部署本欄位前這些值也全部是 undefined，所以任何消費端都必須能在
	// 「一筆都沒有」的情況下正常運作（見 candidate-grouping.ts 的 canGroupBySeller）。
	//
	// 刻意沒有商品層欄位（圖片／標價）：候選的單位是**收件人**不是品項，EXTERNAL lane
	// 也沒有平台商品。價格由 Pricebook 對該品項的參考價區間承接（#2403），不是商品標價
	// ——設計書 57 §1.1-2 只認四種價格事實，商品標價不在其中。
	//
	// ⚠️ 欄名是 storeId 不是 sellerId：這個值是 store-stint surrogate（`sellers.store_id`），
	//    DB／proto／對外 JSON 三層同名。B-246 內文寫的 `sellerId` 已被後端 PR #2702 改掉。
	/** 商家的 store-stint surrogate（十進位字串）。缺值＝這個候選沒有平台賣家。 */
	storeId?: string;
	sellerName?: string;
	/**
	 * 賣家頁 canonical 短碼（`sellers.store_code`）。
	 * 缺值時**不要退回 storeId 組連結**——`/sellers/{storeId}` 那條路徑正在被收掉
	 * （見本檔 SellerSummary.storeCode 的同一條註解）；沒有 storeCode 就不給連結。
	 */
	storeCode?: string;
	/** 商家 logo objectKey（非顯示用 SSOT）。顯示一律用 logoUrl。 */
	logoKey?: string;
	/** Gateway 組好的商家 logo 顯示 URL。 */
	logoUrl?: string;

	// ---- 站外業者的品牌素材 ----
	//
	// 與上面的 logoKey／logoUrl **是兩個不同的軸，不得互相 fallback**：那一組是
	// 平台商家的身分（PLATFORM lane 才有），這一組只是「這家站外業者自己的品牌圖」。
	// 把 brandLogoUrl 讀成 logoUrl 會讓站外業者在畫面上變成平台商家，而分組邏輯
	// （canGroupBySeller）也只看 storeId，不看這兩欄。
	//
	// 有品牌圖**不表示**平台已驗證、已認證或已入駐。這家業者的 unverifiableFacts
	// 恆有一條常設揭露（「外部名單，供應能力未經平台驗證，以賣家回覆為準」），
	// 呈現時不得把它拿掉、折疊或弱化。
	/** 站外業者 logo objectKey（非顯示用 SSOT）。顯示一律用 brandLogoUrl。 */
	brandLogoKey?: string;
	/** Gateway 組好的站外業者 logo 顯示 URL。 */
	brandLogoUrl?: string;
	/** 站外業者封面 objectKey（非顯示用 SSOT）。 */
	brandCoverKey?: string;
	/** Gateway 組好的站外業者封面顯示 URL（4:1）。 */
	brandCoverUrl?: string;
}

/** matches handler.GetCandidateSearch / handler.PostCandidateSearch 回應（PostCandidateSearch
 * 只回傳 requestId/runId/status 三欄，是這個型別的子集，呼叫端直接用內嵌型別接） */
/**
 * run / lane 狀態值域的 SSOT 是 backend 的 CodeMeta（業務狀態碼.md 的
 * ProcurementRunStatus / ProcurementLaneStatus，issue #2446 契約凍結）。
 *
 * 改名前的舊值（run 的 COMPLETED／PARTIAL、lane 的 OK）已於本波移除：backend 的
 * ai-service 與 gateway 都已部署改名版本，DB 既有列也已由 migration 收斂成新值。
 *
 * ⚠️ 這個 union 只是型別層宣告，值是從 API 來的執行期字串——**TypeScript 不會在收到
 * 未列出的值時報錯**，下游的 `===` 比較只會靜默恆為 false。所以「少列一個線上真的會送的
 * 值」這種錯不會有任何編譯期或執行期訊號，只會表現成前端卡住。移除舊值的前提是
 * backend 真的不再送，那件事已在部署後驗證過，不是推論。
 */
export type ProcurementRunStatus =
	| "QUEUED"
	| "RUNNING"
	| "SUCCEEDED"
	| "FAILED";

export type ProcurementLaneStatus = "PENDING" | "SUCCEEDED" | "FAILED";

export interface ProcurementCandidateSearch {
	requestId: string;
	runId: string;
	candidateSetId: string;
	status: ProcurementRunStatus;
	platformLaneStatus: ProcurementLaneStatus;
	externalLaneStatus: ProcurementLaneStatus;
	/**
	 * 值域是 CodeMeta 的 ProcurementRunFailureReason（WORKER_LOST / LANE_ALL_FAILED），
	 * 空字串代表未失敗。原名 `errorCode`，該 key 已於 #2446 契約凍結自 gateway 移除。
	 *
	 * 仍宣告成 optional：這是 API 回應反序列化出來的型別，欄位缺席時是 undefined，
	 * 型別層強制成必填只會製造假的安全感。
	 */
	failureReason?: string;
	candidates: {
		platform: ProcurementCandidate[];
		external: ProcurementCandidate[];
	};
}
