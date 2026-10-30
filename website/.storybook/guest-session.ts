/**
 * Storybook 裡的登入狀態：預覽一律是訪客。
 *
 * session 是 HttpOnly cookie，真的 `AuthProvider` 掛載時一定會問單體 GetSession（lib/api/auth.ts）。
 * 預覽不該連出去（也沒有單體可連），所以這支 GetSession 在瀏覽器端以單體對訪客的答案攔下：
 * `unauthenticated`（Connect 的 HTTP 401）。需要登入身分的預覽改用 `lib/hooks/useQueryIdentity` 的 mock，
 * 不靠這裡。其他請求照舊。
 */
const GET_SESSION = /IdentityAppService\/GetSession(?:\?|$)/;

const realFetch = globalThis.fetch.bind(globalThis);

globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
	const url =
		typeof input === "string"
			? input
			: input instanceof URL
				? input.href
				: input.url;
	if (GET_SESSION.test(url)) {
		return Promise.resolve(
			new Response(
				JSON.stringify({ code: "unauthenticated", message: "guest" }),
				{ status: 401, headers: { "Content-Type": "application/json" } },
			),
		);
	}
	return realFetch(input, init);
}) as typeof fetch;
