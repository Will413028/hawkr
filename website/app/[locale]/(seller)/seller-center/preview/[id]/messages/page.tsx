import SellerMessagesPage from "@/features/sellers/messages/SellerMessagesPage";

export const metadata = { title: "店面預覽 留言 — 賣家中心" };

export default function Page({
	params,
}: {
	params: Promise<{ locale: string; id: string }>;
}) {
	return <SellerMessagesPage params={params} />;
}
