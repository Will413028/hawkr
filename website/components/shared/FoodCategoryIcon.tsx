import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import type { HawkrAgentCategoryId } from "@/lib/animation/hawkr-agent-season";
import { cn } from "@/lib/utils/cn";

const FOOD_CATEGORY_GLYPHS = {
	vegetables: "leaf",
	fruits: "produce",
	seafood: "seafood",
	meat: "meat",
	"egg-dairy": "egg",
	processed: "processed",
	spices: "herb",
	grains: "grain",
	flowers: "flower",
} as const satisfies Readonly<Record<HawkrAgentCategoryId, AppIconName>>;

interface FoodCategoryIconProps {
	categoryId: HawkrAgentCategoryId;
	/** Glyph size in px; the tinted box always fills its parent. */
	size?: number;
	className?: string;
}

/**
 * Decorative stand-in for a food-category illustration: a token-tinted box that
 * fills its parent, holding one registry glyph. The adjacent text label (or the
 * parent's own accessible name) carries the meaning.
 */
export default function FoodCategoryIcon({
	categoryId,
	size = 20,
	className,
}: FoodCategoryIconProps) {
	return (
		<span
			className={cn(
				"grid size-full place-items-center rounded-hk-lg bg-hk-primary-subtle text-hk-primary",
				className,
			)}
			data-food-category={categoryId}
			aria-hidden="true"
		>
			<AppIcon name={FOOD_CATEGORY_GLYPHS[categoryId]} size={size} />
		</span>
	);
}
