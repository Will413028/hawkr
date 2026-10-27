import CustomerDetailClient from "@/features/seller-center/customers/CustomerDetailClient";

export const metadata = { title: "客戶詳情 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ buyerId: string }>;
}) {
	const { buyerId } = await params;
	return <CustomerDetailClient buyerId={decodeURIComponent(buyerId)} />;
}
