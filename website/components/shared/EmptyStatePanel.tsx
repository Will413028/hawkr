import type { CSSProperties } from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import { cn } from "@/lib/utils/cn";

interface EmptyStatePanelProps {
	icon: AppIconName;
	title: string;
	description: string;
	className?: string;
	iconSize?: number;
	/** Heading level for the title. Default `h2`; pass `h3` when the empty state
	 *  sits inside a section that already owns an `h2`. */
	titleAs?: "h2" | "h3" | "h4";
}

export default function EmptyStatePanel({
	icon,
	title,
	description,
	className,
	iconSize = 104,
	titleAs: Title = "h2",
}: EmptyStatePanelProps) {
	return (
		<div
			className={cn(
				"flex flex-col items-center justify-center gap-[var(--hkui-space-2)] px-[var(--hkui-space-6)] py-[var(--hkui-space-12)] bg-[var(--hkui-content1)] border border-dashed border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)] text-center",
				className,
			)}
		>
			<div
				className="grid place-items-center mb-[var(--hkui-space-1)] rounded-full w-[var(--halo-size)] h-[var(--halo-size)] text-[var(--hkui-primary)] bg-[color:rgb(var(--color-brand-ink-rgb)/0.04)] bg-[image:radial-gradient(circle,rgb(var(--color-brand-secondary-rgb)/0.14),transparent_68%)]"
				style={
					{
						"--halo-size": `${Math.max(132, iconSize + 32)}px`,
					} as CSSProperties
				}
				aria-hidden="true"
			>
				<AppIcon name={icon} size={iconSize} />
			</div>
			<Title className="m-0 text-[var(--hkui-foreground)] text-[length:var(--hkui-text-medium)] font-bold">
				{title}
			</Title>
			<p className="max-w-[360px] m-0 text-[var(--hkui-foreground-soft)] text-[length:var(--hkui-text-small)] leading-[1.7]">
				{description}
			</p>
		</div>
	);
}
