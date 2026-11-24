"use client";

import { useTranslations } from "next-intl";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";

export type PaymentStatusKind =
	| "waiting"
	| "succeeded"
	| "anomaly"
	| "timeout"
	| "error";

interface PaymentStatusCardProps {
	kind: PaymentStatusKind;
	orderNo: string;
	/** `error` 的文字（伺服器或 `errorText` 給的）；其他狀態用目錄文字。 */
	message?: string;
}

/** 付款步驟與回跳頁共用的狀態卡：確認中、成功導向中、人工處理、逾時、錯誤。 */
export default function PaymentStatusCard({
	kind,
	orderNo,
	message,
}: PaymentStatusCardProps) {
	const t = useTranslations("payment");
	const waiting = kind === "waiting";
	const body =
		kind === "waiting"
			? t("confirmingHint")
			: kind === "succeeded"
				? t("succeeded")
				: kind === "anomaly"
					? t("anomaly")
					: kind === "timeout"
						? t("timeout")
						: message;
	return (
		<div className="min-h-[calc(100vh-200px)] max-w-[580px] mx-auto py-10 px-5 pb-12 flex items-center justify-center">
			<div
				className="relative w-full bg-content1/90 border border-divider rounded-3xl p-10 px-7 pb-8 text-center flex flex-col items-center gap-3 shadow-hk-card overflow-hidden animate-checkout-pop"
				role="status"
			>
				{waiting || kind === "succeeded" ? (
					<span
						className="m-0 my-4 inline-block text-hk-primary"
						aria-hidden="true"
					>
						{waiting ? (
							<AppIcon name="spinner" size={52} className="animate-spin" />
						) : (
							<AppIcon name="check" size={52} />
						)}
					</span>
				) : null}
				<h1 className="relative z-1 font-display text-xl md:text-2xl m-0 text-foreground">
					{waiting ? t("confirming") : t("title")}
				</h1>
				{body ? (
					<p className="relative z-1 m-0 text-foreground-soft text-sm max-w-[38ch]">
						{body}
					</p>
				) : null}
				{orderNo ? (
					<p className="relative z-1 m-0 text-xs font-bold text-foreground-muted">
						{t("orderNo")} {orderNo}
					</p>
				) : null}
				{kind === "timeout" || kind === "anomaly" ? (
					<Button href="/account/orders" variant="bordered" size="sm">
						{t("viewOrders")}
					</Button>
				) : null}
			</div>
		</div>
	);
}
