import AfterSaleDetailClient from "@/features/account/screens/after-sales/AfterSaleDetailClient";

export const metadata = { title: "售後案件詳情" };

export default async function Page({
	params,
}: {
	params: Promise<{ caseNo: string }>;
}) {
	const { caseNo } = await params;
	return <AfterSaleDetailClient caseNo={caseNo} />;
}
