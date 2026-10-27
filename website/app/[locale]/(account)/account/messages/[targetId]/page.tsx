import ThreadClient from "@/features/account/screens/messages/ThreadClient";

export const metadata = { title: "對話" };

export default async function Page({
	params,
}: {
	params: Promise<{ targetId: string }>;
}) {
	const { targetId } = await params;
	return <ThreadClient targetId={targetId} backHref="/account/messages" />;
}
