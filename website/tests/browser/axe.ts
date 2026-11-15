import axe from "axe-core";
import { expect } from "vitest";

/**
 * Runs axe's default rule set (same as the e2e helper in
 * `tests/e2e/helpers/axe.ts`) and fails on any violation, listing rule id
 * and target for each.
 */
export async function expectNoAxeViolations(
	label: string,
	container: Element | Document = document,
) {
	const { violations } = await axe.run(container);
	const summary = violations.map((violation) => ({
		rule: violation.id,
		impact: violation.impact,
		targets: violation.nodes.map((node) => node.target.join(" ")),
	}));
	expect(summary, `${label} has axe violations`).toEqual([]);
}
