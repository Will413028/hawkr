"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import PaymentGlyph from "@/components/shared/PaymentGlyph";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import OrderAfterSaleSummary from "@/features/account/screens/orders/OrderAfterSaleSummary";
import OrderPaymentCountdown from "@/features/account/screens/orders/OrderPaymentCountdown";
import { buyerOrderStatusBadgeClass } from "@/features/account/screens/orders/statusBadgeStyles";
import { OrderInvoiceSection } from "@/features/invoices/order/OrderInvoiceSection";
import OrderStatusTimeline from "@/features/orders/components/OrderStatusTimeline";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import {
	formatShippingMethodLabel,
	PAYMENT_METHOD_INELIGIBLE_REASON_LABEL,
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
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { safeRedirectUrl } from "@/lib/navigation/safe-url";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import {
	canBuyerCancelOrder,
	cancelOrder,
	confirmOrderReceipt,
	createPayment,
} from "@/lib/orders/repo";
import {
	canWriteOrderReview,
	type Order,
	type OrderReview,
	STATUS_LABEL,
} from "@/lib/orders/types";
import { mapApplePayError, openApplePaySheet } from "@/lib/payment/apple-pay";
import { openGooglePaySheet } from "@/lib/payment/google-pay";
import {
	type NativePayAvailability,
	useNativePayAvailability,
} from "@/lib/payment/useNativePayAvailability";
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

	// 完全沒有賣家資訊就不顯示，避免空白列
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

type OrderPendingAction = "cancel" | "retry" | "confirm" | null;

export default function OrderDetailClient({ orderNo }: { orderNo: string }) {
	const router = useObservedRouter("buyer_order_detail");
	const { order, loading, error, refresh } = useOrder(orderNo);
	const afterSales = useMyAfterSales();
	const nativePay = useNativePayAvailability({
		applePay: true,
		googlePay: true,
	});
	const [pendingAction, setPendingAction] =
		React.useState<OrderPendingAction>(null);
	const busy = pendingAction !== null;
	const [retrySheetOpen, setRetrySheetOpen] = React.useState(false);
	// 所有錯誤訊息走頂部下拉 toast — 既有 inline actionError 位置在訂單詳情
	// 主體下方容易被忽略 (sheet 關閉後 user 焦點不在頁面),已移除。Apple/
	// Google Pay retry 五個 outcome (createPayment fail / 成功 / 取消 / sheet
	// onError / 此瀏覽器無法使用) + cancelOrder / confirmReceipt / runAsync
	// Retry 兩條 fallback 各自 push 對應 toast。信用卡 retry 走 PayUni 跳轉,
	// 有自己的 PaymentReturnClient landing,不在這裡 toast。
	const notify = useNotificationToast();

	if (loading && !order) {
		return (
			<main>
				<p>載入中…</p>
			</main>
		);
	}

	if (error || !order) {
		return (
			<main>
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
			</main>
		);
	}

	const createdStamp = order.createdAt
		? new Date(
				order.createdAt < 1e12 ? order.createdAt * 1000 : order.createdAt,
			)
		: null;

	// Map known timestamps onto the timeline's step keys. Missing stamps
	// simply don't render under their step — partial data is fine.
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

	// 「再次付款」開啟付款方式選擇 sheet(對齊 App 的 showRetryPaymentSheet):
	// 讓卡在 Apple Pay 的買家能改選信用卡/ATM,而不是被原付款方式綁死。
	const openRetrySheet = () => {
		if (!order || order.status !== "pending_payment") return;
		setRetrySheetOpen(true);
	};

	// Apple Pay retry — opens sheet SYNCHRONOUSLY (gesture-bound). Order
	// already exists, so the driver only needs to call createPayment with the
	// returned token.
	const startApplePayRetry = (
		orderNo: string,
		amount: number,
		productName: string,
	) => {
		const sheet = openApplePaySheet(
			{ amount, label: productName },
			{
				onAuthorize: async (token) => {
					try {
						await createPayment({
							orderNo,
							amount,
							productName,
							paymentType: PaymentMethod.APPLE_PAY,
							clientType: "WEB_BROWSER",
							applePayToken: token,
						});
					} catch (e) {
						notify.show({
							variant: "error",
							kind: "order",
							title: "付款失敗",
							body:
								mapApplePayError(e) ??
								(e instanceof Error
									? `付款失敗：${e.message}`
									: "付款失敗，請稍後再試。"),
							dedupeKey: `retry-pay-error:${orderNo}`,
						});
						setPendingAction(null);
						return "failure";
					}
					await refresh();
					notify.show({
						variant: "success",
						kind: "order",
						title: "付款成功",
						body: `訂單 ${orderNo} 已完成付款`,
						dedupeKey: `retry-pay-success:${orderNo}`,
					});
					setPendingAction(null);
					return "success";
				},
				onCancel: () => {
					notify.show({
						variant: "warning",
						kind: "order",
						title: "已取消付款",
						body: "可再試一次或選擇其他付款方式。",
						dedupeKey: `retry-pay-cancel:${orderNo}`,
					});
					setPendingAction(null);
				},
				onError: (e) => {
					notify.show({
						variant: "error",
						kind: "order",
						title: "付款失敗",
						body: mapApplePayError(e) ?? `付款失敗：${e.message}`,
						dedupeKey: `retry-pay-error:${orderNo}`,
					});
					setPendingAction(null);
				},
			},
		);

		if (!sheet) {
			notify.show({
				variant: "error",
				kind: "order",
				title: "無法使用 Apple Pay",
				body: "此裝置或瀏覽器無法使用 Apple Pay。請改用 Safari(或已設定 Apple Pay 的 iPhone)再付款,或聯絡客服協助改用其他付款方式。",
				dedupeKey: `retry-pay-unavailable:${orderNo}`,
			});
			setPendingAction(null);
		}
	};

	const startGooglePayRetry = (
		orderNo: string,
		amount: number,
		productName: string,
	) => {
		const sheet = openGooglePaySheet(
			{
				amount,
				label: productName,
				// paymentMerId 從 order 物件取（後端 orderDetail response 帶回，20 §4.13）
				gatewayMerchantId: order?.paymentMerId ?? "",
			},
			{
				onAuthorize: async (token) => {
					try {
						await createPayment({
							orderNo,
							amount,
							productName,
							paymentType: PaymentMethod.GOOGLE_PAY,
							clientType: "WEB_BROWSER",
							googlePayToken: token,
						});
					} catch (e) {
						notify.show({
							variant: "error",
							kind: "order",
							title: "付款失敗",
							body:
								e instanceof Error
									? `付款失敗：${e.message}`
									: "付款失敗，請稍後再試。",
							dedupeKey: `retry-pay-error:${orderNo}`,
						});
						setPendingAction(null);
						return;
					}
					await refresh();
					notify.show({
						variant: "success",
						kind: "order",
						title: "付款成功",
						body: `訂單 ${orderNo} 已完成付款`,
						dedupeKey: `retry-pay-success:${orderNo}`,
					});
					setPendingAction(null);
				},
				onCancel: () => {
					notify.show({
						variant: "warning",
						kind: "order",
						title: "已取消付款",
						body: "可再試一次或選擇其他付款方式。",
						dedupeKey: `retry-pay-cancel:${orderNo}`,
					});
					setPendingAction(null);
				},
				onError: (e) => {
					notify.show({
						variant: "error",
						kind: "order",
						title: "付款失敗",
						body: `付款失敗：${e.message}`,
						dedupeKey: `retry-pay-error:${orderNo}`,
					});
					setPendingAction(null);
				},
			},
		);

		if (!sheet) {
			notify.show({
				variant: "error",
				kind: "order",
				title: "無法使用 Google Pay",
				body: "此裝置或瀏覽器無法使用 Google Pay。請改用支援的瀏覽器再付款,或聯絡客服協助改用其他付款方式。",
				dedupeKey: `retry-pay-unavailable:${orderNo}`,
			});
			setPendingAction(null);
		}
	};

	// Credit card / ATM — no gesture constraint, existing async flow.
	const runAsyncRetry = async (
		orderNo: string,
		amount: number,
		productName: string,
		paymentType: string,
	) => {
		try {
			const result = await createPayment({
				orderNo,
				amount,
				productName,
				paymentType,
				clientType: "WEB_BROWSER",
			});
			// ATM: backend swaps the virtual account in-place — just refresh
			// the order and surface a hint. Card: bounce to PayUni's hosted
			// page via the returned `redirectUrl`. Mirrors APP's
			// `order_card._executePayment`.
			if (paymentType === PaymentMethod.ATM) {
				await refresh();
				return;
			}
			const redirectTarget = safeRedirectUrl(result.redirectUrl);
			if (redirectTarget) {
				commitDocumentNavigation(redirectTarget);
				return;
			}
			notify.show({
				variant: "error",
				kind: "order",
				title: "付款失敗",
				body: "無法取得付款頁面，請稍後再試。",
				dedupeKey: `retry-pay-no-redirect:${orderNo}`,
			});
		} catch (e) {
			notify.show({
				variant: "error",
				kind: "order",
				title: "付款失敗",
				body:
					e instanceof Error
						? `付款失敗：${e.message}`
						: "付款失敗，請稍後再試。",
				dedupeKey: `retry-pay-error:${orderNo}`,
			});
		} finally {
			setPendingAction(null);
		}
	};

	// SYNC entry — must remain non-async so the native-pay branches stay
	// inside the user gesture's call stack on mobile WebKit.
	const executeRetryPayment = (paymentType: string) => {
		if (!order || order.status !== "pending_payment") return;
		setRetrySheetOpen(false);
		setPendingAction("retry");

		const amount = Number(order.totalActualAmount) || 0;
		const productName = order.items[0]?.name
			? order.items.length > 1
				? `${order.items[0].name} 等 ${order.items.length} 項`
				: order.items[0].name
			: `訂單 ${order.orderNo}`;
		const orderNo = order.orderNo;

		if (paymentType === PaymentMethod.APPLE_PAY) {
			if (!nativePay.applePay) {
				notify.show({
					variant: "error",
					kind: "order",
					title: "無法使用 Apple Pay",
					body: "此裝置或瀏覽器無法使用 Apple Pay。請改用 Safari(或已設定 Apple Pay 的 iPhone)再付款,或聯絡客服協助改用其他付款方式。",
					dedupeKey: `retry-pay-unavailable:${orderNo}`,
				});
				setPendingAction(null);
				return;
			}
			startApplePayRetry(orderNo, amount, productName);
			return;
		}
		if (paymentType === PaymentMethod.GOOGLE_PAY) {
			if (!nativePay.googlePay) {
				notify.show({
					variant: "error",
					kind: "order",
					title: "無法使用 Google Pay",
					body: "此裝置或瀏覽器無法使用 Google Pay。請改用支援的瀏覽器再付款,或聯絡客服協助改用其他付款方式。",
					dedupeKey: `retry-pay-unavailable:${orderNo}`,
				});
				setPendingAction(null);
				return;
			}
			startGooglePayRetry(orderNo, amount, productName);
			return;
		}
		void runAsyncRetry(orderNo, amount, productName, paymentType);
	};

	const handleConfirm = async () => {
		// 已送達訂單一律可確認收貨（後端 confirm_receipt 的語意），其餘狀態才看
		// canConfirm 旗標。
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
		<main>
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

					{/* Pending-payment surface — mirrors APP order_card.dart's
					    待付款 layout: a prominent "尚未付款" banner with 再次付款
					    + 取消訂單 CTAs, plus an ATM remittance hint when the
					    chosen method is ATM. Card / wallet retries open
					    PayUni's hosted page via `redirectUrl`. */}
					{order.status === "pending_payment" ? (
						<PendingPaymentBanner
							order={order}
							busy={busy}
							pendingAction={pendingAction}
							onRetry={openRetrySheet}
							onCancel={handleCancel}
						/>
					) : null}

					{order.status === "delivered" ? (
						<DeliveredAutoCompleteBanner
							autoCompleteAt={order.autoCompleteAt}
							pausedAt={order.countdownPausedAt}
						/>
					) : null}

					{/* 已送達後的主要動作，對齊 APP：確認收貨（提前完成、觸發撥款）
					    或申請退換貨。已送達狀態一律提供這兩個動作（後端 confirm_
					    receipt 即「對已送達訂單確認收貨」），不再依賴可能未回傳的
					    canConfirm/canRefund 旗標。倒數暫停（countdownPausedAt）代表
					    售後／爭議處理中，此時不再提供「申請退換貨」入口。售後 CTA 在此
					    狀態由本卡片擁有，下方 OrderAfterSaleSummary 以 hideApplyCta 隱藏
					    重複按鈕。 */}
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

					{retrySheetOpen ? (
						<RetryPaymentSheet
							order={order}
							currentPayType={resolvePaymentType(order)}
							nativePay={nativePay}
							onClose={() => setRetrySheetOpen(false)}
							onConfirm={executeRetryPayment}
						/>
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
								{/* 店到店（7-11）沒有收件地址，formatShippingAddress 回
								    "—"，此時整列隱藏，只保留取貨門市；宅配才顯示地址。 */}
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

					{/* 發票是付款的後果，順序即「交易 → 憑證 → 評價」。零元訂單與尚未
					    成立開立義務的訂單由這個元件自己 return null（端點 8 回 9301）。 */}
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
						{/*
						 * 這裡曾有第二顆「取消訂單」，閘門是
						 * `order.canCancel && order.status !== "pending_payment"`。
						 * 兩半都不成立：`canCancel` 後端不送（恆 undefined），而
						 * order-service 的 `cancelOrder` **只接受 PENDING_PAYMENT**
						 * ——非待付款訂單按下去必得「僅待付款訂單可取消」。
						 * 待付款的取消鍵在 `PendingPaymentBanner` 內，那是唯一一顆。
						 * 非待付款要退款走售後（`OrderAfterSaleSummary`）。
						 */}
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
		</main>
	);
}

// 兩者現在是同一件事：`formatTaipeiDateTime` 收 Date 也收 unix 秒／毫秒。
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
	if (upper === "ATM") return PaymentMethod.ATM;
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

/* ============================================================
 *   再次付款 — 付款方式選擇 sheet(對齊 App retry_payment_sheet）
 * ============================================================ */

/**
 * 再次付款支援的方式。web 跨瀏覽器環境比 App 多 Google Pay(App 是純 iOS
 * 不需要,故 `_kRetryPayMethods` 沒列;web 沿用時若遺漏則 Android/Chrome
 * 使用者無法再次付款只能客服協助,UX 差)。Apple Pay / Google Pay 在不支
 * 援的瀏覽器由下方 `nativeUsable` 判斷置灰,跟主結帳 PAYMENT_METHODS 邏輯
 * 對齊。順序:wallet 類 (Apple/Google Pay) 在前,銀行/卡別在後。
 */
const RETRY_PAY_METHODS: Array<{ label: string; payType: string }> = [
	{ label: "Apple Pay", payType: PaymentMethod.APPLE_PAY },
	{ label: "Google Pay", payType: PaymentMethod.GOOGLE_PAY },
	{ label: "ATM 虛擬轉帳", payType: PaymentMethod.ATM },
	{ label: "信用卡", payType: PaymentMethod.CREDIT_CARD },
];

function retryAmountText(value?: string) {
	if (!value) return "—";
	const n = Number(value);
	return Number.isFinite(n) ? n.toLocaleString() : value;
}

interface RetryPaymentSheetProps {
	order: Order;
	currentPayType: string;
	nativePay: NativePayAvailability;
	onClose: () => void;
	onConfirm: (payType: string) => void;
}

/**
 * 對齊 App 的 `showRetryPaymentSheet`:價格區(商品/運費/總額,不含折扣券)
 * + 付款方式選擇 + 確認付款。差異是 web 為多瀏覽器環境,所以:
 *   - 原方式為 ATM → ATM 置灰「不可選」(同 App)。
 *   - Apple/Google Pay 在不支援的瀏覽器 → 置灰並註明,且預設不會選到它
 *     (App 是純 iOS 不需要;web 必須避免把卡住的買家又綁回原生付款)。
 */
function RetryPaymentSheet({
	order,
	currentPayType,
	nativePay,
	onClose,
	onConfirm,
}: RetryPaymentSheetProps) {
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => setMounted(true), []);
	React.useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") onClose();
		};
		const releaseScrollLock = acquireBodyScrollLock();
		document.addEventListener("keydown", onKey);
		return () => {
			document.removeEventListener("keydown", onKey);
			releaseScrollLock();
		};
	}, [onClose]);

	const isAtmOriginal = currentPayType === PaymentMethod.ATM;
	const nativeUsable = React.useCallback(
		(payType: string) => {
			if (payType === PaymentMethod.APPLE_PAY) return nativePay.applePay;
			if (payType === PaymentMethod.GOOGLE_PAY) return nativePay.googlePay;
			return true;
		},
		[nativePay],
	);
	// 金額 gating（後端 §3.4.1）：依訂單金額算出的付款方式可用性。
	// 空清單（未載入 / 舊後端）graceful 視為不限制。
	const apmByType = React.useMemo(() => {
		const m = new Map<string, { eligible: boolean; reason: string }>();
		for (const a of order.availablePaymentMethods ?? [])
			m.set(a.paymentType, {
				eligible: a.eligible,
				reason: a.ineligibleReason,
			});
		return m;
	}, [order.availablePaymentMethods]);
	const apmLoaded = (order.availablePaymentMethods ?? []).length > 0;
	const amountEligible = React.useCallback(
		(payType: string) =>
			!apmLoaded || (apmByType.get(payType)?.eligible ?? false),
		[apmLoaded, apmByType],
	);
	const amountReason = React.useCallback(
		(payType: string) => apmByType.get(payType)?.reason ?? "",
		[apmByType],
	);
	const isDisabled = React.useCallback(
		(payType: string) =>
			(isAtmOriginal && payType === PaymentMethod.ATM) ||
			!nativeUsable(payType) ||
			!amountEligible(payType),
		[isAtmOriginal, nativeUsable, amountEligible],
	);

	// 預設選原付款方式;若它被 disable(原 ATM、或原生付款在此瀏覽器不可用),
	// 退而選第一個可用方式(通常是信用卡 → 任何瀏覽器都能跳轉付款)。
	const initialIdx = React.useMemo(() => {
		const preferred = RETRY_PAY_METHODS.findIndex(
			(m) => m.payType === currentPayType && !isDisabled(m.payType),
		);
		if (preferred >= 0) return preferred;
		const firstUsable = RETRY_PAY_METHODS.findIndex(
			(m) => !isDisabled(m.payType),
		);
		return firstUsable >= 0 ? firstUsable : 0;
	}, [currentPayType, isDisabled]);
	const [selectedIdx, setSelectedIdx] = React.useState(initialIdx);

	if (!mounted) return null;

	const selectedDisabled = isDisabled(RETRY_PAY_METHODS[selectedIdx]?.payType);

	return createPortal(
		<div
			className="fixed inset-0 z-[1100] flex items-end justify-center sm:items-center"
			role="dialog"
			aria-modal="true"
			aria-label="再次付款"
		>
			<button
				type="button"
				className="absolute inset-0 cursor-pointer border-0 bg-[rgb(var(--color-ink-950-rgb)/0.42)] backdrop-blur-[6px]"
				onClick={onClose}
				aria-label="關閉"
			/>
			<section className="relative w-full sm:w-[min(440px,100%)] max-h-[88vh] overflow-y-auto rounded-t-[20px] sm:rounded-[20px] border border-divider bg-content1 shadow-hk-card">
				<div className="flex justify-center pt-3 pb-1 sm:hidden">
					<span className="h-1 w-10 rounded-full bg-default-300" />
				</div>

				{/* 付款詳情(不含折扣券,與 App 一致) */}
				<div className="px-5 pt-3">
					<h2 className="text-base font-semibold text-hk-heading mb-3">
						付款詳情
					</h2>
					<dl className="rounded-hkui-lg border border-divider p-4 grid gap-2.5 text-sm">
						<div className="flex justify-between">
							<dt className="text-foreground-soft">商品總金額</dt>
							<dd className="m-0 tabular-nums">
								NT$ {retryAmountText(order.totalOriginalAmount)}
							</dd>
						</div>
						<div className="flex justify-between">
							<dt className="text-foreground-soft">運費總金額</dt>
							<dd className="m-0 tabular-nums">
								NT$ {retryAmountText(order.shippingFee)}
							</dd>
						</div>
						<div className="border-t border-dashed border-divider pt-2.5 flex justify-between items-center">
							<dt className="font-semibold text-hk-heading">總付款金額</dt>
							<dd className="m-0 font-display text-xl font-bold text-hk-danger tabular-nums">
								NT$ {retryAmountText(order.totalActualAmount)}
							</dd>
						</div>
					</dl>
				</div>

				{/* 付款方式 */}
				<div className="px-5 pt-5">
					<h2 className="text-base font-semibold text-hk-heading mb-2">
						付款方式
					</h2>
					<ul className="list-none m-0 p-0 rounded-hkui-lg border border-divider divide-y divide-divider">
						{RETRY_PAY_METHODS.map((m, idx) => {
							const disabled = isDisabled(m.payType);
							const selected = idx === selectedIdx;
							const reason =
								isAtmOriginal && m.payType === PaymentMethod.ATM
									? "不可選"
									: !amountEligible(m.payType)
										? (PAYMENT_METHOD_INELIGIBLE_REASON_LABEL[
												amountReason(m.payType)
											] ?? "金額不符")
										: disabled
											? "此瀏覽器無法使用"
											: null;
							return (
								<li key={m.payType}>
									<button
										type="button"
										disabled={disabled}
										onClick={() => setSelectedIdx(idx)}
										className="w-full flex items-center gap-3 px-4 py-3 text-left bg-transparent border-0 cursor-pointer disabled:cursor-not-allowed"
									>
										<span
											className={`shrink-0 inline-grid place-items-center w-4 h-4 rounded-full border ${
												selected ? "border-hk-primary" : "border-default-300"
											} ${disabled ? "opacity-40" : ""}`}
											aria-hidden="true"
										>
											{selected ? (
												<span className="w-2 h-2 rounded-full bg-hk-primary" />
											) : null}
										</span>
										<span
											className={`w-[42px] h-[28px] rounded-hkui-md border border-divider bg-content2 inline-flex items-center justify-center overflow-hidden shrink-0 ${
												disabled ? "opacity-40" : ""
											}`}
											aria-hidden="true"
										>
											<PaymentGlyph value={m.payType} />
										</span>
										<span
											className={`flex-1 text-[0.9375rem] ${
												disabled ? "text-foreground-soft/60" : "text-hk-heading"
											}`}
										>
											{m.label}
										</span>
										{reason ? (
											<span className="text-xs text-foreground-soft">
												{reason}
											</span>
										) : null}
									</button>
								</li>
							);
						})}
					</ul>
					<p className="mt-2 text-xs text-foreground-soft leading-relaxed">
						{selectedIdx ===
						RETRY_PAY_METHODS.findIndex((m) => m.payType === PaymentMethod.ATM)
							? "虛擬帳號只能用網路銀行或 ATM 轉帳,不能臨櫃匯款。"
							: "選擇後將完成這筆訂單的付款。"}
					</p>
				</div>

				<div className="px-5 py-5 flex gap-3">
					<Button
						variant="bordered"
						color="default"
						className="flex-1"
						onClick={onClose}
					>
						取消
					</Button>
					<Button
						className="flex-1 bg-hk-primary border-hk-primary text-hk-inverse hover:bg-hk-primary-hover hover:border-hk-primary-hover"
						disabled={selectedDisabled}
						onClick={() => onConfirm(RETRY_PAY_METHODS[selectedIdx].payType)}
					>
						確認付款
					</Button>
				</div>
			</section>
		</div>,
		document.body,
	);
}

/**
 * Pick the payment method to retry with. Prefer whatever the order
 * was originally created with (so ATM stays ATM, card stays card);
 * fall back to credit card if the field is missing. Mirrors APP's
 * `_isAtmPayment` / `currentPayType` plumbing in retry_payment_sheet.
 */
function resolvePaymentType(order: Order): string {
	return (
		normalizePaymentMethod(order.payment?.method) ?? PaymentMethod.CREDIT_CARD
	);
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
 *   - 尚未付款 red badge + payment-method label (ATM / 信用卡 / etc.)
 *   - 再次付款 (primary CTA — re-creates a payment intent and either
 *     redirects to PayUni's hosted page or refreshes the order with a
 *     new ATM virtual account)
 *   - 取消訂單 (gated by `order.canCancel`)
 * Background story: PayUni's hosted card page can be abandoned mid-flow
 * (browser tab closed, 3DS timeout, bank rejection). Backend keeps the
 * order at status=0 so the buyer can retry without re-checkout.
 */
function PendingPaymentBanner({
	order,
	busy,
	pendingAction,
	onRetry,
	onCancel,
}: PendingPaymentBannerProps) {
	const methodRaw = order.payment?.method;
	const methodCode = resolvePaymentType(order) as PaymentMethodValue;
	const methodLabel = PAYMENT_METHOD_LABEL[methodCode] ?? methodRaw ?? "未指定";
	const isAtm = methodCode === PaymentMethod.ATM;
	const amount = Number(order.totalActualAmount).toLocaleString();
	const atmVirtualAccount = order.payment?.atmVirtualAccount ?? "";
	const atmBankCode = order.payment?.atmBankCode ?? "";
	const atmExpireDate = order.payment?.atmExpireDate ?? "";
	const showAtmInfo = isAtm && atmVirtualAccount !== "";
	// 走共用判準而不是 `order.canCancel`（後端不送那一欄，見 `lib/orders/repo.ts`）。
	// 本 banner 只在待付款時渲染，所以這裡恆為 true——但判準只留一份，才不會在
	// banner 的渲染條件哪天放寬時，這一顆鍵默默跟著出現在不能取消的訂單上。
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
						付款方式
					</dt>
					<dd className="m-0 text-[0.9375rem] font-semibold text-hk-heading dark:text-[#f5e9c3]">
						{methodLabel}
					</dd>
				</div>
				<div className="flex flex-col gap-1 min-w-0">
					<dt className="text-xs text-[#565656] dark:text-[rgb(var(--color-ink-100-rgb)/0.7)] font-medium tracking-wide">
						付款狀態
					</dt>
					<dd className="m-0 text-[0.9375rem] font-semibold text-hk-danger dark:text-[color:var(--color-brand-secondary)]">
						尚未付款
					</dd>
				</div>
			</dl>
			{showAtmInfo ? (
				<AtmPaymentInfo
					virtualAccount={atmVirtualAccount}
					bankCode={atmBankCode}
					expireDate={atmExpireDate}
					amount={order.totalActualAmount}
				/>
			) : isAtm ? (
				<p className="mt-3 text-xs leading-relaxed text-[#565656] dark:text-[rgb(var(--color-ink-100-rgb)/0.7)]">
					ATM 轉帳：點「再次付款」會重新建立虛擬帳號。請於繳款期限內完成轉帳。
				</p>
			) : null}
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
					isLoading={pendingAction === "retry"}
					loadingLabel="建立付款中"
				>
					再次付款
				</Button>
			</div>
		</section>
	);
}

/**
 * Common Taiwan bank codes used by virtual-account payment gateways, mapped
 * to short names so the buyer can sanity-check they're transferring to the
 * right bank. Unknown codes simply render the number alone.
 */
const TW_BANK_NAMES: Record<string, string> = {
	"004": "臺灣銀行",
	"005": "土地銀行",
	"006": "合作金庫",
	"007": "第一銀行",
	"008": "華南銀行",
	"009": "彰化銀行",
	"011": "上海商業儲蓄銀行",
	"012": "台北富邦",
	"013": "國泰世華",
	"017": "兆豐銀行",
	"700": "中華郵政",
	"803": "聯邦銀行",
	"805": "遠東商銀",
	"806": "元大銀行",
	"807": "永豐銀行",
	"808": "玉山銀行",
	"809": "凱基銀行",
	"812": "台新銀行",
	"822": "中國信託",
};

/**
 * ATM virtual-account panel shown inside the 待付款 banner when the buyer
 * chose ATM at checkout and the backend has minted an account.
 *
 * Laid out as a numbered checklist in the EXACT order a buyer keys the
 * fields into a Taiwan 網銀 / ATM 轉帳 screen — ① 銀行代碼 → ② 匯款帳號 →
 * ③ 轉帳金額 — so the screen reads like the bank flow itself. Each field
 * has its own copy button (digits only) to save a transcription step.
 *
 * The "再次付款" button still appears below so a buyer who lets the account
 * expire can mint a new one without re-checkout.
 */
function AtmPaymentInfo({
	virtualAccount,
	bankCode,
	expireDate,
	amount,
}: {
	virtualAccount: string;
	bankCode: string;
	expireDate: string;
	amount: number | string;
}) {
	const [copiedKey, setCopiedKey] = React.useState<string | null>(null);
	const copy = async (key: string, value: string) => {
		try {
			await navigator.clipboard.writeText(value);
			setCopiedKey(key);
			window.setTimeout(() => setCopiedKey(null), 1500);
		} catch {
			/* clipboard may be denied — silently no-op */
		}
	};

	const bankName = TW_BANK_NAMES[bankCode.trim()] ?? "";
	const amountNumber = Number(amount) || 0;
	const amountDigits = String(Math.round(amountNumber));

	// Step list, conditionally numbered so the sequence stays 1/2/3 even when
	// the backend omits a bank code (then it's just 匯款帳號 → 轉帳金額).
	const steps = [
		bankCode
			? {
					key: "bank",
					label: "銀行代碼",
					value: (
						<>
							{bankCode}
							{bankName ? (
								<span className="ml-2.5 text-sm font-semibold text-hk-muted tracking-normal">
									{bankName}
								</span>
							) : null}
						</>
					),
					copyValue: bankCode.trim(),
				}
			: null,
		{
			key: "account",
			label: "匯款帳號",
			value: virtualAccount,
			copyValue: virtualAccount,
		},
		{
			key: "amount",
			label: "轉帳金額",
			value: <>NT$ {amountNumber.toLocaleString()}</>,
			copyValue: amountDigits,
		},
	].filter(Boolean) as {
		key: string;
		label: string;
		value: React.ReactNode;
		copyValue: string;
	}[];

	return (
		<section
			className="mt-4 p-4 rounded-hkui-md bg-hk-surface-page border border-[rgb(var(--color-status-danger-rgb)/0.22)] dark:border-[rgb(var(--color-status-danger-rgb)/0.4)]"
			aria-label="ATM 轉帳資訊"
		>
			<header className="flex items-center gap-3">
				<span
					className="inline-flex items-center justify-center px-2.5 py-1 rounded bg-hk-danger text-hk-cream font-bold text-sm tracking-wide"
					aria-hidden="true"
				>
					ATM
				</span>
				<strong className="text-[0.9375rem] font-bold text-hk-heading leading-tight min-w-0">
					ATM / 網路銀行轉帳
				</strong>
			</header>

			<ul className="flex flex-col gap-2 mt-4">
				{steps.map((s) => (
					<li
						key={s.key}
						className="flex items-center gap-3 p-3 rounded-hkui-md bg-hk-surface-card border border-[rgb(var(--color-status-danger-rgb)/0.14)]"
					>
						<div className="flex-1 min-w-0">
							<small className="block text-xs text-hk-muted font-medium">
								{s.label}
							</small>
							<strong className="block text-lg font-extrabold text-hk-heading tracking-wider break-all tabular-nums leading-snug">
								{s.value}
							</strong>
						</div>
						<button
							type="button"
							className="shrink-0 px-3 py-1.5 rounded-full border border-hk-danger bg-transparent text-hk-danger text-xs font-bold cursor-pointer transition-colors duration-150 hover:bg-hk-danger hover:text-hk-cream whitespace-nowrap"
							onClick={() => void copy(s.key, s.copyValue)}
							aria-label={`複製${s.label}`}
						>
							{copiedKey === s.key ? "已複製" : "複製"}
						</button>
					</li>
				))}
			</ul>

			<div className="flex items-center gap-2 mt-4 pt-3 border-t border-[rgb(var(--color-status-danger-rgb)/0.12)]">
				<span className="text-xs text-hk-muted font-medium">繳費截止</span>
				<strong className="text-sm font-bold text-hk-heading tabular-nums">
					{formatAtmExpire(expireDate)}
				</strong>
			</div>
			<p className="mt-2 text-xs leading-relaxed text-hk-muted">
				逾期未繳費此虛擬帳號將失效，可重新點「再次付款」取得新帳號。
			</p>
		</section>
	);
}

function formatAtmExpire(raw: string): string {
	if (!raw) return "—";
	// Backend sends compact digit strings (YYYYMMDD or YYYYMMDDHHmm) that the
	// Date constructor can't parse — format them by slicing.
	const digits = raw.replace(/\D/g, "");
	if (digits.length === 8) {
		return `${digits.slice(0, 4)}/${digits.slice(4, 6)}/${digits.slice(6, 8)}`;
	}
	if (digits.length === 12) {
		return `${digits.slice(0, 4)}/${digits.slice(4, 6)}/${digits.slice(6, 8)} ${digits.slice(8, 10)}:${digits.slice(10, 12)}`;
	}
	return formatTaipeiDateTime(raw) || raw;
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
