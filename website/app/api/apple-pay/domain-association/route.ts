import { NextResponse } from "next/server";
import { env } from "@/lib/config/env";
import { APPLE_PAY_DOMAIN_ASSOCIATION } from "@/lib/payment/applePayDomainAssociation";

// The domain-association blob is a PUBLIC, per-environment static value
// (committed in applePayDomainAssociation.ts). It is too large for a Cloudflare
// Worker secret (5.6 kB > 5.1 kB limit), so it does NOT come from process.env —
// it ships in the bundle, picked by build-time flavor. Keep this per-request so
// the flavor-selected blob is served fresh; Apple re-fetches it when verifying
// the merchant domain.
export const dynamic = "force-dynamic";

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
