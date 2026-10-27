"use client";

import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import { SHIPPING_TEMP_LABEL, SHIPPING_TYPE_LABEL, UI_TEXT } from "../i18n";
import { batchUpdateItemsShipping } from "../repo";
import {
	type BatchShippingMethod,
	SHIPPING_TEMPERATURE,
	type ShippingTemperatureValue,
} from "../types";

interface Props {
	open: boolean;
	onClose: () => void;
	jobId: string;
	selectedItemIds: string[];
	onSuccess: () => void;
}

type ShippingTypeKey = "TAQBIN" | "SEVEN";

interface RowState {
	enabled: boolean;
	temperature: ShippingTemperatureValue;
	freeThreshold: string;
}

const SHIPPING_TYPES: ShippingTypeKey[] = ["TAQBIN", "SEVEN"];
const TEMP_OPTIONS: ShippingTemperatureValue[] = [
	SHIPPING_TEMPERATURE.AMBIENT,
	SHIPPING_TEMPERATURE.CHILLED,
	SHIPPING_TEMPERATURE.FROZEN,
];

function initialRow(): RowState {
	return {
		enabled: false,
		temperature: SHIPPING_TEMPERATURE.AMBIENT,
		freeThreshold: "",
	};
}

export function BatchShippingModal({
	open,
	onClose,
	jobId,
	selectedItemIds,
	onSuccess,
}: Props) {
	const toast = useNotificationToast();
	const [rows, setRows] = React.useState<Record<ShippingTypeKey, RowState>>({
		TAQBIN: initialRow(),
		SEVEN: initialRow(),
	});
	const [submitting, setSubmitting] = React.useState(false);

	React.useEffect(() => {
		if (!open) {
			setRows({
				TAQBIN: initialRow(),
				SEVEN: initialRow(),
			});
			setSubmitting(false);
		}
	}, [open]);

	const patch = (key: ShippingTypeKey, p: Partial<RowState>) => {
		setRows((prev) => ({ ...prev, [key]: { ...prev[key], ...p } }));
	};

	const enabledCount = SHIPPING_TYPES.filter((k) => rows[k].enabled).length;
	const disabled = enabledCount === 0 || selectedItemIds.length === 0;

	const onSubmit = async () => {
		const methods: BatchShippingMethod[] = SHIPPING_TYPES.filter(
			(k) => rows[k].enabled,
		).map((k) => {
			const r = rows[k];
			const m: BatchShippingMethod = { type: k, temperature: r.temperature };
			const trimmed = r.freeThreshold.trim();
			if (trimmed !== "") m.freeThreshold = trimmed;
			return m;
		});
		if (methods.length === 0) return;
		setSubmitting(true);
		try {
			const resp = await batchUpdateItemsShipping(jobId, {
				importItemIds: selectedItemIds,
				shippingMethods: methods,
			});
			toast.show({
				variant: "success",
				title: "已套用運送設定",
				body: `已套用至 ${resp.affectedItems} 個商品 / ${resp.affectedSkus} 個 SKU`,
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
			size="lg"
			title={UI_TEXT.shippingModal.title}
			footer={
				<div className="flex w-full justify-end gap-2">
					<Button variant="light" color="default" onClick={onClose}>
						取消
					</Button>
					<Button
						color="primary"
						onClick={onSubmit}
						isLoading={submitting}
						loadingLabel="套用運送設定中…"
						disabled={disabled}
						title={
							enabledCount === 0 ? UI_TEXT.shippingModal.atLeastOne : undefined
						}
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
				<p className="rounded border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] px-3 py-2 text-sm text-[var(--hkui-danger-700)]">
					{UI_TEXT.shippingModal.warning}
				</p>
				<div className="space-y-3">
					{SHIPPING_TYPES.map((k) => {
						const r = rows[k];
						return (
							<section
								key={k}
								className={`rounded border p-3 transition-colors ${
									r.enabled
										? "border-[var(--hkui-primary)] bg-[var(--hkui-primary-50)]/30"
										: "border-[var(--hkui-default-200)]"
								}`}
							>
								<label className="flex items-center gap-2 text-sm font-medium text-[var(--hkui-foreground)]">
									<input
										type="checkbox"
										checked={r.enabled}
										onChange={(e) => patch(k, { enabled: e.target.checked })}
										className="size-4 accent-[var(--hkui-primary)]"
									/>
									<span>套用此運送方式：{SHIPPING_TYPE_LABEL[k]}</span>
								</label>
								{r.enabled ? (
									<div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
										<label className="space-y-1 text-xs">
											<span className="block text-[var(--hkui-default-600)]">
												溫層
											</span>
											<select
												value={r.temperature}
												onChange={(e) =>
													patch(k, {
														temperature: e.target
															.value as ShippingTemperatureValue,
													})
												}
												className="h-9 w-full rounded border border-[var(--hkui-default-300)] bg-white px-2 text-sm"
											>
												{TEMP_OPTIONS.map((t) => (
													<option key={t} value={t}>
														{SHIPPING_TEMP_LABEL[t]}
													</option>
												))}
											</select>
										</label>
										<label className="space-y-1 text-xs">
											<span className="block text-[var(--hkui-default-600)]">
												{UI_TEXT.shippingModal.freeThresholdLabel}
											</span>
											<input
												type="number"
												min={0}
												inputMode="numeric"
												value={r.freeThreshold}
												onChange={(e) =>
													patch(k, { freeThreshold: e.target.value })
												}
												placeholder="例如 800"
												className="h-9 w-full rounded border border-[var(--hkui-default-300)] bg-white px-2 text-sm"
											/>
											<span className="block text-[var(--hkui-default-500)]">
												{UI_TEXT.shippingModal.freeThresholdHint}
											</span>
										</label>
									</div>
								) : null}
								{/* TODO: 接 useSellerShippingMethods hook 後再判斷此配送方式是否「未開通」並 disable */}
							</section>
						);
					})}
				</div>
			</div>
		</Dialog>
	);
}
