export const HAWKR_AGENT_CATEGORY_IDS = [
	"vegetables",
	"fruits",
	"seafood",
	"meat",
	"egg-dairy",
	"processed",
	"spices",
	"grains",
	"flowers",
] as const;

export type HawkrAgentCategoryId = (typeof HAWKR_AGENT_CATEGORY_IDS)[number];

export const HAWKR_AGENT_CATEGORY_METADATA = {
	vegetables: { order: 1, label: "蔬菜" },
	fruits: { order: 2, label: "水果" },
	seafood: { order: 3, label: "海鮮" },
	meat: { order: 4, label: "肉類" },
	"egg-dairy": { order: 5, label: "蛋奶" },
	processed: { order: 6, label: "加工食品" },
	spices: { order: 7, label: "香料" },
	grains: { order: 8, label: "穀物" },
	flowers: { order: 9, label: "花卉" },
} as const satisfies Readonly<
	Record<
		HawkrAgentCategoryId,
		{ readonly order: number; readonly label: string }
	>
>;
