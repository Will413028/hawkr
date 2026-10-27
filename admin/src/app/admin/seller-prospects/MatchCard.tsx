"use client";

import {
	type ProspectMatch,
	ProspectMatchTier,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { StatusBadge } from "@/components/UI";
import { matchReasonsText, matchTierText } from "@/lib/seller-prospects";

/** T1 用 danger 底色讓它比 T2 更醒目：統編相同幾乎確定是同一家業者。 */
export function matchCardStyle(isT1: boolean) {
	return {
		display: "flex",
		alignItems: "center",
		gap: 10,
		minHeight: 44,
		padding: "10px 14px",
		marginBottom: 8,
		borderRadius: 8,
		textDecoration: "none",
		color: "inherit",
		border: `1px solid var(--${isT1 ? "danger-light" : "gray-300"})`,
		background: `var(--${isT1 ? "danger-bg" : "white"})`,
	} as const;
}

/**
 * 一張查重命中卡片：層級與命中原因都要顯示，只給一個等於要管理員在沒有證據的情況下判斷是不是同一家。
 * 新分頁開，避免弄丟正在填的建立表單。
 */
export function MatchCard({ match }: { match: ProspectMatch }) {
	const isT1 = match.tier === ProspectMatchTier.T1_TAX_ID;
	const prospect = match.prospect;
	return (
		<a
			href={`/admin/seller-prospects/${prospect?.id ?? ""}`}
			target="_blank"
			rel="noopener noreferrer"
			style={matchCardStyle(isT1)}
		>
			<StatusBadge variant={isT1 ? "danger" : "warning"} size="sm">
				{matchTierText(match.tier)}
			</StatusBadge>
			<span style={{ fontSize: 13, color: "var(--gray-700)" }}>
				{matchReasonsText(match.matchedOn)}
			</span>
			<span style={{ marginLeft: "auto", fontWeight: 600 }}>
				{prospect?.displayName || "（未命名）"}
				<MaterialSymbol name="open_in_new" style={{ marginLeft: 4 }} />
			</span>
		</a>
	);
}
