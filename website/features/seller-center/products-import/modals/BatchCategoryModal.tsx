"use client";

import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import CategorySelector from "@/features/seller/components/CategorySelector";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { UI_TEXT } from "../i18n";
import {
	batchUpdateItemsCategory,
	getCategoryConfirmationToken,
} from "../repo";
import type { CategoryConfirmationToken } from "../types";

interface Props {
	open: boolean;
	onClose: () => void;
	jobId: string;
	selectedItemIds: string[];
	onSuccess: () => void;
}

export function BatchCategoryModal({
	open,
	onClose,
	jobId,
	selectedItemIds,
	onSuccess,
}: Props) {
	const toast = useNotificationToast();
	const [categoryId, setCategoryId] = React.useState<number | undefined>(
		undefined,
	);
	const [tokenInfo, setTokenInfo] =
		React.useState<CategoryConfirmationToken | null>(null);
	const [confirmed, setConfirmed] = React.useState(false);
	const [loadingToken, setLoadingToken] = React.useState(false);
	const [submitting, setSubmitting] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	React.useEffect(() => {
		if (!open) {
			setCategoryId(undefined);
			setTokenInfo(null);
			setConfirmed(false);
			setLoadingToken(false);
			setSubmitting(false);
			setError(null);
		}
	}, [open]);

	// Reset token when category changes; fetch a new one when a leaf is picked.
	React.useEffect(() => {
		if (categoryId === undefined) {
			setTokenInfo(null);
			setConfirmed(false);
			return;
		}
		let cancelled = false;
		setLoadingToken(true);
		setError(null);
		getCategoryConfirmationToken(jobId, String(categoryId))
			.then((info) => {
				if (cancelled) return;
				setTokenInfo(info);
				setConfirmed(false);
			})
			.catch((e) => {
				if (cancelled) return;
				setTokenInfo(null);
				setError(e instanceof Error ? e.message : "取得分類確認碼失敗");
			})
			.finally(() => {
				if (cancelled) return;
				setLoadingToken(false);
			});
		return () => {
			cancelled = true;
		};
	}, [jobId, categoryId]);

	const pathLabel = tokenInfo
		? [tokenInfo.catL1Name, tokenInfo.catL2Name, tokenInfo.catL3Name]
				.filter(Boolean)
				.join(" › ")
		: "";

	const onSubmit = async () => {
		if (
			!tokenInfo ||
			!confirmed ||
			selectedItemIds.length === 0 ||
			categoryId === undefined
		) {
			return;
		}
		setSubmitting(true);
		setError(null);
		try {
			const r = await batchUpdateItemsCategory(jobId, {
				importItemIds: selectedItemIds,
				categoryId: String(categoryId),
				confirmationToken: tokenInfo.confirmationToken,
			});
			toast.show({
				variant: "success",
				title: "已套用商品類別",
				body: `共 ${r.affectedItems} 個商品`,
				durationMs: 4000,
			});
			onSuccess();
			onClose();
		} catch (e) {
			const msg = e instanceof Error ? e.message : "套用失敗";
			// Backend 回 token 失效 / 重複使用 / 不符 → 提示重新選擇
			if (/PRODUCT_IMPORT_CONFIRMATION_TOKEN/.test(msg)) {
				setError(UI_TEXT.categoryModal.tokenExpired);
				setTokenInfo(null);
				setConfirmed(false);
			} else {
				setError(msg);
			}
		} finally {
			setSubmitting(false);
		}
	};

	const canSubmit =
		!!tokenInfo && confirmed && !submitting && selectedItemIds.length > 0;

	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="md"
			title={UI_TEXT.categoryModal.title}
			footer={
				<div className="flex w-full justify-end gap-2">
					<Button variant="light" color="default" onClick={onClose}>
						取消
					</Button>
					<Button
						color="primary"
						onClick={onSubmit}
						isLoading={submitting}
						loadingLabel="套用商品類別中…"
						disabled={!canSubmit}
					>
						套用
					</Button>
				</div>
			}
		>
			<div className="space-y-4">
				<p className="text-sm text-[var(--hkui-default-700)]">
					{UI_TEXT.bulk.selected(selectedItemIds.length)}
				</p>
				<div>
					<CategorySelector value={categoryId} onChange={setCategoryId} />
				</div>
				{loadingToken ? (
					<p className="text-xs text-[var(--hkui-default-500)]">
						取得分類確認碼中…
					</p>
				) : null}
				{tokenInfo ? (
					<div className="rounded border border-[var(--hkui-primary)]/40 bg-[var(--hkui-primary-50)]/40 px-3 py-2 text-sm">
						<p className="font-medium text-[var(--hkui-foreground)]">
							{UI_TEXT.categoryModal.pathLabel}
							<span className="ml-1 font-semibold text-[var(--hkui-primary)]">
								{pathLabel || "（未命名）"}
							</span>
						</p>
						<label className="mt-2 flex items-start gap-2 text-xs text-[var(--hkui-foreground)]">
							<input
								type="checkbox"
								checked={confirmed}
								onChange={(e) => setConfirmed(e.target.checked)}
								className="mt-0.5 size-4 accent-[var(--hkui-primary)]"
							/>
							<span>
								{UI_TEXT.categoryModal.confirmLabel(selectedItemIds.length)}
							</span>
						</label>
					</div>
				) : null}
				{error ? (
					<p className="rounded border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] px-3 py-2 text-sm text-[var(--hkui-danger-700)]">
						{error}
					</p>
				) : null}
			</div>
		</Dialog>
	);
}
