declare module "react-dom" {
	import type * as React from "react";

	export function createPortal(
		children: React.ReactNode,
		container: Element | DocumentFragment,
		key?: string | null,
	): React.ReactPortal;

	export function flushSync<R>(fn: () => R): R;

	export function preconnect(
		href: string,
		options?: { crossOrigin?: "anonymous" | "use-credentials" | "" },
	): void;
}
