"use client";

import * as React from "react";
import type { DelegatedMutationMeta } from "@/lib/api/delegated";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { type MediaUploadRepo, mediaUploadRepo } from "@/lib/media/upload";
import { type OrderRepo, orderRepo } from "@/lib/orders/repo";
import {
	type SellerProductRepo,
	sellerProductRepo,
} from "@/lib/products/seller-repo";
import { queryKeys } from "@/lib/query/keys";
import {
	type SellerPostRepo,
	type SellerStorefrontRepo,
	sellerPostRepo,
	sellerStorefrontRepo,
} from "@/lib/seller/repo";

/**
 * 賣家中心資料層的注入點（56 號 §4.5 L1053）。
 *
 * 賣家中心的元件會被**兩條路徑**渲染：`/seller-center`（賣家本人）與 `/adm`（管理員
 * 代行）。兩條路徑的傳輸層、快取鍵、啟用條件全都不同，而元件本身完全一樣 ⇒ 這三樣
 * 東西必須由外面給，不能由元件自己 import。
 *
 * **沒有 provider 時的預設值就是賣家路徑**，所以 `/seller-center` 一行都不用改、
 * 行為逐位元不變：`keys.*` 回的是既有的 `queryKeys.seller.*`、repo 是既有的
 * `authedFetch` 實例、`canRead` 原樣回傳呼叫端算出來的既有條件。
 *
 * 代行工作區改由 `/adm` 的 provider 給一份 delegated scope：repo 注入
 * `delegatedFetch`（`lib/api/delegated.ts`）、鍵換 `queryKeys.delegated.*`（工作階段
 * 身分而非登入身分）、`canRead` 改看 capability。
 */

export type SellerDataArea = "storefront" | "products" | "posts" | "orders";

/**
 * `operationReason` 的字數上限（trim 後 1–100，§3.4 ①）。
 *
 * 放在契約層而不是抽屜元件裡：貼文頁的「儲存，暫不發布」不開抽屜（mockup §4.9.5
 * 的底部條就是它的輸入點），兩處各寫一個 100 會在改值時漏掉其中一處。
 */
export const OPERATION_REASON_MAX = 100;

/** 送出前的變更摘要一列（mockup §4.6.5 三欄表；B20 由前端本地自算）。 */
export interface SellerMutationChange {
	field: string;
	before: string;
	after: string;
}

/**
 * 一次寫入要讓操作者確認什麼。**賣家路徑不使用**（`confirmMutation` 直接放行），
 * 代行路徑用它組出 §3.4 三件必備的確認抽屜。
 */
export interface SellerMutationRequest {
	/** 抽屜標題，例：`檢視變更並儲存`。 */
	title: string;
	/** 金底提示卡主文，例：`以 ADM 身分替梨山盛花園儲存商店資訊`。 */
	intent: string;
	hint?: string;
	/** 三欄表。空／未給時不畫（例：`/publish` 的變更恆為單一欄位，L3704）。 */
	changes?: readonly SellerMutationChange[];
	/** `expectedVersion` 露出點（§3.4 ②）。建立類寫入沒有版本，留空。 */
	expectedVersion?: string;
	/**
	 * 沒有 `expectedVersion` 時，替代它出現在同一個位置的說明（例：`不適用（新建
	 * 商品尚未有版本）`）。
	 *
	 * §3.4 ② 是三件必備之一，而**建立類寫入是它唯一合法缺席的情境**（新資源沒有
	 * 版本，create 的 DTO 連這個欄位都沒有）。留白與「漏畫了」在畫面上分不出來，
	 * 而這一格正是操作者用來判斷「我會不會覆蓋別人」的地方 —— 分不出來就等於這道
	 * 檢查在建立這一面上靜默消失。
	 */
	versionNote?: string;
	/**
	 * 多筆寫入的 `expectedVersion` 露出點（§3.4 ② 的批次形），例：批次下架。
	 *
	 * 批次沒有「一個版本」可放進 `expectedVersion` —— 版本是**逐項**送的（L3126 的
	 * `items[]` 每項自帶一個），所以缺這一欄時 §3.4 ② 在批次路徑上等於不存在：
	 * 回 9266 的那幾項，操作者看不出自己剛才是拿哪一版去覆蓋的，也就無從判斷該
	 * 重新載入哪幾筆。
	 */
	versionedItems?: readonly { label: string; version: string }[];
	/** 例：`將產生兩筆代行紀錄`。 */
	auditNote?: string;
	confirmLabel: string;
	/**
	 * 修改原因輸入框的 placeholder。
	 *
	 * **必須逐個寫入情境給**：這個元件是所有代行寫入的共用關卡，寫死一句範例會讓
	 * 「例：協助完成新品上架」出現在批次**下架**的抽屜裡，字面上直接與操作者正在
	 * 做的事相反。未給時退回一句不描述動作的中性文案。
	 */
	reasonPlaceholder?: string;
	/**
	 * 抽屜開啟時的初值。用於「表單上已經有一個修改原因輸入框，而這次寫入要接著開
	 * 抽屜」的情境（貼文頁底部條 → 儲存並發布，mockup §4.9.5 的兩條路徑共用同一個
	 * `代行原因`）—— 不帶進來的話操作者要為同一次寫入打兩次理由。
	 */
	initialReason?: string;
	initialNotifySeller?: boolean;
}

/**
 * 三態，不是布林：**「放行且不帶 meta」（賣家）與「放行且帶 meta」（代行）是兩件事**，
 * 而「取消」必須與兩者都分得開 —— 用 `meta === null` 表達取消，呼叫端一定會有人忘記
 * 判 null 而把 `null` 當成「沒有 meta」送出去，那就是一次沒有修改原因的寫入。
 */
export type SellerMutationDecision =
	| { ok: true; meta?: DelegatedMutationMeta }
	| { ok: false };

/** 代行專屬命令（賣家路徑沒有對應端點，恆為 `null`）。 */
export interface SellerDelegatedCommands {
	batchUnpublishProducts(
		items: readonly { productId: string; expectedVersion: string }[],
		meta: DelegatedMutationMeta,
	): Promise<{
		requested: number;
		succeeded: number;
		failed: number;
		results: readonly {
			productId: string;
			ok: boolean;
			code: number;
			auditLogId: string;
		}[];
	}>;
	publishPost(
		postId: string | number,
		meta: DelegatedMutationMeta,
		schedule?: { publishMode: string; scheduledPublishAt?: string },
	): Promise<void>;
}

export interface SellerDataRepos {
	storefront: SellerStorefrontRepo;
	products: SellerProductRepo;
	posts: SellerPostRepo;
	orders: OrderRepo;
	media: MediaUploadRepo;
}

export interface SellerDataKeys {
	storefront(): readonly unknown[];
	products(params?: Record<string, unknown>): readonly unknown[];
	posts(
		sellerId: string | number,
		params?: Record<string, unknown>,
	): readonly unknown[];
	orders(params?: Record<string, unknown>): readonly unknown[];
	order(
		orderNo: string | number,
		params?: Record<string, unknown>,
	): readonly unknown[];
	orderCounts(params?: Record<string, unknown>): readonly unknown[];
}

export interface SellerDataScope {
	mode: "seller" | "delegated";
	repos: SellerDataRepos;
	keys: SellerDataKeys;
	/**
	 * 該面能不能讀。
	 *
	 * 賣家路徑**原樣回傳呼叫端既有的條件**（`sellerGate`）—— 這是「`/seller-center`
	 * 行為逐位元不變」的機械保證：每個呼叫點的 `enabled` 仍是它自己算出來的那個
	 * 布林值，scope 一個字都沒改。
	 *
	 * 代行路徑忽略 `sellerGate`，改由工作階段的 capability 決定：代行的瀏覽器在 www
	 * 上沒有登入態，沿用 `isLoggedIn && activeRole === "seller"` 會**恆為 false**，
	 * 整個工作區一個查詢都不會發，而畫面只會顯示空清單、不報任何錯。
	 */
	canRead(area: SellerDataArea, sellerGate: boolean): boolean;
	/**
	 * 店家鍵。賣家路徑為 `null`（呼叫端仍從 `useSellerSelf()` 取得），代行路徑由
	 * 工作階段下發 —— 代行下 `useSellerSelf()` 不掛，沒有第二個地方拿得到它。
	 */
	ownerStoreId: string | null;
	/**
	 * 目標店家的店名。賣家路徑為 `null`（呼叫端仍從 `useSellerSelf()` 取得），代行
	 * 路徑由工作階段下發。
	 *
	 * 存在的理由只有一個：確認抽屜的提示卡必須指名店家（§6.5）。雙寫元件（商品
	 * 列表這類同時服務兩條路徑的畫面）在代行下沒有第二個地方拿得到店名 ——
	 * `useSellerSelf()` 不掛，而沒有店名時提示卡只能寫一句「本店」，那正是代行
	 * 事故（按錯店）最後一道關卡上最不該出現的字。
	 */
	ownerStoreName: string | null;
	/**
	 * 寫入前的確認關卡。
	 *
	 * 賣家路徑**立刻放行且不帶 meta**：`/seller-center` 的每一次儲存行為因此一個
	 * 字都沒變。代行路徑開 §3.4 的確認抽屜，拿到修改原因／目前版本／通知賣家三件
	 * 之後才回 `ok`，取消則回 `{ok:false}`。
	 */
	confirmMutation(
		request: SellerMutationRequest,
	): Promise<SellerMutationDecision>;
	/** 代行專屬命令（批次下架、貼文發布）。賣家路徑恆為 `null`。 */
	delegatedCommands: SellerDelegatedCommands | null;
	/**
	 * 這個工作階段有沒有某個 capability。
	 *
	 * **賣家路徑恆為 `true`**：那裡沒有 capability 這個概念，能不能做由後端的賣家
	 * 授權鏈決定；回 false 會讓賣家中心憑空少掉功能。呼叫端要表達「只有代行才做的
	 * 事」時，判準是 `mode === "delegated" && hasCapability(...)`，不是只看這一支。
	 */
	hasCapability(capability: string): boolean;
}

/** 賣家路徑的既有實例（`authedFetch`），與具名匯出是同一份閉包。 */
export const sellerDataRepos: SellerDataRepos = {
	storefront: sellerStorefrontRepo,
	products: sellerProductRepo,
	posts: sellerPostRepo,
	orders: orderRepo,
	media: mediaUploadRepo,
};

const SellerDataScopeContext = React.createContext<SellerDataScope | null>(
	null,
);

export function SellerDataScopeProvider({
	scope,
	children,
}: {
	scope: SellerDataScope;
	children: React.ReactNode;
}) {
	return (
		<SellerDataScopeContext.Provider value={scope}>
			{children}
		</SellerDataScopeContext.Provider>
	);
}

/**
 * 目前該用哪一份資料層。沒有 provider ⇒ 賣家路徑（既有行為）。
 *
 * ⚠️ `useQueryIdentity()` 一律呼叫（hook 不能寫在條件裡），但只有在沒有 provider
 * 時才會被讀 —— 代行工作區在 www 上沒有登入態，它恆為 `{null, null}`。
 */
export function useSellerDataScope(): SellerDataScope {
	const injected = React.useContext(SellerDataScopeContext);
	const identity = useQueryIdentity();

	const sellerScope = React.useMemo<SellerDataScope>(
		() => ({
			mode: "seller",
			repos: sellerDataRepos,
			keys: {
				storefront: () => queryKeys.seller.self(identity),
				products: (params) => queryKeys.seller.products(identity, params),
				posts: (sellerId, params) =>
					queryKeys.seller.posts(identity, sellerId, params),
				orders: (params) => queryKeys.seller.orders(identity, params),
				order: (orderNo, params) =>
					queryKeys.seller.order(identity, orderNo, params),
				orderCounts: (params) => queryKeys.seller.orderCounts(identity, params),
			},
			canRead: (_area, sellerGate) => sellerGate,
			ownerStoreId: null,
			ownerStoreName: null,
			confirmMutation: async () => ({ ok: true }),
			delegatedCommands: null,
			hasCapability: () => true,
		}),
		[identity],
	);

	return injected ?? sellerScope;
}
