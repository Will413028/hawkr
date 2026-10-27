"use client";

import * as React from "react";
import { ImageUploader } from "@/components/ui/ImageUploader";
import { uploadImageFile } from "@/lib/media/upload";

export interface StagedPhoto {
	token: string;
	previewUrl: string;
}

/** 上傳一張照片並回傳可送出的 token 與縮圖網址；預設是公開上傳。 */
export type PhotoUpload = (
	file: File,
	onProgress: (ratio: number) => void,
) => Promise<StagedPhoto>;

const uploadPublicPhoto: PhotoUpload = async (file, onProgress) => {
	const asset = await uploadImageFile(file, onProgress);
	return { token: asset.stagedToken, previewUrl: asset.previewUrl };
};

interface Props {
	value: StagedPhoto[];
	onChange: (next: StagedPhoto[]) => void;
	max?: number;
	disabled?: boolean;
	label?: React.ReactNode;
	hint?: React.ReactNode;
	upload?: PhotoUpload;
	/** 伺服器指出某張照片有問題：以 staged token 為 key，標在該張照片上；照片被移除後自然不再顯示。 */
	itemErrors?: Record<string, string>;
	/** 整個佐證欄位的錯誤（不指向哪一張）。 */
	error?: string;
}

/**
 * Up-to-N photo uploader built on the project's staged-token flow
 * (`uploadImageFile` → 3-step signed upload). Renders a thumb grid plus one
 * empty slot until `max` is reached. Used by the buyer return form, the
 * seller inspection-deduction dialog and the seller appeal form.
 */
export function AfterSalePhotoUploader({
	value,
	onChange,
	max = 3,
	disabled = false,
	label,
	hint,
	upload = uploadPublicPhoto,
	itemErrors,
	error,
}: Props) {
	const [uploading, setUploading] = React.useState(false);
	const [progress, setProgress] = React.useState(0);
	const [uploadError, setUploadError] = React.useState<string | null>(null);

	const handlePick = async (file: File) => {
		if (value.length >= max) return;
		setUploading(true);
		setProgress(0);
		setUploadError(null);
		try {
			onChange([...value, await upload(file, setProgress)]);
		} catch (e) {
			setUploadError(e instanceof Error ? e.message : "上傳失敗，請再試一次");
		} finally {
			setUploading(false);
			setProgress(0);
		}
	};

	const removeAt = (idx: number) => {
		onChange(value.filter((_, i) => i !== idx));
	};

	const canAddMore = value.length < max && !disabled;

	return (
		<div className="flex flex-col gap-2">
			{label && (
				<span className="text-sm font-semibold text-foreground">{label}</span>
			)}
			<div className="flex flex-wrap gap-3">
				{value.map((photo, idx) => (
					<ImageUploader
						key={photo.token}
						variant="compact"
						size="md"
						value={photo.previewUrl}
						onPick={() => {}}
						onClear={() => removeAt(idx)}
						disabled={disabled}
						error={itemErrors?.[photo.token]}
						aria-label={`已上傳照片 ${idx + 1}`}
					/>
				))}
				{canAddMore && (
					<ImageUploader
						variant="compact"
						size="md"
						value={null}
						onPick={handlePick}
						uploading={uploading}
						progress={progress}
						accept="image/*"
						crop="photo"
						aria-label="新增照片"
					/>
				)}
			</div>
			<div className="flex items-center justify-between">
				<span className="text-xs text-foreground-soft">
					{hint ?? `最多 ${max} 張照片`}
				</span>
				<span className="text-xs text-foreground-soft">
					{value.length} / {max}
				</span>
			</div>
			{uploadError && (
				<p className="text-xs text-danger-700 font-semibold" role="alert">
					{uploadError}
				</p>
			)}
			{error && (
				<p className="text-xs text-danger-700 font-semibold" role="alert">
					{error}
				</p>
			)}
		</div>
	);
}

export default AfterSalePhotoUploader;
