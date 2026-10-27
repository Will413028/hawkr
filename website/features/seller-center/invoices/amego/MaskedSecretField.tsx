"use client";

/**
 * N1｜已設定的祕密值（AMEGO AppKey）的顯示與更換。
 *
 * 🔴 **已設定時用 `<span>` 而不是唯讀 `<input>`**：密碼管理器會把 `<input>` 的內容
 * 當成可儲存的密碼，而這裡顯示的只是遮罩後的尾 4 碼——存起來的是垃圾，而且會在
 * 賣家下次登入光貿後台時被自動填入。按下「更新」才切成真的 `<input>`。
 *
 * 🔴 **沒有「顯示完整內容」的按鈕**，因為前端根本拿不到：AppKey 以 AES-GCM 加密
 * 存於後端，解密只發生在 AMEGO client 的呼叫路徑上，沒有任何 RPC 回傳明文。
 * 這不是遮罩，是根本不讀。
 */

import * as React from "react";
import Button from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";

export function MaskedSecretField({
	last4,
	updatedAtText,
	editing,
	onStartEdit,
	onCancelEdit,
	value,
	onChange,
	disabled,
	errorMessage,
	label,
	description,
	inputId,
}: {
	last4: string;
	updatedAtText: string;
	/** 由呼叫端持有：狀態機（`credentialEditable`）也會強制進入編輯態。 */
	editing: boolean;
	onStartEdit: () => void;
	onCancelEdit: () => void;
	value: string;
	onChange: (v: string) => void;
	disabled?: boolean;
	errorMessage?: string;
	label: string;
	description: string;
	inputId: string;
}) {
	if (editing) {
		return (
			<div className="flex flex-col gap-2">
				<Input
					id={inputId}
					type="password"
					autoComplete="off"
					spellCheck={false}
					maxLength={128}
					label={label}
					description={description}
					value={value}
					isInvalid={Boolean(errorMessage)}
					errorMessage={errorMessage}
					disabled={disabled}
					onChange={(e) => onChange(e.target.value)}
				/>
				{last4 ? (
					<div>
						<Button variant="light" size="sm" onClick={onCancelEdit}>
							取消更新，沿用原本的 AppKey
						</Button>
					</div>
				) : null}
			</div>
		);
	}

	return (
		<div className="flex items-center gap-3">
			<div className="flex-1 min-w-0 flex flex-col gap-[2px]">
				<span className="text-[0.625rem] font-extrabold text-hk-accent tracking-[0.05em]">
					{label}
				</span>
				{/* 🔴 span 不是 input —— 見檔頭。 */}
				<span className="text-xs font-extrabold text-foreground font-mono">
					{last4 ? `已設定 ••••••••${last4}` : "尚未設定"}
				</span>
				{updatedAtText ? (
					<span className="text-[0.625rem] text-foreground/60">
						更新於 {updatedAtText}
					</span>
				) : null}
			</div>
			<Button
				variant="bordered"
				size="sm"
				onClick={onStartEdit}
				disabled={disabled}
			>
				更新 AppKey
			</Button>
		</div>
	);
}
