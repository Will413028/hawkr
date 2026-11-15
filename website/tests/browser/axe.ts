import axe from "axe-core";
import { expect } from "vitest";
import { formatAxeViolations } from "@/tests/shared/axe-format";

/**
 * Runs axe's default rule set (the e2e helper in `tests/e2e/helpers/axe.ts`
 * runs the same set, minus the three page-level rules disabled below) and fails on any violation, listing rule id
 * and target for each. Component tests render one component without a page
 * around it, so the page-level rules `region`, `landmark-one-main` and
 * `page-has-heading-one` (jest-axe also disables `region`) are off; the e2e
 * helper still enforces them on whole pages. Two animation frames pass
 * first, so a transition started by the interaction just before the call is
 * already registered. Then every running finite document-wide animation
 * (portals render into body, so not only the container's) must finish:
 * axe samples the blended colour of an element that is still fading in,
 * which reports a contrast failure the settled state does not have. If any
 * is still running after 2s the call throws, naming their targets.
 */
export async function expectNoAxeViolations(
	label: string,
	container: Element | Document = document,
) {
	for (let frame = 0; frame < 2; frame++) {
		await new Promise((resolve) => requestAnimationFrame(resolve));
	}
	const running = document
		.getAnimations()
		.filter(
			(animation) =>
				animation.playState === "running" &&
				animation.effect?.getComputedTiming().iterations !== Infinity,
		);
	let timer: ReturnType<typeof setTimeout> | undefined;
	try {
		const outcome = await Promise.race([
			Promise.all(
				running.map((animation) => animation.finished.catch(() => undefined)),
			).then(() => "settled" as const),
			new Promise<"timeout">((resolve) => {
				timer = setTimeout(() => resolve("timeout"), 2000);
			}),
		]);
		if (outcome === "timeout") {
			const targets = running
				.filter((animation) => animation.playState === "running")
				.map((animation) => {
					const target = (animation.effect as KeyframeEffect | null)?.target;
					return target
						? `${target.tagName.toLowerCase()}${target.id ? `#${target.id}` : ""}`
						: "(no target)";
				});
			throw new Error(
				`${label}: animations still running after 2000ms: ${targets.join(", ")}`,
			);
		}
	} finally {
		clearTimeout(timer);
	}
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
