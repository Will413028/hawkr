"use client";

import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { TARGET_STATUS_LABEL, UI_TEXT } from "../i18n";
import { batchUpdateItemsTargetStatus } from "../repo";
import { TARGET_STATUS, type TargetStatusValue } from "../types";

interface Props {
	open: boolean;
	onClose: () => void;
	jobId: string;
	selectedItemIds: string[];
	onSuccess: () => void;
}

const OPTIONS: TargetStatusValue[] = [
	TARGET_STATUS.ACTIVE,
	TARGET_STATUS.DRAFT,
];

export function BatchTargetStatusModal({
	open,
	onClose,
	jobId,
	selectedItemIds,
	onSuccess,
}: Props) {
	const toast = useNotificationToast();
	const [status, setStatus] = React.useState<TargetStatusValue>(
		TARGET_STATUS.ACTIVE,
	);
	const [submitting, setSubmitting] = React.useState(false);

	React.useEffect(() => {
		if (!open) {
			setStatus(TARGET_STATUS.ACTIVE);
			setSubmitting(false);
		}
	}, [open]);

	const onSubmit = async () => {
		if (selectedItemIds.length === 0) return;
		setSubmitting(true);
		try {
			const r = await batchUpdateItemsTargetStatus(jobId, {
				importItemIds: selectedItemIds,
				targetStatus: status,
			});
			toast.show({
				variant: "success",
				title: "已套用上架狀態",
				body: `共 ${r.affectedItems} 個商品`,
				durationMs: 4000,
			});
			onSuccess();
			onClose();
		} catch (e) {
			toast.show({
				variant: "error",
				title: "套用失敗",
				body: e instanceof Error ? e.message : "請稍後再試",
				durationMs: 5000,
			});
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="sm"
			title={UI_TEXT.targetStatusModal.title}
			footer={
				<div className="flex w-full justify-end gap-2">
					<Button variant="light" color="default" onClick={onClose}>
						取消
					</Button>
					<Button
						color="primary"
						onClick={onSubmit}
						isLoading={submitting}
						loadingLabel="套用上架狀態中…"
						disabled={selectedItemIds.length === 0}
					>
						套用
					</Button>
				</div>
			}
		>
			<p className="text-sm text-[var(--hkui-default-700)]">
				{UI_TEXT.bulk.selected(selectedItemIds.length)}
			</p>
			<p className="mt-1 text-xs text-[var(--hkui-default-500)]">
				{UI_TEXT.targetStatusModal.hint}
			</p>
			<div className="mt-4 space-y-2">
				{OPTIONS.map((s) => (
					<label
						key={s}
						className={`flex cursor-pointer items-center gap-3 rounded border px-3 py-2 transition-colors ${
							status === s
								? "border-[var(--hkui-primary)] bg-[var(--hkui-primary-50)]"
								: "border-[var(--hkui-default-200)] hover:bg-[var(--hkui-default-50)]"
						}`}
					>
						<input
							type="radio"
							name="target_status"
							value={s}
							checked={status === s}
							onChange={() => setStatus(s)}
							className="size-4 accent-[var(--hkui-primary)]"
						/>
						<span className="text-sm font-medium text-[var(--hkui-foreground)]">
							{TARGET_STATUS_LABEL[s]}
						</span>
					</label>
				))}
			</div>
		</Dialog>
	);
}
