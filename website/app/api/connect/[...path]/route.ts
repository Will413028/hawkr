// Connect 的第一方路徑（lib/connect/transport.ts 的 FIRST_PARTY_CONNECT_PATH）在本機的代行：
// /api/connect/{service}/{method} 轉到單體（入口）的根目錄。啟用條件與 app/api/v1/[...path] 相同（本機開發，
// 或 loopback 上明確開啟的 Playwright build）；部署環境由 web host 的第一方反向代理處理這個前綴，這裡一律
// 404。轉送細節（登入憑證 Cookie 原樣送出、3xx 不跟隨、多個 Set-Cookie 逐筆傳回、串流）在
// lib/api/local-proxy.ts 的 passThroughToUpstream。
import { type NextRequest, NextResponse } from "next/server";
import {
	connectProxyTarget,
	isLocalApiProxyEnabled,
	passThroughToUpstream,
} from "@/lib/api/local-proxy";

async function proxy(req: NextRequest): Promise<Response> {
	if (!isLocalApiProxyEnabled(req.url)) {
		return new NextResponse(null, { status: 404 });
	}
	return passThroughToUpstream(req, connectProxyTarget(req.url));
}

export const GET = proxy;
export const POST = proxy;
