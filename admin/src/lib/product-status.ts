import type { MaterialSymbolName } from "@hawkr/icons/material-symbol";
import type { BadgeVariant } from "@/components/UI";
import { ProductStatus } from "@/lib/codemeta";

/**
 * 商品狀態的**呈現 token**（顏色 class 與 icon）—— 後端不下發這些，所以它們留在前端。
 *
 * ⚠️ 這裡沒有 `label`，而且不會再有。顯示文字是 CodeMeta 的資產，一律
 * `codeLabel(PRODUCT_STATUS_LABEL, status)`。本檔曾經自己抄一份 label，抄的時候漏了
 * 4002（PENDING_REVIEW）—— 審核中的商品在三個頁面顯示「未知狀態(4002)」並套上
 * `rejected` 的紅色徽章，看起來像是被退回。手抄的碼表就是這樣壞的：漏一個值不會編譯錯、
 * 不會測試紅，只會在某一種資料出現時才在畫面上現形。
 *
 * 鍵直接引用生成的 `ProductStatus`，不重打一次數字：值域改了這張表跟著改，而不是在
 * 某個頁面留一組沒人發現已經過時的字面值。完整性由
 * `tests/codemeta-no-second-copy.test.ts` 守住（每個值都要有自己的 token，且彼此可分辨）。
 */
export interface StatusMeta {
	/** 全域 `.status-badge.<className>`（`src/app/globals.css`）—— 商品列表 / 商品詳情用。 */
	className: string;
	/** `<StatusBadge variant>` —— 商家詳情頁的商品分頁用。 */
	badgeVariant: BadgeVariant;
	icon: MaterialSymbolName;
}

const STATUS_META: Record<string, StatusMeta> = {
	[ProductStatus.DRAFT]: {
		className: "pending",
		badgeVariant: "gray",
		icon: "edit_document",
	},
	// 審核中必須與草稿一眼分得出來：兩者都「還不能賣」，但只有這一個在等平台動作。
	// 既有值域（pending / approved / rejected / disabled / gray）給不出第四個色調——
	// 灰會跟 4003 撞、紅是「被退回」的意思——所以走 info tone，色票用既有的
	// `--admin-info`，不另造近似色。官網賣家中心的 review 態也是 info ＋ 時鐘，兩邊一致。
	[ProductStatus.PENDING_REVIEW]: {
		className: "info",
		badgeVariant: "info",
		icon: "schedule",
	},
	[ProductStatus.INACTIVE]: {
		className: "disabled",
		badgeVariant: "warning",
		icon: "cancel",
	},
	[ProductStatus.ACTIVE]: {
		className: "approved",
		badgeVariant: "success",
		icon: "check_circle",
	},
};

/**
 * 值域外的碼：紅底問號。
 *
 * 刻意選最刺眼的一組，讓「SSOT 長出新成員而這張表沒跟上」在畫面上顯眼而不是靜靜混過去；
 * 文字側則交給 `codeLabel`，它對未命中回傳原碼值（全站一致的約定）。
 */
export const UNKNOWN_STATUS_META: StatusMeta = {
	className: "rejected",
	badgeVariant: "danger",
	icon: "help",
};

export function getStatusMeta(status: string): StatusMeta {
	return STATUS_META[status] ?? UNKNOWN_STATUS_META;
}
