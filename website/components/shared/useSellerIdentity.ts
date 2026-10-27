"use client";

import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import {
	isStoreMissing,
	isStorePendingDeletion,
	isStoreSuspended,
	needsOnboarding,
	storeImageUrls,
} from "@/lib/seller/store";

export function useSellerIdentity() {
	const { data: self, error, loading } = useSellerSelf();

	// 有賣家角色但還沒有店家（GetMyStore 回 not_found）：SellerCenterShell 用這個旗標把人導去
	// onboarding 補建店家，而不是讓整個工作區對著錯誤擺爛。
	const profileMissing =
		isStoreMissing(error) ||
		(!loading && !error && self !== null && !self.store?.id);

	const store = self?.store;
	const storeName = store?.storeName || "我的商店";
	// 賣家名稱就是店名（沒有另一個欄位）。
	const ownerName = store?.storeName || "";
	// 賣家後台 chrome 顯示用 ID：storeCode（10 碼對外顯示碼）。
	const sellerId = store?.storeCode || store?.id || "";
	const avatar = storeImageUrls(store).logoUrl ?? null;

	/** 開店申請未完成（沒店家，或店家沒有完成新手引導）：賣家工作區入口
	 * 應鎖定並引導去 /seller/onboarding。預設 false——已完成的賣家不會閃鎖。 */
	const setupIncomplete =
		profileMissing || (self !== null && needsOnboarding(self));

	/** 生效中的停權（任何一筆生效中的停權措施，伺服器的 `suspended`）。 */
	const isSuspended = isStoreSuspended(self);
	/** 店家隨帳號刪除、等待未結訂單完成：只能處理訂單與售後。 */
	const isPendingDeletion = isStorePendingDeletion(self);

	return {
		avatar,
		isPendingDeletion,
		isSuspended,
		loading,
		ownerName,
		profileMissing,
		seller: self,
		sellerId,
		setupIncomplete,
		storeName,
	};
}
