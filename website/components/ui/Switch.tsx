"use client";

import { Switch as HeroSwitch } from "@heroui/react";
import { cn } from "@/lib/utils/cn";

export interface SwitchProps {
	checked: boolean;
	onCheckedChange: (checked: boolean) => void;
	ariaLabel: string;
	disabled?: boolean;
	busy?: boolean;
	className?: string;
}

/**
 * Hawkr boolean switch. HeroUI supplies React Aria keyboard, focus and
 * switch semantics; the brand scope maps its checked state to our theme
 * tokens. Keep labels outside this control so settings rows can own their
 * responsive layout without duplicating switch mechanics.
 * @import import { Switch } from '@/components/ui/Switch';
 */
export function Switch({
	checked,
	onCheckedChange,
	ariaLabel,
	disabled = false,
	busy = false,
	className,
}: SwitchProps) {
	return (
		<HeroSwitch
			size="lg"
			isSelected={checked}
			onChange={onCheckedChange}
			isDisabled={disabled || busy}
			aria-label={ariaLabel}
			aria-busy={busy || undefined}
			className={cn(
				// The root is HeroUI's focusable node, so removing its outline
				// outright left keyboard users with no visible focus at all.
				// Paint an explicit ring here instead of erasing one.
				"hk-heroui-brand-scope min-h-11 shrink-0 justify-end rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--hkui-focus)]",
				className,
			)}
		>
			<HeroSwitch.Control>
				<HeroSwitch.Thumb />
			</HeroSwitch.Control>
		</HeroSwitch>
	);
}

export default Switch;
