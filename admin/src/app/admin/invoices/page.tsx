"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, StateRow, StatusBadge } from "@/components/UI";
import StatCard from "@/components/UI/StatCard";
import { INVOICE_OPERATION_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import {
	KPI_ORDER,
	kpiDisplayValue,
	kpiMetaOf,
} from "@/features/invoices/overview";
import type {
	InvoiceEnvelope,
	InvoiceKpiItem,
	InvoiceOverviewData,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_OPERATION_STATUS_LABEL,
	INVOICE_OPERATION_TYPE_LABEL,
} from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";
import { api } from "@/services/api";
import "@/styles/pages/Invoice.css";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

const OVERVIEW_ENDPOINT = "/api/v1/admin/invoices/overview";

export default function InvoiceOverviewPage() {
	const router = useRouter();

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "overview"],
		queryFn: async () => {
			const res =
				await api.get<InvoiceEnvelope<InvoiceOverviewData>>(OVERVIEW_ENDPOINT);
			return {
				kpis: res?.data?.kpis ?? [],
				priorityJobs: res?.data?.priorityJobs ?? [],
			};
		},
	});

	const kpis: InvoiceKpiItem[] = data?.kpis ?? [];
	const jobs = data?.priorityJobs ?? [];

	const knownFirst = [
		...KPI_ORDER.filter((key) => kpis.some((k) => k.key === key)),
		...kpis.map((k) => k.key).filter((key) => !KPI_ORDER.includes(key)),
	];
	const orderedKpis = knownFirst
		.map((key) => kpis.find((k) => k.key === key))
		.filter((k): k is InvoiceKpiItem => Boolean(k));

	const placeholderKpis: InvoiceKpiItem[] = KPI_ORDER.map((key) => ({
		key,
		value: "",
		windowLabel: "",
		definition: "",
	}));
	const cards = orderedKpis.length > 0 ? orderedKpis : placeholderKpis;
	const skeleton = isLoading || isError;

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">發票營運總覽</h1>
					<p className="page-subtitle">
						資料為載入當下的快照，請按「手動更新」重新取得
					</p>
				</div>
				<div className="header-actions">
					<Button
						variant="outline"
						size="sm"
						onClick={() => refetch()}
						loading={isFetching}
					>
						手動更新
					</Button>
				</div>
			</div>

			{isError ? (
				<GovernanceNoticeBanner
					notice={{
						tone: "error",
						text: governanceErrorText(
							error,
							"發票營運總覽載入失敗，請按「手動更新」重試。",
						),
					}}
				/>
			) : null}

			<div className="stats-row-v2">
				{cards.map((kpi) => {
					const meta = kpiMetaOf(kpi.key);
					const value = kpiDisplayValue(skeleton ? "" : kpi.value, skeleton);
					const card = (
						<StatCard
							label={meta.label}
							value={value}
							icon={meta.icon}
							variant={meta.variant}
						/>
					);
					if (!meta.href) return <div key={kpi.key}>{card}</div>;
					return (
						<button
							key={kpi.key}
							type="button"
							className="btn-reset stat-card-clickable"
							onClick={() => meta.href && router.push(meta.href)}
							aria-label={`${meta.label}：${value}。${kpi.windowLabel} ${kpi.definition}`.trim()}
						>
							{card}
						</button>
					);
				})}
			</div>

			{orderedKpis.length > 0 ? (
				<dl className="invoice-kpi-definitions">
					{orderedKpis.map((kpi) => {
						const meta = kpiMetaOf(kpi.key);
						return (
							<div key={kpi.key}>
								<dt>
									{meta.label}
									{kpi.windowLabel ? `（${kpi.windowLabel}）` : ""}
								</dt>
								<dd>
									{kpi.definition}
									{meta.note ? ` ${meta.note}` : ""}
								</dd>
							</div>
						);
					})}
				</dl>
			) : null}

			<div className="card">
				<div className="card-header">
					<h3>
						<MaterialSymbol name="crisis_alert" /> 優先處理工作
					</h3>
				</div>
				<div className="card-body">
					<div className="table-container">
						<table className="data-table">
							<thead>
								<tr>
									<th>訂單</th>
									<th>類型</th>
									<th>狀態</th>
									<th>摘要</th>
									<th>最後嘗試</th>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								<StateRow
									loading={isLoading}
									error={isError ? error : null}
									empty={jobs.length === 0}
									emptyText="目前沒有需要優先處理的發票工作"
									colSpan={6}
									onRetry={() => refetch()}
								>
									{jobs.map((job) => (
										<tr key={job.jobId}>
											<td>
												<div className="cell-main">{job.orderNo}</div>

												<div className="cell-sub">賣家 {job.storeId}</div>
											</td>
											<td>
												{codeLabel(
													INVOICE_OPERATION_TYPE_LABEL,
													job.operationType,
												)}
											</td>
											<td>
												<StatusBadge
													variant={
														INVOICE_OPERATION_STATUS_BADGE_VARIANT[
															job.status
														] ?? "gray"
													}
												>
													{codeLabel(
														INVOICE_OPERATION_STATUS_LABEL,
														job.status,
													)}
												</StatusBadge>
											</td>
											<td>{job.summary || "—"}</td>
											<td>{formatDateTime(job.lastAttemptAt)}</td>
											<td>
												<Button
													variant="outline"
													size="sm"
													onClick={() =>
														router.push(`/admin/invoices/jobs/${job.jobId}`)
													}
												>
													前往處理
												</Button>
											</td>
										</tr>
									))}
								</StateRow>
							</tbody>
						</table>
					</div>
				</div>
			</div>

			<div className="adm-page-footer-actions">
				<Button
					variant="outline"
					onClick={() => router.push("/admin/invoices/documents")}
				>
					查詢全平台發票文件
				</Button>
				<Button
					variant="outline"
					onClick={() => router.push("/admin/invoices/issuances/new")}
				>
					手動開立發票
				</Button>
				<p className="adm-page-footer-note">
					手動開立發票是例外處置：開出的是真實稅務憑證，只能作廢或折讓。
				</p>
			</div>
		</div>
	);
}
