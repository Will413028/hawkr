import BuyerInvoiceListClient from "@/features/invoices/screens/BuyerInvoiceListClient";

export const metadata = { title: "發票與收據" };
// 禁公開快取：本頁內容因人而異（58A §B09 權限與邊界）。
export const dynamic = "force-dynamic";

export default function Page() {
	return <BuyerInvoiceListClient />;
}
