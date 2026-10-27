"use client";

import * as React from "react";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { ApiClientError } from "@/lib/api/client";
import {
	fieldLabel,
	UI_TEXT,
	VALIDATION_ERROR_LABEL,
	validationErrorLabel,
} from "../i18n";
import { useProductsImportUiStore } from "../importUiStore";
import {
	type CreateImportJobResponse,
	createImportJob,
	L1ValidationError,
	reuploadImportCsv,
} from "../repo";
import { IMPORT_JOB_STATUS, type ValidationError } from "../types";
import { CsvFormatGuideModal } from "./CsvFormatGuideModal";

interface Props {
	open: boolean;
	onClose: () => void;
	/** 重傳模式：傳入既有 job_id。 */
	retargetJobId?: string;
	/** 成功時呼叫（含 PARSING / CSV_INVALID 兩種 status）。 */
	onSuccess: (resp: CreateImportJobResponse) => void;
}

const TEMPLATE_PATH = "/templates/hawkr_import_template.csv";
const MAX_BYTES = 2 * 1024 * 1024;

export function UploadCsvModal({
	open,
	onClose,
	retargetJobId,
	onSuccess,
}: Props) {
	const [file, setFile] = React.useState<File | null>(null);
	const [dragOver, setDragOver] = React.useState(false);
	const [uploading, setUploading] = React.useState(false);
	const [errors, setErrors] = React.useState<ValidationError[]>([]);
	const [topError, setTopError] = React.useState<string | null>(null);
	const guideOpen = useProductsImportUiStore(
		(state) => state.uploadCsvGuideOpen,
	);
	const setGuideOpen = useProductsImportUiStore(
		(state) => state.setUploadCsvGuideOpen,
	);
	const inputRef = React.useRef<HTMLInputElement>(null);

	React.useEffect(() => {
		if (!open) {
			setFile(null);
			setErrors([]);
			setTopError(null);
			setUploading(false);
		}
	}, [open]);

	const handlePick = (f: File | null) => {
		if (!f) return;
		setErrors([]);
		setTopError(null);
		if (!f.name.toLowerCase().endsWith(".csv")) {
			setTopError("僅接受 .csv 格式檔案");
			return;
		}
		if (f.size > MAX_BYTES) {
			setTopError("檔案過大（請小於 2 MB）");
			return;
		}
		setFile(f);
	};

	const onDrop = (e: React.DragEvent<HTMLElement>) => {
		e.preventDefault();
		setDragOver(false);
		const f = e.dataTransfer.files?.[0];
		if (f) handlePick(f);
	};

	const onSubmit = async () => {
		if (!file) return;
		setUploading(true);
		setErrors([]);
		setTopError(null);
		try {
			const resp = retargetJobId
				? await reuploadImportCsv(retargetJobId, file)
				: await createImportJob({ file });
			// L2 fail：status=CSV_INVALID，這仍算 success，由父層導頁
			if (resp.status === IMPORT_JOB_STATUS.CSV_INVALID) {
				onSuccess(resp);
				return;
			}
			onSuccess(resp);
		} catch (e) {
			if (e instanceof L1ValidationError) {
				setErrors(e.validationErrors);
				setTopError(UI_TEXT.uploadModal.errors);
			} else if (e instanceof ApiClientError && e.status === 429) {
				// 後端 rate-limit（active jobs ≥ 5 或當日 ≥ 10）→ 顯示友善中文
				setTopError(VALIDATION_ERROR_LABEL.PRODUCT_IMPORT_RATE_LIMITED);
			} else if (e instanceof Error) {
				setTopError(e.message);
			} else {
				setTopError("上傳失敗，請稍後再試");
			}
		} finally {
			setUploading(false);
		}
	};

	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="md"
			title={
				retargetJobId ? UI_TEXT.uploadModal.retitle : UI_TEXT.uploadModal.title
			}
			footer={
				<div className="flex w-full justify-end gap-2">
					<Button variant="light" color="default" onClick={onClose}>
						取消
					</Button>
					<Button
						color="primary"
						onClick={onSubmit}
						isLoading={uploading}
						loadingIndicator={<UploadLoadingIndicator size={18} />}
						loadingLabel={UI_TEXT.uploadModal.uploading}
						disabled={!file}
					>
						{UI_TEXT.uploadModal.upload}
					</Button>
				</div>
			}
		>
			<div className="space-y-4">
				{/* 範本下載 + 指南入口 */}
				<div className="rounded border border-[var(--hkui-default-200)] bg-[var(--hkui-default-50)] px-3 py-2 text-sm">
					<div className="flex items-center justify-between">
						<span className="text-[var(--hkui-default-700)]">
							{UI_TEXT.uploadModal.templateHint}
						</span>
						<a
							href={TEMPLATE_PATH}
							download
							className="text-[var(--hkui-primary)] underline"
						>
							{UI_TEXT.downloadTemplate}
						</a>
					</div>
					<div className="mt-1 flex justify-end">
						<button
							type="button"
							onClick={() => setGuideOpen(true)}
							className="text-xs text-[var(--hkui-primary)] underline"
						>
							{UI_TEXT.uploadModal.openGuide}
						</button>
					</div>
				</div>
				{/* 拖拽區 */}
				<button
					type="button"
					onDragOver={(e: React.DragEvent<HTMLButtonElement>) => {
						e.preventDefault();
						setDragOver(true);
					}}
					onDragLeave={() => setDragOver(false)}
					onDrop={onDrop}
					onClick={() => inputRef.current?.click()}
					className={`flex w-full flex-col items-center justify-center rounded border-2 border-dashed px-4 py-10 text-sm transition-colors ${
						dragOver
							? "border-[var(--hkui-primary)] bg-[var(--hkui-primary-50)]"
							: "border-[var(--hkui-default-300)] bg-white hover:border-[var(--hkui-primary)]"
					}`}
				>
					<p className="text-[var(--hkui-default-700)]">
						{UI_TEXT.uploadModal.dragHint}
					</p>
					<p className="mt-1 text-xs text-[var(--hkui-default-500)]">
						{UI_TEXT.uploadModal.accepted}
					</p>
					{file ? (
						<p className="mt-3 font-mono text-xs text-[var(--hkui-primary)]">
							{file.name} ({(file.size / 1024).toFixed(1)} KB)
						</p>
					) : null}
					<input
						ref={inputRef}
						type="file"
						accept=".csv,text/csv"
						className="hidden"
						onChange={(e) => handlePick(e.target.files?.[0] ?? null)}
					/>
				</button>
				{topError ? (
					<p className="rounded border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] px-3 py-2 text-sm text-[var(--hkui-danger-700)]">
						{topError}
					</p>
				) : null}
				{errors.length > 0 ? (
					<>
						<ul className="max-h-56 space-y-1 overflow-y-auto rounded border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] p-3 text-sm">
							{errors.map((e, idx) => (
								<li
									// biome-ignore lint/suspicious/noArrayIndexKey: server payload has no stable id
									key={idx}
									className="flex flex-wrap gap-x-3 text-[var(--hkui-danger-700)]"
								>
									<span className="text-xs font-medium">
										第 {e.csvLine} 行 · {fieldLabel(e.field) || e.field}
									</span>
									<span>{validationErrorLabel(e.errorType, e.message)}</span>
								</li>
							))}
						</ul>
						<div className="flex justify-end">
							<button
								type="button"
								onClick={() => setGuideOpen(true)}
								className="text-xs text-[var(--hkui-primary)] underline"
							>
								{UI_TEXT.uploadModal.openGuideOnError}
							</button>
						</div>
					</>
				) : null}
			</div>
			<CsvFormatGuideModal
				open={guideOpen}
				onClose={() => setGuideOpen(false)}
			/>
		</Dialog>
	);
}
