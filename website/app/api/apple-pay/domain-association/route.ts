import { NextResponse } from "next/server";
import { env } from "@/lib/config/env";
import { APPLE_PAY_DOMAIN_ASSOCIATION } from "@/lib/payment/applePayDomainAssociation";

export async function GET() {
	const content = APPLE_PAY_DOMAIN_ASSOCIATION[env.flavor];
	if (!content) {
		return new NextResponse("Not Found", {
			status: 404,
			headers: { "Cache-Control": "no-store" },
		});
	}
	return new NextResponse(content, {
		status: 200,
		headers: {
			"Content-Type": "text/plain; charset=utf-8",
			"Cache-Control": "no-store",
		},
	});
}
