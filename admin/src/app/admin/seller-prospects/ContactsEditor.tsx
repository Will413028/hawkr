"use client";

import type { ContactKind } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { Button } from "@/components/UI";
import {
	addContact,
	CONTACT_KIND_OPTIONS,
	type ContactRow,
	changeContactKind,
	contactKindText,
	editContactValue,
	newContactKey,
	removeContact,
	setPrimaryContact,
} from "@/lib/seller-prospects";

/**
 * 聯絡方式的編輯器：每列一個類型與值，同一種類型有多個時選一個主要的（規則在 `@/lib/seller-prospects`，
 * 這裡只接事件）。
 */
export default function ContactsEditor({
	idPrefix,
	rows,
	onChange,
	disabled,
}: {
	idPrefix: string;
	rows: readonly ContactRow[];
	onChange: (rows: ContactRow[]) => void;
	disabled: boolean;
}) {
	return (
		<fieldset
			className="form-group"
			style={{ border: 0, padding: 0, margin: 0 }}
		>
			<legend className="form-label">聯絡方式</legend>
			<p style={{ fontSize: 12, marginBottom: 8, color: "var(--gray-500)" }}>
				同一種類型有多個時選一個主要的：主要的電子郵件用來寄詢價邀請，主要的網站給採購搜尋。
			</p>
			{rows.length === 0 ? (
				<p style={{ fontSize: 13, color: "var(--gray-600)" }}>
					尚未新增聯絡方式
				</p>
			) : (
				rows.map((row) => (
					<div
						key={row.key}
						style={{
							display: "flex",
							flexWrap: "wrap",
							alignItems: "center",
							gap: 8,
							marginBottom: 8,
						}}
					>
						<select
							id={`${idPrefix}-${row.key}-kind`}
							aria-label="類型"
							className="form-control"
							style={{ flex: "0 0 140px", minHeight: 44 }}
							value={row.kind}
							disabled={disabled}
							onChange={(e) =>
								onChange(
									changeContactKind(
										rows,
										row.key,
										Number(e.target.value) as ContactKind,
									),
								)
							}
						>
							{CONTACT_KIND_OPTIONS.map((kind) => (
								<option key={kind} value={kind}>
									{contactKindText(kind)}
								</option>
							))}
						</select>
						<input
							id={`${idPrefix}-${row.key}-value`}
							type="text"
							aria-label={contactKindText(row.kind)}
							className="form-control"
							style={{ flex: "1 1 200px", minHeight: 44 }}
							value={row.value}
							disabled={disabled}
							onChange={(e) =>
								onChange(editContactValue(rows, row.key, e.target.value))
							}
						/>
						<label
							style={{
								display: "inline-flex",
								alignItems: "center",
								gap: 4,
								minHeight: 44,
							}}
						>
							<input
								type="radio"
								name={`${idPrefix}-primary-${row.kind}`}
								checked={row.primary}
								disabled={disabled}
								onChange={() => onChange(setPrimaryContact(rows, row.key))}
								style={{ width: "auto", minWidth: 0 }}
							/>
							主要
						</label>
						<Button
							variant="outline"
							size="sm"
							type="button"
							disabled={disabled}
							onClick={() => onChange(removeContact(rows, row.key))}
						>
							移除
						</Button>
					</div>
				))
			)}
			<Button
				variant="outline"
				size="sm"
				type="button"
				disabled={disabled}
				icon={<MaterialSymbol name="add" />}
				onClick={() => onChange(addContact(rows, newContactKey()))}
			>
				新增聯絡方式
			</Button>
		</fieldset>
	);
}
