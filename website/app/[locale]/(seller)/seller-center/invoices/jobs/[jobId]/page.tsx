import { SellerInvoiceJobPage } from "@/features/seller-center/invoices";

// 🔴 這是 server component 的 static metadata，拿不到 job 的 documentKind ⇒ 只能中性。
// 頁面內的標題（SellerPageShell 的 title）才是依 kind 切的那一個（#3066 裁決 B）。
// 「開立」對發票與收據都成立，不會對免用統一發票的賣家講一件他不會遇到的事。
export const metadata = { title: "開立異常詳情 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ jobId: string }>;
}) {
	const { jobId } = await params;
	return <SellerInvoiceJobPage jobId={jobId} />;
}
