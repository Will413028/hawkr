"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Dialog } from "@/components/ui/Dialog";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { CsvInvalidBanner } from "../components/CsvInvalidBanner";
import { ImportItemStatusBadge } from "../components/ImportItemStatusBadge";
import { ImportJobStatusBadge } from "../components/ImportJobStatusBadge";
import { ImportProgressBar } from "../components/ImportProgressBar";
import { commitSkipReasonLabel, ITEM_STATUS_LABEL, UI_TEXT } from "../i18n";
import { useProductsImportUiStore } from "../importUiStore";
import {
	isFormalProductRemoved,
	isImportItemBatchEditable,
	isImportItemCommitEligible,
	isImportItemEditable,
	isImportItemPreviewable,
	isImportItemRemovable,
	isImportItemRestorable,
	isImportItemSelectable,
} from "../itemState";
import { BatchCategoryModal } from "../modals/BatchCategoryModal";
import { BatchShippingModal } from "../modals/BatchShippingModal";
import { BatchTargetStatusModal } from "../modals/BatchTargetStatusModal";
import { UploadCsvModal } from "../modals/UploadCsvModal";
import {
	cancelImportJob,
	commitImportJob,
	getImportJob,
	listImportItems,
	removeImportItems,
	restoreImportItems,
	retryAllFailedAssets,
	retryItemAssets,
} from "../repo";
import {
	IMPORT_ITEM_STATUS,
	IMPORT_JOB_STATUS,
	type ImportItemBrief,
	type ImportItemStatusCode,
	type ImportJobDetail,
	TERMINAL_JOB_STATUSES,
} from "../types";

interface Props {
	jobId: string;
}

const ITEM_PAGE_SIZE = 50;
const POLL_INTERVAL_MS = 5000;

type PendingAction =
	| "retry-all-assets"
	| "commit-all"
	| "commit-selected"
	| "cancel-job"
	| "remove-selected"
	| "restore-selected"
	| `retry-item:${string}`
	| `restore-item:${string}`
	| `remove-item:${string}`;

const ITEM_STATUS_FILTER_OPTIONS: ImportItemStatusCode[] = [
	IMPORT_ITEM_STATUS.READY,
	IMPORT_ITEM_STATUS.PARSED,
	IMPORT_ITEM_STATUS.ASSET_DOWNLOADING,
	IMPORT_ITEM_STATUS.COMMIT_FAILED,
	IMPORT_ITEM_STATUS.COMMITTED,
	IMPORT_ITEM_STATUS.REMOVED,
];

function fmtTime(iso: string): string {
	// 解不出來就把原字串照登（沿用原本 try/catch 的意圖）。
	return formatTaipeiDateTime(iso) || iso;
}

function fmtPriceRange(min?: string, max?: string): string {
	if (!min && !max) return "—";
	if (min && max && min !== max) return `$${min} - $${max}`;
	return `$${min ?? max}`;
}

export function JobDetailPage({ jobId }: Props) {
	const toast = useNotificationToast();

	const [job, setJob] = React.useState<ImportJobDetail | null>(null);
	const [jobLoading, setJobLoading] = React.useState(true);
	const [jobError, setJobError] = React.useState<string | null>(null);

	const [items, setItems] = React.useState<ImportItemBrief[]>([]);
	const [itemsTotal, setItemsTotal] = React.useState(0);
	const [itemsLoading, setItemsLoading] = React.useState(false);
	const statusFilter = useProductsImportUiStore((state) => state.statusFilter);
	const toggleStatusFilter = useProductsImportUiStore(
		(state) => state.toggleStatusFilter,
	);
	const onlyWithErrors = useProductsImportUiStore(
		(state) => state.onlyWithErrors,
	);
	const setOnlyWithErrors = useProductsImportUiStore(
		(state) => state.setOnlyWithErrors,
	);
	const includeRemoved = useProductsImportUiStore(
		(state) => state.includeRemoved,
	);
	const setIncludeRemoved = useProductsImportUiStore(
		(state) => state.setIncludeRemoved,
	);
	const selected = useProductsImportUiStore(
		(state) => state.selectedImportItemIds,
	);
	const setSelected = useProductsImportUiStore(
		(state) => state.setSelectedImportItemIds,
	);
	const toggleSelectedImportItem = useProductsImportUiStore(
		(state) => state.toggleImportItemSelected,
	);

	const reuploadOpen = useProductsImportUiStore((state) => state.uploadCsvOpen);
	const setReuploadOpen = useProductsImportUiStore(
		(state) => state.setUploadCsvOpen,
	);
	const shippingOpen = useProductsImportUiStore(
		(state) => state.batchShippingOpen,
	);
	const setShippingOpen = useProductsImportUiStore(
		(state) => state.setBatchShippingOpen,
	);
	const categoryOpen = useProductsImportUiStore(
		(state) => state.batchCategoryOpen,
	);
	const setCategoryOpen = useProductsImportUiStore(
		(state) => state.setBatchCategoryOpen,
	);
	const targetStatusOpen = useProductsImportUiStore(
		(state) => state.batchTargetStatusOpen,
	);
	const setTargetStatusOpen = useProductsImportUiStore(
		(state) => state.setBatchTargetStatusOpen,
	);

	const confirmCommit = useProductsImportUiStore(
		(state) => state.confirmCommitOpen,
	);
	const setConfirmCommit = useProductsImportUiStore(
		(state) => state.setConfirmCommitOpen,
	);
	const confirmCommitSelected = useProductsImportUiStore(
		(state) => state.confirmCommitSelectedOpen,
	);
	const setConfirmCommitSelected = useProductsImportUiStore(
		(state) => state.setConfirmCommitSelectedOpen,
	);
	const commitResult = useProductsImportUiStore((state) => state.commitResult);
	const setCommitResult = useProductsImportUiStore(
		(state) => state.setCommitResult,
	);
	const confirmCancel = useProductsImportUiStore(
		(state) => state.confirmCancelOpen,
	);
	const setConfirmCancel = useProductsImportUiStore(
		(state) => state.setConfirmCancelOpen,
	);
	const confirmRemove = useProductsImportUiStore(
		(state) => state.confirmRemoveOpen,
	);
	const setConfirmRemove = useProductsImportUiStore(
		(state) => state.setConfirmRemoveOpen,
	);
	const [pendingAction, setPendingAction] =
		React.useState<PendingAction | null>(null);
	const pendingActionRef = React.useRef<PendingAction | null>(null);
	const busy = pendingAction !== null;
	const beginPendingAction = React.useCallback((action: PendingAction) => {
		if (pendingActionRef.current) return false;
		pendingActionRef.current = action;
		setPendingAction(action);
		return true;
	}, []);
	const finishPendingAction = React.useCallback(() => {
		pendingActionRef.current = null;
		setPendingAction(null);
	}, []);

	const refreshJob = React.useCallback(async () => {
		try {
			const j = await getImportJob(jobId);
			setJob(j);
			setJobError(null);
		} catch (e) {
			setJobError(e instanceof Error ? e.message : "載入任務失敗");
		} finally {
			setJobLoading(false);
		}
	}, [jobId]);

	const refreshItems = React.useCallback(async () => {
		setItemsLoading(true);
		try {
			const r = await listImportItems(jobId, {
				status: statusFilter.length ? statusFilter : undefined,
				onlyWithErrors: onlyWithErrors || undefined,
				includeRemoved: includeRemoved || undefined,
				page: 1,
				pageSize: ITEM_PAGE_SIZE,
			});
			setItems(r.records);
			setItemsTotal(Number(r.total) || 0);
			// Drop selection of items that disappeared from view
			const visibleIds = new Set(
				r.records.filter(isImportItemSelectable).map((i) => i.importItemId),
			);
			setSelected(
				[...useProductsImportUiStore.getState().selectedImportItemIds].filter(
					(id) => visibleIds.has(id),
				),
			);
		} catch (e) {
			toast.show({
				variant: "error",
				title: "載入商品失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			setItemsLoading(false);
		}
	}, [jobId, statusFilter, onlyWithErrors, includeRemoved, toast, setSelected]);

	const refreshAll = React.useCallback(async () => {
		await Promise.all([refreshJob(), refreshItems()]);
	}, [refreshJob, refreshItems]);

	// 訂閱 dataRefreshTick：ItemEditPage 儲存 / 移除 / 批次操作完成後 bump，
	// 觸發本頁 refetch。解決 Next.js client cache 讓返回時不重 mount 的 stale 問題。
	const dataRefreshTick = useProductsImportUiStore(
		(state) => state.dataRefreshTick,
	);

	// biome-ignore lint/correctness/useExhaustiveDependencies: dataRefreshTick is the explicit invalidation signal
	React.useEffect(() => {
		void refreshJob();
	}, [refreshJob, dataRefreshTick]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: dataRefreshTick is the explicit invalidation signal
	React.useEffect(() => {
		void refreshItems();
	}, [refreshItems, dataRefreshTick]);

	/* ===== Polling for active states ===== */
	React.useEffect(() => {
		if (!job) return;
		if (
			job.status !== IMPORT_JOB_STATUS.PARSING &&
			job.status !== IMPORT_JOB_STATUS.COMMITTING
		) {
			return;
		}
		const t = setInterval(() => {
			void refreshAll();
		}, POLL_INTERVAL_MS);
		return () => clearInterval(t);
	}, [job, refreshAll]);

	const selectableItems = React.useMemo(
		() => items.filter(isImportItemSelectable),
		[items],
	);
	const selectedItems = React.useMemo(
		() => items.filter((i) => selected.has(i.importItemId)),
		[items, selected],
	);
	const selectedBatchEditableIds = React.useMemo(
		() =>
			selectedItems
				.filter(isImportItemBatchEditable)
				.map((i) => i.importItemId),
		[selectedItems],
	);
	const selectedCommitEligibleIds = React.useMemo(
		() =>
			selectedItems
				.filter(isImportItemCommitEligible)
				.map((i) => i.importItemId),
		[selectedItems],
	);
	const selectedRemovableIds = React.useMemo(
		() =>
			selectedItems.filter(isImportItemRemovable).map((i) => i.importItemId),
		[selectedItems],
	);
	const selectedRestorableIds = React.useMemo(
		() =>
			selectedItems.filter(isImportItemRestorable).map((i) => i.importItemId),
		[selectedItems],
	);
	const allSelected =
		selectableItems.length > 0 &&
		selectableItems.every((i) => selected.has(i.importItemId));
	const toggleSelectAll = () => {
		if (allSelected) {
			setSelected([]);
		} else {
			setSelected(selectableItems.map((i) => i.importItemId));
		}
	};
	const toggleOne = (item: ImportItemBrief) => {
		if (!isImportItemSelectable(item)) return;
		toggleSelectedImportItem(item.importItemId);
	};
	const selectedIds = React.useMemo(() => Array.from(selected), [selected]);

	const handleRetryAllFailedAssets = async () => {
		if (!beginPendingAction("retry-all-assets")) return;
		try {
			// brief 不攜帶 assetsFailed 計數；對所有 commit-failed / asset-downloading
			// 狀態的 item 全部送 retry，後端自會 noop 無失敗 asset 的 item。
			const r = await listImportItems(jobId, { page: 1, pageSize: 200 });
			const targets = r.records
				.filter(
					(it) =>
						it.status === IMPORT_ITEM_STATUS.COMMIT_FAILED ||
						it.status === IMPORT_ITEM_STATUS.ASSET_DOWNLOADING,
				)
				.map((it) => it.importItemId);
			if (targets.length === 0) {
				toast.show({
					variant: "default",
					title: "沒有需要重試的圖片",
					durationMs: 3000,
				});
				return;
			}
			const resp = await retryAllFailedAssets(jobId, targets);
			toast.show({
				variant: "success",
				title: "已排入重試",
				body: `共 ${resp.queuedCount} 張圖片`,
				durationMs: 4000,
			});
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "重試失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	const handleRetryItem = async (itemId: string) => {
		if (!beginPendingAction(`retry-item:${itemId}`)) return;
		try {
			await retryItemAssets(jobId, itemId);
			toast.show({
				variant: "success",
				title: "已排入重試",
				durationMs: 3000,
			});
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "重試失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	const handleRestoreItem = async (itemId: string) => {
		if (!beginPendingAction(`restore-item:${itemId}`)) return;
		try {
			await restoreImportItems(jobId, [itemId]);
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "恢復失敗",
				body: e instanceof Error ? e.message : "",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	const handleRemoveItem = async (itemId: string) => {
		if (!beginPendingAction(`remove-item:${itemId}`)) return;
		try {
			await removeImportItems(jobId, [itemId]);
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "移除失敗",
				body: e instanceof Error ? e.message : "",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	const handleConfirmCommit = async () => {
		if (!beginPendingAction("commit-all")) return;
		try {
			const r = await commitImportJob(jobId);
			const skipped = r.skippedItemIds?.length ?? 0;
			toast.show({
				variant: "success",
				title: "Commit 已送出",
				body:
					skipped > 0
						? UI_TEXT.commit.successWithSkipped(
								Number(r.queuedCount) || 0,
								skipped,
							)
						: UI_TEXT.commit.success(Number(r.queuedCount) || 0),
				durationMs: 5000,
			});
			setConfirmCommit(false);
			if (skipped > 0) setCommitResult(r);
			setSelected([]);
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "Commit 失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	/** 批次 Commit（include_item_ids = 當前勾選）。 */
	const handleConfirmCommitSelected = async () => {
		if (!beginPendingAction("commit-selected")) return;
		try {
			const r = await commitImportJob(jobId, {
				includeItemIds: selectedCommitEligibleIds,
			});
			const skipped = r.skippedItemIds?.length ?? 0;
			toast.show({
				variant: "success",
				title: "Commit 已送出",
				body:
					skipped > 0
						? UI_TEXT.commit.successWithSkipped(
								Number(r.queuedCount) || 0,
								skipped,
							)
						: UI_TEXT.commit.success(Number(r.queuedCount) || 0),
				durationMs: 5000,
			});
			setConfirmCommitSelected(false);
			if (skipped > 0) setCommitResult(r);
			setSelected([]);
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "Commit 失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	const handleConfirmCancel = async () => {
		if (!beginPendingAction("cancel-job")) return;
		try {
			await cancelImportJob(jobId);
			toast.show({
				variant: "success",
				title: "已取消任務",
				durationMs: 4000,
			});
			setConfirmCancel(false);
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "取消失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	const handleConfirmRemove = async () => {
		if (!beginPendingAction("remove-selected")) return;
		try {
			const r = await removeImportItems(jobId, selectedRemovableIds);
			toast.show({
				variant: "success",
				title: "已移除",
				body: `共 ${r.affectedCount} 個商品`,
				durationMs: 4000,
			});
			setConfirmRemove(false);
			setSelected([]);
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "移除失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	const handleRestoreBatch = async () => {
		if (!beginPendingAction("restore-selected")) return;
		try {
			const r = await restoreImportItems(jobId, selectedRestorableIds);
			toast.show({
				variant: "success",
				title: "已恢復",
				body: `共 ${r.affectedCount} 個商品`,
				durationMs: 4000,
			});
			setSelected([]);
			await refreshAll();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "恢復失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			finishPendingAction();
		}
	};

	if (jobLoading) {
		return (
			<SellerPageShell title="批次匯入 詳情">
				<p className="py-12 text-center text-sm text-[var(--hkui-default-500)]">
					載入中…
				</p>
			</SellerPageShell>
		);
	}

	if (jobError || !job) {
		return (
			<SellerPageShell title="批次匯入 詳情">
				<p className="rounded border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] px-3 py-2 text-sm text-[var(--hkui-danger-700)]">
					{jobError ?? "找不到任務"}
				</p>
			</SellerPageShell>
		);
	}

	const counters = job.counters;
	const itemsNeedsFix = Number(counters?.itemsNeedsFix ?? "0") || 0;
	const itemsEligible = Number(counters?.itemsReady ?? "0") || 0;
	const assetsDownloading = Number(counters?.assetsDownloading ?? "0") || 0;
	const assetsFailed = Number(counters?.assetsFailed ?? "0") || 0;
	const canCommit =
		job.status === IMPORT_JOB_STATUS.READY && itemsNeedsFix === 0;
	const canPreview =
		job.status === IMPORT_JOB_STATUS.READY && itemsEligible > 0;
	const canCancel = !TERMINAL_JOB_STATUSES.includes(job.status);
	const showProgress =
		job.status === IMPORT_JOB_STATUS.PARSING ||
		job.status === IMPORT_JOB_STATUS.READY ||
		job.status === IMPORT_JOB_STATUS.COMMITTING;
	const isCsvInvalid = job.status === IMPORT_JOB_STATUS.CSV_INVALID;
	const showRetryAllAssets = assetsFailed > 0;

	const breadcrumb = (
		<nav
			aria-label="麵包屑"
			className="flex flex-wrap items-center gap-1 text-xs text-[var(--hkui-default-500)]"
		>
			<Link
				href="/seller-center/dashboard"
				className="hover:text-[var(--hkui-primary)]"
			>
				賣家中心
			</Link>
			<AppIcon name="chevron-right" size={14} />
			<Link
				href="/seller-center/products"
				className="hover:text-[var(--hkui-primary)]"
			>
				商品管理
			</Link>
			<AppIcon name="chevron-right" size={14} />
			<Link
				href="/seller-center/products/import"
				className="hover:text-[var(--hkui-primary)]"
			>
				批次匯入
			</Link>
			<AppIcon name="chevron-right" size={14} />
			<span className="font-semibold text-[var(--hkui-foreground)]">詳情</span>
		</nav>
	);

	return (
		<>
			<SellerPageShell
				className="seller-import-detail"
				title={
					<div className="flex flex-col gap-1">
						{breadcrumb}
						<span className="flex flex-wrap items-center gap-3">
							<span className="truncate">{job.sourceFilename}</span>
							<ImportJobStatusBadge status={job.status} />
						</span>
					</div>
				}
				subtitle={
					<span className="text-xs text-[var(--hkui-default-500)]">
						建立 {fmtTime(job.createdAt)} · 到期 {fmtTime(job.expiresAt)}
					</span>
				}
				actions={
					<div className="seller-page-shell__action-group flex flex-wrap items-center gap-2">
						<Button
							color="secondary"
							variant="solid"
							size="sm"
							href={`/seller-product-import-preview/${jobId}`}
							target="_blank"
							rel="noopener noreferrer"
							disabled={!canPreview}
							title={
								canPreview
									? `以商城樣貌預覽本次匯入的 ${itemsEligible} 筆商品（尚未真正上架）`
									: "需任務狀態為「可上架」且有可上架商品才能預覽"
							}
						>
							{UI_TEXT.actions.preview}
							{canPreview && itemsEligible > 0 ? (
								<span className="ml-1 opacity-80">（{itemsEligible} 筆）</span>
							) : null}
						</Button>
						<Button
							color="primary"
							size="sm"
							disabled={!canCommit || busy}
							onClick={() => setConfirmCommit(true)}
							title={
								canCommit
									? `將本次匯入全部 ${itemsEligible} 筆就緒商品一次上架`
									: `尚有 ${itemsNeedsFix} 筆需修正，請先處理才能全部 Commit`
							}
						>
							{UI_TEXT.actions.commit}
							{canCommit && itemsEligible > 0 ? (
								<span className="ml-1 opacity-80">（{itemsEligible} 筆）</span>
							) : null}
						</Button>
						<Button
							color="danger"
							variant="light"
							size="sm"
							disabled={!canCancel || busy}
							onClick={() => setConfirmCancel(true)}
						>
							{UI_TEXT.actions.cancel}
						</Button>
					</div>
				}
			>
				{isCsvInvalid ? (
					<CsvInvalidBanner
						errors={job.validationErrors ?? []}
						onRetry={() => setReuploadOpen(true)}
					/>
				) : null}

				{showProgress ? (
					<section className="rounded border border-[var(--hkui-default-200)] bg-white p-4">
						<ImportProgressBar
							percentage={Number(job.progress?.overallPercentage ?? "0")}
							label="整體進度"
							tone={
								job.status === IMPORT_JOB_STATUS.READY ? "success" : "primary"
							}
						/>
						<p className="mt-3 flex flex-wrap items-center gap-3 text-xs text-[var(--hkui-default-600)]">
							<span>{UI_TEXT.progress.eligible(itemsEligible)}</span>
							<span aria-hidden>·</span>
							<span>{UI_TEXT.progress.needsFix(itemsNeedsFix)}</span>
							<span aria-hidden>·</span>
							<span>
								{UI_TEXT.progress.downloadingCount(assetsDownloading)}
							</span>
							{showRetryAllAssets ? (
								<Button
									size="sm"
									variant="bordered"
									color="warning"
									onClick={handleRetryAllFailedAssets}
									disabled={busy}
									isLoading={pendingAction === "retry-all-assets"}
									loadingLabel="重試失敗下載中…"
								>
									{UI_TEXT.actions.retryAssets}
								</Button>
							) : null}
						</p>
					</section>
				) : null}

				{/* ===== Items 篩選列 ===== */}
				<section className="rounded border border-[var(--hkui-default-200)] bg-white">
					<header className="seller-import-detail__filter-rail flex flex-wrap items-center gap-2 border-b border-[var(--hkui-default-100)] px-4 py-3">
						<span className="text-xs font-medium text-[var(--hkui-default-600)]">
							狀態：
						</span>
						{ITEM_STATUS_FILTER_OPTIONS.map((s) => {
							const active = statusFilter.includes(s);
							return (
								<button
									key={s}
									type="button"
									onClick={() => toggleStatusFilter(s)}
									className={`inline-flex h-7 items-center gap-1 rounded-full border px-2.5 text-xs transition-colors ${
										active
											? "border-[var(--hkui-primary)] bg-[var(--hkui-primary-50)] text-[var(--hkui-primary)]"
											: "border-[var(--hkui-default-300)] bg-transparent text-[var(--hkui-default-700)] hover:bg-[var(--hkui-default-50)]"
									}`}
									aria-pressed={active}
								>
									{ITEM_STATUS_LABEL[s] ?? s}
								</button>
							);
						})}
						<span className="ms-auto flex items-center gap-3 text-xs text-[var(--hkui-default-600)]">
							<label className="flex items-center gap-1.5">
								<input
									type="checkbox"
									checked={onlyWithErrors}
									onChange={(e) => setOnlyWithErrors(e.target.checked)}
									className="size-4 accent-[var(--hkui-primary)]"
								/>
								<span>僅顯示需修正</span>
							</label>
							<label className="flex items-center gap-1.5">
								<input
									type="checkbox"
									checked={includeRemoved}
									onChange={(e) => setIncludeRemoved(e.target.checked)}
									className="size-4 accent-[var(--hkui-primary)]"
								/>
								<span>顯示已移除</span>
							</label>
						</span>
					</header>

					{/* 批次工具列 */}
					<div className="seller-import-detail__bulk-rail flex flex-wrap items-center gap-2 border-b border-[var(--hkui-default-100)] bg-[var(--hkui-default-50)] px-4 py-2 text-xs">
						<label className="flex items-center gap-2">
							<input
								type="checkbox"
								checked={allSelected}
								onChange={toggleSelectAll}
								className="size-4 accent-[var(--hkui-primary)]"
							/>
							<span className="text-[var(--hkui-default-700)]">全選</span>
						</label>
						<span className="text-[var(--hkui-default-500)]">
							{UI_TEXT.bulk.selected(selectedIds.length)}
						</span>
						<span className="ml-auto flex flex-wrap items-center gap-1.5">
							<Button
								size="sm"
								variant="bordered"
								color="primary"
								disabled={selectedBatchEditableIds.length === 0 || busy}
								onClick={() => setShippingOpen(true)}
							>
								{UI_TEXT.bulk.setShipping}
							</Button>
							<Button
								size="sm"
								variant="bordered"
								color="primary"
								disabled={selectedBatchEditableIds.length === 0 || busy}
								onClick={() => setCategoryOpen(true)}
							>
								{UI_TEXT.bulk.setCategory}
							</Button>
							<Button
								size="sm"
								variant="bordered"
								color="primary"
								disabled={selectedBatchEditableIds.length === 0 || busy}
								onClick={() => setTargetStatusOpen(true)}
							>
								{UI_TEXT.bulk.setTargetStatus}
							</Button>
							<Button
								size="sm"
								color="primary"
								disabled={
									selectedCommitEligibleIds.length === 0 ||
									job.status !== IMPORT_JOB_STATUS.READY ||
									busy
								}
								onClick={() => setConfirmCommitSelected(true)}
								title={
									job.status !== IMPORT_JOB_STATUS.READY
										? "需任務 READY 狀態才能 Commit"
										: selectedCommitEligibleIds.length === 0
											? "請勾選已就緒且無阻擋錯誤的商品"
											: undefined
								}
							>
								{UI_TEXT.bulk.commitSelected}
							</Button>
							<Button
								size="sm"
								variant="bordered"
								color="danger"
								disabled={selectedRemovableIds.length === 0 || busy}
								onClick={() => setConfirmRemove(true)}
							>
								{UI_TEXT.bulk.removeBatch}
							</Button>
							<Button
								size="sm"
								variant="bordered"
								color="success"
								disabled={selectedRestorableIds.length === 0 || busy}
								onClick={handleRestoreBatch}
								isLoading={pendingAction === "restore-selected"}
								loadingLabel="恢復已勾選商品中…"
							>
								{UI_TEXT.bulk.restoreBatch}
							</Button>
						</span>
					</div>

					{/* Items table */}
					<div className="seller-import-detail__items">
						<table className="w-full text-sm" data-mobile-layout="cards">
							<thead className="bg-[var(--hkui-default-50)] text-[var(--hkui-default-700)]">
								<tr>
									<th className="w-10 px-3 py-2" />
									<th className="w-16 px-3 py-2 text-left font-medium">縮圖</th>
									<th className="px-3 py-2 text-left font-medium">商品名稱</th>
									<th className="px-3 py-2 text-left font-medium">分類</th>
									<th className="px-3 py-2 text-right font-medium">SKU</th>
									<th className="px-3 py-2 text-right font-medium">價格</th>
									<th className="px-3 py-2 text-left font-medium">狀態</th>
									<th className="px-3 py-2 text-right font-medium">操作</th>
								</tr>
							</thead>
							<tbody>
								{items.map((it) => {
									// presigned／原站 URL，直接當 <img src>。**不要**走
									// resolveCdnUrl —— 匯入中的圖還在 tmp/product-import/
									// 前綴，沒有平台 objectKey，image Worker 只服務 public/。
									const cdn = it.firstImagePreviewUrl ?? "";
									const warningCount = Number(it.warningCount) || 0;
									const canPreviewItem = isImportItemPreviewable(it);
									const isRemoved = it.status === IMPORT_ITEM_STATUS.REMOVED;
									const canSelectItem = isImportItemSelectable(it);
									return (
										<tr
											key={it.importItemId}
											className="border-t border-[var(--hkui-default-100)] align-middle"
										>
											<td className="px-3 py-2" data-label="選取">
												<input
													type="checkbox"
													checked={selected.has(it.importItemId)}
													onChange={() => toggleOne(it)}
													disabled={!canSelectItem}
													className="size-4 accent-[var(--hkui-primary)]"
													aria-label={`選取 ${it.name}`}
												/>
											</td>
											<td className="px-3 py-2" data-label="縮圖">
												<div className="relative size-12 overflow-hidden rounded bg-[var(--hkui-default-100)]">
													{cdn ? (
														<AppImage
															src={cdn}
															alt={it.name}
															fill
															sizes="48px"
															className="object-cover"
														/>
													) : null}
												</div>
											</td>
											<td className="px-3 py-2" data-label="商品名稱">
												<p className="font-medium text-[var(--hkui-foreground)]">
													{it.name || "（未命名）"}
												</p>
												<p className="font-mono text-[0.6875rem] text-[var(--hkui-default-500)]">
													{it.productKey}
												</p>
												<div className="mt-1 flex flex-wrap gap-1">
													{warningCount > 0 ? (
														<Chip size="sm" variant="flat" color="warning">
															{warningCount} 個警告
														</Chip>
													) : null}
													{it.hasDuplicateExisting ? (
														<Chip size="sm" variant="flat" color="danger">
															重名
														</Chip>
													) : null}
												</div>
											</td>
											<td
												className="px-3 py-2 text-xs text-[var(--hkui-default-700)]"
												data-label="分類"
											>
												{it.catL3Name || "—"}
											</td>
											<td
												className="px-3 py-2 text-right font-mono text-xs"
												data-label="SKU"
											>
												{it.skuCount}
											</td>
											<td
												className="px-3 py-2 text-right font-mono text-xs"
												data-label="價格"
											>
												{fmtPriceRange(it.priceMin, it.priceMax)}
											</td>
											<td className="px-3 py-2" data-label="狀態">
												<ImportItemStatusBadge
													status={it.status}
													needsFix={it.needsFix}
												/>
											</td>
											<td className="px-3 py-2 text-right" data-label="操作">
												{!isImportItemEditable(it) ? (
													/* v3：已上架 item 不再提供編輯/移除，改以正式商品狀態 + 連結。
													 * backend #2819：三種狀態必須分得開——
													 *   已移除（4014）→ 說「已移除」，且**不給連結**（點下去是 404）
													 *   查不到        → 說「狀態暫時無法取得」，仍給連結（商品可能還在）
													 *   正常          → 顯示狀態 + 連結
													 */
													<div className="flex flex-col items-end gap-1">
														{isFormalProductRemoved(it) ? (
															<span className="text-[0.6875rem] text-[var(--hkui-danger-600)]">
																目前正式商品狀態：
																<span className="font-medium">
																	{UI_TEXT.actions.formalProductRemoved}
																</span>
															</span>
														) : it.formalProductStatusUnavailable ? (
															<span className="text-[0.6875rem] text-[var(--hkui-default-500)]">
																{UI_TEXT.actions.formalProductStatusUnavailable}
															</span>
														) : it.formalProductStatusLabel ? (
															<span className="text-[0.6875rem] text-[var(--hkui-default-600)]">
																目前正式商品狀態：
																<span className="font-medium text-[var(--hkui-foreground)]">
																	{it.formalProductStatusLabel}
																</span>
															</span>
														) : null}
														{it.formalProductId &&
														!isFormalProductRemoved(it) ? (
															<Button
																size="sm"
																variant="light"
																color="primary"
																href={`/products/${encodeURIComponent(it.formalProductId)}`}
																target="_blank"
																rel="noopener noreferrer"
															>
																{UI_TEXT.actions.openFormalProduct}
															</Button>
														) : (
															<span className="text-[0.6875rem] text-[var(--hkui-default-500)]">
																—
															</span>
														)}
													</div>
												) : (
													<div className="flex justify-end gap-1">
														<Button
															size="sm"
															variant="light"
															color="primary"
															href={`/seller-center/products/import/${jobId}/items/${it.importItemId}/edit`}
															disabled={busy}
														>
															{UI_TEXT.actions.edit}
														</Button>
														<Button
															size="sm"
															variant="light"
															color="primary"
															href={`/seller-product-import-preview/${jobId}/items/${it.importItemId}`}
															target="_blank"
															rel="noopener noreferrer"
															disabled={!canPreviewItem}
															aria-disabled={busy || undefined}
															title={
																!canPreviewItem
																	? "商品有阻擋錯誤或非 READY 狀態"
																	: undefined
															}
														>
															{UI_TEXT.actions.previewItem}
														</Button>
														{it.status === IMPORT_ITEM_STATUS.COMMIT_FAILED ? (
															<Button
																size="sm"
																variant="light"
																color="warning"
																onClick={() => handleRetryItem(it.importItemId)}
																disabled={busy}
																isLoading={
																	pendingAction ===
																	`retry-item:${it.importItemId}`
																}
																loadingLabel="重試商品圖片中…"
															>
																{UI_TEXT.actions.retryItemAssets}
															</Button>
														) : null}
														{isRemoved ? (
															<Button
																size="sm"
																variant="light"
																color="success"
																onClick={() =>
																	handleRestoreItem(it.importItemId)
																}
																disabled={busy}
																isLoading={
																	pendingAction ===
																	`restore-item:${it.importItemId}`
																}
																loadingLabel="恢復匯入商品中…"
															>
																{UI_TEXT.actions.restore}
															</Button>
														) : (
															<Button
																size="sm"
																variant="light"
																color="danger"
																onClick={() =>
																	handleRemoveItem(it.importItemId)
																}
																disabled={busy}
																isLoading={
																	pendingAction ===
																	`remove-item:${it.importItemId}`
																}
																loadingLabel="移除匯入商品中…"
															>
																{UI_TEXT.actions.remove}
															</Button>
														)}
													</div>
												)}
											</td>
										</tr>
									);
								})}
								{itemsLoading && items.length === 0 ? (
									<tr>
										<td
											colSpan={8}
											className="px-3 py-10 text-center text-sm text-[var(--hkui-default-500)]"
										>
											載入中…
										</td>
									</tr>
								) : null}
								{!itemsLoading && items.length === 0 ? (
									<tr>
										<td
											colSpan={8}
											className="px-3 py-10 text-center text-sm text-[var(--hkui-default-500)]"
										>
											沒有符合條件的商品
										</td>
									</tr>
								) : null}
							</tbody>
						</table>
					</div>
					<footer className="border-t border-[var(--hkui-default-100)] bg-[var(--hkui-default-50)] px-4 py-2 text-xs text-[var(--hkui-default-500)]">
						共 {itemsTotal} 筆
					</footer>
				</section>
			</SellerPageShell>

			{/* Modals */}
			<UploadCsvModal
				open={reuploadOpen}
				onClose={() => setReuploadOpen(false)}
				retargetJobId={jobId}
				onSuccess={() => {
					setReuploadOpen(false);
					void refreshAll();
				}}
			/>

			<BatchShippingModal
				open={shippingOpen}
				onClose={() => setShippingOpen(false)}
				jobId={jobId}
				selectedItemIds={selectedBatchEditableIds}
				onSuccess={() => {
					setSelected([]);
					void refreshAll();
				}}
			/>

			<BatchCategoryModal
				open={categoryOpen}
				onClose={() => setCategoryOpen(false)}
				jobId={jobId}
				selectedItemIds={selectedBatchEditableIds}
				onSuccess={() => {
					setSelected([]);
					void refreshAll();
				}}
			/>

			<BatchTargetStatusModal
				open={targetStatusOpen}
				onClose={() => setTargetStatusOpen(false)}
				jobId={jobId}
				selectedItemIds={selectedBatchEditableIds}
				onSuccess={() => {
					setSelected([]);
					void refreshAll();
				}}
			/>

			<Dialog
				open={confirmCommit}
				onClose={() => setConfirmCommit(false)}
				size="sm"
				title={UI_TEXT.commit.confirmTitle}
				footer={
					<div className="flex w-full justify-end gap-2">
						<Button
							variant="light"
							color="default"
							onClick={() => setConfirmCommit(false)}
							disabled={busy}
						>
							取消
						</Button>
						<Button
							color="primary"
							onClick={handleConfirmCommit}
							isLoading={pendingAction === "commit-all"}
							disabled={busy}
							loadingLabel="送出全部商品中…"
						>
							確認 Commit
						</Button>
					</div>
				}
			>
				<p className="text-sm text-[var(--hkui-default-700)]">
					{UI_TEXT.commit.confirmDesc(itemsEligible)}
				</p>
			</Dialog>

			<Dialog
				open={confirmCancel}
				onClose={() => setConfirmCancel(false)}
				size="sm"
				title={UI_TEXT.commit.cancelConfirmTitle}
				footer={
					<div className="flex w-full justify-end gap-2">
						<Button
							variant="light"
							color="default"
							onClick={() => setConfirmCancel(false)}
							disabled={busy}
						>
							返回
						</Button>
						<Button
							color="danger"
							onClick={handleConfirmCancel}
							isLoading={pendingAction === "cancel-job"}
							disabled={busy}
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
			</Dialog>

			<Dialog
				open={confirmCommitSelected}
				onClose={() => setConfirmCommitSelected(false)}
				size="sm"
				title={UI_TEXT.commit.confirmSelectedTitle}
				footer={
					<div className="flex w-full justify-end gap-2">
						<Button
							variant="light"
							color="default"
							onClick={() => setConfirmCommitSelected(false)}
							disabled={busy}
						>
							取消
						</Button>
						<Button
							color="primary"
							onClick={handleConfirmCommitSelected}
							isLoading={pendingAction === "commit-selected"}
							disabled={busy}
							loadingLabel="送出已勾選商品中…"
						>
							確認 Commit
						</Button>
					</div>
				}
			>
				<p className="text-sm text-[var(--hkui-default-700)]">
					{UI_TEXT.commit.confirmSelectedDesc(selectedCommitEligibleIds.length)}
				</p>
			</Dialog>

			<Dialog
				open={commitResult !== null}
				onClose={() => setCommitResult(null)}
				size="md"
				title="Commit 結果"
				footer={
					<div className="flex w-full justify-end">
						<Button color="primary" onClick={() => setCommitResult(null)}>
							了解
						</Button>
					</div>
				}
			>
				{commitResult ? (
					<div className="space-y-3 text-sm text-[var(--hkui-default-700)]">
						<p>
							已送出 Commit：
							<span className="font-semibold text-[var(--hkui-foreground)]">
								{commitResult.queuedCount}
							</span>{" "}
							筆 · 跳過：
							<span className="font-semibold text-[var(--hkui-warning-700)]">
								{commitResult.skippedItemIds?.length ?? 0}
							</span>{" "}
							筆
						</p>
						{commitResult.skipReasons && commitResult.skipReasons.length > 0 ? (
							<section>
								<h4 className="m-0 text-xs font-semibold text-[var(--hkui-default-600)]">
									{UI_TEXT.commit.skipReasonHeading}
								</h4>
								<ul className="mt-1.5 m-0 list-none space-y-1 p-0 text-xs">
									{(() => {
										const grouped = new Map<string, number>();
										for (const r of commitResult.skipReasons ?? []) {
											grouped.set(
												r.reasonCode,
												(grouped.get(r.reasonCode) ?? 0) + 1,
											);
										}
										return Array.from(grouped.entries()).map(
											([reasonCode, count]) => (
												<li key={reasonCode} className="flex gap-2">
													<span className="text-[var(--hkui-warning-700)]">
														{commitSkipReasonLabel(reasonCode)}
													</span>
													<span className="text-[var(--hkui-default-500)]">
														×{count}
													</span>
												</li>
											),
										);
									})()}
								</ul>
							</section>
						) : null}
					</div>
				) : null}
			</Dialog>

			<Dialog
				open={confirmRemove}
				onClose={() => setConfirmRemove(false)}
				size="sm"
				title="確定批次移除？"
				footer={
					<div className="flex w-full justify-end gap-2">
						<Button
							variant="light"
							color="default"
							onClick={() => setConfirmRemove(false)}
							disabled={busy}
						>
							取消
						</Button>
						<Button
							color="danger"
							onClick={handleConfirmRemove}
							isLoading={pendingAction === "remove-selected"}
							disabled={busy}
							loadingLabel="移除已勾選商品中…"
						>
							確認移除
						</Button>
					</div>
				}
			>
				<p className="text-sm text-[var(--hkui-default-700)]">
					將移除 {selectedRemovableIds.length}{" "}
					個商品（可於「顯示已移除」中恢復）。
				</p>
			</Dialog>
		</>
	);
}
