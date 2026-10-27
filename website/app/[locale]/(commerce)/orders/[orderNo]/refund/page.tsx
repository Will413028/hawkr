import RefundResultClient from "@/features/after-sales/screens/RefundResultClient";

export const metadata = { title: "退款結果" };

/**
 * PayUni ATM-refund web return landing. The aftersales-service builds this URL
 * (`{WEB_FRONTEND_URL}/orders/{orderNo}/refund?status=…&trade_no=…`) only for
 * the WEB_BROWSER client; native apps get a `hawkrbuyer://` deep link
 * instead. See `fetchRefundPage` / design doc 09 §5.4.2.
 */
export default async function Page({
	params,
}: {
	params: Promise<{ orderNo: string }>;
}) {
	const { orderNo } = await params;
	return <RefundResultClient orderNo={orderNo} />;
}
