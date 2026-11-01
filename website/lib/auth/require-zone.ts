import "server-only";

import { headers } from "next/headers";
import { redirect } from "@/lib/intl/navigation";
import { verifySession } from "./dal";
import type { RequiredAccess } from "./redirect";
import { RETURN_PATH_HEADER } from "./return-path";
import type { Identity } from "./storage";
import { decideZone } from "./zone";

/**
 * 受保護區 layout 的伺服器端判定（`decideZone`）：導向用 `redirect()`，通過回 `"allow"`，
 * 單體暫時答不出來回 `unavailable`。通過時帶這個請求由伺服器確認過的身分（`ZoneIdentityProvider` 的來源）。
 * 讀 cookie 與 header，所以只能在 `<Suspense>` 內呼叫。
 */
export async function requireZone(
	required: RequiredAccess,
): Promise<{ kind: "allow"; identity: Identity } | { kind: "unavailable" }> {
	const [session, requestHeaders] = await Promise.all([
		verifySession(),
		headers(),
	]);
	const decision = decideZone(
		required,
		session,
		requestHeaders.get(RETURN_PATH_HEADER),
	);
	if (decision.kind === "redirect") {
		redirect({ href: decision.href, locale: decision.locale });
	}
	if (decision.kind === "allow" && session.status === "signedIn") {
		return { kind: "allow", identity: session.identity };
	}
	return { kind: "unavailable" };
}
