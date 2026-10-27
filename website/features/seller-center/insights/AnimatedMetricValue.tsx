interface AnimatedMetricValueProps {
	value?: number;
	format: (value?: number) => string;
}

/**
 * Reveals only the confirmed formatted value. The keyed span gives CSS a short
 * exact-value transition without interpolating through prices that never
 * existed; reduced-motion users receive the value immediately.
 */
export function AnimatedMetricValue({
	value,
	format,
}: AnimatedMetricValueProps) {
	const finalText = format(value);
	return (
		<span
			key={finalText}
			className="seller-price-lookup__animated-value"
			aria-atomic="true"
			aria-live="polite"
		>
			{finalText}
		</span>
	);
}
