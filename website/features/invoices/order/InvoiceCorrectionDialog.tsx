"use client";

/**
 * B06｜買家申請修改發票資料（端點 11）。
 *
 * 🔴 **`handlingMode` 由後端決定**（開票前直接改／開票後作廢重開），前端不得依
 * 「有沒有發票號碼」自己推導——判準在 §7.3.6 的狀態機裡，複製一份必然漂移。
 * 送出後把後端回的 `statusNotice` 原樣顯示，不重寫。
 *
 * 🔴 **`expectedSnapshotVersion` 取內容軸的 `snapshotVersion`**，不是實體軸的
 * `issue_attempt_seq`：作廢重開會讓後者 +1 而前者不變，拿錯會把正常情況誤判成衝突。
 *
 * ⚠️ 三種新資料互斥（統編／捐贈／常用資料），與結帳時的 `chk_ois_*` 同一組約束。
 * 這裡只提供「改成公司用發票」與「改用既有的常用發票資料」兩條——**捐贈碼不提供**：
 * 開立後改捐贈等於把一張已經給了買受人的發票轉給機關，那是作廢重開的另一種情境，
 * 而端點 11 的 `newInvoice` 沒有 donationCode 欄位。
 */

import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { InvoiceCarrierType } from "@/lib/codemeta/generated";
import { useInvoiceProfiles } from "@/lib/hooks/useInvoiceProfiles";
import type { InvoiceCorrectionInput } from "@/lib/invoices/types";

const REASON_MAX = 200;

export function InvoiceCorrectionDialog({
	open,
	onClose,
	onSubmit,
	submitting,
	errorMessage,
	resultNotice,
}: {
	open: boolean;
	onClose: () => void;
	onSubmit: (input: InvoiceCorrectionInput, reason: string) => void;
	submitting: boolean;
	errorMessage: string;
	/** 後端回的 `statusNotice`；有值就切成結果態，**原樣顯示不重寫**。 */
	resultNotice: string;
}) {
	const { profiles } = useInvoiceProfiles();
	const [mode, setMode] = React.useState<"taxId" | "profile">("taxId");
	const [taxId, setTaxId] = React.useState("");
	const [title, setTitle] = React.useState("");
	const [profileId, setProfileId] = React.useState("");
	const [reason, setReason] = React.useState("");

	React.useEffect(() => {
		if (open) {
			setMode("taxId");
			setTaxId("");
			setTitle("");
			setProfileId("");
			setReason("");
		}
	}, [open]);

	const valid =
		mode === "taxId"
			? /^\d{8}$/.test(taxId) && title.trim().length > 0
			: profileId !== "";

	if (resultNotice) {
		return (
			<Dialog
				open={open}
				onClose={onClose}
				size="sm"
				title="修改申請已送出"
				footer={
					<div className="flex justify-end">
						<Button variant="solid" size="sm" onClick={onClose}>
							知道了
						</Button>
					</div>
				}
			>
				{/* 後端的人話 ＋ 下一步，原樣顯示。 */}
				<p className="text-xs leading-relaxed text-foreground">
					{resultNotice}
				</p>
			</Dialog>
		);
	}

	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="sm"
			title="修改發票資料"
			description="送出後由賣家與系統處理；已開立的發票會作廢後重新開立。"
			footer={
				<div className="flex justify-end gap-2">
					<Button
						variant="light"
						size="sm"
						onClick={onClose}
						disabled={submitting}
					>
						取消
					</Button>
					<Button
						variant="solid"
						size="sm"
						isLoading={submitting}
						loadingLabel="送出中…"
						disabled={!valid}
						onClick={() =>
							onSubmit(
								mode === "taxId"
									? {
											carrierType: InvoiceCarrierType.TAX_ID,
											buyerTaxId: taxId,
											buyerName: title.trim(),
										}
									: { profileId },
								reason.trim(),
							)
						}
					>
						送出申請
					</Button>
				</div>
			}
		>
			<div className="flex flex-col gap-3">
				<div
					className="flex flex-col gap-2"
					role="radiogroup"
					aria-label="修改方式"
				>
					<label className="flex items-center gap-3 p-[10px_12px] bg-content2 border border-divider rounded-hkui-lg cursor-pointer">
						<input
							type="radio"
							name="correction-mode"
							checked={mode === "taxId"}
							onChange={() => setMode("taxId")}
						/>
						<span className="text-xs font-extrabold text-foreground">
							改開公司用發票（統編）
						</span>
					</label>
					<label className="flex items-center gap-3 p-[10px_12px] bg-content2 border border-divider rounded-hkui-lg cursor-pointer">
						<input
							type="radio"
							name="correction-mode"
							checked={mode === "profile"}
							disabled={profiles.length === 0}
							onChange={() => setMode("profile")}
						/>
						<span className="text-xs font-extrabold text-foreground">
							{profiles.length > 0
								? "改用我的常用發票資料"
								: "改用我的常用發票資料（尚未建立）"}
						</span>
					</label>
				</div>

				{mode === "taxId" ? (
					<>
						<Input
							label="統一編號"
							inputMode="numeric"
							maxLength={8}
							value={taxId}
							isInvalid={taxId !== "" && !/^\d{8}$/.test(taxId)}
							errorMessage={
								taxId !== "" && !/^\d{8}$/.test(taxId)
									? "統一編號是 8 位數字。"
									: undefined
							}
							onChange={(e) => setTaxId(e.target.value.replace(/\D/g, ""))}
						/>
						<Input
							label="公司抬頭"
							maxLength={60}
							value={title}
							onChange={(e) => setTitle(e.target.value)}
						/>
					</>
				) : (
					<div
						className="flex flex-col gap-2"
						role="radiogroup"
						aria-label="常用發票資料"
					>
						{profiles.map((p) => (
							<label
								key={p.profileId}
								className="flex items-center gap-3 p-[10px_12px] bg-content2 border border-divider rounded-hkui-lg cursor-pointer"
							>
								<input
									type="radio"
									name="correction-profile"
									checked={profileId === p.profileId}
									onChange={() => setProfileId(p.profileId)}
								/>
								<span className="text-xs font-extrabold text-foreground truncate">
									{p.displayValue}
								</span>
							</label>
						))}
					</div>
				)}

				<Input
					label="修改原因（選填）"
					maxLength={REASON_MAX}
					value={reason}
					onChange={(e) => setReason(e.target.value)}
				/>

				{errorMessage ? (
					<p
						role="alert"
						className="text-[0.6875rem] leading-relaxed text-danger-700"
					>
						{errorMessage}
					</p>
				) : null}
			</div>
		</Dialog>
	);
}
