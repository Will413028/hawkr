/**
 * Buyer logistics tracking — GET /app/orders/{orderNo}/logistics.
 *
 * 後端即時向 PayUni 同步當前狀態並回傳單點狀態（PayUni 物流查詢 API 僅提供
 * 當前狀態 + 時間，無多筆軌跡歷史，故無時間線）。Ownership 由後端以 order_no
 * 對應的 shipment.buyer_account_id 驗證。
 */

import { authedFetch } from "../api/client";

export interface LogisticsTrack {
	orderNo: string;
	shipTradeNo: string;
	shippingMethod: string;
	status: string;
	statusLabel: string;
	providerStatus?: string;
	providerMessage?: string;
	statusTime?: string;
	storeName?: string;
	recipientName?: string;
	createdAt?: string;
}

export async function queryLogistics(orderNo: string): Promise<LogisticsTrack> {
	const env = await authedFetch<Record<string, unknown>>(
		`/app/orders/${encodeURIComponent(orderNo)}/logistics`,
		{ method: "GET" },
	);
	const d = env.data ?? {};
	return {
		orderNo: String(d.orderNo ?? orderNo),
		shipTradeNo: String(d.shipTradeNo ?? ""),
		shippingMethod: String(d.shippingMethod ?? ""),
		status: String(d.status ?? ""),
		statusLabel: String(d.statusLabel ?? ""),
		providerStatus: d.providerStatus as string | undefined,
		providerMessage: d.providerMessage as string | undefined,
		statusTime: (d.statusTime as string) || undefined,
		storeName: (d.storeName as string) || undefined,
		recipientName: (d.recipientName as string) || undefined,
		createdAt: (d.createdAt as string) || undefined,
	};
}
