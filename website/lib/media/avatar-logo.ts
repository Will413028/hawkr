import type { CropAreaPixels } from "@/lib/media/crop";

export type AvatarFitMode = "fill" | "logo";

export const AVATAR_OUTPUT = {
	width: 640,
	height: 640,
} as const;

export const AVATAR_LOGO_SAFE_INSET = 0.15;

/** Logo mode starts at 100%; users may zoom out while keeping the whole mark visible. */
export const AVATAR_LOGO_MIN_SCALE = 0.6;

export const AVATAR_BACKGROUND_PRESETS = [
	{ label: "品牌暖白", value: "#FFFDF7" },
	{ label: "白色", value: "#FFFFFF" },
	{ label: "磚橘", value: "#B8441F" },
	{ label: "保留透明", value: "transparent" },
] as const;

export type AvatarBackground =
	(typeof AVATAR_BACKGROUND_PRESETS)[number]["value"];

export const AVATAR_DEFAULT_BACKGROUND: AvatarBackground = "#FFFDF7";

export interface ImageBounds {
	x: number;
	y: number;
	width: number;
	height: number;
}

export interface AvatarLogoTransform {
	offsetX: number;
	offsetY: number;
	scale: number;
}

export const AVATAR_LOGO_DEFAULT_TRANSFORM: AvatarLogoTransform = {
	offsetX: 0,
	offsetY: 0,
	scale: 1,
};

export interface AlphaAnalysis {
	hasTransparency: boolean;
	transparentPixelRatio: number;
	visiblePixelRatio: number;
	visibleBounds: ImageBounds | null;
	visibleBoundsAreaRatio: number;
}

/**
 * Finds the visible alpha bounds without treating antialiased edge pixels as
 * empty. This is intentionally a pure helper so fitting decisions stay
 * covered in Node tests as well as in the browser editor.
 */
export function analyzeAlphaPixels(
	pixels: ArrayLike<number>,
	width: number,
	height: number,
	visibleAlphaThreshold = 8,
): AlphaAnalysis {
	if (
		!Number.isInteger(width) ||
		!Number.isInteger(height) ||
		width <= 0 ||
		height <= 0 ||
		pixels.length < width * height * 4
	) {
		throw new RangeError("Alpha 像素尺寸不正確");
	}

	let visiblePixels = 0;
	let transparentPixels = 0;
	let minX = width;
	let minY = height;
	let maxX = -1;
	let maxY = -1;

	for (let y = 0; y < height; y += 1) {
		for (let x = 0; x < width; x += 1) {
			const alpha = pixels[(y * width + x) * 4 + 3] ?? 0;
			if (alpha < 250) transparentPixels += 1;
			if (alpha <= visibleAlphaThreshold) continue;
			visiblePixels += 1;
			minX = Math.min(minX, x);
			minY = Math.min(minY, y);
			maxX = Math.max(maxX, x);
			maxY = Math.max(maxY, y);
		}
	}

	const totalPixels = width * height;
	const visibleBounds =
		maxX < minX || maxY < minY
			? null
			: {
					x: minX,
					y: minY,
					width: maxX - minX + 1,
					height: maxY - minY + 1,
				};

	return {
		hasTransparency: transparentPixels > 0,
		transparentPixelRatio: transparentPixels / totalPixels,
		visiblePixelRatio: visiblePixels / totalPixels,
		visibleBounds,
		visibleBoundsAreaRatio: visibleBounds
			? (visibleBounds.width * visibleBounds.height) / totalPixels
			: 0,
	};
}

/** Samples a local upload and maps its alpha bounds back to natural pixels. */
export function analyzeImageAlpha(
	image: HTMLImageElement,
	maxSampleSide = 256,
): AlphaAnalysis {
	const scale = Math.min(
		1,
		maxSampleSide / Math.max(image.naturalWidth, image.naturalHeight),
	);
	const sampleWidth = Math.max(1, Math.round(image.naturalWidth * scale));
	const sampleHeight = Math.max(1, Math.round(image.naturalHeight * scale));
	const canvas = document.createElement("canvas");
	canvas.width = sampleWidth;
	canvas.height = sampleHeight;
	const ctx = canvas.getContext("2d", { willReadFrequently: true });
	if (!ctx) throw new Error("此瀏覽器不支援透明背景分析");
	ctx.drawImage(image, 0, 0, sampleWidth, sampleHeight);
	const sampled = analyzeAlphaPixels(
		ctx.getImageData(0, 0, sampleWidth, sampleHeight).data,
		sampleWidth,
		sampleHeight,
	);
	if (!sampled.visibleBounds) return sampled;

	const right = Math.min(
		image.naturalWidth,
		Math.ceil(
			((sampled.visibleBounds.x + sampled.visibleBounds.width) / sampleWidth) *
				image.naturalWidth,
		),
	);
	const bottom = Math.min(
		image.naturalHeight,
		Math.ceil(
			((sampled.visibleBounds.y + sampled.visibleBounds.height) /
				sampleHeight) *
				image.naturalHeight,
		),
	);
	const x = Math.floor(
		(sampled.visibleBounds.x / sampleWidth) * image.naturalWidth,
	);
	const y = Math.floor(
		(sampled.visibleBounds.y / sampleHeight) * image.naturalHeight,
	);

	return {
		...sampled,
		visibleBounds: {
			x,
			y,
			width: Math.max(1, right - x),
			height: Math.max(1, bottom - y),
		},
	};
}

/**
 * Transparent or visibly sparse uploads behave like logos, so they start in
 * complete-image mode instead of being enlarged and clipped like a portrait.
 */
export function shouldUseAvatarLogoFit(
	sourceType: string,
	analysis: AlphaAnalysis,
): boolean {
	const alphaCapable =
		sourceType.toLowerCase() === "image/png" ||
		sourceType.toLowerCase() === "image/webp";
	if (!alphaCapable && !analysis.hasTransparency) return false;
	if (!analysis.visibleBounds) return true;
	return (
		analysis.transparentPixelRatio >= 0.06 ||
		analysis.visibleBoundsAreaRatio <= 0.78 ||
		analysis.visiblePixelRatio <= 0.72
	);
}

export function calculateAvatarLogoRect(
	sourceWidth: number,
	sourceHeight: number,
	targetWidth: number = AVATAR_OUTPUT.width,
	targetHeight: number = AVATAR_OUTPUT.height,
	safeInset = AVATAR_LOGO_SAFE_INSET,
): ImageBounds {
	if (sourceWidth <= 0 || sourceHeight <= 0) {
		return { x: 0, y: 0, width: 0, height: 0 };
	}
	const inset = Math.min(0.45, Math.max(0, safeInset));
	const availableWidth = targetWidth * (1 - inset * 2);
	const availableHeight = targetHeight * (1 - inset * 2);
	const scale = Math.min(
		availableWidth / sourceWidth,
		availableHeight / sourceHeight,
	);
	const width = Math.max(1, Math.round(sourceWidth * scale));
	const height = Math.max(1, Math.round(sourceHeight * scale));
	return {
		x: Math.round((targetWidth - width) / 2),
		y: Math.round((targetHeight - height) / 2),
		width,
		height,
	};
}

/**
 * Largest scale that keeps every corner of the visible logo inside the final
 * circular mask. Wide and tall marks can safely grow a little more than square
 * ones, while the default 15% inset remains the initial 100% framing.
 */
export function calculateAvatarLogoMaxScale(
	sourceWidth: number,
	sourceHeight: number,
	targetWidth: number = AVATAR_OUTPUT.width,
	targetHeight: number = AVATAR_OUTPUT.height,
): number {
	const base = calculateAvatarLogoRect(
		sourceWidth,
		sourceHeight,
		targetWidth,
		targetHeight,
	);
	if (base.width <= 0 || base.height <= 0) return 1;
	const radius = Math.min(targetWidth, targetHeight) / 2;
	const halfDiagonal = Math.hypot(base.width / 2, base.height / 2);
	if (halfDiagonal <= 0) return 1;
	return Math.max(1, radius / halfDiagonal);
}

/**
 * Keeps a panned/scaled logo completely inside the circular output. The radial
 * clamp is deliberately conservative: every corner stays inside the mask, so
 * the editor can promise "完整顯示" without preview/output drift.
 */
export function constrainAvatarLogoTransform(
	sourceWidth: number,
	sourceHeight: number,
	transform: Partial<AvatarLogoTransform>,
	targetWidth: number = AVATAR_OUTPUT.width,
	targetHeight: number = AVATAR_OUTPUT.height,
): AvatarLogoTransform {
	const rawScale = Number.isFinite(transform.scale)
		? (transform.scale ?? 1)
		: 1;
	const maxScale = calculateAvatarLogoMaxScale(
		sourceWidth,
		sourceHeight,
		targetWidth,
		targetHeight,
	);
	const scale = Math.min(maxScale, Math.max(AVATAR_LOGO_MIN_SCALE, rawScale));
	const base = calculateAvatarLogoRect(
		sourceWidth,
		sourceHeight,
		targetWidth,
		targetHeight,
	);
	const radius = Math.min(targetWidth, targetHeight) / 2;
	const halfDiagonal = Math.hypot(
		(base.width * scale) / 2,
		(base.height * scale) / 2,
	);
	const maxOffset = Math.max(0, radius - halfDiagonal);
	let offsetX = Number.isFinite(transform.offsetX)
		? (transform.offsetX ?? 0)
		: 0;
	let offsetY = Number.isFinite(transform.offsetY)
		? (transform.offsetY ?? 0)
		: 0;
	const offsetLength = Math.hypot(offsetX, offsetY);
	if (offsetLength > maxOffset && offsetLength > 0) {
		const ratio = maxOffset / offsetLength;
		offsetX *= ratio;
		offsetY *= ratio;
	}
	return { offsetX, offsetY, scale };
}

export function calculateAvatarLogoDestination(
	sourceWidth: number,
	sourceHeight: number,
	transform: Partial<AvatarLogoTransform> = AVATAR_LOGO_DEFAULT_TRANSFORM,
	targetWidth: number = AVATAR_OUTPUT.width,
	targetHeight: number = AVATAR_OUTPUT.height,
): ImageBounds {
	const base = calculateAvatarLogoRect(
		sourceWidth,
		sourceHeight,
		targetWidth,
		targetHeight,
	);
	const constrained = constrainAvatarLogoTransform(
		sourceWidth,
		sourceHeight,
		transform,
		targetWidth,
		targetHeight,
	);
	const width = base.width * constrained.scale;
	const height = base.height * constrained.scale;
	return {
		x: (targetWidth - width) / 2 + constrained.offsetX,
		y: (targetHeight - height) / 2 + constrained.offsetY,
		width,
		height,
	};
}

function calculateSquareCrop(
	sourceWidth: number,
	sourceHeight: number,
): CropAreaPixels {
	const size = Math.min(sourceWidth, sourceHeight);
	return {
		x: (sourceWidth - size) / 2,
		y: (sourceHeight - size) / 2,
		width: size,
		height: size,
	};
}

export interface DrawAvatarOptions {
	mode: AvatarFitMode;
	background: AvatarBackground;
	sourceArea?: CropAreaPixels | null;
	visibleBounds?: ImageBounds | null;
	logoTransform?: Partial<AvatarLogoTransform>;
	targetWidth?: number;
	targetHeight?: number;
}

/** Paints the canonical square file. The circular ring stays in CSS. */
export function drawAvatarImage(
	ctx: CanvasRenderingContext2D,
	image: CanvasImageSource,
	sourceWidth: number,
	sourceHeight: number,
	options: DrawAvatarOptions,
): void {
	const targetWidth = options.targetWidth ?? AVATAR_OUTPUT.width;
	const targetHeight = options.targetHeight ?? AVATAR_OUTPUT.height;
	ctx.clearRect(0, 0, targetWidth, targetHeight);
	if (options.background !== "transparent") {
		ctx.fillStyle = options.background;
		ctx.fillRect(0, 0, targetWidth, targetHeight);
	}
	ctx.imageSmoothingEnabled = true;
	ctx.imageSmoothingQuality = "high";

	if (options.mode === "logo") {
		const source = options.visibleBounds ?? {
			x: 0,
			y: 0,
			width: sourceWidth,
			height: sourceHeight,
		};
		if (source.width <= 0 || source.height <= 0) return;
		const destination = calculateAvatarLogoDestination(
			source.width,
			source.height,
			options.logoTransform,
			targetWidth,
			targetHeight,
		);
		ctx.drawImage(
			image,
			source.x,
			source.y,
			source.width,
			source.height,
			destination.x,
			destination.y,
			destination.width,
			destination.height,
		);
		return;
	}

	const source =
		options.sourceArea ?? calculateSquareCrop(sourceWidth, sourceHeight);
	ctx.drawImage(
		image,
		source.x,
		source.y,
		source.width,
		source.height,
		0,
		0,
		targetWidth,
		targetHeight,
	);
}

export async function createAvatarFile(
	file: File,
	image: HTMLImageElement,
	options: DrawAvatarOptions,
): Promise<File> {
	const canvas = document.createElement("canvas");
	canvas.width = AVATAR_OUTPUT.width;
	canvas.height = AVATAR_OUTPUT.height;
	const ctx = canvas.getContext("2d");
	if (!ctx) throw new Error("此瀏覽器不支援頭像圖片處理");

	drawAvatarImage(ctx, image, image.naturalWidth, image.naturalHeight, options);
	const blob = await new Promise<Blob | null>((resolve) => {
		canvas.toBlob(resolve, "image/png");
	});
	if (!blob) throw new Error("頭像圖片處理失敗，請再試一次");

	const base = file.name.replace(/\.[^.]+$/, "") || "avatar";
	return new File([blob], `${base}-avatar.png`, {
		type: "image/png",
		lastModified: file.lastModified,
	});
}
