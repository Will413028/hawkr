"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { GovernanceNoticeBanner } from "./GovernanceNotice";
import { measureBasisText } from "./measure";
import { END_HINT_TEXT } from "./store-detail";

interface EndMeasureDialogProps {
	/** 要結束的措施（生效中、`can_end` 為真的那一筆）；結束帶它的 etag。 */
	measure: Measure;
	onClose: () => void;
	/** 送出結束；reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: () => Promise<void>;
}

/** 結束措施的確認框（只有風險依據的停權能結束）。版面沿用商家詳情既有的 modal 寫法。 */
export default function EndMeasureDialog({
	measure,
	onClose,
	onConfirm,
}: EndMeasureDialogProps) {
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const submit = async () => {
		if (submitting) return;
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm();
		} catch (caught) {
			setError(
				connectErrorText(
					caught,
					"結束措施失敗，賣家仍在停權中；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	const createdAt = measure.createTime
		? formatDateTimeShort(timestampDate(measure.createTime))
		: "-";

	return (
		<Dialog
			title="結束措施"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button variant="primary" onClick={submit} disabled={submitting}>
						{submitting ? "結束中…" : "確認結束"}
					</Button>
				</>
			}
		>
			<p className="form-label" style={{ marginBottom: 8 }}>
				當前措施
			</p>
			<div className="suspension-summary-box">
				<div>依據：{measureBasisText(measure)}</div>
				<div>給賣家的理由：{measure.statement || "-"}</div>
				<div>建立時間：{createdAt}</div>
				<div>操作者 ID：{measure.createdByAdminId || "-"}</div>
			</div>
			<p className="suspension-lift-hint">{END_HINT_TEXT}</p>
			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
