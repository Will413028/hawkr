import { buildImageUrl } from "./cdn";

/**
 * Custom `next/image` loader (wired via `images.loaderFile` in next.config.ts).
 * next/image calls this once per srcset width with `{ src, width, quality }`.
 *
 * Per-width resizing is no longer expressible: R2 + the image gateway serve
 * fixed named presets only (design book 49 §4.4 forbids width/height/quality/
 * format from the frontend), and the old Aliyun `x-oss-process` pipeline is
 * gone. So we ignore `width`/`quality` and return the ORIGINAL object:
 *
 *   Gate OFF — the source object off the CDN (no processing).
 *   Gate ON  — `{mediaBaseUrl}/i/original/{objectKey}` via the Worker.
 *
 * next/image dedupes the identical URL across srcset widths, so the browser
 * fetches one image. Surfaces that need a smaller derivative should render a
 * raw <img> through `buildImageUrl(value, preset)` with a §4.4 preset instead
 * of relying on next/image's width ladder.
 *
 * `width`/`quality` are still accepted (next/image always passes them) but are intentionally ignored.
 */
export default function ossImageLoader({
	src,
}: {
	src: string;
	width?: number;
	quality?: number;
}): string {
	return buildImageUrl(src, "original") || src;
}
