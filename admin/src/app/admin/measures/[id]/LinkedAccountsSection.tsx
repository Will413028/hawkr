"use client";

import type {
	LinkedAccount,
	Measure,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import Link from "next/link";
import { Button } from "@/components/UI";
import { linkedIdentityLabel } from "../../merchants/[id]/measure";

/**
 * 「相同身分的其他帳號」（Measure.linked_accounts，只有單筆讀取帶）：店號連結、命中的身分類別，以及已建立的關聯終止或建立關聯終止的入口
 * （入口只看伺服器給的 can_create_linked_termination）。沒有命中時不顯示這個區塊。
 */
export default function LinkedAccountsSection({
	measure,
	onCreateLinked,
}: {
	measure: Pick<Measure, "id" | "linkedAccounts">;
	/** 從某一列建立關聯終止（來源是這筆措施）。 */
	onCreateLinked: (sourceMeasureId: string, account: LinkedAccount) => void;
}) {
	if (measure.linkedAccounts.length === 0) return null;
	return (
		<section
			className="detail-card"
			aria-labelledby="measure-detail-linked-title"
		>
			<div className="section-title">
				<MaterialSymbol name="add_link" />
				<h2 id="measure-detail-linked-title">相同身分的其他帳號</h2>
			</div>
			<div className="card-body" style={{ overflowX: "auto" }}>
				<table className="tab-table">
					<thead>
						<tr>
							<th>帳號</th>
							<th>店號</th>
							<th>命中的身分類別</th>
							<th>狀態</th>
						</tr>
					</thead>
					<tbody>
						{measure.linkedAccounts.map((row) => (
							<tr key={row.accountId}>
								<td>{row.accountId}</td>
								<td>
									<Link href={`/admin/merchants/${row.storeId}`}>
										{row.storeCode || row.storeId}
									</Link>
								</td>
								<td>{linkedIdentityLabel(row.matchedIdentity)}</td>
								<td>
									{row.linkedMeasureId ? (
										<>
											已建立關聯終止（措施{" "}
											<Link href={`/admin/measures/${row.linkedMeasureId}`}>
												{row.linkedMeasureId}
											</Link>
											）
										</>
									) : row.canCreateLinkedTermination ? (
										<Button
											variant="outline"
											onClick={() => onCreateLinked(measure.id, row)}
										>
											建立關聯終止
										</Button>
									) : (
										"-"
									)}
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
		</section>
	);
}
