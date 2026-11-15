import { expect } from "vitest";

/** 取得必須存在的元素：不存在就讓測試失敗，並把型別收斂為非 null。 */
export function queryRequired<T extends Element>(
	root: ParentNode,
	selector: string,
): T {
	const element = root.querySelector<T>(selector);
	expect(element, `找不到 ${selector}`).not.toBeNull();
	return element as T;
}
