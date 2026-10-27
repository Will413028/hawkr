import { ImageResponse } from "next/og";
import { HAWKR_LOCKUP_DARK_SVG, svgDataUri } from "@/lib/brand/hawkr-svg";

export const alt = "Hawkr — 在地職人的市集";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpengraphImage() {
	return new ImageResponse(
		<div
			style={{
				width: "100%",
				height: "100%",
				display: "flex",
				flexDirection: "column",
				alignItems: "flex-start",
				justifyContent: "center",
				padding: "96px",
				background: "#1f2933",
				color: "#f5f7fa",
				fontFamily: "sans-serif",
			}}
		>
			{/* biome-ignore lint/performance/noImgElement: ImageResponse (Satori) only renders <img>, not next/image */}
			<img
				src={svgDataUri(HAWKR_LOCKUP_DARK_SVG)}
				width={628}
				height={150}
				alt=""
			/>
			<div style={{ fontSize: "2.5rem", marginTop: 24, opacity: 0.9 }}>
				Fresh from local farms, every day.
			</div>
		</div>,
		{ ...size },
	);
}
