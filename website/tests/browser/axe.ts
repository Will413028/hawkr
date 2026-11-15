import axe from "axe-core";
import { expect } from "vitest";
import { formatAxeViolations } from "@/tests/shared/axe-format";

/**
 * Runs axe's default rule set (the e2e helper in `tests/e2e/helpers/axe.ts`
 * runs the same set, minus the three page-level rules disabled below) and fails on any violation, listing rule id
 * and target for each. Component tests render one component without a page
 * around it, so the page-level rules `region`, `landmark-one-main` and
 * `page-has-heading-one` (jest-axe also disables `region`) are off; the e2e
 * helper still enforces them on whole pages. Finite CSS animations and
 * transitions (dialog and popover fade-ins) finish first: axe samples the
 * blended colour of an element that is still fading in, which reports a
 * contrast failure the settled state does not have. Two animation frames
 * pass before collecting them, so a transition started by the interaction
 * just before the call is already registered.
 */
export async function expectNoAxeViolations(
	label: string,
	container: Element | Document = document,
) {
	for (let frame = 0; frame < 2; frame++) {
		await new Promise((resolve) => requestAnimationFrame(resolve));
	}
	const settled = Promise.all(
		document
			.getAnimations()
			.filter(
				(animation) =>
					animation.effect?.getComputedTiming().iterations !== Infinity &&
					(container instanceof Element
						? container.contains(
								(animation.effect as KeyframeEffect | null)?.target ?? null,
							)
						: true),
			)
			.map((animation) => animation.finished.catch(() => undefined)),
	);
	await Promise.race([
		settled,
		new Promise((resolve) => setTimeout(resolve, 2000)),
	]);
	const { violations } = await axe.run(container, {
		rules: {
			region: { enabled: false },
			"landmark-one-main": { enabled: false },
			"page-has-heading-one": { enabled: false },
		},
	});
	const summary = formatAxeViolations(violations);
	expect(summary, `${label} has axe violations`).toEqual([]);
}
