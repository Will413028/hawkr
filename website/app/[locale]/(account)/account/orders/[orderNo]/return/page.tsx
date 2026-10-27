import ReturnClient from "@/features/account/screens/orders/ReturnClient";

export const metadata = { title: "申請退換貨" };

export default async function Page({
	params,
}: {
	params: Promise<{ orderNo: string }>;
}) {
	const { orderNo } = await params;
	return <ReturnClient orderNo={orderNo} />;
}
