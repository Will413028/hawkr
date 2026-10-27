"use client";

import { createClient } from "@connectrpc/connect";
import { useTransport } from "@connectrpc/connect-query";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { useMutation } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import type React from "react";
import { useMemo, useState } from "react";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, StatusBadge } from "@/components/UI";
import { INVOICE_OPERATION_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import UnavailableOperationsNotice from "@/features/invoices/components/UnavailableOperationsNotice";
import {
	A11_CARRIER_TYPES,
	ALLOWED_COMBOS,
	createInvoiceIdempotencyKey,
	hasMixedTaxCategories,
	type IssuanceDraft,
	issuanceBlockers,
	LINE_TYPE_OPTIONS,
	type LineDraft,
	lineNetAmount,
	MAX_LINES,
	MAX_REASON_RUNES,
	SOURCE_KIND_LABEL,
	SOURCE_KIND_MANUAL_LINES,
	SOURCE_KIND_ORDER_SNAPSHOT,
	summariseLines,
	TAX_CATEGORY_OPTIONS,
	toIssuanceRequest,
} from "@/features/invoices/issuance";
import type {
	InvoiceEnvelope,
	ManualIssuanceResultData,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_DOCUMENT_KIND_LABEL,
	INVOICE_ISSUER_TYPE_LABEL,
	INVOICE_OPERATION_STATUS_LABEL,
	INVOICE_RECIPIENT_TYPE_LABEL,
	InvoiceDocumentKind,
	InvoiceIssuerType,
	InvoiceLineType,
	InvoiceRecipientType,
} from "@/lib/codemeta";
import { errorMessage } from "@/lib/error-text";
import { formatAmount } from "@/lib/format";
import { request } from "@/services/api";
import { ApiClientError } from "@/services/api-error";
import ManualIssuanceConfirmDialog from "./ManualIssuanceConfirmDialog";
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A11｜手動開立發票（58A §A11，端點 57 `POST /api/v1/admin/invoices/issuances`）。
//
// ## 這一頁的東西全部是不可逆的
//
// 送出即產生一張真實的稅務憑證，之後只能作廢或折讓。頁頂的紅框警示是常駐的，
// 送出前一定要經過確認框（明文 checkbox），而理由是必填且會寫進發票稽核紀錄。
//
// ## `Idempotency-Key` 走 header，所以不能用 `api.post`
//
// gateway 的 `middleware.RequireIdempotencyKey` 掛在整個 `/admin/invoices` 群組上，
// POST 少了這個 header 一律 400。而 `services/api.ts` 的 `api.post` 不接受自訂
// header ⇒ 本頁直接用它底下的 `request()`（同一支實作、同一套 envelope 處理）。
// BFF 的轉送白名單已經含 `idempotency-key`
// （`features/delegated-operations/server/bff-boundary.ts` 的
// `GENERIC_BFF_FORWARDED_HEADERS`），不需要另外開洞。
//
// ## `ORDER_SNAPSHOT` 本波不可選
//
// 矩陣第一列（賣家 → 買家 → 訂單快照）在契約上合法，但 invoice-service 的
// `AdminIssuance.Create` 對它回 `ErrIssuanceOrderSnapshotUnavailable`
// （`internal/usecase/admin_issuance.go`）⇒ `codes.Unimplemented` ⇒ Gateway 5xx。
// 原因寫在那個 error 的註解：ORD 的訂單快照回應不帶 `store_id` / `buyer_id`，
// 本服務無法驗「訂單的賣家 ＝ issuer.storeId」，也拿不到 I-47 要求的 `buyer_id`；
// 拿 ADM 手填的 buyerId 頂替是明令禁止的（那會讓本頁變成 IDOR 入口）。
// ⇒ 這個來源與只能搭配它的「受票人＝買家」都渲染成不可選並附原因，另加一則
// `UnavailableOperationsNotice`。**不是靜默省略**：從 A05／A06 帶 `?order_no=`
// 進來的人必須看得到「為什麼這裡沒有訂單快照」。
//
// ## 載具本波不可選
//
// `carrierType` / `carrierValue` 在 proto、Gateway DTO 與 usecase 的
// `ManualIssuanceRecipient` 都存在，**但 `AdminIssuance.buildDocument` 一個 `Carrier*`
// 欄位都沒有設**（實查 `internal/usecase/admin_issuance.go`：`Carrier` 在該檔只出現在
// struct 宣告的兩行，`domain.NewDocument` 的 `CarrierType` / `CarrierId1Ciphertext` /
// `CarrierValueHash` 全部沒被填）⇒ 送過去的載具**會被靜默丟掉**，開出來的是一張無載具
// 的發票。
//
// ⇒ 這一波把載具欄位渲染成不可選並附原因，而不是收一個沒有人會用的值。
// 收了才是最糟的：`carrierValue` 是 P 類個資，而它換不到任何東西——58A §2.0 記過同一個
// 形狀（「強制收集一個永遠不會被顯示的欄位」）。
//
// ⚠️ 附帶查證：gateway access log 的遮罩是 **allowlist**（`middleware/logger.go` 的
// `allowedKeys`，「Anything not here defaults to redacted」）⇒ `carrierValue` 本來就會被
// 遮，本節的理由是「值被丟掉」不是「值會外洩」。
//
// ## 前端不算稅
//
// 金額卡只回顯輸入值的合計。表頭三欄與稅額一律由後端依 58 §5A.6 計算（含尾差的
// 最大餘數處理）——前端算一份就是第二個真相，而發票金額算錯是不可回收的。

const ISSUANCES_ENDPOINT = "/api/v1/admin/invoices/issuances";

/** 本波不可用的 `source.kind`（理由見檔頭）。 */
const UNAVAILABLE_SOURCES = new Set<string>([SOURCE_KIND_ORDER_SNAPSHOT]);
const UNAVAILABLE_SOURCE_REASON =
	"後端尚未提供訂單快照的賣家與買家解析，本波不開放。";

function newLine(): LineDraft {
	return {
		key:
			typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
				? crypto.randomUUID()
				: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
		lineType: InvoiceLineType.PRODUCT,
		name: "",
		quantity: "1",
		unitPrice: "",
		// 預設稅別取碼表的第一個 **Code**（`TAXABLE`）。不寫死字面值也不用
		// `InvoiceTaxCategory.TAXABLE`——後者給的是 Value（`"1"`），而端點 57 收的是
		// Code（`admin_issuance.go` 的 `InvoiceTaxCategoryFromCode`），送錯整張表單被拒。
		taxCategory: TAX_CATEGORY_OPTIONS[0][0],
	};
}

function Card({
	title,
	hint,
	children,
}: {
	title: string;
	hint?: string;
	children: React.ReactNode;
}) {
	return (
		<div className="card invoice-form-card">
			<div className="card-header">
				<h3>{title}</h3>
			</div>
			<div className="card-body">
				{hint ? <p className="invoice-section__hint">{hint}</p> : null}
				{children}
			</div>
		</div>
	);
}

/** 一組 radio。`disabledReason` 非空 ＝ 不可選，並在選項下方印出原因。 */
function RadioGroup({
	name,
	legend,
	value,
	options,
	onChange,
}: {
	name: string;
	legend: string;
	value: string;
	options: Array<{ value: string; label: string; disabledReason?: string }>;
	onChange: (next: string) => void;
}) {
	return (
		<fieldset className="invoice-radio-group">
			<legend>{legend}</legend>
			{options.map((option) => (
				<div key={option.value} className="invoice-radio">
					<label htmlFor={`${name}-${option.value}`}>
						<input
							id={`${name}-${option.value}`}
							type="radio"
							name={name}
							value={option.value}
							checked={value === option.value}
							disabled={Boolean(option.disabledReason)}
							onChange={() => onChange(option.value)}
						/>
						<span>{option.label}</span>
					</label>
					{option.disabledReason ? (
						<p className="invoice-radio__reason">{option.disabledReason}</p>
					) : null}
				</div>
			))}
		</fieldset>
	);
}

export default function ManualIssuancePage() {
	const router = useRouter();
	const searchParams = useSearchParams();

	const [issuerKind, setIssuerKind] = useState<string>(
		InvoiceIssuerType.SELLER,
	);
	const [issuerStoreId, setIssuerStoreId] = useState(
		searchParams.get("issuer_store_id") ?? "",
	);
	const transport = useTransport();
	const sellerAdmin = useMemo(
		() => createClient(SellerAdminService, transport),
		[transport],
	);
	const [issuerStoreName, setIssuerStoreName] = useState("");
	const [lookupNotice, setLookupNotice] = useState<GovernanceNotice | null>(
		null,
	);

	const [recipientKind, setRecipientKind] = useState<string>(
		InvoiceRecipientType.EXTERNAL,
	);
	const [recipientBuyerId, setRecipientBuyerId] = useState("");
	const [recipientStoreId, setRecipientStoreId] = useState("");
	const [recipientName, setRecipientName] = useState("");
	const [recipientTaxId, setRecipientTaxId] = useState("");
	const [recipientEmail, setRecipientEmail] = useState("");
	const [carrierType, setCarrierType] = useState("");
	const [carrierValue, setCarrierValue] = useState("");

	const [sourceKind, setSourceKind] = useState<string>(
		SOURCE_KIND_MANUAL_LINES,
	);
	const [orderNo, setOrderNo] = useState(searchParams.get("order_no") ?? "");
	const [lines, setLines] = useState<LineDraft[]>(() => [newLine()]);

	const [documentKind, setDocumentKind] = useState<string>(
		InvoiceDocumentKind.B2C,
	);
	const [reason, setReason] = useState("");

	const [dialogOpen, setDialogOpen] = useState(false);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const [result, setResult] = useState<ManualIssuanceResultData | null>(null);

	// 冪等鍵在整頁生命週期內固定，只有 9350（同鍵不同內容）才換一把。
	// 失敗重送一律沿用同一把：它落進稽核列的 request_id，換掉等於把同一個操作
	// 在稽核上斷成兩截。
	const [idempotencyKey, setIdempotencyKey] = useState(
		createInvoiceIdempotencyKey,
	);

	const draft: IssuanceDraft = {
		issuerKind,
		issuerStoreId,
		recipientKind,
		recipientBuyerId,
		recipientStoreId,
		recipientName,
		recipientTaxId,
		recipientEmail,
		carrierType,
		carrierValue,
		sourceKind,
		orderNo,
		lines,
		documentKind,
		reason,
	};

	const totals = useMemo(() => summariseLines(lines), [lines]);
	const blockers = issuanceBlockers(draft);
	const mixedTax = hasMixedTaxCategories(draft);
	const isManualLines = sourceKind === SOURCE_KIND_MANUAL_LINES;

	// 矩陣是後端契約，前端只是提早給回饋。最後一列（訂單賣家 ≠ issuer.storeId）
	// 前端判斷不了 ⇒ 9346 仍必須處理。
	const recipientOptions = Object.values(InvoiceRecipientType).map((kind) => {
		const rows = ALLOWED_COMBOS.filter(
			(c) => c.issuer === issuerKind && c.recipient === kind,
		);
		let disabledReason: string | undefined;
		if (rows.length === 0) {
			disabledReason = `${codeLabel(INVOICE_ISSUER_TYPE_LABEL, issuerKind)}不能開票給${codeLabel(INVOICE_RECIPIENT_TYPE_LABEL, kind)}。`;
		} else if (rows.every((c) => UNAVAILABLE_SOURCES.has(c.source))) {
			disabledReason = UNAVAILABLE_SOURCE_REASON;
		}
		return {
			value: kind,
			label: codeLabel(INVOICE_RECIPIENT_TYPE_LABEL, kind),
			disabledReason,
		};
	});

	const sourceOptions = [
		SOURCE_KIND_ORDER_SNAPSHOT,
		SOURCE_KIND_MANUAL_LINES,
	].map((kind) => {
		let disabledReason: string | undefined;
		if (UNAVAILABLE_SOURCES.has(kind)) {
			disabledReason = UNAVAILABLE_SOURCE_REASON;
		} else if (
			!ALLOWED_COMBOS.some(
				(c) =>
					c.issuer === issuerKind &&
					c.recipient === recipientKind &&
					c.source === kind,
			)
		) {
			disabledReason = "目前的開票人與受票人組合不適用這個品項來源。";
		}
		return {
			value: kind,
			label: SOURCE_KIND_LABEL[kind] ?? kind,
			disabledReason,
		};
	});

	/** 換 issuer 時，把不再合法的 recipient／source 一起帶到合法的一組上。 */
	const changeIssuer = (next: string) => {
		setIssuerKind(next);
		setIssuerStoreName("");
		setLookupNotice(null);
		const usable = ALLOWED_COMBOS.filter(
			(c) => c.issuer === next && !UNAVAILABLE_SOURCES.has(c.source),
		);
		if (!usable.some((c) => c.recipient === recipientKind)) {
			setRecipientKind(usable[0]?.recipient ?? InvoiceRecipientType.EXTERNAL);
		}
		setSourceKind(SOURCE_KIND_MANUAL_LINES);
	};

	const changeRecipient = (next: string) => {
		setRecipientKind(next);
		const usable = ALLOWED_COMBOS.filter(
			(c) =>
				c.issuer === issuerKind &&
				c.recipient === next &&
				!UNAVAILABLE_SOURCES.has(c.source),
		);
		setSourceKind(usable[0]?.source ?? SOURCE_KIND_MANUAL_LINES);
	};

	const lookupSeller = useMutation({
		mutationFn: async (storeId: string) => {
			const { store } = await sellerAdmin.getStore({ id: storeId });
			return store ?? null;
		},
		onSuccess: (seller) => {
			setIssuerStoreName(seller?.storeName ?? "");
			setLookupNotice(
				seller?.storeName
					? { tone: "success", text: `已確認：${seller.storeName}` }
					: {
							tone: "error",
							text: "查到這個 store_id，但沒有回傳店名；請人工確認後再送出。",
						},
			);
		},
		onError: (caught) => {
			setIssuerStoreName("");
			setLookupNotice({
				tone: "error",
				text: errorMessage(caught),
			});
		},
	});

	const submit = useMutation({
		mutationFn: async () => {
			const res = await request<InvoiceEnvelope<ManualIssuanceResultData>>(
				ISSUANCES_ENDPOINT,
				{
					method: "POST",
					headers: { "Idempotency-Key": idempotencyKey },
					body: JSON.stringify(toIssuanceRequest(draft)),
				},
			);
			return res?.data ?? null;
		},
		onSuccess: (data) => {
			setDialogOpen(false);
			setResult(data);
			setNotice(null);
		},
		onError: (caught) => {
			if (!(caught instanceof ApiClientError)) return;
			// 9347（HTTP 200，preserveData）：同一份內容已經開過了 ⇒ 導向那一份，
			// **不是顯示錯誤**。漏了這一條的症狀是「送出後看到一則像錯誤的訊息，但
			// 發票其實已經開好了」，而管理員很可能因此再送一次。
			if (caught.code === 9347) {
				const documentId = (caught.data as ManualIssuanceResultData | undefined)
					?.documentId;
				if (documentId) {
					setDialogOpen(false);
					router.push(
						`/admin/invoices/documents/${encodeURIComponent(documentId)}?from=duplicate`,
					);
					return;
				}
			}
			// 9350：同一把鍵配到不同內容 ⇒ 內容真的改了，換一把新的重來。
			// （這是本頁唯一該換鍵的情況；其餘失敗一律沿用同一把。）
			if (caught.code === 9350) {
				setIdempotencyKey(createInvoiceIdempotencyKey());
			}
		},
	});

	/** 送出。丟回 rejection 讓確認框自己就地顯示錯誤、不關閉。 */
	const handleSubmit = async () => {
		await submit.mutateAsync();
	};

	const updateLine = (key: string, patch: Partial<LineDraft>) => {
		setLines((prev) =>
			prev.map((line) => (line.key === key ? { ...line, ...patch } : line)),
		);
	};

	if (result) {
		return (
			<div>
				<div className="page-header">
					<div className="invoice-header-titles">
						<h1 className="page-title">發票已送出開立</h1>
						<div className="invoice-meta">
							<span>稽核紀錄編號：{result.auditLogId || "—"}</span>
						</div>
					</div>
				</div>
				<GovernanceNoticeBanner
					notice={{
						tone: "success",
						text: "已建立文件與開立工作，實際送往 AMEGO 由背景工作完成。理由與這次操作已寫入發票稽核紀錄。",
					}}
				/>
				<div className="card invoice-form-card">
					<div className="card-body">
						<div className="invoice-fields">
							<div>
								<div className="invoice-field__label">文件編號</div>
								<div className="invoice-field__value">
									{result.documentId || "—"}
								</div>
							</div>
							<div>
								<div className="invoice-field__label">工作編號</div>
								<div className="invoice-field__value">
									{result.jobId || "—"}
								</div>
							</div>
							<div>
								{/*
								 * `status` 在 code === 0 時是 **operation** 狀態（READY），
								 * 9347 時才是 document 狀態——而 9347 已經導頁走了，
								 * 所以這裡查的一定是 operation 表。
								 */}
								<div className="invoice-field__label">開立工作狀態</div>
								<div className="invoice-field__value">
									<StatusBadge
										variant={
											INVOICE_OPERATION_STATUS_BADGE_VARIANT[result.status] ??
											"gray"
										}
									>
										{codeLabel(INVOICE_OPERATION_STATUS_LABEL, result.status)}
									</StatusBadge>
								</div>
							</div>
						</div>
					</div>
				</div>
				<div className="adm-page-footer-actions">
					<Button
						variant="primary"
						onClick={() =>
							router.push(
								`/admin/invoices/documents/${encodeURIComponent(result.documentId)}`,
							)
						}
						disabled={!result.documentId}
					>
						查看這張發票
					</Button>
					<Button variant="outline" onClick={() => router.back()}>
						返回
					</Button>
				</div>
			</div>
		);
	}

	return (
		<div>
			<div className="page-header">
				<div className="invoice-header-main">
					<button
						type="button"
						className="btn-back"
						aria-label="返回上一頁"
						onClick={() => router.back()}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="invoice-header-titles">
						<h1 className="page-title">手動開立發票</h1>
						<div className="invoice-meta">
							<span>例外處置，不是常規動作</span>
						</div>
					</div>
				</div>
			</div>

			<div className="invoice-danger-notice" role="alert">
				<MaterialSymbol name="error" />
				<div>
					手動開票會<strong>立即產生一張真實的稅務憑證</strong>
					，開出後只能作廢或折讓，無法撤回。請先確認自動線確實無法處理這筆案件（A06
					的「安全重試」已不可用）。請勿以手動開票取代系統工作。
				</div>
			</div>

			<UnavailableOperationsNotice
				operations={[
					"沿用訂單快照補開訂單發票（受票人為買家）",
					"指定發票載具（手機條碼／自然人憑證／捐贈碼）",
				]}
			/>

			<Card
				title="開票人（issuer）"
				hint="開票人決定發票上的銷售方抬頭與統一編號，兩者都由後端在開立當下取快照。"
			>
				<RadioGroup
					name="issuer-kind"
					legend="開票人種類"
					value={issuerKind}
					onChange={changeIssuer}
					options={Object.values(InvoiceIssuerType).map((kind) => ({
						value: kind,
						label: codeLabel(INVOICE_ISSUER_TYPE_LABEL, kind),
					}))}
				/>
				{issuerKind === InvoiceIssuerType.SELLER ? (
					<div className="invoice-inline-field">
						<div className="form-group">
							<label className="form-label" htmlFor="issuer-store-id">
								賣家 store_id（必填）
							</label>
							<input
								id="issuer-store-id"
								className="form-control"
								type="text"
								inputMode="numeric"
								value={issuerStoreId}
								onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
									setIssuerStoreId(e.target.value);
									setIssuerStoreName("");
									setLookupNotice(null);
								}}
								placeholder="例：2685914682"
							/>
						</div>
						<Button
							variant="outline"
							size="sm"
							onClick={() => lookupSeller.mutate(issuerStoreId.trim())}
							disabled={issuerStoreId.trim() === ""}
							loading={lookupSeller.isPending}
						>
							查詢店名
						</Button>
						{issuerStoreName ? (
							<span className="invoice-lookup-name">{issuerStoreName}</span>
						) : null}
					</div>
				) : null}
				<GovernanceNoticeBanner notice={lookupNotice} />
			</Card>

			<Card
				title="受票人（recipient）"
				hint="統一編號、載具與捐贈碼三者互斥，只能擇一；留空代表沒有載具。"
			>
				<RadioGroup
					name="recipient-kind"
					legend="受票人種類"
					value={recipientKind}
					onChange={changeRecipient}
					options={recipientOptions}
				/>
				{/*
				 * 切換受票人種類時整組欄位會換掉 ⇒ 用 aria-live 容器包住，
				 * 否則螢幕閱讀器使用者不會知道畫面上多了或少了哪些欄位。
				 */}
				<div aria-live="polite">
					{recipientKind === InvoiceRecipientType.BUYER ? (
						<div className="form-group">
							<label className="form-label" htmlFor="recipient-buyer-id">
								買家 buyer_id（必填）
							</label>
							<input
								id="recipient-buyer-id"
								className="form-control"
								type="text"
								inputMode="numeric"
								value={recipientBuyerId}
								onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
									setRecipientBuyerId(e.target.value)
								}
							/>
						</div>
					) : null}
					{recipientKind === InvoiceRecipientType.SELLER ? (
						<div className="form-group">
							<label className="form-label" htmlFor="recipient-store-id">
								賣家 store_id（必填）
							</label>
							<input
								id="recipient-store-id"
								className="form-control"
								type="text"
								inputMode="numeric"
								value={recipientStoreId}
								onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
									setRecipientStoreId(e.target.value)
								}
							/>
						</div>
					) : null}
					{recipientKind === InvoiceRecipientType.EXTERNAL ? (
						<div className="form-group">
							<label className="form-label" htmlFor="recipient-name">
								抬頭（必填，最多 60 字）
							</label>
							<input
								id="recipient-name"
								className="form-control"
								type="text"
								maxLength={60}
								value={recipientName}
								onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
									setRecipientName(e.target.value)
								}
							/>
						</div>
					) : null}

					<div className="form-group">
						<label className="form-label" htmlFor="recipient-tax-id">
							統一編號（三聯式必填 8 位數字）
						</label>
						<input
							id="recipient-tax-id"
							className="form-control"
							type="text"
							inputMode="numeric"
							maxLength={8}
							value={recipientTaxId}
							onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
								setRecipientTaxId(e.target.value)
							}
						/>
					</div>
					<div className="form-group">
						<label className="form-label" htmlFor="recipient-email">
							通知信箱
						</label>
						<input
							id="recipient-email"
							className="form-control"
							type="email"
							value={recipientEmail}
							onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
								setRecipientEmail(e.target.value)
							}
						/>
					</div>
					{/*
					 * 載具本波不可選：後端 `buildDocument` 不寫任何 `Carrier*` 欄位，
					 * 送過去會被靜默丟掉（理由見檔頭）。選項照畫出來但整組 disabled，
					 * 這樣「為什麼不能設載具」有答案，而不是一個看起來能用、實際無效的欄位。
					 */}
					<div className="form-group">
						<label className="form-label" htmlFor="carrier-type">
							載具
						</label>
						<select
							id="carrier-type"
							className="form-control"
							value={carrierType}
							disabled
							onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
								setCarrierType(e.target.value);
								if (e.target.value === "") setCarrierValue("");
							}}
						>
							<option value="">不使用載具</option>
							{A11_CARRIER_TYPES.map(([value, label]) => (
								<option key={value} value={value}>
									{label}
								</option>
							))}
						</select>
						<p className="invoice-field__note">
							後端目前不會把手動開票的載具寫進發票，因此本波一律開立無載具發票。要指定載具請走自動線或事後由買家自行歸戶。
						</p>
					</div>
				</div>
			</Card>

			<Card title="品項來源（source）">
				<RadioGroup
					name="source-kind"
					legend="品項來源"
					value={sourceKind}
					onChange={setSourceKind}
					options={sourceOptions}
				/>
				<div aria-live="polite">
					{isManualLines ? (
						<>
							<div className="table-container">
								<table className="data-table">
									<thead>
										<tr>
											<th>種類</th>
											<th>品名</th>
											<th>數量</th>
											<th>單價</th>
											<th>稅別</th>
											<th>小計</th>
											<th>操作</th>
										</tr>
									</thead>
									<tbody>
										{lines.map((line, index) => {
											const net = lineNetAmount(line);
											return (
												<tr key={line.key}>
													<td>
														<select
															className="form-control"
															aria-label={`第 ${index + 1} 列的種類`}
															value={line.lineType}
															onChange={(
																e: React.ChangeEvent<HTMLSelectElement>,
															) =>
																updateLine(line.key, {
																	lineType: e.target.value,
																})
															}
														>
															{LINE_TYPE_OPTIONS.map(([value, label]) => (
																<option key={value} value={value}>
																	{label}
																</option>
															))}
														</select>
													</td>
													<td>
														<input
															className="form-control"
															type="text"
															maxLength={100}
															aria-label={`第 ${index + 1} 列的品名`}
															value={line.name}
															onChange={(
																e: React.ChangeEvent<HTMLInputElement>,
															) =>
																updateLine(line.key, { name: e.target.value })
															}
														/>
													</td>
													{/*
													 * `type="text" inputMode="numeric"` 不是 `type="number"`：
													 * 金額與數量一律以字串過線，`type="number"` 會在輸入當下
													 * 就把值轉成 number 而失去精度與前導字元。
													 */}
													<td>
														<input
															className="form-control"
															type="text"
															inputMode="numeric"
															aria-label={`第 ${index + 1} 列的數量`}
															value={line.quantity}
															onChange={(
																e: React.ChangeEvent<HTMLInputElement>,
															) =>
																updateLine(line.key, {
																	quantity: e.target.value,
																})
															}
														/>
													</td>
													<td>
														<input
															className="form-control"
															type="text"
															inputMode="numeric"
															aria-label={`第 ${index + 1} 列的單價`}
															value={line.unitPrice}
															onChange={(
																e: React.ChangeEvent<HTMLInputElement>,
															) =>
																updateLine(line.key, {
																	unitPrice: e.target.value,
																})
															}
														/>
													</td>
													<td>
														<select
															className="form-control"
															aria-label={`第 ${index + 1} 列的稅別`}
															value={line.taxCategory}
															onChange={(
																e: React.ChangeEvent<HTMLSelectElement>,
															) =>
																updateLine(line.key, {
																	taxCategory: e.target.value,
																})
															}
														>
															{TAX_CATEGORY_OPTIONS.map(([value, label]) => (
																<option key={value} value={value}>
																	{label}
																</option>
															))}
														</select>
													</td>
													<td>
														{net === null ? "—" : formatAmount(String(net))}
													</td>
													<td>
														{/*
														 * 原生 <button> 不是共用的 `Button`：這一顆需要
														 * `aria-label`（同一頁有 N 顆「刪除」，沒有序號的話
														 * 螢幕閱讀器讀到的是 N 個一模一樣的鍵），而 `Button`
														 * 的 props 不轉發 aria-*。
														 */}
														<button
															type="button"
															className="invoice-line-delete"
															aria-label={`刪除第 ${index + 1} 列`}
															onClick={() =>
																setLines((prev) =>
																	prev.filter((l) => l.key !== line.key),
																)
															}
														>
															<MaterialSymbol name="delete" />
														</button>
													</td>
												</tr>
											);
										})}
									</tbody>
								</table>
							</div>
							<div className="invoice-table-actions">
								<Button
									variant="outline"
									size="sm"
									onClick={() => setLines((prev) => [...prev, newLine()])}
									disabled={lines.length >= MAX_LINES}
								>
									新增一列
								</Button>
								<span className="invoice-field__note">
									{lines.length} / {MAX_LINES}{" "}
									行。折抵請用「折抵」種類並填負數單價。
								</span>
							</div>
						</>
					) : (
						<div className="form-group">
							<label className="form-label" htmlFor="source-order-no">
								訂單編號（必填）
							</label>
							<input
								id="source-order-no"
								className="form-control"
								type="text"
								value={orderNo}
								onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
									setOrderNo(e.target.value)
								}
							/>
							<p className="invoice-field__note">
								品項由後端依訂單快照取得，本頁不預覽也不計算金額。
							</p>
						</div>
					)}
				</div>
			</Card>

			<Card title="文件類型與稅別">
				<RadioGroup
					name="document-kind"
					legend="文件類型"
					value={documentKind}
					onChange={setDocumentKind}
					options={Object.values(InvoiceDocumentKind).map((kind) => ({
						value: kind,
						label: codeLabel(INVOICE_DOCUMENT_KIND_LABEL, kind),
					}))}
				/>
				<p className="invoice-field__note">
					收據只適用於已核准「免用統一發票」的賣家；三聯式必須填受票人統一編號。
				</p>
				{mixedTax ? (
					// 不擋送出：後端會 fail-closed 成 AWAITING_MANUAL（理由碼
					// B2B_MIXED_TAX_UNVERIFIED）。前端擋掉等於做了一個後端沒有承諾的規則。
					<div className="invoice-page-notice" role="status">
						<MaterialSymbol name="error" />
						<div>
							這張發票混用了多種稅別。B2B（打統編）的混合稅別在本平台會轉為人工處理，不會立即開出。
						</div>
					</div>
				) : null}
			</Card>

			{isManualLines ? (
				<Card title="金額">
					{/*
					 * 這裡刻意沒有「稅額」與「未稅金額」欄位：表頭三欄與稅額的拆分一律由
					 * 後端依 58 §5A.6 算（含尾差的最大餘數處理），前端算一份就是第二個真相。
					 */}
					<dl className="invoice-amount-summary" aria-live="polite">
						<div>
							<dt>品項小計</dt>
							<dd>{formatAmount(String(totals.product))}</dd>
						</div>
						<div>
							<dt>運費</dt>
							<dd>{formatAmount(String(totals.shipping))}</dd>
						</div>
						<div>
							<dt>折扣</dt>
							<dd>{formatAmount(String(totals.discount))}</dd>
						</div>
						<div className="invoice-amount-summary__total">
							<dt>合計</dt>
							<dd>{formatAmount(String(totals.total))}</dd>
						</div>
					</dl>
					<p className="invoice-field__note">
						以上為輸入值的合計。實際發票金額、稅額與銷售額拆分以送出後的系統計算為準。
						{totals.incomplete ? "（有欄位還沒填完，合計只是部分和。）" : ""}
					</p>
				</Card>
			) : null}

			<Card title="理由">
				<div className="form-group">
					<label className="form-label" htmlFor="issuance-reason">
						開立理由（必填）
					</label>
					<textarea
						id="issuance-reason"
						className="form-control"
						rows={3}
						maxLength={MAX_REASON_RUNES}
						value={reason}
						onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
							setReason(e.target.value)
						}
						placeholder="例：AMEGO 連線中斷期間漏開，經賣家確認後補開"
					/>
					<p className="invoice-field__note">
						理由會寫入發票稽核紀錄，最多 {MAX_REASON_RUNES} 字。
					</p>
				</div>
			</Card>

			<GovernanceNoticeBanner notice={notice} />

			{blockers.length > 0 ? (
				<div className="invoice-blockers" role="status">
					<p className="invoice-blockers__title">還不能送出：</p>
					<ul>
						{blockers.map((text) => (
							<li key={text}>{text}</li>
						))}
					</ul>
				</div>
			) : null}

			<div className="adm-page-footer-actions">
				<Button
					variant="danger"
					disabled={blockers.length > 0}
					onClick={() => {
						setNotice(null);
						setDialogOpen(true);
					}}
				>
					預覽並送出
				</Button>
				<p className="adm-page-footer-note">
					送出前會再確認一次；開出的發票只能作廢或折讓。
				</p>
			</div>

			{dialogOpen ? (
				<ManualIssuanceConfirmDialog
					draft={draft}
					totals={totals}
					onClose={() => setDialogOpen(false)}
					onSubmit={handleSubmit}
				/>
			) : null}
		</div>
	);
}
