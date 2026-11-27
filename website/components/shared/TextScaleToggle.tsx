"use client";

import * as React from "react";
import { FOOTER_ICON_LINK } from "@/components/shared/footerChrome";
import LazyAppIcon from "@/components/shared/LazyAppIcon";
import { THEME_TOGGLE_CHROME_CLASS } from "@/components/shared/ThemeToggle";
import { TEXT_SCALE_LABELS, useTextScale } from "@/lib/theme/text-scale";
import { cn } from "@/lib/utils/cn";

interface TextScaleToggleProps {
	className?: string;
	verbose?: boolean;
	variant?: "default" | "footer";
}

/**
 * 字體大小 cycler for the navbar rail and the footer — the sibling of
 * `ThemeToggle`, sharing its chrome in the navbar and using the shared dark
 * action chrome in the footer.
 *
 * Three states cycle 中 → 大 → 小 → 中. The full 小 / 中 / 大 picker lives in
 * 設定 (`TextScaleControl`); this is the one-tap version.
 */
export default function TextScaleToggle({
	className = "",
	verbose = false,
	variant = "default",
}: TextScaleToggleProps) {
	const { scale, cycle } = useTextScale();
	const [announcement, setAnnouncement] = React.useState("");
	const label = TEXT_SCALE_LABELS[scale];

	const previousScale = React.useRef(scale);
	React.useEffect(() => {
		if (previousScale.current === scale) return;
		previousScale.current = scale;
		setAnnouncement(`字級：${TEXT_SCALE_LABELS[scale]}`);
	}, [scale]);

	return (
		<span className="contents">
			<button
				type="button"
				data-verbose={verbose ? "true" : undefined}
				className={cn(
					variant === "footer" ? FOOTER_ICON_LINK : THEME_TOGGLE_CHROME_CLASS,
					className,
				)}
				aria-label={`字級：${label}，點擊切換`}
				title={`字級：${label}，點擊切換`}
				onClick={cycle}
			>
				<LazyAppIcon
					name="document"
					size={18}
					style={{
						color:
							variant === "footer"
								? "var(--color-neutral-cream)"
								: "var(--page-theme-toggle-fg)",
					}}
				/>
				{verbose && (
					<span className="text-[0.8125rem] font-semibold tracking-[0.02em]">
						字級：{label}
					</span>
				)}
			</button>
			<span role="status" aria-live="polite" className="sr-only">
				{announcement}
			</span>
		</span>
	);
}
