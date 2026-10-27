"use client";

import Link from "next/link";
import { use } from "react";
import SessionAuditConsole from "@/features/delegated-operations/components/SessionAuditConsole";
import { isDelegatedSessionId } from "@/features/delegated-operations/governance";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/**
 * 代行紀錄（治理視角）。工作階段 ID 的取得管道有三條：入口頁的列表（56 §11.3 P1
 * 的 `GET /admin/delegated-operations`）、建立工作階段當下的回應，以及維運 SQL。
 * 列表是常態管道 —— 沒有它的時候，回看紀錄得先請後端工程師代查一個 ID。
 */
export default function DelegatedOperationAuditPage({
	params,
}: {
	params: Promise<{ sessionId: string }>;
}) {
	const { sessionId } = use(params);

	if (!isDelegatedSessionId(sessionId)) {
		return (
			<div className="adm-audit-console">
				<div className="adm-operation-preflight-blocked" role="alert">
					<MaterialSymbol name="gpp_bad" />
					<div>
						<strong>工作階段識別碼格式不正確</strong>
						<p>
							代行工作階段 ID 是一組數字。工作階段代號（OPS-
							開頭）不是這個值，請回列表以代號查詢。
						</p>
						<Link href="/admin/delegated-operations">回代行紀錄列表</Link>
					</div>
				</div>
			</div>
		);
	}

	return <SessionAuditConsole sessionId={sessionId} />;
}
