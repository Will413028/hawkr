import PostEditClient from "@/features/seller-center/posts/PostEditClient";

export const metadata = { title: "編輯動態 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	return <PostEditClient id={id} />;
}
