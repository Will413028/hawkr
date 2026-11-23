"use client";

import { isStoreMissing, storeIdOf } from "@/lib/seller/store";
import { useSellerSelf } from "./useSellerSelf";

/**
 * CRM 每個方法帶的店家 id（`GetMyStore` 的店）；店還沒讀到或沒有店是 undefined。
 * 讀店失敗（不是「還沒開店」）放在 `error`：呼叫端要顯示讀取失敗，不能當成沒有客戶。
 */
export function useCrmStoreId(): {
	storeId: string | undefined;
	loading: boolean;
	error: Error | null;
} {
	const { data, loading, error } = useSellerSelf();
	return {
		storeId: storeIdOf(data),
		loading,
		error: error && !isStoreMissing(error) ? error : null,
	};
}
