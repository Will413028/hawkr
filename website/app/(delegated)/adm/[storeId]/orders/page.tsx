import DelegatedOrdersPage from "@/features/delegated-operations/orders/DelegatedOrdersPage";

/**
 * 代行工作區・訂單管理（`orders.read`，唯讀）。
 *
 * 具名段永遠優先於同層的 `[...area]` catch-all，所以這一頁接得到 `/adm/{storeId}/
 * orders`，而尚未開放的模組仍落在 catch-all 的空狀態頁。
 */
export const metadata = { title: "訂單管理 — 代行工作區" };

export default function Page() {
	return <DelegatedOrdersPage />;
}
