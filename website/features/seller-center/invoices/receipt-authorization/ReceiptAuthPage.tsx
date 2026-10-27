"use client";

/**
 * S05｜免用統一發票與收據授權（58A §S05）。
 *
 * 🔴 **本頁沒有任何「自行切換成免發票」的開關。** 免用資格是國稅局核定 ＋ ADM 審核
 * 的結果，API 層根本沒有讓賣家寫入的端點 ⇒ 畫面上不得出現 toggle、radio 或
 * 「切換開票方式」入口。這裡能做的只有一件事：**授權平台代製收據**。
 *
 * 🔴 **不具資格時整頁換 gate，不渲染一個 disabled 的表單。** blocked 退化成
 * 「看起來可以送出的表單」會讓賣家反覆嘗試一個結構上不可能成功的動作。
 *
 * 🔴 **「查看核定依據文件」看 `approvalBasisAvailable`，不看 assetId 非空**：
 * asset 可能存在但這個賣家無權下載（`UPLOADER_NOT_SELLER`）⇒ 組得出路徑、點下去
 * 必定失敗。做出一顆註定 404 的按鈕比不給按鈕更糟。
 */

import * as React from "react";
import { BackLink } from "@/components/shared/BackLink";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Chip } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { formatTaipeiDate, formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useConfirmReceiptAuthorization,
	useSellerReceiptAuthorization,
} from "@/lib/hooks/useSellerReceiptAuthorization";
import { receiptApprovalBasisHref } from "@/lib/invoices/seller-repo";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { SellerCardGridSkeleton } from "../../components/SellerSkeletons";
import { isRevokedOrExpired, receiptAuthGateCopy } from "./receiptAuthGate";

const OVERVIEW_HREF = "/seller-center/invoices";

export function ReceiptAuthPage() {
	const { authorization, loading, error, refresh } =
		useSellerReceiptAuthorization();
	const confirmAuth = useConfirmReceiptAuthorization(
		authorization?.version ?? "0",
	);
	const [checked, setChecked] = React.useState(false);
	const [dialogOpen, setDialogOpen] = React.useState(false);

	if (loading) {
		return (
			<SellerPageShell title="收據代製授權" eyebrow="發票與稅籍">
				<SellerCardGridSkeleton variant="panel" count={3} />
			</SellerPageShell>
		);
	}

	if (error || !authorization) {
		return (
			<SellerPageShell title="收據代製授權" eyebrow="發票與稅籍">
				<BackLink fallbackHref={OVERVIEW_HREF}>回發票與稅籍</BackLink>
				<div className="seller-card mt-4 p-6 text-center">
					<p className="text-sm font-extrabold text-foreground">
						讀取授權資料時發生問題。
					</p>
					<div className="mt-4">
						<Button variant="bordered" size="sm" onClick={() => void refresh()}>
							重試
						</Button>
					</div>
				</div>
			</SellerPageShell>
		);
	}

	const revokedOrExpired = isRevokedOrExpired(authorization.ineligibleReason);
	const authorized = Boolean(authorization.authorizedAt);
	// 端點 28 不回 `taxReviewStatus`，`approvedAt` 是唯一分得出「已核定」與「審核
	// 進行中」的欄位：它在核准那一句才寫（`AdminApproveSellerTaxProfile`），審核進行
	// 中的賣家恆為 NULL。複審中的賣家仍留著值，那是對的——核定確實發生過，複審不
	// 追溯已代製的收據（`AdminReopenSellerTaxProfileReview` 刻意不動這兩欄）。
	const approved = Boolean(authorization.approvedAt);

	// 從來沒核定過 ⇒ 沒有歷史資料可看，整頁只有 gate。
	// 撤銷／到期 ⇒ 歷史核定資料仍要顯示（見下方），只是不能新增授權。
	if (!authorization.eligible && !revokedOrExpired) {
		return (
			<SellerPageShell title="收據代製授權" eyebrow="發票與稅籍">
				<BackLink fallbackHref={OVERVIEW_HREF}>回發票與稅籍</BackLink>
				<GateBanner reason={authorization.ineligibleReason} />
			</SellerPageShell>
		);
	}

	const templateUrl = authorization.receiptTemplateKey
		? resolveCdnUrl(authorization.receiptTemplateKey)
		: "";
	const basisHref = authorization.approvalBasisAvailable
		? receiptApprovalBasisHref(authorization.approvalBasisAssetId)
		: "";

	return (
		<SellerPageShell
			title="收據代製授權"
			eyebrow="發票與稅籍"
			subtitle={
				approved
					? "平台已核定你免用統一發票，可授權平台依你的名義製作交易收據。"
					: "稅務身分審核中。你現在就可以先完成授權，核定免用統一發票後立即生效，不必再回來一次。"
			}
		>
			<BackLink fallbackHref={OVERVIEW_HREF}>回發票與稅籍</BackLink>

			{/* 資格已失效：整頁唯讀，但歷史核定資料照常顯示。 */}
			{revokedOrExpired ? (
				<GateBanner reason={authorization.ineligibleReason} />
			) : null}

			{/* ③ 核准狀態 */}
			<section className="seller-card mt-4 p-5">
				<div className="flex items-center gap-2 mb-3">
					<h2 className="text-sm font-extrabold text-foreground flex-1">
						核定狀態
					</h2>
					<Chip
						variant="flat"
						color={
							revokedOrExpired ? "warning" : approved ? "success" : "primary"
						}
						size="sm"
					>
						{revokedOrExpired
							? "已失效"
							: approved
								? "已核定免用統一發票"
								: "審核中"}
					</Chip>
				</div>
				<dl className="flex flex-col gap-2 text-[0.6875rem]">
					<Row
						label="核定日期"
						value={
							authorization.approvedAt
								? formatTaipeiDate(authorization.approvedAt)
								: "—"
						}
					/>
					<Row
						label="有效期限"
						value={
							authorization.approvalValidUntil
								? formatTaipeiDate(authorization.approvalValidUntil)
								: "—"
						}
					/>
				</dl>
				<div className="mt-3 pt-3 border-t border-divider">
					{basisHref ? (
						<a
							href={basisHref}
							target="_blank"
							rel="noopener noreferrer"
							className="text-[0.6875rem] font-extrabold text-hk-accent underline"
						>
							查看核定依據文件
							<span className="sr-only">（在新分頁開啟）</span>
						</a>
					) : approved ? (
						// 🔴 兩種 unavailableReason 對賣家的文案逐字相同——區分只在後端稽核。
						<small
							role="status"
							className="text-[0.6875rem] text-foreground/60"
						>
							依據文件尚未提供，請洽平台客服。
						</small>
					) : (
						// 還沒核定就沒有核定依據文件，這不是缺漏，不要叫他洽客服。
						<small
							role="status"
							className="text-[0.6875rem] text-foreground/60"
						>
							核定完成後，這裡會顯示平台的核定依據文件。
						</small>
					)}
				</div>
			</section>

			{/* ④ 說明 */}
			<section className="seller-card mt-4 p-5">
				<h2 className="text-sm font-extrabold text-foreground">
					平台代製收據的意思
				</h2>
				<ul className="mt-3 flex flex-col gap-2 text-[0.6875rem] leading-relaxed text-foreground/70 list-disc pl-5">
					<li>Hawkr 依你的授權，代為製作與寄送交易收據。</li>
					<li>
						{
							"你仍然是交易主體與收據上的銷售人，收據內容以這家商店的核定資料為準。"
						}
					</li>
					<li>平台不會為這些交易開立統一發票。</li>
				</ul>
			</section>

			{/* ⑤ 收據樣式 */}
			{templateUrl ? (
				<section className="seller-card mt-4 p-5">
					<h2 className="text-sm font-extrabold text-foreground mb-2">
						收據樣式
					</h2>
					<a
						href={templateUrl}
						target="_blank"
						rel="noopener noreferrer"
						className="text-[0.6875rem] font-extrabold text-hk-accent underline"
					>
						查看收據樣式範本
						<span className="sr-only">（在新分頁開啟）</span>
					</a>
				</section>
			) : null}

			{/* ⑥ 授權 */}
			<section className="seller-card mt-4 p-5">
				<h2 className="text-sm font-extrabold text-foreground mb-3">授權</h2>
				{authorized ? (
					<dl className="flex flex-col gap-2 text-[0.6875rem]">
						<Row
							label="授權時間"
							value={formatTaipeiDateTime(authorization.authorizedAt)}
						/>
						<Row label="條款版本" value={authorization.consentVersion || "—"} />
					</dl>
				) : revokedOrExpired ? (
					<p className="text-[0.6875rem] leading-relaxed text-foreground/70">
						{"資格失效期間無法新增授權。請先回到「稅務身分」重新提交核定證明。"}
					</p>
				) : (
					<>
						<label className="flex items-start gap-3 cursor-pointer">
							<input
								type="checkbox"
								className="w-[18px] h-[18px] accent-hkui-primary shrink-0 mt-[1px]"
								checked={checked}
								onChange={(e) => setChecked(e.target.checked)}
							/>
							<span className="text-[0.6875rem] leading-relaxed text-foreground">
								{"我確認上述收據資料正確，並授權 Hawkr 依此製作交易收據"}
							</span>
						</label>
						<div className="mt-4 flex flex-col gap-2">
							<Button
								variant="solid"
								size="sm"
								isLoading={confirmAuth.confirming}
								loadingLabel="送出中…"
								disabled={!checked}
								onClick={() => setDialogOpen(true)}
							>
								確認收據資料與授權
							</Button>
							{!checked ? (
								<small
									role="status"
									className="text-[0.625rem] text-foreground/55"
								>
									請先勾選上方確認項目
								</small>
							) : null}
							{confirmAuth.error ? (
								<small role="alert" className="text-[0.625rem] text-danger-700">
									{"送出失敗。資格可能已變更，請重新整理後再試。"}
								</small>
							) : null}
						</div>
					</>
				)}
			</section>

			<Dialog
				open={dialogOpen}
				onClose={() => setDialogOpen(false)}
				size="sm"
				title="確認授權"
				footer={
					<div className="flex justify-end gap-2">
						<Button
							variant="light"
							size="sm"
							onClick={() => setDialogOpen(false)}
						>
							返回
						</Button>
						<Button
							variant="solid"
							size="sm"
							onClick={() => {
								setDialogOpen(false);
								confirmAuth.confirm();
							}}
						>
							確認授權
						</Button>
					</div>
				}
			>
				<p className="text-xs leading-relaxed text-foreground">
					{
						"授權後，這家商店的交易收據將由 Hawkr 依你的名義製作與寄送。你仍然是交易主體與收據上的銷售人。"
					}
				</p>
			</Dialog>
		</SellerPageShell>
	);
}

function GateBanner({ reason }: { reason: string }) {
	const copy = receiptAuthGateCopy(reason);
	return (
		<section
			role="status"
			className="seller-card mt-4 flex flex-col gap-2 p-5 border border-warning-200 bg-warning-50"
		>
			<strong className="text-sm font-extrabold text-warning-700">
				{copy.title}
			</strong>
			<p className="m-0 text-[0.6875rem] leading-relaxed text-warning-700/90">
				{copy.description}
			</p>
			<div className="mt-1">
				<Button size="sm" variant="bordered" href={copy.ctaHref}>
					{copy.ctaLabel}
				</Button>
			</div>
		</section>
	);
}

function Row({ label, value }: { label: string; value: string }) {
	return (
		<div className="flex items-baseline justify-between gap-3">
			<dt className="text-foreground/60 shrink-0">{label}</dt>
			<dd className="font-extrabold text-foreground text-right min-w-0 truncate">
				{value}
			</dd>
		</div>
	);
}
