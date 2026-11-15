import { expect, vi } from "vitest";

/** vi.waitFor 的預設逾時是 1 秒；CI 上抽屜動畫與版面需要較長的重試窗口。 */
export function waitFor<T>(
	callback: () => T | Promise<T>,
	options: { timeout?: number } = {},
) {
	return vi.waitFor(callback, { timeout: 5000, interval: 50, ...options });
}

/** 重試到元素的文字（空白正規化後）包含 `text`；`getElement` 每次重試都重新取得元素。 */
export async function expectTextContains(
	getElement: () => Element | null,
	text: string,
) {
	if (text === "") throw new TypeError("expectTextContains 不接受空字串");
	await waitFor(() => {
		const element = getElement();
		expect(element, "找不到元素").not.toBeNull();
		const actual = (element?.textContent ?? "").replace(/\s+/g, " ").trim();
		expect(actual).toContain(text);
	});
}

/** 重試到元素不存在（對應 testing-library 的 queryBy*().not.toBeInTheDocument()）。 */
export async function expectAbsent(query: () => Element | null) {
	await waitFor(() => expect(query()).toBeNull());
}

/** 非 null 斷言：元素不存在就讓測試失敗。 */
export function required<T>(value: T | null | undefined, what: string): T {
	expect(value, `找不到 ${what}`).not.toBeNull();
	expect(value, `找不到 ${what}`).not.toBeUndefined();
	return value as T;
}
