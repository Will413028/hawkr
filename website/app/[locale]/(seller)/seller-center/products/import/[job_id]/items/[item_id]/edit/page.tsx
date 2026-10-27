import { ItemEditPage } from "@/features/seller-center/products-import";

export const metadata = { title: "編輯匯入商品 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ job_id: string; item_id: string }>;
}) {
	const { job_id, item_id } = await params;
	return <ItemEditPage jobId={job_id} itemId={item_id} />;
}
