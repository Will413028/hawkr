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
