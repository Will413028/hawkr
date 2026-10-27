"use client";

import * as React from "react";
import { Dialog, Input, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import type { StagedPhoto } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { AfterSalePhotoUploader } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { confirmReceipt } from "@/lib/after-sales/seller-repo";

interface Props {
	open: boolean;
	caseNo: string;
	/** Resolved/refundable amount — the default full-refund figure. */
	refundableAmount: string;
	onClose: () => void;
	onDone: () => void | Promise<void>;
}

/**
 * Seller 確認收貨 / 驗貨扣款 dialog. Default mode refunds the full agreed
 * amount; switching to 扣款 reveals a deduction reason (required) + inspection
 * note/photos that the BUYER will see, so the seller is warned to be specific.
 */
export function AfterSaleConfirmReceiptDialog({
	open,
	caseNo,
	refundableAmount,
	onClose,
	onDone,
}: Props) {
	const max = Number(refundableAmount || 0);
	const [deduct, setDeduct] = React.useState(false);
	const [amount, setAmount] = React.useState(String(max));
	const [deductionReason, setDeductionReason] = React.useState("");
	const [note, setNote] = React.useState("");
	const [photos, setPhotos] = React.useState<StagedPhoto[]>([]);
	const [busy, setBusy] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	React.useEffect(() => {
		if (open) {
			setDeduct(false);
			setAmount(String(max));
			setDeductionReason("");
			setNote("");
			setPhotos([]);
			setError(null);
		}
	}, [open, max]);

	const actual = Number(amount || 0);
	const isDeducting = deduct && actual < max;

	const handleSubmit = async () => {
		if (Number.isNaN(actual) || actual < 0 || actual > max) {
			setError(`實際退款金額需介於 0 ～ ${max.toLocaleString()}`);
			return;
		}
		if (isDeducting && deductionReason.trim().length === 0) {
			setError("扣款時必須填寫扣款原因，買家會看到此說明");
			return;
		}
		setBusy(true);
		setError(null);
		try {
			await confirmReceipt({
				caseNo,
				actualRefundAmount: String(actual),
				deductionReason: isDeducting ? deductionReason.trim() : undefined,
				inspectionNote: note.trim() || undefined,
				inspectionPhotoTokens: photos.map((p) => p.token),
			});
			await onDone();
			onClose();
		} catch (e) {
			setError(e instanceof Error ? e.message : "確認收貨失敗");
		} finally {
			setBusy(false);
		}
	};

	return (
		<Dialog
			open={open}
			onClose={onClose}
			title="確認收貨 / 驗貨"
			size="md"
			footer={
				<>
					<Button variant="bordered" color="default" onClick={onClose}>
						取消
					</Button>
					<Button
						color={isDeducting ? "warning" : "primary"}
						disabled={busy}
						isLoading={busy}
						loadingLabel={isDeducting ? "處理扣款退款中" : "處理全額退款中"}
						onClick={handleSubmit}
					>
						{isDeducting ? "確認驗貨並扣款退款" : "確認收貨並退款"}
					</Button>
				</>
			}
		>
			<p className="text-sm text-foreground-soft mb-3">
				確認已收到買家退回的商品。預設全額退款 NT$ {max.toLocaleString()}
				，如商品有損可切換為扣款並說明原因。
			</p>

			<label className="flex items-center gap-2 mb-3 text-sm cursor-pointer">
				<input
					type="checkbox"
					checked={deduct}
					className="w-4 h-4 accent-hkui-primary"
					onChange={(e) => {
						setDeduct(e.target.checked);
						if (!e.target.checked) setAmount(String(max));
					}}
				/>
				<span className="font-semibold text-foreground">
					驗貨扣款（退款少於原金額）
				</span>
			</label>

			{deduct ? (
				<div className="flex flex-col gap-3">
					<Input
						type="number"
						label="實際退款金額"
						value={amount}
						min={0}
						max={max}
						onChange={(e) => setAmount(e.target.value)}
						description={`可退上限 NT$ ${max.toLocaleString()}`}
					/>
					<Textarea
						label="扣款原因"
						variant="bordered"
						value={deductionReason}
						isRequired={isDeducting}
						onChange={(e) => setDeductionReason(e.target.value.slice(0, 200))}
						placeholder="例如：退回商品外盒破損、缺少配件，扣除 NT$ 100。"
						minRows={2}
						maxLength={200}
					/>
					{isDeducting ? (
						<p className="text-xs text-warning-700 bg-warning-50 rounded-hkui-md px-3 py-2">
							買家會看到扣款金額、扣款原因與驗貨照片，請據實說明。
						</p>
					) : null}
				</div>
			) : null}

			<div className="mt-3">
				<Textarea
					label="驗貨備註（選填）"
					variant="bordered"
					value={note}
					onChange={(e) => setNote(e.target.value.slice(0, 200))}
					placeholder="驗貨情形備註"
					minRows={2}
					maxLength={200}
				/>
			</div>

			<div className="mt-3">
				<AfterSalePhotoUploader
					value={photos}
					onChange={setPhotos}
					max={3}
					disabled={busy}
					label="驗貨照片（選填）"
				/>
			</div>

			{error ? (
				<p className="mt-3 text-sm text-danger-700 font-semibold" role="alert">
					{error}
				</p>
			) : null}
		</Dialog>
	);
}

export default AfterSaleConfirmReceiptDialog;
