"use client";

import type * as React from "react";
import { usePathname } from "@/lib/intl/navigation";
import { DelegatedSessionProvider } from "../state/DelegatedSessionContext";
import DelegatedWorkspaceFrame from "./DelegatedWorkspaceFrame";

/**
 * `/adm` 之下的兩種畫面。
 *
 * 工作區的外框需要一個目標店家（倒數、情境列、側欄連結都以它為錨），所以判準
 * 就是路徑上有沒有 storeId：
 * - `/adm/{storeId}/**` → 工作階段 provider ＋ 工作區外框
 * - 其餘（`/adm/unavailable`）→ 直接渲染，不掛 provider、不輪詢
 *
 * storeId 是 TSID 數字字串，用「全為數字」判定即可把 `/adm/unavailable` 這類
 * 靜態段排除掉，而不必維護一份靜態段清單。
 */
export default function DelegatedRuntimeBoundary({
	admOrigin,
	children,
}: {
	admOrigin: string;
	children: React.ReactNode;
}) {
	const pathname = usePathname() ?? "";
	const segment = pathname.split("/")[2] ?? "";
	const storeId = /^\d{1,19}$/.test(segment) ? segment : null;

	if (!storeId) return <>{children}</>;

	return (
		<DelegatedSessionProvider
			storeId={storeId}
			returnUrl={`${admOrigin}/admin/merchants/${encodeURIComponent(storeId)}`}
			admOrigin={admOrigin}
		>
			<DelegatedWorkspaceFrame storeId={storeId}>
				{children}
			</DelegatedWorkspaceFrame>
		</DelegatedSessionProvider>
	);
}
