"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";

interface Props {
	/** Backend's `shipBackDeadlineAt` (unix ms). Null/undefined = render
	 *  nothing — the caller is expected to gate on status === "20"
	 *  (RETURN_PENDING) too, but we also short-circuit defensively. */
	deadline?: number | null;
	className?: string;
	/** Wrap output in a different tag — defaults to span. */
	as?: "span" | "div" | "strong";
}

interface Remaining {
	expired: boolean;
	label: string;
}

function formatRemaining(deadline: number, now: number): Remaining {
	const ms = deadline - now;
	if (ms <= 0) return { expired: true, label: "已逾時，等待系統關案" };
	const totalSec = Math.floor(ms / 1000);
	const hours = Math.floor(totalSec / 3600);
	const minutes = Math.floor((totalSec % 3600) / 60);
	if (hours >= 1) {
		return {
			expired: false,
			label: `剩 ${hours} 小時 ${minutes.toString().padStart(2, "0")} 分將解除退款資格`,
		};
	}
	const seconds = totalSec % 60;
	if (minutes >= 1) {
		return {
			expired: false,
			label: `剩 ${minutes} 分 ${seconds.toString().padStart(2, "0")} 秒將解除退款資格`,
		};
	}
	return { expired: false, label: `剩 ${seconds} 秒將解除退款資格` };
}

/**
 * Live countdown for a status=20 (RETURN_PENDING) case's
 * `shipBackDeadlineAt`. Re-renders every second while < 1 hour remains
 * (so HH:mm:ss reads correctly at the end), every 30s otherwise (saves
 * a few ticks per minute on long waits). After expiry it stops the
 * timer and just shows "已逾時" — the caller is expected to re-fetch
 * the case at that point because the server-side scheduler will flip
 * status to 33 AUTO_COMPLETED_RETURN_PENDING_TIMEOUT on its next sweep.
 *
 * Forked from `OrderPaymentCountdown` so the buyer-facing visual
 * vocabulary (clock icon, warning palette, cadence) stays identical.
 */
export default function ShipBackDeadlineCountdown({
	deadline,
	className,
	as = "span",
}: Props) {
	const [now, setNow] = React.useState(() => Date.now());

	React.useEffect(() => {
		if (!deadline) return;
		setNow(Date.now());
		const tick = () => setNow(Date.now());
		const remainingMs = deadline - Date.now();
		// One second cadence inside the final hour; thirty seconds otherwise.
		// Re-evaluates inside the interval so the cadence ratchets down on its
		// own as the deadline approaches.
		const id = window.setInterval(
			tick,
			remainingMs > 3_600_000 ? 30_000 : 1_000,
		);
		return () => window.clearInterval(id);
	}, [deadline]);

	if (!deadline) return null;

	const { label, expired } = formatRemaining(deadline, now);
	const Tag = as;
	return (
		<Tag
			className={className}
			data-expired={expired ? "true" : undefined}
			aria-label={`寄回期限：${label}`}
		>
			<AppIcon
				name="clock"
				size={15}
				style={{ color: "var(--hkui-warning)" }}
				className={
					expired
						? "-ml-0.5"
						: "-ml-0.5 animate-pulse motion-reduce:animate-none"
				}
			/>
			{label}
		</Tag>
	);
}
