import TrackingClient from "@/features/account/screens/orders/TrackingClient";

export const metadata = { title: "物流追蹤" };

export default async function Page({
	params,
}: {
	params: Promise<{ orderNo: string }>;
}) {
	const { orderNo } = await params;
	return <TrackingClient orderNo={orderNo} />;
}
