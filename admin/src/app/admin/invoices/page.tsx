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

// A01｜發票營運總覽（58A §A01，端點 37 `GET /api/v1/admin/invoices/overview`）。
//
// 本頁是發票分組的根與排班起點：8 張 KPI 卡各自下鑽到對應佇列，優先工作清單直接
// 進 A06。唯讀頁，沒有任何寫入動作。
//
// 三件刻意不做的事：
//   - **不做圖表**：`chart.js` 在 package.json 裡但 `src/` 零 import，全站無已實作圖表。
//   - **不做自動輪詢**：發票端點與該管理員的其他請求共用同一個限流桶，`refetchInterval`
//     會吃掉額度並打壞同頁其他請求。一律「手動更新」鍵 ＋ 副標明寫快照。
//   - **不做 per-KPI 重試**：端點 37 一支回全部 8 個 KPI；拆成 8 支會讓 8 個數字來自
//     8 個時點而互相矛盾。

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

	// 後端沒回的 key 不補位（少一張卡看得出來），回了而清單沒有的 key 排在後面。
	const knownFirst = [
		...KPI_ORDER.filter((key) => kpis.some((k) => k.key === key)),
		...kpis.map((k) => k.key).filter((key) => !KPI_ORDER.includes(key)),
	];
	const orderedKpis = knownFirst
		.map((key) => kpis.find((k) => k.key === key))
		.filter((k): k is InvoiceKpiItem => Boolean(k));

	// 載入中 / 錯誤時仍畫 8 格版面（值為 "--"），版面不塌。
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
							// 只唸得到一個裸數字的卡片對讀屏使用者沒有意義：label ＋ 值 ＋ 口徑
							// 三者都要進 aria-label。
							onClick={() => meta.href && router.push(meta.href)}
							aria-label={`${meta.label}：${value}。${kpi.windowLabel} ${kpi.definition}`.trim()}
						>
							{card}
						</button>
					);
				})}
			</div>

			{/*
			 * 口徑（definition）必須真的渲染出來：A01 是「今天先處理什麼」的排班依據，
			 * 沒有口徑的數字會讓兩個人看同一個畫面得到不同結論。ADM 沒有 tooltip
			 * primitive，所以整批列在卡片下方。
			 */}
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
												{/*
												 * 這裡本來要顯示店名，但端點 37 的 `InvoiceJobInfo`
												 * 只帶 `storeId`（見 58A 與 wire 的落差，已在回報中列出）。
												 * 造一個不存在的 `storeName` 欄只會渲染出 undefined。
												 */}
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

			{/*
			 * 頁尾入口是規格的一部分，不是裝飾：A12 雖然在 Sidebar，但 A01 是排班起點。
			 *
			 * 「手動開立發票」（A11）是 **outline 次要樣式**，不得做成主 CTA：它是低頻的
			 * 例外出口，而 A11 自己的頁頂警示逐字寫著「請勿以手動開票取代系統工作」。
			 * 同理它也不進 Sidebar——放進選單會讓人以為手動開票是常規動作。
			 *
			 * ⚠️ ADM **沒有路由清冊 gate**，指向不存在頁面的按鈕完全靜默 ⇒
			 * `tests/invoice-adm-contract.test.ts` 的「發票模組內所有指向 /admin/invoices
			 * 的連結都有對應的頁面」是那個 gate；這顆鍵曾經因為 A11 未建而被它抓到。
			 */}
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
