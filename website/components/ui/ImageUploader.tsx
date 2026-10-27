"use client";

import dynamic from "next/dynamic";
import * as React from "react";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import type { CropConfig, CropPreset } from "@/components/ui/ImageCropDialog";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { cn } from "@/lib/utils/cn";

/**
 * Reusable image / document upload tile.
 *
 * Mirrors the Flutter app's upload UX (hawkr_seller `ProfileAvatarSection`
 * + `SellerCertificationUploadScreen`):
 *   - Empty state shows the upload indicator + helper copy in a
 *     dashed-border tile
 *   - Uploading state animates the same indicator with a translucent overlay
 *   - Preview state renders the asset with hover-revealed change / remove
 *     actions
 *
 * Surfaces use design tokens (`--hkui-*`, `--page-uploader-*`), never raw hex,
 * so dark mode and brand re-themes flow through automatically.
 *
 * Structure note: the `<input type="file">` is a SIBLING of the tile, never a
 * child of it — an interactive element nested inside a `<button>` is invalid
 * HTML and the outer `disabled` never reached the inner control. The visible
 * tile is the button, the `<label>` is bound to the input via `htmlFor`, and
 * the button borrows that label through `aria-labelledby`.
 */

/**
 * The crop editor is a ~2.4k-line module that also pulls in react-easy-crop and
 * HeroUI. Loading it statically put all of that in the initial bundle of every
 * page that merely *has* an uploader, so it is code-split and only mounted once
 * a file is actually queued for cropping.
 */
const ImageCropDialog = dynamic(
	() => import("@/components/ui/ImageCropDialog"),
	{ ssr: false, loading: () => null },
);

type Variant = "card" | "avatar" | "cover" | "compact";
type Size = "sm" | "md" | "lg";
type PreviewFit = "cover" | "contain";
type ActionsVariant = "link" | "button";

export interface ImageUploaderProps {
	/** Preview URL — empty string / undefined renders the empty state. */
	value?: string | null;
	onPick: (file: File) => void;
	onClear?: () => void;

	/** Tile shape. `card` = dashed rectangle, `avatar` = circle, `cover`
	 * = seller-background rectangle (4:1), `compact` = smaller card. */
	variant?: Variant;
	size?: Size;

	label?: React.ReactNode;
	hint?: React.ReactNode;
	fileHint?: React.ReactNode;
	error?: React.ReactNode;

	uploading?: boolean;
	/** Optional 0–1 upload progress; renders a thin bar at the bottom. */
	progress?: number;

	disabled?: boolean;
	required?: boolean;
	accept?: string;
	/** Accept multiple files at once. `onPick` is called once per file. */
	multiple?: boolean;

	/** Opt-in crop step: picked images open ImageCropDialog first and
	 * `onPick` receives the cropped File. Non-image files (e.g. PDF)
	 * bypass the dialog untouched. */
	crop?: CropPreset | CropConfig;

	/** When true, the tile takes 100% of its parent's width. */
	fullWidth?: boolean;
	/** CSS aspect-ratio override (e.g. "1 / 1", "4 / 3"). */
	aspect?: string;
	/** How image previews fill the tile. Defaults to cover for existing surfaces. */
	previewFit?: PreviewFit;
	/** Preview action styling. Defaults to link for existing surfaces. */
	actionsVariant?: ActionsVariant;

	className?: string;
	tileStyle?: React.CSSProperties;
	id?: string;
	"aria-label"?: string;
}

const DEFAULT_ACCEPT = "image/*,application/pdf";

/**
 * How long the crop dialog stays mounted after its queue empties. It keeps the
 * last file on screen for `IMAGE_CROP_EXIT_CONTENT_MS` (450ms) so the panel does
 * not blink to an empty loading state while it eases out; unmounting on the
 * same tick as the queue clears would cut that short. Duplicated as a literal
 * rather than imported so the `next/dynamic` split is not defeated by a value
 * import of the crop module.
 */
const CROP_DIALOG_EXIT_MS = 600;

/**
 * Nominal intrinsic size for the preview `<img>`. The element is absolutely
 * positioned and stretched (`inset-0 w-full h-full`), so these attributes never
 * change layout — they only stop the decode from happening at an unknown
 * intrinsic ratio, which is what the `width`/`height`-less version did.
 */
function previewIntrinsicSize(
	variant: Variant,
	size: Size,
): { width: number; height: number } {
	if (variant === "avatar") {
		const side = size === "sm" ? 88 : size === "lg" ? 144 : 120;
		return { width: side, height: side };
	}
	if (variant === "cover") return { width: 480, height: 120 };
	if (variant === "compact") return { width: 240, height: 240 };
	return {
		width: 480,
		height: size === "sm" ? 96 : size === "lg" ? 200 : 140,
	};
}

export function ImageUploader({
	value,
	onPick,
	onClear,
	variant = "card",
	size = "md",
	label,
	hint,
	fileHint = "JPG / PNG / WebP / PDF · 10 MB 以下",
	error,
	uploading = false,
	progress,
	disabled = false,
	required = false,
	accept = DEFAULT_ACCEPT,
	multiple = false,
	crop,
	fullWidth = false,
	aspect,
	previewFit = "cover",
	actionsVariant = "link",
	className = "",
	tileStyle,
	id,
	"aria-label": ariaLabel,
}: ImageUploaderProps) {
	const inputRef = React.useRef<HTMLInputElement>(null);
	const [dragging, setDragging] = React.useState(false);
	const [hovering, setHovering] = React.useState(false);
	const [cropQueue, setCropQueue] = React.useState<File[]>([]);
	const [cropMounted, setCropMounted] = React.useState(false);
	const reactId = React.useId();
	const inputId = `${reactId}-file`;
	const labelId = `${reactId}-label`;
	const fileHintId = `${reactId}-filehint`;
	const messageId = `${reactId}-message`;
	// Animate only when interacting (hover / dragging) or actively uploading;
	// stays paused on initial render so the empty state reads as calm.
	const playing = uploading || dragging || hovering;
	const inert = disabled || uploading;

	// The crop dialog is only mounted once something is actually queued, and it
	// stays mounted just past its own exit animation.
	React.useEffect(() => {
		if (cropQueue.length > 0) {
			setCropMounted(true);
			return;
		}
		if (!cropMounted) return;
		const timer = window.setTimeout(
			() => setCropMounted(false),
			CROP_DIALOG_EXIT_MS,
		);
		return () => window.clearTimeout(timer);
	}, [cropQueue.length, cropMounted]);

	const handleClick = () => {
		if (inert) return;
		inputRef.current?.click();
	};

	const handleFiles = (files: FileList | null | undefined) => {
		if (!files || files.length === 0) return;
		const picked = multiple ? Array.from(files) : files[0] ? [files[0]] : [];
		if (!crop) {
			for (const f of picked) onPick(f);
			return;
		}
		// Images queue through the crop dialog; anything else (PDF documents)
		// has no crop semantics and passes straight through.
		const images = picked.filter((f) => f.type.startsWith("image/"));
		for (const f of picked) {
			if (!f.type.startsWith("image/")) onPick(f);
		}
		if (images.length > 0) setCropQueue(images);
	};

	const onDragOver = (e: React.DragEvent) => {
		if (inert) return;
		e.preventDefault();
		setDragging(true);
	};
	const onDragLeave = (e: React.DragEvent) => {
		e.preventDefault();
		setDragging(false);
	};
	const onDrop = (e: React.DragEvent) => {
		e.preventDefault();
		setDragging(false);
		if (inert) return;
		handleFiles(e.dataTransfer.files);
	};
	// Touch has no hover: mirror the pressed state onto pointerdown so the tile
	// reacts to a finger the way it reacts to a cursor.
	const onPointerDown = (e: React.PointerEvent) => {
		if (inert || e.pointerType === "mouse") return;
		setHovering(true);
	};
	const clearCoarseActive = (e: React.PointerEvent) => {
		if (e.pointerType === "mouse") return;
		setHovering(false);
	};

	const previewUrl = resolveCdnUrl(value);
	const hasValue = Boolean(previewUrl);
	const showUploadIndicator = !hasValue || uploading;
	const clampedProgress =
		typeof progress === "number"
			? Math.min(1, Math.max(0, progress))
			: undefined;

	const rootClasses = cn(
		"flex flex-col gap-2 min-w-0",
		fullWidth && "w-full",
		className,
	);

	const isPdf = hasValue && /\.pdf($|\?)/i.test(previewUrl);
	const resolvedPreviewFit = variant === "avatar" ? "cover" : previewFit;
	const previewImageClasses = cn(
		"absolute inset-0 w-full h-full",
		resolvedPreviewFit === "contain" ? "object-contain p-2" : "object-cover",
		variant === "avatar" && "rounded-full",
	);
	const previewSize = previewIntrinsicSize(variant, size);
	// Spread rather than inline attributes so `<img src={previewUrl}` stays on
	// one line — tests/runtime-media-sinks.test.ts asserts that exact string to
	// prove the preview goes through resolveCdnUrl.
	const previewImgAttrs = {
		width: previewSize.width,
		height: previewSize.height,
		decoding: "async",
		className: previewImageClasses,
	} satisfies React.ImgHTMLAttributes<HTMLImageElement>;
	const actionRowClasses =
		actionsVariant === "button" ? "flex flex-wrap gap-2" : "flex gap-3";
	const changeActionClasses =
		actionsVariant === "button"
			? "inline-flex h-11 items-center justify-center rounded-[var(--radius-pill)] border border-[var(--hkui-divider)] bg-[var(--hkui-content1)] px-4 text-[length:var(--text-xs)] font-semibold text-[var(--hkui-primary)] cursor-pointer transition-[background-color,border-color,color] duration-160 motion-reduce:transition-none enabled:hover:border-[var(--hkui-primary)] enabled:hover:bg-[var(--hkui-primary-50)] disabled:opacity-50 disabled:cursor-not-allowed dark:bg-[var(--hkui-content2)]"
			: "inline-flex min-h-11 items-center bg-transparent border-0 px-1 text-[length:var(--text-xs)] font-semibold text-[var(--hkui-primary)] cursor-pointer enabled:hover:underline disabled:opacity-50 disabled:cursor-not-allowed";
	const clearActionClasses =
		actionsVariant === "button"
			? "inline-flex h-11 items-center justify-center rounded-[var(--radius-pill)] border border-[var(--hkui-divider)] bg-[var(--hkui-content1)] px-4 text-[length:var(--text-xs)] font-semibold text-[var(--hkui-danger-text)] cursor-pointer transition-[background-color,border-color,color] duration-160 motion-reduce:transition-none enabled:hover:border-[var(--hkui-danger)] enabled:hover:bg-[var(--page-uploader-danger-bg)] disabled:opacity-50 disabled:cursor-not-allowed dark:bg-[var(--hkui-content2)]"
			: "inline-flex min-h-11 items-center bg-transparent border-0 px-1 text-[length:var(--text-xs)] font-semibold text-[var(--hkui-danger-text)] cursor-pointer enabled:hover:underline disabled:opacity-50 disabled:cursor-not-allowed";

	const tileClasses = [
		"relative flex items-center justify-center w-full border-[1.5px] border-dashed rounded-[var(--radius-lg)] cursor-pointer overflow-hidden transition-[border-color,background-color,transform] duration-160 ease-out motion-reduce:transition-none font-inherit [-webkit-tap-highlight-color:transparent] focus-visible:outline-2 focus-visible:outline-[var(--hkui-primary)] focus-visible:outline-offset-2",
		// Disabled
		inert ? "cursor-not-allowed opacity-65" : "",
		// Aspect / Size / Shape variants
		variant === "avatar"
			? `w-[120px] h-[120px] min-h-0 rounded-full p-0 ${size === "sm" ? "w-[88px] h-[88px]" : size === "lg" ? "w-[144px] h-[144px]" : ""}`
			: variant === "cover"
				? "aspect-[4/1] min-h-0"
				: variant === "compact"
					? "aspect-square min-h-0 p-3 rounded-[var(--radius-md)]"
					: `min-h-[140px] ${size === "sm" ? "min-h-[96px]" : size === "lg" ? "min-h-[200px]" : ""}`,
		// Filled vs Empty states
		hasValue
			? "border-solid border-[var(--hkui-divider)] bg-[var(--color-neutral-cream)] dark:bg-[var(--hkui-content1)] p-0"
			: "border-[var(--hkui-divider)] bg-[var(--page-uploader-tile-bg)] dark:bg-[var(--hkui-content2)] text-[var(--brand-text-gray)] dark:text-[var(--hkui-foreground-soft)]",
		// Invalid / Error state
		error
			? "border-[var(--hkui-danger)] bg-[var(--page-uploader-danger-bg-alt)]"
			: "",
		// Dragging or Hovering (not disabled)
		!inert && (hovering || dragging)
			? "border-[var(--hkui-primary)] bg-[var(--hkui-primary-50)] text-[var(--brand-text-black)] dark:text-[var(--hkui-primary-700)]"
			: "",
		// Tile content layout stacking direction
		variant === "card" || variant === "cover" ? "flex-col" : "",
	]
		.filter(Boolean)
		.join(" ");

	const describedBy =
		[error || hint ? messageId : "", fileHint ? fileHintId : ""]
			.filter(Boolean)
			.join(" ") || undefined;

	return (
		<div className={rootClasses} data-testid="image-uploader">
			{label && (
				<label
					id={labelId}
					htmlFor={inputId}
					className="inline-flex items-baseline gap-1 text-[0.8125rem] font-semibold text-[var(--brand-text-black)] dark:text-[var(--hkui-foreground)]"
				>
					<span>{label}</span>
					{required && (
						<em
							aria-hidden="true"
							className="text-[var(--hkui-danger)] not-italic font-bold"
						>
							*
						</em>
					)}
				</label>
			)}

			{/* Sibling of the tile, never nested inside it. `tabIndex={-1}` keeps
			    the tile button as the single keyboard stop while the label's
			    `htmlFor` still opens the picker on click. */}
			<input
				ref={inputRef}
				id={inputId}
				type="file"
				accept={accept}
				multiple={multiple}
				disabled={disabled}
				tabIndex={-1}
				aria-label={label ? undefined : (ariaLabel ?? "上傳圖片")}
				className="sr-only"
				onChange={(e) => {
					handleFiles(e.target.files);
					e.target.value = "";
				}}
			/>

			{fileHint && (
				<span id={fileHintId} className="sr-only">
					{fileHint}
				</span>
			)}

			<button
				type="button"
				className={tileClasses}
				onClick={handleClick}
				onDragOver={onDragOver}
				onDragEnter={onDragOver}
				onDragLeave={onDragLeave}
				onDrop={onDrop}
				onMouseEnter={() => setHovering(true)}
				onMouseLeave={() => setHovering(false)}
				onFocus={() => setHovering(true)}
				onBlur={() => setHovering(false)}
				onPointerDown={onPointerDown}
				onPointerUp={clearCoarseActive}
				onPointerCancel={clearCoarseActive}
				// `uploading` must NOT set `disabled`: that drops the element out of
				// the tab order mid-interaction and silently moves focus to <body>.
				// The handlers all guard on `inert` instead.
				disabled={disabled}
				aria-disabled={inert || undefined}
				aria-busy={uploading || undefined}
				aria-invalid={error ? true : undefined}
				aria-describedby={describedBy}
				aria-labelledby={label ? labelId : undefined}
				aria-label={label ? undefined : (ariaLabel ?? "上傳圖片")}
				id={id}
				style={
					aspect || tileStyle
						? ({
								...tileStyle,
								...(aspect ? { aspectRatio: aspect } : {}),
							} as React.CSSProperties)
						: undefined
				}
			>
				{hasValue && !isPdf && (
					// biome-ignore lint/performance/noImgElement: preview can be a blob: URL or remote CDN
					<img src={previewUrl} alt="" {...previewImgAttrs} />
				)}
				{hasValue && isPdf && (
					<div
						className="flex flex-col items-center gap-2 text-[var(--hkui-foreground-soft)]"
						aria-hidden="true"
					>
						<PdfGlyph />
						<small>PDF 已上傳</small>
					</div>
				)}

				{showUploadIndicator && (
					<div
						className={`flex items-center justify-center pointer-events-none text-[var(--hkui-primary)] ${size === "sm" || variant === "avatar" || variant === "compact" ? "w-11 h-11" : "w-16 h-16"} ${hasValue && uploading ? "absolute inset-0 m-auto bg-black/40 rounded-[inherit] text-[var(--color-neutral-cream)]" : ""}`}
						aria-hidden="true"
					>
						<UploadLoadingIndicator
							size={
								size === "sm" || variant === "avatar" || variant === "compact"
									? 44
									: 64
							}
							active={playing}
							style={{ width: "100%", height: "100%" }}
						/>
					</div>
				)}

				{!hasValue && !uploading && (
					<div
						className={`flex flex-col items-center gap-0.5 mt-2 text-center pointer-events-none ${variant === "avatar" || variant === "compact" ? "hidden" : ""}`}
					>
						<strong className="text-[0.8125rem] font-semibold text-[var(--brand-text-black)] dark:text-[var(--hkui-foreground)]">
							{dragging ? "放開上傳" : "點擊或拖曳上傳"}
						</strong>
						<small
							aria-hidden="true"
							className="text-xs text-[var(--brand-text-gray)] dark:text-[var(--hkui-foreground-muted)]"
						>
							{fileHint}
						</small>
					</div>
				)}

				{uploading && (
					<div
						className="absolute inset-0 flex flex-col items-center justify-center gap-2.5 bg-black/45 text-[var(--color-neutral-cream)] text-[0.8125rem] font-semibold rounded-[inherit]"
						role="status"
					>
						<span>上傳中…</span>
						{clampedProgress !== undefined && (
							<div
								className="w-[70%] h-1 rounded-[var(--radius-pill)] bg-white/25 overflow-hidden"
								role="progressbar"
								aria-label="上傳進度"
								aria-valuemin={0}
								aria-valuemax={100}
								aria-valuenow={Math.round(clampedProgress * 100)}
							>
								{/* scaleX instead of width: width animates layout. */}
								<span
									className="block h-full w-full origin-left bg-[var(--hkui-primary)] transition-transform duration-200 ease-out motion-reduce:transition-none"
									style={{ transform: `scaleX(${clampedProgress})` }}
								/>
							</div>
						)}
					</div>
				)}
			</button>

			{hasValue && !uploading && (
				<div
					className={actionRowClasses}
					data-image-uploader-actions={actionsVariant}
				>
					<button
						type="button"
						data-image-uploader-action="change"
						className={changeActionClasses}
						onClick={handleClick}
						disabled={disabled}
					>
						更換
					</button>
					{onClear && (
						<button
							type="button"
							data-image-uploader-action="clear"
							className={clearActionClasses}
							onClick={onClear}
							disabled={disabled}
						>
							移除
						</button>
					)}
				</div>
			)}

			{(error || hint) && (
				<small
					id={messageId}
					className={`text-[length:var(--text-xs)] leading-[1.4] ${error ? "text-[var(--hkui-danger-text)]" : "text-[var(--hkui-foreground-muted)]"}`}
					role={error ? "alert" : undefined}
				>
					{error || hint}
				</small>
			)}

			{crop && cropMounted && (
				<ImageCropDialog
					files={cropQueue}
					crop={crop}
					onCropped={(f) => onPick(f)}
					onClose={() => setCropQueue([])}
				/>
			)}
		</div>
	);
}

function PdfGlyph() {
	return (
		<svg viewBox="0 0 24 24" fill="none" aria-hidden="true" className="w-9 h-9">
			<path
				d="M7 3h7l4 4v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z"
				stroke="currentColor"
				strokeWidth="1.5"
				strokeLinejoin="round"
			/>
			<path d="M14 3v4h4" stroke="currentColor" strokeWidth="1.5" />
			<path
				d="M9 14h1.5a1.5 1.5 0 1 0 0-3H9v6m4-6v6h1.5a1.5 1.5 0 0 0 1.5-1.5V12.5A1.5 1.5 0 0 0 14.5 11H13Z"
				stroke="currentColor"
				strokeWidth="1.3"
				strokeLinejoin="round"
			/>
		</svg>
	);
}

export default ImageUploader;
