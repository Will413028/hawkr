import OrderDetailClient from "@/features/account/screens/orders/OrderDetailClient";

export const metadata = { title: "訂單詳情" };

export default async function Page({
	params,
}: {
	params: Promise<{ orderNo: string }>;
}) {
	const { orderNo } = await params;
	return <OrderDetailClient orderNo={orderNo} />;
}
