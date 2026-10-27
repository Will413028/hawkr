"use client";

import type { Image as ApiImage } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import Image, { type ImageProps } from "next/image";
import * as React from "react";
import ImageFallback from "@/components/shared/ImageFallback";
import { buildImageUrl, type ImagePreset } from "@/lib/media/cdn";
import { imageUri } from "@/lib/media/image";

type CommonProps = Omit<ImageProps, "width" | "height" | "src"> & {
	width?: number;
	height?: number;
	/** Override the caption shown by the fallback when the image can't load. */
	fallbackLabel?: string;
};

/** 舊 REST 的圖片（object key 或網址），依 `preset` 組成媒體網址（buildImageUrl）。 */
type LegacyImageProps = CommonProps & {
	src?: ImageProps["src"];
	image?: never;
	/**
	 * Named media-gateway derivative. Card surfaces should request the smallest
	 * preset that preserves their rendered quality instead of downloading the
	 * original object. Gate-off deployments keep the legacy CDN URL unchanged.
	 */
	preset?: ImagePreset;
};

/**
 * 對外 API（Connect）回的圖片：只把 `uri_template` 的 `{preset}` 換成 `preset`，不經過 buildImageUrl
 * （ADR 2026-09-25-api-image-uri-template D3）；值是 undefined 時顯示沒有圖片。preset 必填：API 的圖片
 * 不以原圖為預設（同一份 ADR 不選「以原圖為預設網址」的理由）。
 */
type ApiImageProps = CommonProps & {
	src?: never;
	image: ApiImage | undefined;
	preset: ImagePreset;
};

type AppImageProps = LegacyImageProps | ApiImageProps;

function hasUsableSrc(src: ImageProps["src"]): boolean {
	if (typeof src === "string") return src.trim().length > 0;
	return src != null;
}

/**
 * `fill` has no intrinsic size, so it genuinely needs a `sizes` hint. A fixed
 * width/height box does not: telling the browser a 64px thumbnail may be
 * `100vw` makes it pick the largest derivative it can find for a slot it
 * already knows the size of.
 */
const FILL_SIZES_DEFAULT = "(max-width: 768px) 100vw, 800px";

export default function AppImage(allProps: AppImageProps) {
	const {
		width,
		height,
		fill,
		sizes,
		src: legacySrc,
		image,
		unoptimized,
		className,
		onError,
		preset = "original",
		fallbackLabel,
		...props
	} = allProps;
	// 有 image 這個屬性（即使值是 undefined）就是 API 的圖片：網址已經由伺服器組好，只換 preset。
	const fromApi = "image" in allProps;
	const src: ImageProps["src"] = fromApi
		? (imageUri(image, preset) ?? "")
		: (legacySrc ?? "");
	const [errored, setErrored] = React.useState(false);
	// Reset the error flag when the caller swaps in a new src (render-phase
	// state sync, no effect needed — same pattern as <Avatar>).
	const [lastSrc, setLastSrc] = React.useState(src);
	if (lastSrc !== src) {
		setLastSrc(src);
		setErrored(false);
	}

	if (!hasUsableSrc(src) || errored) {
		// Distinguish the two fallback reasons: an empty src means the record
		// simply has no image yet (an empty state), while `errored` means a real
		// image URL failed to load. Only the latter is "圖片無法顯示"; an empty
		// state reads as "尚無圖片" (mirrors the card's "尚無評價" wording).
		const label = fallbackLabel ?? (errored ? "圖片無法顯示" : "尚無圖片");
		return (
			<ImageFallback
				className={className}
				fill={fill}
				width={width}
				height={height}
				label={label}
			/>
		);
	}

	// The media gateway exposes named presets, not arbitrary Next.js widths.
	// Resolve string keys once and render the requested named preset unoptimized;
	// otherwise Next asks the custom loader for a width ladder it cannot produce
	// and emits a warning for every image. Static imports can still use Next's
	// build-time image metadata normally. buildImageUrl also restores an existing
	// /i/{preset}/ URL to its object key before applying `preset`, so an explicit
	// card/thumb request can never be rewritten back to the original object.
	const resolvedSrc =
		!fromApi && typeof src === "string" ? buildImageUrl(src, preset) : src;
	const shouldBypassOptimization =
		typeof src === "string" || unoptimized === true;
	// When the caller supplies its own onError it owns the fallback (e.g.
	// swapping img.src to a domain-specific placeholder like a seller avatar),
	// so we defer to it and skip the fallback. Only images with no
	// fallback of their own fall through to the "圖片無法顯示" placeholder.
	const handleError: React.ReactEventHandler<HTMLImageElement> = (event) => {
		if (onError) {
			onError(event);
			return;
		}
		setErrored(true);
	};

	// `fill` mode is mutually exclusive with width/height in next/image,
	// so skip the defaults when caller opts into fill.
	if (fill) {
		return (
			<Image
				fill
				sizes={sizes ?? FILL_SIZES_DEFAULT}
				src={resolvedSrc}
				unoptimized={shouldBypassOptimization}
				className={className}
				onError={handleError}
				{...props}
			/>
		);
	}
	if (
		process.env.NODE_ENV !== "production" &&
		(width === undefined || height === undefined)
	) {
		// 800×600 is not this image's shape — it is a guess, and it reserves a 4:3
		// box that the real picture then has to be squeezed into or repaint over.
		// That guess IS the layout shift the width/height pair exists to prevent.
		console.warn(
			`AppImage: ${String(src)} was rendered without width/height and without \`fill\`. ` +
				"Pass the rendered box (or `fill` inside a positioned parent); the 800×600 " +
				"placeholder below is a fallback, not a size.",
		);
	}
	return (
		<Image
			width={width ?? 800}
			height={height ?? 600}
			sizes={sizes}
			src={resolvedSrc}
			unoptimized={shouldBypassOptimization}
			className={className}
			onError={handleError}
			{...props}
		/>
	);
}
