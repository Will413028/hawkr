import { SellerInvoiceAppealDetailPage } from "@/features/seller-center/invoices";

export const metadata = { title: "申訴詳情 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ complaintId: string }>;
}) {
	const { complaintId } = await params;
	return <SellerInvoiceAppealDetailPage complaintId={complaintId} />;
}
