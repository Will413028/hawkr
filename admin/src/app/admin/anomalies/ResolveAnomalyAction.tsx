"use client";

import {
	createConnectQueryKey,
	useMutation as useConnectMutation,
} from "@connectrpc/connect-query";
import { PaymentAdminService } from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button, useToast } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import ResolveAnomalyDialog from "./ResolveAnomalyDialog";
import {
	type ResolveDraft,
	type ResolveTarget,
	resolveAnomalyRequest,
	TARGET_NOUN,
} from "./resolve-anomaly";

export const RESOLVED_TEXT = "已結案";
export const RESOLVE_FAILED_TEXT =
	"結案失敗，異常沒有變動；請確認網路狀態後再試一次。";

const LIST_KEYS = [
	createConnectQueryKey({
		schema: PaymentAdminService.method.listPayments,
		cardinality: undefined,
	}),
	createConnectQueryKey({
		schema: PaymentAdminService.method.listPaymentTransfers,
		cardinality: undefined,
	}),
	createConnectQueryKey({
		schema: PaymentAdminService.method.listPaymentRefunds,
		cardinality: undefined,
	}),
];

/**
 * 列上的「結案」入口：每個可結案的物件一顆按鈕（一筆退款可有退款、收回、補差額），開對話框收結果與證據，
 * 成功後重讀三個列表（結案會改變付款、轉帳、退款任一個的狀態）。
 */
export default function ResolveAnomalyAction({
	targets,
	displayId,
}: {
	targets: readonly ResolveTarget[];
	displayId: string;
}) {
	const queryClient = useQueryClient();
	const { showToast } = useToast();
	const mutation = useConnectMutation(
		PaymentAdminService.method.resolvePaymentAnomaly,
	);
	const [selected, setSelected] = useState<ResolveTarget | null>(null);

	if (targets.length === 0) return null;

	const confirm = async (draft: ResolveDraft) => {
		if (!selected) throw new Error("沒有選定要結案的物件");
		const result = await runAction(
			() => mutation.mutateAsync(resolveAnomalyRequest(selected, draft)),
			{ fallback: RESOLVE_FAILED_TEXT, conflictText: RESOLVE_FAILED_TEXT },
		);
		if (result.ok) {
			setSelected(null);
			showToast(RESOLVED_TEXT, "success");
			for (const queryKey of LIST_KEYS) {
				void queryClient.invalidateQueries({ queryKey });
			}
		}
		return result;
	};

	return (
		<>
			<div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
				{targets.map((t) => (
					<Button
						key={`${t.slot}:${t.id}`}
						variant="outline"
						size="sm"
						onClick={() => setSelected(t)}
					>
						{targets.length > 1 ? `結案${TARGET_NOUN[t.slot]}` : "結案"}
					</Button>
				))}
			</div>
			{selected && (
				<ResolveAnomalyDialog
					target={selected}
					displayId={displayId}
					onClose={() => setSelected(null)}
					onConfirm={confirm}
				/>
			)}
		</>
	);
}
