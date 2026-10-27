"use server";

import { auth } from "@clerk/nextjs/server";
import { revalidateTag } from "next/cache";
import { sellerTag } from "@/lib/seo/tags";

function validSellerCacheId(value: string | undefined): value is string {
	return Boolean(value && /^\d{1,20}$/.test(value));
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
	if (!(await auth()).userId) {
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
