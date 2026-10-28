import type { MaterialSymbolName } from "@hawkr/icons/material-symbol";
import type { StatCardVariant } from "@/components/UI/StatCard";

/**
 * A01 發票營運總覽的 KPI 呈現規則（端點 37）。
 *
 * 放在 feature 模組而不是頁面裡，是為了讓契約測試能在不啟動 React 的情況下
 * 驗值域與導向——「8 個 key 逐字相同」這件事打錯不會報錯，只會少一張卡。
 */

export interface KpiMeta {
	/**
	 * 卡片標題。
	 *
	 * ⚠️ 由前端提供是因為 wire 上沒有這個欄位：`InvoiceKpiItem` 只有 `key` /
	 * `value` / `windowLabel` / `definition`。**口徑（windowLabel、definition）一律
	 * 用後端下發的，前端一個字都不寫**——那兩個欄位是「這個數字怎麼算出來的」，
	 * 前端自己寫一份等於造第二個真相。
	 */
	label: string;
	icon: MaterialSymbolName;
	variant: StatCardVariant;
	/** 點擊導向；省略 ＝ 這張卡不可點。 */
	href?: string;
	/** 卡片口徑下方的補充（目前只有第 8 張需要）。 */
	note?: string;
}

/**
 * 8 個 KPI 的封閉列舉，順序固定（key 逐字對 58 §6.1.11a）。
 *
 * ⚠️ **四張卡刻意不帶篩選**（`pending_tax_reviews` / `provider_errors` /
 * `pending_jobs` / `open_complaints`）：KPI 的口徑是**一組狀態的集合**，而列表端點
 * 的篩選只吃一個值 ⇒ 帶單值 filter 會少算，兩個數字對不上而沒有任何訊號。所以
 * 導向不帶篩選的列表頁，UI 上也不暗示「已為你篩好」。
 *
 * ⚠️ `variant` 只有四值（`StatCardVariant`），不得新增——新增一個 variant 等於
 * 新增一個 hex，會被色值棘輪擋住。
 */
export const KPI_META: Record<string, KpiMeta> = {
	pending_tax_reviews: {
		label: "待審稅務身分",
		icon: "account_balance",
		variant: "warning",
		href: "/admin/invoices/tax-reviews",
	},
	near_deadline: {
		label: "即將到期的審核",
		icon: "timer",
		variant: "warning",
		href: "/admin/invoices/tax-reviews?filter=NEAR_DEADLINE",
	},
	provider_errors: {
		label: "AMEGO 串接異常",
		icon: "electrical_services",
		variant: "danger",
		href: "/admin/invoices/provider-accounts",
	},
	pending_jobs: {
		label: "待處理發票工作",
		icon: "list_alt",
		variant: "default",
		href: "/admin/invoices/jobs",
	},
	needs_manual: {
		label: "需人工處理",
		icon: "person_search",
		variant: "danger",
		href: "/admin/invoices/jobs?tab=NEEDS_MANUAL",
	},
	open_complaints: {
		label: "未結案發票申訴",
		icon: "support_agent",
		variant: "warning",
		href: "/admin/invoices/complaints",
	},
	overdue_complaints: {
		label: "逾期申訴",
		icon: "crisis_alert",
		variant: "danger",
		href: "/admin/invoices/complaints?tab=OVERDUE",
	},
	suspended_sellers: {
		label: "因發票違規停權",
		icon: "block",
		variant: "danger",
		// 沒有 `/admin/violations` 這條路由；`merchants?filter=suspended` 是支援的 deep-link（商家清單的停權篩選）。
		href: "/admin/merchants?filter=suspended",
		note: "點擊後看到的是全部停權中的賣家；個別賣家的措施請到商家詳情的「措施歷史」查看。",
	},
};

/** 卡片呈現順序。後端回了清單以外的 key 時排在後面，不靜默丟掉。 */
export const KPI_ORDER = Object.keys(KPI_META);

/**
 * 未登錄 key 的 default 分支。
 *
 * 後端先上線一個前端還不知道的 KPI 時，這張卡以 key 當標題、不可點地出現，
 * 而不是整頁白畫面（exhaustive 對照最常見的死法）或靜默消失。
 */
export function kpiMetaOf(key: string): KpiMeta {
	return (
		KPI_META[key] ?? {
			label: key,
			icon: "help",
			variant: "default",
		}
	);
}

/**
 * KPI 值的顯示。
 *
 * 🔴 空字串 ＝ **這個數字取不到**，不是 0。`suspended_sellers` 的權威是
 * seller 模組的停權措施，發票服務不持有 ⇒ 後端刻意回空字串而不是 "0"。
 * 印成 0 看起來就像「今天沒有人被停權」，而那是一句後端拒絕說的謊。
 */
export function kpiDisplayValue(value: string, loading: boolean): string {
	if (loading) return "--";
	return value === "" ? "取不到" : value;
}
