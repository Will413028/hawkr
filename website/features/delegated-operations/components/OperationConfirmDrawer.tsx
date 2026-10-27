"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Drawer } from "@/components/shared/Drawer";
import Button from "@/components/ui/Button";
import { Textarea } from "@/components/ui/Input";
import type { DelegatedMutationMeta } from "@/lib/api/delegated";
import {
	OPERATION_REASON_MAX,
	type SellerMutationRequest,
} from "@/lib/seller-data/scope";

/**
 * 代行寫入的確認抽屜（mockup §4.6.5，L1093 的三件必備樣板）。
 *
 * 每一個代行寫入表單都經過這裡，因為三件事缺一即不能用：
 * ① `修改原因`（trim 後 1–100 字必填，帶字數計數器）—— 缺了是 9260／400，表單完全
 *    不能用，而 mockup 的商店設定頁與新增貼文頁都沒有畫這一欄
 * ② `目前版本` 露出點（`expectedVersion`）—— 版本衝突是 9266／409，操作者要看得到
 *    自己正在覆蓋哪一版
 * ③ `通知賣家` 勾選框（**預設勾選**）—— 不畫它等於這個旗標永遠只有預設值
 *
 * 「變更摘要」三欄表在**送出前**由呼叫端本地自算（比較已載入的 DTO 與表單狀態），
 * 欄位名取表單自己的標籤；送出後要顯示的是後端 `changedFields[]`，那是另一件事，
 * 不由這個元件負責。
 */

const REASON_MAX = OPERATION_REASON_MAX;

/**
 * 沒給 `reasonPlaceholder` 時的退路。刻意**不描述任何動作**：這個元件同時服務上架與
 * 下架、儲存與發布，任何帶動作的範例句在其中一半的情境裡都是相反的。
 */
const REASON_PLACEHOLDER_FALLBACK = "例：依賣家來信請求協助調整";

export interface OperationConfirmDrawerProps {
	request: SellerMutationRequest | null;
	submitting?: boolean;
	onCancel: () => void;
	onConfirm: (meta: DelegatedMutationMeta) => void;
}

export default function OperationConfirmDrawer({
	request,
	submitting = false,
	onCancel,
	onConfirm,
}: OperationConfirmDrawerProps) {
	const [reason, setReason] = React.useState("");
	const [notifySeller, setNotifySeller] = React.useState(true);
	const [touched, setTouched] = React.useState(false);

	// 每次開新的一筆寫入都從乾淨的表單開始：沿用上一次的理由會讓「複製貼上同一句」
	// 變成預設行為，而稽核列的價值正來自它逐筆不同。呼叫端把表單上已填的理由當
	// `initialReason` 帶進來時才是例外——那是同一次寫入的接續，不是沿用上一次。
	//
	// ⚠️ 相依只能是 `request` 物件本身，不能是它的欄位摘要：關掉抽屜、改掉底部條的
	// 理由、再開同一種寫入時，標題與版本都沒變，摘要當鍵會讓 effect 不重跑而沿用
	// 上一次的值。
	React.useEffect(() => {
		if (!request) return;
		setReason(request.initialReason ?? "");
		setNotifySeller(request.initialNotifySeller ?? true);
		setTouched(false);
	}, [request]);

	const trimmed = reason.trim();
	const reasonValid = trimmed.length >= 1 && trimmed.length <= REASON_MAX;

	const handleConfirm = () => {
		setTouched(true);
		if (!reasonValid || !request) return;
		onConfirm({
			...(request.expectedVersion === undefined
				? {}
				: { expectedVersion: request.expectedVersion }),
			operationReason: trimmed,
			notifySeller,
		});
	};

	return (
		<Drawer
			open={request !== null}
			onClose={() => {
				if (!submitting) onCancel();
			}}
			side="right"
			className="adm-confirm"
			title={request?.title ?? "檢視變更"}
			isDismissable={!submitting}
			footer={
				<>
					<Button
						type="button"
						variant="bordered"
						className="adm-quiet-button"
						onClick={onCancel}
						disabled={submitting}
					>
						返回修改
					</Button>
					<Button
						type="button"
						color="primary"
						onClick={handleConfirm}
						disabled={submitting || !reasonValid}
						isLoading={submitting}
						loadingLabel="正在寫入代行紀錄"
					>
						{request?.confirmLabel ?? "確認"}
					</Button>
				</>
			}
		>
			{request ? (
				<div className="flex flex-col gap-4">
					<div className="adm-confirm__intent">
						<AppIcon name="shield" size={18} className="shrink-0" />
						<div>
							<strong>{request.intent}</strong>
							{request.hint ? <span>{request.hint}</span> : null}
						</div>
					</div>

					<Textarea
						variant="bordered"
						size="lg"
						label="修改原因"
						isRequired
						value={reason}
						onChange={(e) => setReason(e.target.value.slice(0, REASON_MAX))}
						onBlur={() => setTouched(true)}
						minRows={3}
						placeholder={
							request.reasonPlaceholder ?? REASON_PLACEHOLDER_FALLBACK
						}
						description={`${trimmed.length} / ${REASON_MAX} 字`}
						errorMessage={
							touched && !reasonValid
								? `修改原因為必填，且不得超過 ${REASON_MAX} 字。`
								: undefined
						}
						isInvalid={touched && !reasonValid}
					/>

					{request.changes && request.changes.length > 0 ? (
						<section className="adm-confirm__changes">
							<h3>變更摘要</h3>
							<p>以下為本次修改的重點欄位差異：</p>
							<table>
								<thead>
									<tr>
										<th>欄位</th>
										<th>原始內容</th>
										<th>變更後</th>
									</tr>
								</thead>
								<tbody>
									{request.changes.map((change) => (
										<tr key={change.field}>
											<th scope="row">{change.field}</th>
											<td>{change.before || "—"}</td>
											<td>{change.after || "—"}</td>
										</tr>
									))}
								</tbody>
							</table>
						</section>
					) : null}

					{request.expectedVersion ? (
						<p className="adm-confirm__version">
							<span>目前版本</span>
							<strong>{request.expectedVersion}</strong>
						</p>
					) : request.versionNote ? (
						// 建立類寫入沒有版本可覆蓋。空著與「漏畫」在畫面上分不出來，而這一
						// 格正是操作者判斷「我會不會覆蓋別人」的地方。
						<p className="adm-confirm__version">
							<span>目前版本</span>
							<strong>{request.versionNote}</strong>
						</p>
					) : null}

					{request.versionedItems && request.versionedItems.length > 0 ? (
						<section className="adm-confirm__versions">
							<h3>目前版本</h3>
							<ul>
								{request.versionedItems.map((item) => (
									<li key={`${item.label}|${item.version}`}>
										<span>{item.label}</span>
										<strong>{item.version}</strong>
									</li>
								))}
							</ul>
						</section>
					) : null}

					<label className="adm-confirm__notify">
						<input
							type="checkbox"
							checked={notifySeller}
							onChange={(e) => setNotifySeller(e.target.checked)}
						/>
						<span>
							<strong>通知賣家</strong>
							<small>完成後將以站內訊息通知賣家。</small>
						</span>
					</label>

					<p className="adm-confirm__audit">
						<AppIcon name="lock" size={15} className="shrink-0" />
						<span>{request.auditNote ?? "將產生不可竄改的代行紀錄"}</span>
					</p>
				</div>
			) : null}
		</Drawer>
	);
}
