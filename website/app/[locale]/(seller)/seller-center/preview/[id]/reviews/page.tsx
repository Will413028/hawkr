import SellerReviewsPage from "@/features/sellers/reviews/SellerReviewsPage";

export const metadata = { title: "店面預覽 買家評價 — 賣家中心" };

export default function Page({
	params,
}: {
	params: Promise<{ locale: string; id: string }>;
}) {
	return <SellerReviewsPage params={params} />;
}
