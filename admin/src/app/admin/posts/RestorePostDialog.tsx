"use client";

import { useMutation, useQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import RevokeMeasureDialog from "../measures/[id]/RevokeMeasureDialog";
import { buildRevoke } from "../measures/measure-actions";

interface Props {
	/** 下架這則貼文的內容下架措施。 */
	measureId: string;
	onClose: () => void;
	onRestored: () => void;
}

/**
 * 恢復貼文：撤銷下架它的措施，沒有第二個入口（社群模組沒有平台隱藏貼文）。先讀措施的現況與 etag，
 * 再用既有的撤銷對話框，撤銷原因只能是手動原因。
 */
export default function RestorePostDialog({
	measureId,
	onClose,
	onRestored,
}: Props) {
	const G = SellerGovernanceAdminService.method;
	const read = useQuery(
		G.getMeasure,
		{ id: measureId },
		{ staleTime: 0, refetchOnMount: "always" },
	);
	const revoke = useMutation(G.revokeMeasure);
	const measure = read.data?.measure;

	if (!measure) {
		return (
			<Dialog title="恢復貼文" closeLabel="關閉" onClose={onClose}>
				{read.isError ? (
					<div className="adm-operation-error" role="alert">
						<MaterialSymbol name="error" />
						<div>{connectErrorText(read.error, "措施讀取失敗")}</div>
						<Button variant="outline" onClick={() => void read.refetch()}>
							重試
						</Button>
					</div>
				) : (
					<p role="status">載入中...</p>
				)}
			</Dialog>
		);
	}
	return (
		<RevokeMeasureDialog
			measure={measure}
			onClose={onClose}
			onConfirm={async (reason, revokeFindings) => {
				await revoke.mutateAsync(buildRevoke(measure, reason, revokeFindings));
				onRestored();
			}}
		/>
	);
}
