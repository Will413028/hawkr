// 單體檔案端點的第一方路徑在本機的代行：/api/files/secure_uploads/{token}/chunks/{index}（機敏檔分段上傳，PUT 原始位元組）
// 與 /api/files/secure_uploads/{token}（暫存預覽，GET）原路徑轉到單體（入口）的根目錄 `/api/files`，不是 `/api/v1/files`。
// 啟用條件與 app/api/connect/[...path] 相同；部署環境由 web host 的第一方反向代理處理 `/api/files/*`，這裡一律 404。
// body 不經解析，由 passThroughToUpstream 原樣轉送。
import { type NextRequest, NextResponse } from "next/server";
import {
	filesProxyTarget,
	isLocalApiProxyEnabled,
	passThroughToUpstream,
} from "@/lib/api/local-proxy";

async function proxy(req: NextRequest): Promise<Response> {
	if (!isLocalApiProxyEnabled(req.url)) {
		return new NextResponse(null, { status: 404 });
	}
	return passThroughToUpstream(req, filesProxyTarget(req.url));
}

export const GET = proxy;
export const PUT = proxy;
