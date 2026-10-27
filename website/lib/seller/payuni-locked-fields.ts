/**
 * 認證資料送到 PayUni 之後被鎖定的商店欄位。
 *
 * `storeName`／`description` 對應 PayUni 的 `MerName`／`MerDesc`，是 `bind_merchant` 建立後
 * 不可改的欄位——`update_merchant` 碰不到它們。鎖定由伺服器推導（`GetMyKYC` 的
 * `flow.identity_locked`：註冊已有 MerID 且綁定沒有失敗），這裡只是提示與 UI 阻擋，
 * 不是防線：最終防線在 `UpdateMyStore`。
 */

import type { StoreChanges } from "./store-form";

/**
 * 資料已送到 PayUni 之後的鎖定說明。這時我們這邊誰都改不了，連客服代行也不行。
 *
 * 與 seller App 的 `kPayuniLockedNotice` 逐字相同，兩邊改動要一起改。
 */
export const PAYUNI_LOCKED_NOTICE =
	"資料已送至 PayUni，店名與商店介紹無法修改，如需變更請聯繫 PayUni";

/**
 * 送審確認畫面的提醒。鎖定發生在 PayUni 收下資料之後，那之前是賣家最後有選擇權
 * 的時點。
 *
 * 與 seller App 的 `kPayuniLockOnSubmitNotice` 措辭對齊（欄位名依各平台標籤）。
 */
export const PAYUNI_LOCK_ON_SUBMIT_NOTICE =
	"送出後商店名稱與商店介紹將無法修改，請先確認無誤";

/** 被鎖住的欄位。 */
export const PAYUNI_LOCKED_FIELDS = ["storeName", "description"] as const;

/** 商店名稱／商店介紹在這個狀態下該顯示哪句鎖定說明；`undefined` 表示可以改。 */
export function payuniLockedNoticeFor(
	identityLocked: boolean | undefined,
): string | undefined {
	return identityLocked ? PAYUNI_LOCKED_NOTICE : undefined;
}

/**
 * 送出前把鎖定欄位從變更拿掉。
 *
 * 不能只靠把 input 設成唯讀：改 DOM 就能繞過。這一層擋的是「前端不該送出一個
 * 必定被伺服器拒絕的請求」——真正的權威仍是伺服器。
 */
export function stripPayuniLockedFields(
	changes: StoreChanges,
	locked: boolean,
): StoreChanges {
	if (!locked) return changes;
	const next: StoreChanges = { ...changes };
	for (const field of PAYUNI_LOCKED_FIELDS) {
		delete next[field];
	}
	return next;
}
