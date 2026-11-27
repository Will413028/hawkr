import CouponFormClient from "@/features/seller-center/coupons/CouponFormClient";

export const metadata = { title: "建立優惠券 — 賣家中心" };

export default async function Page({
	searchParams,
}: {
	searchParams: Promise<{ buyerId?: string; buyerName?: string }>;
}) {
	const { buyerId, buyerName } = await searchParams;
	const presetBuyer = buyerId ? { id: buyerId, name: buyerName } : undefined;
	return <CouponFormClient mode="create" presetBuyer={presetBuyer} />;
}
