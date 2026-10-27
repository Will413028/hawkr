import { type NextRequest, NextResponse } from "next/server";
import {
	encodeGenericBffPath,
	isAllowedGenericBffRequestHeader,
	isDelegatedOperationNamespace,
	isReservedDelegatedOperationHeader,
} from "@/features/delegated-operations/server/bff-boundary";
import { adminSession } from "@/lib/session";
import { BACKEND_BASE_URL } from "@/services/backend";

export const runtime = "nodejs";

const HOP_BY_HOP = new Set([
	"connection",
	"content-length",
	"cookie",
	"host",
	"keep-alive",
	"proxy-authenticate",
	"proxy-authorization",
	"te",
	"trailer",
	"transfer-encoding",
	"upgrade",
]);
// fetch() auto-decompresses but leaves Content-Encoding → would tell the
// browser to re-decompress plain bytes (ERR_CONTENT_DECODING_FAILED). Strip it.
const STRIPPED_RESPONSE = new Set(["content-encoding"]);

type Ctx = { params: Promise<{ path?: string[] }> };

async function handler(request: NextRequest, ctx: Ctx) {
	const { path } = await ctx.params;
	if (isDelegatedOperationNamespace(path ?? [])) {
		return NextResponse.json(
			{
				code: "DELEGATED_OPERATION_REQUIRES_DEDICATED_BFF",
				message: "代行營運請求不得使用一般 BFF",
			},
			{ status: 403 },
		);
	}
	for (const name of request.headers.keys()) {
		if (isReservedDelegatedOperationHeader(name)) {
			return NextResponse.json(
				{
					code: "RESERVED_OPERATION_HEADER",
					message: "請求包含保留的代行營運標頭",
				},
				{ status: 403 },
			);
		}
	}
	const token = await adminSession.token(request);
	if (!token) {
		return NextResponse.json(
			{ code: 401, message: "Unauthorized" },
			{ status: 401 },
		);
	}
	const target = new URL(
		`${BACKEND_BASE_URL}/${encodeGenericBffPath(path ?? [])}`,
	);
	for (const [k, v] of request.nextUrl.searchParams.entries()) {
		target.searchParams.append(k, v);
	}

	const headers = new Headers();
	for (const [k, v] of request.headers.entries()) {
		if (!isAllowedGenericBffRequestHeader(k)) continue;
		headers.set(k, v);
	}
	headers.set("Authorization", `Bearer ${token}`);

	const hasBody = !["GET", "HEAD"].includes(request.method);
	const upstream = await fetch(target, {
		method: request.method,
		headers,
		body: hasBody ? await request.arrayBuffer() : undefined,
		redirect: "manual",
	});

	const resHeaders = new Headers();
	for (const [k, v] of upstream.headers.entries()) {
		const lower = k.toLowerCase();
		if (HOP_BY_HOP.has(lower) || lower === "set-cookie") continue;
		if (STRIPPED_RESPONSE.has(lower)) continue;
		resHeaders.set(k, v);
	}
	// 後端的 Set-Cookie 一律不轉。
	return new NextResponse(upstream.body, {
		status: upstream.status,
		statusText: upstream.statusText,
		headers: resHeaders,
	});
}

export const GET = handler;
export const POST = handler;
export const PUT = handler;
export const PATCH = handler;
export const DELETE = handler;
