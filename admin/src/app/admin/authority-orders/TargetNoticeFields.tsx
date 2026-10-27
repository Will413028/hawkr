"use client";

import { AuthorityTargetNoticeKind } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import type { NoticeForm } from "./authority-actions";
import {
	authorityNoticeKindText,
	MEASURE_NOTICE_CONDITION_TEXT,
} from "./authority-text";

interface TargetNoticeFieldsProps {
	/** 欄位 id 的前綴；同一個對話框裡每個對象各自一個。 */
	idPrefix: string;
	/** 選項（記下告知只能選已告知或新的暫不告知）。 */
	kinds: readonly AuthorityTargetNoticeKind[];
	value: NoticeForm;
	disabled: boolean;
	onChange: (patch: Partial<NoticeForm>) => void;
}

/** 一個對象的告知（M6）：方式，加上這個方式要填的欄位（已告知：時間、方式；暫不告知：向機關確認的日期；由措施告知：限制說明）。 */
export default function TargetNoticeFields({
	idPrefix,
	kinds,
	value,
	disabled,
	onChange,
}: TargetNoticeFieldsProps) {
	return (
		<>
			<label className="form-label" htmlFor={`${idPrefix}-kind`}>
				告知被指名者 <span className="required">*</span>
			</label>
			<select
				id={`${idPrefix}-kind`}
				className="form-select"
				value={value.kind}
				disabled={disabled}
				onChange={(e) =>
					onChange({
						kind: Number(e.target.value) as AuthorityTargetNoticeKind,
					})
				}
			>
				<option value={AuthorityTargetNoticeKind.UNSPECIFIED}>請選擇</option>
				{kinds.map((kind) => (
					<option key={kind} value={kind}>
						{authorityNoticeKindText(kind)}
					</option>
				))}
			</select>
			{value.kind === AuthorityTargetNoticeKind.NOTIFIED && (
				<>
					<label
						className="form-label"
						htmlFor={`${idPrefix}-time`}
						style={{ marginTop: 12 }}
					>
						時間
					</label>
					<input
						id={`${idPrefix}-time`}
						type="datetime-local"
						className="form-control"
						value={value.notifyTime}
						disabled={disabled}
						onChange={(e) => onChange({ notifyTime: e.target.value })}
					/>
					<label
						className="form-label"
						htmlFor={`${idPrefix}-note`}
						style={{ marginTop: 12 }}
					>
						方式
					</label>
					<input
						id={`${idPrefix}-note`}
						type="text"
						className="form-control"
						value={value.note}
						disabled={disabled}
						onChange={(e) => onChange({ note: e.target.value })}
					/>
				</>
			)}
			{value.kind === AuthorityTargetNoticeKind.WITHHELD && (
				<>
					<label
						className="form-label"
						htmlFor={`${idPrefix}-recheck`}
						style={{ marginTop: 12 }}
					>
						向機關確認的日期
					</label>
					<input
						id={`${idPrefix}-recheck`}
						type="date"
						className="form-control"
						value={value.recheckDate}
						disabled={disabled}
						onChange={(e) => onChange({ recheckDate: e.target.value })}
					/>
				</>
			)}
			{value.kind === AuthorityTargetNoticeKind.MEASURE_IS_NOTICE && (
				<p className="adm-operation-field-note">
					{MEASURE_NOTICE_CONDITION_TEXT}
				</p>
			)}
		</>
	);
}
