import { getCachedStore } from "@/lib/api/cached";
import type { PublicStoreView } from "./public-store";

export type StorefrontSeller = Partial<PublicStoreView>;

export interface ResolvedStorefront {
	seller: StorefrontSeller;
	/** 其餘讀取（商品、評價、貼文、討論區、追蹤）的參照鍵，不是網址段。 */
	storeId: string;
}

/**
 * 店面各分頁共用的店家解析。公開店面的網址段是 store_code，以 `GetStore` 讀並取回 `store.id`。
 */
export async function resolveStorefront(
	routeId: string,
): Promise<ResolvedStorefront | null> {
	const store = await getCachedStore(routeId);
	return store ? { seller: store, storeId: store.storeId } : null;
}
