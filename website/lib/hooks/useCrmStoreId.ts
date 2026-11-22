"use client";

import { storeIdOf } from "@/lib/seller/store";
import { useSellerSelf } from "./useSellerSelf";

/** CRM 每個方法帶的店家 id（`GetMyStore` 的店）；店還沒讀到或沒有店是 undefined。 */
export function useCrmStoreId(): {
	storeId: string | undefined;
	loading: boolean;
} {
	const { data, loading } = useSellerSelf();
	return { storeId: storeIdOf(data), loading };
}
