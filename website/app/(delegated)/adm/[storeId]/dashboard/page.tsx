import DelegatedDashboardPage from "@/features/delegated-operations/dashboard/DelegatedDashboardPage";

/**
 * 代行工作區・賣家總覽（§6.1.14 第 1 列）。
 *
 * 總覽在 `nav-policy.ts` 是**可操作**的（capability 為 `null`＝不由單一 capability
 * 決定），所以它必須有具名路由：少了這個目錄，側欄的第一項會落到同層的
 * `[...area]` catch-all，變成「選單項是 active、主區卻寫著不在授權範圍」——
 * 唯一可操作卻長得像禁區的模組。
 */
export const metadata = { title: "賣家總覽 — 代行工作區" };

export default function Page() {
	return <DelegatedDashboardPage />;
}
