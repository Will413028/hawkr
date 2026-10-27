import ReviewClient from "@/features/account/screens/orders/ReviewClient";

export const metadata = { title: "評價商品" };

export default async function Page({
	params,
}: {
	params: Promise<{ orderNo: string }>;
}) {
	const { orderNo } = await params;
	return <ReviewClient orderNo={orderNo} />;
}
