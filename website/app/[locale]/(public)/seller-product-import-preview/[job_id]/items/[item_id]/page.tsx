import { PreviewDetailPage } from "@/features/seller-center/products-import";

export const metadata = { title: "匯入商品預覽" };

export default async function Page({
	params,
}: {
	params: Promise<{ job_id: string; item_id: string }>;
}) {
	const { job_id, item_id } = await params;
	return <PreviewDetailPage jobId={job_id} itemId={item_id} />;
}
