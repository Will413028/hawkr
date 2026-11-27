import { SellerInvoiceJobPage } from "@/features/seller-center/invoices";

export const metadata = { title: "開立異常詳情 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ jobId: string }>;
}) {
	const { jobId } = await params;
	return <SellerInvoiceJobPage jobId={jobId} />;
}
