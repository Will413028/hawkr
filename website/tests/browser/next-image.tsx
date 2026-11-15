import type { ImageProps } from "next/image";
import type { CSSProperties } from "react";

// 取代 `next/image`：它是 CJS，經 Vite 預先打包後 default 匯入成物件（`{ default: Image }`），
// 渲染會丟 "Element type is invalid"。這裡只還原 DOM 契約：src 原樣輸出（AppImage 對字串 src
// 一律 unoptimized，真元件也是原樣輸出）、fill 的絕對定位，以及不外洩到 <img> 的 next 專屬屬性；輸出對齊 next 16 的 get-img-props／image-component。
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
	// next 16 的 getImgProps：priority／preload 不給 loading（也不推導 fetchPriority），
	// data:／blob: 不 lazy；其餘預設 lazy。unoptimized 時 sizes／srcSet 都是 undefined。
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
