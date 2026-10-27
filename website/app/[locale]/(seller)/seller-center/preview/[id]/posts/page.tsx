import SellerPostsPage from "@/features/sellers/posts/SellerPostsPage";

export const metadata = { title: "店面預覽 貼文 — 賣家中心" };

export default function Page({
	params,
}: {
	params: Promise<{ locale: string; id: string }>;
}) {
	return <SellerPostsPage params={params} />;
}
