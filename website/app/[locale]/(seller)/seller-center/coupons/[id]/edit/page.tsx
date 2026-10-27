import CouponFormClient from "@/features/seller-center/coupons/CouponFormClient";

export const metadata = { title: "編輯優惠券 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	// templateId 是 18 位 TSID（> 2^53），全程以字串傳遞，禁止 Number() 化
	// 否則精度丟失會讓詳情端點查無此券回 7001。
	if (!/^\d+$/.test(id)) {
		return <CouponFormClient mode="create" />;
	}
	return <CouponFormClient mode="edit" couponId={id} />;
}
