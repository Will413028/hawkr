"use client";

/**
 * S04｜自行開票責任條款（58A §S04）。
 *
 * 角色：①條款全文的閱讀處 ②**補簽**與推進 `SELF_ISSUE_PENDING` 的唯一路徑。
 * （裁決 C 之後本頁不再是 S02 送審的前置——勾選框已在送審表單裡。）
 *
 * 🔴 **主 CTA 的判準是 `taxReviewStatus === "SUBMITTED"`，不是 `effective === false`。**
 * 判準退化的兩種方向都會壞，而且都不會轉紅：
 *   讀「簽過沒」⇒ 端點 19 送審時就寫了 `attested_at` ⇒ CTA 對送過審的人永遠不渲染
 *   讀 `effective === false` ⇒ 在 `DRAFT` 也成立 ⇒ 對還沒送審的人渲染一顆必回 9321 的鍵
 * ⇒ 本頁因此**必須同時消費端點 17**，不能只靠端點 21。
 *
 * 🔴 **不渲染「接受帳號」**：端點 21 的 `acceptedByAccountId` 恆為空字串
 * （`seller_tax_profiles` 沒有存接受人，見 `grpc/query.go`）。渲染一個永遠空白的欄位
 * 比不顯示更糟。要查是誰簽的走 ADM 稽核主控台的狀態歷程。
 *
 * 🔴 **「下載條款」不走 `termsKey`**：它同樣恆為空字串。連到既有的服務條款正文
 * （已上線、被 `tests/legal-policy-consistency.test.ts` 覆蓋），不給一個點不開的連結。
 */

import * as React from "react";
import { BackLink } from "@/components/shared/BackLink";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Chip } from "@/components/ui";
import Button from "@/components/ui/Button";
import { SellerTaxReviewStatus } from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useAcceptSelfIssueAttestation,
	useSelfIssueAttestation,
} from "@/lib/hooks/useSelfIssueAttestation";
import { useSellerTaxProfile } from "@/lib/hooks/useSellerInvoiceRemedy";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { SellerCardGridSkeleton } from "../../components/SellerSkeletons";
import {
	SELF_ISSUE_CONSENT_LABEL,
	SELF_ISSUE_DEADLINE_NOTICE,
	SELF_ISSUE_TERM_POINTS,
} from "./selfIssueTerms";

const OVERVIEW_HREF = "/seller-center/invoices";
const TAX_STATUS_HREF = "/seller-center/invoices/tax-status";
/** 既有權威正文：服務條款第二章已明定賣家為發票銷售人。 */
const TERMS_FULLTEXT_HREF = "/terms#fees";

export function SelfIssueTermsPage() {
	const router = useObservedRouter("seller_invoice_self_issue_terms");
	const { attestation, taxProfileMissing, loading, error, refresh } =
		useSelfIssueAttestation();
	const { profile, notStarted, loading: taxLoading } = useSellerTaxProfile();

	const currentTermsVersion = attestation?.currentTermsVersion ?? "";
	const accept = useAcceptSelfIssueAttestation(currentTermsVersion);
	const [checked, setChecked] = React.useState(false);

	if (loading || taxLoading) {
		return (
			<SellerPageShell title="自行開票責任條款" eyebrow="發票與稅籍">
				<SellerCardGridSkeleton
					variant="panel"
					count={2}
					className="mt-4 flex flex-col-reverse gap-4 md:grid md:grid-cols-[minmax(0,1fr)_360px] md:items-start"
				/>
			</SellerPageShell>
		);
	}

	if (error) {
		return (
			<SellerPageShell title="自行開票責任條款" eyebrow="發票與稅籍">
				<div className="seller-card mt-4 p-6 text-center">
					<p className="text-sm font-extrabold text-foreground">
						讀取條款資料時發生問題。
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

	// 還沒建立稅務身分 ⇒ 這一頁在他身上還沒有意義（條款是送審流程的一部分）。
	// 🔴 導向「先去建立」，**不是**錯誤卡：那是每個新賣家的常態。
	if (taxProfileMissing || notStarted) {
		return (
			<SellerPageShell
				title="自行開票責任條款"
				eyebrow="發票與稅籍"
				subtitle="稅務身分審核完成前，發票由你自行開立與交付。"
			>
				<BackLink fallbackHref={OVERVIEW_HREF}>回發票與稅籍</BackLink>
				<section
					role="status"
					className="seller-card mt-4 flex flex-col gap-2 p-5"
				>
					<strong className="text-sm font-extrabold text-foreground">
						還沒有稅務身分資料
					</strong>
					<p className="m-0 text-[0.6875rem] leading-relaxed text-foreground/70">
						{
							"這份條款是送出稅務身分審核時要具結的內容。請先填寫稅籍資料並送出審核。"
						}
					</p>
					<div className="mt-1">
						<Button href={TAX_STATUS_HREF} variant="bordered" size="sm">
							前往稅務身分
						</Button>
					</div>
				</section>
			</SellerPageShell>
		);
	}

	// 🔴 這一行就是本頁的核心判準——見檔頭。
	const reviewStatus = notStarted
		? SellerTaxReviewStatus.NOT_STARTED
		: (profile?.reviewStatus ?? "");
	const canAccept = reviewStatus === SellerTaxReviewStatus.SUBMITTED;

	const onAccept = () => {
		accept.accept();
		// mutation 成功後 hook 會 invalidate；導頁交給 S02 顯示 toast（T24）。
		router.push(`${TAX_STATUS_HREF}?submitted=1`);
	};

	return (
		<SellerPageShell
			title="自行開票責任條款"
			eyebrow="發票與稅籍"
			subtitle="稅務身分審核完成前，發票由你自行開立與交付。"
		>
			<BackLink fallbackHref={OVERVIEW_HREF}>回發票與稅籍</BackLink>

			{/* <768px 時條款排在同意卡之前（先讀後同意）。 */}
			<div className="mt-4 flex flex-col-reverse gap-4 md:grid md:grid-cols-[minmax(0,1fr)_360px] md:items-start">
				<section className="seller-card p-5">
					<h2 className="text-sm font-extrabold text-foreground">
						審核期間的開票責任
					</h2>
					<ol className="mt-3 flex flex-col gap-3">
						{SELF_ISSUE_TERM_POINTS.map((point, i) => (
							<li key={point.title} className="flex gap-3">
								<span className="w-5 h-5 rounded-full bg-hk-accent text-white text-[0.625rem] font-extrabold grid place-items-center shrink-0 mt-[2px]">
									{i + 1}
								</span>
								<span className="flex flex-col gap-[2px] min-w-0">
									<strong className="text-xs font-extrabold text-foreground">
										{point.title}
									</strong>
									<small className="text-[0.6875rem] leading-relaxed text-foreground/65">
										{point.description}
									</small>
								</span>
							</li>
						))}
					</ol>
					<p className="mt-4 pt-3 border-t border-divider text-[0.625rem] text-foreground/55">
						版本 {currentTermsVersion || "—"}
					</p>
				</section>

				<section className="seller-card p-5 md:sticky md:top-4">
					<div className="flex items-center gap-2 mb-3">
						<h2 className="text-sm font-extrabold text-foreground flex-1">
							具結
						</h2>
						{/* `effective` 可以直接點亮 chip——它答的正是「生效中沒」。 */}
						<Chip
							variant="flat"
							color={attestation?.effective ? "success" : "warning"}
							size="sm"
						>
							{attestation?.effective ? "已同意" : "尚未同意"}
						</Chip>
					</div>

					{canAccept ? (
						<>
							<label className="flex items-start gap-3 cursor-pointer">
								<input
									type="checkbox"
									className="w-[18px] h-[18px] accent-hkui-primary shrink-0 mt-[1px]"
									checked={checked}
									onChange={(e) => setChecked(e.target.checked)}
								/>
								<span className="text-[0.6875rem] leading-relaxed text-foreground">
									{SELF_ISSUE_CONSENT_LABEL}
								</span>
							</label>
							<div className="mt-4 flex flex-col gap-2">
								<Button
									variant="solid"
									size="sm"
									isLoading={accept.accepting}
									loadingLabel="送出中…"
									disabled={!checked || !currentTermsVersion}
									onClick={onAccept}
								>
									同意並繼續接單
								</Button>
								{!checked ? (
									// disabled 一定要有可見的原因。
									<small
										role="status"
										className="text-[0.625rem] text-foreground/55"
									>
										請先勾選上方同意項目
									</small>
								) : null}
								{accept.error ? (
									<small
										role="alert"
										className="text-[0.625rem] text-danger-700"
									>
										條款已更新或送出失敗，請重新整理後再同意。
									</small>
								) : null}
							</div>
						</>
					) : (
						// 🔴 已具結／尚未送審 ⇒ checkbox 與主 CTA **完全不在 DOM 內**，
						// 不是 disabled 的 checkbox。
						<dl className="flex flex-col gap-2 text-[0.6875rem]">
							<Row
								label="接受時間"
								value={
									attestation?.acceptedAt
										? formatTaipeiDateTime(attestation.acceptedAt)
										: "尚未接受"
								}
							/>
							<Row label="條款版本" value={attestation?.termsVersion || "—"} />
							{/* 「接受帳號」不渲染——後端恆為空字串，見檔頭。 */}
						</dl>
					)}

					<div className="mt-4">
						<Button
							href={TERMS_FULLTEXT_HREF}
							variant="light"
							size="sm"
							target="_blank"
							rel="noopener noreferrer"
						>
							閱讀條款全文
						</Button>
					</div>
				</section>
			</div>

			{/* 恆在，不得改寫、刪除或摺疊。 */}
			<p
				role="status"
				className="mt-4 p-[12px_14px] rounded-hkui-lg border border-warning-200 bg-warning-50 text-[0.6875rem] leading-relaxed text-warning-700"
			>
				{SELF_ISSUE_DEADLINE_NOTICE}
			</p>

			<p className="mt-3 text-[0.625rem] text-foreground/55">
				想查看目前的審核進度，回到{" "}
				<Link href={TAX_STATUS_HREF} className="text-hk-accent underline">
					稅務身分
				</Link>
				。
			</p>
		</SellerPageShell>
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
