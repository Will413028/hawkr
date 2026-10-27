"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import { Drawer } from "@/components/shared/Drawer";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { buyerOrderStatusBadgeClass } from "@/features/account/screens/orders/statusBadgeStyles";
import { AfterSaleStatusBadge } from "@/features/after-sales/components/AfterSaleStatusBadge";
import type { AfterSaleCase } from "@/lib/after-sales/types";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { confirmOrderReceipt } from "@/lib/orders/repo";
import {
	canWriteOrderReview,
	type Order,
	STATUS_LABEL,
} from "@/lib/orders/types";
import { sellerPath } from "@/lib/seo/urls";

/**
 * Buyer order quick-action drawer — the list-page counterpart of the seller
 * order drawer. Same shared {@link Drawer} primitive: an in-drawer mutation
 * (確認收貨) fires `onMutated` so the list re-syncs immediately, orthogonal to
 * `onClose` (#1247). Tapping a row opens this right-side drawer with the
 * order summary plus the status-appropriate「下一步」action so the buyer can
 * act (付款 / 查看物流 / 確認收貨 / 退換貨 / 評價) without leaving the list.
 * 「查看訂單詳情」deep-links to the full `/account/orders/[orderNo]` page.
 */
export default function OrderQuickActionDrawer({
	order,
	aftersale,
	afterSaleLoading = false,
	open,
	onClose,
	onMutated,
	loading = false,
}: {
	order: Order | null;
	aftersale?: AfterSaleCase;
	afterSaleLoading?: boolean;
	open: boolean;
	onClose: () => void;
	/** Fired the moment an in-drawer mutation (確認收貨) succeeds so the list
	 * re-syncs immediately — orthogonal to onClose (#1247). */
	onMutated?: () => void;
	/** When the caller only has an order number and is still fetching the full
	 *  order (e.g. the 會員卡 spending list), show a loading state in the body. */
	loading?: boolean;
}) {
	const router = useObservedRouter("order_quick_action_drawer");
	const [busy, setBusy] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	// Reset transient state whenever the drawer targets a different order.
	const orderNo = order?.orderNo ?? null;
	// biome-ignore lint/correctness/useExhaustiveDependencies: orderNo is the intended trigger — clear busy/error when the drawer switches orders, even though the body only calls stable setters.
	React.useEffect(() => {
		setBusy(false);
		setError(null);
	}, [orderNo]);

	const detailHref = orderNo ? `/account/orders/${orderNo}` : "#";

	const handleConfirm = async () => {
		if (!order) return;
		if (
			!window.confirm(
				"確認收貨後不能再申請退換貨，且系統會通知賣家準備結算。確定嗎？",
			)
		) {
			return;
		}
		setBusy(true);
		setError(null);
		try {
			await confirmOrderReceipt(order.orderNo);
			onMutated?.();
			onClose();
			router.push(`/account/orders/${order.orderNo}/review`);
		} catch (e) {
			setError(e instanceof Error ? e.message : "確認收貨失敗");
		} finally {
			setBusy(false);
		}
	};

	const itemCount = order
		? order.items.reduce((sum, it) => sum + Number(it.quantity ?? 0), 0)
		: 0;
	const cover = order?.items[0]?.image;
	const coverSrc = cover ? resolveCdnUrl(cover) : "";
	const created = order?.createdAt
		? new Date(
				order.createdAt < 1e12 ? order.createdAt * 1000 : order.createdAt,
			)
		: null;

	return (
		<Drawer
			open={open}
			onClose={onClose}
			onMutated={onMutated}
			side="right"
			title="訂單操作"
			className="max-w-md"
			footer={
				order ? (
					<div className="flex w-full flex-col gap-2">
						{error ? (
							<p
								className="m-0 text-sm font-semibold text-danger-700"
								role="alert"
							>
								{error}
							</p>
						) : null}
						<NextStepActions
							order={order}
							aftersale={aftersale}
							afterSaleLoading={afterSaleLoading}
							busy={busy}
							onConfirm={handleConfirm}
						/>
						<Button
							href={detailHref}
							variant="bordered"
							color="default"
							onClick={onClose}
						>
							查看訂單詳情
						</Button>
					</div>
				) : null
			}
		>
			{!order && loading ? (
				<p className="py-6 text-center text-sm text-foreground-soft">
					載入訂單中…
				</p>
			) : null}
			{order ? (
				<div className="flex flex-col gap-4">
					<div className="flex items-center justify-between gap-3">
						{order.storeCode ? (
							<Link
								href={sellerPath(order.storeCode)}
								onClick={onClose}
								className="text-base font-semibold text-inherit no-underline hover:text-hk-primary"
							>
								{order.sellerName || "未知賣家"}
							</Link>
						) : (
							<strong className="text-base font-semibold">
								{order.sellerName || "未知賣家"}
							</strong>
						)}
						<span className="inline-flex shrink-0 items-center gap-2">
							{aftersale ? (
								<AfterSaleStatusBadge
									status={aftersale.status}
									appealState={aftersale.appealState}
									size="sm"
								/>
							) : null}
							<span
								className={`rounded-full px-2.5 py-1 text-xs font-semibold ${buyerOrderStatusBadgeClass(order.status)}`}
							>
								{STATUS_LABEL[order.status] || order.status}
							</span>
						</span>
					</div>

					<div className="grid grid-cols-[56px_1fr_auto] items-center gap-3">
						{coverSrc ? (
							order.items[0]?.productId ? (
								<Link
									href={`/products/${encodeURIComponent(String(order.items[0].productId))}`}
									onClick={onClose}
									className="block h-14 w-14"
								>
									<AppImage
										src={coverSrc}
										alt={order.items[0]?.name || ""}
										width={56}
										height={56}
										className="h-14 w-14 rounded-hkui-md bg-default-100 object-cover"
										loading="lazy"
										sizes="56px"
									/>
								</Link>
							) : (
								<AppImage
									src={coverSrc}
									alt={order.items[0]?.name || ""}
									width={56}
									height={56}
									className="h-14 w-14 rounded-hkui-md bg-default-100 object-cover"
									loading="lazy"
									sizes="56px"
								/>
							)
						) : (
							<span
								className="h-14 w-14 rounded-hkui-md bg-default-100"
								aria-hidden="true"
							/>
						)}
						<div className="flex flex-col gap-[2px] text-sm text-hk-body">
							{order.items.slice(0, 2).map((it) => (
								<span key={String(it.orderItemId)}>
									{it.productId ? (
										<Link
											href={`/products/${encodeURIComponent(String(it.productId))}`}
											onClick={onClose}
											className="text-inherit no-underline hover:text-hk-primary"
										>
											{it.name}
										</Link>
									) : (
										it.name
									)}
									<small className="text-foreground-soft">
										{" "}
										× {it.quantity}
									</small>
								</span>
							))}
							{order.items.length > 2 ? (
								<small className="text-foreground-soft">
									等 {order.items.length} 項商品
								</small>
							) : null}
						</div>
						<div className="flex flex-col text-right">
							<small className="text-xs text-foreground-soft">
								共 {itemCount} 件
							</small>
							<strong className="text-base font-bold text-hk-primary">
								NT$ {Number(order.totalActualAmount || 0).toLocaleString()}
							</strong>
						</div>
					</div>

					<dl className="m-0 flex flex-col gap-1 border-t border-divider pt-3 text-xs text-foreground-soft">
						<div className="flex justify-between">
							<dt>訂單編號</dt>
							<dd className="m-0 font-medium text-hk-body">#{order.orderNo}</dd>
						</div>
						{created ? (
							<div className="flex justify-between">
								<dt>下單時間</dt>
								<dd className="m-0 text-hk-body">
									{formatTaipeiDateTime(created)}
								</dd>
							</div>
						) : null}
					</dl>
					{aftersale ? (
						<section className="rounded-hkui-md border border-divider bg-content2 px-3 py-2 text-sm">
							<div className="flex items-center justify-between gap-2">
								<strong className="text-foreground">售後案件</strong>
								<AfterSaleStatusBadge
									status={aftersale.status}
									appealState={aftersale.appealState}
									size="sm"
								/>
							</div>
							<p className="m-0 mt-1 text-xs text-foreground-soft">
								案件 #{aftersale.caseNo} 已建立，請從售後頁查看退貨與退款進度。
							</p>
						</section>
					) : null}
				</div>
			) : null}
		</Drawer>
	);
}

/**
 * Status-driven primary action(s) inside the drawer footer. Complex flows
 * (付款重試) deep-link to the detail page where the retry sheet lives; light
 * flows act in place or link to the dedicated sub-route.
 */
function NextStepActions({
	order,
	aftersale,
	afterSaleLoading,
	busy,
	onConfirm,
}: {
	order: Order;
	aftersale?: AfterSaleCase;
	afterSaleLoading: boolean;
	busy: boolean;
	onConfirm: () => void;
}) {
	const base = `/account/orders/${order.orderNo}`;

	if (aftersale) {
		return (
			<Button href={`/account/after-sales/${aftersale.caseNo}`} color="primary">
				查看售後案件
			</Button>
		);
	}

	if (order.status === "pending_payment") {
		return (
			<Button href={base} color="primary">
				前往付款
			</Button>
		);
	}

	if (order.status === "delivered") {
		return (
			<div className="flex flex-wrap gap-2">
				{!order.countdownPausedAt && !afterSaleLoading ? (
					<Button
						href={`${base}/return`}
						variant="bordered"
						color="warning"
						className="flex-1"
						disabled={busy}
					>
						申請退換貨
					</Button>
				) : null}
				<Button
					color="primary"
					className="flex-1"
					disabled={busy}
					isLoading={busy}
					loadingLabel="確認收貨中…"
					onClick={onConfirm}
				>
					確認收貨
				</Button>
			</div>
		);
	}

	if (order.status === "shipping") {
		return (
			<Button href={`${base}/tracking`} color="primary">
				查看物流
			</Button>
		);
	}

	if (order.status === "completed" && canWriteOrderReview(order)) {
		return (
			<Button href={`${base}/review`} color="primary">
				評價商品
			</Button>
		);
	}

	return null;
}
