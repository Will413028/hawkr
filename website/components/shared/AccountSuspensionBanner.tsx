"use client";

import Link from "@/components/shared/Link";
import WarningNotice from "@/components/shared/WarningNotice";
import { useAuth } from "@/lib/auth/context";
import {
	suspensionCategoryText,
	suspensionReasonText,
} from "@/lib/auth/suspension";
import { useUiText } from "@/lib/i18n/ui-text";

/**
 * 帳號被停權時的全站橫幅（受限登入，A7-D17）：停權的帳號仍能瀏覽，橫幅說明有部分功能受限並連到
 * 帳號設定頁。只在伺服器確認 `signedIn` 且帶停權狀態時顯示；`unknown`、`guest`、`unavailable` 一律不顯示。
 */
export default function AccountSuspensionBanner() {
	const ut = useUiText();
	const auth = useAuth();
	if (auth.status !== "signedIn") return null;
	const suspension = auth.identity.suspension;
	if (!suspension) return null;
	const category = suspensionCategoryText(ut, suspension);
	const reason = suspensionReasonText(suspension);
	return (
		<div className="px-4 pt-2">
			<WarningNotice>
				<span>帳號已停權，部分功能受限</span>
				{category ? <span>{category}</span> : null}
				{reason ? <span>{reason}</span> : null}
				<Link href="/account/settings" className="font-bold underline">
					查看詳情
				</Link>
			</WarningNotice>
		</div>
	);
}
