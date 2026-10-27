"use client";

interface Props {
	percentage: number; // 0-100
	tone?: "primary" | "success" | "warning";
	label?: string;
}

const TONE_BG: Record<NonNullable<Props["tone"]>, string> = {
	primary: "bg-[var(--hkui-primary)]",
	success: "bg-[var(--hkui-success)]",
	warning: "bg-[var(--hkui-warning)]",
};

export function ImportProgressBar({
	percentage,
	tone = "primary",
	label,
}: Props) {
	const pct = Math.max(0, Math.min(100, Math.round(percentage)));
	return (
		<div className="w-full space-y-1">
			{label ? (
				<div className="flex justify-between text-xs text-[var(--hkui-default-600)]">
					<span>{label}</span>
					<span className="font-mono">{pct}%</span>
				</div>
			) : null}
			<div className="h-2 w-full overflow-hidden rounded-full bg-[var(--hkui-default-100)]">
				<div
					className={`${TONE_BG[tone]} h-full transition-[width] duration-500 ease-out`}
					style={{ width: `${pct}%` }}
				/>
			</div>
		</div>
	);
}
