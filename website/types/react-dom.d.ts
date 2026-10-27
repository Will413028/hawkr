declare module "react-dom" {
	import type * as React from "react";

	export function createPortal(
		children: React.ReactNode,
		container: Element | DocumentFragment,
		key?: string | null,
	): React.ReactPortal;

	// ⛔ 這份本地宣告會**蓋掉** @types/react-dom，所以用得到的 API 必須列在這裡，
	// 否則 tsc 會說 react-dom 沒有這個 export（執行期其實一直都有）。
	// 簽章與 @types/react-dom 相同。
	export function flushSync<R>(fn: () => R): R;

	export function preconnect(
		href: string,
		options?: { crossOrigin?: "anonymous" | "use-credentials" | "" },
	): void;
}
