import StoreReportDetailClient from "@/features/account/screens/reports/StoreReportDetailClient";

export const metadata = { title: "檢舉詳情" };

export default async function Page({
	params,
}: {
	params: Promise<{ id: string }>;
}) {
	const { id } = await params;
	return <StoreReportDetailClient reportId={id} />;
}
