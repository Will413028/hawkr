"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import AccountEmptyIcon from "@/features/account/components/AccountEmptyIcon";
import type { StagedPhoto } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { AfterSalePhotoUploader } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { createAfterSale } from "@/lib/after-sales/buyer-repo";
import type { RefundReasonValue } from "@/lib/codemeta/enums";
import { REFUND_REASON_LABEL, RefundReason } from "@/lib/codemeta/enums";
import { useOrder } from "@/lib/hooks/useOrders";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

/** codemeta RefundReason — the after-sale create endpoint takes a reason code,
 * not free text. Subtitles clarify what each code covers. */
const REASON_OPTIONS: Array<{
	value: RefundReasonValue;
	subtitle: string;
}> = [
	{ value: RefundReason.DEFECTIVE, subtitle: "商品損壞、腐壞或功能異常" },
	{ value: RefundReason.WRONG_ITEM, subtitle: "收到的商品與下單品項不同" },
	{
		value: RefundReason.NOT_AS_DESCRIBED,
		subtitle: "規格、數量或內容與賣場描述不符",
	},
	{ value: RefundReason.CHANGE_OF_MIND, subtitle: "不需要了或改變心意" },
	{ value: RefundReason.OTHER, subtitle: "其他原因，請於下方描述" },
];

const DESCRIPTION_MAX = 300;
const DESCRIPTION_MIN = 5;

export default function ReturnClient({ orderNo }: { orderNo: string }) {
	const router = useObservedRouter("order_return");
	const { order, loading } = useOrder(orderNo);

	const [reason, setReason] = React.useState<RefundReasonValue>(
		RefundReason.DEFECTIVE,
	);
	const [description, setDescription] = React.useState("");
	const [photos, setPhotos] = React.useState<StagedPhoto[]>([]);
	const [submitting, setSubmitting] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const [caseNo, setCaseNo] = React.useState<string | null>(null);

	if (loading && !order) {
		return (
			<main className="returns-page">
				<p>載入中…</p>
			</main>
		);
	}

	if (!order) {
		return (
			<main className="returns-page">
				<SellerPageShell title="申請退換貨">
					<p className="text-danger-700 text-sm font-semibold">找不到訂單</p>
				</SellerPageShell>
			</main>
		);
	}

	const trimmed = description.trim();
	const canSubmit = trimmed.length >= DESCRIPTION_MIN && !submitting;
	const descriptionTooShort =
		trimmed.length > 0 && trimmed.length < DESCRIPTION_MIN;
	const descriptionHelp = descriptionTooShort
		? `還需要 ${DESCRIPTION_MIN - trimmed.length} 個字才能送出申請。`
		: `請至少輸入 ${DESCRIPTION_MIN} 個字，讓賣家能判斷問題與處理方式。`;

	const handleSubmit = async () => {
		if (trimmed.length < DESCRIPTION_MIN) {
			setError(`詳細描述至少需要 ${DESCRIPTION_MIN} 個字，請再補充問題狀況。`);
			return;
		}
		setSubmitting(true);
		setError(null);
		try {
			const result = await createAfterSale({
				orderNo,
				reason,
				description: trimmed,
				photoTokens: photos.map((p) => p.token),
			});
			setCaseNo(result.caseNo || `AS_${orderNo}`);
		} catch (e) {
			setError(e instanceof Error ? e.message : "送出失敗，請稍後再試");
		} finally {
			setSubmitting(false);
		}
	};

	if (caseNo) {
		return (
			<main className="returns-page">
				<SellerPageShell title="申請退換貨" subtitle={`訂單 #${orderNo}`}>
					<div className="relative w-full bg-content1/90 border border-divider rounded-3xl p-10 px-7 pb-8 text-center flex flex-col items-center gap-3 shadow-[0_1px_0_rgba(255,255,255,0.6)_inset,0_18px_40px_-22px_rgb(var(--color-ink-700-rgb)/0.28),0_6px_18px_-12px_rgb(var(--color-ink-950-rgb)/0.18)] backdrop-blur-md overflow-hidden dark:bg-content1/86 animate-checkout-pop before:absolute before:inset-x-0 before:top-0 before:h-[140px] before:pointer-events-none before:z-0 before:bg-gradient-to-b before:from-success-50 before:to-transparent">
						<span
							className="relative z-1 inline-flex items-center justify-center w-[148px] h-[148px] -mt-1.5 mb-[-4px] animate-checkout-icon-pop"
							aria-hidden="true"
						>
							<AccountEmptyIcon variant="return" size={148} />
						</span>
						<h1 className="relative z-1 font-display text-xl md:text-2xl m-0 text-foreground">
							售後申請已送出
						</h1>
						<p className="relative z-1 m-0 text-foreground-soft text-sm max-w-[38ch]">
							賣家會在 24-72
							小時內審核。審核結果將以通知告知，可在售後案件頁追蹤進度。
						</p>
						<p className="relative z-1 w-full max-w-[360px] mt-3 p-4 rounded-2xl bg-content2 border border-divider text-left text-sm text-foreground flex justify-between items-center">
							<span>售後案件編號：</span>
							<strong>{caseNo}</strong>
						</p>
						<div className="relative z-1 flex gap-2 flex-wrap justify-center mt-4 w-full">
							<Button
								onClick={() => router.push(`/account/after-sales/${caseNo}`)}
							>
								查看售後進度
							</Button>
							<Button
								variant="bordered"
								color="default"
								onClick={() => router.push(`/account/orders/${orderNo}`)}
							>
								回訂單詳情
							</Button>
						</div>
					</div>
				</SellerPageShell>
			</main>
		);
	}

	return (
		<main className="returns-page">
			<SellerPageShell
				title="申請退換貨"
				subtitle={`訂單 #${orderNo}`}
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
				{/* 1 · 訂單摘要 — 售後以整張訂單為案件，不再逐項勾選 */}
				<section className="bg-content1 border border-divider p-[18px] mb-4 shadow-hk-card">
					<header className="flex items-center justify-between gap-3 mb-3">
						<h2 className="text-base font-bold text-foreground">訂單摘要</h2>
						{order.sellerName ? (
							<span className="text-xs text-foreground-soft">
								{order.sellerName}
							</span>
						) : null}
					</header>
					<ul className="flex flex-col gap-2">
						{order.items.map((it) => {
							const imageSrc = it.image ? resolveCdnUrl(it.image) : "";
							return (
								<li
									key={String(it.orderItemId)}
									className="flex items-center gap-3 p-[10px_12px] bg-content2 border border-divider rounded-hkui-lg"
								>
									{imageSrc ? (
										<AppImage
											src={imageSrc}
											alt={it.name}
											width={48}
											height={48}
											sizes="48px"
											className="w-12 h-12 rounded-[var(--hkui-radius-md)] object-cover"
										/>
									) : (
										<span
											className="w-12 h-12 shrink-0 rounded-[var(--hkui-radius-md)] bg-default-100 border border-divider"
											aria-hidden="true"
										/>
									)}
									<div className="flex flex-col gap-0.5 flex-1 min-w-0">
										<strong className="text-sm font-semibold text-foreground truncate">
											{it.name}
										</strong>
										<span className="text-xs text-foreground-soft">
											數量 {it.quantity} · NT${" "}
											{Number(it.price).toLocaleString()}
										</span>
									</div>
								</li>
							);
						})}
					</ul>
					<div className="flex justify-between items-center mt-3 pt-3 border-t border-divider text-sm">
						<span className="text-foreground-soft">實付金額</span>
						<strong className="text-hk-primary text-base">
							NT$ {Number(order.totalActualAmount).toLocaleString()}
						</strong>
					</div>
				</section>

				{/* 2 · 問題描述 */}
				<section className="bg-content1 border border-divider p-[18px] mb-4 shadow-hk-card">
					<h2 className="text-base font-bold text-foreground mb-4">問題描述</h2>
					<ul className="list-none m-0 p-0 flex flex-col gap-2 mb-4">
						{REASON_OPTIONS.map((opt) => (
							<li key={opt.value}>
								<label className="flex items-center gap-3 p-[14px_16px] bg-content2 border border-divider rounded-hkui-lg cursor-pointer transition-all duration-150 has-[:checked]:border-hkui-primary has-[:checked]:bg-hkui-primary-50 has-[:checked]:shadow-[inset_0_0_0_1px_var(--hkui-primary)]">
									<input
										type="radio"
										name="reason"
										checked={reason === opt.value}
										className="w-[18px] h-[18px] accent-hkui-primary"
										onChange={() => setReason(opt.value)}
									/>
									<div className="flex flex-col gap-0.5">
										<strong className="text-sm font-bold text-foreground">
											{REFUND_REASON_LABEL[opt.value]}
										</strong>
										<span className="text-xs text-foreground-soft">
											{opt.subtitle}
										</span>
									</div>
								</label>
							</li>
						))}
					</ul>
					<Textarea
						label="詳細描述"
						isRequired
						variant="bordered"
						size="lg"
						value={description}
						onChange={(e) =>
							setDescription(e.target.value.slice(0, DESCRIPTION_MAX))
						}
						placeholder="請描述遇到的問題（會傳給賣家審核），例如：收到時外包裝完整，但內部商品已壓壞。"
						minRows={3}
						maxLength={DESCRIPTION_MAX}
						isInvalid={descriptionTooShort}
						errorMessage={descriptionTooShort ? descriptionHelp : undefined}
						description={descriptionTooShort ? undefined : descriptionHelp}
					/>
					<div className="flex justify-between gap-3 mt-1 text-xs text-foreground-soft">
						<span>
							{trimmed.length < DESCRIPTION_MIN
								? `最低 ${DESCRIPTION_MIN} 字`
								: "已達申請門檻"}
						</span>
						<span>
							{trimmed.length} / {DESCRIPTION_MAX}
						</span>
					</div>
				</section>

				{/* 3 · 照片證據 */}
				<section className="bg-content1 border border-divider p-[18px] mb-4 shadow-hk-card">
					<h2 className="text-base font-bold text-foreground mb-1">照片證據</h2>
					<p className="text-xs text-foreground-soft mb-3">
						上傳商品問題照片有助於賣家更快審核（選填，最多 3 張）。
					</p>
					<AfterSalePhotoUploader
						value={photos}
						onChange={setPhotos}
						max={3}
						disabled={submitting}
					/>
				</section>

				{error && (
					<p className="text-danger-700 text-sm font-semibold" role="alert">
						{error}
					</p>
				)}

				<div className="flex gap-2 flex-wrap justify-center mt-4 w-full">
					<Button
						href={`/account/orders/${orderNo}`}
						variant="bordered"
						color="default"
					>
						取消
					</Button>
					<Button
						disabled={!canSubmit}
						isLoading={submitting}
						loadingLabel="送出售後申請中…"
						onClick={handleSubmit}
					>
						送出售後申請
					</Button>
				</div>
			</SellerPageShell>
		</main>
	);
}
