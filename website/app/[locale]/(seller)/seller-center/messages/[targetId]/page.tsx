import ThreadClient from "@/features/account/screens/messages/ThreadClient";

export const metadata = { title: "對話 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ targetId: string }>;
}) {
	const { targetId } = await params;
	return (
		<ThreadClient targetId={targetId} backHref="/seller-center/messages" />
	);
}
