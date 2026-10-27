import { JobDetailPage } from "@/features/seller-center/products-import";

export const metadata = { title: "批次匯入 詳情 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ job_id: string }>;
}) {
	const { job_id } = await params;
	return <JobDetailPage jobId={job_id} />;
}
