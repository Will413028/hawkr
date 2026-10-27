"use client";

import * as React from "react";
import { AnimateHeight } from "@/components/shared/AnimateHeight";
import AppImage from "@/components/shared/AppImage";
import { Drawer } from "@/components/shared/Drawer";
import { Input, Select, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { AfterSaleArbitrationCard } from "@/features/after-sales/components/AfterSaleArbitrationCard";
import { AfterSaleConfirmReceiptDialog } from "@/features/after-sales/components/AfterSaleConfirmReceiptDialog";
import { AfterSaleEvidenceGrid } from "@/features/after-sales/components/AfterSaleEvidenceGrid";
import { AfterSaleStatusBadge } from "@/features/after-sales/components/AfterSaleStatusBadge";
import OrderStatusTimeline from "@/features/orders/components/OrderStatusTimeline";
import { returnFailureGuidance } from "@/lib/after-sales/return-failure-guidance";
import { resolveAfterSale } from "@/lib/after-sales/seller-repo";
import {
	afterSaleCanConfirmReceipt,
	afterSaleCanResolve,
	afterSaleIsAppealResolved,
	afterSaleIsUnderAppeal,
	REFUND_TASK_STATUS_LABEL,
	returnLogisticsStatusLabel,
} from "@/lib/after-sales/status";
import type {
	AfterSaleCase,
	AfterSaleDetail,
	AfterSaleResolution,
} from "@/lib/after-sales/types";
import { AFTER_SALE_STATUS } from "@/lib/after-sales/types";
import type { RefundReasonValue } from "@/lib/codemeta/enums";
import {
	AFTER_SALE_REJECT_REASON_LABEL,
	AFTER_SALE_RESOLUTION_TYPE_LABEL,
	AfterSaleRejectReason,
	AfterSaleResolutionType,
	type AfterSaleResolutionTypeValue,
	REFUND_REASON_LABEL,
	ShippingMethod,
} from "@/lib/codemeta/enums";
import {
	useInvalidateSellerAfterSaleDetail,
	useSellerAfterSaleDetail,
} from "@/lib/hooks/useSellerAfterSaleDetail";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { fetchOrder } from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";

type DecisionType = "1" | "2" | "3" | "4" | "5";

const DECISIONS: Array<{
	type: DecisionType;
	title: string;
	subtitle: string;
}> = [
	{
		type: AfterSaleResolutionType.RETURN_REFUND,
		title: "同意退貨退款",
		subtitle: "請買家寄回商品，收到後退款",
	},
	{
		type: AfterSaleResolutionType.PARTIAL_REFUND,
		title: "部分退款",
		subtitle: "不需退貨，退回部分金額",
	},
	{
		type: AfterSaleResolutionType.FULL_REFUND_NO_RETURN,
		title: "全額退款（免退貨）",
		subtitle: "不需退貨，全額退款",
	},
	{
		type: AfterSaleResolutionType.COUPON_COMPENSATION,
		title: "發放補償優惠券",
		subtitle: "以優惠券補償買家",
	},
	{
		type: AfterSaleResolutionType.REJECT,
		title: "拒絕售後",
		subtitle: "說明原因，婉拒本次申請",
	},
];

function reasonLabel(raw: string): string {
	return REFUND_REASON_LABEL[raw as RefundReasonValue] ?? raw ?? "—";
}

function money(raw?: string): string {
	const n = Number(raw || 0);
	return Number.isNaN(n) ? "—" : `NT$ ${n.toLocaleString()}`;
}

function returnMethodLabel(raw?: string): string {
	switch (raw) {
		case ShippingMethod.TAQBIN:
			return "黑貓宅急便";
		case ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP:
			return "7-11 店到店";
		default:
			return raw || "—";
	}
}

function resolutionLabel(resolution?: AfterSaleResolution | null): string {
	if (!resolution) return "處理方式已送出";
	return (
		AFTER_SALE_RESOLUTION_TYPE_LABEL[
			resolution.resolutionType as AfterSaleResolutionTypeValue
		] ?? "處理方式已送出"
	);
}

function actionStateTitle(status: string): string {
	switch (status) {
		case AFTER_SALE_STATUS.RETURN_PENDING:
		case AFTER_SALE_STATUS.RETURN_SHIPPING:
		case AFTER_SALE_STATUS.RETURN_AT_STORE:
		case AFTER_SALE_STATUS.RETURN_DELIVERED:
		case AFTER_SALE_STATUS.RETURN_RECEIVED:
			return "已進入退貨流程";
		case AFTER_SALE_STATUS.RETURN_FAILED:
			return "退貨尚未完成";
		case AFTER_SALE_STATUS.REFUNDING:
			return "已進入退款流程";
		case AFTER_SALE_STATUS.CLOSED:
			return "售後已結案";
		case AFTER_SALE_STATUS.REJECTED:
			return "售後已拒絕";
		case AFTER_SALE_STATUS.CANCELLED:
			return "售後已取消";
		case AFTER_SALE_STATUS.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT:
			return "寄回逾時已關案";
		default:
			return "目前無需賣家處理";
	}
}

function latestResolutionOf(
	detail: AfterSaleDetail | null,
): AfterSaleResolution | null {
	return detail?.resolutions?.at(-1) ?? null;
}

function actionStateLead(status: string): string {
	switch (status) {
		case AFTER_SALE_STATUS.RETURN_FAILED:
			return "退貨尚未完成，先確認物流原因，再由客服協助安排同原物流退貨。";
		case AFTER_SALE_STATUS.REFUNDING:
			return "退款已送出，等待金流完成；若需買家填寫退款帳戶，流程會先暫停。";
		case AFTER_SALE_STATUS.CLOSED:
			return "此案件已完成，處理結果與退款資訊可於紀錄中查核。";
		case AFTER_SALE_STATUS.REJECTED:
			return "此案件已拒絕，後續以買家申訴結果為準。";
		case AFTER_SALE_STATUS.CANCELLED:
			return "買家已取消申請，這筆案件不需要再處理。";
		case AFTER_SALE_STATUS.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT:
			return "買家逾期未寄回，系統已解除退款資格並關閉案件。";
		default:
			return "";
	}
}

interface Props {
	open: boolean;
	/** The list-row case (instant header data before detail loads). */
	summary: AfterSaleCase | null;
	onClose: () => void;
	/** 抽屜內任一決策（同意 / 拒絕 / 部分退款 / 確認收貨）成功後即時通知父頁重新
	 * 同步售後清單 + 訂單資料，與 onClose 正交（#1247）。 */
	onMutated: () => void | Promise<void>;
}

export function AfterSaleDecisionDrawer({
	open,
	summary,
	onClose,
	onMutated,
}: Props) {
	const caseNo = summary?.caseNo ?? "";
	const orderNo = summary?.orderNo ?? "";

	// Case detail (+ arbitration once appeal-locked), keyed by caseNo and gated
	// on the drawer being open. The summary row supplies header data until it
	// resolves; confirm-receipt invalidates it instead of an imperative reload.
	const {
		data,
		loading,
		error: detailError,
	} = useSellerAfterSaleDetail(caseNo, { enabled: open });
	const detail = data?.detail ?? null;
	const arbitration = data?.arbitration ?? null;
	const invalidateDetail = useInvalidateSellerAfterSaleDetail();

	const [selected, setSelected] = React.useState<DecisionType | null>(null);
	const [confirmOpen, setConfirmOpen] = React.useState(false);
	const [busy, setBusy] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const [result, setResult] = React.useState<string | null>(null);

	// Decision form fields
	// Return logistics must match the original order shipping method. Sellers
	// should not choose a different return method here.
	const [originalReturnMethod, setOriginalReturnMethod] = React.useState<
		string | null
	>(null);
	const [refundAmount, setRefundAmount] = React.useState("");
	const [couponAmount, setCouponAmount] = React.useState("");
	const [couponMinSpend, setCouponMinSpend] = React.useState("");
	const [couponMessage, setCouponMessage] = React.useState("");
	const [rejectReason, setRejectReason] = React.useState<string>(
		AfterSaleRejectReason.INSUFFICIENT_EVIDENCE,
	);
	const [rejectMessage, setRejectMessage] = React.useState("");
	const [timelineOrder, setTimelineOrder] = React.useState<Order | null>(null);

	const refundable = Number(
		detail?.refundableAmount || summary?.refundableAmount || 0,
	);

	// Form/validation errors are local; detail-load errors come from the query.
	const displayError = error ?? detailError;

	// Reset the decision form each time the drawer opens for a case, and read
	// the original order logistics once. Return shipping must stay identical to
	// that original method, so this is read-only form state.
	React.useEffect(() => {
		if (!(open && caseNo)) return;
		let cancelled = false;
		setSelected(null);
		setResult(null);
		setError(null);
		setRefundAmount("");
		setCouponAmount("");
		setCouponMinSpend("");
		setCouponMessage("");
		setRejectMessage("");
		setOriginalReturnMethod(null);
		setTimelineOrder(null);
		if (orderNo) {
			void fetchOrder(orderNo, { view: "seller" })
				.then((order) => {
					if (cancelled) return;
					const m = order.shipping?.method;
					setOriginalReturnMethod(m || null);
					setTimelineOrder(order);
				})
				.catch(() => {
					if (!cancelled) {
						setTimelineOrder(null);
					}
				});
		}
		return () => {
			cancelled = true;
		};
	}, [open, caseNo, orderNo]);

	// Default the full-refund amount once we know the refundable ceiling.
	React.useEffect(() => {
		if (selected === AfterSaleResolutionType.FULL_REFUND_NO_RETURN) {
			setRefundAmount(String(refundable));
		}
	}, [selected, refundable]);

	const status = detail?.status ?? summary?.status ?? "";
	// Once the buyer escalates to a platform appeal, the case is locked by
	// arbitration — the seller can no longer resolve/confirm it unilaterally
	// (the backend rejects such calls). Gate all seller actions on this.
	const isUnderAppeal = afterSaleIsUnderAppeal(detail?.appealState);
	const isAppealResolved = afterSaleIsAppealResolved(detail?.appealState);
	const canResolve = afterSaleCanResolve(status, detail?.appealState);
	const canConfirmReceipt = afterSaleCanConfirmReceipt(
		status,
		detail?.appealState,
	);
	const latestResolution = latestResolutionOf(detail);
	const drawerTitle = canResolve || canConfirmReceipt ? "售後處理" : "售後詳情";
	const returnRefundBlocked =
		selected === AfterSaleResolutionType.RETURN_REFUND && !originalReturnMethod;
	const timelineEvents = React.useMemo(() => {
		const events: Partial<Record<string, number>> = {};
		if (!timelineOrder) return events;
		const paidStamp =
			timelineOrder.paidAt ?? timelineOrder.payment?.paidAt ?? null;
		if (timelineOrder.createdAt)
			events.pending_payment = timelineOrder.createdAt;
		if (paidStamp) events.paid = paidStamp;
		if (timelineOrder.shipping?.deliveredAt) {
			events.completed = timelineOrder.shipping.deliveredAt;
		}
		return events;
	}, [timelineOrder]);
	const timelineStatus =
		timelineOrder?.status ??
		(status === AFTER_SALE_STATUS.CLOSED ? "completed" : "delivered");
	const timelineShippingMethod =
		timelineOrder?.shipping?.method ?? originalReturnMethod;

	const submit = async () => {
		if (!selected) return;
		setError(null);

		const input: Parameters<typeof resolveAfterSale>[0] = {
			caseNo,
			resolutionType: selected,
		};
		if (selected === AfterSaleResolutionType.RETURN_REFUND) {
			if (!originalReturnMethod) {
				setError("尚未取得原訂單物流方式，請稍後再送出。");
				return;
			}
			input.returnShippingMethod = originalReturnMethod;
		} else if (selected === AfterSaleResolutionType.PARTIAL_REFUND) {
			const n = Number(refundAmount);
			if (Number.isNaN(n) || n <= 0 || n > refundable) {
				setError(`部分退款金額需介於 1 ～ ${refundable.toLocaleString()}`);
				return;
			}
			input.refundAmount = String(n);
		} else if (selected === AfterSaleResolutionType.FULL_REFUND_NO_RETURN) {
			input.refundAmount = String(refundable);
		} else if (selected === AfterSaleResolutionType.COUPON_COMPENSATION) {
			const amt = Number(couponAmount);
			const min = Number(couponMinSpend);
			if (Number.isNaN(amt) || amt <= 0) {
				setError("請填寫優惠券面額");
				return;
			}
			if (Number.isNaN(min) || min < amt) {
				setError("最低消費金額需大於或等於面額");
				return;
			}
			input.couponAmount = String(amt);
			input.couponMinSpend = String(min);
			if (couponMessage.trim()) input.couponMessage = couponMessage.trim();
		} else if (selected === AfterSaleResolutionType.REJECT) {
			input.rejectReason = rejectReason;
			if (rejectMessage.trim()) input.rejectMessage = rejectMessage.trim();
		}

		setBusy(true);
		try {
			const res = await resolveAfterSale(input);
			if (
				selected === AfterSaleResolutionType.RETURN_REFUND &&
				originalReturnMethod === ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP &&
				res.returnShipmentNo
			) {
				setResult(
					`已建立 7-11 退貨代碼：${res.returnShipmentNo}${
						res.validationNo ? `（驗證碼 ${res.validationNo}）` : ""
					}`,
				);
			} else if (selected === AfterSaleResolutionType.RETURN_REFUND) {
				setResult("已同意退貨，退貨物流已建立。");
			} else {
				setResult("已送出處理結果。");
			}
			await onMutated();
		} catch (e) {
			setError(e instanceof Error ? e.message : "送出失敗");
		} finally {
			setBusy(false);
		}
	};

	return (
		<>
			<Drawer
				open={open}
				onClose={onClose}
				onMutated={onMutated}
				side="right"
				title={drawerTitle}
			>
				{!summary ? null : (
					<div className="flex flex-col gap-4">
						{/* Header */}
						<div className="flex items-start justify-between gap-3">
							<div className="min-w-0">
								<p className="text-sm font-bold text-foreground break-all">
									{summary.caseNo}
								</p>
								<p className="text-xs text-foreground-soft mt-0.5 break-all">
									訂單 #{summary.orderNo}
								</p>
							</div>
							<AfterSaleStatusBadge status={status} />
						</div>
						{(detail?.processDeadline ?? summary.processDeadline) ? (
							<p className="text-xs text-warning-700 bg-warning-50 rounded-hkui-md px-3 py-2">
								處理期限：{detail?.processDeadline ?? summary.processDeadline}
							</p>
						) : null}
						{(detail?.storePickupDeadlineAt ?? summary.storePickupDeadlineAt) &&
						status === AFTER_SALE_STATUS.RETURN_AT_STORE ? (
							<p className="text-xs text-info-700 bg-info-50 rounded-hkui-md px-3 py-2">
								超商待取期限：
								{detail?.storePickupDeadlineAt ?? summary.storePickupDeadlineAt}
							</p>
						) : null}
						<OrderStatusTimeline
							status={timelineStatus}
							events={timelineEvents}
							aftersale={detail ?? summary}
							shippingMethod={timelineShippingMethod}
							variant="compact"
						/>

						{arbitration ? (
							<section>
								<h3 className="text-sm font-semibold text-foreground mb-2">
									平台申訴
								</h3>
								<AfterSaleArbitrationCard arbitration={arbitration} />
							</section>
						) : null}

						{/* Buyer application */}
						<section>
							<h3 className="text-sm font-semibold text-foreground mb-2">
								買家申請
							</h3>
							<dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-sm">
								<dt className="text-foreground-soft">買家</dt>
								<dd className="m-0 text-hk-body">{summary.buyerName || "—"}</dd>
								<dt className="text-foreground-soft">原因</dt>
								<dd className="m-0 text-hk-body">
									{reasonLabel(detail?.reason ?? summary.reason)}
								</dd>
								<dt className="text-foreground-soft">可退金額</dt>
								<dd className="m-0 text-hk-primary font-semibold">
									{money(detail?.refundableAmount ?? summary.refundableAmount)}
								</dd>
								<dt className="text-foreground-soft">描述</dt>
								<dd className="m-0 text-hk-body whitespace-pre-wrap">
									{detail?.description || summary.description || "—"}
								</dd>
							</dl>
							{detail?.photos && detail.photos.length > 0 ? (
								<div className="mt-2">
									<AfterSaleEvidenceGrid photos={detail.photos} />
								</div>
							) : null}
						</section>

						{/* Seller inspection evidence — `evidence[]` tagged
						    AfterSaleEvidenceType.INSPECTION_PHOTO ("8602"), uploaded by the
						    seller at confirm-receipt. The buyer detail page has always had
						    its own block; the seller had no render point at all, so the
						    photos backing an inspection deduction were invisible to the
						    person who took it (#430 A3-c). */}
						{detail?.sellerPhotos && detail.sellerPhotos.length > 0 ? (
							<section>
								<h3 className="text-sm font-semibold text-foreground mb-2">
									驗貨照片
								</h3>
								<AfterSaleEvidenceGrid photos={detail.sellerPhotos} />
							</section>
						) : null}

						{/* Order snapshot */}
						{detail?.items && detail.items.length > 0 ? (
							<section>
								<h3 className="text-sm font-semibold text-foreground mb-2">
									商品
								</h3>
								<ul className="list-none m-0 p-0 flex flex-col gap-2">
									{detail.items.map((it) => {
										const src = it.image ? resolveCdnUrl(it.image) : "";
										return (
											<li
												key={it.orderItemId}
												className="flex items-center gap-2"
											>
												{src ? (
													<AppImage
														src={src}
														alt={it.name}
														width={40}
														height={40}
														sizes="40px"
														className="w-10 h-10 rounded-hkui-md object-cover bg-default-100"
													/>
												) : (
													<span className="w-10 h-10 rounded-hkui-md bg-default-100" />
												)}
												<span className="flex-1 min-w-0 text-sm text-hk-body truncate">
													{it.name}
												</span>
												<span className="text-sm text-foreground-soft">
													× {it.quantity}
												</span>
											</li>
										);
									})}
								</ul>
							</section>
						) : null}

						{displayError ? (
							<p className="text-sm text-danger-700 font-semibold" role="alert">
								{displayError}
							</p>
						) : null}

						{result ? (
							<div className="rounded-hkui-md border border-success-200 bg-success-50 p-3 text-sm text-success-700">
								{result}
							</div>
						) : null}

						{/* Decision area */}
						{loading ? (
							<p className="text-sm text-foreground-soft">載入中…</p>
						) : result ? (
							<Button onClick={onClose}>完成</Button>
						) : isUnderAppeal ? (
							<section className="rounded-hkui-md border border-info-200 bg-info-50 p-3">
								<strong className="block text-sm text-info-700">
									此案件已進入平台申訴
								</strong>
								<p className="m-0 mt-1 text-xs text-info-700 leading-relaxed">
									買家已對處理結果提出申訴，案件由平台裁決，賣家暫時無法自行處理或退款。請等待平台裁決結果。
								</p>
							</section>
						) : isAppealResolved ? (
							<section className="rounded-hkui-md border border-info-200 bg-info-50 p-3">
								<strong className="block text-sm text-info-700">
									平台已完成裁決
								</strong>
								<p className="m-0 mt-1 text-xs text-info-700 leading-relaxed">
									本案件已由平台裁決，後續依裁決結果處理，裁決內容請見上方「平台申訴」紀錄。
								</p>
							</section>
						) : canResolve ? (
							<section>
								<h3 className="text-sm font-semibold text-foreground mb-2">
									選擇處理方式
								</h3>
								<div className="grid grid-cols-1 gap-2">
									{DECISIONS.map((d) => {
										const active = selected === d.type;
										return (
											<button
												key={d.type}
												type="button"
												onClick={() => setSelected(d.type)}
												className={`text-left rounded-hkui-md border p-3 transition-colors ${
													active
														? "border-hkui-primary bg-hkui-primary-50"
														: "border-divider bg-content1 hover:border-hkui-primary-200"
												}`}
											>
												<strong className="block text-sm text-foreground">
													{d.title}
												</strong>
												<span className="text-xs text-foreground-soft">
													{d.subtitle}
												</span>
											</button>
										);
									})}
								</div>

								{/* Per-decision fields — animated height so switching CTAs
								    glides instead of yanking the footer buttons around. */}
								<AnimateHeight className="mt-2">
									{selected === AfterSaleResolutionType.RETURN_REFUND ? (
										<section className="rounded-hkui-md border border-divider bg-content2 px-3 py-2">
											<h4 className="m-0 text-sm font-semibold text-foreground">
												退貨物流方式
											</h4>
											<p className="m-0 mt-1 text-sm font-semibold text-hk-body">
												{originalReturnMethod
													? returnMethodLabel(originalReturnMethod)
													: "讀取原訂單物流中…"}
												{originalReturnMethod ? (
													<span className="ml-1 text-xs font-medium text-foreground-soft">
														（同原訂單物流）
													</span>
												) : null}
											</p>
											<p className="m-0 mt-1 text-xs text-foreground-soft leading-relaxed">
												退貨方式會跟原本寄送方式相同，賣家不能改成其他物流。
											</p>
										</section>
									) : null}

									{selected === AfterSaleResolutionType.PARTIAL_REFUND ? (
										<Input
											type="number"
											label="退款金額"
											value={refundAmount}
											min={1}
											max={refundable}
											onChange={(e) => setRefundAmount(e.target.value)}
											description={`可退上限 ${money(String(refundable))}`}
										/>
									) : null}

									{selected ===
									AfterSaleResolutionType.FULL_REFUND_NO_RETURN ? (
										<p className="text-sm text-hk-body bg-content2 rounded-hkui-md px-3 py-2">
											將全額退款 {money(String(refundable))}
											。信用卡原路退回；ATM 付款需買家於退款頁填寫銀行帳號。
										</p>
									) : null}

									{selected === AfterSaleResolutionType.COUPON_COMPENSATION ? (
										<div className="flex flex-col gap-2">
											<Input
												type="number"
												label="優惠券面額"
												value={couponAmount}
												min={1}
												onChange={(e) => setCouponAmount(e.target.value)}
											/>
											<Input
												type="number"
												label="最低消費金額"
												value={couponMinSpend}
												min={0}
												onChange={(e) => setCouponMinSpend(e.target.value)}
											/>
											<Textarea
												label="給買家的訊息（選填）"
												variant="bordered"
												value={couponMessage}
												onChange={(e) =>
													setCouponMessage(e.target.value.slice(0, 100))
												}
												minRows={2}
												maxLength={100}
											/>
										</div>
									) : null}

									{selected === AfterSaleResolutionType.REJECT ? (
										<div className="flex flex-col gap-2">
											<Select
												label="拒絕原因"
												value={rejectReason}
												onChange={(e) => setRejectReason(e.target.value)}
											>
												{Object.entries(AFTER_SALE_REJECT_REASON_LABEL).map(
													([code, label]) => (
														<option key={code} value={code}>
															{label}
														</option>
													),
												)}
											</Select>
											<Textarea
												label="補充說明（選填）"
												variant="bordered"
												value={rejectMessage}
												onChange={(e) =>
													setRejectMessage(e.target.value.slice(0, 200))
												}
												minRows={2}
												maxLength={200}
											/>
										</div>
									) : null}
								</AnimateHeight>

								<div className="mt-4 flex gap-2 justify-end">
									<Button variant="bordered" color="default" onClick={onClose}>
										取消
									</Button>
									<Button
										color={
											selected === AfterSaleResolutionType.REJECT
												? "danger"
												: "primary"
										}
										disabled={busy || !selected || returnRefundBlocked}
										isLoading={busy}
										loadingLabel="送出售後處理中"
										onClick={submit}
									>
										送出處理
									</Button>
								</div>
							</section>
						) : canConfirmReceipt ? (
							<div className="flex flex-col gap-3">
								<AfterSaleActionStateCard
									status={status}
									detail={detail}
									resolution={latestResolution}
								/>
								<section className="rounded-hkui-md border border-warning-200 bg-warning-50 p-3">
									<strong className="block text-sm text-warning-800">
										等待你確認收貨
									</strong>
									<p className="m-0 mt-1 text-xs text-warning-800 leading-relaxed">
										買家已寄回商品，收到並驗貨後再送出退款確認。
									</p>
									<Button className="mt-3" onClick={() => setConfirmOpen(true)}>
										確認收貨
									</Button>
								</section>
							</div>
						) : (
							<AfterSaleActionStateCard
								status={status}
								detail={detail}
								resolution={latestResolution}
							/>
						)}
					</div>
				)}
			</Drawer>

			<AfterSaleConfirmReceiptDialog
				open={confirmOpen}
				caseNo={caseNo}
				refundableAmount={String(refundable)}
				onClose={() => setConfirmOpen(false)}
				onDone={async () => {
					setResult("已確認收貨並送出退款。");
					await onMutated();
					await invalidateDetail(caseNo);
				}}
			/>
		</>
	);
}

function AfterSaleActionStateCard({
	status,
	detail,
	resolution,
}: {
	status: string;
	detail: AfterSaleDetail | null;
	resolution: AfterSaleResolution | null;
}) {
	const refundTaskStatus = detail?.refundProgress?.refundTaskStatus;
	const refundStatusLabel = refundTaskStatus
		? (REFUND_TASK_STATUS_LABEL[refundTaskStatus] ?? refundTaskStatus)
		: "";
	const returnShipmentNo =
		detail?.returnLogistics?.shipmentNo || detail?.returnShipmentNo || "";
	const returnMethod =
		detail?.returnLogistics?.method || detail?.returnShippingMethod || "";
	const returnStatusLabel = returnLogisticsStatusLabel(
		status,
		detail?.returnLogistics,
	);
	const refundAmount = resolution?.refundAmount || detail?.refundAmount || "";
	const shouldShowReturn =
		resolution?.resolutionType === AfterSaleResolutionType.RETURN_REFUND ||
		status === AFTER_SALE_STATUS.RETURN_PENDING ||
		status === AFTER_SALE_STATUS.RETURN_SHIPPING ||
		status === AFTER_SALE_STATUS.RETURN_AT_STORE ||
		status === AFTER_SALE_STATUS.RETURN_DELIVERED ||
		status === AFTER_SALE_STATUS.RETURN_FAILED ||
		status === AFTER_SALE_STATUS.RETURN_RECEIVED;
	const resolutionValue = resolution ? resolutionLabel(resolution) : "";
	const lead = actionStateLead(status);
	const failureGuidance =
		status === AFTER_SALE_STATUS.RETURN_FAILED
			? returnFailureGuidance({
					method: returnMethod,
					providerMessage: detail?.returnProviderMessage,
					providerStatus: detail?.returnProviderStatus,
				})
			: null;

	const rows = [
		resolutionValue ? { label: "已選方式", value: resolutionValue } : null,
		shouldShowReturn && returnMethod
			? { label: "退貨方式", value: returnMethodLabel(returnMethod) }
			: null,
		shouldShowReturn && returnStatusLabel
			? { label: "退貨狀態", value: returnStatusLabel }
			: null,
		returnShipmentNo ? { label: "退貨單號", value: returnShipmentNo } : null,
		refundAmount ? { label: "退款金額", value: money(refundAmount) } : null,
		refundStatusLabel ? { label: "退款進度", value: refundStatusLabel } : null,
		detail?.refundProgress?.estimatedRefundDate
			? {
					label: "預估退款",
					value: detail.refundProgress.estimatedRefundDate,
				}
			: null,
	].filter((row): row is { label: string; value: string } =>
		Boolean(row?.value),
	);

	return (
		<section className="rounded-hkui-md border border-divider bg-content2 p-3">
			<strong className="block text-sm text-foreground">
				{actionStateTitle(status)}
			</strong>
			{lead ? (
				<p className="m-0 mt-1 text-xs text-foreground-soft leading-relaxed">
					{lead}
				</p>
			) : null}
			{rows.length > 0 ? (
				<dl className="mt-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1.5 border-t border-divider pt-3 text-sm">
					{rows.map((row) => (
						<React.Fragment key={row.label}>
							<dt className="text-foreground-soft">{row.label}</dt>
							<dd className="m-0 text-hk-body break-words">{row.value}</dd>
						</React.Fragment>
					))}
				</dl>
			) : null}
			{detail?.refundProgress?.needRefundPage ? (
				<p className="m-0 mt-3 rounded-hkui-md bg-warning-50 px-3 py-2 text-xs font-semibold text-warning-800">
					等待買家填寫退款帳戶後，退款才會繼續執行。
				</p>
			) : null}
			{failureGuidance ? (
				<div className="mt-3 border-t border-danger-200 pt-3 text-danger-700">
					<strong className="block text-sm">退貨異常處理</strong>
					{detail?.returnProviderMessage ? (
						<p className="m-0 mt-1 text-xs font-semibold leading-relaxed">
							物流回覆：{detail.returnProviderMessage}
						</p>
					) : null}
					<p className="m-0 mt-1 text-xs leading-relaxed text-danger-700/90">
						{failureGuidance.cause}
					</p>
					<ol className="m-0 mt-2 grid gap-1 pl-4 text-xs leading-relaxed">
						{failureGuidance.steps.map((step) => (
							<li key={step}>{step}</li>
						))}
					</ol>
					{failureGuidance.links.length > 0 ? (
						<div className="mt-3 flex flex-wrap gap-2">
							{failureGuidance.links.map((link) => (
								<a
									key={link.href}
									href={link.href}
									target="_blank"
									rel="noopener noreferrer"
									className="text-xs font-bold underline underline-offset-2 hover:no-underline"
								>
									{link.label}
								</a>
							))}
						</div>
					) : null}
				</div>
			) : null}
		</section>
	);
}

export default AfterSaleDecisionDrawer;
