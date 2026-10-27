import type { CropAreaPixels } from "@/lib/media/crop";

export type SellerCoverFillMode = "cover" | "contain";

export const SELLER_COVER_ASPECT = 4;

export const SELLER_COVER_OUTPUT = {
	width: 1600,
	height: 400,
} as const;

export const SELLER_COVER_BACKGROUND_PRESETS = [
	{ label: "奶油白", color: "#FFFDF7" },
	{ label: "淺沙", color: "#FDF1E1" },
	{ label: "磚橘", color: "#B8441F" },
	{ label: "品牌橘", color: "#E4572E" },
	{ label: "暖陽橘", color: "#F2A541" },
	{ label: "晴空", color: "#9FCCDD" },
] as const;

export const SELLER_COVER_DEFAULT_BACKGROUND =
	SELLER_COVER_BACKGROUND_PRESETS[1].color;

/**
 * A cover/4:1 mismatch of 35% or more is large enough that filling the frame
 * hides meaningful content. Transparent artwork should also remain complete.
 */
export function shouldContainSellerCover(
	naturalAspect: number,
	hasTransparency: boolean,
): boolean {
	if (hasTransparency) return true;
	if (!Number.isFinite(naturalAspect) || naturalAspect <= 0) return false;
	const mismatch = Math.max(
		naturalAspect / SELLER_COVER_ASPECT,
		SELLER_COVER_ASPECT / naturalAspect,
	);
	return mismatch >= 1.35;
}

export interface ImageRect {
	x: number;
	y: number;
	width: number;
	height: number;
}

export function calculateContainRect(
	sourceWidth: number,
	sourceHeight: number,
	targetWidth: number,
	targetHeight: number,
): ImageRect {
	const scale = Math.min(
		targetWidth / sourceWidth,
		targetHeight / sourceHeight,
	);
	const width = sourceWidth * scale;
	const height = sourceHeight * scale;
	return {
		x: (targetWidth - width) / 2,
		y: (targetHeight - height) / 2,
		width,
		height,
	};
}

export function calculateCoverCrop(
	sourceWidth: number,
	sourceHeight: number,
	targetAspect: number,
): CropAreaPixels {
	const sourceAspect = sourceWidth / sourceHeight;
	if (sourceAspect > targetAspect) {
		const width = sourceHeight * targetAspect;
		return {
			x: (sourceWidth - width) / 2,
			y: 0,
			width,
			height: sourceHeight,
		};
	}
	const height = sourceWidth / targetAspect;
	return {
		x: 0,
		y: (sourceHeight - height) / 2,
		width: sourceWidth,
		height,
	};
}

interface DrawSellerCoverOptions {
	mode: SellerCoverFillMode;
	backgroundColor: string;
	sourceArea?: CropAreaPixels | null;
	targetWidth?: number;
	targetHeight?: number;
}

/** Paints the canonical seller cover that is persisted by the upload flow. */
export function drawSellerCover(
	ctx: CanvasRenderingContext2D,
	image: CanvasImageSource,
	sourceWidth: number,
	sourceHeight: number,
	options: DrawSellerCoverOptions,
): void {
	const targetWidth = options.targetWidth ?? SELLER_COVER_OUTPUT.width;
	const targetHeight = options.targetHeight ?? SELLER_COVER_OUTPUT.height;

	ctx.clearRect(0, 0, targetWidth, targetHeight);
	ctx.fillStyle = options.backgroundColor;
	ctx.fillRect(0, 0, targetWidth, targetHeight);
	ctx.imageSmoothingEnabled = true;
	ctx.imageSmoothingQuality = "high";

	if (options.mode === "contain") {
		const rect = calculateContainRect(
			sourceWidth,
			sourceHeight,
			targetWidth,
			targetHeight,
		);
		ctx.drawImage(image, rect.x, rect.y, rect.width, rect.height);
		return;
	}

	const area =
		options.sourceArea ??
		calculateCoverCrop(sourceWidth, sourceHeight, targetWidth / targetHeight);
	ctx.drawImage(
		image,
		area.x,
		area.y,
		area.width,
		area.height,
		0,
		0,
		targetWidth,
		targetHeight,
	);
}

export async function createSellerCoverFile(
	file: File,
	image: HTMLImageElement,
	options: DrawSellerCoverOptions,
): Promise<File> {
	const canvas = document.createElement("canvas");
	canvas.width = SELLER_COVER_OUTPUT.width;
	canvas.height = SELLER_COVER_OUTPUT.height;
	const ctx = canvas.getContext("2d");
	if (!ctx) throw new Error("此瀏覽器不支援封面圖片處理");

	drawSellerCover(ctx, image, image.naturalWidth, image.naturalHeight, options);
	const blob = await new Promise<Blob | null>((resolve) => {
		canvas.toBlob(resolve, "image/webp", 0.9);
	});
	if (!blob) throw new Error("封面圖片處理失敗，請再試一次");

	const base = file.name.replace(/\.[^.]+$/, "") || "seller-cover";
	const extension = blob.type === "image/webp" ? "webp" : "png";
	return new File([blob], `${base}-cover.${extension}`, {
		type: blob.type || "image/png",
		lastModified: file.lastModified,
	});
}
