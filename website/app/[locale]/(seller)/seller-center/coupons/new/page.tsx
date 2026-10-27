import CouponFormClient from "@/features/seller-center/coupons/CouponFormClient";

export const metadata = { title: "建立優惠券 — 賣家中心" };

export default async function Page({
	searchParams,
}: {
	searchParams: Promise<{ buyerId?: string; buyerName?: string }>;
}) {
	const { buyerId, buyerName } = await searchParams;
	// Deep-link from the customer-detail「發優惠券」CTA pre-targets the form at
	// one buyer; without it the form opens on the normal「全體買家」path.
	const presetBuyer = buyerId ? { id: buyerId, name: buyerName } : undefined;
	return <CouponFormClient mode="create" presetBuyer={presetBuyer} />;
}
