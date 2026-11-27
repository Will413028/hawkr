import type { ImageProps } from "next/image";
import type { CSSProperties } from "react";

export default function NextImageStub({
	src,
	alt,
	width,
	height,
	fill,
	style,
	loader: _loader,
	quality: _quality,
	priority,
	preload,
	sizes: _sizes,
	decoding = "async",
	loading,
	placeholder: _placeholder,
	blurDataURL: _blurDataURL,
	unoptimized: _unoptimized,
	overrideSrc: _overrideSrc,
	onLoadingComplete: _onLoadingComplete,
	...rest
}: ImageProps) {
	const resolved =
		typeof src === "string"
			? src
			: "src" in src
				? src.src
				: (src as { default: { src: string } }).default.src;
	const fillStyle: CSSProperties | undefined = fill
		? {
				position: "absolute",
				height: "100%",
				width: "100%",
				left: 0,
				top: 0,
				right: 0,
				bottom: 0,
			}
		: undefined;
	const isLazy =
		!priority &&
		!preload &&
		(loading === "lazy" || loading === undefined) &&
		!/^(data|blob):/.test(resolved);
	return (
		<img
			{...rest}
			alt={alt}
			src={resolved}
			width={fill ? undefined : width}
			height={fill ? undefined : height}
			loading={isLazy ? "lazy" : loading}
			decoding={decoding}
			data-nimg={fill ? "fill" : "1"}
			style={{ ...fillStyle, color: "transparent", ...style }}
		/>
	);
}
