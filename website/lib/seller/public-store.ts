import { Code, ConnectError } from "@connectrpc/connect";
import { townByCode } from "@hawkr/twaddr/lookup";
import { sellerPublicClient } from "@/lib/connect/seller-public";
import type { PublicTransportOptions } from "@/lib/connect/transport";
import { imageUri } from "@/lib/media/image";

/**
 * 店面頁讀到的店家：`GetStore`（以網址上的 store_code 讀）的 `PublicStore` 加 `StoreStats`，整理成
 * 頁面元件直接用的形狀。`storeId` 是其餘讀取（商品、評價、貼文、討論區、追蹤）的參照鍵（ADR 2026-10-03：
 * 網址用 code，API 參照用 id）；統計欄位讀不到時不設，呈現層顯示「—」。
 */
export interface PublicStoreView {
	storeId: string;
	storeCode: string;
	name: string;
	description: string;
	prize: string;
	websiteUrl: string;
	logo_url: string;
	coverImage: string;
	/** 縣市名稱；地址只公開到縣市與鄉鎮市區。 */
	city: string;
	district: string;
	followerCount?: number;
	reviewCount?: number;
	ratingAverage?: number;
}

/** `stores_store_code_check`：store_code 是 1 到 9999999999 的十進位整數。 */
export function isStoreCodeShape(segment: string): boolean {
	return /^[1-9]\d{0,9}$/.test(segment);
}

/** `GetStore` 只以 not_found 表示這個網址沒有店面；其他錯誤（含 invalid_argument）是未知。 */
export function isStoreAbsentError(err: unknown): boolean {
	return ConnectError.from(err).code === Code.NotFound;
}

/**
 * 以網址上的 store_code 讀店面；沒有這家店（不存在、不公開）或形狀不對回 null，其他錯誤照拋，
 * 讓呼叫端與快取不把一次故障當成「沒有店」。
 */
export async function fetchPublicStore(
	storeCode: string,
	opts: PublicTransportOptions & {
		signal?: AbortSignal;
		timeoutMs?: number;
	} = {},
): Promise<PublicStoreView | null> {
	const { signal, timeoutMs, ...transport } = opts;
	if (!isStoreCodeShape(storeCode)) return null;
	let res: Awaited<
		ReturnType<ReturnType<typeof sellerPublicClient>["getStore"]>
	>;
	try {
		res = await sellerPublicClient(transport).getStore(
			{ storeCode },
			{ signal, timeoutMs },
		);
	} catch (err) {
		if (isStoreAbsentError(err)) return null;
		throw err;
	}
	const store = res.store;
	if (!store?.id) throw new Error("GetStore returned a store without id");
	const town = store.townCode ? townByCode(store.townCode) : undefined;
	const stats = res.stats;
	return {
		storeId: store.id,
		storeCode: store.storeCode,
		name: store.storeName,
		description: store.description,
		prize: store.prize,
		websiteUrl: store.websiteUrl,
		logo_url: imageUri(store.logo, "avatar") ?? "",
		coverImage: imageUri(store.coverImage, "banner") ?? "",
		city: town?.countyName ?? "",
		district: town?.name ?? "",
		...(stats?.followerCount !== undefined
			? { followerCount: stats.followerCount }
			: {}),
		...(stats?.reviewCount !== undefined
			? { reviewCount: stats.reviewCount }
			: {}),
		...(stats?.ratingAverage !== undefined
			? { ratingAverage: stats.ratingAverage }
			: {}),
	};
}
