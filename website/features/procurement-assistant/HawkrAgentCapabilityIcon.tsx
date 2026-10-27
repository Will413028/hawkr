import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";

export type HawkrAgentCapabilityIconName =
	| "find-suppliers"
	| "organise-requirements"
	| "compare-candidates";

const CAPABILITY_ICONS: Readonly<
	Record<HawkrAgentCapabilityIconName, AppIconName>
> = {
	"find-suppliers": "search",
	"organise-requirements": "clipboard",
	"compare-candidates": "chart",
};

interface HawkrAgentCapabilityIconProps {
	icon: HawkrAgentCapabilityIconName;
}

/**
 * Entry capability glyph. Decorative: the adjacent HTML label and description
 * carry the accessible name.
 */
export default function HawkrAgentCapabilityIcon({
	icon,
}: HawkrAgentCapabilityIconProps) {
	return (
		<span
			className="hawkr-agent-entry__capability-icon"
			data-capability-icon={icon}
			aria-hidden="true"
		>
			<AppIcon name={CAPABILITY_ICONS[icon]} size={20} />
		</span>
	);
}
