import { StorefrontPage } from "@/features/sellers/storefront";

export const metadata = { title: "店面預覽 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	return <StorefrontPage sellerId={id} previewMode />;
}
