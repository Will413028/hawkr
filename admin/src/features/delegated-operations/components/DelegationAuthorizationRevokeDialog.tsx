"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import {
	DELEGATION_REASON_MAX_LENGTH,
	type DelegationAuthorization,
} from "@/features/delegated-operations/authorization";
import { delegationCapabilityLabel } from "@/features/delegated-operations/labels";
import {
	createDelegatedOperationIdempotencyKey,
	delegationAuthorizationErrorText,
	revokeDelegationAuthorization,
} from "@/services/delegated-operations";

/**
 * 撤回一家店的代行授權。
 *
 * 撤回是治理動作，不是「暫停」：三欄（`revokedAt`／`revokedByAdminId`／
 * `revokeReason`）同生同滅，少一個就是一筆解釋不了的撤回。
 *
 * ⚠️ 連帶影響必須在按下之前講清楚：後端在**同一個交易內**中止該店所有進行中的代行
 * 工作階段。把「還有 N 個人正在代行這家店」留到事後才顯示，等於讓人在不知情下把
 * 別人的作業中途切斷。筆數取自單店詳情端點的 `activeSessionCount`——它與撤回看到的
 * 是同一個資料來源；另打工作階段列表去數，看到的不會是同一個瞬間。
 *
 * ⚠️ 撤回失敗時**不換新的冪等鍵**：撤回可能已經生效而回應沒送到，換一把鍵會讓後端
 * 把重試當成第二次撤回。
 */

interface DelegationAuthorizationRevokeDialogProps {
	storeId: string;
	storeName: string;
	authorization: DelegationAuthorization;
	activeSessionCount: number;
	onClose: () => void;
	onRevoked: (message: string) => void;
}

export default function DelegationAuthorizationRevokeDialog({
	storeId,
	storeName,
	authorization,
	activeSessionCount,
	onClose,
	onRevoked,
}: DelegationAuthorizationRevokeDialogProps) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [idempotencyKey] = useState(() => {
		try {
			return createDelegatedOperationIdempotencyKey();
		} catch {
			return "";
		}
	});

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		const trimmed = reason.trim();
		if (trimmed.length === 0) {
			setError("請填寫撤回理由，它會寫入授權變更歷程。");
			return;
		}
		if (!idempotencyKey) {
			setError("瀏覽器無法建立安全的操作識別碼，請更新瀏覽器後再試。");
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			const result = await revokeDelegationAuthorization({
				storeId,
				revokeReason: trimmed,
				idempotencyKey,
			});
			// 已撤回是冪等成功，不是失敗；但也不能宣稱這一次中止了工作階段。
			// 實際中止筆數以回應的 `revokedSessionIds` 為準，不是按下之前查到的數字
			//（中間可能有人自行結束）。
			onRevoked(
				result.alreadyRevoked
					? "這家店的代行授權先前已被撤回，本次沒有變更。"
					: result.revokedSessionIds.length > 0
						? `已撤回代行授權，並同時中止 ${result.revokedSessionIds.length} 個進行中的代行工作階段。`
						: "已撤回代行授權。",
			);
		} catch (caught) {
			setError(
				delegationAuthorizationErrorText(
					caught,
					"撤回失敗，請重試；重試會沿用同一次請求。",
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="撤回這家店的代行授權？"
			titleIcon="gpp_bad"
			description="撤回即時生效：這家店之後無法再建立代行工作階段。要恢復必須重新開通，並重新填寫授權依據。"
			closeLabel="關閉撤回代行授權視窗"
			onClose={onClose}
			busy={submitting}
			form={{ onSubmit: handleSubmit }}
			footer={
				<>
					<Button
						variant="ghost"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						返回
					</Button>
					<Button variant="danger" type="submit" loading={submitting}>
						{submitting ? "正在撤回" : "撤回授權"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>授權對象</span>
				<strong>{storeName}</strong>
				<span>Store ID · {storeId}</span>
				<span>
					目前範圍 ·{" "}
					{authorization.grantedCapabilities
						.map(delegationCapabilityLabel)
						.join("、")}
				</span>
			</div>

			<div
				className={`adm-authorization-impact${activeSessionCount > 0 ? " is-critical" : ""}`}
				role={activeSessionCount > 0 ? "alert" : undefined}
			>
				<MaterialSymbol
					name={activeSessionCount > 0 ? "crisis_alert" : "info"}
				/>
				<div>
					{activeSessionCount > 0 ? (
						<>
							<strong>
								此操作將同時中止該店 {activeSessionCount} 個進行中的代行工作階段
							</strong>
							<p>
								中止與撤回在同一個交易內完成。正在代行的管理員會在下一次操作時被拒絕，未儲存的內容不會保留。
							</p>
						</>
					) : (
						<>
							<strong>該店目前沒有進行中的代行工作階段</strong>
							<p>撤回只會停止之後的代行；筆數以送出當下的伺服器狀態為準。</p>
						</>
					)}
				</div>
			</div>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-authorization-revoke-reason">
					撤回理由（必填）
				</label>
				<textarea
					id="adm-authorization-revoke-reason"
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					maxLength={DELEGATION_REASON_MAX_LENGTH}
					rows={3}
					placeholder="例如：合約已終止，停止一切代行"
					required
				/>
				<span className="adm-operation-field-note">
					理由會寫入授權變更歷程，最多 200 字。
				</span>
			</div>

			{error && (
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>{error}</div>
				</div>
			)}
		</Dialog>
	);
}
