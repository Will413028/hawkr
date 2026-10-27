import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import type { InsightsTabId } from "./InsightsWorkspaceTabs";

interface EmptyWorkspaceCopy {
	title: string;
	description: string;
}

const EMPTY_WORKSPACE_COPY: Record<
	Exclude<InsightsTabId, "spatial">,
	EmptyWorkspaceCopy
> = {
	overview: {
		title: "行情內容會顯示在這裡",
		description: "從上方選好品項後，查看官方成交價、成交量與條件式預測。",
	},
	forecast: {
		title: "七日預測會顯示在這裡",
		description: "選好品項與市場後，查看每日 P10、P50、P90 與可用狀態。",
	},
	markets: {
		title: "市場比較會顯示在這裡",
		description: "選好品項後，比較相同品項在不同批發市場的行情。",
	},
	quality: {
		title: "資料品質會顯示在這裡",
		description: "選好品項後，核對來源、模型版本、資料日期與缺值規則。",
	},
};

export function InsightsEmptyWorkspace({
	activeTab,
	onRequestSearch,
}: {
	activeTab: Exclude<InsightsTabId, "spatial">;
	onRequestSearch: () => void;
}) {
	const copy = EMPTY_WORKSPACE_COPY[activeTab];

	return (
		<section
			className="seller-price-lookup__starter"
			aria-labelledby={`insights-empty-${activeTab}`}
		>
			<h2 id={`insights-empty-${activeTab}`}>{copy.title}</h2>
			<p>{copy.description}</p>
			<Button
				type="button"
				className="seller-price-lookup__starter-action"
				startContent={<AppIcon name="search" size={18} aria-hidden="true" />}
				onClick={onRequestSearch}
			>
				選擇商品分類
			</Button>
		</section>
	);
}
