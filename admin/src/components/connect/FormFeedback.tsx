import type React from "react";

/** 欄位下方的錯誤文字（伺服器的 BadRequest 或表單自己的檢查）。沒有文字時不佔位置。 */
export function FieldErrorText({ message }: { message?: string }) {
	if (!message) return null;
	return (
		<p
			role="alert"
			className="text-[14px] leading-[20px] text-[var(--hk-danger)]"
		>
			{message}
		</p>
	);
}

/**
 * 版本衝突：別人先改過這筆資料。保留使用者的輸入，列出最新版本讓使用者對照，再決定以自己的內容送出（帶最新的
 * etag）或改用最新版本。
 */
export function VersionConflictNotice({
	title,
	children,
	onOverwrite,
	onUseLatest,
	busy,
}: {
	title: string;
	children: React.ReactNode;
	onOverwrite: () => void;
	onUseLatest: () => void;
	busy?: boolean;
}) {
	return (
		<section
			role="alert"
			aria-label={title}
			className="flex flex-col gap-[12px] rounded-[8px] border border-[var(--hk-danger)] bg-[var(--hk-danger-soft)] p-[16px]"
		>
			<h3 className="text-[16px] font-bold text-[var(--hk-danger)]">{title}</h3>
			<p className="text-[14px] leading-[22px] text-[var(--brand-gray1-100)]">
				你的修改還在表單裡，沒有送出。以下是目前的最新版本，對照後選擇要以你的內容覆蓋，或改用最新版本。
			</p>
			<dl className="grid grid-cols-[auto_1fr] gap-x-[16px] gap-y-[6px] text-[14px] text-[var(--brand-gray1-100)]">
				{children}
			</dl>
			<div className="flex justify-end gap-[12px]">
				<button
					type="button"
					onClick={onUseLatest}
					disabled={busy}
					className="h-[36px] rounded-[8px] border border-[var(--brand-gray1-50)] px-[12px] text-[14px] text-[var(--brand-gray1-100)] disabled:opacity-50"
				>
					改用最新版本
				</button>
				<button
					type="button"
					onClick={onOverwrite}
					disabled={busy}
					className="h-[36px] rounded-[8px] bg-[var(--color)] px-[12px] text-[14px] font-bold text-white disabled:opacity-50"
				>
					以我的內容送出
				</button>
			</div>
		</section>
	);
}

/** 對照面板的一列。 */
export function ConflictRow({
	label,
	value,
}: {
	label: string;
	value: React.ReactNode;
}) {
	return (
		<>
			<dt className="font-bold">{label}</dt>
			<dd>{value}</dd>
		</>
	);
}
