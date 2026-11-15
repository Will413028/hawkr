// vitest/browser 的型別提供 jest-dom 風格的 matcher 宣告（它們只在 expect.element 上有實作）。
import "vitest/browser";
import { expect } from "vitest";

// 同步的 DOM matcher，語意對齊 jest-dom：原 Storybook play 的 waitFor 重試迴圈裡直接呼叫它們。
// `expect.element` 的版本是自己重試、且字串版 toHaveTextContent 是整段比對，不能在 waitFor 內同步使用。
// 只有 import 本檔的測試檔才註冊。

type Received = Element | null;

function pass(ok: boolean, expectation: string, received: unknown) {
	return {
		pass: ok,
		message: () =>
			`expected ${describe(received)} ${ok ? "not " : ""}${expectation}`,
	};
}

function describe(received: unknown): string {
	if (received instanceof Element) {
		return `<${received.localName}${received.id ? `#${received.id}` : ""}> "${(received.textContent ?? "").slice(0, 60)}"`;
	}
	return String(received);
}

function kebab(name: string) {
	return name.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`);
}

function isStyleVisible(element: Element) {
	const style = element.ownerDocument.defaultView?.getComputedStyle(element);
	if (!style) return false;
	return (
		style.display !== "none" &&
		style.visibility !== "hidden" &&
		style.visibility !== "collapse" &&
		style.opacity !== "0"
	);
}

function isVisible(element: Element): boolean {
	if (!element.ownerDocument.contains(element)) return false;
	for (let node: Element | null = element; node; node = node.parentElement) {
		if (!isStyleVisible(node)) return false;
		if (node.hasAttribute("hidden")) return false;
	}
	return true;
}

function normalizeStyleValue(
	doc: Document,
	property: string,
	value: string,
): string {
	const probe = doc.createElement("span");
	probe.style.setProperty(kebab(property), value);
	return probe.style.getPropertyValue(kebab(property)) || value;
}

expect.extend({
	toBeVisible(received: Received) {
		return pass(
			received !== null && isVisible(received),
			"to be visible",
			received,
		);
	},
	toBeInTheDocument(received: Received) {
		return pass(
			received?.ownerDocument.contains(received) === true,
			"to be in the document",
			received,
		);
	},
	toBeEnabled(received: Element) {
		return pass(!received.matches(":disabled"), "to be enabled", received);
	},
	toBeDisabled(received: Element) {
		return pass(received.matches(":disabled"), "to be disabled", received);
	},
	toHaveFocus(received: Element) {
		return pass(
			received.ownerDocument.activeElement === received,
			"to have focus",
			received,
		);
	},
	toHaveAttribute(received: Element, name: string, value?: string) {
		const actual = received.getAttribute(name);
		const ok = value === undefined ? actual !== null : actual === value;
		return pass(
			ok,
			`to have attribute ${name}${value === undefined ? "" : `="${value}"`} (actual: ${actual})`,
			received,
		);
	},
	toBeChecked(received: Element) {
		const ok =
			received instanceof HTMLInputElement
				? received.checked
				: received.getAttribute("aria-checked") === "true";
		return pass(ok, "to be checked", received);
	},
	toBePartiallyChecked(received: Element) {
		const ok =
			received instanceof HTMLInputElement
				? received.indeterminate
				: received.getAttribute("aria-checked") === "mixed";
		return pass(ok, "to be partially checked", received);
	},
	toHaveValue(received: Element, expected: string | number) {
		let actual: string | number | null;
		if (received instanceof HTMLInputElement && received.type === "number") {
			actual = received.value === "" ? null : Number(received.value);
		} else {
			actual = (received as HTMLInputElement).value;
		}
		return pass(
			actual === expected,
			`to have value ${JSON.stringify(expected)} (actual: ${JSON.stringify(actual)})`,
			received,
		);
	},
	toHaveStyle(received: Element, expected: Record<string, string>) {
		const computed =
			received.ownerDocument.defaultView?.getComputedStyle(received);
		const mismatches = Object.entries(expected).filter(([property, value]) => {
			const want = normalizeStyleValue(received.ownerDocument, property, value);
			return computed?.getPropertyValue(kebab(property)) !== want;
		});
		return pass(
			mismatches.length === 0,
			`to have style ${JSON.stringify(expected)}`,
			received,
		);
	},
	toHaveTextContent(received: Element, expected: string | RegExp) {
		const text = (received.textContent ?? "").replace(/\s+/g, " ").trim();
		const ok =
			typeof expected === "string"
				? text.includes(expected)
				: expected.test(text);
		return pass(
			ok,
			`to have text content ${String(expected)} (actual: "${text}")`,
			received,
		);
	},
});
