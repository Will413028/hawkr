import type * as React from "react";

export interface AiSparkleIconProps
	extends Omit<React.SVGProps<SVGSVGElement>, "width" | "height"> {
	size?: number;
}

/**
 * Two-tone "AI 美化" magic-wand mark. The wand strokes paint with
 * `--icon-accent` (primary) and the sparkle accents with `--icon-accent-secondary`
 * (gold). Decorative by default; pass `aria-label` to expose it.
 */
export default function AiSparkleIcon({
	size = 20,
	"aria-label": ariaLabel,
	...rest
}: AiSparkleIconProps) {
	return (
		<svg
			width={size}
			height={size}
			viewBox="0 0 24 24"
			fill="none"
			role={ariaLabel ? "img" : undefined}
			aria-label={ariaLabel}
			aria-hidden={ariaLabel ? undefined : true}
			{...rest}
		>
			{/* Wand body + tip line — primary */}
			<path
				fillRule="evenodd"
				clipRule="evenodd"
				d="M17.6319 3.74169C16.7546 3.04039 15.4851 3.1402 14.7268 3.96758L8.78298 10.3039C7.69032 11.499 7.83565 13.3691 9.10079 14.3803C10.3659 15.3916 12.2221 15.121 13.1466 13.792L18.0512 6.62481C18.6921 5.70287 18.5091 4.44211 17.6319 3.74169Z"
				stroke="var(--icon-accent)"
				strokeWidth={1.5}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
			<path
				d="M11.58 10.9144L13.8458 12.7259"
				stroke="var(--icon-accent)"
				strokeWidth={1.5}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
			{/* Sparkle accents — secondary */}
			<path
				fillRule="evenodd"
				clipRule="evenodd"
				d="M3.49085 20.7224C3.49085 20.7224 4.05119 19.9843 7.52616 19.8889C11.0011 19.7943 11.3338 16.1644 8.96903 14.2741C7.23023 12.8838 4.63692 13.7558 4.09146 15.914L3.01369 20.1752C2.93839 20.4746 3.18441 20.7565 3.49085 20.7224Z"
				stroke="var(--icon-accent-secondary)"
				strokeWidth={1.5}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
			<path
				d="M18.2618 20.5668L18.1552 20.2784C17.7535 19.191 16.8974 18.3337 15.8116 17.9313L15.5237 17.8246L15.8116 17.7179C16.8974 17.3155 17.7535 16.4582 18.1552 15.3708L18.2618 15.0824L18.3684 15.3708C18.7701 16.4582 19.6262 17.3155 20.712 17.7179L21 17.8246L20.712 17.9313C19.6262 18.3337 18.7701 19.191 18.3684 20.2784L18.2618 20.5668Z"
				stroke="var(--icon-accent-secondary)"
				strokeWidth={1.5}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
			<path
				d="M12.4517 20.2111H12.4617"
				stroke="var(--icon-accent-secondary)"
				strokeWidth={1.5}
				strokeLinecap="round"
				strokeLinejoin="round"
			/>
		</svg>
	);
}
