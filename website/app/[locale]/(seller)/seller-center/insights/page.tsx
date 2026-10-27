import "maplibre-gl/dist/maplibre-gl.css";
import InsightsClient from "@/features/seller-center/insights/InsightsClient";

export const metadata = { title: "市場行情查詢 — 賣家中心" };

export default function Page() {
	return <InsightsClient />;
}
