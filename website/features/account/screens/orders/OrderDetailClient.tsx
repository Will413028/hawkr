"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import OrderAfterSaleSummary from "@/features/account/screens/orders/OrderAfterSaleSummary";
import OrderPaymentCountdown from "@/features/account/screens/orders/OrderPaymentCountdown";
import { buyerOrderStatusBadgeClass } from "@/features/account/screens/orders/statusBadgeStyles";
import { OrderInvoiceSection } from "@/features/invoices/order/OrderInvoiceSection";
import OrderStatusTimeline from "@/features/orders/components/OrderStatusTimeline";
import {
	formatShippingMethodLabel,
	PAYMENT_METHOD_LABEL,
	PAYMENT_STATUS_LABEL,
	PaymentMethod,
	type PaymentMethodValue,
	PaymentStatus,
	type PaymentStatusValue,
} from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useMyAfterSales } from "@/lib/hooks/useMyAfterSales";
import { useOrder } from "@/lib/hooks/useOrders";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import {
	canBuyerCancelOrder,
	cancelOrder,
	confirmOrderReceipt,
} from "@/lib/orders/repo";
import {
	canWriteOrderReview,
	type Order,
	type OrderReview,
	STATUS_LABEL,
} from "@/lib/orders/types";
import { useStoreCardViews } from "@/lib/seller/useStoreCardViews";

/**
 * 賣家列：訂單為單一賣家模型，顯示在商品清單上方。
 * 店家卡片（BatchGetStores）顯示營業中時整列可點進賣家店面 /sellers/{store_code}；
 * 暫停營業、已關閉或沒有卡片時只留文字（B11），暫停與已關閉另標標籤。
 */
function OrderSellerRow({ order }: { order: Order }) {
	const { views } = useStoreCardViews([order.storeId]);
	const view = views.get(order.storeId);
	const name = view?.name || order.sellerName?.trim() || "";
	const sellerHref = view?.href ?? "";

	if (!sellerHref && !name) return null;

	const displayName = name || "賣家";
	const avatar = (
		<span className="grid place-items-center w-9 h-9 shrink-0 overflow-hidden rounded-full bg-default-100">
			<Avatar
				src={order.sellerLogo}
				name={displayName}
				alt=""
				entity="seller"
				fallback="empty"
				loading="eager"
				className="w-full h-full object-cover"
			/>
		</span>
	);
	const label = (
		<strong className="min-w-0 flex-1 truncate text-base font-semibold group-hover:underline">
			{displayName}
			{view?.labelText ? (
				<small className="ml-2 font-normal text-foreground-muted">
					{view.labelText}
				</small>
			) : null}
		</strong>
	);

	if (!sellerHref) {
		return (
			<div className="flex items-center gap-3 mb-3 pb-3 border-b border-divider">
				{avatar}
				{label}
			</div>
		);
	}

	return (
		<Link
			href={sellerHref}
			className="group flex items-center gap-3 mb-3 pb-3 border-b border-divider no-underline text-inherit"
			aria-label={`查看 ${displayName} 的店家頁`}
		>
			{avatar}
			{label}
			<span className="inline-flex items-center gap-0.5 text-sm text-foreground-muted shrink-0">
				進店
				<svg
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					strokeWidth="2"
					strokeLinecap="round"
					strokeLinejoin="round"
					aria-hidden="true"
				>
					<path d="m9 18 6-6-6-6" />
				</svg>
			</span>
		</Link>
	);
}

type OrderPendingAction = "cancel" | "confirm" | null;

export default function OrderDetailClient({ orderNo }: { orderNo: string }) {
	const router = useObservedRouter("buyer_order_detail");
	const { order, loading, error, refresh } = useOrder(orderNo);
	const afterSales = useMyAfterSales();
	const [pendingAction, setPendingAction] =
		React.useState<OrderPendingAction>(null);
	const busy = pendingAction !== null;
	const notify = useNotificationToast();

	if (loading && !order) {
		return (
			<div>
				<h1 className="sr-only">訂單詳情</h1>
				<p>載入中…</p>
			</div>
		);
	}

	if (error || !order) {
		return (
			<div>
				<SellerPageShell
					title="找不到訂單"
					actions={
						<Button
							href="/account/orders"
							variant="bordered"
							color="default"
							size="sm"
						>
							← 回訂單列表
						</Button>
					}
				>
					<p className="text-danger-700 text-sm font-semibold">
						{error || "找不到訂單"}
					</p>
				</SellerPageShell>
			</div>
		);
	}

	const createdStamp = order.createdAt
		? new Date(
				order.createdAt < 1e12 ? order.createdAt * 1000 : order.createdAt,
			)
		: null;

	const paidStamp = order.paidAt ?? order.payment?.paidAt ?? null;
	const timelineEvents: Partial<Record<string, number>> = {};
	if (order.createdAt) timelineEvents.pending_payment = order.createdAt;
	if (paidStamp) timelineEvents.paid = paidStamp;
	if (order.shipping?.deliveredAt) {
		timelineEvents.completed = order.shipping.deliveredAt;
	}
	const aftersale = afterSales.lookup.get(order.orderNo);
	const canApplyAfterSale =
		order.status === "delivered" &&
		!order.countdownPausedAt &&
		!aftersale &&
		!afterSales.loading;

	const handleCancel = async () => {
		if (!canBuyerCancelOrder(order)) return;
		if (!window.confirm(`確定要取消訂單 #${order.orderNo} 嗎？`)) return;
		setPendingAction("cancel");
		try {
			await cancelOrder(order.orderNo);
			await refresh();
		} catch (e) {
			notify.show({
				variant: "error",
				kind: "order",
				title: "取消訂單失敗",
				body: e instanceof Error ? e.message : "請稍後再試。",
				dedupeKey: `action-cancel-error:${order.orderNo}`,
			});
		} finally {
			setPendingAction(null);
		}
	};

	const handleConfirm = async () => {
		if (!order.canConfirm && order.status !== "delivered") return;
		if (
			!window.confirm(
				"確認收貨後不能再申請退換貨，且系統會通知賣家準備結算。確定嗎？",
			)
		) {
			return;
		}
		setPendingAction("confirm");
		try {
			await confirmOrderReceipt(order.orderNo);
			await refresh();
			router.push(`/account/orders/${order.orderNo}/review`);
		} catch (e) {
			notify.show({
				variant: "error",
				kind: "order",
				title: "確認收貨失敗",
				body: e instanceof Error ? e.message : "請稍後再試。",
				dedupeKey: `action-confirm-error:${order.orderNo}`,
			});
		} finally {
			setPendingAction(null);
		}
	};

	return (
		<div>
			<SellerPageShell
				title="訂單詳情"
				subtitle={
					<>
						#{order.orderNo} ·{" "}
						<span
							className={`text-xs font-semibold px-2.5 py-1 rounded-full ${buyerOrderStatusBadgeClass(order.status)}`}
						>
							{STATUS_LABEL[order.status] || order.status}
						</span>
						{createdStamp ? `　建立於 ${formatStamp(createdStamp)}` : ""}
					</>
				}
				actions={
					<Button
						href="/account/orders"
						variant="bordered"
						color="default"
						size="sm"
					>
						← 回訂單列表
					</Button>
				}
			>
				<div className="flex flex-col gap-[18px] min-w-0">
					<OrderStatusTimeline
						status={order.status}
						events={timelineEvents}
						cancelReason={order.cancelReason}
						cancelByRole={order.cancelByRole}
						cancelRemark={order.cancelRemark}
						aftersale={aftersale}
						shippingMethod={order.shipping?.method}
					/>

					{order.status === "pending_payment" ? (
						<PendingPaymentBanner
							order={order}
							busy={busy}
							pendingAction={pendingAction}
							onRetry={() =>
								router.push(`/orders/${encodeURIComponent(order.orderNo)}`)
							}
							onCancel={handleCancel}
						/>
					) : null}

					{order.status === "delivered" ? (
						<DeliveredAutoCompleteBanner
							autoCompleteAt={order.autoCompleteAt}
							pausedAt={order.countdownPausedAt}
						/>
					) : null}

					{order.status === "delivered" ? (
						<section className="flex flex-col gap-3 rounded-hkui-lg border border-divider bg-content1 p-4 sm:flex-row sm:items-center sm:justify-between">
							<p className="m-0 text-sm text-hk-body">
								收到商品了嗎？確認收貨後賣家才能結算；若商品有問題，可申請退換貨。
							</p>
							<div className="flex flex-wrap gap-2 max-sm:justify-end">
								{canApplyAfterSale ? (
									<Button
										href={`/account/orders/${order.orderNo}/return`}
										variant="bordered"
										color="warning"
										disabled={busy}
									>
										申請退換貨
									</Button>
								) : null}
								<Button
									disabled={busy}
									isLoading={pendingAction === "confirm"}
									loadingLabel="確認收貨中"
									onClick={handleConfirm}
								>
									確認收貨
								</Button>
							</div>
						</section>
					) : null}

					<section className="bg-content1 border border-divider rounded-hkui-lg p-5 mb-0">
						<OrderSellerRow order={order} />
						<h2 className="text-base font-semibold mb-3">
							商品（{order.items.length}）
						</h2>
						<ul className="list-none m-0 p-0 flex flex-col gap-3">
							{order.items.map((it) => {
								const unitPrice = Number(it.price) || 0;
								const qty = Number(it.quantity) || 0;
								const subtotal = it.subtotal
									? Number(it.subtotal)
									: unitPrice * qty;
								return (
									<li
										key={String(it.orderItemId)}
										className="grid grid-cols-[64px_1fr_auto] gap-3 items-center"
									>
										<AppImage
											src={resolveCdnUrl(it.image)}
											alt={it.name}
											width={64}
											height={64}
											sizes="64px"
											loading="eager"
											className="w-16 h-16 rounded-hkui-md object-cover bg-default-100"
										/>
										<div className="flex flex-col gap-[2px]">
											<strong className="text-base">{it.name}</strong>
											{it.specName ? (
												<small className="text-sm text-foreground-muted">
													{it.specName}
												</small>
											) : null}
											<span className="text-sm text-foreground-muted">
												NT$ {unitPrice.toLocaleString()} × {qty}
											</span>
										</div>
										<b className="text-base text-hk-primary">
											NT$ {subtotal.toLocaleString()}
										</b>
									</li>
								);
							})}
						</ul>
					</section>

					{order.shipping ? (
						<section className="bg-content1 border border-divider rounded-hkui-lg p-5 mb-0">
							<h2 className="text-base font-semibold mb-3">配送資訊</h2>
							<dl className="m-0 grid grid-cols-[88px_minmax(0,1fr)] gap-x-4 gap-y-2 max-[540px]:grid-cols-1 max-[540px]:gap-x-0 max-[540px]:gap-y-1">
								<dt className="m-0 text-[0.8125rem] text-hk-muted max-[540px]:mt-2 max-[540px]:first-of-type:mt-0">
									收件人
								</dt>
								<dd className="m-0 break-words text-sm text-hk-body">
									{order.shipping.receiverName ?? "—"}
									{order.shipping.receiverPhone
										? ` · ${order.shipping.receiverPhone}`
										: ""}
								</dd>

								{formatShippingAddress(order.shipping) !== "—" ? (
									<>
										<dt className="m-0 text-[0.8125rem] text-hk-muted max-[540px]:mt-2 max-[540px]:first-of-type:mt-0">
											地址
										</dt>
										<dd className="m-0 break-words text-sm text-hk-body">
											{formatShippingAddress(order.shipping)}
										</dd>
									</>
								) : null}
								{order.shipping.method ? (
									<>
										<dt className="m-0 text-[0.8125rem] text-hk-muted max-[540px]:mt-2 max-[540px]:first-of-type:mt-0">
											配送方式
										</dt>
										<dd className="m-0 break-words text-sm text-hk-body">
											{formatShippingMethodLabel(order.shipping.method)}
										</dd>
									</>
								) : null}
								{order.shipping.storeName ? (
									<>
										<dt className="m-0 text-[0.8125rem] text-hk-muted max-[540px]:mt-2 max-[540px]:first-of-type:mt-0">
											取貨門市
										</dt>
										<dd className="m-0 break-words text-sm text-hk-body">
											{order.shipping.storeName}
										</dd>
									</>
								) : null}
								{order.shipping.trackingNo ? (
									<>
										<dt className="m-0 text-[0.8125rem] text-hk-muted max-[540px]:mt-2 max-[540px]:first-of-type:mt-0">
											物流追蹤號
										</dt>
										<dd className="m-0 break-words text-sm text-hk-body">
											<strong>{order.shipping.trackingNo}</strong>
										</dd>
									</>
								) : null}
							</dl>
						</section>
					) : null}

					<section className="bg-content1 border border-divider rounded-hkui-lg p-5 mb-0">
						<h2 className="text-base font-semibold mb-3">付款與金額</h2>
						<div className="flex flex-wrap gap-[14px] pb-3 mb-3 border-b border-divider dark:border-hk-border-soft text-[0.8125rem] text-hk-muted">
							<span className="inline-flex items-center">
								方式：{formatPaymentMethod(order.payment?.method)}
							</span>
							<span className="inline-flex items-center">
								狀態：{formatPaymentStatus(order)}
							</span>
							{paidStamp ? (
								<span className="inline-flex items-center">
									付款時間：{formatStampShort(paidStamp)}
								</span>
							) : null}
						</div>
						<dl className="flex flex-col gap-1 m-0">
							{order.totalOriginalAmount ? (
								<div className="flex justify-between text-sm">
									<dt>商品小計</dt>
									<dd>
										NT$ {Number(order.totalOriginalAmount).toLocaleString()}
									</dd>
								</div>
							) : null}
							{order.shippingFee ? (
								<div className="flex justify-between text-sm">
									<dt>運費</dt>
									<dd>NT$ {Number(order.shippingFee).toLocaleString()}</dd>
								</div>
							) : null}
							{order.discount ? (
								<div className="flex justify-between text-sm">
									<dt>折抵</dt>
									<dd>− NT$ {Number(order.discount).toLocaleString()}</dd>
								</div>
							) : null}
							<div className="border-t border-divider pt-2 mt-2 text-base font-bold text-hk-primary flex justify-between">
								<dt>實付</dt>
								<dd>NT$ {Number(order.totalActualAmount).toLocaleString()}</dd>
							</div>
						</dl>
					</section>

					<OrderInvoiceSection
						orderNo={order.orderNo}
						sellerName={order.sellerName?.trim() || ""}
					/>

					{order.review ? <OrderReviewCard review={order.review} /> : null}

					<OrderAfterSaleSummary
						orderNo={order.orderNo}
						canRefund={Boolean(order.canRefund)}
						hideApplyCta={order.status === "delivered"}
					/>

					<section className="flex gap-2 flex-wrap justify-end mt-4">
						{order.status === "shipping" ? (
							<Button
								href={`/account/orders/${order.orderNo}/tracking`}
								variant="bordered"
								color="default"
							>
								查看物流
							</Button>
						) : null}
						{order.canConfirm && order.status !== "delivered" ? (
							<Button
								disabled={busy}
								isLoading={pendingAction === "confirm"}
								loadingLabel="確認收貨中"
								onClick={handleConfirm}
							>
								確認收貨
							</Button>
						) : null}
						{canWriteOrderReview(order) ? (
							<Button href={`/account/orders/${order.orderNo}/review`}>
								評價商品
							</Button>
						) : null}
					</section>
				</div>
			</SellerPageShell>
		</div>
	);
}

const formatStamp = formatTaipeiDateTime;
const formatStampShort = formatTaipeiDateTime;

function formatShippingAddress(shipping: Order["shipping"]): string {
	if (!shipping) return "—";
	const city = shipping.city?.trim() ?? "";
	const district = shipping.district?.trim() ?? "";
	const address = shipping.address?.trim() ?? "";
	const prefix = `${city}${district}`;
	if (!address) return prefix || "—";
	if (prefix && address.startsWith(prefix)) return address;
	return `${prefix}${address}` || "—";
}

function normalizePaymentMethod(raw?: string): PaymentMethodValue | undefined {
	if (!raw) return undefined;
	const allowed = new Set<string>(Object.values(PaymentMethod));
	if (allowed.has(raw)) return raw as PaymentMethodValue;
	const upper = raw.toUpperCase().replace(/[\s-]/g, "_");
	if (upper === "CREDIT" || upper === "CREDIT_CARD" || upper === "CARD") {
		return PaymentMethod.CREDIT_CARD;
	}
	if (upper === "APPLE_PAY" || upper === "APPLEPAY") {
		return PaymentMethod.APPLE_PAY;
	}
	if (upper === "GOOGLE_PAY" || upper === "GOOGLEPAY") {
		return PaymentMethod.GOOGLE_PAY;
	}
	if (upper === "ZERO" || upper === "ZERO_AMOUNT") {
		return PaymentMethod.ZERO_AMOUNT;
	}
	return undefined;
}

function formatPaymentMethod(raw?: string): string {
	const code = normalizePaymentMethod(raw);
	if (code) return PAYMENT_METHOD_LABEL[code] ?? raw ?? "—";
	return raw || "—";
}

function normalizePaymentStatus(raw?: string): PaymentStatusValue | undefined {
	if (!raw) return undefined;
	const allowed = new Set<string>(Object.values(PaymentStatus));
	if (allowed.has(raw)) return raw as PaymentStatusValue;
	const upper = raw.toUpperCase().replace(/[\s-]/g, "_");
	if (upper === "0" || upper === "UNPAID" || upper === "PENDING_PAYMENT") {
		return PaymentStatus.UNPAID;
	}
	if (
		upper === "1" ||
		upper === "10" ||
		upper === "PAID" ||
		upper === "SUCCESS" ||
		upper === "COMPLETED"
	) {
		return PaymentStatus.SUCCESS;
	}
	if (upper === "FAILED" || upper === "FAILURE") {
		return PaymentStatus.FAILED;
	}
	if (upper === "60" || upper === "PARTIAL_REFUNDED") {
		return PaymentStatus.PARTIAL_REFUNDED;
	}
	if (upper === "70" || upper === "80" || upper === "REFUNDED") {
		return PaymentStatus.REFUNDED;
	}
	return undefined;
}

function formatPaymentStatus(order: Order): string {
	const raw = order.payment?.status?.trim();
	const explicit = normalizePaymentStatus(raw);
	if (explicit === PaymentStatus.UNPAID) return "尚未付款";
	if (explicit === PaymentStatus.SUCCESS) return "已付款";
	if (explicit === PaymentStatus.REFUNDED) return "已退款";
	if (explicit) return PAYMENT_STATUS_LABEL[explicit] ?? raw ?? "—";
	if (raw) return raw;
	switch (order.status) {
		case "pending_payment":
			return "尚未付款";
		case "paid":
		case "preparing":
		case "shipping":
		case "waiting_pickup":
		case "shipping_abnormal":
		case "delivered":
		case "completed":
			return "已付款";
		case "partially_refunded":
			return "部分退款";
		case "refunded":
			return "已退款";
		case "cancelled":
			return "已取消";
		default:
			return "—";
	}
}

interface DeliveredAutoCompleteBannerProps {
	autoCompleteAt?: number | null;
	pausedAt?: number | null;
}

function DeliveredAutoCompleteBanner({
	autoCompleteAt,
	pausedAt,
}: DeliveredAutoCompleteBannerProps) {
	const [now, setNow] = React.useState(() => Date.now());

	React.useEffect(() => {
		if (!autoCompleteAt || pausedAt) return;
		setNow(Date.now());
		const ms = toUnixMs(autoCompleteAt) - Date.now();
		const id = window.setInterval(
			() => setNow(Date.now()),
			ms > 3_600_000 ? 30_000 : 1_000,
		);
		return () => window.clearInterval(id);
	}, [autoCompleteAt, pausedAt]);

	if (pausedAt) {
		return (
			<section
				className="rounded-hkui-lg border p-4 text-sm text-warning-700"
				role="status"
			>
				<strong className="block text-base text-warning-700">
					自動完成倒數已暫停
				</strong>
				<p className="m-0 mt-1 leading-relaxed">
					暫停於 {formatStampShort(pausedAt)}
					。售後或爭議處理完成後，系統會恢復自動完成流程。
				</p>
			</section>
		);
	}

	if (!autoCompleteAt) return null;

	const remaining = formatAutoCompleteRemaining(autoCompleteAt, now);
	return (
		<section
			className="rounded-hkui-lg border border-success-200 bg-success-50 p-4 text-sm text-success-700"
			role="status"
		>
			<strong className="block text-base text-success-700">訂單已送達</strong>
			<p className="m-0 mt-1 leading-relaxed">{remaining}</p>
		</section>
	);
}

function toUnixMs(ts: number): number {
	return ts < 1e12 ? ts * 1000 : ts;
}

function formatAutoCompleteRemaining(deadline: number, now: number): string {
	const ms = toUnixMs(deadline) - now;
	if (ms <= 0) return "已達自動完成時間，等待系統完成訂單。";
	const totalSec = Math.floor(ms / 1000);
	const hours = Math.floor(totalSec / 3600);
	const minutes = Math.floor((totalSec % 3600) / 60);
	const seconds = totalSec % 60;
	if (hours >= 24) {
		const days = Math.floor(hours / 24);
		const restHours = hours % 24;
		return `${days} 天 ${restHours} 小時後自動完成。`;
	}
	if (hours >= 1) {
		return `${hours} 小時 ${minutes.toString().padStart(2, "0")} 分後自動完成。`;
	}
	if (minutes >= 1) {
		return `${minutes} 分 ${seconds.toString().padStart(2, "0")} 秒後自動完成。`;
	}
	return `${seconds} 秒後自動完成。`;
}

interface PendingPaymentBannerProps {
	order: Order;
	busy: boolean;
	pendingAction: OrderPendingAction;
	onRetry: () => void;
	onCancel: () => void;
}

/**
 * Buyer-facing "your order is created but payment hasn't completed"
 * surface. Mirrors the APP's 待付款 card actions:
 *   - 尚未付款 red badge
 *   - 再次付款 (primary CTA — goes to the payment step, which creates or
 *     resumes the payment for this order)
 *   - 取消訂單 (gated by `canBuyerCancelOrder`)
 */
function PendingPaymentBanner({
	order,
	busy,
	pendingAction,
	onRetry,
	onCancel,
}: PendingPaymentBannerProps) {
	const amount = Number(order.totalActualAmount).toLocaleString();
	const canCancel = canBuyerCancelOrder(order);
	return (
		<section
			className="bg-[rgb(var(--color-status-danger-rgb)/0.06)] dark:bg-[rgb(var(--color-status-danger-rgb)/0.1)] border border-[rgb(var(--color-status-danger-rgb)/0.22)] dark:border-[rgb(var(--color-status-danger-rgb)/0.32)] rounded-hkui-lg p-5 mb-4 max-[600px]:p-4"
			aria-label="尚未完成付款"
		>
			<header className="flex items-start gap-4 max-[600px]:flex-wrap">
				<div
					className="shrink-0 w-8 h-8 rounded-full bg-hk-danger text-hk-cream grid place-items-center font-bold text-base leading-none font-display mt-0.5"
					aria-hidden="true"
				>
					!
				</div>
				<div className="flex-1 min-w-0">
					<strong className="block text-lg font-bold text-hk-heading dark:text-[#f5e9c3] tracking-tight mb-1">
						尚未完成付款
					</strong>
					<p className="m-0 text-sm text-[#565656] dark:text-[rgb(var(--color-ink-100-rgb)/0.7)] leading-relaxed max-w-[56ch]">
						訂單 #{order.orderNo}{" "}
						已建立但尚未付款。可使用「再次付款」完成這筆訂單，或選擇取消。
					</p>
					{order.paymentDeadline ? (
						<OrderPaymentCountdown
							deadline={order.paymentDeadline}
							className="inline-flex items-center gap-1.5 mt-2.5 px-3 py-1 rounded-full bg-[rgba(237,149,75,0.14)] text-sm font-bold tabular-nums tracking-wide data-[expired=true]:bg-black/8 data-[expired=true]:text-hk-body data-[expired=true]:font-semibold"
							as="strong"
						/>
					) : null}
				</div>
				<div className="shrink-0 text-right flex flex-col gap-0.5 max-[600px]:flex-basis-100 max-[600px]:text-left">
					<small className="text-xs text-[#565656] dark:text-[rgb(var(--color-ink-100-rgb)/0.7)] tracking-wide">
						應付金額
					</small>
					<b className="font-display text-3xl font-bold text-hk-danger dark:text-[color:var(--color-brand-secondary)] tracking-tight leading-none tabular-nums">
						NT$ {amount}
					</b>
				</div>
			</header>
			<dl className="flex gap-8 mt-5 pt-4 border-t border-[rgb(var(--color-status-danger-rgb)/0.18)] dark:border-[rgb(var(--color-status-danger-rgb)/0.28)] max-[600px]:flex-col max-[600px]:gap-3">
				<div className="flex flex-col gap-1 min-w-0">
					<dt className="text-xs text-[#565656] dark:text-[rgb(var(--color-ink-100-rgb)/0.7)] font-medium tracking-wide">
						付款狀態
					</dt>
					<dd className="m-0 text-[0.9375rem] font-semibold text-hk-danger dark:text-[color:var(--color-brand-secondary)]">
						尚未付款
					</dd>
				</div>
			</dl>
			<div className="flex gap-3 justify-end mt-5 flex-wrap">
				{canCancel ? (
					<Button
						variant="bordered"
						color="default"
						onClick={onCancel}
						disabled={busy}
						isLoading={pendingAction === "cancel"}
						loadingLabel="取消訂單中"
					>
						取消訂單
					</Button>
				) : null}
				<Button
					className="bg-hk-danger border-hk-danger text-hk-cream hover:bg-[color-mix(in_oklch,var(--color-status-danger)_88%,black_12%)] hover:border-[color-mix(in_oklch,var(--color-status-danger)_88%,black_12%)]"
					onClick={onRetry}
					disabled={busy}
				>
					再次付款
				</Button>
			</div>
		</section>
	);
}

/** 我的評價：訂單已評論時顯示買家對此訂單的評分與評論（後端 orders.review 快照，取最近一筆商品評論）。 */
function OrderReviewCard({ review }: { review: OrderReview }) {
	const rating = Math.max(0, Math.min(5, Math.round(review.rating)));
	return (
		<section className="mt-4 bg-content1 border border-divider rounded-hkui-lg p-4">
			<h3 className="text-base font-bold text-hk-primary mb-2">我的評價</h3>
			<div className="flex items-center gap-1 text-[1.25rem] leading-none">
				{[1, 2, 3, 4, 5].map((n) => (
					<span
						key={n}
						className={
							rating >= n
								? "text-[color:var(--hkui-warning)]"
								: "text-[color:var(--hkui-default-200)]"
						}
					>
						★
					</span>
				))}
				<span className="ml-1 text-sm text-foreground-500">{rating} / 5</span>
			</div>
			{review.content ? (
				<p className="mt-2 text-sm text-foreground-700 whitespace-pre-wrap">
					{review.content}
				</p>
			) : null}
			{review.imageUrls && review.imageUrls.length > 0 ? (
				<div className="mt-2 flex gap-2 flex-wrap">
					{review.imageUrls.map((url) => (
						// biome-ignore lint/performance/noImgElement: backend CDN URL
						<img
							key={url}
							src={resolveCdnUrl(url)}
							alt="評論附圖"
							className="w-16 h-16 rounded-hkui-md object-cover bg-default-100"
						/>
					))}
				</div>
			) : null}
			{review.createdAt ? (
				<p className="mt-2 text-xs text-foreground-500">
					{formatTaipeiDateTime(review.createdAt)}
				</p>
			) : null}
		</section>
	);
}
