import MemberCardDetailClient from "@/features/account/screens/wallet/MemberCardDetailClient";

export const metadata = { title: "會員卡明細" };

export default async function Page({
	params,
	searchParams,
}: {
	params: Promise<{ sellerId: string }>;
	searchParams: Promise<{ name?: string }>;
}) {
	const [{ sellerId }, { name }] = await Promise.all([params, searchParams]);
	return <MemberCardDetailClient sellerId={sellerId} sellerName={name} />;
}
