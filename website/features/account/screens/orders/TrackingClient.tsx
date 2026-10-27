"use client";

import * as React from "react";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { formatTaipeiDateTime as formatTime } from "@/lib/date/taipei";
import { useOrder } from "@/lib/hooks/useOrders";
import { type LogisticsTrack, queryLogistics } from "@/lib/logistics/repo";

export default function TrackingClient({ orderNo }: { orderNo: string }) {
	const { order } = useOrder(orderNo);
	const [track, setTrack] = React.useState<LogisticsTrack | null>(null);
	const [loading, setLoading] = React.useState(false);
	/**
	 * 🔴 `retryable` 不是裝飾：**403 給重試鍵是叫使用者按一顆永遠不會亮的燈**。
	 * 舊行為把所有失敗（含「這不是你的訂單」）壓成同一句「請稍後再試」＋重試鍵，
	 * 於是無權查看的人會一直按、一直看到同一句話，然後才去找客服。
	 */
	const [error, setError] = React.useState<{
		message: string;
		retryable: boolean;
	} | null>(null);

	const trackingNo = order?.shipping?.trackingNo;

	const load = React.useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const r = await queryLogistics(orderNo);
			setTrack(r);
		} catch (e) {
			// 原始 HTTP 碼不外露，但**可行動性**要外露：403 再重試幾次都不會變。
			const status =
				e != null && typeof e === "object" && "status" in e
					? (e as { status?: number }).status
					: undefined;
			setError(
				status === 403
					? {
							message:
								"這筆訂單的物流資訊不屬於這個帳號，請確認登入的帳號是否正確。",
							retryable: false,
						}
					: { message: "目前無法載入物流資訊，請稍後再試。", retryable: true },
			);
		} finally {
			setLoading(false);
		}
	}, [orderNo]);

	React.useEffect(() => {
		void load();
	}, [load]);

	// 後端回傳但無託運單號（例：尚未交運）
	const hasTracking = Boolean(track?.shipTradeNo);
	const subtitleTrackingNo = track?.shipTradeNo || trackingNo;

	return (
		<main>
			<SellerPageShell
				title="物流追蹤"
				subtitle={`訂單 #${orderNo}${subtitleTrackingNo ? `　·　託運單號 ${subtitleTrackingNo}` : ""}`}
				actions={
					<Button
						href={`/account/orders/${orderNo}`}
						variant="bordered"
						color="default"
						size="sm"
					>
						← 回訂單詳情
					</Button>
				}
			>
				{loading && !track ? (
					<p className="">查詢中…</p>
				) : error ? (
					<>
						<p
							className=""
							role="alert"
							data-testid={
								error.retryable ? "tracking-error" : "tracking-forbidden"
							}
						>
							{error.message}
						</p>
						{error.retryable ? (
							<Button
								variant="bordered"
								color="default"
								onClick={() => void load()}
							>
								重試
							</Button>
						) : null}
					</>
				) : track && !hasTracking ? (
					<div className="flex flex-col items-center justify-center gap-[var(--hkui-space-3)] py-[var(--hkui-space-8)] px-[var(--hkui-space-5)] text-center text-[var(--hkui-foreground-soft)]">
						<AccountEmptyStateIcon variant="tracking" />
						<p className="text-[length:var(--text-base)] font-extrabold text-[var(--hkui-foreground)] m-0">
							尚無物流追蹤資訊
						</p>
						<p className="max-w-[32ch] text-[0.8125rem] leading-[1.6] m-0">
							賣家還沒交運包裹，或此配送方式不提供物流追蹤，請稍後再回來查看。
						</p>
					</div>
				) : track ? (
					<>
						<section className="bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)] p-[var(--hkui-space-5)] grid grid-cols-[repeat(auto-fit,minmax(140px,1fr))] gap-[var(--hkui-space-4)] mb-[var(--hkui-space-4)]">
							<div className="flex flex-col gap-1">
								<small className="text-[length:var(--hkui-text-tiny)] text-[color:var(--hkui-foreground-soft)]">
									目前狀態
								</small>
								<strong className="text-[length:var(--hkui-text-medium)] text-[color:var(--hkui-primary)]">
									{track.statusLabel || track.providerMessage || "處理中"}
								</strong>
							</div>
							{track.statusTime && (
								<div className="flex flex-col gap-1">
									<small className="text-[length:var(--hkui-text-tiny)] text-[color:var(--hkui-foreground-soft)]">
										狀態更新時間
									</small>
									<strong className="text-[length:var(--hkui-text-medium)] text-[color:var(--hkui-primary)]">
										{formatTime(track.statusTime)}
									</strong>
								</div>
							)}
						</section>

						{(track.recipientName || track.storeName) && (
							<section className="bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)] p-[var(--hkui-space-5)]">
								<h2 className="text-[length:var(--hkui-text-medium)] font-semibold m-0 mb-[var(--hkui-space-2)]">
									收件資訊
								</h2>
								{track.recipientName && (
									<p className="m-0 text-[length:var(--hkui-text-small)] text-[color:var(--hkui-foreground-muted)]">
										收件人：{track.recipientName}
									</p>
								)}
								{track.storeName && (
									<p className="m-0 text-[length:var(--hkui-text-small)] text-[color:var(--hkui-foreground-muted)]">
										取貨門市：{track.storeName}
									</p>
								)}
							</section>
						)}
					</>
				) : null}
			</SellerPageShell>
		</main>
	);
}
