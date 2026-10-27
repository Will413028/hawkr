import BrandLogo from "@/components/shared/BrandLogo";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import { cn } from "@/lib/utils/cn";
import { type AgentMotionState, spriteMotion } from "./agent-motion";

interface HawkrAgentSpriteProps {
	state: AgentMotionState;
	/**
	 * 固定像素尺寸。省略時**改由 `className` 的 CSS 決定**——responsive 版位
	 * 需要這個：inline style 會壓過 media query，而在 JS 裡量斷點會在手機首次
	 * 繪製時先畫大再跳小，正是 §V.10.10-1 禁止的 layout shift。
	 */
	size?: number;
	/** 為真時不渲染；旁邊文字承載全部語意（§V.10.10-5）。 */
	disabled?: boolean;
	/**
	 * 覆寫 `role="img"` 的標籤。省略時用 state 自己的固定標籤；傳空字串則整顆
	 * 視為裝飾（`aria-hidden`），用在旁邊已經有等值文字的版位。
	 */
	label?: string;
	className?: string;
}

/**
 * Hawkr Agent Sprite——active rail 與 terminal result 的狀態記號
 *（設計書 §V.10.10-12 的 `HawkrAgentSprite`）。
 *
 * 與 `HawkrAgentMark` 的分工是設計書明訂的，不要合併：`HawkrAgentMark` 永遠是
 * 中性靜態記號，給 sidebar／歷史對話／小型 identity 用；這一顆吃 motion
 * state，是**當前這一輪**的狀態。
 *
 * - `SPRITE_MOTION[state].loop` 為真的處理中狀態，渲染標誌的載入動態
 *   （`BrandLogoLoader`）；其餘狀態渲染靜態標誌。狀態語意由 `data-motion`、
 *   `role="img"` 與標籤承載，不靠動畫。
 * - `prefers-reduced-motion` 由載入動態的 CSS 處理。
 */
export default function HawkrAgentSprite({
	state,
	size,
	disabled = false,
	label,
	className,
}: HawkrAgentSpriteProps) {
	if (disabled) return null;

	const motion = spriteMotion(state);
	const resolvedLabel = label ?? motion.label;
	const decorative = resolvedLabel === "";

	return (
		<span
			className={cn("hawkr-agent-sprite", className)}
			data-motion={state}
			// 外框在內容畫出前就佔好位置——§V.10.10-1「無首屏 layout shift」。
			// size 省略時寬高留給 className 的 CSS，不寫 inline 值去壓 media query。
			style={{
				display: "inline-block",
				...(size === undefined ? null : { width: size, height: size }),
			}}
			{...(decorative
				? { "aria-hidden": true }
				: { role: "img" as const, "aria-label": resolvedLabel })}
		>
			{motion.loop ? (
				<BrandLogoLoader size="fit" />
			) : (
				<BrandLogo variant="mark" size="fill" />
			)}
		</span>
	);
}
