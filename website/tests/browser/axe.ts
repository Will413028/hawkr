import axe from "axe-core";
import { expect } from "vitest";

/**
 * Runs axe's default rule set (same as the e2e helper in
 * `tests/e2e/helpers/axe.ts`) and fails on any violation, listing rule id
 * and target for each. Component tests render one component without a page
 * around it, so the page-level rules `region`, `landmark-one-main` and
 * `page-has-heading-one` (jest-axe also disables `region`) are off; the e2e
 * helper still enforces them on whole pages. Finite CSS animations and
 * transitions (dialog and popover fade-ins) finish first: axe samples the
 * blended colour of an element that is still fading in, which reports a
 * contrast failure the settled state does not have.
 */
export async function expectNoAxeViolations(
	label: string,
	container: Element | Document = document,
) {
	await Promise.all(
		document
			.getAnimations()
			.filter(
				(animation) =>
					animation.effect?.getComputedTiming().iterations !== Infinity,
			)
			.map((animation) => animation.finished.catch(() => undefined)),
	);
	const { violations } = await axe.run(container, {
		rules: {
			region: { enabled: false },
			"landmark-one-main": { enabled: false },
			"page-has-heading-one": { enabled: false },
		},
	});
	const summary = violations.map((violation) => ({
		rule: violation.id,
		impact: violation.impact,
		targets: violation.nodes.map((node) => node.target.join(" ")),
	}));
	expect(summary, `${label} has axe violations`).toEqual([]);
}
