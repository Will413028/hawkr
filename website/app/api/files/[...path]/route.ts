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
