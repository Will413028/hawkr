import DelegatedProductCreatePage from "@/features/delegated-operations/products/DelegatedProductCreatePage";

/**
 * 代行工作區 — 上架新商品（mockup §4.4.2 的 CTA／§6.1.8 `POST .../products`）。
 *
 * 具名路由必須存在：`[...area]` 是同層的 catch-all，少了這一支，`＋ 上架新商品`
 * 會落進「不在本次代行授權範圍」空狀態頁 —— 一顆有 capability 才出現、按下去卻說
 * 沒授權的按鈕（這正是它上一輪被整顆移除的原因）。
 *
 * ⚠️ 目標店家不由路由決定：它由 operation credential 對應的工作階段決定，路徑上的
 * `{storeId}` 只用於顯示與 URL 可讀性。
 */
export const metadata = { title: "上架新商品 — 代行工作區" };

export default function Page() {
	return <DelegatedProductCreatePage />;
}
