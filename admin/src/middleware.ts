import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { isCrossOriginWrite } from "@/lib/cross-origin";

export function middleware(request: NextRequest) {
	if (
		request.nextUrl.pathname.startsWith("/api/") &&
		isCrossOriginWrite(request.method, request.headers, request.nextUrl.origin)
	) {
		return NextResponse.json(
			{ message: "不接受跨來源的請求" },
			{ status: 403, headers: { "Cache-Control": "no-store" } },
		);
	}
	return NextResponse.next();
}

export const config = {
	matcher: ["/((?!_next/static|_next/image|favicon\\.png).*)"],
};
