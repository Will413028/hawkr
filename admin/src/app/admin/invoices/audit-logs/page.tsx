"use client";

import { useSearchParams } from "next/navigation";
import InvoiceAuditConsole from "@/features/invoices/components/InvoiceAuditConsole";
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";

// A10｜發票稽核紀錄（58A §A10，端點 61）。
//
// 本檔是薄 wrapper：只負責讀 `?job_id=`（A06 的「查看完整稽核紀錄」帶進來）並把它
// 交給本體。實作在 `features/invoices/components/InvoiceAuditConsole.tsx`——理由寫在
// 那一支的檔頭（體量與唯一的 feature 模組先例同級）。
//
// 兩份 CSS 都要 import：`.adm-audit-*` / `.adm-operation-*` 住在 DelegatedOperations.css
// （頁面級樣式表，不是全域），`.invoice-*`（`<pre>`、`<details>`）住在 Invoice.css。
// 少 import 一份的症狀是排版散掉但不報錯——這正是 §0.6 C07 記過的那一類。
// A01 已經是同樣的兩份 import，本頁不是新慣例。

export default function InvoiceAuditLogsPage() {
	const searchParams = useSearchParams();
	return (
		<InvoiceAuditConsole initialJobId={searchParams.get("job_id") ?? ""} />
	);
}
