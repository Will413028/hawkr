"use client";

import { Code } from "@connectrpc/connect";
import { useMutation } from "@connectrpc/connect-query";
import {
	type ProspectDuplicate,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import { useState } from "react";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog, StatusBadge, useToast } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import { matchReasonsText } from "@/lib/seller-prospects";
import { useProspectCache } from "../prospect-queries";
import "@/styles/pages/DelegatedOperations.css";

/**
 * 疑似重複的一對標記為「不是同一家」的確認框。`MarkProspectsDistinct` 不帶 etag（決定的是身分、不是內容）且冪等。
 * 成功或失敗後清單都失效重讀：有一筆不再是進行中（failed_precondition）或不存在（not_found）時，伺服器的句子顯示在框內，
 * 清單重讀後那一對由伺服器決定是否還在。
 */

const TEXT = {
	title: "確定這兩筆不是同一家嗎？",
	body: "標記後這一對不會再出現在疑似重複清單，之後修改或還原也不會再因為它們相符而提醒。標記無法在畫面上取消，也不會再提示這兩筆可以合併，請確認後再送出。",
	confirm: "確認不是同一家",
	success: "已標記為不是同一家",
} as const;

export default function MarkDistinctDialog({
	pair,
	onClose,
}: {
	pair: ProspectDuplicate;
	onClose: () => void;
}) {
	const { showToast } = useToast();
	const cache = useProspectCache();
	const mark = useMutation(
		SellerProspectAdminService.method.markProspectsDistinct,
	);
	const [submitting, setSubmitting] = useState(false);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);

	const handleConfirm = async () => {
		if (submitting || !pair.left || !pair.right) return;
		setNotice(null);
		setSubmitting(true);
		try {
			const fallback = defaultErrorTextZhTW(Code.Unavailable);
			const result = await runAction(
				() =>
					mark.mutateAsync({
						firstProspectId: pair.left?.id,
						secondProspectId: pair.right?.id,
					}),
				{ fallback, conflictText: fallback },
			);
			cache.written();
			if (result.ok) {
				showToast(TEXT.success, "success");
				onClose();
			} else {
				setNotice({ tone: "error", text: result.text });
			}
		} finally {
			setSubmitting(false);
		}
	};

	const descId = "adm-prospect-distinct-description";
	const leftName = pair.left?.displayName || "（未命名）";
	const rightName = pair.right?.displayName || "（未命名）";

	return (
		<Dialog
			title={TEXT.title}
			describedBy={descId}
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button
						variant="outline"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						取消
					</Button>
					<Button
						variant="primary"
						type="button"
						onClick={() => void handleConfirm()}
						loading={submitting}
					>
						{TEXT.confirm}
					</Button>
				</>
			}
		>
			<div className="adm-operation-identity-grid">
				<div className="adm-operation-identity-card">
					<strong>{leftName}</strong>
					<span>統一編號 · {pair.left?.taxId || "未填寫"}</span>
				</div>
				<div className="adm-operation-identity-card">
					<strong>{rightName}</strong>
					<span>統一編號 · {pair.right?.taxId || "未填寫"}</span>
				</div>
			</div>
			<p className="adm-operation-field-note">
				比對依據 ·{" "}
				<StatusBadge variant="warning" size="sm">
					{matchReasonsText(pair.matchedOn)}
				</StatusBadge>
			</p>
			<p id={descId} className="adm-operation-field-note">
				{TEXT.body}
			</p>
			<GovernanceNoticeBanner notice={notice} />
		</Dialog>
	);
}
