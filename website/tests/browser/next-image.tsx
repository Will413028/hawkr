import type { ImageProps } from "next/image";
import type { CSSProperties } from "react";

// 取代 `next/image`：它是 CJS，經 Vite 預先打包後 default 匯入成物件（`{ default: Image }`），
// 渲染會丟 "Element type is invalid"。這裡只還原 DOM 契約：src 原樣輸出（AppImage 對字串 src
// 一律 unoptimized，真元件也是原樣輸出）、fill 的絕對定位，以及不外洩到 <img> 的 next 專屬屬性。
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
				inset: 0,
				color: "transparent",
			}
		: undefined;
	return (
		<img
			{...rest}
			alt={alt}
			src={resolved}
			width={fill ? undefined : width}
			height={fill ? undefined : height}
			loading={priority ? "eager" : (loading ?? "lazy")}
			style={{ ...fillStyle, ...style }}
		/>
	);
}
