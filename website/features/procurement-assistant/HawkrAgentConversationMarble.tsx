import FoodCategoryIcon from "@/components/shared/FoodCategoryIcon";
import {
	HAWKR_AGENT_CATEGORY_METADATA,
	type HawkrAgentCategoryId,
} from "@/lib/animation/hawkr-agent-season";

interface HawkrAgentConversationMarbleProps {
	primaryCategoryId?: HawkrAgentCategoryId | null;
}

/**
 * Category identity for one Agent conversation. Known categories expose a
 * concise accessible label; legacy or unclassified rows stay decorative with a
 * plain tinted circle.
 */
export default function HawkrAgentConversationMarble({
	primaryCategoryId,
}: HawkrAgentConversationMarbleProps) {
	return (
		<span
			className="chat-sidebar__avatar chat-sidebar__category-marble"
			data-category={primaryCategoryId ?? "unclassified"}
			{...(primaryCategoryId
				? {
						role: "img" as const,
						"aria-label": `大分類：${HAWKR_AGENT_CATEGORY_METADATA[primaryCategoryId].label}`,
					}
				: { "aria-hidden": true })}
		>
			{primaryCategoryId ? (
				<FoodCategoryIcon
					categoryId={primaryCategoryId}
					className="rounded-full"
				/>
			) : (
				<span className="size-full rounded-full bg-hk-primary-subtle" />
			)}
		</span>
	);
}
