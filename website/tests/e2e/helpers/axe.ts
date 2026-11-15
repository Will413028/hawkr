import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";

/**
 * Runs axe's default rule set on the current page and fails on any violation,
 * matching the component-level `a11y.test: "error"` gate this replaces.
 */
export async function expectNoAxeViolations(page: Page, label: string) {
	const { violations } = await new AxeBuilder({ page }).analyze();
	const summary = violations.map((violation) => ({
		rule: violation.id,
		impact: violation.impact,
		targets: violation.nodes.map(
			(node) =>
				`${node.target.join(" ")} :: ${node.html} :: ${node.failureSummary} :: ${JSON.stringify(node.any.concat(node.all, node.none).map((c) => c.data))}`,
		),
	}));
	expect(summary, `${label} has axe violations`).toEqual([]);
}
