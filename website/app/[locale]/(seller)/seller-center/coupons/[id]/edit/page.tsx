import CouponFormClient from "@/features/seller-center/coupons/CouponFormClient";

export const metadata = { title: "編輯優惠券 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	if (!/^\d+$/.test(id)) {
		return <CouponFormClient mode="create" />;
	}
	return <CouponFormClient mode="edit" couponId={id} />;
}
