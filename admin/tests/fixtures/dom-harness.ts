import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { join } from "node:path";
import { afterEach } from "node:test";
import type { Transport } from "@connectrpc/connect";
import { QueryClient } from "@tanstack/react-query";
import { JSDOM } from "jsdom";

/**
 * 元件實際掛進 DOM 的測試用具：JSDOM、React、connect-query 的 transport 與 QueryClient，以及 next/navigation 的替身
 * （頁面讀 useParams／useRouter）。必須在載入任何 React 元件之前 import 這個檔案（全域的 window 與 document 要先就位）。
 */

export const dom = new JSDOM("<!doctype html><body></body>", {
	url: "http://localhost/",
	pretendToBeVisual: true,
});
const g = globalThis as Record<string, unknown>;
g.window = dom.window;
// next/link 載入時讀 `self`。
g.self = dom.window;
g.document = dom.window.document;
g.HTMLElement = dom.window.HTMLElement;
g.Node = dom.window.Node;
g.IS_REACT_ACT_ENVIRONMENT = true;

/** 相對路徑從 tests/ 起算（跟其他測試檔一樣寫 "../src/..."）。 */
export const requireFromTest = createRequire(
	join(__dirname, "..", "anchor.js"),
);
requireFromTest.extensions[".css"] = () => {};

/** 頁面讀到的路由參數、網址查詢字串，與 router.push／replace 的記錄。 */
export const navigation: {
	params: Record<string, string>;
	search: string;
	pushed: string[];
	replaced: string[];
} = {
	params: {},
	search: "",
	pushed: [],
	replaced: [],
};
const searchListeners = new Set<() => void>();
let searchSnapshot = { key: "", params: new URLSearchParams() };
function currentSearchParams(): URLSearchParams {
	if (searchSnapshot.key !== navigation.search) {
		searchSnapshot = {
			key: navigation.search,
			params: new URLSearchParams(navigation.search),
		};
	}
	return searchSnapshot.params;
}
/** 測試改網址查詢字串（等同瀏覽器網址列變了），讓正在掛載的頁面重新讀取。 */
export function setSearch(search: string) {
	navigation.search = search;
	for (const listener of [...searchListeners]) listener();
}
{
	const path = requireFromTest.resolve("next/navigation");
	const react = requireFromTest("react") as {
		useSyncExternalStore: <T>(
			subscribe: (onChange: () => void) => () => void,
			getSnapshot: () => T,
		) => T;
	};
	const subscribe = (listener: () => void) => {
		searchListeners.add(listener);
		return () => {
			searchListeners.delete(listener);
		};
	};
	const router = {
		push: (href: string) => {
			navigation.pushed.push(href);
		},
		replace: (href: string) => {
			navigation.replaced.push(href);
			setSearch(href.includes("?") ? href.slice(href.indexOf("?") + 1) : "");
		},
	};
	requireFromTest.cache[path] = {
		id: path,
		filename: path,
		loaded: true,
		exports: {
			useParams: () => navigation.params,
			useSearchParams: () =>
				react.useSyncExternalStore(subscribe, currentSearchParams),
			useRouter: () => router,
		},
	} as never;
}

export const React = requireFromTest("react") as {
	createElement: (
		type: unknown,
		props?: unknown,
		...children: unknown[]
	) => unknown;
	act: (fn: () => void | Promise<void>) => Promise<void>;
};
const { createRoot } = requireFromTest("react-dom/client") as {
	createRoot: (el: Element) => {
		render: (node: unknown) => void;
		unmount: () => void;
	};
};
const { QueryClientProvider } = requireFromTest("@tanstack/react-query") as {
	QueryClientProvider: unknown;
};
const { TransportProvider } = requireFromTest("@connectrpc/connect-query") as {
	TransportProvider: unknown;
};

export function setValue(el: Element, value: string) {
	const proto =
		el instanceof dom.window.HTMLTextAreaElement
			? dom.window.HTMLTextAreaElement.prototype
			: el instanceof dom.window.HTMLSelectElement
				? dom.window.HTMLSelectElement.prototype
				: dom.window.HTMLInputElement.prototype;
	Object.getOwnPropertyDescriptor(proto, "value")?.set?.call(el, value);
	const type = el instanceof dom.window.HTMLSelectElement ? "change" : "input";
	el.dispatchEvent(new dom.window.Event(type, { bubbles: true }));
}

export function byId<T extends Element = Element>(id: string): T {
	const el = dom.window.document.getElementById(id);
	assert.ok(el, `找不到 #${id}`);
	return el as unknown as T;
}

export function maybeById(id: string): Element | null {
	return dom.window.document.getElementById(id);
}

export function buttonByText(text: string): HTMLButtonElement | undefined {
	return [...dom.window.document.querySelectorAll("button")].find(
		(b) => b.textContent === text,
	) as HTMLButtonElement | undefined;
}

export function buttonsByText(text: string): HTMLButtonElement[] {
	return [...dom.window.document.querySelectorAll("button")].filter(
		(b) => b.textContent === text,
	) as HTMLButtonElement[];
}

export async function click(text: string) {
	const button = buttonByText(text);
	assert.ok(button, `找不到按鈕「${text}」`);
	await React.act(async () => {
		button.click();
	});
}

export async function clickElement(el: Element) {
	await React.act(async () => {
		(el as HTMLElement).click();
	});
}

export const bodyText = () => dom.window.document.body.textContent ?? "";

export async function pressKey(key: string, shiftKey = false) {
	await React.act(async () => {
		const target =
			dom.window.document.activeElement ?? dom.window.document.body;
		target.dispatchEvent(
			new dom.window.KeyboardEvent("keydown", { key, shiftKey, bubbles: true }),
		);
	});
}

/** 等所有查詢完成（停用的查詢不算）。 */
export async function settle(queryClient: QueryClient) {
	const done = () => {
		const queries = queryClient.getQueryCache().getAll();
		return (
			queries.length > 0 &&
			queryClient.isFetching() === 0 &&
			queries.every(
				(q) => q.state.status !== "pending" || q.state.fetchStatus === "idle",
			)
		);
	};
	const deadline = Date.now() + 5000;
	while (!done()) {
		assert.ok(Date.now() < deadline, "查詢在 5 秒內沒有完成");
		await React.act(async () => {
			await new Promise((resolve) => setTimeout(resolve, 1));
		});
	}
	await React.act(async () => {
		await new Promise((resolve) => setTimeout(resolve, 0));
	});
}

/** 反覆讓 React 與查詢跑一輪，直到條件成立（最多 5 秒）。 */
export async function until(done: () => boolean, what: string) {
	const deadline = Date.now() + 5000;
	while (!done()) {
		assert.ok(Date.now() < deadline, `5 秒內沒有等到：${what}`);
		await React.act(async () => {
			await new Promise((resolve) => setTimeout(resolve, 1));
		});
	}
}

const openRoots = new Set<() => Promise<void>>();
afterEach(async () => {
	for (const unmount of [...openRoots]) await unmount();
	openRoots.clear();
	navigation.params = {};
	navigation.search = "";
	navigation.pushed = [];
	navigation.replaced = [];
});

/** 掛載一個元件（包在 transport 與 QueryClient 裡）；測試結束時自動卸載。 */
export function mount(element: unknown, transport: Transport) {
	const host = dom.window.document.createElement("div");
	dom.window.document.body.appendChild(host);
	const root = createRoot(host);
	const queryClient = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	let mounted = true;
	const unmount = () =>
		React.act(async () => {
			if (!mounted) return;
			mounted = false;
			root.unmount();
			host.remove();
		});
	openRoots.add(unmount);
	return {
		queryClient,
		unmount,
		render: async ({ settle: wait = true }: { settle?: boolean } = {}) => {
			await React.act(async () => {
				root.render(
					React.createElement(
						TransportProvider,
						{ transport },
						React.createElement(
							QueryClientProvider,
							{ client: queryClient },
							element,
						),
					),
				);
			});
			if (wait) await settle(queryClient);
		},
	};
}
