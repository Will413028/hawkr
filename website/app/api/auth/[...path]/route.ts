// 單體發起登入的第一方路徑（lib/auth/oauth-start.ts 的 OAUTH_START_PATH）在本機的代行：
// /api/auth/{provider}/start 與 /api/auth/{provider}/callback 原路徑轉到單體（入口）。啟用條件與
// app/api/connect/[...path] 相同；部署環境由 web host 的第一方反向代理處理 `/api/auth/*`，這裡一律 404。
// 單體回的是 302（start 去 Google 或 LINE、callback 設 `__Host-hk_session` 後回站內）：由
// passThroughToUpstream 不跟隨、連同 Location 與 Set-Cookie 原樣交給瀏覽器。登入流程只有 GET。
import { type NextRequest, NextResponse } from "next/server";
import {
	authProxyTarget,
	isLocalApiProxyEnabled,
	passThroughToUpstream,
} from "@/lib/api/local-proxy";

async function proxy(req: NextRequest): Promise<Response> {
	if (!isLocalApiProxyEnabled(req.url)) {
		return new NextResponse(null, { status: 404 });
	}
	return passThroughToUpstream(req, authProxyTarget(req.url));
}

export const GET = proxy;
