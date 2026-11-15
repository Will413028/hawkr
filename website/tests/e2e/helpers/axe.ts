import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";
import { formatAxeViolations } from "@/tests/shared/axe-format";

/**
 * Runs axe's default rule set on the current page and fails on any violation,
 * matching the component-level `a11y.test: "error"` gate this replaces.
 */
export async function expectNoAxeViolations(page: Page, label: string) {
	const { violations } = await new AxeBuilder({ page }).analyze();
	const summary = formatAxeViolations(violations);
	expect(summary, `${label} has axe violations`).toEqual([]);
}
