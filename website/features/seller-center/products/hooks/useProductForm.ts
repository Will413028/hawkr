"use client";

/**
 * State machine for the product form. Owns the draft, all field-level
 * mutations, and the submit pipeline; the page component supplies only
 * JSX. Imperative "click this button" handlers (sku CRUD / cert toggle /
 * AI patch / submit) are returned as stable callbacks so sections can be
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
import { resolveCertTypeView } from "@/lib/products/certification-status";
import {
	createProduct,
	fetchProductCertificationApplications,
	fetchSellerProductEdit,
	type ImageGalleryUpdate,
	type ProductCertificationApplication,
	type ProductCertificationCode,
	type ProductCertificationEntry,
	type ProductCreateResult,
	type ProductSku,
	type SellerProductDetail,
	submitProductCertificationApplication,
	updateProduct,
	updateProductStatus,
	withdrawProductCertificationApplication,
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
	toggleCert(code: ProductCertificationCode): void;
	/**
	 * 這個商品的認證申請（19 §9.1）。**只有編輯模式有值**：建立頁還沒有 productId，
	 * 打不到申請端點。空陣列與「還沒載到」在畫面上同義（都不畫徽章），刻意不分開。
	 */
	certApplications: ProductCertificationApplication[];
	/** 送出一筆認證申請。建立模式為 no-op（沒有 productId）。 */
	submitCert(code: ProductCertificationCode, certNo: string): Promise<void>;
	/** 認證送出／撤回進行中的類型碼；元件據此鎖住那一列。 */
	certBusyCode: ProductCertificationCode | null;
	/** 認證操作的錯誤訊息，以類型碼為鍵。 */
	certErrors: Partial<Record<ProductCertificationCode, string>>;
	/** 尚未確認送出結果的認證資料，與商品的公開認證投影分開保存。 */
	certificationDrafts: ProductCertificationEntry[];
	updateCertificationDraft(
		code: ProductCertificationCode,
		certNo: string | null,
	): void;
	/** 建立已成功；後續失敗只能重試認證，不能重新建立商品。 */
	createdProduct: ProductCreateResult | null;
	createdCertificationCount: number;
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
	const [certificationDrafts, setCertificationDrafts] = React.useState<
		ProductCertificationEntry[]
	>([]);
	const [createdProduct, setCreatedProduct] =
		React.useState<ProductCreateResult | null>(null);
	const createdProductRef = React.useRef<ProductCreateResult | null>(null);
	const [createdCertificationCount, setCreatedCertificationCount] =
		React.useState(0);
	const submitInFlight = React.useRef(false);
	const certificationRequestKeys = React.useRef(new Map<string, string>());
	// Signature of the gallery as loaded on edit. Lets submit() detect an
	// untouched gallery and skip re-staging every existing image. `null` until
	// an edit draft hydrates (create flow never sets it), so create / unhydrated
	// submits never count as "unchanged".
	const initialImagesSig = React.useRef<string | null>(null);
	const [certApplications, setCertApplications] = React.useState<
		ProductCertificationApplication[]
	>([]);
	const [certBusyCode, setCertBusyCode] =
		React.useState<ProductCertificationCode | null>(null);
	const [certErrors, setCertErrors] = React.useState<
		Partial<Record<ProductCertificationCode, string>>
	>({});

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

	/**
	 * 認證申請清單。
	 *
	 * ⚠️ 必須與商品 detail **分開抓**：`product.certifications` 自 19 號起是
	 * approved-only 公開投影，賣家剛送出的申請根本不在裡面。只讀商品的話，賣家送完
	 * 回來看不到自己那一筆，會以為沒存到（19 §14.2 的第一個症狀）。
	 */
	const loadCertApplications = React.useCallback(async () => {
		if (mode !== "edit" || !productId) return;
		try {
			setCertApplications(
				await fetchProductCertificationApplications(productId),
			);
		} catch {
			// 徽章載不到不該擋住整張表單；這一格退回「只看得到投影」的舊行為。
			setCertApplications([]);
		}
	}, [mode, productId]);

	React.useEffect(() => {
		void loadCertApplications();
	}, [loadCertApplications]);

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

	const removeCertLocally = React.useCallback(
		(code: ProductCertificationCode) => {
			setDraft((prev) => {
				const list = prev.certifications ?? [];
				if (!list.some((c) => c.code === code)) return prev;
				return {
					...prev,
					certifications: list.filter((c) => c.code !== code),
				};
			});
		},
		[],
	);

	/**
	 * 取消勾選。
	 *
	 * 🔴 編輯模式一律走 withdraw 端點，**不是**把它從商品 payload 拿掉：全量取代的
	 * 「沒送」以 `certification_type` 為粒度，而待審中的申請在那條路徑上是**保留**的
	 * （19 §6.4）——只從 payload 移除的結果是賣家看到勾選消失、申請卻還躺在 ADM 佇列。
	 * 待審與已核准兩筆都要撤（待審的先撤），否則「停止宣稱這個類型」只做了一半。
	 */
	const toggleCert = React.useCallback(
		(code: ProductCertificationCode) => {
			if (mode !== "edit" || !productId) {
				removeCertLocally(code);
				return;
			}
			const view = resolveCertTypeView({
				code,
				mode: "review",
				entry: (draft.certifications ?? []).find((c) => c.code === code),
				applications: certApplications,
			});
			if (view.withdrawable.length === 0) {
				// 申請清單還沒載到（或端點尚未上線）時只剩投影可依據；至少讓這一頁的
				// 送出不再帶著它，行為與本次改動之前一致。
				removeCertLocally(code);
				return;
			}
			setCertBusyCode(code);
			setCertErrors((prev) => ({ ...prev, [code]: undefined }));
			void (async () => {
				try {
					for (const app of view.withdrawable) {
						await withdrawProductCertificationApplication(
							productId,
							app.id,
							app.version,
						);
					}
					await Promise.all([loadCertApplications(), loadDraft()]);
				} catch (e) {
					setCertErrors((prev) => ({
						...prev,
						[code]: e instanceof Error ? e.message : "撤回失敗，請稍後再試",
					}));
				} finally {
					setCertBusyCode(null);
				}
			})();
		},
		[
			certApplications,
			draft.certifications,
			loadCertApplications,
			loadDraft,
			mode,
			productId,
			removeCertLocally,
		],
	);

	/**
	 * 送出一筆認證申請。
	 *
	 * 單筆端點不走剔除語意（19 §6.2.10）：非法輸入會回 typed 業務碼，所以錯誤要顯示
	 * 出來，不能吞掉——吞掉就回到「送出成功但清單沒有這筆」。
	 */
	const submitCert = React.useCallback(
		async (code: ProductCertificationCode, certNo: string) => {
			if (mode !== "edit" || !productId) return;
			const value = certNo.trim();
			if (!value) return;
			setCertBusyCode(code);
			setCertErrors((prev) => ({ ...prev, [code]: undefined }));
			try {
				await submitProductCertificationApplication(productId, {
					certificationType: code,
					certificateNo: value,
				});
				await loadCertApplications();
			} catch (e) {
				setCertErrors((prev) => ({
					...prev,
					[code]: e instanceof Error ? e.message : "送出失敗，請稍後再試",
				}));
			} finally {
				setCertBusyCode(null);
			}
		},
		[loadCertApplications, mode, productId],
	);

	const handleImagesChange = React.useCallback((next: ImageGalleryUpdate) => {
		setDraft((prev) => applyImagesUpdate(prev, next));
	}, []);

	const handleAiPatch = React.useCallback((patch: AiDraftPatch) => {
		setDraft((prev) => applyAiPatch(prev, patch));
	}, []);

	const updateCertificationDraft = React.useCallback(
		(code: ProductCertificationCode, certNo: string | null) => {
			if (
				mode !== "create" ||
				submitInFlight.current ||
				createdProductRef.current
			)
				return;
			setCertificationDrafts((prev) => {
				if (certNo === null) return prev.filter((entry) => entry.code !== code);
				return prev.some((entry) => entry.code === code)
					? prev.map((entry) =>
							entry.code === code ? { code, certNo } : entry,
						)
					: [...prev, { code, certNo }];
			});
			setCertErrors((prev) => ({ ...prev, [code]: undefined }));
		},
		[mode],
	);

	const submit = React.useCallback(
		async (action: ShelfSubmitAction): Promise<boolean> => {
			if (submitInFlight.current) return false;
			const problems = createdProductRef.current ? [] : validateDraft(draft);
			if (mode === "create") {
				const missingNumbers = certificationDrafts.filter(
					(entry) => !entry.certNo?.trim(),
				);
				if (missingNumbers.length > 0) {
					setCertErrors(
						Object.fromEntries(
							missingNumbers.map((entry) => [
								entry.code,
								"請填寫證書編號，或取消選取這項認證",
							]),
						),
					);
					problems.push("請補齊所選認證的證書編號");
				}
			}
			if (problems.length > 0) {
				setErrors(problems);
				return false;
			}
			submitInFlight.current = true;
			setSubmitting(true);
			setErrors([]);
			let succeeded = false;
			try {
				if (!createdProductRef.current) {
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
						// Product writes also have certification-sync semantics. Send no
						// claims here: the explicit endpoint below reports per-item failures.
						const created = await createProduct({
							...body,
							certifications: [],
						});
						createdProductRef.current = created;
						setCreatedProduct(created);
						void invalidateSellerProducts();
					} else if (productId) {
						await updateProduct(productId, body);
						// 後端 updateProduct 忽略 body.status，上下架要另打 PUT /status。
						if (action === "publish") {
							await updateProductStatus(productId, "4004");
						}
					}
				}
				if (mode === "create" && certificationDrafts.length > 0) {
					const id = createdProductRef.current?.id;
					if (!id) {
						setErrors([
							"商品已建立，但未取得商品編號，認證尚未送出。請從商品管理開啟該商品繼續申請。",
						]);
						setSubmitting(false);
						return false;
					}
					const failed: Partial<Record<ProductCertificationCode, string>> = {};
					for (const entry of certificationDrafts) {
						const certificateNo = (entry.certNo ?? "").trim();
						const request = JSON.stringify([entry.code, certificateNo]);
						let idempotencyKey = certificationRequestKeys.current.get(request);
						if (!idempotencyKey) {
							idempotencyKey = `cert-${crypto.randomUUID()}`;
							certificationRequestKeys.current.set(request, idempotencyKey);
						}
						try {
							const application = await submitProductCertificationApplication(
								id,
								{
									certificationType: entry.code,
									certificateNo,
									idempotencyKey,
								},
							);
							if (!application?.id)
								throw new Error("未取得認證申請結果，請重試確認");
							setCertificationDrafts((prev) =>
								prev.filter((pending) => pending.code !== entry.code),
							);
							setCreatedCertificationCount((count) => count + 1);
						} catch (error) {
							failed[entry.code] =
								error instanceof Error
									? error.message
									: "認證申請結果尚未確認，請稍後重試";
						}
					}
					setCertErrors(failed);
					// Some applications may already be approved even when another failed.
					void invalidateSellerProducts();
					if (Object.keys(failed).length > 0) {
						setErrors([
							"商品已建立，部分認證申請結果尚未確認。可重試確認，或前往商品編輯查看。",
						]);
						setSubmitting(false);
						return false;
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
					const id = createdProductRef.current?.id;
					router.push(
						id &&
							(certificationDrafts.length > 0 || createdCertificationCount > 0)
							? `/seller-center/products/${encodeURIComponent(String(id))}/edit`
							: "/seller-center/products",
					);
				}
				succeeded = true;
				return true;
			} catch (e2) {
				setErrors([e2 instanceof Error ? e2.message : "儲存失敗"]);
				setSubmitting(false);
				return false;
			} finally {
				// Keep the guard closed during successful navigation; failures explicitly
				// restore it below so only unfinished applications can be retried.
				submitInFlight.current = succeeded;
			}
		},
		[
			draft,
			mode,
			onMutated,
			productId,
			router,
			invalidateSellerProducts,
			certificationDrafts,
			createdCertificationCount,
		],
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
		toggleCert,
		certApplications,
		submitCert,
		certBusyCode,
		certErrors,
		certificationDrafts,
		updateCertificationDraft,
		createdProduct,
		createdCertificationCount,
		handleImagesChange,
		handleAiPatch,
		submit,
		unshelf,
		reload: loadDraft,
		clearError,
	};
}
