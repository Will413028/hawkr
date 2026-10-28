"use client";

/**
 * Seller-side order detail page — Phase 3 of the migration plan §6.2 Flow D.
 *
 * Renders order info + invokes the right shipping action via
 * `nextShipmentAction(...)`. The four flows are:
 *
 *   黑貓 / 預約出貨    TcatReservationDialog → generateHomeLabel()
 *   黑貓 / 呼叫司機    CallTcatDialog        → callTcat()
 *   黑貓 / 下載 PDF    open labelUrl
 *   7-11 / 預約取號   (no dialog)            → printCvsLabel()
 *   7-11 / 出貨確認   SevenElevenShipDialog → confirmShipment()
 *
 * 5046 / 5047 cooldown + odno override sit in `lib/seller/shipment.ts` and
 * are consulted before each network call. Errors flow through
 * `classifyShipmentError(...)` so the seller sees a friendly message
 * instead of `[5046] xxx`.
 */

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import { BackLink } from "@/components/shared/BackLink";
import { useDrawerMutated } from "@/components/shared/Drawer";
import PaymentGlyph from "@/components/shared/PaymentGlyph";
import { TruckSpinner } from "@/components/shared/TruckSpinner";
import Button, { buttonClasses } from "@/components/ui/Button";
import { DrawerSkeleton } from "@/components/ui/DrawerSkeleton";
import { TemperatureTag } from "@/features/cart/components/TemperatureTag";
import OrderStatusTimeline from "@/features/orders/components/OrderStatusTimeline";
import {
	CallTcatDialog,
	type CallTcatParams,
	SevenElevenShipDialog,
	type SevenElevenShipResult,
	TcatReservationDialog,
	type TcatReservationResult,
	type TcatReservationSubmitResult,
} from "@/features/seller/components/ShipmentDialogs";
import { SellerOrderInvoiceCard } from "@/features/seller-center/invoices";
import CancelOrderDialog, {
	canCancelOrder,
} from "@/features/seller-center/orders/CancelOrderDialog";
import { SellerOrderAfterSaleCard } from "@/features/seller-center/orders/SellerOrderAfterSaleCard";
import { afterSaleNeedsSeller } from "@/lib/after-sales/status";
import {
	formatShippingMethodLabel,
	PAYMENT_METHOD_LABEL,
} from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useOrder } from "@/lib/hooks/useOrders";
import { useResettableError } from "@/lib/hooks/useResettableError";
import { useSellerAfterSales } from "@/lib/hooks/useSellerAfterSales";
import { useSellerShipmentAnomalies } from "@/lib/hooks/useSellerShipmentAnomalies";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { type OrderReview, STATUS_LABEL } from "@/lib/orders/types";
import {
	callTcat,
	classifyShipmentError,
	confirmShipment,
	effectiveOdno,
	effectiveValidationNo,
	generateHomeLabel,
	guardShipmentCooldown,
	isErrorShipStatus,
	isFinalShipStatus,
	isLabelExpired,
	type LogisticsInfo,
	lgsTypeForShippingMethod,
	markShipmentCreated,
	nextShipmentAction,
	printCvsLabel,
	progressPercentFor,
	queryLogistics,
	resetShipmentRetryCount,
	type ShipmentErrorGuidance,
	type ShipmentErrorOutcome,
	setLocalOdnoOverride,
	shipmentErrorGuidance,
	shipStatusLabel,
} from "@/lib/seller/shipment";
import { useChatDrawerStore } from "@/lib/state/chat-drawer-store";
import {
	deliveryTempLabel,
	formatDeadline,
	formatLabelExpire,
} from "./orderDisplay";
import {
	isShipmentOperationExpired,
	SHIPMENT_FALLBACK_REFRESH_DELAY_MS,
	shipmentOperationExpiresAt,
	shipmentOperationLabel,
	useSellerShipmentOperationStore,
} from "./shipmentOperationStore";

interface BannerState {
	tone: "success" | "info" | "error" | "warning";
	message: string;
	/** 進階引導（如黑貓寄件地址不在收件範圍時的處理建議） */
	guidance?: ShipmentErrorGuidance | null;
}

const orderActionIcons = {
	shipConfirm: "truck",
	phoneContact: "call",
	sendMessage: "send",
	cancelOrder: "close",
} as const;

type OrderActionIconName = keyof typeof orderActionIcons;

function OrderActionIcon({ name }: { name: OrderActionIconName }) {
	return (
		<AppIcon
			name={orderActionIcons[name] as AppIconName}
			size={18}
			className="shrink-0"
		/>
	);
}

function OrderActionLabel({
	icon,
	children,
}: {
	icon: OrderActionIconName;
	children: React.ReactNode;
}) {
	return (
		<span className="inline-flex items-center justify-center gap-2">
			<OrderActionIcon name={icon} />
			<span>{children}</span>
		</span>
	);
}

export default function OrderDetailClient({
	orderNo,
	bare = false,
}: {
	orderNo: string;
	/** Rendered inside the orders-list drawer — hide the「← 返回」links since
	 * the Sheet already provides a close button and stays on the list. */
	bare?: boolean;
}) {
	const { order, loading, error, refresh } = useOrder(orderNo, {
		view: "seller",
	});
	// 抽屜模式：出貨 / 取號 / 取消等操作成功後即時通知父頁訂單工作區同步；
	// 全頁模式（/orders/[orderNo]）為 no-op，僅靠自身 refresh。
	const { active: drawerActive, notify: notifyDrawer } = useDrawerMutated();
	const openChat = useChatDrawerStore((state) => state.openChat);
	const { lookup: afterSaleLookup } = useSellerAfterSales();
	const aftersaleCase = afterSaleLookup.get(orderNo);
	const aftersaleNeedsAction = afterSaleNeedsSeller(aftersaleCase);
	// 配送異常的訂單在列表的「訂單狀態」欄顯示「異常」徽章；點進詳情時在這裡把
	// 異常原因（類型／嚴重度／偵測時間）顯示給賣家。
	const { lookup: anomalyLookup } = useSellerShipmentAnomalies();
	const anomaly = anomalyLookup.get(orderNo);

	const [busy, setBusy] = React.useState(false);
	const [banner, setBanner] = React.useState<BannerState | null>(null);
	const [tcatReserveOpen, setTcatReserveOpen] = React.useState(false);
	const [callTcatOpen, setCallTcatOpen] = React.useState(false);
	const [sevenOpen, setSevenOpen] = React.useState(false);
	const [cancelOpen, setCancelOpen] = React.useState(false);
	const shipmentOperation = useSellerShipmentOperationStore(
		(state) => state.operations[orderNo],
	);
	const startTcatReservation = useSellerShipmentOperationStore(
		(state) => state.startTcatReservation,
	);
	const syncTcatReservation = useSellerShipmentOperationStore(
		(state) => state.syncTcatReservation,
	);
	const clearShipmentOperation = useSellerShipmentOperationStore(
		(state) => state.clearOperation,
	);
	const pruneExpiredShipmentOperations = useSellerShipmentOperationStore(
		(state) => state.pruneExpired,
	);

	const showBanner = React.useCallback((b: BannerState) => {
		setBanner(b);
		// 帶引導的錯誤需要賣家閱讀步驟並點連結，不自動關閉；其餘 6 秒後淡出。
		if (b.guidance) return;
		window.setTimeout(() => {
			setBanner((prev) => (prev === b ? null : prev));
		}, 6000);
	}, []);

	// 將物流錯誤統一轉成 banner：沿用 classifyShipmentError 的 tone / 訊息，並補上
	// 可行動的引導（如黑貓寄件地址不在收件範圍）。未處理的碼才加動作前綴。
	// shouldReload 仍由各 catch 處理（避免重整把錯誤橫幅蓋掉的時機差異）。
	const showShipmentError = React.useCallback(
		(o: ShipmentErrorOutcome, fallbackPrefix: string) => {
			showBanner({
				tone: o.tone === "info" ? "info" : o.tone,
				message: o.handled ? o.message : `${fallbackPrefix}：${o.message}`,
				guidance: shipmentErrorGuidance(o),
			});
		},
		[showBanner],
	);

	const refreshAfterMutation = React.useCallback(async () => {
		await refresh();
		notifyDrawer();
	}, [notifyDrawer, refresh]);

	const action = React.useMemo(
		() => nextShipmentAction(orderNo, order?.shipping, order?.status ?? ""),
		[order?.shipping, order?.status, orderNo],
	);
	const tcatReservationProcessing =
		shipmentOperation?.kind === "tcat-reserve" &&
		!isShipmentOperationExpired(shipmentOperation) &&
		action.kind === "tcat-reserve";
	const tcatReservationSyncing =
		tcatReservationProcessing && shipmentOperation?.phase === "syncing";

	// The list page owns realtime reconciliation while this detail is inside its
	// drawer. On the standalone detail route, release the lock as soon as its
	// focused order query advances.
	React.useEffect(() => {
		pruneExpiredShipmentOperations();
		if (!order || !shipmentOperation) return;
		if (action.kind !== shipmentOperation.kind) {
			clearShipmentOperation(orderNo);
		}
	}, [
		action.kind,
		clearShipmentOperation,
		order,
		orderNo,
		pruneExpiredShipmentOperations,
		shipmentOperation,
	]);

	// Mutation success already performs an immediate focused refresh. The
	// standalone route gets only one delayed fallback request in case the
	// backend event/read model lagged; drawer mode delegates this to the list.
	const standaloneFallbackKey =
		!drawerActive && tcatReservationSyncing && shipmentOperation
			? `${orderNo}:${shipmentOperation.phase}:${shipmentOperation.updatedAt}`
			: "";
	const refreshOrderRef = React.useRef(refresh);
	refreshOrderRef.current = refresh;
	React.useEffect(() => {
		if (!standaloneFallbackKey) return;
		const timer = window.setTimeout(() => {
			pruneExpiredShipmentOperations();
			void refreshOrderRef.current();
		}, SHIPMENT_FALLBACK_REFRESH_DELAY_MS);
		return () => window.clearTimeout(timer);
	}, [pruneExpiredShipmentOperations, standaloneFallbackKey]);

	// Local-only expiry unlock; this timer never calls the backend.
	const shipmentOperationExpiryAt =
		shipmentOperation && !isShipmentOperationExpired(shipmentOperation)
			? shipmentOperationExpiresAt(shipmentOperation)
			: null;
	React.useEffect(() => {
		if (shipmentOperationExpiryAt == null) return;
		const delay = Math.max(0, shipmentOperationExpiryAt - Date.now() + 50);
		const timer = window.setTimeout(
			() => pruneExpiredShipmentOperations(),
			delay,
		);
		return () => window.clearTimeout(timer);
	}, [pruneExpiredShipmentOperations, shipmentOperationExpiryAt]);

	// 配送方式原碼。**不再算 isTaqbin 這個布林** —— 它只答得出「是不是黑貓」，
	// 而畫面拿它當二分法用（不是黑貓就當 7-11），3003／3004／3005 全被畫成 7-11。
	const shippingMethod = order?.shipping?.method ?? "";

	/* ===== Action handlers ===== */

	const handleTcatReserve = async (
		result: TcatReservationResult,
	): Promise<TcatReservationSubmitResult> => {
		const existingOperation =
			useSellerShipmentOperationStore.getState().operations[orderNo];
		if (existingOperation && !isShipmentOperationExpired(existingOperation)) {
			setTcatReserveOpen(false);
			showBanner({
				tone: "info",
				message: "這筆訂單正在處理預約，完成後會自動更新",
			});
			return { ok: true };
		}
		const cooldownMsg = guardShipmentCooldown(orderNo);
		if (cooldownMsg) {
			syncTcatReservation(orderNo);
			setTcatReserveOpen(false);
			showBanner({ tone: "info", message: cooldownMsg });
			return { ok: true };
		}
		startTcatReservation(orderNo);
		setBusy(true);
		try {
			await generateHomeLabel(orderNo, {
				shipDate: result.shipDate,
				stagedTokens: result.stagedToken ? [result.stagedToken] : undefined,
			});
			markShipmentCreated(orderNo);
			resetShipmentRetryCount(orderNo);
			syncTcatReservation(orderNo);
			setTcatReserveOpen(false);
			showBanner({
				tone: "success",
				message: "已收到預約，完成後訂單狀態會自動更新",
			});
			await refreshAfterMutation();
			return { ok: true };
		} catch (e) {
			const o = classifyShipmentError(e, orderNo);
			if (o.shouldReload || o.bizCode === 5046 || o.bizCode === 5047) {
				syncTcatReservation(orderNo);
				setTcatReserveOpen(false);
				showShipmentError(o, "預約出貨失敗");
				if (o.shouldReload) await refreshAfterMutation();
				return { ok: true };
			} else {
				clearShipmentOperation(orderNo);
			}
			showShipmentError(o, "預約出貨失敗");
			return {
				ok: false,
				error: o.handled ? o.message : `預約出貨失敗：${o.message}`,
			};
		} finally {
			setBusy(false);
		}
	};

	const handleCallTcat = async (params: CallTcatParams) => {
		setCallTcatOpen(false);
		setBusy(true);
		try {
			await callTcat(orderNo, params);
			showBanner({
				tone: "success",
				message: "已呼叫黑貓取件，司機將於兩日內到府收件",
			});
			await refreshAfterMutation();
		} catch (e) {
			// 黑貓叫件失敗（5044，PayUni E072 等真因）已由 classifyShipmentError
			// 透傳實際原因（handled:true），不再吞成籠統文案或猜成假日。未知碼
			// 才走 fallback，並保留後端訊息而非整段丟棄。寄件地址不在收件範圍時，
			// showShipmentError 會再補上「修正地址 / 找站所 / 自行預約」的引導。
			const o = classifyShipmentError(e, orderNo);
			showShipmentError(o, "呼叫黑貓取件失敗");
			if (o.shouldReload) await refreshAfterMutation();
		} finally {
			setBusy(false);
		}
	};

	const handleCvsPrint = async () => {
		const cooldownMsg = guardShipmentCooldown(orderNo);
		if (cooldownMsg) {
			showBanner({ tone: "info", message: cooldownMsg });
			return;
		}
		setBusy(true);
		try {
			const label = await printCvsLabel(orderNo);
			markShipmentCreated(orderNo);
			resetShipmentRetryCount(orderNo);
			if (!label.webhookReceived) {
				showBanner({
					tone: "info",
					message: "系統繁忙中，交貨便代碼即將生成，請稍後刷新",
				});
				await refreshAfterMutation();
				return;
			}
			if (!label.odno) {
				showBanner({ tone: "error", message: "無法取得交貨便代碼" });
				return;
			}
			// Local override fallback for B-147 (list endpoint lag)
			setLocalOdnoOverride(orderNo, label.odno, label.validationNo);
			showBanner({
				tone: "success",
				message: `取號成功：${label.odno}${label.validationNo}`,
			});
			await refreshAfterMutation();
		} catch (e) {
			const o = classifyShipmentError(e, orderNo);
			showShipmentError(o, "預約取號失敗");
			if (o.shouldReload) await refreshAfterMutation();
		} finally {
			setBusy(false);
		}
	};

	const handleSevenSubmit = async (result: SevenElevenShipResult) => {
		setSevenOpen(false);
		setBusy(true);
		try {
			await confirmShipment(orderNo, {
				stagedTokens: result.stagedToken ? [result.stagedToken] : undefined,
			});
			showBanner({ tone: "success", message: "出貨確認成功" });
			await refreshAfterMutation();
		} catch (e) {
			const o = classifyShipmentError(e, orderNo);
			showShipmentError(o, "出貨確認失敗");
			if (o.shouldReload) await refreshAfterMutation();
		} finally {
			setBusy(false);
		}
	};

	/* ===== Render guards ===== */

	if (loading && !order) {
		// 固定形狀骨架取代單行「載入中…」：抽屜/頁面內容從一行跳成整頁
		// 會讓底部大幅跳動（捲軸也跟著出現/消失），骨架把落差降到最小。
		return (
			<div className="seller-order-detail">
				<DrawerSkeleton rows={3} className="p-4" />
			</div>
		);
	}

	if (error || !order) {
		return (
			<div className="seller-order-detail">
				<p className="seller-order-detail__error" role="alert">
					{error || "找不到訂單"}
				</p>
				{!bare && (
					<BackLink
						fallbackHref="/seller-center/orders"
						className="seller-order-detail__back"
					>
						<AppIcon name="arrow-left" size={16} />
						返回
					</BackLink>
				)}
			</div>
		);
	}

	const created = order.createdAt
		? new Date(
				order.createdAt < 1e12 ? order.createdAt * 1000 : order.createdAt,
			)
		: null;

	const itemCount = (order.items ?? []).reduce(
		(sum, it) => sum + Number(it.quantity || 0),
		0,
	);

	// 出貨/取消期限 + 溫層 + 託運單到期 + 出貨照片 — 對齊列表頁與 App OrderInfoColumn。
	// 待出貨(paid/preparing) 顯示「出貨期限」(amber)，其餘顯示「取消期限」(red)。
	const isShipDeadline =
		order.status === "paid" || order.status === "preparing";
	const deadlineLabel = formatDeadline(order.autoCancelAt);
	const tempLabel = deliveryTempLabel(order.shipping?.deliveryTemp);
	const labelExpireLabel = formatLabelExpire(order.shipping?.labelExpireAt);
	// 取號後 4 天失效（對齊 App）：失效的託運單一律不再提供下載。
	const labelExpired = isLabelExpired(order.shipping);
	const shipmentPhotos = order.shipping?.shipmentPhotoUrls ?? [];
	const receiptPhotos = order.shipping?.receiptPhotoUrls ?? [];

	/* ===== Action button ===== */

	let primaryCta: React.ReactNode = null;
	if (action.kind === "tcat-reserve") {
		const reservationBusyLabel = shipmentOperation
			? shipmentOperationLabel(shipmentOperation)
			: "";
		primaryCta = (
			<Button
				size="lg"
				onClick={() => setTcatReserveOpen(true)}
				disabled={busy || tcatReservationProcessing}
				isLoading={tcatReservationProcessing}
				loadingIndicator={
					<TruckSpinner label={reservationBusyLabel} size={18} />
				}
				loadingLabel={reservationBusyLabel}
			>
				預約出貨
			</Button>
		);
	} else if (action.kind === "tcat-call") {
		primaryCta = (
			<Button
				size="lg"
				onClick={() => setCallTcatOpen(true)}
				disabled={busy}
				isLoading={busy}
				loadingIndicator={<TruckSpinner label="呼叫黑貓取件中" size={18} />}
				loadingLabel="呼叫黑貓取件中…"
			>
				呼叫黑貓取件
			</Button>
		);
	} else if (action.kind === "tcat-pdf") {
		primaryCta = (
			<a
				href={action.labelUrl}
				target="_blank"
				rel="noreferrer noopener"
				className={buttonClasses({ size: "lg" })}
			>
				下載寄件單 PDF
			</a>
		);
	} else if (action.kind === "cvs-print") {
		primaryCta = (
			<Button
				size="lg"
				onClick={() => void handleCvsPrint()}
				disabled={busy}
				isLoading={busy}
				loadingIndicator={<TruckSpinner label="預約取號中" size={18} />}
				loadingLabel="預約取號中…"
			>
				預約取號
			</Button>
		);
	} else if (action.kind === "cvs-confirm") {
		primaryCta = (
			<Button
				size="lg"
				onClick={() => setSevenOpen(true)}
				disabled={busy}
				isLoading={busy}
				loadingIndicator={<TruckSpinner label="確認出貨中" size={18} />}
				loadingLabel="確認出貨中…"
			>
				<OrderActionLabel icon="shipConfirm">出貨確認</OrderActionLabel>
			</Button>
		);
	} else if (action.kind === "unsupported") {
		// 🔴 未知配送方式：不猜成黑貓也不猜成 7-11，明說不支援＋原碼。
		// 舊版在這裡會畫出「預約取號」，按下去真的打超商取號 API。
		primaryCta = (
			<p className="seller-order-detail__tracking-error" role="alert">
				此配送方式（{action.method || "未指定"}
				）尚未支援平台出貨操作，請聯繫平台客服。
			</p>
		);
	}

	const cvsOdno = effectiveOdno(orderNo, order.shipping?.odno);
	const cvsValidationNo = effectiveValidationNo(
		orderNo,
		order.shipping?.validationNo,
	);

	// Build the timeline-event map from whatever timestamps the order
	// record carries. Missing stamps just don't render under their step.
	const paidStamp = order.paidAt ?? order.payment?.paidAt ?? null;
	const timelineEvents: Partial<Record<string, number>> = {};
	if (order.createdAt) timelineEvents.pending_payment = order.createdAt;
	if (paidStamp) timelineEvents.paid = paidStamp;
	if (order.shipping?.deliveredAt) {
		timelineEvents.completed = order.shipping.deliveredAt;
	}

	return (
		<div
			className={`seller-order-detail-page${
				bare ? " seller-order-detail-page--bare" : ""
			}`}
			style={bare ? { position: "relative" } : undefined}
		>
			<div
				className="seller-order-detail"
				style={bare ? { position: "relative", zIndex: 1 } : undefined}
			>
				<header className="seller-order-detail__head">
					{!bare && (
						<BackLink
							fallbackHref="/seller-center/orders"
							className="seller-order-detail__back"
						>
							<AppIcon name="arrow-left" size={16} />
							返回
						</BackLink>
					)}
					<div className="seller-order-detail__title">
						<h1>訂單 #{order.orderNo}</h1>
						<span className={`seller-order-detail__status is-${order.status}`}>
							{STATUS_LABEL[order.status] || order.status}
						</span>
						<CopyButton value={order.orderNo} label="複製單號" />
					</div>
					{created && <small>下單時間：{formatTaipeiDateTime(created)}</small>}
					{deadlineLabel && (
						<small
							className={[
								"seller-orders__deadline",
								isShipDeadline
									? "seller-orders__deadline--ship"
									: "seller-orders__deadline--cancel",
							].join(" ")}
						>
							{isShipDeadline ? "出貨期限" : "取消期限"}：{deadlineLabel}
						</small>
					)}
				</header>

				<OrderStatusTimeline
					status={order.status}
					events={timelineEvents}
					cancelReason={order.cancelReason}
					cancelByRole={order.cancelByRole}
					cancelRemark={order.cancelRemark}
					aftersale={aftersaleCase}
					shippingMethod={order.shipping?.method}
				/>

				{anomaly && (
					<div className="seller-order-detail__banner is-warning" role="alert">
						配送異常（{anomaly.severityLabel}）：{anomaly.anomalyTypeLabel}
						{anomaly.detectedAt
							? `，偵測於 ${formatTaipeiDateTime(anomaly.detectedAt)}`
							: ""}
						{anomaly.providerMessage && (
							<div className="seller-order-detail__banner-note">
								PayUni：{anomaly.providerMessage}
							</div>
						)}
					</div>
				)}

				{banner && (
					<div
						className={`seller-order-detail__banner is-${banner.tone}`}
						role={banner.tone === "error" ? "alert" : "status"}
					>
						<div className="seller-order-detail__banner-message">
							{banner.message}
						</div>
						{banner.guidance && (
							<div className="seller-order-detail__banner-guidance">
								<p className="seller-order-detail__banner-note">
									{banner.guidance.cause}
								</p>
								<ol className="seller-order-detail__banner-steps">
									{banner.guidance.steps.map((step) => (
										<li key={step}>{step}</li>
									))}
								</ol>
								{banner.guidance.links.length > 0 && (
									<div className="seller-order-detail__banner-links">
										{banner.guidance.links.map((lnk) => (
											<a
												key={lnk.href}
												href={lnk.href}
												target="_blank"
												rel="noopener noreferrer"
												className="seller-order-detail__banner-link"
											>
												{lnk.label}
											</a>
										))}
									</div>
								)}
							</div>
						)}
					</div>
				)}

				<div className="seller-order-detail__grid">
					<section className="seller-card seller-order-detail__main">
						<header className="seller-card__head">
							<h2>商品（{itemCount} 件）</h2>
						</header>
						<ul className="seller-order-detail__items">
							{(order.items ?? []).map((it) => {
								const imageSrc = it.image ? resolveCdnUrl(it.image) : "";
								return (
									<li key={String(it.orderItemId)}>
										{imageSrc ? (
											<AppImage
												src={imageSrc}
												alt={it.name}
												width={64}
												height={64}
												sizes="64px"
												preset="thumb"
											/>
										) : (
											<span
												className="seller-order-detail__item-fallback"
												aria-hidden
											/>
										)}
										<div>
											<strong>{it.name}</strong>
											{it.specName && <small>{it.specName}</small>}
										</div>
										<div className="seller-order-detail__item-amt">
											<span>NT$ {Number(it.price || 0).toLocaleString()}</span>
											<small>× {it.quantity}</small>
										</div>
									</li>
								);
							})}
						</ul>

						<dl className="seller-order-detail__totals">
							<div>
								<dt>商品小計</dt>
								<dd>
									NT${" "}
									{Number(
										order.totalOriginalAmount ?? order.totalActualAmount,
									).toLocaleString()}
								</dd>
							</div>
							{order.shippingFee && (
								<div>
									<dt>運費</dt>
									<dd>NT$ {Number(order.shippingFee).toLocaleString()}</dd>
								</div>
							)}
							{order.discount && Number(order.discount) > 0 && (
								<div>
									<dt>折扣</dt>
									<dd>- NT$ {Number(order.discount).toLocaleString()}</dd>
								</div>
							)}
							<div className="seller-order-detail__totals-final">
								<dt>買家實付</dt>
								<dd>NT$ {Number(order.totalActualAmount).toLocaleString()}</dd>
							</div>
							{(order.platformFee || order.sellerNetAmount) && (
								<>
									{order.platformFee && Number(order.platformFee) > 0 && (
										<div className="seller-order-detail__totals-deduction">
											<dt>平台手續費</dt>
											<dd>
												- NT$ {Number(order.platformFee).toLocaleString()}
											</dd>
										</div>
									)}
									{order.sellerNetAmount && (
										<div className="seller-order-detail__totals-net">
											<dt>你的入帳金額</dt>
											<dd>
												NT$ {Number(order.sellerNetAmount).toLocaleString()}
											</dd>
										</div>
									)}
								</>
							)}
						</dl>

						<DeliveryPhotoComparison
							sellerPhotos={shipmentPhotos}
							buyerPhotos={receiptPhotos}
						/>
					</section>

					<aside className="seller-order-detail__side">
						{aftersaleCase && aftersaleNeedsAction ? (
							<SellerOrderAfterSaleCard aftersale={aftersaleCase} />
						) : null}
						<section className="seller-card">
							<header className="seller-card__head">
								<h2>顧客資訊</h2>
							</header>
							<dl className="seller-order-detail__info">
								<div>
									<dt>收件人</dt>
									<dd>{order.shipping?.receiverName || "—"}</dd>
								</div>
								<div>
									<dt>電話</dt>
									<dd>
										{order.shipping?.receiverPhone || "—"}
										{order.shipping?.receiverPhone && (
											<span className="seller-order-detail__inline-actions">
												<CopyButton value={order.shipping.receiverPhone} />
											</span>
										)}
									</dd>
								</div>
								{order.shipping?.address && (
									<div>
										<dt>地址</dt>
										<dd>
											{(() => {
												const fullAddress = [
													order.shipping.city,
													order.shipping.district,
													order.shipping.address,
												]
													.filter(Boolean)
													.join("");
												return (
													<>
														{fullAddress}
														<span className="seller-order-detail__inline-actions">
															<CopyButton value={fullAddress} />
														</span>
													</>
												);
											})()}
										</dd>
									</div>
								)}
								{order.shipping?.storeName && (
									<div>
										<dt>取件門市</dt>
										<dd>
											{order.shipping.storeName}
											{order.shipping.storeCode && (
												<small> ({order.shipping.storeCode})</small>
											)}
										</dd>
									</div>
								)}
							</dl>
							{(order.shipping?.receiverPhone ||
								order.buyerId ||
								canCancelOrder(order)) && (
								<div className="seller-order-detail__quick-actions">
									{order.shipping?.receiverPhone && (
										<a
											className={buttonClasses({
												variant: "bordered",
												color: "default",
												size: "sm",
											})}
											href={`tel:${order.shipping.receiverPhone}`}
										>
											<OrderActionLabel icon="phoneContact">
												電話聯絡
											</OrderActionLabel>
										</a>
									)}
									{order.buyerId && (
										<button
											type="button"
											className={buttonClasses({
												variant: "bordered",
												color: "default",
												size: "sm",
											})}
											onClick={() => openChat(order.buyerId as string)}
										>
											<OrderActionLabel icon="sendMessage">
												傳送訊息
											</OrderActionLabel>
										</button>
									)}
									{canCancelOrder(order) && (
										<Button
											variant="bordered"
											color="danger"
											size="sm"
											className="hover:!bg-[#ff5a1f]/10 dark:hover:!bg-[#ff8a5b]/10"
											onClick={() => setCancelOpen(true)}
											disabled={busy || tcatReservationProcessing}
										>
											<OrderActionLabel icon="cancelOrder">
												取消訂單
											</OrderActionLabel>
										</Button>
									)}
								</div>
							)}
						</section>

						{order.review ? <SellerReviewCard review={order.review} /> : null}

						<section className="seller-card">
							<header className="seller-card__head">
								<h2>出貨</h2>
								{primaryCta}
							</header>
							{tcatReservationSyncing && (
								<div
									className="seller-order-detail__shipment-processing"
									role="status"
									aria-live="polite"
								>
									<TruckSpinner label="預約處理中" size={20} />
									<span>
										已收到預約，正在建立寄件單。完成後會自動更新，這段期間不需重複操作。
									</span>
								</div>
							)}
							<dl className="seller-order-detail__info">
								<div>
									<dt>配送方式</dt>
									<dd>
										{formatShippingMethodLabel(shippingMethod)}
										{tempLabel && (
											<>
												{" "}
												<TemperatureTag
													value={order.shipping?.deliveryTemp ?? ""}
												/>
											</>
										)}
									</dd>
								</div>
								{order.shipping?.shipmentNo && (
									<div>
										<dt>物流單號</dt>
										<dd>{order.shipping.shipmentNo}</dd>
									</div>
								)}
								{labelExpireLabel && (
									<div>
										<dt>託運單到期</dt>
										<dd>
											{labelExpireLabel}
											{labelExpired && "（已失效）"}
										</dd>
									</div>
								)}
								{cvsOdno && (
									<div>
										<dt>交貨便代碼</dt>
										<dd>
											<code>
												{cvsOdno}
												{cvsValidationNo}
											</code>
										</dd>
									</div>
								)}
								{order.shipping?.labelUrl && !labelExpired && (
									<div>
										<dt>寄件單</dt>
										<dd>
											<a
												href={order.shipping.labelUrl}
												target="_blank"
												rel="noreferrer noopener"
											>
												下載 PDF
											</a>
										</dd>
									</div>
								)}
							</dl>

							{order.shipping?.shipTradeNo && (
								<LogisticsTracking
									orderNo={orderNo}
									shipTradeNo={order.shipping.shipTradeNo}
									method={shippingMethod}
								/>
							)}
						</section>

						<section className="seller-card">
							<header className="seller-card__head">
								<h2>付款</h2>
							</header>
							<dl className="seller-order-detail__info">
								<div>
									<dt>狀態</dt>
									<dd>{order.payment?.paidAt ? "已付款" : "未付款"}</dd>
								</div>
								{order.payment?.method && (
									<div>
										<dt>方式</dt>
										<dd className="flex min-h-6 items-center gap-2">
											{PAYMENT_METHOD_LABEL[order.payment.method] ? (
												<>
													<PaymentGlyph value={order.payment.method} label="" />
													<span>
														{PAYMENT_METHOD_LABEL[order.payment.method]}
													</span>
												</>
											) : (
												<PaymentGlyph value={order.payment.method} />
											)}
										</dd>
									</div>
								)}
								{order.payment?.tradeNo && (
									<div>
										<dt>交易序號</dt>
										<dd>
											<code>{order.payment.tradeNo}</code>
										</dd>
									</div>
								)}
							</dl>
						</section>

						{/*
						 * 58 號電子發票 S07（58A §S07）。插在付款卡之後、售後卡之前。
						 *
						 * 🔴 只吃 `orderNo` 一個 prop、不回傳任何 state ——「發票異常
						 * disable 出貨鍵」在結構上就不可能發生。加 callback prop 之前
						 * 先讀該元件的檔頭。
						 */}
						<SellerOrderInvoiceCard orderNo={orderNo} />

						{aftersaleCase && !aftersaleNeedsAction ? (
							<SellerOrderAfterSaleCard aftersale={aftersaleCase} />
						) : null}
					</aside>
				</div>

				<TcatReservationDialog
					open={tcatReserveOpen}
					onClose={() => setTcatReserveOpen(false)}
					onSubmit={handleTcatReserve}
				/>
				<CallTcatDialog
					open={callTcatOpen}
					onClose={() => setCallTcatOpen(false)}
					onSubmit={handleCallTcat}
				/>
				<SevenElevenShipDialog
					open={sevenOpen}
					onClose={() => setSevenOpen(false)}
					onSubmit={handleSevenSubmit}
					odno={cvsOdno}
					validationNo={cvsValidationNo}
				/>
				{cancelOpen && (
					<CancelOrderDialog
						order={order}
						onClose={() => setCancelOpen(false)}
						onDone={async () => {
							setCancelOpen(false);
							await refreshAfterMutation();
						}}
					/>
				)}
			</div>
		</div>
	);
}

function DeliveryPhotoComparison({
	sellerPhotos,
	buyerPhotos,
}: {
	sellerPhotos: string[];
	buyerPhotos: string[];
}) {
	const hasSeller = sellerPhotos.length > 0;
	const hasBuyer = buyerPhotos.length > 0;
	if (!hasSeller && !hasBuyer) return null;

	return (
		<div className="seller-order-detail__delivery-photos">
			<div className="seller-order-detail__delivery-photos-head">
				<h3>買賣家上傳照片</h3>
			</div>
			{hasSeller && hasBuyer ? (
				<div className="seller-order-detail__photo-comparison">
					<DeliveryPhotoColumn label="賣家出貨照片" photos={sellerPhotos} />
					<span className="seller-order-detail__photo-vs" aria-hidden>
						VS
					</span>
					<DeliveryPhotoColumn label="買家收貨照片" photos={buyerPhotos} />
				</div>
			) : (
				<DeliveryPhotoGroup
					label={hasSeller ? "賣家出貨照片" : "買家收貨照片"}
					photos={hasSeller ? sellerPhotos : buyerPhotos}
				/>
			)}
		</div>
	);
}

function DeliveryPhotoColumn({
	label,
	photos,
}: {
	label: string;
	photos: string[];
}) {
	return (
		<div className="seller-order-detail__photo-column">
			<span className="seller-order-detail__photo-title">{label}</span>
			<DeliveryPhotoThumb
				url={photos[0] ?? ""}
				overflowCount={photos.length - 1}
			/>
		</div>
	);
}

function DeliveryPhotoGroup({
	label,
	photos,
}: {
	label: string;
	photos: string[];
}) {
	return (
		<div className="seller-order-detail__photo-group">
			<span className="seller-order-detail__photo-title">{label}</span>
			<div className="seller-order-detail__photo-strip">
				{photos.map((url) => (
					<DeliveryPhotoThumb key={url} url={url} />
				))}
			</div>
		</div>
	);
}

function DeliveryPhotoThumb({
	url,
	overflowCount = 0,
}: {
	url: string;
	overflowCount?: number;
}) {
	const src = resolveCdnUrl(url) || url;
	if (!src) return null;

	return (
		<a
			href={src}
			target="_blank"
			rel="noopener noreferrer"
			className="seller-order-detail__photo-thumb"
		>
			<AppImage
				src={url}
				alt="上傳照片"
				width={82}
				height={82}
				sizes="82px"
				preset="thumb"
			/>
			{overflowCount > 0 ? (
				<span className="seller-order-detail__photo-overflow">
					+{overflowCount}
				</span>
			) : null}
		</a>
	);
}

/**
 * 小型複製按鈕 — 點擊把 `value` 寫入剪貼簿，並短暫顯示「已複製」。剪貼簿被
 * 瀏覽器阻擋時靜默忽略（例如非 HTTPS 或權限被拒）。
 */
function CopyButton({
	value,
	label = "複製",
}: {
	value: string;
	label?: string;
}) {
	const [copied, setCopied] = React.useState(false);
	const copy = async () => {
		try {
			await navigator.clipboard.writeText(value);
			setCopied(true);
			window.setTimeout(() => setCopied(false), 1500);
		} catch {
			/* clipboard unavailable — no-op */
		}
	};
	return (
		<button
			type="button"
			className="seller-order-detail__inline-action"
			onClick={() => void copy()}
		>
			{copied ? "已複製" : label}
		</button>
	);
}

/* ============================================================
 *               LogisticsTracking subcomponent
 * ============================================================ */

function LogisticsTracking({
	orderNo,
	shipTradeNo,
	method,
}: {
	orderNo: string;
	shipTradeNo: string;
	/** 配送方式原碼（CodeMeta `ShippingMethod` value 或其歷史別名）。 */
	method: string;
}) {
	const [info, setInfo] = React.useState<LogisticsInfo | null>(null);
	const [loading, setLoading] = React.useState(false);
	// shipTradeNo 是物流追蹤的天然 session boundary:同 component 切換不同
	// 物流單號時用 resetKey 確保前次錯誤不殘留。
	const [error, setError] = useResettableError(shipTradeNo);

	// 🔴 認不得的配送方式回 null。舊寫法 `isTaqbin ? HOME : C2C`
	// 讓 3003／3004／3005 走 7-11 的進度表 —— 型別對、不拋例外、只有進度條是錯的。
	const lgsType = lgsTypeForShippingMethod(method);

	const fetchInfo = React.useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const result = await queryLogistics(orderNo);
			setInfo(result);
			if (!result) setError("查無物流資料");
		} catch (e) {
			setError(e instanceof Error ? e.message : "查詢失敗");
		} finally {
			setLoading(false);
		}
	}, [orderNo]);

	// lgsType 為 null（未知配送方式）時進度未知：不畫進度條，改顯示說明。
	const percent =
		info && lgsType
			? info.progressPercent ||
				(progressPercentFor(info.shipStatus, lgsType) ?? 0)
			: null;
	const statusText = info
		? info.shipStatusDesc || shipStatusLabel(info.shipStatus)
		: "";
	const isError = info ? isErrorShipStatus(info.shipStatus) : false;
	const isFinal = info ? isFinalShipStatus(info.shipStatus) : false;

	return (
		<div className="seller-order-detail__tracking">
			<div className="seller-order-detail__tracking-head">
				<strong>物流追蹤</strong>
				<Button
					variant="bordered"
					color="default"
					size="sm"
					onClick={() => void fetchInfo()}
					disabled={loading}
					isLoading={loading}
					loadingIndicator={<TruckSpinner label="查詢物流狀態中" size={16} />}
					loadingLabel="查詢物流狀態中…"
				>
					{info ? "重新查詢" : "查詢狀態"}
				</Button>
			</div>
			{error && (
				<p className="seller-order-detail__tracking-error" role="alert">
					{error}
				</p>
			)}
			{info && (
				<>
					{percent == null ? (
						<p className="seller-order-detail__tracking-error" role="status">
							此配送方式（{method || "未指定"}
							）的配送進度尚未支援，請以物流商查詢為準。
						</p>
					) : (
						<div
							className={`seller-order-detail__tracking-bar ${
								isError ? "is-error" : isFinal ? "is-final" : ""
							}`}
						>
							<div style={{ width: `${percent}%` }} />
						</div>
					)}
					<dl className="seller-order-detail__info">
						<div>
							<dt>狀態</dt>
							<dd>{statusText}</dd>
						</div>
						{info.shipStatusTime && (
							<div>
								<dt>更新時間</dt>
								<dd>{info.shipStatusTime}</dd>
							</div>
						)}
					</dl>
				</>
			)}
		</div>
	);
}

/** 商品評價：賣家檢視買家對此訂單的評分與評論（後端 orders.review 快照，取最近一筆商品評論）。 */
function SellerReviewCard({ review }: { review: OrderReview }) {
	const rating = Math.max(0, Math.min(5, Math.round(review.rating)));
	return (
		<section className="seller-card">
			<header className="seller-card__head">
				<h2>商品評價</h2>
			</header>
			<div className="seller-order-detail__info">
				{/* display:flex overrides the inherited `.seller-order-detail__info
				    > div` 2-column info grid (80px 1fr) — without it the five star
				    spans + "5 / 5" flow into grid cells and wrap 2-2-1. */}
				<div
					style={{
						display: "flex",
						alignItems: "center",
						fontSize: "1.25rem",
						lineHeight: 1,
					}}
				>
					{[1, 2, 3, 4, 5].map((n) => (
						<span
							key={n}
							style={{
								color:
									rating >= n
										? "var(--hkui-warning)"
										: "var(--hkui-default-200)",
							}}
						>
							<AppIcon name="star" size={20} fill={rating >= n} />
						</span>
					))}
					<span style={{ marginLeft: 8, fontSize: "0.875rem" }}>
						{rating} / 5
					</span>
				</div>
				{review.content ? (
					<p style={{ marginTop: 8, whiteSpace: "pre-wrap" }}>
						{review.content}
					</p>
				) : null}
				{review.createdAt ? (
					<p style={{ marginTop: 8, fontSize: "0.75rem", opacity: 0.7 }}>
						{formatTaipeiDateTime(review.createdAt)}
					</p>
				) : null}
			</div>
		</section>
	);
}
