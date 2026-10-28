"use client";

/**
 * State machine for the product form. Owns the draft, all field-level
 * mutations, and the submit pipeline; the page component supplies only
 * JSX. Imperative "click this button" handlers (sku CRUD / AI patch /
 * submit) are returned as stable callbacks so sections can be
 * memoised cheaply later.
 *
 * @example
 *   const form = useProductForm({ mode, productId });
 *   <input value={form.draft.name} onChange={(e) => form.update({ name: e.target.value })} />
 */

import * as React from "react";
import { useInvalidateSellerProducts } from "@/lib/hooks/useSellerProducts";
import {
	gallerySignature,
	resolveStagedImages,
} from "@/lib/media/resolve-staged-images";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	createProduct,
	fetchSellerProductEdit,
	type ImageGalleryUpdate,
	type ProductSku,
	type SellerProductDetail,
	updateProduct,
	updateProductStatus,
} from "@/lib/products/seller-repo";
import type { AiDraftPatch } from "@/lib/seller/ai-listing-chat";
import {
	applyAiPatch,
	applyImagesUpdate,
	draftToSubmitBody,
	emptyDraft,
	NEW_SKU_TEMPLATE,
	normaliseDraftFromApi,
	type ShelfSubmitAction,
	validateDraft,
} from "../adapters/productFormAdapter";

export interface UseProductFormArgs {
	mode: "create" | "edit";
	productId?: number | string;
	/**
	 * 抽屜模式的統一資料變更通知（{@link DrawerMutation.notify}）：每個成功的
	 * mutation（submit / publish / unshelf）後呼叫，讓父頁即時重新同步清單列 +
	 * 統計卡。**只表「資料變了」，不含關閉抽屜語意** —— 開關由抽屜自己負責，所以
	 * unshelf 這種「就地下架、抽屜不關」的路徑也會正確通知父頁（修 #1247）。
	 *
	 * 不提供 = 全頁模式：submit 成功導向商品列表、unshelf 走整頁 reload。
	 */
	onMutated?: () => void;
}

export interface UseProductFormResult {
	/**
	 * 建立模式先收集認證草稿，取得商品 id 後再送出；編輯模式沿用逐筆申請。
	 */
	mode: "create" | "edit";
	draft: SellerProductDetail;
	loading: boolean;
	submitting: boolean;
	/**
	 * Everything currently blocking a save — empty when there is nothing wrong.
	 * A list, not a string, because `validateDraft` reports every missing field
	 * at once (#435); a request failure lands here as a single entry.
	 */
	errors: string[];
	update(patch: Partial<SellerProductDetail>): void;
	updateSku(index: number, patch: Partial<ProductSku>): void;
	addSku(): void;
	removeSku(index: number): void;
	handleImagesChange(next: ImageGalleryUpdate): void;
	handleAiPatch(patch: AiDraftPatch): void;
	/** @returns 成功為 `true`；抽屜可據此決定是否關閉。 */
	submit(action: ShelfSubmitAction): Promise<boolean>;
	unshelf(): Promise<void>;
	/** Re-fetch the edit draft from the API (no-op in create mode). */
	reload(): Promise<void>;
	clearError(): void;
}

export function useProductForm({
	mode,
	productId,
	onMutated,
}: UseProductFormArgs): UseProductFormResult {
	const router = useObservedRouter("seller_product_form");
	const invalidateSellerProducts = useInvalidateSellerProducts();
	const [draft, setDraft] = React.useState<SellerProductDetail>(emptyDraft());
	const [loading, setLoading] = React.useState(mode === "edit");
	const [submitting, setSubmitting] = React.useState(false);
	const [errors, setErrors] = React.useState<string[]>([]);
	const submitInFlight = React.useRef(false);
	// Signature of the gallery as loaded on edit. Lets submit() detect an
	// untouched gallery and skip re-staging every existing image. `null` until
	// an edit draft hydrates (create flow never sets it), so create / unhydrated
	// submits never count as "unchanged".
	const initialImagesSig = React.useRef<string | null>(null);
	// Hydrate edit drafts from the API; the create flow keeps emptyDraft.
	// Exposed as `reload` so the drawer can re-fetch after an unshelf without a
	// hard page reload.
	const loadDraft = React.useCallback(async () => {
		if (mode !== "edit" || !productId) return;
		setLoading(true);
		try {
			const p = await fetchSellerProductEdit(productId);
			if (p) {
				const next = normaliseDraftFromApi({
					...emptyDraft(),
					...p,
				} as Parameters<typeof normaliseDraftFromApi>[0]);
				initialImagesSig.current = gallerySignature(next.images ?? []);
				setDraft(next);
			}
		} catch (e) {
			setErrors([e instanceof Error ? e.message : "載入失敗"]);
		} finally {
			setLoading(false);
		}
	}, [mode, productId]);

	React.useEffect(() => {
		void loadDraft();
	}, [loadDraft]);

	const update = React.useCallback((patch: Partial<SellerProductDetail>) => {
		setDraft((prev) => ({ ...prev, ...patch }));
	}, []);

	const updateSku = React.useCallback(
		(index: number, patch: Partial<ProductSku>) => {
			setDraft((prev) => ({
				...prev,
				skus: (prev.skus ?? []).map((s, i) =>
					i === index ? { ...s, ...patch } : s,
				),
			}));
		},
		[],
	);

	const addSku = React.useCallback(() => {
		setDraft((prev) => ({
			...prev,
			skus: [...(prev.skus ?? []), { ...NEW_SKU_TEMPLATE }],
		}));
	}, []);

	const removeSku = React.useCallback((index: number) => {
		setDraft((prev) => ({
			...prev,
			skus: (prev.skus ?? []).filter((_, i) => i !== index),
		}));
	}, []);

	const handleImagesChange = React.useCallback((next: ImageGalleryUpdate) => {
		setDraft((prev) => applyImagesUpdate(prev, next));
	}, []);

	const handleAiPatch = React.useCallback((patch: AiDraftPatch) => {
		setDraft((prev) => applyAiPatch(prev, patch));
	}, []);

	const submit = React.useCallback(
		async (action: ShelfSubmitAction): Promise<boolean> => {
			if (submitInFlight.current) return false;
			const problems = validateDraft(draft);
			if (problems.length > 0) {
				setErrors(problems);
				return false;
			}
			submitInFlight.current = true;
			setSubmitting(true);
			setErrors([]);
			let succeeded = false;
			try {
				// Gallery images go out as `{ stagedToken }` (new upload) or
				// `{ assetId }` (an image the product already owns). The backend
				// rebuilds the whole gallery from exactly this array, so anything left
				// out is deleted — `resolveStagedImages` must therefore be handed the
				// complete draft gallery, not just the new entries. Omitting `images`
				// altogether is the one way to say "unchanged": updateProduct only
				// touches the gallery when the field is present, so an untouched
				// gallery skips the array (and any restage work) entirely.
				const galleryUnchanged =
					mode === "edit" &&
					initialImagesSig.current !== null &&
					gallerySignature(draft.images ?? []) === initialImagesSig.current;
				const images = galleryUnchanged
					? undefined
					: await resolveStagedImages(draft.images ?? []);
				// Stock is editable in both create AND edit — the seller can adjust
				// per-SKU 庫存 straight from the product form (the update payload now
				// carries `stock`, instead of deferring solely to 庫存管理).
				const body = draftToSubmitBody(draft, undefined, {
					includeStock: true,
					...(images ? { images } : {}),
				});
				if (mode === "create") {
					await createProduct(body);
					void invalidateSellerProducts();
				} else if (productId) {
					await updateProduct(productId, body);
					// 後端 updateProduct 忽略 body.status，上下架要另打 PUT /status。
					if (action === "publish") {
						await updateProductStatus(productId, "4004");
					}
				}
				// 讓 seller-products query family（Shell 低庫存徽章 + Dashboard 商品數）
				// 失效並重抓。Drawer 路徑靠父頁 onMutated 也會 invalidate，但全頁路徑
				// 只 router.push soft-nav，不清 TanStack cache → 不補這行會停在舊清單
				// （與 Drawer 路徑不對稱）。invalidate 冪等，重複觸發無害。
				void invalidateSellerProducts();
				// Drawer: 通知父頁即時重新同步（清單 + 統計卡）；開關抽屜是抽屜自己的
				// 事（依 truthy 回傳關閉）。Page: 導向商品列表（原行為）。
				if (onMutated) {
					setSubmitting(false);
					onMutated();
				} else {
					router.push("/seller-center/products");
				}
				succeeded = true;
				return true;
			} catch (e2) {
				setErrors([e2 instanceof Error ? e2.message : "儲存失敗"]);
				setSubmitting(false);
				return false;
			} finally {
				// Keep the guard closed during successful navigation.
				submitInFlight.current = succeeded;
			}
		},
		[draft, mode, onMutated, productId, router, invalidateSellerProducts],
	);

	// 上架商品內容不可改，編輯前必須先下架。status-only，不走 updateProduct。
	// Drawer: re-fetch in place so the gate lifts and the form becomes editable
	// without losing the drawer. Page: hard reload (its prior behaviour).
	const unshelf = React.useCallback(async () => {
		if (!productId) return;
		setSubmitting(true);
		setErrors([]);
		try {
			await updateProductStatus(productId, "4003");
			// 就地重抓讓表單解鎖可編輯，並在抽屜情境通知父頁同步清單與統計。
			// 獨立編輯頁也沿用同一路徑，避免整頁 reload 清掉步驟與捲動位置。
			await loadDraft();
			onMutated?.();
			setSubmitting(false);
		} catch (e2) {
			setErrors([e2 instanceof Error ? e2.message : "下架失敗"]);
			setSubmitting(false);
		}
	}, [loadDraft, onMutated, productId]);

	const clearError = React.useCallback(() => setErrors([]), []);

	return {
		mode,
		draft,
		loading,
		submitting,
		errors,
		update,
		updateSku,
		addSku,
		removeSku,
		handleImagesChange,
		handleAiPatch,
		submit,
		unshelf,
		reload: loadDraft,
		clearError,
	};
}
