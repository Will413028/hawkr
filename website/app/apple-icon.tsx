import { ImageResponse } from "next/og";
import { HAWKR_APP_ICON_SVG, svgDataUri } from "@/lib/brand/hawkr-svg";

export const size = { width: 180, height: 180 };
export const contentType = "image/png";

// iOS 自己加圓角遮罩，圖要是直角的方圖。
const SQUARE_APP_ICON_SVG = HAWKR_APP_ICON_SVG.replace('rx="224"', 'rx="0"');

export default function AppleIcon() {
	return new ImageResponse(
		// biome-ignore lint/performance/noImgElement: ImageResponse (Satori) only renders <img>, not next/image
		<img
			src={svgDataUri(SQUARE_APP_ICON_SVG)}
			width={size.width}
			height={size.height}
			alt=""
		/>,
		{ ...size },
	);
}
