// Connect 的第一方路徑（lib/connect/transport.ts 的 FIRST_PARTY_CONNECT_PATH）在本機的代行：
// /api/connect/{service}/{method} 轉到單體（入口）的根目錄。啟用條件與 app/api/v1/[...path] 相同（本機開發，
// 或 loopback 上明確開啟的 Playwright build）；部署環境由 web host 的第一方反向代理處理這個前綴，這裡一律
// 404。回應以串流原樣傳回，Connect 的 server streaming 也逐筆送達。
// 請求的 header（Authorization、Cookie 等登入憑證）原樣轉送，回應的 Set-Cookie 逐筆傳回。
import { type NextRequest, NextResponse } from "next/server";
import {
	connectProxyTarget,
	isLocalApiProxyEnabled,
} from "@/lib/api/local-proxy";

async function proxy(req: NextRequest): Promise<Response> {
	if (!isLocalApiProxyEnabled(req.url)) {
		return new NextResponse(null, { status: 404 });
	}
	const headers = new Headers();
	for (const [k, v] of req.headers.entries()) {
		const lk = k.toLowerCase();
		if (lk === "host" || lk.startsWith("x-nextjs-")) continue;
		headers.set(k, v);
	}
	let upstream: Response;
	try {
		upstream = await fetch(connectProxyTarget(req.url), {
			method: req.method,
			headers,
			body: req.method === "GET" ? undefined : await req.arrayBuffer(),
			signal: req.signal,
		});
	} catch {
		return new NextResponse("upstream unreachable", { status: 502 });
	}
	const respHeaders = new Headers(upstream.headers);
	// Node 的 fetch 已經解壓縮，去掉過時的編碼與長度，瀏覽器才不會再解一次。
	respHeaders.delete("content-encoding");
	respHeaders.delete("transfer-encoding");
	respHeaders.delete("content-length");
	// `new Headers(upstream.headers)` 會把多個 Set-Cookie 併成一個以逗號串起的值（undici），cookie 就壞了；
	// 以 getSetCookie() 拆開後逐筆 append（同 app/api/v1/[...path]）。
	const setCookies = upstream.headers.getSetCookie();
	respHeaders.delete("set-cookie");
	for (const cookie of setCookies) respHeaders.append("set-cookie", cookie);
	return new Response(upstream.body, {
		status: upstream.status,
		headers: respHeaders,
	});
}

export const GET = proxy;
export const POST = proxy;
