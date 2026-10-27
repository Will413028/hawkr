"use client";

import {
	ColorSwatch,
	ColorSwatchPicker,
	parseColor,
	Slider,
	ToggleButton,
	ToggleButtonGroup,
} from "@heroui/react";
import * as React from "react";
import type { Area, Point, Size } from "react-easy-crop";
import Cropper from "react-easy-crop";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import GifEditControls, { useGifEditor } from "@/components/ui/GifEditControls";
import { Input } from "@/components/ui/Input";
import {
	type AlphaAnalysis,
	AVATAR_BACKGROUND_PRESETS,
	AVATAR_DEFAULT_BACKGROUND,
	AVATAR_LOGO_DEFAULT_TRANSFORM,
	AVATAR_LOGO_MIN_SCALE,
	AVATAR_OUTPUT,
	type AvatarBackground,
	type AvatarFitMode,
	type AvatarLogoTransform,
	analyzeImageAlpha,
	calculateAvatarLogoMaxScale,
	constrainAvatarLogoTransform,
	createAvatarFile,
	drawAvatarImage,
	shouldUseAvatarLogoFit,
} from "@/lib/media/avatar-logo";
import {
	type CropAreaPixels,
	calculateContainedOutputSize,
	calculateCropContainRect,
	cropImageFile,
	loadImageElement,
} from "@/lib/media/crop";
import { isGifUploadFile } from "@/lib/media/gif";
import {
	calculateProductImageContainZoom,
	calculateProductImageOutputSize,
	createProductImageFile,
	drawProductImage,
	PRODUCT_IMAGE_ASPECT,
	PRODUCT_IMAGE_BACKGROUND,
	PRODUCT_IMAGE_DEFAULT_TRANSFORM,
	PRODUCT_IMAGE_MAX_SCALE,
	PRODUCT_IMAGE_MIN_SCALE,
	type ProductImageTransform,
	productImageTransformWillCrop,
} from "@/lib/media/product-image";
import {
	calculateCoverCrop,
	createSellerCoverFile,
	drawSellerCover,
	SELLER_COVER_ASPECT,
	SELLER_COVER_BACKGROUND_PRESETS,
	SELLER_COVER_DEFAULT_BACKGROUND,
	SELLER_COVER_OUTPUT,
	type SellerCoverFillMode,
	shouldContainSellerCover,
} from "@/lib/media/seller-cover";
import { cn } from "@/lib/utils/cn";

/**
 * Instagram-style crop step for photo uploads.
 *
 * The crop frame stays fixed while the photo pans (drag) and zooms (wheel /
 * pinch / slider); the dimmed area outside the frame is the part of the
 * upload that will not be shown. A preview strip underneath renders the
 * same crop onto every real surface it will appear on (product card 4:3,
 * PDP 1:1, circular avatars, …) so users see the final visible range
 * before the file is uploaded.
 *
 * Feed it a queue of Files: each confirmed (or skipped) file is emitted
 * through `onCropped` and rides the existing staged-upload pipeline —
 * no backend changes involved. Closing mid-queue drops the remaining files.
 *
 * PROPOSED (needs the owner's sign-off) — split this module.
 * One ~2.4k-line component currently switches between FIVE editors through
 * four booleans on the crop config (`postEditor` / `sellerCoverEditor` /
 * `productEditor` / `avatarEditor` / generic), and the render is a five-way
 * nested ternary. Every consumer pays for all five plus react-easy-crop and
 * the HeroUI colour pickers. Suggested shape, no behaviour change:
 *   components/ui/image-crop/
 *     ImageCropDialog.tsx        — queue, Dialog shell, confirm / skip / close,
 *                                  processing + error state, editor selection
 *     useCropQueue.ts            — file queue, decode, natural size, cropReady
 *     editors/PostImageEditor.tsx        (ratio band, custom W:H, GIF branch)
 *     editors/SellerCoverEditor.tsx      (fill mode, background colour)
 *     editors/ProductImageEditor.tsx     (contain zoom, pan transform)
 *     editors/AvatarEditor.tsx           (fit switch, logo drag, backgrounds)
 *     editors/GenericCropEditor.tsx      (plain Cropper + zoom)
 *     SurfacePreviewStrip.tsx    — the shared canvas preview row
 * Each editor is then `next/dynamic`-loaded from the shell so a page that only
 * uploads avatars never ships the post/GIF path. Out of scope for this pass:
 * the five editors share ~15 pieces of state and the preview-draw effect, so
 * the split needs its own review rather than riding an a11y/token sweep.
 */

export interface CropSurfacePreview {
	label: string;
	/** Aspect ratio of the destination surface (w / h). */
	aspect: number;
	/** How the final surface places the image. Defaults to cover. */
	fit?: "cover" | "contain";
	/** Rendered preview height in px. Defaults to 84. */
	size?: number;
	circle?: boolean;
}

export interface CropAspectOption {
	label: string;
	/** Frame aspect (w / h). */
	aspect: number;
}

/** Canonical frame shared by the editor viewport and the feed-card preview. */
export const POST_FEED_ASPECT = 4 / 3;

/**
 * Feed-post ratio band supported by Hawkr's post editor. The upload keeps
 * the shape the seller chooses while the surface preview shows how the fixed
 * 4:3 feed card will frame it. Ordered tall → wide for predictable scanning.
 */
export const POST_ASPECT_OPTIONS: ReadonlyArray<CropAspectOption> = [
	{ label: "4:5 直式", aspect: 4 / 5 },
	{ label: "1:1 方形", aspect: 1 },
	{ label: "4:3 橫式", aspect: POST_FEED_ASPECT },
	{ label: "1.91:1 寬幅", aspect: 1.91 },
];

export const POST_MIN_ASPECT = 4 / 5;
export const POST_MAX_ASPECT = 1.91;
type PostAspectMode = "original" | "preset" | "custom";

/** Parse the second-level custom W:H control without guessing invalid input. */
export function parseCustomAspectInput(
	widthInput: string,
	heightInput: string,
): number | null {
	if (widthInput.trim() === "" || heightInput.trim() === "") return null;
	const width = Number(widthInput);
	const height = Number(heightInput);
	if (
		!Number.isFinite(width) ||
		!Number.isFinite(height) ||
		width <= 0 ||
		height <= 0
	) {
		return null;
	}
	const aspect = width / height;
	if (aspect < POST_MIN_ASPECT || aspect > POST_MAX_ASPECT) return null;
	return aspect;
}

/**
 * The option closest to a photo's own shape, used as the per-file default so
 * the uploader usually never touches the control.
 *
 * Distance is measured in log space: a linear `|a - b|` would put 4:5 (0.8)
 * 0.2 from square but 4:3 (1.33) only 0.33 away while 1.91 sits 0.91 out, so
 * every portrait photo would drift toward the wider option. In log space a
 * ratio and its reciprocal are equidistant from square, which is what "closest
 * shape" means to a human.
 */
export function closestAspectOption(
	options: ReadonlyArray<CropAspectOption>,
	naturalAspect: number,
): CropAspectOption | null {
	if (!Number.isFinite(naturalAspect) || naturalAspect <= 0) return null;
	let best: CropAspectOption | null = null;
	let bestDistance = Number.POSITIVE_INFINITY;
	for (const option of options) {
		if (!Number.isFinite(option.aspect) || option.aspect <= 0) continue;
		const distance = Math.abs(Math.log(naturalAspect / option.aspect));
		if (distance < bestDistance) {
			bestDistance = distance;
			best = option;
		}
	}
	return best;
}

export interface CropConfig {
	/** Crop frame aspect (w / h), or "original" to keep the photo's own. */
	aspect: number | "original";
	/**
	 * When present the dialog renders a ratio picker. Generic configs default to
	 * `closestAspectOption`; the post editor starts from the original photo.
	 * `aspect` stays the fallback for files that fail to decode.
	 */
	aspectOptions?: ReadonlyArray<CropAspectOption>;
	shape?: "rect" | "circle";
	/** Destination surfaces rendered in the live preview strip. */
	surfaces?: CropSurfacePreview[];
	/** Shows a "使用原圖" action that forwards the file untouched. */
	allowSkip?: boolean;
	/** Enables the seller-cover fill-mode and background-colour editor. */
	sellerCoverEditor?: boolean;
	/** Enables full-image-first framing on the canonical 4:3 product canvas. */
	productEditor?: boolean;
	/** Enables portrait fill and complete-logo modes on a canonical canvas. */
	avatarEditor?: boolean;
	/** Enables the image-first, progressively disclosed post crop experience. */
	postEditor?: boolean;
	/** Long-side cap for the encoded output (px). */
	maxOutput?: number;
	title?: string;
	description?: string;
	confirmLabel?: string;
}

export type CropPreset =
	| "product"
	| "post"
	| "avatar"
	| "sellerCover"
	| "review"
	| "photo";

export const CROP_PRESETS: Record<CropPreset, CropConfig> = {
	product: {
		// Every product surface shows 4:3. Keep that shared canvas, but default
		// to fitting the complete upload inside it so packaging and text survive.
		aspect: PRODUCT_IMAGE_ASPECT,
		productEditor: true,
		title: "調整商品圖片",
		description:
			"預設完整保留原圖；只有您主動縮放或移動取景時，框外內容才會被裁切。",
		confirmLabel: "確認上傳",
		maxOutput: 1600,
		surfaces: [
			{ label: "商品詳情頁", aspect: 4 / 3, size: 84 },
			{ label: "市集商品卡", aspect: 4 / 3, size: 84 },
			{ label: "購物車 / 圖庫縮圖", aspect: 4 / 3, size: 48 },
		],
	},
	post: {
		// Frame ratio is the uploader's choice (see POST_ASPECT_OPTIONS); 4:3
		// remains the fallback for undecodable files. The surfaces below are
		// destination shapes and stay fixed — the feed grid is a uniform 4:3
		// board, so the preview has to show the further crop a tall or wide
		// choice will take there.
		aspect: POST_FEED_ASPECT,
		aspectOptions: POST_ASPECT_OPTIONS,
		postEditor: true,
		title: "調整貼文圖片",
		confirmLabel: "完成",
		maxOutput: 1600,
		surfaces: [
			{
				label: "動態牆卡片",
				aspect: POST_FEED_ASPECT,
				fit: "contain",
				size: 96,
			},
			{ label: "分享縮圖", aspect: 1, size: 64 },
		],
	},
	avatar: {
		aspect: 1,
		shape: "circle",
		avatarEditor: true,
		title: "調整頭像",
		description:
			"照片與 Logo 都可拖曳；透明 Logo 會先完整置中，也可在圓形安全範圍內縮放。",
		maxOutput: 640,
		surfaces: [
			{ label: "個人頁", aspect: 1, size: 84, circle: true },
			{ label: "留言列", aspect: 1, size: 44, circle: true },
			{ label: "導覽列", aspect: 1, size: 30, circle: true },
		],
	},
	sellerCover: {
		aspect: SELLER_COVER_ASPECT,
		sellerCoverEditor: true,
		title: "調整店鋪背景",
		description: "固定輸出 4:1；會依圖片自動建議填滿或完整顯示，也可自行切換。",
		confirmLabel: "確認上傳",
		maxOutput: 1600,
		surfaces: [
			{
				label: "店鋪頁 · 桌面",
				aspect: SELLER_COVER_ASPECT,
				fit: "cover",
				size: 62,
			},
			{
				label: "店鋪頁 · 行動",
				aspect: SELLER_COVER_ASPECT,
				fit: "cover",
				size: 86,
			},
		],
	},
	review: {
		aspect: 1,
		title: "調整評價照片",
		allowSkip: true,
		maxOutput: 1600,
		surfaces: [{ label: "評價相片", aspect: 1, size: 72 }],
	},
	photo: {
		aspect: "original",
		title: "調整照片",
		allowSkip: true,
		maxOutput: 2048,
	},
};

export function resolveCropConfig(crop: CropPreset | CropConfig): CropConfig {
	return typeof crop === "string" ? CROP_PRESETS[crop] : crop;
}

const TRANSPARENCY_GRID_STYLE: React.CSSProperties = {
	backgroundColor: "var(--page-crop-grid-bg)",
	backgroundImage:
		"linear-gradient(45deg, var(--page-crop-grid-tile) 25%, transparent 25%), linear-gradient(-45deg, var(--page-crop-grid-tile) 25%, transparent 25%), linear-gradient(45deg, transparent 75%, var(--page-crop-grid-tile) 75%), linear-gradient(-45deg, transparent 75%, var(--page-crop-grid-tile) 75%)",
	backgroundPosition: "0 0, 0 8px, 8px -8px, -8px 0",
	backgroundSize: "16px 16px",
};

const AVATAR_FIT_TOGGLE_CLASS =
	"z-10 rounded-[18px]! border border-transparent font-semibold [--toggle-button-bg:transparent] [--toggle-button-bg-hover:var(--page-crop-chrome-bg-hover)] [--toggle-button-bg-pressed:var(--page-crop-chrome-bg-pressed)] [--toggle-button-bg-selected:transparent] [--toggle-button-bg-selected-hover:var(--page-crop-chrome-bg-selected-hover)] [--toggle-button-bg-selected-pressed:var(--page-crop-chrome-bg-selected-pressed)] [--toggle-button-fg-selected:var(--page-crop-chrome-fg)] data-[selected=true]:font-bold";
const IMAGE_CROP_EXIT_CONTENT_MS = 450;

export interface ImageCropDialogProps {
	/** Pending queue. Empty array = dialog closed. */
	files: File[];
	crop: CropPreset | CropConfig;
	/** Fires once per file, in order, with the cropped (or skipped) File. */
	onCropped: (file: File, index: number) => void;
	/** Queue finished, or the user dismissed mid-way (remaining files drop). */
	onClose: () => void;
}

export function ImageCropDialog({
	files,
	crop,
	onCropped,
	onClose,
}: ImageCropDialogProps) {
	const config = resolveCropConfig(crop);
	const isSellerCoverEditor = config.sellerCoverEditor === true;
	const isProductEditor = config.productEditor === true;
	const isAvatarEditor = config.avatarEditor === true;
	const isPostEditor = config.postEditor === true;
	const [index, setIndex] = React.useState(0);
	const [src, setSrc] = React.useState<string | null>(null);
	const [naturalAspect, setNaturalAspect] = React.useState<number | null>(null);
	const [naturalSize, setNaturalSize] = React.useState<{
		width: number;
		height: number;
	} | null>(null);
	const [cropPos, setCropPos] = React.useState<Point>({ x: 0, y: 0 });
	const [zoom, setZoom] = React.useState(1);
	// Explicit ratio override for the CURRENT file only — cleared per file so
	// each photo re-defaults to its own closest shape instead of inheriting the
	// previous one's choice.
	const [pickedAspect, setPickedAspect] = React.useState<number | null>(null);
	const [postAspectMode, setPostAspectMode] =
		React.useState<PostAspectMode>("original");
	const [postRatioPanelOpen, setPostRatioPanelOpen] = React.useState(false);
	const [postCustomAspectOpen, setPostCustomAspectOpen] = React.useState(false);
	const [customAspectWidth, setCustomAspectWidth] = React.useState("4");
	const [customAspectHeight, setCustomAspectHeight] = React.useState("3");
	const [appliedCustomAspectLabel, setAppliedCustomAspectLabel] =
		React.useState("4:3");
	const [customAspectError, setCustomAspectError] = React.useState<
		string | null
	>(null);
	const [cropReady, setCropReady] = React.useState(false);
	const [processingError, setProcessingError] = React.useState<string | null>(
		null,
	);
	const [processing, setProcessing] = React.useState(false);
	const [gifPreviewState, setGifPreview] = React.useState<{
		source: File;
		signature: string;
		revision: string;
		output: File;
		dimensions: { width: number; height: number };
		autoResized: boolean;
	} | null>(null);
	const [gifPreviewUrl, setGifPreviewUrl] = React.useState<string | null>(null);
	const gifPreviewFigureRef = React.useRef<HTMLElement>(null);
	const [processingProgress, setProcessingProgress] = React.useState<
		number | null
	>(null);
	const [decodeFailed, setDecodeFailed] = React.useState(false);
	const [coverFillMode, setCoverFillMode] =
		React.useState<SellerCoverFillMode>("cover");
	const [coverBackground, setCoverBackground] = React.useState<string>(
		SELLER_COVER_DEFAULT_BACKGROUND,
	);
	const [productCropSize, setProductCropSize] = React.useState<Size | null>(
		null,
	);
	const [isProductInteracting, setIsProductInteracting] = React.useState(false);
	const [avatarFitMode, setAvatarFitMode] =
		React.useState<AvatarFitMode>("fill");
	const [avatarBackground, setAvatarBackground] =
		React.useState<AvatarBackground>(AVATAR_DEFAULT_BACKGROUND);
	const [avatarLogoTransform, setAvatarLogoTransform] =
		React.useState<AvatarLogoTransform>(() => ({
			...AVATAR_LOGO_DEFAULT_TRANSFORM,
		}));
	const [isAvatarLogoDragging, setIsAvatarLogoDragging] = React.useState(false);
	const [alphaAnalysis, setAlphaAnalysis] =
		React.useState<AlphaAnalysis | null>(null);
	const productGestureHelpId = React.useId();
	const [displayFiles, setDisplayFiles] = React.useState(files);

	const imageRef = React.useRef<HTMLImageElement | null>(null);
	const areaRef = React.useRef<CropAreaPixels | null>(null);
	const avatarMainCanvasRef = React.useRef<HTMLCanvasElement | null>(null);
	const previewCanvasRefs = React.useRef<(HTMLCanvasElement | null)[]>([]);
	const drawScheduled = React.useRef(false);
	const processingAbortRef = React.useRef<AbortController | null>(null);
	const processingGenerationRef = React.useRef(0);
	const avatarLogoDragRef = React.useRef<{
		pointerId: number;
		startX: number;
		startY: number;
		startOffsetX: number;
		startOffsetY: number;
		renderedSize: number;
	} | null>(null);

	const open = files.length > 0;
	// Callers clear `files` as soon as close starts. Keep the last queue just
	// beyond the shared Dialog fallback so the image/title do not turn into an
	// empty loading state while the panel is visibly easing out.
	React.useEffect(() => {
		if (files.length > 0) {
			setDisplayFiles(files);
			return;
		}
		const timer = window.setTimeout(
			() => setDisplayFiles([]),
			IMAGE_CROP_EXIT_CONTENT_MS,
		);
		return () => window.clearTimeout(timer);
	}, [files]);
	const renderedFiles = open ? files : displayFiles;
	const current = renderedFiles[index];
	const isAnimatedGif = Boolean(current && isGifUploadFile(current));
	const gifEditor = useGifEditor(current, isPostEditor && isAnimatedGif);
	const gifEditRevision = JSON.stringify({
		cropPos,
		zoom,
		pickedAspect,
		postAspectMode,
		settings: gifEditor.key,
	});
	const gifPreview =
		gifPreviewState &&
		gifPreviewState.source === current &&
		gifPreviewState.revision === gifEditRevision
			? gifPreviewState
			: null;

	React.useEffect(() => {
		if (!gifPreview) {
			setGifPreviewUrl(null);
			return;
		}
		const url = URL.createObjectURL(gifPreview.output);
		setGifPreviewUrl(url);
		return () => URL.revokeObjectURL(url);
	}, [gifPreview]);
	React.useEffect(() => {
		if (!gifPreview || !gifPreviewUrl) return;
		const frame = requestAnimationFrame(() => {
			const figure = gifPreviewFigureRef.current;
			figure?.focus({ preventScroll: true });
			figure?.scrollIntoView({ block: "nearest" });
		});
		return () => cancelAnimationFrame(frame);
	}, [gifPreview, gifPreviewUrl]);

	// An animation preview is valid only for the exact authored crop/settings.
	// Drop it on interaction, including before the cropper reports new pixels.
	React.useEffect(() => {
		if (
			gifPreviewState &&
			(gifPreviewState.source !== current ||
				gifPreviewState.revision !== gifEditRevision)
		) {
			setGifPreview(null);
		}
	}, [current, gifPreviewState, gifEditRevision]);

	React.useEffect(() => {
		if (!open || !current) return;
		setProcessing(false);
		setProcessingProgress(null);
		return () => {
			processingGenerationRef.current += 1;
			processingAbortRef.current?.abort();
			processingAbortRef.current = null;
		};
	}, [current, open]);

	// New queue → start over from the first file.
	React.useEffect(() => {
		if (files.length > 0) setIndex(0);
	}, [files]);

	// Load the current file for the cropper and the preview strip.
	React.useEffect(() => {
		if (!current) return;
		let cancelled = false;
		const url = URL.createObjectURL(current);
		setSrc(url);
		setCropPos({ x: 0, y: 0 });
		setZoom(1);
		setDecodeFailed(false);
		setNaturalAspect(null);
		setNaturalSize(null);
		setPickedAspect(null);
		setPostAspectMode("original");
		setPostRatioPanelOpen(false);
		setPostCustomAspectOpen(false);
		setCustomAspectWidth("4");
		setCustomAspectHeight("3");
		setAppliedCustomAspectLabel("4:3");
		setCustomAspectError(null);
		setCropReady(false);
		setProcessingError(null);
		setProcessingProgress(null);
		setCoverFillMode("cover");
		setCoverBackground(SELLER_COVER_DEFAULT_BACKGROUND);
		setProductCropSize(null);
		setIsProductInteracting(false);
		setAvatarFitMode("fill");
		setAvatarBackground(AVATAR_DEFAULT_BACKGROUND);
		setAvatarLogoTransform({ ...AVATAR_LOGO_DEFAULT_TRANSFORM });
		setIsAvatarLogoDragging(false);
		avatarLogoDragRef.current = null;
		setAlphaAnalysis(null);
		areaRef.current = null;
		imageRef.current = null;
		loadImageElement(url)
			.then((img) => {
				if (cancelled) return;
				imageRef.current = img;
				setNaturalSize({ width: img.naturalWidth, height: img.naturalHeight });
				const nextNaturalAspect = img.naturalWidth / img.naturalHeight;
				setNaturalAspect(nextNaturalAspect);
				if (isProductEditor) {
					setZoom(calculateProductImageContainZoom(nextNaturalAspect));
				}
				let nextAlphaAnalysis: AlphaAnalysis | null = null;
				try {
					nextAlphaAnalysis = analyzeImageAlpha(img);
					setAlphaAnalysis(nextAlphaAnalysis);
				} catch {
					// Alpha inspection is only a fitting hint. Decoding and manual
					// controls remain available when a browser cannot read pixels.
					setAlphaAnalysis(null);
				}
				if (
					isAvatarEditor &&
					nextAlphaAnalysis &&
					shouldUseAvatarLogoFit(current.type, nextAlphaAnalysis)
				) {
					setAvatarFitMode("logo");
				}
				if (
					isSellerCoverEditor &&
					shouldContainSellerCover(
						nextNaturalAspect,
						nextAlphaAnalysis?.hasTransparency ?? false,
					)
				) {
					setCoverFillMode("contain");
				}
			})
			.catch(() => {
				if (!cancelled) setDecodeFailed(true);
			});
		return () => {
			cancelled = true;
			URL.revokeObjectURL(url);
		};
	}, [current, isAvatarEditor, isProductEditor, isSellerCoverEditor]);

	const aspectOptions = config.aspectOptions;
	// Generic aspect-option configs retain their closest-shape default. The post
	// editor deliberately starts from the original photo and reveals presets on
	// demand instead.
	const autoAspect = React.useMemo(() => {
		if (!aspectOptions?.length || naturalAspect === null) return null;
		return closestAspectOption(aspectOptions, naturalAspect)?.aspect ?? null;
	}, [aspectOptions, naturalAspect]);
	const selectedAspect = isPostEditor
		? postAspectMode === "original"
			? naturalAspect
			: pickedAspect
		: (pickedAspect ?? autoAspect);
	const suggestedAvatarFitMode: AvatarFitMode =
		current &&
		alphaAnalysis &&
		shouldUseAvatarLogoFit(current.type, alphaAnalysis)
			? "logo"
			: "fill";
	const suggestedCoverFillMode: SellerCoverFillMode =
		naturalAspect !== null &&
		shouldContainSellerCover(
			naturalAspect,
			alphaAnalysis?.hasTransparency ?? false,
		)
			? "contain"
			: "cover";
	const avatarLogoSourceWidth =
		alphaAnalysis?.visibleBounds?.width ?? naturalSize?.width ?? 0;
	const avatarLogoSourceHeight =
		alphaAnalysis?.visibleBounds?.height ?? naturalSize?.height ?? 0;
	const avatarLogoMaxScale = calculateAvatarLogoMaxScale(
		avatarLogoSourceWidth,
		avatarLogoSourceHeight,
	);
	const productOutputSize = React.useMemo(
		() =>
			isProductEditor && naturalSize
				? calculateProductImageOutputSize(
						naturalSize.width,
						naturalSize.height,
						config.maxOutput,
					)
				: null,
		[config.maxOutput, isProductEditor, naturalSize],
	);
	const productContainZoom = React.useMemo(
		() =>
			isProductEditor && naturalAspect !== null
				? calculateProductImageContainZoom(naturalAspect)
				: 1,
		[isProductEditor, naturalAspect],
	);
	const productScale = isProductEditor
		? Math.min(
				PRODUCT_IMAGE_MAX_SCALE,
				Math.max(PRODUCT_IMAGE_MIN_SCALE, zoom / productContainZoom),
			)
		: PRODUCT_IMAGE_MIN_SCALE;
	const productTransform = React.useMemo<ProductImageTransform>(() => {
		if (
			!isProductEditor ||
			!productCropSize ||
			productCropSize.width <= 0 ||
			productCropSize.height <= 0
		) {
			return PRODUCT_IMAGE_DEFAULT_TRANSFORM;
		}
		return {
			scale: productScale,
			offsetX: cropPos.x / productCropSize.width,
			offsetY: cropPos.y / productCropSize.height,
		};
	}, [cropPos.x, cropPos.y, isProductEditor, productCropSize, productScale]);
	const productIsCustomized =
		Math.abs(productTransform.scale - PRODUCT_IMAGE_MIN_SCALE) > 0.005 ||
		Math.abs(productTransform.offsetX) > 0.001 ||
		Math.abs(productTransform.offsetY) > 0.001;
	const productWillCrop = Boolean(
		isProductEditor &&
			naturalSize &&
			productOutputSize &&
			productImageTransformWillCrop(
				naturalSize.width,
				naturalSize.height,
				productOutputSize.width,
				productOutputSize.height,
				productTransform,
			),
	);
	const clampAvatarLogoTransform = React.useCallback(
		(transform: Partial<AvatarLogoTransform>): AvatarLogoTransform =>
			avatarLogoSourceWidth > 0 && avatarLogoSourceHeight > 0
				? constrainAvatarLogoTransform(
						avatarLogoSourceWidth,
						avatarLogoSourceHeight,
						transform,
					)
				: {
						offsetX: transform.offsetX ?? 0,
						offsetY: transform.offsetY ?? 0,
						scale: transform.scale ?? 1,
					},
		[avatarLogoSourceHeight, avatarLogoSourceWidth],
	);

	React.useEffect(() => {
		if (avatarLogoSourceWidth <= 0 || avatarLogoSourceHeight <= 0) return;
		setAvatarLogoTransform((currentTransform) => {
			const constrained = clampAvatarLogoTransform(currentTransform);
			return constrained.offsetX === currentTransform.offsetX &&
				constrained.offsetY === currentTransform.offsetY &&
				constrained.scale === currentTransform.scale
				? currentTransform
				: constrained;
		});
	}, [avatarLogoSourceHeight, avatarLogoSourceWidth, clampAvatarLogoTransform]);

	const updateAvatarLogoTransform = React.useCallback(
		(patch: Partial<AvatarLogoTransform>) => {
			setAvatarLogoTransform((currentTransform) =>
				clampAvatarLogoTransform({ ...currentTransform, ...patch }),
			);
		},
		[clampAvatarLogoTransform],
	);

	const handleAvatarLogoPointerDown = React.useCallback(
		(event: React.PointerEvent<HTMLButtonElement>) => {
			if (
				processing ||
				decodeFailed ||
				(event.pointerType === "mouse" && event.button !== 0)
			)
				return;
			const rect = event.currentTarget.getBoundingClientRect();
			const renderedSize = Math.min(rect.width, rect.height);
			if (renderedSize <= 0) return;
			event.preventDefault();
			event.currentTarget.focus({ preventScroll: true });
			event.currentTarget.setPointerCapture(event.pointerId);
			avatarLogoDragRef.current = {
				pointerId: event.pointerId,
				startX: event.clientX,
				startY: event.clientY,
				startOffsetX: avatarLogoTransform.offsetX,
				startOffsetY: avatarLogoTransform.offsetY,
				renderedSize,
			};
			setIsAvatarLogoDragging(true);
		},
		[
			avatarLogoTransform.offsetX,
			avatarLogoTransform.offsetY,
			decodeFailed,
			processing,
		],
	);

	const handleAvatarLogoPointerMove = React.useCallback(
		(event: React.PointerEvent<HTMLButtonElement>) => {
			const drag = avatarLogoDragRef.current;
			if (!drag || drag.pointerId !== event.pointerId) return;
			event.preventDefault();
			const outputPerRenderedPixel = AVATAR_OUTPUT.width / drag.renderedSize;
			updateAvatarLogoTransform({
				offsetX:
					drag.startOffsetX +
					(event.clientX - drag.startX) * outputPerRenderedPixel,
				offsetY:
					drag.startOffsetY +
					(event.clientY - drag.startY) * outputPerRenderedPixel,
			});
		},
		[updateAvatarLogoTransform],
	);

	const finishAvatarLogoDrag = React.useCallback(
		(event: React.PointerEvent<HTMLButtonElement>) => {
			if (avatarLogoDragRef.current?.pointerId !== event.pointerId) return;
			avatarLogoDragRef.current = null;
			setIsAvatarLogoDragging(false);
			if (event.currentTarget.hasPointerCapture(event.pointerId)) {
				event.currentTarget.releasePointerCapture(event.pointerId);
			}
		},
		[],
	);

	const handleAvatarLogoKeyDown = React.useCallback(
		(event: React.KeyboardEvent<HTMLButtonElement>) => {
			const directions: Record<string, Point> = {
				ArrowLeft: { x: -1, y: 0 },
				ArrowRight: { x: 1, y: 0 },
				ArrowUp: { x: 0, y: -1 },
				ArrowDown: { x: 0, y: 1 },
			};
			const direction = directions[event.key];
			if (!direction || processing || decodeFailed) return;
			event.preventDefault();
			const step = event.shiftKey ? 24 : 8;
			updateAvatarLogoTransform({
				offsetX: avatarLogoTransform.offsetX + direction.x * step,
				offsetY: avatarLogoTransform.offsetY + direction.y * step,
			});
		},
		[
			avatarLogoTransform.offsetX,
			avatarLogoTransform.offsetY,
			decodeFailed,
			processing,
			updateAvatarLogoTransform,
		],
	);

	const handleAvatarLogoWheel = React.useCallback(
		(event: React.WheelEvent<HTMLButtonElement>) => {
			if (processing || decodeFailed) return;
			const requestedScale = avatarLogoTransform.scale - event.deltaY * 0.0015;
			const nextTransform = clampAvatarLogoTransform({
				...avatarLogoTransform,
				scale: requestedScale,
			});
			if (nextTransform.scale === avatarLogoTransform.scale) return;
			event.preventDefault();
			setAvatarLogoTransform(nextTransform);
		},
		[avatarLogoTransform, clampAvatarLogoTransform, decodeFailed, processing],
	);

	const aspect =
		config.aspect === "original"
			? (naturalAspect ?? 1)
			: (selectedAspect ?? config.aspect);
	const surfaces = config.surfaces ?? [];
	const usesAvatarLogoTransform = isAvatarEditor && avatarFitMode === "logo";
	const activeZoom = usesAvatarLogoTransform ? avatarLogoTransform.scale : zoom;
	const activeMinZoom = usesAvatarLogoTransform ? AVATAR_LOGO_MIN_SCALE : 1;
	const activeMaxZoom = usesAvatarLogoTransform ? avatarLogoMaxScale : 3;
	const postAspectLabel = React.useMemo(() => {
		if (postAspectMode === "original") return "原圖";
		if (postAspectMode === "custom") return appliedCustomAspectLabel;
		return (
			aspectOptions?.find(
				(option) =>
					pickedAspect !== null &&
					Math.abs(option.aspect - pickedAspect) < 0.0001,
			)?.label ?? "比例"
		);
	}, [appliedCustomAspectLabel, aspectOptions, pickedAspect, postAspectMode]);

	// Switching ratio re-frames the photo, so drop the pan/zoom the uploader set
	// against the old frame — keeping them leaves the subject off-centre in a
	// box that no longer matches what they were aiming at.
	const handlePickAspect = React.useCallback(
		(next: number, mode: PostAspectMode = "preset") => {
			setPickedAspect(next);
			setPostAspectMode(mode);
			setCropPos({ x: 0, y: 0 });
			setZoom(1);
			setPostRatioPanelOpen(false);
			setPostCustomAspectOpen(false);
			setCustomAspectError(null);
			setProcessingError(null);
			// react-easy-crop reports geometry asynchronously after the aspect prop
			// changes. Never let Confirm consume the previous frame in that gap.
			areaRef.current = null;
			setCropReady(false);
		},
		[],
	);

	const handlePickOriginalAspect = React.useCallback(() => {
		if (naturalAspect === null) return;
		handlePickAspect(naturalAspect, "original");
	}, [handlePickAspect, naturalAspect]);

	const handleApplyCustomAspect = React.useCallback(() => {
		const next = parseCustomAspectInput(customAspectWidth, customAspectHeight);
		if (next === null) {
			setCustomAspectError("請輸入 4:5 至 1.91:1 之間的正數比例。");
			return;
		}
		setAppliedCustomAspectLabel(
			`${customAspectWidth.trim()}:${customAspectHeight.trim()}`,
		);
		handlePickAspect(next, "custom");
	}, [customAspectHeight, customAspectWidth, handlePickAspect]);

	const drawPreviews = React.useCallback(() => {
		const img = imageRef.current;
		const area = areaRef.current;
		const avatarCanDrawWithoutArea = isAvatarEditor && avatarFitMode === "logo";
		const productCanDrawWithoutArea = isProductEditor;
		if (
			!img ||
			(!area &&
				!isSellerCoverEditor &&
				!avatarCanDrawWithoutArea &&
				!productCanDrawWithoutArea)
		)
			return;

		if (isSellerCoverEditor) {
			const master = document.createElement("canvas");
			master.width = SELLER_COVER_OUTPUT.width;
			master.height = SELLER_COVER_OUTPUT.height;
			const masterCtx = master.getContext("2d");
			if (!masterCtx) return;
			drawSellerCover(masterCtx, img, img.naturalWidth, img.naturalHeight, {
				mode: coverFillMode,
				backgroundColor: coverBackground,
				sourceArea: coverFillMode === "cover" ? area : null,
			});

			surfaces.forEach((surface, i) => {
				const canvas = previewCanvasRefs.current[i];
				const ctx = canvas?.getContext("2d");
				if (!canvas || !ctx) return;
				const source = calculateCoverCrop(
					master.width,
					master.height,
					surface.aspect,
				);
				ctx.clearRect(0, 0, canvas.width, canvas.height);
				ctx.imageSmoothingEnabled = true;
				ctx.imageSmoothingQuality = "high";
				ctx.drawImage(
					master,
					source.x,
					source.y,
					source.width,
					source.height,
					0,
					0,
					canvas.width,
					canvas.height,
				);
			});
			return;
		}

		if (isProductEditor) {
			const master = document.createElement("canvas");
			master.width = 640;
			master.height = Math.round(master.width / PRODUCT_IMAGE_ASPECT);
			const masterCtx = master.getContext("2d");
			if (!masterCtx) return;
			drawProductImage(masterCtx, img, img.naturalWidth, img.naturalHeight, {
				transform: productTransform,
				targetWidth: master.width,
				targetHeight: master.height,
			});

			surfaces.forEach((_surface, i) => {
				const canvas = previewCanvasRefs.current[i];
				const ctx = canvas?.getContext("2d");
				if (!canvas || !ctx) return;
				ctx.clearRect(0, 0, canvas.width, canvas.height);
				ctx.imageSmoothingEnabled = true;
				ctx.imageSmoothingQuality = "high";
				ctx.drawImage(master, 0, 0, canvas.width, canvas.height);
			});
			return;
		}

		if (isAvatarEditor) {
			const master = document.createElement("canvas");
			master.width = AVATAR_OUTPUT.width;
			master.height = AVATAR_OUTPUT.height;
			const masterCtx = master.getContext("2d");
			if (!masterCtx) return;
			drawAvatarImage(masterCtx, img, img.naturalWidth, img.naturalHeight, {
				mode: avatarFitMode,
				background: avatarBackground,
				sourceArea: avatarFitMode === "fill" ? area : null,
				visibleBounds: alphaAnalysis?.visibleBounds,
				logoTransform: avatarLogoTransform,
			});

			const mainCanvas = avatarMainCanvasRef.current;
			const mainCtx = mainCanvas?.getContext("2d");
			if (mainCanvas && mainCtx) {
				mainCtx.clearRect(0, 0, mainCanvas.width, mainCanvas.height);
				mainCtx.drawImage(master, 0, 0, mainCanvas.width, mainCanvas.height);
			}

			surfaces.forEach((_surface, i) => {
				const canvas = previewCanvasRefs.current[i];
				const ctx = canvas?.getContext("2d");
				if (!canvas || !ctx) return;
				ctx.clearRect(0, 0, canvas.width, canvas.height);
				ctx.imageSmoothingEnabled = true;
				ctx.imageSmoothingQuality = "high";
				ctx.drawImage(master, 0, 0, canvas.width, canvas.height);
			});
			return;
		}

		if (!area) return;
		surfaces.forEach((surface, i) => {
			const canvas = previewCanvasRefs.current[i];
			const ctx = canvas?.getContext("2d");
			if (!canvas || !ctx) return;
			const targetAspect = surface.aspect;
			const cropAspect = area.width / area.height;
			let { x: sx, y: sy, width: sw, height: sh } = area;
			ctx.clearRect(0, 0, canvas.width, canvas.height);

			if (surface.fit === "contain") {
				const destination = calculateCropContainRect(
					area.width,
					area.height,
					canvas.width,
					canvas.height,
				);
				ctx.drawImage(
					img,
					area.x,
					area.y,
					area.width,
					area.height,
					destination.x,
					destination.y,
					destination.width,
					destination.height,
				);
				return;
			}

			// Cover-fit matches every destination that intentionally fills its frame,
			// including product media and seller storefront banners.
			if (cropAspect > targetAspect) {
				sw = area.height * targetAspect;
				sx = area.x + (area.width - sw) / 2;
			} else if (cropAspect < targetAspect) {
				sh = area.width / targetAspect;
				sy = area.y + (area.height - sh) / 2;
			}
			ctx.drawImage(img, sx, sy, sw, sh, 0, 0, canvas.width, canvas.height);
		});
	}, [
		alphaAnalysis?.visibleBounds,
		avatarBackground,
		avatarFitMode,
		avatarLogoTransform,
		coverBackground,
		coverFillMode,
		isAvatarEditor,
		isProductEditor,
		isSellerCoverEditor,
		productTransform,
		surfaces,
	]);

	const scheduleDraw = React.useCallback(() => {
		if (drawScheduled.current) return;
		drawScheduled.current = true;
		requestAnimationFrame(() => {
			drawScheduled.current = false;
			drawPreviews();
		});
	}, [drawPreviews]);

	React.useEffect(() => {
		if (
			(!isSellerCoverEditor && !isProductEditor && !isAvatarEditor) ||
			naturalAspect === null
		)
			return;
		scheduleDraw();
	}, [
		isAvatarEditor,
		isProductEditor,
		isSellerCoverEditor,
		naturalAspect,
		scheduleDraw,
	]);

	const handleCropComplete = React.useCallback(
		(_area: Area, areaPixels: Area) => {
			areaRef.current = areaPixels;
			setCropReady(true);
			scheduleDraw();
		},
		[scheduleDraw],
	);

	const advance = React.useCallback(() => {
		if (index + 1 < files.length) {
			setIndex(index + 1);
		} else {
			onClose();
		}
	}, [index, files.length, onClose]);

	const handleConfirm = async (previewOnly = false) => {
		if (!current || processing || processingAbortRef.current) return;
		if (isPostEditor && isAnimatedGif && !gifEditor.ready) {
			setProcessingError(gifEditor.error ?? "正在讀取動畫資料，請稍候。");
			return;
		}
		// Legacy presets can still preserve their explicit "use original" escape
		// hatch. A post crop is an authored choice, so an unavailable frame must
		// stay recoverable in place instead of silently uploading different pixels.
		if (
			decodeFailed ||
			!imageRef.current ||
			(!isSellerCoverEditor &&
				!isProductEditor &&
				(!isAvatarEditor || avatarFitMode === "fill") &&
				!areaRef.current)
		) {
			if (isPostEditor) {
				setProcessingError(
					decodeFailed
						? "這張圖片無法讀取，尚未上傳。請回到貼文重新選擇圖片。"
						: "圖片還在準備裁切範圍，請稍候再試。",
				);
				return;
			}
			onCropped(current, index);
			advance();
			return;
		}
		const area = areaRef.current
			? ({ ...areaRef.current } as CropAreaPixels)
			: null;
		const operation = processingGenerationRef.current + 1;
		processingGenerationRef.current = operation;
		const controller = new AbortController();
		processingAbortRef.current = controller;
		setProcessing(true);
		setProcessingError(null);
		setProcessingProgress(isAnimatedGif ? 0 : null);
		setPostRatioPanelOpen(false);
		try {
			let cropped: File;
			let gifSignature: string | null = null;
			let gifDimensions: { width: number; height: number } | null = null;
			let autoResized = false;
			let requirePreview = previewOnly;
			if (isPostEditor && isGifUploadFile(current)) {
				if (!area) {
					throw new Error("圖片還在準備裁切範圍，請稍候再試。");
				}
				gifSignature = JSON.stringify({ area, options: gifEditor.options });
				if (
					gifPreview?.source === current &&
					gifPreview.signature === gifSignature
				) {
					cropped = gifPreview.output;
					gifDimensions = gifPreview.dimensions;
					autoResized = gifPreview.autoResized;
				} else {
					const { cropAnimatedGifFile } = await import("@/lib/media/gif-crop");
					cropped = await cropAnimatedGifFile(current, area, {
						...gifEditor.options,
						containAspect: POST_FEED_ASPECT,
						signal: controller.signal,
						onProgress: (progress) => {
							if (processingGenerationRef.current === operation) {
								setProcessingProgress(progress);
							}
						},
					});
					const dimensions = new DataView(
						await cropped.slice(6, 10).arrayBuffer(),
					);
					gifDimensions = {
						width: dimensions.getUint16(0, true),
						height: dimensions.getUint16(2, true),
					};
					const requestedSize = calculateContainedOutputSize(
						area.width,
						area.height,
						POST_FEED_ASPECT,
						gifEditor.options.maxOutput,
					);
					autoResized = Boolean(
						gifEditor.options.autoFit &&
							(gifDimensions.width < requestedSize.width ||
								gifDimensions.height < requestedSize.height),
					);
					// Let the seller review a smaller automatic result before applying it.
					requirePreview ||= autoResized;
				}
			} else {
				cropped = isSellerCoverEditor
					? await createSellerCoverFile(current, imageRef.current, {
							mode: coverFillMode,
							backgroundColor: coverBackground,
							sourceArea: coverFillMode === "cover" ? areaRef.current : null,
						})
					: isProductEditor
						? await createProductImageFile(current, imageRef.current, {
								transform: productTransform,
								maxEdge: config.maxOutput,
							})
						: isAvatarEditor
							? await createAvatarFile(current, imageRef.current, {
									mode: avatarFitMode,
									background: avatarBackground,
									sourceArea: avatarFitMode === "fill" ? areaRef.current : null,
									visibleBounds: alphaAnalysis?.visibleBounds,
									logoTransform: avatarLogoTransform,
								})
							: await cropImageFile(current, area as CropAreaPixels, {
									maxOutput: config.maxOutput,
									containAspect: isPostEditor ? POST_FEED_ASPECT : undefined,
								});
			}
			if (
				controller.signal.aborted ||
				processingGenerationRef.current !== operation
			) {
				return;
			}
			if (requirePreview && gifSignature && gifDimensions) {
				setGifPreview({
					source: current,
					signature: gifSignature,
					revision: gifEditRevision,
					output: cropped,
					dimensions: gifDimensions,
					autoResized,
				});
				return;
			}
			onCropped(cropped, index);
			advance();
		} catch (error) {
			const wasAborted =
				controller.signal.aborted ||
				(error instanceof Error && error.name === "AbortError");
			if (wasAborted) return;
			if (isPostEditor) {
				setProcessingError(
					isAnimatedGif && error instanceof Error
						? error.message
						: "圖片處理失敗，原圖尚未上傳。請重試，或回到貼文更換圖片。",
				);
				return;
			}
			// Other crop presets retain their existing best-effort fallback.
			onCropped(current, index);
			advance();
		} finally {
			if (processingGenerationRef.current === operation) {
				processingAbortRef.current = null;
				setProcessing(false);
				setProcessingProgress(null);
			}
		}
	};

	const handleCancelProcessing = () => {
		processingGenerationRef.current += 1;
		processingAbortRef.current?.abort();
		processingAbortRef.current = null;
		setProcessing(false);
		setProcessingProgress(null);
	};

	const handleSkip = () => {
		if (!current || processing) return;
		onCropped(current, index);
		advance();
	};

	return (
		<Dialog
			open={open}
			onClose={() => {
				if (!isPostEditor || !processing) onClose();
			}}
			isDismissable={!isPostEditor || !processing}
			showCloseButton={!isPostEditor || !processing}
			// 裁切一定是「某個介面裡的下一步」，所以它必須是最上層：ImageUploader
			// 也出現在出貨對話框（.shipment-dialog，z-index 1300）裡，用 Dialog 預設
			// 的 1200 會被壓在底下 —— 露在外面的按鈕按下去等於點到出貨對話框的
			// backdrop，父層直接關掉、照片也丟了（7-11「送出照片」因此完全按不到）。
			overlayClassName={cn("z-[1400]", isPostEditor && "max-sm:p-0")}
			className={
				isPostEditor
					? "h-full w-full max-w-none rounded-none border-0 sm:h-fit sm:max-h-[calc(100dvh-2rem)] sm:max-w-5xl sm:rounded-[var(--hkui-radius-2xl)] sm:border"
					: undefined
			}
			bodyClassName={
				isPostEditor
					? "flex min-h-0 flex-1 flex-col px-3 py-3 [scrollbar-gutter:auto] sm:px-5 sm:py-5"
					: undefined
			}
			title={
				isPostEditor && renderedFiles.length > 1
					? `${config.title ?? "調整圖片"} · ${index + 1}/${renderedFiles.length}`
					: (config.title ?? "調整圖片")
			}
			description={
				isPostEditor
					? undefined
					: (config.description ??
						(renderedFiles.length > 1
							? `第 ${index + 1} / ${renderedFiles.length} 張，拖曳調整位置，滾輪或滑桿縮放`
							: "拖曳調整位置，滾輪或滑桿縮放，框外變暗處不會顯示"))
			}
			size={isPostEditor ? "full" : "lg"}
			footer={
				isPostEditor ? (
					<div className="flex w-full items-center justify-between gap-3">
						<Button
							variant="light"
							color="default"
							size="md"
							onClick={() => {
								if (processing) {
									handleCancelProcessing();
									return;
								}
								handlePickOriginalAspect();
								gifEditor.reset();
								setCustomAspectWidth("4");
								setCustomAspectHeight("3");
							}}
							disabled={!processing && (decodeFailed || naturalAspect === null)}
						>
							{processing ? "取消處理" : "重設"}
						</Button>
						<Button
							variant="solid"
							color="primary"
							size="md"
							className="min-w-28"
							onClick={() => void handleConfirm()}
							isLoading={processing}
							loadingLabel={isAnimatedGif ? "處理 GIF 中…" : "處理圖片中…"}
							disabled={
								processing ||
								decodeFailed ||
								!cropReady ||
								(isAnimatedGif && !gifEditor.ready)
							}
						>
							{config.confirmLabel ?? "完成"}
						</Button>
					</div>
				) : (
					<>
						<Button
							variant="light"
							color="default"
							size="md"
							onClick={() => {
								setCropPos({ x: 0, y: 0 });
								setZoom(1);
								// Back to the ratio auto-picked from the photo, not to the
								// preset's fallback — "reset" means "as it opened".
								setPickedAspect(null);
								if (isSellerCoverEditor) {
									setCoverFillMode(suggestedCoverFillMode);
									setCoverBackground(SELLER_COVER_DEFAULT_BACKGROUND);
								}
								if (isProductEditor) {
									setZoom(productContainZoom);
									setIsProductInteracting(false);
								}
								if (isAvatarEditor) {
									setAvatarFitMode(suggestedAvatarFitMode);
									setAvatarBackground(AVATAR_DEFAULT_BACKGROUND);
									setAvatarLogoTransform({
										...AVATAR_LOGO_DEFAULT_TRANSFORM,
									});
								}
							}}
							disabled={processing || decodeFailed}
						>
							重設
						</Button>
						{config.allowSkip && (
							<Button
								variant="bordered"
								color="default"
								size="md"
								onClick={handleSkip}
								disabled={processing}
							>
								使用原圖
							</Button>
						)}
						<Button
							variant="solid"
							color="primary"
							size="md"
							onClick={() => void handleConfirm()}
							isLoading={processing}
							loadingLabel="處理圖片中…"
						>
							{decodeFailed ? "使用原圖" : (config.confirmLabel ?? "確認裁切")}
						</Button>
					</>
				)
			}
		>
			<div className={cn("flex flex-col gap-4", isPostEditor && "min-h-0")}>
				{isPostEditor ? (
					<section
						className="grid min-h-0 gap-4 md:grid-cols-[minmax(0,1fr)_15rem]"
						aria-label="貼文圖片調整與實際預覽"
						aria-busy={processing}
						data-post-image-editor="true"
						inert={processing ? true : undefined}
					>
						<section
							aria-label="4:3 貼文取景畫布"
							aria-busy={processing}
							data-post-crop-viewport="true"
							style={{ aspectRatio: POST_FEED_ASPECT }}
							className="relative w-full overflow-hidden rounded-[var(--hkui-radius-lg)] bg-[var(--page-crop-canvas-bg)]"
						>
							{decodeFailed ? (
								<div className="flex h-full flex-col items-center justify-center gap-4 px-8 text-center text-sm text-[var(--color-neutral-cream)]/85">
									<p className="m-0 max-w-sm">
										無法讀取這張圖片，圖片尚未上傳。
									</p>
									<Button
										variant="bordered"
										color="default"
										size="sm"
										className="border-white/45 text-white hover:bg-white/10"
										onClick={onClose}
									>
										回到貼文更換圖片
									</Button>
								</div>
							) : src && naturalAspect !== null ? (
								<Cropper
									image={src}
									crop={cropPos}
									zoom={zoom}
									minZoom={1}
									maxZoom={3}
									aspect={aspect}
									showGrid
									restrictPosition
									onCropChange={(nextCrop) => {
										if (processing) return;
										setCropPos(nextCrop);
										setProcessingError(null);
									}}
									onZoomChange={(nextZoom) => {
										if (processing) return;
										setZoom(nextZoom);
										setProcessingError(null);
									}}
									onCropComplete={handleCropComplete}
									cropperProps={{
										"aria-label": "圖片裁切區，可拖曳或使用方向鍵調整位置",
										tabIndex: processing ? -1 : 0,
									}}
									mediaProps={{ alt: "待裁切的貼文圖片" }}
								/>
							) : (
								<div
									className="flex h-full items-center justify-center text-sm text-[var(--color-neutral-cream)]/75"
									role="status"
								>
									載入圖片中…
								</div>
							)}

							{processing ? (
								<div
									className="absolute inset-0 z-30 cursor-wait bg-black/10"
									aria-hidden="true"
								/>
							) : null}

							{!decodeFailed && naturalAspect !== null ? (
								<button
									type="button"
									data-post-ratio-trigger="true"
									aria-expanded={postRatioPanelOpen}
									aria-controls="post-ratio-panel"
									onClick={() => {
										setPostRatioPanelOpen((value) => !value);
										setCustomAspectError(null);
									}}
									disabled={processing}
									className="absolute bottom-4 left-4 z-20 inline-flex min-h-11 items-center gap-2 rounded-full border border-white/25 bg-black/65 px-4 text-sm font-semibold text-white shadow-lg backdrop-blur-md transition motion-reduce:transition-none hover:bg-black/78 focus-visible:outline-none focus-visible:[box-shadow:var(--page-crop-focus-ring)] disabled:opacity-50"
								>
									<svg
										width="18"
										height="18"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										strokeWidth="2"
										strokeLinecap="round"
										strokeLinejoin="round"
										aria-hidden="true"
									>
										<path d="M7 3H5a2 2 0 0 0-2 2v2M17 3h2a2 2 0 0 1 2 2v2M7 21H5a2 2 0 0 1-2-2v-2M17 21h2a2 2 0 0 0 2-2v-2" />
									</svg>
									<span>{postAspectLabel}</span>
									<svg
										width="14"
										height="14"
										viewBox="0 0 20 20"
										fill="currentColor"
										aria-hidden="true"
										className={cn(
											"transition-transform motion-reduce:transition-none",
											postRatioPanelOpen && "rotate-180",
										)}
									>
										<path d="m5.4 7.5 4.6 4.6 4.6-4.6 1.4 1.4-6 6-6-6 1.4-1.4Z" />
									</svg>
								</button>
							) : null}
						</section>

						<aside className="flex min-h-0 flex-col gap-4 md:h-[clamp(360px,58dvh,540px)] md:overflow-y-auto md:pr-1">
							<div className="grid gap-1">
								<p className="m-0 text-sm font-semibold text-foreground">
									{isAnimatedGif
										? "拖曳 GIF 調整每一幀取景"
										: "拖曳圖片調整取景"}
								</p>
								<p className="m-0 text-xs leading-5 text-foreground-soft">
									{isAnimatedGif
										? "先調整取景與動畫設定，再預覽結果。非 4:3 取景會補透明邊。"
										: "用雙指、滾輪或滑桿縮放；非 4:3 取景會自動補透明邊。"}
								</p>
							</div>

							{isAnimatedGif && processing ? (
								<div className="grid gap-2 rounded-hkui-md border border-primary/25 bg-primary/5 p-3">
									<div className="flex items-center justify-between gap-3 text-xs font-semibold text-foreground">
										<span>正在處理動畫</span>
										<span className="tabular-nums" aria-hidden="true">
											{Math.round((processingProgress ?? 0) * 100)}%
										</span>
									</div>
									<div
										className="h-1.5 overflow-hidden rounded-full bg-default-200"
										role="progressbar"
										aria-label="GIF 處理進度"
										aria-valuemin={0}
										aria-valuemax={100}
										aria-valuenow={Math.round((processingProgress ?? 0) * 100)}
									>
										{/* scaleX, not width: width animates layout. */}
										<div
											className="h-full w-full origin-left rounded-full bg-primary transition-transform duration-200 ease-out motion-reduce:transition-none"
											style={{
												transform: `scaleX(${Math.min(
													1,
													Math.max(0, processingProgress ?? 0),
												)})`,
											}}
										/>
									</div>
								</div>
							) : null}

							{postRatioPanelOpen && (
								<section
									id="post-ratio-panel"
									className="grid gap-3 rounded-[var(--hkui-radius-lg)] border border-hk-border-soft bg-hk-surface-muted p-3"
									aria-label="選擇貼文裁切比例"
								>
									<div className="grid grid-cols-2 gap-2">
										<button
											type="button"
											aria-pressed={postAspectMode === "original"}
											onClick={handlePickOriginalAspect}
											className={cn(
												"min-h-11 rounded-hkui-md border px-3 text-sm font-medium transition motion-reduce:transition-none focus-visible:outline-none focus-visible:[box-shadow:var(--hkui-focus-ring)]",
												postAspectMode === "original"
													? "border-primary bg-primary/10 text-primary"
													: "border-hk-border-soft bg-hk-surface-card text-foreground hover:bg-default-100",
											)}
										>
											<span>原圖</span>
										</button>
										{aspectOptions?.map((option) => {
											const selected =
												postAspectMode === "preset" &&
												pickedAspect !== null &&
												Math.abs(option.aspect - pickedAspect) < 0.0001;
											return (
												<button
													type="button"
													key={option.label}
													aria-pressed={selected}
													onClick={() => handlePickAspect(option.aspect)}
													className={cn(
														"min-h-11 rounded-hkui-md border px-3 text-sm font-medium transition motion-reduce:transition-none focus-visible:outline-none focus-visible:[box-shadow:var(--hkui-focus-ring)]",
														selected
															? "border-primary bg-primary/10 text-primary"
															: "border-hk-border-soft bg-hk-surface-card text-foreground hover:bg-default-100",
													)}
												>
													{option.label}
												</button>
											);
										})}
										<button
											type="button"
											aria-expanded={postCustomAspectOpen}
											onClick={() => {
												setPostCustomAspectOpen((value) => !value);
												setCustomAspectError(null);
											}}
											className={cn(
												"min-h-11 rounded-hkui-md border px-3 text-sm font-medium transition motion-reduce:transition-none focus-visible:outline-none focus-visible:[box-shadow:var(--hkui-focus-ring)]",
												postAspectMode === "custom"
													? "border-primary bg-primary/10 text-primary"
													: "border-hk-border-soft bg-hk-surface-card text-foreground hover:bg-default-100",
											)}
										>
											<span>自訂</span>
										</button>
									</div>

									{postCustomAspectOpen && (
										<form
											className="grid gap-3 border-t border-divider pt-3"
											onSubmit={(event) => {
												event.preventDefault();
												handleApplyCustomAspect();
											}}
										>
											<div className="grid grid-cols-[1fr_auto_1fr] items-end gap-2">
												<Input
													id="post-custom-aspect-width"
													label="寬"
													type="number"
													inputMode="decimal"
													min="0.01"
													step="0.01"
													value={customAspectWidth}
													onChange={(event) => {
														setCustomAspectWidth(event.target.value);
														setCustomAspectError(null);
													}}
													size="sm"
												/>
												<button
													type="button"
													aria-label="交換寬高"
													onClick={() => {
														setCustomAspectWidth(customAspectHeight);
														setCustomAspectHeight(customAspectWidth);
														setCustomAspectError(null);
													}}
													className="mb-0.5 inline-grid size-11 place-items-center rounded-full text-foreground-muted transition motion-reduce:transition-none hover:bg-default-100 hover:text-foreground focus-visible:outline-none focus-visible:[box-shadow:var(--hkui-focus-ring)]"
												>
													<svg
														width="16"
														height="16"
														viewBox="0 0 24 24"
														fill="none"
														stroke="currentColor"
														strokeWidth="2"
														strokeLinecap="round"
														strokeLinejoin="round"
														aria-hidden="true"
													>
														<path d="m7 7 3-3m-3 3 3 3M17 17l-3 3m3-3-3-3M7 7h10M17 17H7" />
													</svg>
												</button>
												<Input
													id="post-custom-aspect-height"
													label="高"
													type="number"
													inputMode="decimal"
													min="0.01"
													step="0.01"
													value={customAspectHeight}
													onChange={(event) => {
														setCustomAspectHeight(event.target.value);
														setCustomAspectError(null);
													}}
													size="sm"
												/>
											</div>
											<p className="m-0 text-xs leading-4 text-foreground-soft">
												可輸入例如 3:2；自訂比例介於 4:5 與
												1.91:1。原圖不受此限制。
											</p>
											{customAspectError ? (
												<p className="m-0 text-xs text-danger" role="alert">
													{customAspectError}
												</p>
											) : null}
											<Button type="submit" size="sm" fullWidth>
												套用比例
											</Button>
										</form>
									)}
								</section>
							)}

							<div className="flex items-center gap-3">
								<label
									htmlFor="post-image-crop-zoom"
									className="shrink-0 text-xs font-semibold text-foreground-muted"
								>
									縮放
								</label>
								<input
									id="post-image-crop-zoom"
									type="range"
									min={1}
									max={3}
									step={0.01}
									value={zoom}
									disabled={processing || decodeFailed}
									onChange={(event) => {
										setZoom(Number(event.target.value));
										setProcessingError(null);
									}}
									className="h-11 min-w-0 flex-1 cursor-pointer accent-[var(--hkui-primary)] disabled:cursor-not-allowed [&::-moz-range-track]:h-1 [&::-webkit-slider-runnable-track]:h-1"
								/>
								<output
									htmlFor="post-image-crop-zoom"
									className="w-10 text-right text-xs text-foreground-muted tabular-nums"
								>
									{Math.round(zoom * 100)}%
								</output>
							</div>

							{isAnimatedGif ? (
								<section
									className="grid gap-3 border-t border-divider pt-3"
									aria-label="編輯 GIF 動畫"
								>
									<GifEditControls
										editor={gifEditor}
										disabled={processing}
										onChange={() => setProcessingError(null)}
									/>
									<Button
										variant="bordered"
										color="primary"
										fullWidth
										onClick={() => void handleConfirm(true)}
										disabled={
											processing ||
											decodeFailed ||
											!cropReady ||
											!gifEditor.ready
										}
									>
										{gifPreview ? "重新預覽動畫" : "預覽調整結果"}
									</Button>
									{gifPreview && gifPreviewUrl ? (
										<figure
											ref={gifPreviewFigureRef}
											tabIndex={-1}
											className="m-0 grid gap-2"
											aria-label="調整後的 GIF 預覽"
										>
											{/* biome-ignore lint/performance/noImgElement: native playback of a local GIF blob; no remote image optimization. */}
											<img
												src={gifPreviewUrl}
												alt="調整後的 GIF 動畫"
												className="aspect-[4/3] w-full rounded-hkui-md border border-divider bg-default-100 object-contain"
											/>
											<figcaption
												className="text-xs leading-5 text-foreground-muted"
												role="status"
											>
												{gifPreview.autoResized ? "已自動縮小為" : "輸出尺寸："}
												{gifPreview.dimensions.width} ×{" "}
												{gifPreview.dimensions.height} px，
												{(gifPreview.output.size / 1024 / 1024).toFixed(2)}{" "}
												MiB，符合上傳限制。按「完成」套用。
											</figcaption>
										</figure>
									) : null}
								</section>
							) : null}

							{processingError ? (
								<div
									className="grid gap-2 rounded-hkui-md border border-danger/35 bg-danger/5 p-3 text-xs leading-5 text-danger"
									role="alert"
								>
									<p className="m-0">{processingError}</p>
									<button
										type="button"
										className="inline-flex min-h-11 w-fit items-center font-semibold underline underline-offset-2"
										onClick={onClose}
									>
										回到貼文更換圖片
									</button>
								</div>
							) : null}

							{!postRatioPanelOpen && surfaces.length > 0 && !decodeFailed ? (
								<section className="grid gap-2 border-t border-divider pt-3">
									<div className="grid gap-0.5">
										<span className="text-xs font-semibold text-foreground-muted">
											實際顯示範圍
										</span>
										<span className="text-xs leading-4 text-foreground-soft">
											{isAnimatedGif
												? "取景示意；完整動畫會套用相同範圍與透明邊。"
												: "透明邊會一起上傳，動態牆不再裁切構圖。"}
										</span>
									</div>
									<div className="flex flex-wrap items-end gap-3">
										{surfaces.map((surface, surfaceIndex) => {
											const height = surface.size ?? 84;
											const width = Math.round(height * surface.aspect);
											const dpr = 2;
											return (
												<figure key={surface.label} className="m-0 grid gap-1">
													<canvas
														ref={(element) => {
															previewCanvasRefs.current[surfaceIndex] = element;
														}}
														width={width * dpr}
														height={height * dpr}
														style={{ width, height }}
														className="block h-auto max-w-full rounded-hkui-md border border-divider bg-default-100"
													/>
													<figcaption className="text-xs text-foreground-soft">
														{surface.label}
													</figcaption>
												</figure>
											);
										})}
									</div>
								</section>
							) : null}

							<div className="mt-auto flex min-w-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs text-foreground-soft">
								<span className="min-w-0 truncate" title={current?.name}>
									{current?.name ?? "讀取中"}
								</span>
								<span className="shrink-0 tabular-nums">
									{naturalSize
										? `${naturalSize.width} × ${naturalSize.height} px`
										: "讀取中"}
								</span>
							</div>
						</aside>
					</section>
				) : isSellerCoverEditor ? (
					<section
						className="hk-heroui-brand-scope flex flex-col gap-4"
						aria-label="店鋪背景調整與預覽"
					>
						<ToggleButtonGroup
							aria-label="封面填滿方式"
							selectionMode="single"
							disallowEmptySelection
							selectedKeys={new Set([coverFillMode])}
							onSelectionChange={(keys) => {
								const next = Array.from(keys)[0];
								if (next === "cover" || next === "contain") {
									setCoverFillMode(next);
								}
							}}
							fullWidth
							size="md"
						>
							<ToggleButton id="cover">寬度填滿</ToggleButton>
							<ToggleButton id="contain">完整顯示</ToggleButton>
						</ToggleButtonGroup>
						<p className="m-0 text-xs text-foreground-soft">
							{suggestedCoverFillMode === "contain"
								? alphaAnalysis?.hasTransparency
									? "偵測到透明背景，已預選「完整顯示」，避免圖案被放大裁切。"
									: "原圖比例與 4:1 差距較大，已預選「完整顯示」。"
								: "原圖接近 4:1，已預選「寬度填滿」。"}
						</p>

						<div className="relative aspect-[4/1] min-h-[118px] w-full overflow-hidden rounded-hkui-md border border-hk-border-soft bg-default-100">
							{decodeFailed ? (
								<div className="grid h-full place-items-center px-5 text-center text-sm text-foreground-muted">
									無法預覽這張圖片，確認後將直接使用原圖上傳。
								</div>
							) : src && naturalAspect !== null ? (
								coverFillMode === "cover" ? (
									<Cropper
										image={src}
										crop={cropPos}
										zoom={zoom}
										minZoom={1}
										maxZoom={3}
										aspect={aspect}
										showGrid
										restrictPosition
										onCropChange={setCropPos}
										onZoomChange={setZoom}
										onCropComplete={handleCropComplete}
										cropperProps={{
											"aria-label":
												"店鋪背景裁切區，可拖曳或使用方向鍵調整位置",
										}}
										mediaProps={{ alt: "待裁切的店鋪背景" }}
									/>
								) : (
									<div
										className="absolute inset-0 overflow-hidden"
										style={{ backgroundColor: coverBackground }}
									>
										{/* biome-ignore lint/performance/noImgElement: local blob preview */}
										<img
											src={src}
											alt="完整顯示的店鋪封面預覽"
											className="absolute inset-0"
											style={{
												width: "100%",
												height: "100%",
												objectFit: "contain",
												objectPosition: "center",
											}}
										/>
									</div>
								)
							) : null}
						</div>

						{coverFillMode === "cover" && (
							<Slider
								aria-label="縮放"
								minValue={1}
								maxValue={3}
								step={0.01}
								value={zoom}
								isDisabled={processing || decodeFailed}
								onChange={(value) =>
									setZoom(Array.isArray(value) ? (value[0] ?? 1) : value)
								}
							>
								<span data-slot="label">縮放</span>
								<Slider.Output>{Math.round(zoom * 100)}%</Slider.Output>
								<Slider.Track>
									<Slider.Fill />
									<Slider.Thumb />
								</Slider.Track>
							</Slider>
						)}

						{coverFillMode === "contain" && (
							<div className="flex flex-wrap items-end justify-between gap-3">
								<div className="flex flex-col gap-1">
									<span className="text-sm font-medium text-foreground">
										背景顏色
									</span>
									<span className="text-xs text-foreground-soft">
										填滿圖片以外的區域
									</span>
								</div>
								<div className="flex flex-wrap items-center gap-2">
									<ColorSwatchPicker
										aria-label="背景顏色"
										value={parseColor(coverBackground)}
										onChange={(color) =>
											setCoverBackground(color.toString("hex").toUpperCase())
										}
										size="sm"
									>
										{SELLER_COVER_BACKGROUND_PRESETS.map((preset) => (
											<ColorSwatchPicker.Item
												key={preset.color}
												color={preset.color}
												aria-label={preset.label}
											>
												<ColorSwatchPicker.Swatch />
											</ColorSwatchPicker.Item>
										))}
									</ColorSwatchPicker>
									<label className="relative inline-flex h-11 cursor-pointer items-center gap-2 rounded-hkui-md border border-hk-border-soft bg-hk-surface-card px-2.5 text-xs font-medium text-foreground transition-colors motion-reduce:transition-none hover:bg-hk-surface-muted focus-within:[box-shadow:var(--hkui-focus-ring)]">
										<input
											type="color"
											value={coverBackground}
											onChange={(event) =>
												setCoverBackground(event.target.value.toUpperCase())
											}
											aria-label="自訂背景顏色"
											className="absolute inset-0 cursor-pointer opacity-0"
										/>
										<ColorSwatch
											color={coverBackground}
											size="xs"
											shape="square"
										/>
										<span>自訂</span>
									</label>
									<output className="text-xs text-foreground-soft tabular-nums">
										{coverBackground}
									</output>
								</div>
							</div>
						)}
						<div className="flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-foreground-soft">
							<span className="min-w-0 truncate" title={current?.name}>
								{current?.name ?? "讀取中"}
							</span>
							<span className="shrink-0 tabular-nums">
								{naturalSize
									? `${naturalSize.width} × ${naturalSize.height} px`
									: "讀取中"}
								{" / "}輸出 {SELLER_COVER_OUTPUT.width} ×{" "}
								{SELLER_COVER_OUTPUT.height} px
							</span>
						</div>
					</section>
				) : isProductEditor ? (
					<section
						className="hk-heroui-brand-scope flex flex-col gap-4"
						aria-label="商品圖片取景與預覽"
					>
						<div className="flex flex-wrap items-baseline justify-between gap-2">
							<span
								className={`text-sm font-semibold ${
									productWillCrop ? "text-warning-700" : "text-primary"
								}`}
								aria-live="polite"
							>
								{productWillCrop
									? "自訂取景 · 框外會裁切"
									: productIsCustomized
										? "已調整 · 原圖仍完整"
										: "完整原圖 · 未裁切"}
							</span>
							<span className="text-xs text-foreground-soft tabular-nums">
								縮放 {Math.round(productScale * 100)}%
							</span>
						</div>
						<p
							id={productGestureHelpId}
							className="m-0 text-xs leading-relaxed text-foreground-soft"
						>
							原圖比例不足處會以柔米色補齊。桌機：滾輪縮放，拖曳圖片或按住滑鼠中鍵移動。手機：雙指縮放，單指拖曳。方向鍵也可微調位置。
						</p>

						<div
							data-product-image-viewport="true"
							data-product-image-scale={productScale.toFixed(4)}
							data-product-image-offset-x={productTransform.offsetX.toFixed(6)}
							data-product-image-offset-y={productTransform.offsetY.toFixed(6)}
							data-product-image-will-crop={productWillCrop ? "true" : "false"}
							onMouseDownCapture={(event) => {
								if (processing || decodeFailed) {
									event.preventDefault();
									event.stopPropagation();
									return;
								}
								// react-easy-crop supports primary and middle-button panning.
								// Block right-drag from entering that path, while preserving the
								// browser's native context menu.
								if (event.button === 2) {
									event.stopPropagation();
								} else if (event.button === 1) {
									event.preventDefault();
								}
							}}
							onTouchStartCapture={(event) => {
								if (processing || decodeFailed) {
									event.preventDefault();
									event.stopPropagation();
								}
							}}
							onAuxClick={(event) => {
								if (event.button === 1) event.preventDefault();
							}}
							className="relative aspect-[4/3] w-full overflow-hidden rounded-hkui-lg border border-hk-border-soft bg-hk-surface-muted focus-within:[box-shadow:var(--hkui-focus-ring)] sm:h-[clamp(260px,42vh,380px)] sm:w-auto sm:max-w-full sm:self-center"
						>
							{decodeFailed ? (
								<div className="grid h-full place-items-center px-5 text-center text-sm text-foreground-muted">
									無法預覽這張圖片，確認後將直接使用原圖上傳。
								</div>
							) : src && naturalAspect !== null ? (
								<Cropper
									image={src}
									crop={cropPos}
									zoom={zoom}
									minZoom={productContainZoom}
									maxZoom={productContainZoom * PRODUCT_IMAGE_MAX_SCALE}
									zoomSpeed={productContainZoom * 0.8}
									zoomWithScroll
									objectFit="cover"
									aspect={aspect}
									showGrid={productIsCustomized || isProductInteracting}
									restrictPosition
									onCropChange={setCropPos}
									onZoomChange={setZoom}
									onCropComplete={handleCropComplete}
									onCropSizeChange={setProductCropSize}
									onInteractionStart={() => setIsProductInteracting(true)}
									onInteractionEnd={() => setIsProductInteracting(false)}
									onWheelRequest={(event) =>
										!processing && !event.ctrlKey && !event.metaKey
									}
									onTouchRequest={() => !processing}
									style={{
										containerStyle: {
											backgroundColor: PRODUCT_IMAGE_BACKGROUND,
											cursor: isProductInteracting ? "grabbing" : "grab",
										},
									}}
									cropperProps={{
										"aria-label":
											"商品圖片取景區，可拖曳、使用中鍵或方向鍵調整位置",
										"aria-describedby": productGestureHelpId,
										tabIndex: processing ? -1 : 0,
									}}
									mediaProps={{ alt: "待調整的商品圖片" }}
								/>
							) : null}
						</div>

						<Slider
							aria-label="商品圖片縮放"
							minValue={PRODUCT_IMAGE_MIN_SCALE}
							maxValue={PRODUCT_IMAGE_MAX_SCALE}
							step={0.01}
							value={productScale}
							isDisabled={processing || decodeFailed}
							onChange={(value) => {
								const nextScale = Array.isArray(value)
									? (value[0] ?? PRODUCT_IMAGE_MIN_SCALE)
									: value;
								setZoom(nextScale * productContainZoom);
							}}
						>
							<span data-slot="label">縮放</span>
							<Slider.Output>{Math.round(productScale * 100)}%</Slider.Output>
							<Slider.Track>
								<Slider.Fill />
								<Slider.Thumb />
							</Slider.Track>
						</Slider>

						<div className="flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-foreground-soft">
							<span className="min-w-0 truncate" title={current?.name}>
								{current?.name ?? "讀取中"}
							</span>
							<span className="shrink-0 tabular-nums">
								{naturalSize
									? `${naturalSize.width} × ${naturalSize.height} px`
									: "讀取中"}
								{productOutputSize
									? ` / 輸出 ${productOutputSize.width} × ${productOutputSize.height} px`
									: ""}
							</span>
						</div>
					</section>
				) : (
					<>
						{isAvatarEditor && !decodeFailed && (
							<section
								className="hk-heroui-brand-scope flex flex-col gap-2"
								aria-label="頭像顯示方式"
							>
								<div
									className="relative isolate overflow-hidden rounded-[22px] border border-hk-border-soft bg-hk-surface-muted [box-shadow:var(--page-crop-lens-shell-shadow)]"
									data-avatar-fit-mode={avatarFitMode}
								>
									{/* The two translucent-white background literals below are pinned by
									    tests/image-crop-presets.test.ts, which asserts the exact class
									    string; everything else on this lens is tokenised. See the
									    PROPOSED note on splitting this file for the follow-up. */}
									<span
										aria-hidden="true"
										data-avatar-fit-indicator="true"
										className={`pointer-events-none absolute inset-y-1 left-1 z-[1] w-[calc(50%_-_0.25rem)] rounded-[18px] border border-[color:var(--page-crop-lens-border)] bg-[rgba(255,255,255,0.72)] [backdrop-filter:blur(10px)_saturate(115%)] [-webkit-backdrop-filter:blur(10px)_saturate(115%)] [box-shadow:var(--page-crop-lens-shadow)] transition-transform duration-200 ease-[var(--hkui-ease)] motion-reduce:transition-none dark:bg-[rgba(255,255,255,0.14)] ${
											avatarFitMode === "logo"
												? "translate-x-full"
												: "translate-x-0"
										}`}
									/>
									<ToggleButtonGroup
										aria-label="頭像顯示方式"
										selectionMode="single"
										disallowEmptySelection
										selectedKeys={new Set([avatarFitMode])}
										onSelectionChange={(keys) => {
											const next = Array.from(keys)[0];
											if (next === "fill" || next === "logo") {
												setAvatarFitMode(next);
											}
										}}
										isDisabled={processing}
										fullWidth
										size="md"
										className="relative z-[2] p-1"
									>
										<ToggleButton id="fill" className={AVATAR_FIT_TOGGLE_CLASS}>
											照片填滿
										</ToggleButton>
										<ToggleButton id="logo" className={AVATAR_FIT_TOGGLE_CLASS}>
											Logo 完整顯示
										</ToggleButton>
									</ToggleButtonGroup>
								</div>
								<span className="text-xs text-foreground-soft">
									{avatarFitMode === "logo"
										? suggestedAvatarFitMode === "logo"
											? "已自動去除透明外圍；可拖曳位置，並在圓形安全範圍內縮放。"
											: "完整保留圖片；可拖曳位置，並在圓形安全範圍內縮放。"
										: suggestedAvatarFitMode === "logo"
											? "目前會填滿圓形並可能裁切 Logo；可拖曳位置並調整縮放。"
											: "不透明照片預設填滿圓形；可拖曳位置並調整縮放。"}
								</span>
							</section>
						)}
						{aspectOptions && aspectOptions.length > 0 && !decodeFailed && (
							<section
								className="hk-heroui-brand-scope flex flex-col gap-2"
								aria-label="裁切比例"
							>
								<ToggleButtonGroup
									aria-label="裁切比例"
									selectionMode="single"
									disallowEmptySelection
									selectedKeys={
										selectedAspect === null
											? new Set<string>()
											: new Set([String(selectedAspect)])
									}
									onSelectionChange={(keys) => {
										const next = Number(Array.from(keys)[0]);
										if (Number.isFinite(next) && next > 0) {
											handlePickAspect(next);
										}
									}}
									isDisabled={processing}
									fullWidth
									size="md"
								>
									{aspectOptions.map((option) => (
										<ToggleButton key={option.label} id={String(option.aspect)}>
											{option.label}
										</ToggleButton>
									))}
								</ToggleButtonGroup>
								<span className="text-xs text-foreground-soft">
									已依原圖比例預選，可自行更換。下方預覽為動態牆卡片的實際取景。
								</span>
							</section>
						)}
						<div
							className={`relative h-[320px] w-full overflow-hidden rounded-[var(--hkui-radius-lg)] sm:h-[380px] ${
								isAvatarEditor ? "" : "bg-[var(--page-crop-canvas-bg-alt)]"
							}`}
							style={
								isAvatarEditor
									? avatarBackground === "transparent"
										? TRANSPARENCY_GRID_STYLE
										: { backgroundColor: avatarBackground }
									: undefined
							}
						>
							{decodeFailed ? (
								<div
									className={`flex h-full items-center justify-center px-6 text-center text-sm ${
										isAvatarEditor
											? "text-foreground-muted"
											: "text-[var(--color-neutral-cream)]/80"
									}`}
								>
									無法預覽這張圖片，確認後將直接使用原圖上傳。
								</div>
							) : (
								src &&
								naturalAspect !== null &&
								(isAvatarEditor && avatarFitMode === "logo" ? (
									<div className="absolute inset-3 flex items-center justify-center">
										<button
											type="button"
											disabled={processing || decodeFailed}
											aria-label="Logo 位置調整區；拖曳或使用方向鍵移動"
											aria-describedby="avatar-logo-position-help"
											data-avatar-logo-control="true"
											data-logo-offset-x={avatarLogoTransform.offsetX}
											data-logo-offset-y={avatarLogoTransform.offsetY}
											data-logo-scale={avatarLogoTransform.scale}
											onPointerDown={handleAvatarLogoPointerDown}
											onPointerMove={handleAvatarLogoPointerMove}
											onPointerUp={finishAvatarLogoDrag}
											onPointerCancel={finishAvatarLogoDrag}
											onLostPointerCapture={finishAvatarLogoDrag}
											onKeyDown={handleAvatarLogoKeyDown}
											onWheel={handleAvatarLogoWheel}
											className={`relative aspect-square h-auto w-full max-h-full max-w-full select-none rounded-full border-0 bg-transparent p-0 focus-visible:outline-none focus-visible:[box-shadow:var(--hkui-focus-ring)] disabled:cursor-not-allowed sm:h-full sm:w-auto ${
												isAvatarLogoDragging ? "cursor-grabbing" : "cursor-grab"
											}`}
											style={{ touchAction: "none" }}
										>
											<canvas
												ref={avatarMainCanvasRef}
												width={AVATAR_OUTPUT.width}
												height={AVATAR_OUTPUT.height}
												role="img"
												aria-label="Logo 完整顯示的圓形頭像預覽"
												className="pointer-events-none block size-full rounded-full"
												style={{
													boxShadow: "var(--page-crop-inset-shadow)",
												}}
											/>
											<span id="avatar-logo-position-help" className="sr-only">
												拖曳或使用方向鍵調整位置；按住 Shift 可加速移動。
											</span>
										</button>
									</div>
								) : (
									<Cropper
										image={src}
										crop={cropPos}
										zoom={zoom}
										minZoom={1}
										maxZoom={3}
										aspect={aspect}
										cropShape={config.shape === "circle" ? "round" : "rect"}
										showGrid
										restrictPosition
										onCropChange={setCropPos}
										onZoomChange={setZoom}
										onCropComplete={handleCropComplete}
										cropperProps={{
											"aria-label": isAvatarEditor
												? "頭像裁切區，可拖曳或使用方向鍵調整位置"
												: "圖片裁切區，可拖曳或使用方向鍵調整位置",
										}}
										mediaProps={{ alt: "待裁切的圖片" }}
									/>
								))
							)}
						</div>

						{isAvatarEditor && !decodeFailed && (
							<section
								className="flex flex-col gap-2"
								aria-labelledby="avatar-background-label"
							>
								<div className="flex flex-wrap items-baseline justify-between gap-2">
									<span
										id="avatar-background-label"
										className="text-sm font-medium text-foreground"
									>
										背景
									</span>
									<span className="text-xs text-foreground-soft">
										外框只顯示在介面，不會寫入圖片
									</span>
								</div>
								<div className="flex flex-wrap gap-2">
									{AVATAR_BACKGROUND_PRESETS.map((preset) => {
										const selected = avatarBackground === preset.value;
										return (
											<button
												type="button"
												key={preset.value}
												aria-pressed={selected}
												disabled={processing}
												onClick={() => setAvatarBackground(preset.value)}
												className={`inline-flex min-h-11 items-center gap-2 rounded-hkui-md border px-3 text-xs font-medium transition-colors motion-reduce:transition-none focus-visible:outline-none focus-visible:[box-shadow:var(--hkui-focus-ring)] disabled:cursor-not-allowed disabled:opacity-50 ${
													selected
														? "border-primary bg-primary/10 text-primary"
														: "border-hk-border-soft bg-hk-surface-card text-foreground hover:bg-hk-surface-muted"
												}`}
											>
												<span
													aria-hidden="true"
													className="h-4 w-4 shrink-0 rounded-full border border-black/15"
													style={
														preset.value === "transparent"
															? TRANSPARENCY_GRID_STYLE
															: { backgroundColor: preset.value }
													}
												/>
												{preset.label}
											</button>
										);
									})}
								</div>
								<div className="flex min-w-0 flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-foreground-soft">
									<span className="min-w-0 truncate" title={current?.name}>
										{current?.name ?? "讀取中"}
									</span>
									<span className="shrink-0 tabular-nums">
										輸出 {AVATAR_OUTPUT.width} × {AVATAR_OUTPUT.height} px
									</span>
								</div>
							</section>
						)}
					</>
				)}

				{!isPostEditor && !isSellerCoverEditor && !isProductEditor && (
					<div className="flex items-center gap-3">
						<label
							htmlFor="image-crop-zoom"
							className="shrink-0 text-xs font-semibold text-[var(--hkui-foreground-muted)]"
						>
							{usesAvatarLogoTransform ? "安全縮放" : "縮放"}
						</label>
						<input
							id="image-crop-zoom"
							type="range"
							min={activeMinZoom}
							max={activeMaxZoom}
							step={0.01}
							value={activeZoom}
							disabled={processing || decodeFailed}
							onChange={(event) => {
								const nextZoom = Number(event.target.value);
								if (usesAvatarLogoTransform) {
									updateAvatarLogoTransform({ scale: nextZoom });
								} else {
									setZoom(nextZoom);
								}
							}}
							className="h-11 flex-1 cursor-pointer accent-[var(--hkui-primary)] disabled:cursor-not-allowed [&::-moz-range-track]:h-1 [&::-webkit-slider-runnable-track]:h-1"
						/>
						<output
							htmlFor="image-crop-zoom"
							className="w-11 text-right text-xs text-[var(--hkui-foreground-muted)] tabular-nums"
						>
							{Math.round(activeZoom * 100)}%
						</output>
					</div>
				)}

				{!isPostEditor && surfaces.length > 0 && !decodeFailed && (
					<section className="flex flex-col gap-3 border-t border-divider pt-4">
						<span className="text-xs font-medium text-foreground-muted">
							{isSellerCoverEditor
								? "各裝置實際顯示效果"
								: isProductEditor
									? "上架後實際顯示效果"
									: isAvatarEditor
										? "實際尺寸預覽（84 / 44 / 30 px）"
										: "最終顯示範圍"}
						</span>
						<div className="flex flex-wrap items-end gap-4">
							{surfaces.map((surface, i) => {
								const height = surface.size ?? 84;
								const width = Math.round(height * surface.aspect);
								const dpr = 2;
								return (
									<figure
										key={surface.label}
										className="m-0 flex max-w-full flex-col items-start gap-1.5"
									>
										<canvas
											ref={(el) => {
												previewCanvasRefs.current[i] = el;
											}}
											width={width * dpr}
											height={height * dpr}
											style={{
												width,
												height,
												...(isAvatarEditor ? TRANSPARENCY_GRID_STYLE : {}),
												boxShadow: surface.circle
													? "var(--page-crop-inset-shadow)"
													: undefined,
											}}
											className={`block h-auto max-w-full border border-divider bg-default-100 ${
												surface.circle ? "rounded-full" : "rounded-hkui-md"
											}`}
										/>
										<figcaption className="text-xs text-foreground-soft">
											{surface.label}
											{isAvatarEditor ? ` · ${height}px` : ""}
										</figcaption>
									</figure>
								);
							})}
						</div>
					</section>
				)}
			</div>
		</Dialog>
	);
}

export default ImageCropDialog;
