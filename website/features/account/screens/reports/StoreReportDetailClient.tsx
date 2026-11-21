"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { commonPolicyCategory } from "@hawkr/ui-text/keys";
import { useTranslations } from "next-intl";
import * as React from "react";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { useMyReport } from "@/features/report/hooks";
import {
	canSupplement,
	MAX_SUPPLEMENTS,
	ownEvidenceCount,
	storeReportOutcomeMessage,
} from "@/features/report/report-logic";
import SupplementDialog from "@/features/report/SupplementDialog";
import {
	optionalText,
	useDefaultErrorText,
	useUiText,
} from "@/lib/i18n/ui-text";
import {
	formatReportTime,
	ReportOutcomeChip,
	StoreName,
} from "./ReportViewParts";

export default function StoreReportDetailClient({
	reportId,
}: {
	reportId: string;
}) {
	const t = useTranslations("report");
	const ut = useUiText();
	const defaults = useDefaultErrorText();
	const { detail, loading, error, notFound, refetch, adopt, invalidate } =
		useMyReport(reportId);
	const [supplementOpen, setSupplementOpen] = React.useState(false);

	const backLink = (
		<Link
			href="/account/reports"
			className="text-sm font-bold text-[var(--hkui-primary)]"
		>
			{t("backToList")}
		</Link>
	);
	const title = t("detailTitle");

	if (!detail?.report) {
		if (error) {
			return (
				<SellerPageShell title={title} actions={backLink}>
					<Card
						data-testid={notFound ? "store-report-not-found" : undefined}
						className="flex flex-col items-center gap-3 p-6 text-center"
					>
						<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
							{errorText(error, defaults)}
						</p>
						{notFound ? null : (
							<Button variant="bordered" color="default" onClick={refetch}>
								{t("retry")}
							</Button>
						)}
					</Card>
				</SellerPageShell>
			);
		}
		return (
			<SellerPageShell title={title} actions={backLink}>
				{loading ? (
					<p className="text-sm text-foreground-soft">{t("loading")}</p>
				) : null}
			</SellerPageShell>
		);
	}

	const r = detail.report;
	const outcome = optionalText(ut, storeReportOutcomeMessage(r));
	const category = optionalText(ut, commonPolicyCategory(r.category));
	const own = ownEvidenceCount(detail);
	return (
		<SellerPageShell title={title} actions={backLink}>
			<Card className="flex flex-col gap-3 p-5">
				<div className="flex items-start justify-between gap-2">
					<strong className="text-lg text-hk-heading">
						<StoreName storeId={r.storeId} />
					</strong>
					{outcome ? <ReportOutcomeChip text={outcome} /> : null}
				</div>
				{category ? (
					<p className="m-0 text-sm text-hk-body">
						{t("reasonLine", { category })}
					</p>
				) : null}
				{r.orderNo ? (
					<p className="m-0 text-sm text-hk-body">
						{t("orderNoLine", { orderNo: r.orderNo })}
					</p>
				) : null}
				{r.resolutionNote ? (
					<section className="flex flex-col gap-1">
						<h2 className="m-0 text-base font-semibold">
							{t("resolutionNote")}
						</h2>
						<p
							data-testid="store-report-resolution-note"
							className="m-0 whitespace-pre-wrap text-sm text-hk-body"
						>
							{r.resolutionNote}
						</p>
					</section>
				) : null}
				<section className="flex flex-col gap-1">
					<h2 className="m-0 text-base font-semibold">{t("yourReport")}</h2>
					<p className="m-0 text-xs text-hk-muted">
						{formatReportTime(r.createTime)}
					</p>
					{r.description ? (
						<p className="m-0 whitespace-pre-wrap text-sm text-hk-body">
							{r.description}
						</p>
					) : null}
					{own > 0 ? (
						<p className="m-0 text-xs text-hk-muted">
							{t("evidenceCount", { count: own })}
						</p>
					) : null}
				</section>
				<section className="flex flex-col gap-2">
					<h2 className="m-0 text-base font-semibold">
						{t("supplementsHeading", {
							count: detail.supplements.length,
							max: MAX_SUPPLEMENTS,
						})}
					</h2>
					{detail.supplements.map((s) => (
						<div key={s.id} className="flex flex-col gap-1">
							<p className="m-0 text-xs text-hk-muted">
								{formatReportTime(s.createTime)}
							</p>
							<p className="m-0 whitespace-pre-wrap text-sm text-hk-body">
								{s.description}
							</p>
							{s.evidencePositions.length > 0 ? (
								<p className="m-0 text-xs text-hk-muted">
									{t("evidenceCount", { count: s.evidencePositions.length })}
								</p>
							) : null}
						</div>
					))}
				</section>
				{canSupplement(detail) ? (
					<Button onClick={() => setSupplementOpen(true)}>
						{t("supplementButton")}
					</Button>
				) : null}
			</Card>
			<SupplementDialog
				open={supplementOpen}
				onClose={() => setSupplementOpen(false)}
				reportId={reportId}
				onAdded={adopt}
				onStale={() => void invalidate()}
			/>
		</SellerPageShell>
	);
}
