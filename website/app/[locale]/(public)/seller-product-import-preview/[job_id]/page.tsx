import { PreviewListingPage } from "@/features/seller-center/products-import";

export const metadata = { title: "匯入預覽" };

export default async function Page({
	params,
}: {
	params: Promise<{ job_id: string }>;
}) {
	const { job_id } = await params;
	return <PreviewListingPage jobId={job_id} />;
}
