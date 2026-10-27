import type { Store } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import Link from "next/link";
import { storeFoodRows } from "@/lib/food-registration";

/** 商家詳情的食品業者登錄：核准的字號與待審（或被退回）的登錄連到審核頁；兩者都沒有時不畫。 */
export function FoodRegistrationCard({ store }: { store: Store }) {
	const rows = storeFoodRows(store);
	if (rows.length === 0) return null;
	return (
		<div className="detail-card">
			<div className="section-title">
				<MaterialSymbol name="verified_user" />
				<span>食品業者登錄</span>
			</div>
			<div className="card-body">
				<div className="info-grid">
					{rows.map((row) => (
						<div className="info-field" key={row.label}>
							<div className="field-label">{row.label}</div>
							<div className="field-value">
								{row.registrationId ? (
									<Link
										href={`/admin/food-registrations/${row.registrationId}`}
									>
										{row.value}
									</Link>
								) : (
									row.value
								)}
							</div>
						</div>
					))}
				</div>
			</div>
		</div>
	);
}
