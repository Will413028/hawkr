export const PRODUCT_IMAGE_ASPECT = 4 / 3;
export const PRODUCT_IMAGE_MAX_EDGE = 1600;
export const PRODUCT_IMAGE_BACKGROUND = "#F5F1E8";
export const PRODUCT_IMAGE_MIN_SCALE = 1;
export const PRODUCT_IMAGE_MAX_SCALE = 4;

/**
 * Product-image positioning is stored relative to the canonical output frame,
 * so the editor preview and the encoded 4:3 canvas stay identical at every
 * viewport size.
 */
export interface ProductImageTransform {
	/** 1 = the whole source is visible inside the frame. */
	scale: number;
	/** Horizontal translation as a fraction of the output width. */
	offsetX: number;
	/** Vertical translation as a fraction of the output height. */
	offsetY: number;
}

export const PRODUCT_IMAGE_DEFAULT_TRANSFORM: ProductImageTransform = {
	scale: PRODUCT_IMAGE_MIN_SCALE,
	offsetX: 0,
	offsetY: 0,
};

export interface ProductImageOutputSize {
	width: number;
	height: number;
}

export interface ProductImageRect extends ProductImageOutputSize {
	x: number;
	y: number;
}

function assertPositiveDimension(value: number): void {
	if (!Number.isFinite(value) || value <= 0) {
		throw new Error("商品圖片尺寸無效");
	}
}

/**
 * Keep the canonical 4:3 product canvas no larger than the source's longest
 * edge. Portrait and square uploads are therefore reduced to fit rather than
 * being enlarged just to manufacture a 1600px file.
 */
export function calculateProductImageOutputSize(
	sourceWidth: number,
	sourceHeight: number,
	maxEdge = PRODUCT_IMAGE_MAX_EDGE,
): ProductImageOutputSize {
	if (
		!Number.isFinite(sourceWidth) ||
		!Number.isFinite(sourceHeight) ||
		!Number.isFinite(maxEdge) ||
		sourceWidth <= 0 ||
		sourceHeight <= 0 ||
		maxEdge <= 0
	) {
		throw new Error("商品圖片尺寸無效");
	}

	const width = Math.max(
		1,
		Math.round(Math.min(maxEdge, Math.max(sourceWidth, sourceHeight))),
	);
	return {
		width,
		height: Math.max(1, Math.round(width / PRODUCT_IMAGE_ASPECT)),
	};
}

/**
 * react-easy-crop lays media out as `cover` before applying its zoom. This is
 * the raw zoom at which that covered media becomes fully contained instead.
 * The UI presents this value as 100%, hiding implementation geometry from the
 * seller.
 */
export function calculateProductImageContainZoom(
	sourceAspect: number,
	targetAspect = PRODUCT_IMAGE_ASPECT,
): number {
	assertPositiveDimension(sourceAspect);
	assertPositiveDimension(targetAspect);
	return Math.min(1, sourceAspect / targetAspect, targetAspect / sourceAspect);
}

/** Full-image destination rect. Every source pixel remains visible. */
export function calculateProductContainRect(
	sourceWidth: number,
	sourceHeight: number,
	targetWidth: number,
	targetHeight: number,
): ProductImageRect {
	assertPositiveDimension(sourceWidth);
	assertPositiveDimension(sourceHeight);
	assertPositiveDimension(targetWidth);
	assertPositiveDimension(targetHeight);
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

/**
 * Destination rectangle for the seller's live viewport. The canvas clips any
 * part outside its 4:3 bounds, while the default transform is exactly the full
 * source rect returned by `calculateProductContainRect`.
 */
export function calculateProductImageDestinationRect(
	sourceWidth: number,
	sourceHeight: number,
	targetWidth: number,
	targetHeight: number,
	transform: ProductImageTransform = PRODUCT_IMAGE_DEFAULT_TRANSFORM,
): ProductImageRect {
	const contained = calculateProductContainRect(
		sourceWidth,
		sourceHeight,
		targetWidth,
		targetHeight,
	);
	if (
		!Number.isFinite(transform.scale) ||
		!Number.isFinite(transform.offsetX) ||
		!Number.isFinite(transform.offsetY) ||
		transform.scale < PRODUCT_IMAGE_MIN_SCALE ||
		transform.scale > PRODUCT_IMAGE_MAX_SCALE
	) {
		throw new Error("商品圖片取景設定無效");
	}

	const width = contained.width * transform.scale;
	const height = contained.height * transform.scale;
	return {
		x: (targetWidth - width) / 2 + transform.offsetX * targetWidth,
		y: (targetHeight - height) / 2 + transform.offsetY * targetHeight,
		width,
		height,
	};
}

export function productImageTransformWillCrop(
	sourceWidth: number,
	sourceHeight: number,
	targetWidth: number,
	targetHeight: number,
	transform: ProductImageTransform,
): boolean {
	const destination = calculateProductImageDestinationRect(
		sourceWidth,
		sourceHeight,
		targetWidth,
		targetHeight,
		transform,
	);
	const epsilon = 0.5;
	return (
		destination.x < -epsilon ||
		destination.y < -epsilon ||
		destination.x + destination.width > targetWidth + epsilon ||
		destination.y + destination.height > targetHeight + epsilon
	);
}

interface DrawProductImageOptions {
	transform?: ProductImageTransform;
	targetWidth: number;
	targetHeight: number;
}

/**
 * Paints one 4:3 product image. Complete-image mode uses the design system's
 * muted-paper surface behind the untouched foreground, so every source edge
 * is obvious and packaging text is never repeated or mistaken for a crop.
 */
export function drawProductImage(
	ctx: CanvasRenderingContext2D,
	image: CanvasImageSource,
	sourceWidth: number,
	sourceHeight: number,
	options: DrawProductImageOptions,
): void {
	const { targetWidth, targetHeight } = options;
	ctx.clearRect(0, 0, targetWidth, targetHeight);
	ctx.imageSmoothingEnabled = true;
	ctx.imageSmoothingQuality = "high";

	ctx.fillStyle = PRODUCT_IMAGE_BACKGROUND;
	ctx.fillRect(0, 0, targetWidth, targetHeight);

	const destination = calculateProductImageDestinationRect(
		sourceWidth,
		sourceHeight,
		targetWidth,
		targetHeight,
		options.transform,
	);
	ctx.drawImage(
		image,
		0,
		0,
		sourceWidth,
		sourceHeight,
		destination.x,
		destination.y,
		destination.width,
		destination.height,
	);
}

export async function createProductImageFile(
	file: File,
	image: HTMLImageElement,
	options: {
		transform?: ProductImageTransform;
		maxEdge?: number;
	},
): Promise<File> {
	const output = calculateProductImageOutputSize(
		image.naturalWidth,
		image.naturalHeight,
		options.maxEdge,
	);
	const canvas = document.createElement("canvas");
	canvas.width = output.width;
	canvas.height = output.height;
	const ctx = canvas.getContext("2d");
	if (!ctx) throw new Error("此瀏覽器不支援商品圖片處理");

	drawProductImage(ctx, image, image.naturalWidth, image.naturalHeight, {
		transform: options.transform,
		targetWidth: output.width,
		targetHeight: output.height,
	});

	const blob = await new Promise<Blob | null>((resolve) => {
		canvas.toBlob(resolve, "image/webp", 0.9);
	});
	if (!blob) throw new Error("商品圖片處理失敗，請再試一次");

	const base = file.name.replace(/\.[^.]+$/, "") || "product-image";
	return new File([blob], `${base}-product.webp`, {
		type: blob.type || "image/webp",
		lastModified: file.lastModified,
	});
}
