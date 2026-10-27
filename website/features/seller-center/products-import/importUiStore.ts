"use client";

import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";
import type { CommitImportJobResponse, ImportItemStatusCode } from "./types";

type ProductsImportPersistedState = {
	includeRemoved: boolean;
	onlyWithErrors: boolean;
	statusFilter: ImportItemStatusCode[];
};

const defaultPersistedState: ProductsImportPersistedState = {
	includeRemoved: false,
	onlyWithErrors: false,
	statusFilter: [],
};

type ProductsImportUiState = ProductsImportPersistedState & {
	batchCategoryOpen: boolean;
	batchShippingOpen: boolean;
	batchTargetStatusOpen: boolean;
	commitResult: CommitImportJobResponse | null;
	confirmCancelOpen: boolean;
	confirmCommitOpen: boolean;
	confirmCommitSelectedOpen: boolean;
	confirmRemoveOpen: boolean;
	csvInvalidExpanded: boolean;
	/**
	 * 編輯 / 批次操作完成後 bump 此版本，JobDetailPage 訂閱以強制 refetch。
	 * 解決：Next.js App Router client cache 讓返回條列頁時不重新 mount，
	 * useEffect 不重跑、resole 舊 state 留下「詳細頁有改、條列頁沒同步」現象。
	 */
	dataRefreshTick: number;
	bumpDataRefreshTick: () => void;
	selectedImportItemIds: Set<string>;
	uploadCsvGuideOpen: boolean;
	uploadCsvOpen: boolean;
	cancelTargetJobId: string | null;
	setBatchCategoryOpen: (open: boolean) => void;
	setBatchShippingOpen: (open: boolean) => void;
	setBatchTargetStatusOpen: (open: boolean) => void;
	setCancelTargetJobId: (id: string | null) => void;
	setCommitResult: (result: CommitImportJobResponse | null) => void;
	setConfirmCancelOpen: (open: boolean) => void;
	setConfirmCommitOpen: (open: boolean) => void;
	setConfirmCommitSelectedOpen: (open: boolean) => void;
	setConfirmRemoveOpen: (open: boolean) => void;
	setCsvInvalidExpanded: (expanded: boolean) => void;
	setIncludeRemoved: (includeRemoved: boolean) => void;
	setOnlyWithErrors: (onlyWithErrors: boolean) => void;
	setSelectedImportItemIds: (ids: Iterable<string>) => void;
	setStatusFilter: (statusFilter: ImportItemStatusCode[]) => void;
	setUploadCsvGuideOpen: (open: boolean) => void;
	setUploadCsvOpen: (open: boolean) => void;
	toggleCsvInvalidExpanded: () => void;
	toggleImportItemSelected: (id: string) => void;
	toggleStatusFilter: (status: ImportItemStatusCode) => void;
};

const emptySelection = () => new Set<string>();

export const useProductsImportUiStore = createLocalPrivateStore<
	ProductsImportUiState,
	ProductsImportPersistedState
>(
	(set, get) => ({
		batchCategoryOpen: false,
		batchShippingOpen: false,
		batchTargetStatusOpen: false,
		cancelTargetJobId: null,
		commitResult: null,
		confirmCancelOpen: false,
		confirmCommitOpen: false,
		confirmCommitSelectedOpen: false,
		confirmRemoveOpen: false,
		csvInvalidExpanded: true,
		dataRefreshTick: 0,
		...defaultPersistedState,
		selectedImportItemIds: emptySelection(),
		uploadCsvGuideOpen: false,
		uploadCsvOpen: false,
		bumpDataRefreshTick: () =>
			set(
				{ dataRefreshTick: get().dataRefreshTick + 1 },
				false,
				"products-import/bumpDataRefreshTick",
			),
		setBatchCategoryOpen: (batchCategoryOpen) =>
			set({ batchCategoryOpen }, false, "products-import/setBatchCategoryOpen"),
		setBatchShippingOpen: (batchShippingOpen) =>
			set({ batchShippingOpen }, false, "products-import/setBatchShippingOpen"),
		setBatchTargetStatusOpen: (batchTargetStatusOpen) =>
			set(
				{ batchTargetStatusOpen },
				false,
				"products-import/setBatchTargetStatusOpen",
			),
		setCancelTargetJobId: (cancelTargetJobId) =>
			set({ cancelTargetJobId }, false, "products-import/setCancelTargetJobId"),
		setCommitResult: (commitResult) =>
			set({ commitResult }, false, "products-import/setCommitResult"),
		setConfirmCancelOpen: (confirmCancelOpen) =>
			set({ confirmCancelOpen }, false, "products-import/setConfirmCancelOpen"),
		setConfirmCommitOpen: (confirmCommitOpen) =>
			set({ confirmCommitOpen }, false, "products-import/setConfirmCommitOpen"),
		setConfirmCommitSelectedOpen: (confirmCommitSelectedOpen) =>
			set(
				{ confirmCommitSelectedOpen },
				false,
				"products-import/setConfirmCommitSelectedOpen",
			),
		setConfirmRemoveOpen: (confirmRemoveOpen) =>
			set({ confirmRemoveOpen }, false, "products-import/setConfirmRemoveOpen"),
		setCsvInvalidExpanded: (csvInvalidExpanded) =>
			set(
				{ csvInvalidExpanded },
				false,
				"products-import/setCsvInvalidExpanded",
			),
		setIncludeRemoved: (includeRemoved) =>
			set({ includeRemoved }, false, "products-import/setIncludeRemoved"),
		setOnlyWithErrors: (onlyWithErrors) =>
			set({ onlyWithErrors }, false, "products-import/setOnlyWithErrors"),
		setSelectedImportItemIds: (ids) =>
			set(
				{ selectedImportItemIds: new Set(ids) },
				false,
				"products-import/setSelectedImportItemIds",
			),
		setStatusFilter: (statusFilter) =>
			set({ statusFilter }, false, "products-import/setStatusFilter"),
		setUploadCsvGuideOpen: (uploadCsvGuideOpen) =>
			set(
				{ uploadCsvGuideOpen },
				false,
				"products-import/setUploadCsvGuideOpen",
			),
		setUploadCsvOpen: (uploadCsvOpen) =>
			set({ uploadCsvOpen }, false, "products-import/setUploadCsvOpen"),
		toggleCsvInvalidExpanded: () =>
			set(
				(state) => ({ csvInvalidExpanded: !state.csvInvalidExpanded }),
				false,
				"products-import/toggleCsvInvalidExpanded",
			),
		toggleImportItemSelected: (id) =>
			set(
				(state) => {
					const selectedImportItemIds = new Set(state.selectedImportItemIds);
					if (selectedImportItemIds.has(id)) {
						selectedImportItemIds.delete(id);
					} else {
						selectedImportItemIds.add(id);
					}
					return { selectedImportItemIds };
				},
				false,
				"products-import/toggleImportItemSelected",
			),
		toggleStatusFilter: (status) =>
			set(
				(state) => ({
					statusFilter: state.statusFilter.includes(status)
						? state.statusFilter.filter((item) => item !== status)
						: [...state.statusFilter, status],
				}),
				false,
				"products-import/toggleStatusFilter",
			),
	}),
	{
		keyName: "productsImportUi",
		devtoolsName: "products-import-ui-store",
		defaults: defaultPersistedState,
		partialize: (state): ProductsImportPersistedState => ({
			includeRemoved: state.includeRemoved,
			onlyWithErrors: state.onlyWithErrors,
			statusFilter: state.statusFilter,
		}),
	},
);
