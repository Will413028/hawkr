import DelegatedProductEditPage from "@/features/delegated-operations/products/DelegatedProductEditPage";

/**
 * 代行工作區 — 編輯商品（mockup image6／§6.1.8）。
 *
 * 路由帶 `{productId}`，但**目標店家不由路由決定** —— 它由 operation credential
 * 對應的工作階段決定，路徑上的 `{storeId}` 只用於顯示與 URL 可讀性。
 */
export const metadata = { title: "編輯商品 — 代行工作區" };

export default async function Page({
	params,
}: {
	params: Promise<{ productId: string }>;
}) {
	const { productId } = await params;
	return <DelegatedProductEditPage productId={productId} />;
}
