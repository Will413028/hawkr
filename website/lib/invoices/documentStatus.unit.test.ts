import { describe, expect, it } from "vitest";
import {
	INVOICE_DOCUMENT_STATUS_LABEL,
	InvoiceDocumentStatus,
} from "@/lib/codemeta/generated";
import { invoiceCarrierLabel } from "./carrier";
import {
	canResendInvoiceNotification,
	foldInvoiceDocumentStatus,
} from "./documentStatus";

/**
 * 這兩張 display map 守的是同一件事：**後端先上線一個前端還不知道的值時，
 * 畫面不能白掉**（58A §0.6 C11）。
 *
 * 之所以要單獨測「未知值」而不是只測已知值：exhaustive switch 對現有值域一定
 * 全綠，而漏掉 default 分支的症狀只在後端加值那一天才出現，那時前端已經上線了。
 */
describe("發票文件狀態的折疊表", () => {
	it("五個進行中的狀態折成同一個買家可見標籤", () => {
		const processing = [
			InvoiceDocumentStatus.PENDING,
			InvoiceDocumentStatus.ISSUING,
			InvoiceDocumentStatus.ISSUE_FAILED,
			InvoiceDocumentStatus.AWAITING_MANUAL,
			InvoiceDocumentStatus.VOIDING,
		].map((status) => foldInvoiceDocumentStatus(status).label);
		expect(new Set(processing).size).toBe(1);
	});

	it("已開立、已作廢與不由平台開立必須彼此可分辨", () => {
		const labels = [
			InvoiceDocumentStatus.ISSUED,
			InvoiceDocumentStatus.VOIDED,
			InvoiceDocumentStatus.NOT_APPLICABLE,
		].map((status) => foldInvoiceDocumentStatus(status).label);
		expect(new Set(labels).size).toBe(3);
	});

	it("中文一律取自 CodeMeta，不在前端手抄第二份", () => {
		expect(foldInvoiceDocumentStatus(InvoiceDocumentStatus.ISSUED).label).toBe(
			INVOICE_DOCUMENT_STATUS_LABEL.ISSUED,
		);
		// 「已被重開取代」對買家的意思就是已作廢——借 VOIDED 的 label，不另寫中文。
		expect(
			foldInvoiceDocumentStatus(InvoiceDocumentStatus.SUPERSEDED).label,
		).toBe(INVOICE_DOCUMENT_STATUS_LABEL.VOIDED);
	});

	it("後端加了新狀態時落 default，而不是空字串", () => {
		const unseen = foldInvoiceDocumentStatus("ISSUED_BY_TAX_AUTHORITY");
		expect(unseen.unknown).toBe(true);
		expect(unseen.label.length).toBeGreaterThan(0);
		// 未知值不得被說成已開立或已作廢——那會讓買家對一張還沒開的發票停止追。
		expect(unseen.label).not.toBe(INVOICE_DOCUMENT_STATUS_LABEL.ISSUED);
		expect(unseen.label).not.toBe(INVOICE_DOCUMENT_STATUS_LABEL.VOIDED);
	});
});

describe("可否重寄通知（端點 71／32 的前置）", () => {
	it("三個可重寄的狀態逐一為 true", () => {
		// 這三值必須與 invoice-service 的 `resendableStatus` 一字不差。
		for (const status of [
			InvoiceDocumentStatus.ISSUED,
			InvoiceDocumentStatus.RESOLVED_EXTERNALLY,
			InvoiceDocumentStatus.VOIDED,
		]) {
			expect(canResendInvoiceNotification(status)).toBe(true);
		}
	});

	it("還沒開好的文件一律 false —— 按下去只會拿到看不懂的 9301", () => {
		for (const status of [
			InvoiceDocumentStatus.PENDING,
			InvoiceDocumentStatus.ISSUING,
			InvoiceDocumentStatus.ISSUE_FAILED,
			InvoiceDocumentStatus.AWAITING_MANUAL,
			InvoiceDocumentStatus.VOIDING,
			InvoiceDocumentStatus.NOT_APPLICABLE,
			InvoiceDocumentStatus.CANCELLED_BEFORE_ISSUE,
			InvoiceDocumentStatus.SUPERSEDED,
		]) {
			expect(canResendInvoiceNotification(status)).toBe(false);
		}
	});

	it("後端加了新狀態時 fail-closed（不渲染），而不是樂觀放行", () => {
		expect(canResendInvoiceNotification("ISSUED_BY_TAX_AUTHORITY")).toBe(false);
	});
});

describe("載具類型的名稱表", () => {
	it("四種載具的中文取自 CodeMeta", () => {
		expect(invoiceCarrierLabel("MOBILE_BARCODE")).toBe("手機條碼");
		expect(invoiceCarrierLabel("TAX_ID")).toBe("打統編（三聯式）");
	});

	it("後端加了第五種載具時落 default，畫面仍有東西可印", () => {
		expect(invoiceCarrierLabel("MEMBER_CARRIER")).toBe("MEMBER_CARRIER");
		expect(invoiceCarrierLabel("MEMBER_CARRIER").length).toBeGreaterThan(0);
	});
});
