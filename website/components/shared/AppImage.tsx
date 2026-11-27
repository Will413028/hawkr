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
	const fromApi = "image" in allProps;
	const src: ImageProps["src"] = fromApi
		? (imageUri(image, preset) ?? "")
		: (legacySrc ?? "");
	const [errored, setErrored] = React.useState(false);
	const [lastSrc, setLastSrc] = React.useState(src);
	if (lastSrc !== src) {
		setLastSrc(src);
		setErrored(false);
	}

	if (!hasUsableSrc(src) || errored) {
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

	const resolvedSrc =
		!fromApi && typeof src === "string" ? buildImageUrl(src, preset) : src;
	const shouldBypassOptimization =
		typeof src === "string" || unoptimized === true;
	const handleError: React.ReactEventHandler<HTMLImageElement> = (event) => {
		if (onError) {
			onError(event);
			return;
		}
		setErrored(true);
	};

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
