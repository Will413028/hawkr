"use client";

import { errorText } from "@hawkr/api/connect/errors";
import type { MyReportItem } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import {
	commonPolicyCategory,
	moderationReportTargetKind,
} from "@hawkr/ui-text/keys";
import { useTranslations } from "next-intl";
import Link from "@/components/shared/Link";
import { ScEmpty, ScEmptyTitle } from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { useMyReports } from "@/features/report/hooks";
import {
	communityReportOutcomeMessage,
	storeReportOutcomeMessage,
} from "@/features/report/report-logic";
import {
	optionalText,
	useDefaultErrorText,
	useUiText,
} from "@/lib/i18n/ui-text";
import {
	formatReportTime,
	ReportOutcomeChip,
	StoreName,
	type StoreNameState,
	useStoreNames,
} from "./ReportViewParts";

function ReportCard({
	item,
	storeName,
}: {
	item: MyReportItem;
	storeName: (storeId: string) => StoreNameState;
}) {
	const t = useTranslations("report");
	const ut = useUiText();
	const store = item.subject.case === "store" ? item.subject.value : undefined;
	const community =
		item.subject.case === "community" ? item.subject.value : undefined;
	if (!store && !community) return null;
	const outcome = store
		? optionalText(ut, storeReportOutcomeMessage(store))
		: community
			? optionalText(ut, communityReportOutcomeMessage(community))
			: undefined;
	const category = optionalText(ut, commonPolicyCategory(item.category));
	const body = (
		<div className="flex flex-col gap-1.5 rounded-hkui-lg border border-divider bg-content1 p-4">
			<div className="flex items-start justify-between gap-2">
				<strong className="text-base text-hk-heading">
					{store ? (
						<StoreName {...storeName(store.storeId)} />
					) : (
						optionalText(
							ut,
							community?.target
								? moderationReportTargetKind(community.target.kind)
								: undefined,
						)
					)}
				</strong>
				{outcome ? <ReportOutcomeChip text={outcome} /> : null}
			</div>
			{category ? (
				<p className="m-0 text-sm text-hk-body">
					{t("reasonLine", { category })}
				</p>
			) : null}
			{item.description ? (
				<p className="m-0 line-clamp-3 whitespace-pre-wrap text-sm text-hk-muted">
					{item.description}
				</p>
			) : null}
			<p className="m-0 text-xs text-hk-muted">
				{formatReportTime(item.createTime)}
			</p>
		</div>
	);
	if (!store) return body;
	return (
		<Link
			href={`/account/reports/${encodeURIComponent(item.id)}`}
			className="block no-underline"
		>
			{body}
		</Link>
	);
}

export default function MyReportsClient() {
	const t = useTranslations("report");
	const defaults = useDefaultErrorText();
	const { reports, loading, error, hasMore, loadingMore, loadMore, refetch } =
		useMyReports();
	const visible = reports.filter(
		(r) => r.subject.case === "community" || r.subject.case === "store",
	);
	const storeName = useStoreNames(
		visible.flatMap((r) =>
			r.subject.case === "store" ? [r.subject.value.storeId] : [],
		),
	);
	const nextPageFailed = error != null && reports.length > 0;

	return (
		<SellerPageShell title={t("myReportsTitle")}>
			{error && reports.length === 0 ? (
				<div className="flex flex-col items-start gap-2">
					<p className="m-0 text-sm font-semibold text-danger-700" role="alert">
						{errorText(error, defaults)}
					</p>
					<Button variant="bordered" color="default" onClick={refetch}>
						{t("retry")}
					</Button>
				</div>
			) : loading ? (
				<p className="text-sm text-foreground-soft">{t("loading")}</p>
			) : visible.length === 0 && !hasMore && !nextPageFailed ? (
				<ScEmpty>
					<ScEmptyTitle>{t("myReportsEmpty")}</ScEmptyTitle>
				</ScEmpty>
			) : (
				<>
					<ul className="m-0 flex list-none flex-col gap-3 p-0">
						{visible.map((item) => (
							<li key={`${item.subject.case}:${item.id}`}>
								<ReportCard item={item} storeName={storeName} />
							</li>
						))}
					</ul>
					{nextPageFailed ? (
						<p
							className="m-0 text-sm font-semibold text-danger-700"
							role="alert"
						>
							{errorText(error, defaults)}
						</p>
					) : null}
					{hasMore ? (
						<Button
							variant="bordered"
							color="default"
							onClick={loadMore}
							isLoading={loadingMore}
							loadingLabel={t("loading")}
						>
							{t("loadMore")}
						</Button>
					) : null}
				</>
			)}
		</SellerPageShell>
	);
}
