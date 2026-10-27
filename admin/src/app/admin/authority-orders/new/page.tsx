"use client";

import { useMutation } from "@connectrpc/connect-query";
import { AuthorityOrderKind } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import {
	type RequestIdSlot,
	requestIdFor,
	toLocalInput,
} from "@/app/admin/merchants/[id]/measure";
import { useInvalidateGovernance } from "@/app/admin/violation-reports/governance-queries";
import { Button } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	ADD_TARGET_TEXT,
	AUTHORITY_FAILED_TEXT,
	type AuthorityOrderForm,
	authorityOrderRequestKey,
	buildCreateAuthorityOrder,
	type TargetRowForm,
} from "../authority-actions";
import { AUTHORITY_KIND_OPTIONS, authorityKindText } from "../authority-text";
import TargetRow from "./TargetRow";
import "@/styles/pages/MerchantCertificationDetail.css";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

let rowSeq = 0;
const newRow = (): TargetRowForm => ({
	key: `row-${++rowSeq}`,
	accountId: "",
	storeId: "",
	productIds: "",
});

/**
 * 登記來函（表單頁：指名的對象有多列，對話框放不下）。收文時間預設現在；期限一開始是空的，填了才送，沒填由伺服器依種類取預設（L12），
 * 客戶端不複製這個常數。request_id 在送出時才產生，同一份內容重送沿用同一個，內容改了換新的。
 * 欄位的長度、範圍、對象的個數與組合由伺服器判斷並回已核准的句子；成功導向詳情頁。
 */
export default function NewAuthorityOrderPage() {
	const router = useRouter();
	const requestIdRef = useRef<RequestIdSlot | null>(null);
	const invalidate = useInvalidateGovernance();
	const create = useMutation(
		SellerGovernanceAdminService.method.createAuthorityOrder,
		{ onSuccess: invalidate },
	);
	const [form, setForm] = useState<AuthorityOrderForm>(() => ({
		agency: "",
		documentNo: "",
		receiveTime: toLocalInput(new Date()),
		kind: AuthorityOrderKind.UNSPECIFIED,
		summary: "",
		targets: [newRow()],
		dueTime: "",
	}));
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const edit = (patch: Partial<typeof form>) => {
		setForm((f) => ({ ...f, ...patch }));
		setError(null);
	};
	const editRow = (key: string, patch: Partial<TargetRowForm>) =>
		edit({
			targets: form.targets.map((t) =>
				t.key === key ? { ...t, ...patch } : t,
			),
		});

	const submit = async () => {
		if (submitting) return;
		setError(null);
		setSubmitting(true);
		const slot = requestIdFor(
			requestIdRef.current,
			authorityOrderRequestKey(form),
			() => crypto.randomUUID(),
		);
		requestIdRef.current = slot;
		try {
			const res = await create.mutateAsync(
				buildCreateAuthorityOrder(slot.id, form),
			);
			router.push(`/admin/authority-orders/${res.order?.id ?? ""}`);
		} catch (caught) {
			setError(connectErrorText(caught, AUTHORITY_FAILED_TEXT));
			setSubmitting(false);
		}
	};

	return (
		<div className="certification-detail-page">
			<div className="detail-header">
				<div className="header-left">
					<button
						type="button"
						className="btn-back"
						aria-label="返回列表"
						onClick={() => router.push("/admin/authority-orders")}
					>
						<MaterialSymbol name="keyboard_arrow_left" />
					</button>
					<div className="header-info">
						<div className="header-title-row">
							<h1 className="merchant-name">登記來函</h1>
						</div>
					</div>
				</div>
			</div>

			<section
				className="detail-card"
				aria-labelledby="authority-new-order-title"
			>
				<div className="section-title">
					<MaterialSymbol name="account_balance" />
					<h2 id="authority-new-order-title">來函</h2>
				</div>
				<div className="card-body">
					<label className="form-label" htmlFor="authority-new-agency">
						機關 <span className="required">*</span>
					</label>
					<input
						id="authority-new-agency"
						type="text"
						className="form-control"
						value={form.agency}
						disabled={submitting}
						onChange={(e) => edit({ agency: e.target.value })}
					/>
					<label
						className="form-label"
						htmlFor="authority-new-document-no"
						style={{ marginTop: 16 }}
					>
						文號 <span className="required">*</span>
					</label>
					<input
						id="authority-new-document-no"
						type="text"
						className="form-control"
						value={form.documentNo}
						disabled={submitting}
						onChange={(e) => edit({ documentNo: e.target.value })}
					/>
					<label
						className="form-label"
						htmlFor="authority-new-receive-time"
						style={{ marginTop: 16 }}
					>
						收文時間 <span className="required">*</span>
					</label>
					<input
						id="authority-new-receive-time"
						type="datetime-local"
						className="form-control"
						value={form.receiveTime}
						disabled={submitting}
						onChange={(e) => edit({ receiveTime: e.target.value })}
					/>
					<label
						className="form-label"
						htmlFor="authority-new-kind"
						style={{ marginTop: 16 }}
					>
						種類 <span className="required">*</span>
					</label>
					<select
						id="authority-new-kind"
						className="form-select"
						value={form.kind}
						disabled={submitting}
						onChange={(e) =>
							edit({ kind: Number(e.target.value) as AuthorityOrderKind })
						}
					>
						<option value={AuthorityOrderKind.UNSPECIFIED}>請選擇</option>
						{AUTHORITY_KIND_OPTIONS.map((kind) => (
							<option key={kind} value={kind}>
								{authorityKindText(kind)}
							</option>
						))}
					</select>
					<label
						className="form-label"
						htmlFor="authority-new-summary"
						style={{ marginTop: 16 }}
					>
						內容摘要 <span className="required">*</span>
					</label>
					<textarea
						id="authority-new-summary"
						className="form-textarea"
						rows={4}
						value={form.summary}
						disabled={submitting}
						onChange={(e) => edit({ summary: e.target.value })}
					/>
					<label
						className="form-label"
						htmlFor="authority-new-due"
						style={{ marginTop: 16 }}
					>
						期限
					</label>
					<input
						id="authority-new-due"
						type="datetime-local"
						className="form-control"
						value={form.dueTime}
						disabled={submitting}
						onChange={(e) => edit({ dueTime: e.target.value })}
					/>
					<p className="adm-operation-field-note">
						依種類預設：限制或移除商品、暫停帳號 48 小時；調取資料、其他 3
						天。來函寫了期限時，照來函填。
					</p>
				</div>
			</section>

			<section
				className="detail-card"
				aria-labelledby="authority-new-targets-title"
			>
				<div className="section-title">
					<MaterialSymbol name="storefront" />
					<h2 id="authority-new-targets-title">指名的對象</h2>
				</div>
				<div className="card-body">
					{form.targets.map((row, index) => (
						<TargetRow
							key={row.key}
							index={index}
							row={row}
							disabled={submitting}
							removable={form.targets.length > 1}
							onChange={(patch) => editRow(row.key, patch)}
							onRemove={() =>
								edit({ targets: form.targets.filter((t) => t.key !== row.key) })
							}
						/>
					))}
					<Button
						variant="outline"
						disabled={submitting}
						onClick={() => edit({ targets: [...form.targets, newRow()] })}
					>
						{ADD_TARGET_TEXT}
					</Button>
					<p className="adm-operation-field-note">
						登記之後，指名對象在平台的店家與認證資料會保存到收文後 184
						天；進入訴訟時可以延長。
					</p>
				</div>
			</section>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
			<div style={{ display: "flex", gap: 16, flexWrap: "wrap" }}>
				<Button variant="primary" onClick={submit} loading={submitting}>
					登記
				</Button>
			</div>
		</div>
	);
}
