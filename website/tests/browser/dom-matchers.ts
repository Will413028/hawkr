import { expect } from "vitest";

// 同步 DOM matcher：`expect(element)` 直接檢查一個已取得的元素，用在 waitFor 的重試迴圈裡。
// 名稱都帶 Sync 後綴，不覆蓋 vitest/browser 內建（只用在 `expect.element`、自己重試）的同名 matcher。
// 只有 import 本檔的測試檔才註冊。

declare module "vitest" {
	interface Assertion {
		toBeVisibleSync(): void;
		toBeInTheDocumentSync(): void;
		toBeEnabledSync(): void;
		toBeDisabledSync(): void;
		toHaveFocusSync(): void;
		toHaveAttributeSync(name: string, value?: string): void;
		toBeCheckedSync(): void;
		toBePartiallyCheckedSync(): void;
		toHaveValueSync(expected: string | number): void;
		toHaveStyleSync(expected: Record<string, string>): void;
	}
}

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

/** 與 jest-dom 相同：收到的不是元素就丟錯，`not` 也不會讓它通過。 */
function element(received: unknown, matcher: string): Element {
	if (!(received instanceof Element)) {
		throw new TypeError(
			`${matcher} 需要 DOM 元素，收到 ${describe(received)}；要檢查「不存在」請用 expectAbsent`,
		);
	}
	return received;
}

function kebab(name: string) {
	return name.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`);
}

function isStyleVisible(node: Element) {
	const style = node.ownerDocument.defaultView?.getComputedStyle(node);
	if (!style) return false;
	return (
		style.display !== "none" &&
		style.visibility !== "hidden" &&
		style.visibility !== "collapse" &&
		style.opacity !== "0"
	);
}

function isVisible(target: Element): boolean {
	if (!target.ownerDocument.contains(target)) return false;
	for (let node: Element | null = target; node; node = node.parentElement) {
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
	toBeVisibleSync(received: unknown) {
		const target = element(received, "toBeVisibleSync");
		return pass(isVisible(target), "to be visible", target);
	},
	toBeInTheDocumentSync(received: unknown) {
		const target = element(received, "toBeInTheDocumentSync");
		return pass(
			target.ownerDocument.contains(target),
			"to be in the document",
			target,
		);
	},
	toBeEnabledSync(received: unknown) {
		const target = element(received, "toBeEnabledSync");
		return pass(!target.matches(":disabled"), "to be enabled", target);
	},
	toBeDisabledSync(received: unknown) {
		const target = element(received, "toBeDisabledSync");
		return pass(target.matches(":disabled"), "to be disabled", target);
	},
	toHaveFocusSync(received: unknown) {
		const target = element(received, "toHaveFocusSync");
		return pass(
			target.ownerDocument.activeElement === target,
			"to have focus",
			target,
		);
	},
	toHaveAttributeSync(received: unknown, name: string, value?: string) {
		const target = element(received, "toHaveAttributeSync");
		const actual = target.getAttribute(name);
		const ok = value === undefined ? actual !== null : actual === value;
		return pass(
			ok,
			`to have attribute ${name}${value === undefined ? "" : `="${value}"`} (actual: ${actual})`,
			target,
		);
	},
	toBeCheckedSync(received: unknown) {
		const target = element(received, "toBeCheckedSync");
		const ok =
			target instanceof HTMLInputElement
				? target.checked
				: target.getAttribute("aria-checked") === "true";
		return pass(ok, "to be checked", target);
	},
	toBePartiallyCheckedSync(received: unknown) {
		const target = element(received, "toBePartiallyCheckedSync");
		const ok =
			target instanceof HTMLInputElement
				? target.indeterminate
				: target.getAttribute("aria-checked") === "mixed";
		return pass(ok, "to be partially checked", target);
	},
	toHaveValueSync(received: unknown, expected: string | number) {
		const target = element(received, "toHaveValueSync");
		let actual: string | number | null;
		if (target instanceof HTMLInputElement && target.type === "number") {
			actual = target.value === "" ? null : Number(target.value);
		} else {
			actual = (target as HTMLInputElement).value;
		}
		return pass(
			actual === expected,
			`to have value ${JSON.stringify(expected)} (actual: ${JSON.stringify(actual)})`,
			target,
		);
	},
	toHaveStyleSync(received: unknown, expected: Record<string, string>) {
		const target = element(received, "toHaveStyleSync");
		const computed = target.ownerDocument.defaultView?.getComputedStyle(target);
		const mismatches = Object.entries(expected).filter(([property, value]) => {
			const want = normalizeStyleValue(target.ownerDocument, property, value);
			return computed?.getPropertyValue(kebab(property)) !== want;
		});
		return pass(
			mismatches.length === 0,
			`to have style ${JSON.stringify(expected)}`,
			target,
		);
	},
});
