"use client";

import AppIcon from "@/components/shared/AppIcon";
import { cn } from "@/lib/utils/cn";

export interface ImageFallbackProps {
	/**
	 * Sizing/shape classes mirrored from the image (e.g. `w-16 h-16 rounded-md`)
	 * so the placeholder occupies the exact same box as the image would have.
	 */
	className?: string;
	/** Mirrors next/image `fill` — absolutely fills the positioned parent. */
	fill?: boolean;
	/** Pixel box of the image, used to scale the icon and decide caption visibility. */
	width?: number;
	height?: number;
	/** Accessible + visible caption. Defaults to 圖片無法顯示. */
	label?: string;
}

/**
 * Graceful stand-in for an image that is missing or failed to load. Instead of
 * the browser's broken-image glyph, it renders the `image` icon inside the
 * image's footprint and (when there is room) a "圖片無法顯示" caption.
 */
export default function ImageFallback({
	className,
	fill,
	width,
	height,
	label = "圖片無法顯示",
}: ImageFallbackProps) {
	const box = Math.min(width || Infinity, height || Infinity);
	const finiteBox = Number.isFinite(box) ? box : 0;
	const iconSize = fill
		? 64
		: Math.min(64, Math.max(20, Math.round((finiteBox || 64) * 0.45)));
	// A caption needs vertical room; hide it on small thumbnails. When we don't
	// know the size (fill, or no dimensions) assume there is room.
	const showCaption = fill || finiteBox === 0 || finiteBox >= 120;

	return (
		<span
			role="img"
			aria-label={label}
			style={
				!fill && width && height
					? {
							inlineSize: `min(100%, ${width}px)`,
							aspectRatio: `${width} / ${height}`,
						}
					: undefined
			}
			className={cn(
				"flex flex-col items-center justify-center gap-1.5 bg-default-100 text-foreground-soft overflow-hidden",
				fill && "absolute inset-0",
				className,
			)}
		>
			{/* Decorative: the wrapper above is the `role="img"` that carries the
			    caption as its accessible name, and a nested name inside it is
			    unreachable anyway. */}
			<AppIcon name="image" size={iconSize} />
			{showCaption ? (
				<span className="px-2 text-center text-xs leading-tight">{label}</span>
			) : null}
		</span>
	);
}
