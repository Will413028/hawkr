import DelegatedProductPreviewPage from "@/features/delegated-operations/products/DelegatedProductPreviewPage";

/**
 * 代行工作區 — 商品的買家視角預覽（56 號 §2.2 out-of-scope 第 13 列）。
 *
 * 具名路由而不是連去 `/products/{id}`：買家頁只有**已上架**的商品才有，草稿與已下架
 * 連過去是 404，而代行者最需要預覽的正是那兩種。留在 `/adm` 之下也才不會把工作階段
 * 的上下文丟掉。
 *
 * 與同層的其他頁一樣：目標店家由 operation credential 決定，路徑上的 `{storeId}`
 * 只用於顯示與 URL 可讀性。
 */
export const metadata = { title: "買家視角預覽 — 代行工作區" };

export default async function Page({
	params,
}: {
	params: Promise<{ productId: string }>;
}) {
	const { productId } = await params;
	return <DelegatedProductPreviewPage productId={productId} />;
}
