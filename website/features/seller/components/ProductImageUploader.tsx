"use client";

import * as React from "react";
import AiSparkleIcon from "@/components/shared/AiSparkleIcon";
import AppIcon from "@/components/shared/AppIcon";
import { ImageCropDialog } from "@/components/ui/ImageCropDialog";
import { ImageUploader } from "@/components/ui/ImageUploader";
import AiImageEnhanceDialog from "@/features/seller/components/AiImageEnhanceDialog";
import { buildImageUrl } from "@/lib/media/cdn";
import { uploadImageFile } from "@/lib/media/upload";
import type {
	ImageGalleryUpdate,
	ProductImageDraft,
} from "@/lib/products/seller-repo";

interface PendingUpload {
	id: string;
	name: string;
	previewBlob: string; // local objectURL for instant preview
	progress: number; // 0..1
	error?: string;
}

interface Props {
	images: ProductImageDraft[];
	/**
	 * Accepts a replacement array *or* a functional updater. Uploads resolve
	 * asynchronously and out of order, so completion handlers MUST use the
	 * functional form — passing a value computed from the closure-captured
	 * `images` makes overlapping uploads share a stale base and clobber each
	 * other down to a single image (see multi-select regression test).
	 */
	onChange: (images: ImageGalleryUpdate) => void;
	max?: number;
	/** Hint text under the dropzone. */
	hint?: string;
}

/**
 * Multi-image uploader for the seller product form. Uses the backend's
 * presigned-URL flow via lib/media/upload.ts. Supports drag-and-drop,
 * click-to-select, per-file progress, retry on failure, reorder
 * (drag-to-reorder), and remove. The first image is treated as 主圖.
 */
export default function ProductImageUploader({
	images,
	onChange,
	max = 9,
	hint,
}: Props) {
	const [pending, setPending] = React.useState<PendingUpload[]>([]);
	const [globalError, setGlobalError] = React.useState<string | null>(null);
	const [aiSource, setAiSource] = React.useState<Blob | null>(null);
	const [aiOpen, setAiOpen] = React.useState(false);
	const [aiCropQueue, setAiCropQueue] = React.useState<File[]>([]);
	const aiInputRef = React.useRef<HTMLInputElement>(null);

	const remaining = Math.max(0, max - images.length - pending.length);

	const startUploads = React.useCallback(
		(files: File[]) => {
			if (files.length === 0) return;
			setGlobalError(null);

			const slots = max - images.length - pending.length;
			if (slots <= 0) {
				setGlobalError(`已達上限 ${max} 張`);
				return;
			}
			const accepted = files.slice(0, slots);
			if (accepted.length < files.length) {
				setGlobalError(`一次最多上傳 ${slots} 張（已自動忽略多餘檔案）`);
			}

			for (const file of accepted) {
				const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
				const previewBlob = URL.createObjectURL(file);
				setPending((prev) => [
					...prev,
					{ id, name: file.name, previewBlob, progress: 0 },
				]);

				void uploadImageFile(file, (ratio) => {
					setPending((prev) =>
						prev.map((p) => (p.id === id ? { ...p, progress: ratio } : p)),
					);
				})
					.then((asset) => {
						URL.revokeObjectURL(previewBlob);
						setPending((prev) => prev.filter((p) => p.id !== id));
						// Functional updater: append onto the latest gallery, not the
						// stale `images` captured when this upload started. Multiple
						// uploads from one multi-select resolve against a shared closure
						// base otherwise and overwrite each other down to one image.
						onChange((prev) =>
							[
								...prev,
								{ url: asset.previewUrl, stagedToken: asset.stagedToken },
							].slice(0, max),
						);
					})
					.catch((e: unknown) => {
						const msg = e instanceof Error ? e.message : "上傳失敗";
						setPending((prev) =>
							prev.map((p) => (p.id === id ? { ...p, error: msg } : p)),
						);
					});
			}
		},
		[images, onChange, max, pending.length],
	);

	const removePending = (id: string) => {
		setPending((prev) => {
			const target = prev.find((p) => p.id === id);
			if (target) URL.revokeObjectURL(target.previewBlob);
			return prev.filter((p) => p.id !== id);
		});
	};

	const removeImage = (idx: number) => {
		onChange((prev) => prev.filter((_, i) => i !== idx));
	};

	const move = (from: number, to: number) => {
		if (from === to) return;
		onChange((prev) => {
			if (to < 0 || to >= prev.length) return prev;
			const next = prev.slice();
			const [item] = next.splice(from, 1);
			next.splice(to, 0, item);
			return next;
		});
	};

	const setAsMain = (idx: number) => move(idx, 0);

	const handleAiPick = (e: React.ChangeEvent<HTMLInputElement>) => {
		const file = e.target.files?.[0];
		e.target.value = ""; // allow re-picking the same file
		if (!file) return;
		setAiSource(file);
		setAiOpen(true);
	};

	const handleAiApply = (enhanced: Blob) => {
		const file = new File([enhanced], `ai-${Date.now()}.jpg`, {
			type: enhanced.type || "image/jpeg",
		});
		// AI output is not guaranteed to be 4:3. Route it through the exact same
		// full-image-first framing step as a normal pick before staging bytes.
		setAiCropQueue([file]);
	};

	return (
		<div className="img-uploader">
			<ImageUploader
				variant="card"
				size="md"
				accept="image/*"
				multiple
				crop="product"
				value=""
				disabled={remaining === 0}
				onPick={(f) => startUploads([f])}
				hint={
					remaining === 0
						? `已達上限 ${max} 張`
						: hint ||
							`JPEG / PNG / WebP，最大 10 MB，最多 ${max} 張，第 1 張為主圖；預設完整保留原圖`
				}
				fullWidth
			/>

			<div className="mt-2">
				<button
					type="button"
					className="inline-flex items-center gap-1.5 h-9 px-3.5 rounded-full border border-hkui-primary-200 text-sm font-bold text-hkui-primary transition-colors hover:bg-hkui-primary/8 disabled:opacity-50 dark:border-[#f1cd70]/38 dark:bg-[#f1cd70]/10 dark:text-[#f5e9c3] dark:hover:bg-[#f1cd70]/16"
					onClick={() => aiInputRef.current?.click()}
					disabled={remaining === 0}
				>
					<AiSparkleIcon size={16} />
					AI 美化後上傳
				</button>
				<input
					ref={aiInputRef}
					type="file"
					accept="image/*"
					className="hidden"
					onChange={handleAiPick}
				/>
			</div>

			<AiImageEnhanceDialog
				open={aiOpen}
				category="product"
				source={aiSource}
				onApply={handleAiApply}
				onClose={() => {
					setAiOpen(false);
					setAiSource(null);
				}}
			/>
			<ImageCropDialog
				files={aiCropQueue}
				crop="product"
				onCropped={(file) => startUploads([file])}
				onClose={() => setAiCropQueue([])}
			/>

			{globalError && (
				<p className="img-uploader__error" role="alert">
					{globalError}
				</p>
			)}

			{(images.length > 0 || pending.length > 0) && (
				<ol className="img-uploader__grid">
					{images.map((img, i) => (
						<li
							// biome-ignore lint/suspicious/noArrayIndexKey: composite key disambiguates duplicate URLs
							key={`${img.url}-${i}`}
							className={`img-uploader__thumb${i === 0 ? " is-main" : ""}`}
						>
							{/* biome-ignore lint/performance/noImgElement: dynamic CDN URLs without configured next/image domains */}
							<img
								src={buildImageUrl(img.url, "thumb")}
								alt={`第 ${i + 1} 張`}
								loading="lazy"
							/>
							<div className="img-uploader__thumb-overlay">
								{i === 0 ? (
									<span className="img-uploader__badge">主圖</span>
								) : (
									<button
										type="button"
										className="img-uploader__btn"
										onClick={() => setAsMain(i)}
										aria-label="設為主圖"
									>
										設為主圖
									</button>
								)}
								<div className="img-uploader__actions">
									<button
										type="button"
										className="img-uploader__icon-btn"
										onClick={() => move(i, i - 1)}
										disabled={i === 0}
										aria-label="向前"
									>
										<AppIcon name="chevron-left" size={16} />
									</button>
									<button
										type="button"
										className="img-uploader__icon-btn"
										onClick={() => move(i, i + 1)}
										disabled={i === images.length - 1}
										aria-label="向後"
									>
										<AppIcon name="chevron-right" size={16} />
									</button>
									<button
										type="button"
										className="img-uploader__icon-btn img-uploader__icon-btn--danger"
										onClick={() => removeImage(i)}
										aria-label="移除"
									>
										<AppIcon name="delete" size={16} />
									</button>
								</div>
							</div>
						</li>
					))}

					{pending.map((p) => (
						<li
							key={p.id}
							className={`img-uploader__thumb img-uploader__thumb--pending${p.error ? " has-error" : ""}`}
						>
							{/* biome-ignore lint/performance/noImgElement: local objectURL preview for in-flight uploads */}
							<img src={p.previewBlob} alt={p.name} />
							<div className="img-uploader__pending-overlay">
								{p.error ? (
									<>
										<small>{p.error}</small>
										<button
											type="button"
											className="img-uploader__btn"
											onClick={() => removePending(p.id)}
										>
											關閉
										</button>
									</>
								) : (
									<>
										<div
											className="img-uploader__progress"
											role="progressbar"
											aria-label="上傳進度"
											aria-valuenow={Math.round(p.progress * 100)}
											aria-valuemin={0}
											aria-valuemax={100}
										>
											<div
												className="img-uploader__progress-bar"
												style={{ width: `${Math.round(p.progress * 100)}%` }}
											/>
										</div>
										<small>{Math.round(p.progress * 100)}%</small>
									</>
								)}
							</div>
						</li>
					))}
				</ol>
			)}
		</div>
	);
}
