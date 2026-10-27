export interface DescriptionHsvColor {
	h: number;
	s: number;
	v: number;
}

export type DescriptionRgbColor = [number, number, number];

/** The product-description wire format stores opaque hex colors only. */
export function normalizeDescriptionHex(input: string): string | null {
	const match = input.trim().match(/^#?([\da-f]{3}|[\da-f]{6})$/i);
	if (!match) return null;
	const digits = match[1].toLowerCase();
	return `#${digits.length === 3 ? [...digits].map((digit) => digit + digit).join("") : digits}`;
}

export function descriptionHexToRgb(hex: string): DescriptionRgbColor {
	const normalized = normalizeDescriptionHex(hex);
	if (!normalized) throw new Error("Invalid opaque hex color");
	return [1, 3, 5].map((start) =>
		Number.parseInt(normalized.slice(start, start + 2), 16),
	) as DescriptionRgbColor;
}

export function descriptionRgbToHex(rgb: DescriptionRgbColor): string {
	return `#${rgb
		.map((channel) =>
			Math.round(Math.min(255, Math.max(0, channel)))
				.toString(16)
				.padStart(2, "0"),
		)
		.join("")}`;
}

export function parseDescriptionRgb(
	channels: readonly string[],
): DescriptionRgbColor | null {
	if (
		channels.length !== 3 ||
		channels.some((value) => !/^\d{1,3}$/.test(value.trim()))
	)
		return null;
	const numbers = channels.map((value) => Number(value));
	return numbers.every((value) => value >= 0 && value <= 255)
		? (numbers as DescriptionRgbColor)
		: null;
}

export function descriptionHexToHsv(
	hex: string,
	previousHue = 0,
): DescriptionHsvColor {
	const [red, green, blue] = descriptionHexToRgb(hex).map(
		(channel) => channel / 255,
	);
	const maximum = Math.max(red, green, blue);
	const minimum = Math.min(red, green, blue);
	const delta = maximum - minimum;
	let hue = previousHue;
	if (delta > 0) {
		if (maximum === red) hue = 60 * (((green - blue) / delta) % 6);
		else if (maximum === green) hue = 60 * ((blue - red) / delta + 2);
		else hue = 60 * ((red - green) / delta + 4);
	}
	return {
		h: (hue + 360) % 360,
		s: maximum === 0 ? 0 : (delta / maximum) * 100,
		v: maximum * 100,
	};
}

export function descriptionHsvToHex({ h, s, v }: DescriptionHsvColor): string {
	const hue = (((h % 360) + 360) % 360) / 60;
	const saturation = Math.min(100, Math.max(0, s)) / 100;
	const value = Math.min(100, Math.max(0, v)) / 100;
	const chroma = value * saturation;
	const second = chroma * (1 - Math.abs((hue % 2) - 1));
	const channels =
		hue < 1
			? [chroma, second, 0]
			: hue < 2
				? [second, chroma, 0]
				: hue < 3
					? [0, chroma, second]
					: hue < 4
						? [0, second, chroma]
						: hue < 5
							? [second, 0, chroma]
							: [chroma, 0, second];
	return descriptionRgbToHex(
		channels.map(
			(channel) => (channel + value - chroma) * 255,
		) as DescriptionRgbColor,
	);
}

/** Pointer geometry is clamped so a captured drag may safely leave the area. */
export function descriptionColorPoint(
	x: number,
	y: number,
	box: { left: number; top: number; width: number; height: number },
): Pick<DescriptionHsvColor, "s" | "v"> {
	return {
		s: Math.min(
			100,
			Math.max(0, ((x - box.left) / Math.max(1, box.width)) * 100),
		),
		v: Math.min(
			100,
			Math.max(0, 100 - ((y - box.top) / Math.max(1, box.height)) * 100),
		),
	};
}
