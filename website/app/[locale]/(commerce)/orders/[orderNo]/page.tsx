import PaymentLandingClient from "@/features/payments/screens/PaymentLandingClient";

/**
 * PayUni Web redirect landing for /orders/{orderNo}?status=&trade_no=&code=
 * (issue #760).
 *
 * The page itself is intentionally minimal — all polling, status
 * classification, and routing decisions live in the client component
 * via `usePaymentResolution`. When a buyer reaches this route without
 * any PayUni query params (typed URL, bookmark), the client redirects
 * them to the canonical `/account/orders/{orderNo}` detail surface.
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
