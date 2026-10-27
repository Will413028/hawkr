import InvoiceDocumentDetailClient from "@/features/invoices/screens/InvoiceDocumentDetailClient";

export const metadata = { title: "發票與收據" };

export default async function Page({
	params,
}: {
	params: Promise<{ documentId: string }>;
}) {
	const { documentId } = await params;
	return <InvoiceDocumentDetailClient documentId={documentId} />;
}
