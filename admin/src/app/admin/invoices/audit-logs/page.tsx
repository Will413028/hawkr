"use client";

import { useSearchParams } from "next/navigation";
import InvoiceAuditConsole from "@/features/invoices/components/InvoiceAuditConsole";
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";


export default function InvoiceAuditLogsPage() {
	const searchParams = useSearchParams();
	return (
		<InvoiceAuditConsole initialJobId={searchParams.get("job_id") ?? ""} />
	);
}
