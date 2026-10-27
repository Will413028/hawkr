"use client";

import * as React from "react";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { ImportJobStatusBadge } from "../components/ImportJobStatusBadge";
import { ImportProgressBar } from "../components/ImportProgressBar";
import { UI_TEXT } from "../i18n";
import { useProductsImportUiStore } from "../importUiStore";
import { UploadCsvModal } from "../modals/UploadCsvModal";
import { cancelImportJob, listImportJobs } from "../repo";
import {
	ACTIVE_JOB_STATUSES,
	IMPORT_JOB_STATUS,
	type ImportJobBrief,
	TERMINAL_JOB_STATUSES,
} from "../types";

const TEMPLATE_PATH = "/templates/hawkr_import_template.csv";
// 後端 rate-limit：active jobs ≥ 5 或當日已建 ≥ 10 即拒絕（HTTP 429）。
// 前端只能精確掌握 active count（page 1 jobs 已含足夠資訊），daily count 後端未回；
// 因此 button-disable 仍只看 active，但 tooltip / 錯誤文案明示雙重上限，由後端 429 兜底擋下。
const ACTIVE_LIMIT = 5;
const PAGE_SIZE = 20;

function importJobKey(job: ImportJobBrief): string {
	return job.importJobId;
}

function daysUntil(iso: string): number {
	const t = new Date(iso).getTime();
	if (Number.isNaN(t)) return 0;
	return Math.max(0, Math.ceil((t - Date.now()) / (24 * 60 * 60 * 1000)));
}

function fmtTime(iso: string): string {
	// 解不出來就把原字串照登（沿用原本 try/catch 的意圖）。
	return formatTaipeiDateTime(iso) || iso;
}

export function JobsListPage() {
	const router = useObservedRouter("product_import_jobs");
	const toast = useNotificationToast();
	const uploadOpen = useProductsImportUiStore((state) => state.uploadCsvOpen);
	const setUploadOpen = useProductsImportUiStore(
		(state) => state.setUploadCsvOpen,
	);
	const cancelTargetJobId = useProductsImportUiStore(
		(state) => state.cancelTargetJobId,
	);
	const setCancelTargetJobId = useProductsImportUiStore(
		(state) => state.setCancelTargetJobId,
	);

	const [jobs, setJobs] = React.useState<ImportJobBrief[]>([]);
	const [total, setTotal] = React.useState(0);
	const [loading, setLoading] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const [cancelling, setCancelling] = React.useState(false);

	const refresh = React.useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			const r = await listImportJobs({ page: 1, pageSize: PAGE_SIZE });
			setJobs(r.records);
			setTotal(Number(r.total) || 0);
		} catch (e) {
			setError(e instanceof Error ? e.message : "載入匯入任務失敗");
		} finally {
			setLoading(false);
		}
	}, []);

	React.useEffect(() => {
		void refresh();
	}, [refresh]);

	const activeCount = React.useMemo(
		() => jobs.filter((j) => ACTIVE_JOB_STATUSES.includes(j.status)).length,
		[jobs],
	);
	const reachedLimit = activeCount >= ACTIVE_LIMIT;
	const cancelTarget =
		cancelTargetJobId === null
			? null
			: (jobs.find((job) => job.importJobId === cancelTargetJobId) ?? null);

	const handleNewJob = () => {
		setUploadOpen(true);
	};

	const handleCancelConfirm = async () => {
		if (!cancelTarget) return;
		setCancelling(true);
		try {
			await cancelImportJob(cancelTarget.importJobId);
			toast.show({
				variant: "success",
				title: "已取消任務",
				body: cancelTarget.sourceFilename,
				durationMs: 4000,
			});
			setCancelTargetJobId(null);
			await refresh();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "取消失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			setCancelling(false);
		}
	};

	return (
		<>
			<SellerPageShell
				title={UI_TEXT.pageTitle}
				subtitle={`共 ${total} 個任務 · 進行中 ${activeCount} / ${ACTIVE_LIMIT}`}
				actions={
					<div className="seller-page-shell__action-group flex items-center gap-2">
						<a
							href={TEMPLATE_PATH}
							download
							className="inline-flex h-8 items-center justify-center gap-1.5 rounded-[var(--hkui-control-radius-sm)] border-2 border-[var(--hkui-default-300)] bg-transparent px-3 text-xs font-semibold text-[var(--hkui-foreground)] transition-colors hover:bg-[var(--hkui-default-100)]"
						>
							{UI_TEXT.downloadTemplate}
						</a>
						<Button
							color="primary"
							size="sm"
							onClick={handleNewJob}
							disabled={reachedLimit}
							title={reachedLimit ? UI_TEXT.rateLimited : undefined}
						>
							{UI_TEXT.newJob}
						</Button>
					</div>
				}
			>
				{error ? (
					<p className="rounded border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] px-3 py-2 text-sm text-[var(--hkui-danger-700)]">
						{error}
					</p>
				) : null}
				{!loading && jobs.length === 0 ? (
					<div className="seller-import-jobs__empty rounded border border-dashed border-[var(--hkui-default-300)] bg-white p-12 text-center text-[var(--hkui-default-500)]">
						<p>{UI_TEXT.jobsListEmpty}</p>
						<Button
							className="mt-4"
							color="primary"
							size="sm"
							onClick={handleNewJob}
							disabled={reachedLimit}
						>
							{UI_TEXT.newJob}
						</Button>
					</div>
				) : null}
				{jobs.length > 0 ? (
					<div className="seller-import-jobs rounded border border-[var(--hkui-default-200)] bg-white">
						<div className="seller-mobile-disclosure-desktop">
							<table className="w-full text-sm" data-mobile-layout="cards">
								<thead className="bg-[var(--hkui-default-50)] text-[var(--hkui-default-700)]">
									<tr>
										<th className="px-4 py-3 text-left font-medium">
											{UI_TEXT.tableCols.createdAt}
										</th>
										<th className="px-4 py-3 text-left font-medium">
											{UI_TEXT.tableCols.filename}
										</th>
										<th className="px-4 py-3 text-left font-medium">
											{UI_TEXT.tableCols.status}
										</th>
										<th className="px-4 py-3 text-left font-medium">
											{UI_TEXT.tableCols.progress}
										</th>
										<th className="px-4 py-3 text-right font-medium">
											{UI_TEXT.tableCols.totalItems}
										</th>
										<th className="px-4 py-3 text-right font-medium">
											{UI_TEXT.tableCols.committedItems}
										</th>
										<th className="px-4 py-3 text-right font-medium">
											{UI_TEXT.tableCols.expiresIn}
										</th>
										<th className="px-4 py-3 text-right font-medium">
											{UI_TEXT.tableCols.actions}
										</th>
									</tr>
								</thead>
								<tbody>
									{jobs.map((j) => {
										const days = daysUntil(j.expiresAt);
										// brief 不攜帶 progress；列表頁進度估算為「committed / total」百分比。
										const total = Number(j.totalItems) || 0;
										const committed = Number(j.committedCount) || 0;
										const pct =
											total > 0 ? Math.round((committed / total) * 100) : 0;
										const canCancel =
											!TERMINAL_JOB_STATUSES.includes(j.status) &&
											j.status !== IMPORT_JOB_STATUS.COMMITTING;
										return (
											<tr
												key={j.importJobId}
												className="cursor-pointer border-t border-[var(--hkui-default-100)] hover:bg-[var(--hkui-default-50)]"
												onClick={() =>
													router.push(
														`/seller-center/products/import/${j.importJobId}`,
													)
												}
											>
												<td
													className="px-4 py-3 text-xs text-[var(--hkui-default-600)]"
													data-label={UI_TEXT.tableCols.createdAt}
												>
													{fmtTime(j.createdAt)}
												</td>
												<td
													className="max-w-[260px] truncate px-4 py-3 font-medium"
													data-label={UI_TEXT.tableCols.filename}
												>
													{j.sourceFilename}
												</td>
												<td
													className="px-4 py-3"
													data-label={UI_TEXT.tableCols.status}
												>
													<ImportJobStatusBadge status={j.status} />
												</td>
												<td
													className="px-4 py-3"
													data-label={UI_TEXT.tableCols.progress}
												>
													<div className="w-32">
														<ImportProgressBar
															percentage={pct}
															tone={
																j.status === IMPORT_JOB_STATUS.COMMITTED
																	? "success"
																	: j.status === IMPORT_JOB_STATUS.CSV_INVALID
																		? "warning"
																		: "primary"
															}
														/>
													</div>
												</td>
												<td
													className="px-4 py-3 text-right font-mono text-xs"
													data-label={UI_TEXT.tableCols.totalItems}
												>
													{j.totalItems}
												</td>
												<td
													className="px-4 py-3 text-right font-mono text-xs"
													data-label={UI_TEXT.tableCols.committedItems}
												>
													{j.committedCount}
												</td>
												<td
													className="px-4 py-3 text-right text-xs text-[var(--hkui-default-600)]"
													data-label={UI_TEXT.tableCols.expiresIn}
												>
													{TERMINAL_JOB_STATUSES.includes(j.status)
														? "—"
														: `${days} 天`}
												</td>
												<td
													className="px-4 py-3 text-right"
													data-label={UI_TEXT.tableCols.actions}
												>
													{/* biome-ignore lint/a11y/noStaticElementInteractions: row owns navigation; this wrapper only stops bubbling so action buttons do not double-trigger */}
													<div
														className="flex justify-end gap-1"
														onClick={(e) => e.stopPropagation()}
														onKeyDown={(e) => e.stopPropagation()}
													>
														<Button
															size="sm"
															variant="light"
															color="primary"
															href={`/seller-center/products/import/${j.importJobId}`}
														>
															{UI_TEXT.actions.view}
														</Button>
														{canCancel ? (
															<Button
																size="sm"
																variant="light"
																color="danger"
																onClick={() =>
																	setCancelTargetJobId(j.importJobId)
																}
															>
																{UI_TEXT.actions.cancel}
															</Button>
														) : null}
													</div>
												</td>
											</tr>
										);
									})}
								</tbody>
							</table>
						</div>
						<div className="p-2.5">
							<SellerMobileDisclosureList
								items={jobs}
								getKey={importJobKey}
								ariaLabel="批次匯入任務"
								getSummary={(job) => ({
									label: `匯入任務：${job.sourceFilename}`,
									leading: <SellerMobileDisclosureIcon name="upload" />,
									title: job.sourceFilename,
									meta: `${job.committedCount}/${job.totalItems}`,
									status: <ImportJobStatusBadge status={job.status} />,
								})}
								renderDetails={(job) => {
									const days = daysUntil(job.expiresAt);
									const totalItems = Number(job.totalItems) || 0;
									const committed = Number(job.committedCount) || 0;
									const percentage =
										totalItems > 0
											? Math.round((committed / totalItems) * 100)
											: 0;
									const canCancel =
										!TERMINAL_JOB_STATUSES.includes(job.status) &&
										job.status !== IMPORT_JOB_STATUS.COMMITTING;
									return (
										<>
											<div className="seller-mobile-disclosure__body">
												<div className="seller-mobile-disclosure__detail-row">
													<span>{UI_TEXT.tableCols.createdAt}</span>
													<strong>{fmtTime(job.createdAt)}</strong>
												</div>
												<div className="seller-mobile-disclosure__detail-row">
													<span>{UI_TEXT.tableCols.expiresIn}</span>
													<strong>
														{TERMINAL_JOB_STATUSES.includes(job.status)
															? "—"
															: `${days} 天`}
													</strong>
												</div>
												<ImportProgressBar
													percentage={percentage}
													tone={
														job.status === IMPORT_JOB_STATUS.COMMITTED
															? "success"
															: job.status === IMPORT_JOB_STATUS.CSV_INVALID
																? "warning"
																: "primary"
													}
												/>
											</div>
											<div className="seller-mobile-disclosure__actions">
												<Button
													size="sm"
													color="primary"
													href={`/seller-center/products/import/${job.importJobId}`}
												>
													{UI_TEXT.actions.view}
												</Button>
												{canCancel ? (
													<Button
														size="sm"
														variant="bordered"
														color="danger"
														onClick={() =>
															setCancelTargetJobId(job.importJobId)
														}
													>
														{UI_TEXT.actions.cancel}
													</Button>
												) : null}
											</div>
										</>
									);
								}}
							/>
						</div>
					</div>
				) : null}
				{loading ? (
					<p className="py-12 text-center text-sm text-[var(--hkui-default-500)]">
						載入中…
					</p>
				) : null}
			</SellerPageShell>

			<UploadCsvModal
				open={uploadOpen}
				onClose={() => setUploadOpen(false)}
				onSuccess={(resp) => {
					setUploadOpen(false);
					router.push(`/seller-center/products/import/${resp.importJobId}`);
				}}
			/>

			<Dialog
				open={!!cancelTarget}
				onClose={() => setCancelTargetJobId(null)}
				size="sm"
				title={UI_TEXT.commit.cancelConfirmTitle}
				footer={
					<div className="flex w-full justify-end gap-2">
						<Button
							variant="light"
							color="default"
							onClick={() => setCancelTargetJobId(null)}
						>
							取消
						</Button>
						<Button
							color="danger"
							onClick={handleCancelConfirm}
							isLoading={cancelling}
							loadingLabel="取消匯入任務中…"
						>
							確認取消
						</Button>
					</div>
				}
			>
				<p className="text-sm text-[var(--hkui-default-700)]">
					{UI_TEXT.commit.cancelConfirmDesc}
				</p>
				{cancelTarget ? (
					<p className="mt-2 font-mono text-xs text-[var(--hkui-default-500)]">
						{cancelTarget.sourceFilename}
					</p>
				) : null}
			</Dialog>
		</>
	);
}
