interface AxeViolationLike {
	id: string;
	impact?: string | null;
	nodes: ReadonlyArray<{
		target: ReadonlyArray<unknown>;
		any: ReadonlyArray<{ data?: unknown }>;
	}>;
}

/**
 * Reduces axe violations to rule id, impact and node targets; a colour
 * contrast failure also carries foreground, background and ratio.
 */
export function formatAxeViolations(
	violations: ReadonlyArray<AxeViolationLike>,
) {
	return violations.map((violation) => ({
		rule: violation.id,
		impact: violation.impact,
		targets: violation.nodes.map((node) => {
			const data = node.any[0]?.data as
				| { fgColor?: string; bgColor?: string; contrastRatio?: number }
				| undefined;
			const target = node.target.join(" ");
			return data?.contrastRatio === undefined
				? target
				: `${target} (${data.fgColor} on ${data.bgColor}, ${data.contrastRatio}:1)`;
		}),
	}));
}
