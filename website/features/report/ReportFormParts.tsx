"use client";

import { Code } from "@connectrpc/connect";
import { submitError } from "@hawkr/api/connect/submit";
import { goTrimmedRuneCount } from "@hawkr/api/text/server-text";
import { useTranslations } from "next-intl";
import * as React from "react";
import { Textarea } from "@/components/ui";
import AfterSalePhotoUploader, {
	type StagedPhoto,
} from "@/features/after-sales/components/AfterSalePhotoUploader";
import { DESCRIPTION_MAX, EVIDENCE_MAX } from "./report-logic";

/**
 * 佐證上傳的狀態：每次開啟表單清空；關閉或重開之前開始的上傳，完成時不寫回（避免舊照片出現在新表單）。
 */
export function useEvidenceUploads(open: boolean) {
	const [photos, setPhotos] = React.useState<StagedPhoto[]>([]);
	const [uploading, setUploading] = React.useState(false);
	const generation = React.useRef(0);
	const uploadGeneration = React.useRef(0);

	React.useEffect(() => {
		generation.current += 1;
		if (open) setPhotos([]);
	}, [open]);

	return {
		photos,
		uploading,
		tokens: photos.map((p) => p.token),
		onChange: (next: StagedPhoto[]): boolean => {
			if (uploadGeneration.current !== generation.current) return false;
			setPhotos(next);
			return true;
		},
		onUploadingChange: (v: boolean) => {
			if (v) uploadGeneration.current = generation.current;
			setUploading(v);
		},
	};
}

export type EvidenceUploads = ReturnType<typeof useEvidenceUploads>;

export function EvidenceField({
	evidence,
	disabled,
	error,
	onChanged,
}: {
	evidence: EvidenceUploads;
	disabled: boolean;
	error?: string;
	onChanged?: () => void;
}) {
	const t = useTranslations("report");
	return (
		<>
			<AfterSalePhotoUploader
				value={evidence.photos}
				onChange={(next) => {
					if (evidence.onChange(next)) onChanged?.();
				}}
				max={EVIDENCE_MAX}
				disabled={disabled}
				label={t("evidence")}
				hint={t("evidenceHint", { max: EVIDENCE_MAX })}
				onUploadingChange={evidence.onUploadingChange}
				error={error}
			/>
			{evidence.uploading ? (
				<p className="m-0 text-xs text-hk-muted" role="status">
					{t("uploadingWait")}
				</p>
			) : null}
		</>
	);
}

/** 說明欄：以字元計長的計數（修剪後，與伺服器一致）。 */
export function DescriptionField({
	value,
	onChange,
	placeholder,
	disabled,
	error,
}: {
	value: string;
	onChange: (next: string) => void;
	placeholder: string;
	disabled: boolean;
	error?: string;
}) {
	const t = useTranslations("report");
	const countId = React.useId();
	return (
		<div className="grid gap-1">
			<Textarea
				label={t("description")}
				variant="bordered"
				size="lg"
				isRequired
				value={value}
				onChange={(e) => onChange(e.target.value)}
				placeholder={placeholder}
				minRows={4}
				aria-describedby={countId}
				disabled={disabled}
				isInvalid={error !== undefined}
				errorMessage={error}
			/>
			<span
				id={countId}
				className="text-right text-xs text-hk-muted tabular-nums"
			>
				{goTrimmedRuneCount(value)}/{DESCRIPTION_MAX}
			</span>
		</div>
	);
}

export interface SubmitFailureView {
	/** 伺服器欄位違規，鍵是欄位路徑。 */
	fields: Record<string, string>;
	/** 表單層的訊息；欄位違規都有對應欄位時為空字串。 */
	message: string;
}

/** 把送出失敗對應到表單：欄位違規顯示在欄位上，沒有任何欄位對得上的才併入表單層訊息。 */
export function submitFailureView(
	e: unknown,
	defaults: Parameters<typeof submitError>[1],
	renderedFields: ReadonlySet<string>,
): SubmitFailureView {
	const failure = submitError(e, defaults);
	if (failure.kind === "fields") {
		const unmatched = !Object.keys(failure.fields).some((k) =>
			renderedFields.has(k),
		);
		return {
			fields: failure.fields,
			message: unmatched
				? Object.values(failure.fields).join("；") ||
					failure.message ||
					defaults(Code.InvalidArgument)
				: (failure.message ?? ""),
		};
	}
	return {
		fields: {},
		message:
			failure.kind === "message" ? failure.message : defaults(Code.Internal),
	};
}
