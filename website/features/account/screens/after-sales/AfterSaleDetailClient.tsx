"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Select, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { AfterSaleArbitrationCard } from "@/features/after-sales/components/AfterSaleArbitrationCard";
import { AfterSaleEvidenceGrid } from "@/features/after-sales/components/AfterSaleEvidenceGrid";
import type { StagedPhoto } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { AfterSalePhotoUploader } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { AfterSaleProgress } from "@/features/after-sales/components/AfterSaleProgress";
import { AfterSaleRefundBanner } from "@/features/after-sales/components/AfterSaleRefundBanner";
import { AfterSaleStatusBadge } from "@/features/after-sales/components/AfterSaleStatusBadge";
import {
	appealAfterSale,
	cancelAfterSale,
	supplementAfterSale,
} from "@/lib/after-sales/buyer-repo";
import {
	afterSaleCanAppeal,
	afterSaleCanCancel,
	afterSaleIsAppealLocked,
	afterSaleIsAppealResolved,
	afterSaleIsUnderAppeal,
	afterSaleNarrative,
	REFUND_TASK_STATUS_LABEL,
	returnLogisticsStatusLabel,
} from "@/lib/after-sales/status";
import type {
	AfterSaleDetail,
	AfterSaleResolution,
} from "@/lib/after-sales/types";
import { AFTER_SALE_STATUS } from "@/lib/after-sales/types";
import type {
	AfterSaleRejectReasonValue,
	AfterSaleResolutionTypeValue,
	RefundReasonValue,
	ShippingMethodValue,
} from "@/lib/codemeta/enums";
import {
	AFTER_SALE_REJECT_REASON_LABEL,
	AFTER_SALE_RESOLUTION_TYPE_LABEL,
	REFUND_REASON_LABEL,
	SHIPPING_METHOD_LABEL,
} from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useInvalidateMyAfterSaleDetail,
	useMyAfterSaleDetail,
} from "@/lib/hooks/useMyAfterSaleDetail";
import { resolveCdnUrl } from "@/lib/media/cdn";
import ShipBackDeadlineCountdown from "./ShipBackDeadlineCountdown";

function reasonLabel(reason: string): string {
	return REFUND_REASON_LABEL[reason as RefundReasonValue] ?? reason ?? "—";
}

function formatDateTime(raw?: string): string {
	if (!raw) return "";
	// 解不出來就把原字串照登（沿用本畫面原本的行為）。
	return formatTaipeiDateTime(raw) || raw;
}

const Card = ({
	title,
	children,
}: {
	title: string;
	children: React.ReactNode;
}) => (
	<section className="bg-content1 border border-divider rounded-hkui-lg p-5">
		<h2 className="text-base font-semibold mb-3">{title}</h2>
		{children}
	</section>
);

export default function AfterSaleDetailClient({ caseNo }: { caseNo: string }) {
	// Case detail (+ arbitration once appeal-locked) is keyed by caseNo; buyer
	// actions invalidate it instead of an imperative reload.
	const { data, loading, error } = useMyAfterSaleDetail(caseNo);
	const detail = data?.detail ?? null;
	const arbitration = data?.arbitration ?? null;
	const invalidateDetail = useInvalidateMyAfterSaleDetail();

	const [actionError, setActionError] = React.useState<string | null>(null);
	const [pendingAction, setPendingAction] = React.useState<
		"cancel" | "supplement" | "appeal" | null
	>(null);
	const busy = pendingAction !== null;
	const [noteMode, setNoteMode] = React.useState<"supplement" | null>(null);
	const [appealOpen, setAppealOpen] = React.useState(false);

	const handleCancel = async () => {
		if (!window.confirm("確定要取消這筆售後申請嗎？取消後需重新申請。")) return;
		setPendingAction("cancel");
		setActionError(null);
		try {
			await cancelAfterSale(caseNo);
			await invalidateDetail(caseNo);
		} catch (e) {
			setActionError(e instanceof Error ? e.message : "取消失敗");
		} finally {
			setPendingAction(null);
		}
	};

	const submitNote = async (description: string, photoTokens: string[]) => {
		setPendingAction("supplement");
		setActionError(null);
		try {
			await supplementAfterSale(caseNo, {
				additionalDescription: description,
				photoTokens,
			});
			setNoteMode(null);
			await invalidateDetail(caseNo);
		} catch (e) {
			setActionError(e instanceof Error ? e.message : "送出失敗");
		} finally {
			setPendingAction(null);
		}
	};

	const submitAppeal = async (
		reason: string,
		description: string,
		evidencePhotoTokens: string[],
	) => {
		setPendingAction("appeal");
		setActionError(null);
		try {
			await appealAfterSale(caseNo, {
				reason,
				description,
				evidencePhotoTokens,
			});
			setAppealOpen(false);
			await invalidateDetail(caseNo);
		} catch (e) {
			setActionError(e instanceof Error ? e.message : "申訴送出失敗");
		} finally {
			setPendingAction(null);
		}
	};

	if (loading && !detail) {
		return (
			<main>
				<SellerPageShell title="售後案件">
					<p className="text-foreground-soft text-sm">載入中…</p>
				</SellerPageShell>
			</main>
		);
	}

	// `!detail` (not `error || !detail`): keep the last-good case visible if a
	// background refetch fails — only fall to not-found when there's no data.
	if (!detail) {
		return (
			<main>
				<SellerPageShell
					title="找不到售後案件"
					actions={
						<Button
							href="/account/after-sales"
							variant="bordered"
							color="default"
							size="sm"
						>
							← 回售後列表
						</Button>
					}
				>
					<p className="text-danger-700 text-sm font-semibold">
						{error || "找不到售後案件"}
					</p>
				</SellerPageShell>
			</main>
		);
	}

	const status = detail.status;
	// Once an appeal exists the case is locked by platform arbitration — hide
	// buyer actions so the case can't be pushed back to a seller-actionable
	// state mid-arbitration. After a rejection, appeal is the only escalation.
	const isUnderAppeal = afterSaleIsUnderAppeal(detail.appealState);
	const isAppealLocked = afterSaleIsAppealLocked(detail.appealState);
	const isAppealResolved = afterSaleIsAppealResolved(detail.appealState);
	const canCancel = afterSaleCanCancel(status, detail.appealState);
	const isRejected = status === AFTER_SALE_STATUS.REJECTED;
	const isReturnFailed = status === AFTER_SALE_STATUS.RETURN_FAILED;
	const isShipBackTimeout =
		status === AFTER_SALE_STATUS.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT;
	const canAppeal = afterSaleCanAppeal(detail.canAppeal, detail.appealState);

	// Ship-back countdown is only meaningful while the buyer still has time to
	// send the parcel back (status=20 RETURN_PENDING + a backend-issued deadline).
	// Surface the D-1 warning banner inside the final 24h window only — outside
	// of that, the inline countdown on the progress narrative is enough.
	const shipBackDeadlineMs = (() => {
		if (status !== AFTER_SALE_STATUS.RETURN_PENDING) return null;
		const raw = detail.shipBackDeadlineAt;
		if (!raw) return null;
		const t = new Date(raw).getTime();
		return Number.isNaN(t) ? null : t;
	})();
	const shipBackShowD1Banner =
		shipBackDeadlineMs !== null &&
		shipBackDeadlineMs - Date.now() < 24 * 60 * 60 * 1000;

	return (
		<main>
			<SellerPageShell
				title="售後案件"
				subtitle={
					<>
						#{detail.caseNo} ·{" "}
						<span className="align-middle">
							<AfterSaleStatusBadge status={status} size="sm" />
						</span>
						{detail.orderNo ? `　訂單 #${detail.orderNo}` : ""}
					</>
				}
				actions={
					<Button
						href="/account/after-sales"
						variant="bordered"
						color="default"
						size="sm"
					>
						← 回售後列表
					</Button>
				}
			>
				<div className="flex flex-col gap-4 min-w-0">
					{/* 1 · Progress + narrative */}
					<section className="bg-content1 border border-divider rounded-hkui-lg p-5">
						<AfterSaleProgress
							status={status}
							size="expanded"
							className="mb-4"
						/>
						<p className="text-sm text-hk-body bg-content2 rounded-hkui-md px-3 py-2">
							{afterSaleNarrative(status)}
						</p>
					</section>

					{/* 2 · Next action banners */}
					<AfterSaleRefundBanner
						orderNo={detail.orderNo}
						progress={detail.refundProgress}
					/>

					{shipBackDeadlineMs !== null && !shipBackShowD1Banner ? (
						<section
							className="rounded-hkui-lg border border-info-50 bg-info-50 p-4 text-info-700"
							role="status"
						>
							<strong className="block text-base">寄回退貨倒數</strong>
							<p className="m-0 mt-1 text-sm leading-relaxed">
								<ShipBackDeadlineCountdown deadline={shipBackDeadlineMs} />
							</p>
						</section>
					) : null}

					{shipBackShowD1Banner ? (
						<section
							className="rounded-hkui-lg border border-warning-200 bg-warning-50 p-4 text-warning-700"
							role="alert"
						>
							<strong className="block text-base">
								<ShipBackDeadlineCountdown deadline={shipBackDeadlineMs} />
							</strong>
							<p className="m-0 mt-1 text-sm leading-relaxed">
								請於期限內寄回退貨商品，逾時將解除退款資格並關閉售後案件。
							</p>
						</section>
					) : null}

					{isShipBackTimeout && !isAppealLocked ? (
						<section
							className="rounded-hkui-lg border border-default-200 bg-default-100 p-4 text-foreground-soft"
							role="status"
						>
							<strong className="block text-base">案件已逾時關閉</strong>
							<p className="m-0 mt-1 mb-3 text-sm leading-relaxed">
								逾時未寄回退貨商品，退款資格已解除；若有特殊情況可於申訴期內提出申訴。
							</p>
							{canAppeal ? (
								<Button
									color="warning"
									disabled={busy}
									onClick={() => setAppealOpen(true)}
								>
									提出申訴
								</Button>
							) : null}
						</section>
					) : null}

					{isReturnFailed ? (
						<section className="rounded-hkui-lg border border-danger-200 bg-danger-50 p-4 text-danger-700">
							<strong className="block text-base">退貨物流失敗</strong>
							<p className="m-0 mt-1 text-sm leading-relaxed">
								{detail.returnProviderMessage ||
									"退貨物流建立失敗，請聯絡客服協助重新安排退貨。"}
							</p>
						</section>
					) : null}

					{isRejected && !isAppealLocked ? (
						<section className="rounded-hkui-lg border border-danger-200 bg-danger-50 p-4 text-danger-700">
							<strong className="block text-base">賣家已拒絕本次售後</strong>
							<p className="m-0 mt-1 mb-3 text-sm leading-relaxed">
								{canAppeal
									? "若你仍有疑慮，可向平台提出一次申訴，由客服裁決。"
									: "若你仍有疑慮，請聯絡平台客服協助。"}
							</p>
							{canAppeal ? (
								<div className="flex gap-2 flex-wrap">
									<Button
										color="danger"
										disabled={busy}
										onClick={() => setAppealOpen(true)}
									>
										提出申訴
									</Button>
								</div>
							) : null}
						</section>
					) : null}

					{isUnderAppeal ? (
						<section className="rounded-hkui-lg border border-info-200 bg-info-50 p-4 text-info-700">
							<strong className="block text-base">已提出平台申訴</strong>
							<p className="m-0 mt-1 text-sm leading-relaxed">
								你已針對處理結果提出申訴，案件由平台客服裁決中，此期間無法重複申訴。請等待平台裁決結果。
							</p>
						</section>
					) : null}

					{isAppealResolved ? (
						<section className="rounded-hkui-lg border border-info-200 bg-info-50 p-4 text-info-700">
							<strong className="block text-base">平台已完成裁決</strong>
							<p className="m-0 mt-1 text-sm leading-relaxed">
								你的申訴已由平台裁決完成，後續依裁決結果處理，裁決內容請見下方申訴紀錄。
							</p>
						</section>
					) : null}

					{canAppeal && !isRejected ? (
						<section className="rounded-hkui-lg border border-warning-200 bg-warning-50 p-4 text-warning-700">
							<strong className="block text-base">對處理結果不滿意？</strong>
							<p className="m-0 mt-1 mb-3 text-sm leading-relaxed">
								你可以針對賣家的處理結果，向平台提出一次申訴，由客服協助裁決
								{detail.appealDeadline
									? `（申訴期限至 ${formatDateTime(detail.appealDeadline)}）`
									: ""}
								。
							</p>
							<Button
								color="warning"
								disabled={busy}
								onClick={() => setAppealOpen(true)}
							>
								提出申訴
							</Button>
						</section>
					) : null}

					{arbitration ? (
						<Card title="平台申訴">
							<AfterSaleArbitrationCard arbitration={arbitration} />
						</Card>
					) : null}

					{actionError ? (
						<p className="text-danger-700 text-sm font-semibold" role="alert">
							{actionError}
						</p>
					) : null}

					{/* 3 · Application summary */}
					<Card title="申請內容">
						<dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-sm">
							<dt className="text-hk-muted">申請原因</dt>
							<dd className="m-0 text-hk-body">{reasonLabel(detail.reason)}</dd>
							{detail.createdAt ? (
								<>
									<dt className="text-hk-muted">申請時間</dt>
									<dd className="m-0 text-hk-body">
										{formatDateTime(detail.createdAt)}
									</dd>
								</>
							) : null}
							<dt className="text-hk-muted">問題描述</dt>
							<dd className="m-0 text-hk-body whitespace-pre-wrap">
								{detail.description || "—"}
							</dd>
						</dl>
						{detail.photos.length > 0 ? (
							<div className="mt-3">
								<p className="text-xs text-hk-muted mb-1.5">申請照片</p>
								<AfterSaleEvidenceGrid photos={detail.photos} />
							</div>
						) : null}
					</Card>

					{/* Order items snapshot */}
					{detail.items.length > 0 ? (
						<Card title="商品">
							<ul className="list-none m-0 p-0 flex flex-col gap-2">
								{detail.items.map((it) => {
									const src = it.image ? resolveCdnUrl(it.image) : "";
									return (
										<li
											key={it.orderItemId}
											className="flex items-center gap-3"
										>
											{src ? (
												<AppImage
													src={src}
													alt={it.name}
													width={48}
													height={48}
													sizes="48px"
													className="w-12 h-12 rounded-hkui-md object-cover bg-default-100"
												/>
											) : (
												<span className="w-12 h-12 rounded-hkui-md bg-default-100" />
											)}
											<div className="flex-1 min-w-0">
												<strong className="text-sm text-foreground truncate block">
													{it.name}
												</strong>
												{it.skuName ? (
													<span className="text-xs text-foreground-soft">
														{it.skuName}
													</span>
												) : null}
											</div>
											<span className="text-sm text-hk-body">
												× {it.quantity}
											</span>
										</li>
									);
								})}
							</ul>
						</Card>
					) : null}

					{/* 4 · Return logistics */}
					{detail.returnLogistics ? (
						<Card title="退貨資訊">
							<ReturnLogistics
								logistics={detail.returnLogistics}
								status={detail.status}
								providerMessage={detail.returnProviderMessage}
							/>
						</Card>
					) : null}

					{/* 5 · Refund progress detail */}
					{detail.refundProgress &&
					(detail.refundProgress.refundTaskStatus ||
						detail.refundProgress.estimatedRefundDate ||
						detail.refundProgress.bankDate) ? (
						<Card title="退款進度">
							<dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-sm">
								{detail.refundProgress.refundTaskStatus ? (
									<>
										<dt className="text-hk-muted">退款狀態</dt>
										<dd className="m-0 text-hk-body">
											{REFUND_TASK_STATUS_LABEL[
												detail.refundProgress.refundTaskStatus
											] ?? detail.refundProgress.refundTaskStatus}
										</dd>
									</>
								) : null}
								{detail.refundProgress.estimatedRefundDate ? (
									<>
										<dt className="text-hk-muted">預計退款日</dt>
										<dd className="m-0 text-hk-body">
											{detail.refundProgress.estimatedRefundDate}
										</dd>
									</>
								) : null}
								{detail.refundProgress.bankDate ? (
									<>
										<dt className="text-hk-muted">撥款日期</dt>
										<dd className="m-0 text-hk-body">
											{detail.refundProgress.bankDate}
										</dd>
									</>
								) : null}
							</dl>
						</Card>
					) : null}

					{/* Seller inspection evidence */}
					{detail.sellerPhotos.length > 0 ? (
						<Card title="賣家驗貨照片">
							<AfterSaleEvidenceGrid photos={detail.sellerPhotos} />
						</Card>
					) : null}

					{/* 6 · Resolution history */}
					{detail.resolutions.length > 0 ? (
						<Card title="處理紀錄">
							<ul className="list-none m-0 p-0 flex flex-col gap-3">
								{detail.resolutions.map((r, i) => (
									<ResolutionRow
										key={`${r.resolutionType}-${r.createdAt ?? i}`}
										resolution={r}
									/>
								))}
							</ul>
						</Card>
					) : null}

					{/* Footer actions */}
					{canCancel ? (
						<section className="flex gap-2 flex-wrap justify-end">
							<Button
								variant="bordered"
								color="danger"
								disabled={busy}
								isLoading={pendingAction === "cancel"}
								loadingLabel="取消申請中"
								onClick={handleCancel}
							>
								取消申請
							</Button>
						</section>
					) : null}
				</div>
			</SellerPageShell>

			<NoteDialog
				open={noteMode !== null}
				busy={busy}
				isLoading={pendingAction === "supplement"}
				onClose={() => setNoteMode(null)}
				onSubmit={submitNote}
			/>

			<AppealDialog
				open={appealOpen}
				busy={busy}
				isLoading={pendingAction === "appeal"}
				onClose={() => setAppealOpen(false)}
				onSubmit={submitAppeal}
			/>
		</main>
	);
}

function ReturnLogistics({
	logistics,
	status,
	providerMessage,
}: {
	logistics: NonNullable<AfterSaleDetail["returnLogistics"]>;
	status: string;
	providerMessage?: string;
}) {
	const methodLabel = logistics.method
		? (SHIPPING_METHOD_LABEL[logistics.method as ShippingMethodValue] ??
			logistics.method)
		: undefined;

	// 7-11 store-to-store shows three segment codes.
	const isCvs = logistics.partnerId || logistics.odno || logistics.validationNo;
	const logisticsStatus = returnLogisticsStatusLabel(status, logistics);

	return (
		<dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-sm">
			{methodLabel ? (
				<>
					<dt className="text-hk-muted">退貨方式</dt>
					<dd className="m-0 text-hk-body">{methodLabel}</dd>
				</>
			) : null}
			{isCvs ? (
				<>
					{logistics.partnerId ? (
						<>
							<dt className="text-hk-muted">寄件代碼</dt>
							<dd className="m-0 text-hk-body font-mono break-all">
								{logistics.partnerId}
							</dd>
						</>
					) : null}
					{logistics.odno ? (
						<>
							<dt className="text-hk-muted">訂單編號</dt>
							<dd className="m-0 text-hk-body font-mono break-all">
								{logistics.odno}
							</dd>
						</>
					) : null}
					{logistics.validationNo ? (
						<>
							<dt className="text-hk-muted">驗證碼</dt>
							<dd className="m-0 text-hk-body font-mono break-all">
								{logistics.validationNo}
							</dd>
						</>
					) : null}
				</>
			) : null}
			{logistics.shipmentNo ? (
				<>
					<dt className="text-hk-muted">退貨單號</dt>
					<dd className="m-0 text-hk-body break-all">{logistics.shipmentNo}</dd>
				</>
			) : null}
			{logisticsStatus ? (
				<>
					<dt className="text-hk-muted">物流狀態</dt>
					<dd className="m-0 text-hk-body">{logisticsStatus}</dd>
				</>
			) : null}
			{providerMessage && providerMessage !== logisticsStatus ? (
				<>
					<dt className="text-hk-muted">物流回覆</dt>
					<dd className="m-0 text-hk-body">{providerMessage}</dd>
				</>
			) : null}
		</dl>
	);
}

function ResolutionRow({ resolution }: { resolution: AfterSaleResolution }) {
	const typeLabel =
		AFTER_SALE_RESOLUTION_TYPE_LABEL[
			resolution.resolutionType as AfterSaleResolutionTypeValue
		] ?? "賣家處理";
	const rejectLabel = resolution.rejectReason
		? AFTER_SALE_REJECT_REASON_LABEL[
				resolution.rejectReason as AfterSaleRejectReasonValue
			]
		: undefined;
	return (
		<li className="border border-divider rounded-hkui-md p-3">
			<div className="flex items-center justify-between gap-2">
				<strong className="text-sm text-foreground">{typeLabel}</strong>
				{resolution.createdAt ? (
					<span className="text-xs text-foreground-soft">
						{formatDateTime(resolution.createdAt)}
					</span>
				) : null}
			</div>
			<dl className="mt-1.5 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-xs">
				{resolution.refundAmount ? (
					<>
						<dt className="text-foreground-soft">退款金額</dt>
						<dd className="m-0 text-hk-body">
							NT$ {Number(resolution.refundAmount).toLocaleString()}
						</dd>
					</>
				) : null}
				{resolution.deductionReason ? (
					<>
						<dt className="text-foreground-soft">扣款原因</dt>
						<dd className="m-0 text-hk-body">{resolution.deductionReason}</dd>
					</>
				) : null}
				{resolution.couponAmount ? (
					<>
						<dt className="text-foreground-soft">補償優惠券</dt>
						<dd className="m-0 text-hk-body">
							NT$ {Number(resolution.couponAmount).toLocaleString()}
							{resolution.couponMinSpend
								? `（滿 NT$ ${Number(resolution.couponMinSpend).toLocaleString()} 可用）`
								: ""}
						</dd>
					</>
				) : null}
				{rejectLabel ? (
					<>
						<dt className="text-foreground-soft">拒絕原因</dt>
						<dd className="m-0 text-hk-body">{rejectLabel}</dd>
					</>
				) : null}
				{resolution.rejectMessage ? (
					<>
						<dt className="text-foreground-soft">賣家說明</dt>
						<dd className="m-0 text-hk-body whitespace-pre-wrap">
							{resolution.rejectMessage}
						</dd>
					</>
				) : null}
				{resolution.inspectionNote ? (
					<>
						<dt className="text-foreground-soft">驗貨備註</dt>
						<dd className="m-0 text-hk-body whitespace-pre-wrap">
							{resolution.inspectionNote}
						</dd>
					</>
				) : null}
			</dl>
		</li>
	);
}

function NoteDialog({
	open,
	busy,
	isLoading,
	onClose,
	onSubmit,
}: {
	open: boolean;
	busy: boolean;
	isLoading: boolean;
	onClose: () => void;
	onSubmit: (description: string, photoTokens: string[]) => void;
}) {
	const [text, setText] = React.useState("");
	const [photos, setPhotos] = React.useState<StagedPhoto[]>([]);

	React.useEffect(() => {
		if (open) {
			setText("");
			setPhotos([]);
		}
	}, [open]);

	return (
		<Dialog
			open={open}
			onClose={onClose}
			title="補充說明"
			size="md"
			footer={
				<>
					<Button variant="bordered" color="default" onClick={onClose}>
						取消
					</Button>
					<Button
						disabled={busy || text.trim().length === 0}
						isLoading={isLoading}
						loadingLabel="送出說明中"
						onClick={() =>
							onSubmit(
								text.trim(),
								photos.map((p) => p.token),
							)
						}
					>
						送出
					</Button>
				</>
			}
		>
			<span className="block text-sm font-semibold text-foreground mb-1.5">
				說明內容
			</span>
			<Textarea
				variant="bordered"
				size="lg"
				value={text}
				onChange={(e) => setText(e.target.value.slice(0, 300))}
				placeholder="補充說明或提供更多照片。"
				minRows={3}
				maxLength={300}
			/>
			<div className="mt-3">
				<AfterSalePhotoUploader
					value={photos}
					onChange={setPhotos}
					max={3}
					disabled={busy}
				/>
			</div>
		</Dialog>
	);
}

const APPEAL_REASONS = [
	"賣家判定不合理",
	"退款金額有爭議",
	"未收到應退款項",
	"其他",
];

function AppealDialog({
	open,
	busy,
	isLoading,
	onClose,
	onSubmit,
}: {
	open: boolean;
	busy: boolean;
	isLoading: boolean;
	onClose: () => void;
	onSubmit: (
		reason: string,
		description: string,
		evidencePhotoTokens: string[],
	) => void;
}) {
	const [reason, setReason] = React.useState(APPEAL_REASONS[0]);
	const [text, setText] = React.useState("");
	const [photos, setPhotos] = React.useState<StagedPhoto[]>([]);

	React.useEffect(() => {
		if (open) {
			setReason(APPEAL_REASONS[0]);
			setText("");
			setPhotos([]);
		}
	}, [open]);

	return (
		<Dialog
			open={open}
			onClose={onClose}
			title="向平台提出申訴"
			description="每筆售後僅能申訴一次，平台將依雙方證據裁決。"
			size="md"
			footer={
				<>
					<Button variant="bordered" color="default" onClick={onClose}>
						取消
					</Button>
					<Button
						color="warning"
						disabled={busy || text.trim().length === 0}
						isLoading={isLoading}
						loadingLabel="送出申訴中"
						onClick={() =>
							onSubmit(
								reason,
								text.trim(),
								photos.map((p) => p.token),
							)
						}
					>
						送出申訴
					</Button>
				</>
			}
		>
			<Select
				label="申訴理由"
				value={reason}
				onChange={(e) => setReason(e.target.value)}
			>
				{APPEAL_REASONS.map((r) => (
					<option key={r} value={r}>
						{r}
					</option>
				))}
			</Select>
			<div className="mt-3">
				<span className="block text-sm font-semibold text-foreground mb-1.5">
					詳細說明 <span className="text-danger-700">*</span>
				</span>
				<Textarea
					variant="bordered"
					size="lg"
					value={text}
					onChange={(e) => setText(e.target.value.slice(0, 300))}
					placeholder="請具體說明申訴原因與期望的處理結果，並附上佐證。"
					minRows={3}
					maxLength={300}
				/>
			</div>
			<div className="mt-3">
				<AfterSalePhotoUploader
					value={photos}
					onChange={setPhotos}
					max={5}
					disabled={busy}
					label="佐證照片（最多 5 張）"
				/>
			</div>
		</Dialog>
	);
}
