import OrderDetailClient from "@/features/seller-center/orders/OrderDetailClient";

export const metadata = { title: "訂單詳情 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ orderNo: string }>;
}) {
	const { orderNo } = await params;
	return <OrderDetailClient orderNo={decodeURIComponent(orderNo)} />;
}
