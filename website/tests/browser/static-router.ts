/** 預覽不掛 App Router：身分穩定的靜態 router，給 `@/lib/intl/use-router` 的 mock 用。 */
export const staticRouter = {
	push: () => {},
	replace: () => {},
	prefetch: () => {},
	back: () => {},
	forward: () => {},
	refresh: () => {},
};
