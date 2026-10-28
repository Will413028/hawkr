"use client";

/**
 * 商品匯入精靈 — 單筆 ImportItem 編輯 form state machine。
 *
 * 設計理念：與 `seller-center/products/hooks/useProductForm.ts` 對齊（draft +
 * 12 個 callback），但把 hydrate / submit 換成 import endpoint，並額外暴露
 * import 場景才有的欄位：
 *   - target_status / setTargetStatus（form `status` 只有 0/1，不含 PENDING_REVIEW）
 *   - allowDuplicate / setAllowDuplicate（has_duplicate_existing 場景才相關）
 *   - errors / warnings（後端解析時產生的阻擋與警告，每次 update 後刷新）
 *   - item（最新一次 fetch / update 後的完整 ImportItemDetail，給 UI 顯示用）
 *
 * **與 useProductForm 的差異**：
 *   - 沒有 `initialImagesSig`/skip-restage — 每次 submit 都把 gallery 整包送回去
 *     （後端 set-pattern 自己 noop 未變更欄位，前端不做這層優化）
 *   - 沒有 AI patch（import 場景 AI 助手不在 wizard 範圍）
 *   - 沒有 mode=create/edit 分歧（只有「編輯既有 import item」）
 *   - 透過 hook 內的 setDraft 把 normalised target_status 與 form status 同步
 */

import * as React from "react";
import { ApiClientError } from "@/lib/api/client";
import type {
	ImageGalleryUpdate,
	ProductSku,
	SellerProductDetail,
} from "@/lib/products/seller-repo";
import {
	draftToImportUpdateInput,
	formStatusToTargetStatus,
	importItemToDraft,
	targetStatusToFormStatus,
} from "../adapters/importItemAdapter";
import { getImportItem, updateImportItem } from "../repo";
import {
	type ImportItemDetail,
	TARGET_STATUS,
	type TargetStatusValue,
} from "../types";

export interface UseImportItemFormArgs {
	jobId: string;
	itemId: string;
}

export interface UseImportItemFormResult {
	/** form draft（SellerProductDetail 形狀，給既有 form 元件 reuse）。 */
	draft: SellerProductDetail;
	/** 最新一筆 ImportItemDetail（errors / warnings / duplicate / status 等 import 元資料）。 */
	item: ImportItemDetail | null;
	loading: boolean;
	submitting: boolean;
	/** 載入 / 儲存錯誤訊息。 */
	error: string | null;
	notFound: boolean;
	/** form draft 自掛載後是否被改過。Page 用來決定 beforeunload 提示。 */
	dirty: boolean;

	/** Import 特有：完整 target_status（含 PENDING_REVIEW）。 */
	targetStatus: TargetStatusValue;
	setTargetStatus(next: TargetStatusValue): void;

	/** has_duplicate_existing 場景的 checkbox。 */
	allowDuplicate: boolean;
	setAllowDuplicate(next: boolean): void;

	/** 與 useProductForm 對齊的 draft 操作。 */
	update(patch: Partial<SellerProductDetail>): void;
	updateSku(index: number, patch: Partial<ProductSku>): void;
	addSku(): void;
	removeSku(index: number): void;
	handleImagesChange(next: ImageGalleryUpdate): void;
	/** 儲存。回傳更新後的 item（成功）或 null（失敗，error 已寫進 state）。 */
	submit(): Promise<ImportItemDetail | null>;
	clearError(): void;
}

const NEW_SKU_TEMPLATE: ProductSku = {
	id: "",
	sku_name: "",
	price: "0",
	original_price: "",
	sale_price: "",
	stock: 0,
	unit: "3301", // WeightUnit.KG
};

export function useImportItemForm({
	jobId,
	itemId,
}: UseImportItemFormArgs): UseImportItemFormResult {
	const [item, setItem] = React.useState<ImportItemDetail | null>(null);
	const [draft, setDraft] = React.useState<SellerProductDetail>(() => ({
		id: 0,
		name: "",
		description: { version: "1.0", blocks: [] },
		main_image: "",
		images: [],
		tags: [],
		origin: "",
		preorder_ship_at: null,
		low_stock_threshold: 5,
		expiry_days: 7,
		max_per_order: 0,
		show_sale_countdown: false,
		status: 0,
		skus: [{ ...NEW_SKU_TEMPLATE }],
	}));
	const [targetStatus, setTargetStatusState] =
		React.useState<TargetStatusValue>(TARGET_STATUS.DRAFT);
	const [allowDuplicate, setAllowDuplicateState] = React.useState(false);
	const [loading, setLoading] = React.useState(true);
	const [submitting, setSubmitting] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const [notFound, setNotFound] = React.useState(false);
	const [dirty, setDirty] = React.useState(false);

	// 一旦 hydrate 過後，後續 setDraft / setTargetStatus / setAllowDuplicate
	// 都標 dirty；hydrate 本身不算。
	const hydratedRef = React.useRef(false);

	// Hydrate
	React.useEffect(() => {
		let cancelled = false;
		setLoading(true);
		setError(null);
		setNotFound(false);
		hydratedRef.current = false;
		getImportItem(jobId, itemId)
			.then((it) => {
				if (cancelled) return;
				setItem(it);
				setDraft(importItemToDraft(it));
				setTargetStatusState(it.targetStatus);
				// Backend response 沒有 allow_duplicate_name；只在 has duplicate 場景才有意義，
				// hook 自己預設 false，submit 時若 brief.hasDuplicateExisting 才送回。
				setAllowDuplicateState(false);
				hydratedRef.current = true;
				setDirty(false);
			})
			.catch((e) => {
				if (cancelled) return;
				if (e instanceof ApiClientError && e.status === 404) {
					setNotFound(true);
				}
				setError(e instanceof Error ? e.message : "載入商品失敗");
			})
			.finally(() => {
				if (cancelled) return;
				setLoading(false);
			});
		return () => {
			cancelled = true;
		};
	}, [jobId, itemId]);

	const markDirty = React.useCallback(() => {
		if (hydratedRef.current) setDirty(true);
	}, []);

	/* ===== draft 操作（與 useProductForm 對齊） ===== */

	const update = React.useCallback(
		(patch: Partial<SellerProductDetail>) => {
			setDraft((prev) => ({ ...prev, ...patch }));
			markDirty();
		},
		[markDirty],
	);

	const updateSku = React.useCallback(
		(index: number, patch: Partial<ProductSku>) => {
			setDraft((prev) => ({
				...prev,
				skus: (prev.skus ?? []).map((s, i) =>
					i === index ? { ...s, ...patch } : s,
				),
			}));
			markDirty();
		},
		[markDirty],
	);

	const addSku = React.useCallback(() => {
		setDraft((prev) => ({
			...prev,
			skus: [...(prev.skus ?? []), { ...NEW_SKU_TEMPLATE }],
		}));
		markDirty();
	}, [markDirty]);

	const removeSku = React.useCallback(
		(index: number) => {
			setDraft((prev) => ({
				...prev,
				skus: (prev.skus ?? []).filter((_, i) => i !== index),
			}));
			markDirty();
		},
		[markDirty],
	);

	const handleImagesChange = React.useCallback(
		(next: ImageGalleryUpdate) => {
			setDraft((prev) => {
				const images =
					typeof next === "function" ? next(prev.images ?? []) : next;
				return {
					...prev,
					images,
					main_image: images[0]?.url ?? prev.main_image,
				};
			});
			markDirty();
		},
		[markDirty],
	);

	/* ===== import 特有 state ===== */

	const setTargetStatus = React.useCallback(
		(next: TargetStatusValue) => {
			setTargetStatusState(next);
			// form status 同步：ACTIVE → 1，其餘 → 0。PENDING_REVIEW 不會改 form status。
			setDraft((prev) => ({ ...prev, status: targetStatusToFormStatus(next) }));
			markDirty();
		},
		[markDirty],
	);

	const setAllowDuplicate = React.useCallback(
		(next: boolean) => {
			setAllowDuplicateState(next);
			markDirty();
		},
		[markDirty],
	);

	/* ===== Submit ===== */

	const submit =
		React.useCallback(async (): Promise<ImportItemDetail | null> => {
			if (!item) {
				setError("尚未載入商品");
				return null;
			}
			// 簡易必填檢查 — 與 useProductForm.validateDraft 等價，但不依賴 form-only
			// 的 product 規則（import 場景容許更寬的草稿狀態，後端 validation 才是
			// 最終真實來源 — submit 後 item.errors 會回傳）。
			const name = (draft.name ?? "").trim();
			if (!name) {
				setError("商品名稱必填");
				return null;
			}
			if (!draft.skus || draft.skus.length === 0) {
				setError("至少要一個 SKU");
				return null;
			}
			// 多 SKU 必須有名稱、單 SKU 可空（後端會 backfill）。
			if (draft.skus.length > 1) {
				for (const sku of draft.skus) {
					if (!sku.sku_name?.trim()) {
						setError("多個規格時，每個規格名稱不可空白");
						return null;
					}
				}
			}
			for (const sku of draft.skus) {
				const price = Number(sku.price ?? 0);
				if (!Number.isFinite(price) || price < 0) {
					setError("規格原價不可為負");
					return null;
				}
				const salePrice = sku.sale_price ? Number(sku.sale_price) : 0;
				if (salePrice > 0 && salePrice >= price) {
					setError(`規格「${sku.sku_name ?? ""}」的售價需小於原價`);
					return null;
				}
			}

			setSubmitting(true);
			setError(null);
			try {
				// 同步 target_status：form status 改成上架/下架時即時推回（PENDING_REVIEW
				// 維持原狀，由 setTargetStatus 控制）。
				const finalTargetStatus =
					targetStatus === TARGET_STATUS.PENDING_REVIEW
						? TARGET_STATUS.PENDING_REVIEW
						: formStatusToTargetStatus(draft.status);

				const input = draftToImportUpdateInput(draft, item, {
					allowDuplicateName: allowDuplicate,
					overrideTargetStatus: finalTargetStatus,
				});
				const updated = await updateImportItem(jobId, itemId, input);
				setItem(updated);
				setDraft(importItemToDraft(updated));
				setTargetStatusState(updated.targetStatus);
				// allowDuplicate 保留 user 當下選擇（backend 不回傳此欄位）。
				setDirty(false);
				return updated;
			} catch (e) {
				setError(e instanceof Error ? e.message : "儲存失敗");
				return null;
			} finally {
				setSubmitting(false);
			}
		}, [draft, item, jobId, itemId, allowDuplicate, targetStatus]);

	const clearError = React.useCallback(() => setError(null), []);

	return {
		draft,
		item,
		loading,
		submitting,
		error,
		notFound,
		dirty,
		targetStatus,
		setTargetStatus,
		allowDuplicate,
		setAllowDuplicate,
		update,
		updateSku,
		addSku,
		removeSku,
		handleImagesChange,
		submit,
		clearError,
	};
}
