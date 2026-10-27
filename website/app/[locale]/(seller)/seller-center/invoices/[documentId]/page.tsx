import { SellerInvoiceDetailPage } from "@/features/seller-center/invoices";

export const metadata = { title: "發票詳情 — 賣家中心" };

/**
 * S11。⚠️ 這個動態段與同層的 `documents`／`tax-status`／`appeals`／`jobs` 四個靜態段
 * 並存：App Router 的靜態段優先於動態段，所以 `/seller-center/invoices/documents`
 * 仍由 `documents/page.tsx` 承接，不會落到這裡。新增同層靜態路由時不必改本檔，
 * 但要記得那個字串從此不可能成為 documentId。
 */
export default async function Page({
	params,
}: {
	params: Promise<{ documentId: string }>;
}) {
	const { documentId } = await params;
	return <SellerInvoiceDetailPage documentId={documentId} />;
}
