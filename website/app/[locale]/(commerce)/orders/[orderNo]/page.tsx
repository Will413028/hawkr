import PaymentLandingClient from "@/features/payments/screens/PaymentLandingClient";

/**
 * 付款步驟 `/orders/{orderNo}`：結帳確認訂單後（或訂單詳情的「再次付款」）進來，
 * 由 client 元件 CreatePayment、以 Stripe Payment Element 收款，結果以 GetPayment 為準。
 * 訪客導向 `/account/orders/{orderNo}`，由受保護區帶去登入。
 */
export const metadata = { title: "付款處理中" };

interface PageProps {
	params: Promise<{ orderNo: string }>;
}

export function generateStaticParams() {
	// A sentinel PPR shell keeps the dynamic param from turning the whole route
	// into a blocking render. No real order number is ever prerendered.
	return [{ orderNo: "0" }];
}

export default async function Page({ params }: PageProps) {
	const { orderNo } = await params;
	return <PaymentLandingClient orderNo={orderNo} />;
}
