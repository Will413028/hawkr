"use server";

import { createClient } from "@connectrpc/connect";
import { IdentityAppService } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { revalidateTag } from "next/cache";
import { sessionCookieHeader } from "@/lib/auth/session-cookie.server";
import { authedTransport } from "@/lib/connect/transport";
import { defaultLocale } from "@/lib/intl/locales";
import { sellerTag } from "@/lib/seo/tags";

function validSellerCacheId(value: string | undefined): value is string {
	return Boolean(value && /^\d{1,20}$/.test(value));
}

/**
 * 伺服器端確認這個請求帶著有效的 session：把訪客的 `__Host-hk_session` 轉送給單體的 GetSession（GET，
 * 唯讀；D6，ADR 2026-10-30-identity-self-hosted-federated-login）。沒有 cookie、單體說沒登入或答不出來，
 * 都不放行。
 */
async function hasLiveSession(): Promise<boolean> {
	const cookie = await sessionCookieHeader();
	if (!cookie) return false;
	try {
		const res = await createClient(
			IdentityAppService,
			authedTransport({ locale: defaultLocale, cookie }),
		).getSession({});
		return Boolean(res.account?.id);
	} catch {
		// 「沒登入」與「單體暫時答不出來」都不放行；後者讓使用者重試。
		return false;
	}
}

/**
 * Seller profile writes happen against the gateway, while the public
 * storefront is cached by both the internal account id and canonical
 * store-code route. Expire both immediately after a successful own-profile
 * update so the next storefront request reads the new public profile.
 */
export async function revalidateOwnSellerProfile(input: {
	accountId?: string;
	storeCode?: string;
}): Promise<void> {
	if (!(await hasLiveSession())) {
		throw new Error("登入狀態已失效，無法更新公開商店頁");
	}

	const ids = new Set(
		[input.accountId, input.storeCode].filter(validSellerCacheId),
	);
	if (ids.size === 0) {
		throw new Error("缺少可更新的商店識別碼");
	}

	for (const id of ids) {
		revalidateTag(sellerTag(id), { expire: 0 });
	}
}
