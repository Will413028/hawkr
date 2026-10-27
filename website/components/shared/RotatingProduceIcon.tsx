"use client";

import * as React from "react";
import ProduceIcon, {
	PRODUCE_ICON_NAMES,
	type ProduceIconProps,
} from "@/components/shared/ProduceIcon";

const MIN_ROTATION_DELAY_MS = 4_800;
const ROTATION_DELAY_VARIANCE_MS = 2_400;

export type RotatingProduceIconProps = Omit<ProduceIconProps, "name">;

/**
 * Keeps the server/client first render deterministic, then rotates through a
 * different random produce glyph after mount. The SVG footprint never changes,
 * and reduced-motion users keep a stable icon.
 */
export default function RotatingProduceIcon(props: RotatingProduceIconProps) {
	const [index, setIndex] = React.useState(0);
	const indexRef = React.useRef(0);

	React.useEffect(() => {
		const media = window.matchMedia("(prefers-reduced-motion: reduce)");
		if (media.matches) return;

		let timeoutId: number | undefined;
		const scheduleNext = () => {
			const delay =
				MIN_ROTATION_DELAY_MS + Math.random() * ROTATION_DELAY_VARIANCE_MS;
			timeoutId = window.setTimeout(() => {
				const current = indexRef.current;
				let next = Math.floor(Math.random() * (PRODUCE_ICON_NAMES.length - 1));
				if (next >= current) next += 1;
				indexRef.current = next;
				setIndex(next);
				scheduleNext();
			}, delay);
		};

		scheduleNext();
		return () => {
			if (timeoutId !== undefined) window.clearTimeout(timeoutId);
		};
	}, []);

	return <ProduceIcon {...props} name={PRODUCE_ICON_NAMES[index]} />;
}
