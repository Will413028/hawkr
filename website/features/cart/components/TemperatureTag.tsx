import {
	SHIPPING_TEMPERATURE_LABEL,
	ShippingTemperature,
	type ShippingTemperatureValue,
} from "@/lib/codemeta/enums";

/**
 * Coloured pill showing a delivery temperature class.
 *   3201 常溫 → grey
 *   3202 冷藏 → orange
 *   3203 冷凍 → blue
 *
 * Accepts either the numeric wire code (`"3202"`) or its Chinese label
 * (`"冷藏"`) since the backend has historically returned both in different
 * endpoints.
 */
const TEMP_TONE: Record<string, string> = {
	[ShippingTemperature.AMBIENT]: "text-[#6b6b6b]",
	[ShippingTemperature.CHILLED]: "text-[#ee964b]",
	[ShippingTemperature.FROZEN]: "text-[#51a2ff]",
	常溫: "text-[#6b6b6b]",
	冷藏: "text-[#ee964b]",
	冷凍: "text-[#51a2ff]",
};

function labelFor(value: string): string {
	const known = SHIPPING_TEMPERATURE_LABEL[value as ShippingTemperatureValue];
	return known ?? value;
}

export function TemperatureTag({ value }: { value: string }) {
	const tone = TEMP_TONE[value] ?? "text-[#6b6b6b]";
	const label = labelFor(value);
	return (
		<span
			data-temperature={label}
			className={`inline-flex items-center px-2 h-[18px] rounded-full text-[0.6875rem] font-bold border border-current bg-transparent whitespace-nowrap ${tone}`}
		>
			{label}
		</span>
	);
}

export function TemperatureTagRow({ values }: { values: string[] }) {
	if (!values || values.length === 0) return null;
	return (
		<span className="inline-flex gap-1 flex-wrap">
			{values.map((v) => (
				<TemperatureTag key={v} value={v} />
			))}
		</span>
	);
}
