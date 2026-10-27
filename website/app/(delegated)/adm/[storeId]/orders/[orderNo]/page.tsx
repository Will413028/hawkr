import DelegatedOrderDetailRoute from "@/features/delegated-operations/orders/DelegatedOrderDetailRoute";

/**
 * 訂單詳情的獨立路由（深連結／重新整理的落點）。
 *
 * 列表頁的「查看詳情」開的是右抽屜（mockup image8 的形狀），這一支存在的理由與賣家
 * 中心的 `/seller-center/orders/[orderNo]` 相同：把一筆訂單的網址貼給別人、或在抽屜
 * 開著時重新整理，都要有一個真的頁面接得住。兩邊渲染的是**同一個**
 * `DelegatedOrderDetailPanel`，不是第二份實作。
 */
export const metadata = { title: "訂單詳情 — 代行工作區" };

export default async function Page({
	params,
}: {
	params: Promise<{ storeId: string; orderNo: string }>;
}) {
	const { storeId, orderNo } = await params;
	return <DelegatedOrderDetailRoute storeId={storeId} orderNo={orderNo} />;
}
