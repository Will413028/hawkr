"use client";

/**
 * 編輯單筆 ImportItem 的頁面殼 — 只負責：
 *   - 載入 / 錯誤 / 找不到狀態的 skeleton
 *   - dirty-on-leave 提示
 *   - 「儲存後返回」/「儲存並下一筆」的 routing（從 listImportItems 找下一個
 *      needs_fix 的 item id）
 *   - 「移除此商品」/「取消」的 toast + 路由
 *   - 預覽列表 / 預覽此商品 的新 tab href（指向新搬到 `/seller-product-import-preview`
 *      底下的 public 預覽路由）
 *
 * 真正的編輯體驗（CategorySelector / DescriptionEditor / ProductImageUploader /
 * 認證人工審核過渡面 / SKU 卡 + 運費試算）由 ImportItemEditForm 組合既有元件呈現。
 */

import { notFound } from "next/navigation";
import * as React from "react";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import ImportItemEditForm from "../components/ImportItemEditForm";
import { useImportItemForm } from "../hooks/useImportItemForm";
import { UI_TEXT } from "../i18n";
import { useProductsImportUiStore } from "../importUiStore";
import { listImportItems, removeImportItems } from "../repo";

interface Props {
	jobId: string;
	itemId: string;
}

export function ItemEditPage({ jobId, itemId }: Props) {
	const router = useObservedRouter("product_import_item_edit");
	const toast = useNotificationToast();
	const form = useImportItemForm({ jobId, itemId });
	const removePendingRef = React.useRef(false);

	// Dirty-guard：close tab / refresh 時提示。
	React.useEffect(() => {
		if (!form.dirty) return;
		const handler = (e: BeforeUnloadEvent) => {
			e.preventDefault();
			e.returnValue = UI_TEXT.leaveEditConfirm;
			return UI_TEXT.leaveEditConfirm;
		};
		window.addEventListener("beforeunload", handler);
		return () => window.removeEventListener("beforeunload", handler);
	}, [form.dirty]);

	const handleSaved = React.useCallback(() => {
		toast.show({ variant: "success", title: "已儲存", durationMs: 3000 });
		// 通知條列頁 refetch（Next.js client cache 不會自動重 mount/重跑 useEffect，
		// 必須以 Zustand tick 顯式觸發）
		useProductsImportUiStore.getState().bumpDataRefreshTick();
		router.push(`/seller-center/products/import/${jobId}`);
	}, [router, toast, jobId]);

	const handleSavedAndNext = React.useCallback(async () => {
		toast.show({ variant: "success", title: "已儲存", durationMs: 2500 });
		useProductsImportUiStore.getState().bumpDataRefreshTick();
		try {
			const r = await listImportItems(jobId, {
				onlyWithErrors: true,
				page: 1,
				pageSize: 50,
			});
			const next = r.records.find((x) => x.importItemId !== itemId);
			if (next) {
				router.push(
					`/seller-center/products/import/${jobId}/items/${next.importItemId}/edit`,
				);
			} else {
				toast.show({
					variant: "default",
					title: "已沒有需修正的商品",
					durationMs: 3000,
				});
				router.push(`/seller-center/products/import/${jobId}`);
			}
		} catch {
			router.push(`/seller-center/products/import/${jobId}`);
		}
	}, [router, toast, jobId, itemId]);

	const handleCancel = React.useCallback(() => {
		if (form.dirty && !window.confirm(UI_TEXT.leaveEditConfirm)) return;
		router.push(`/seller-center/products/import/${jobId}`);
	}, [form.dirty, router, jobId]);

	const handleRemove = React.useCallback(async () => {
		if (removePendingRef.current || !window.confirm("確定移除此商品？")) return;
		removePendingRef.current = true;
		try {
			await removeImportItems(jobId, [itemId]);
			toast.show({ variant: "success", title: "已移除", durationMs: 3000 });
			useProductsImportUiStore.getState().bumpDataRefreshTick();
			router.push(`/seller-center/products/import/${jobId}`);
		} catch (e) {
			toast.show({
				variant: "error",
				title: "移除失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			removePendingRef.current = false;
		}
	}, [jobId, itemId, router, toast]);

	if (form.loading) {
		return (
			<SellerPageShell title="編輯匯入商品">
				<p className="py-12 text-center text-sm text-[var(--hkui-default-500)]">
					載入中…
				</p>
			</SellerPageShell>
		);
	}

	if (form.notFound) notFound();

	if (form.error && !form.item) {
		return (
			<SellerPageShell title="編輯匯入商品">
				<p className="rounded border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] px-3 py-2 text-sm text-[var(--hkui-danger-700)]">
					{form.error}
				</p>
			</SellerPageShell>
		);
	}

	if (!form.item) {
		return (
			<SellerPageShell title="編輯匯入商品">
				<p className="py-12 text-center text-sm text-[var(--hkui-default-500)]">
					找不到商品
				</p>
			</SellerPageShell>
		);
	}

	// 預覽路由為新搬遷的 public 路徑 — 新 tab 開，使用者管理頁不會被取代。
	const previewListHref = `/seller-product-import-preview/${jobId}`;
	const previewItemHref = `/seller-product-import-preview/${jobId}/items/${itemId}`;

	return (
		<ImportItemEditForm
			form={form}
			previewListHref={previewListHref}
			previewItemHref={previewItemHref}
			onSaved={handleSaved}
			onSavedAndNext={handleSavedAndNext}
			onCancel={handleCancel}
			onRemove={handleRemove}
			csvLine={form.item.brief?.productKey ?? ""}
			productKey={form.item.brief?.productKey ?? ""}
		/>
	);
}
