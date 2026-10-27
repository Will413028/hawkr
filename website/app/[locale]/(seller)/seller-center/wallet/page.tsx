import WalletClient from "@/features/seller-center/wallet/WalletClient";

export const metadata = { title: "錢包與認證 — 賣家中心" };

export default async function Page({
	searchParams,
}: {
	searchParams: Promise<{ section?: string | string[] }>;
}) {
	const params = await searchParams;
	const section = Array.isArray(params.section)
		? params.section[0]
		: params.section;
	return <WalletClient initialFoodOpen={section === "food-registration"} />;
}
