"use client";

/**
 * 站外業者表單的共用欄位原語，泛型化到表單型別 `F`。
 *
 * 抽成獨立檔案的理由：建檔（`CreateProspectDialog.tsx`，Task 3）與編輯
 * （`ProspectEditForm.tsx`，Task 4）的欄位幾乎完全相同，Task 4 只多一個
 * `outreachStatus` 下拉。留在 `CreateProspectDialog.tsx` 內部又不 export 的話，
 * Task 4 只剩兩條路：逐欄位複製貼上，或臨時決定抽檔——兩者都不該是實作到
 * 一半才發現的事。`FieldSpec<F>`／`fs<F>()` 因此泛型化：`F` 換成 Task 4 的
 * form state（多一欄 `outreachStatus`），其餘原封不動就能重用。
 */

export interface FieldProps {
	id: string;
	label: string;
	required?: boolean;
	value: string;
	onChange: (value: string) => void;
	disabled: boolean;
	type?: string;
	as?: "input" | "textarea" | "select";
	options?: ReadonlyArray<{
		value: string;
		label: string;
		disabled?: boolean;
	}>;
	hint?: string;
	invalid?: boolean;
	/** 只接受數字：輸入時去掉非數字字元。 */
	numeric?: boolean;
}

/**
 * 宣告式欄位清單的共用形狀：只描述「有哪些欄位、怎麼排」，真正的
 * value／onChange 由呼叫端在渲染迴圈裡從各自的 form state 動態接上。
 *
 * 欄位刻意叫 `field` 不叫 `key`：這個物件會整個用 `{...spec}` 展開進
 * `<Field key={spec.id} {...spec} .../>`，若也叫 `key` 會在展開時悄悄蓋掉
 * 前面那個給 React 用的 `key={spec.id}`。
 */
export type FieldSpec<F> = Omit<
	FieldProps,
	"value" | "onChange" | "disabled"
> & {
	field: keyof F;
};

/** 欄位清單的建構捷徑：位置參數比具名欄位的物件字面量省版面，欄位一多纍計下來差很多。 */
export function fs<F>(
	id: string,
	field: keyof F,
	label: string,
	extra: Partial<FieldSpec<F>> = {},
): FieldSpec<F> {
	return { ...extra, id, field, label };
}

/** 表單欄位共用外殼：建檔／編輯共用同一份 label／必填星號／提示文字排版。 */
export function Field({
	id,
	label,
	required,
	value,
	onChange,
	disabled,
	type = "text",
	as = "input",
	options,
	hint,
	invalid,
	numeric,
}: FieldProps) {
	const hintId = hint ? `${id}-hint` : undefined;
	return (
		<div className="form-group">
			<label className="form-label" htmlFor={id}>
				{label}{" "}
				{required ? <span style={{ color: "var(--danger)" }}>*</span> : null}
			</label>
			{as === "textarea" ? (
				<textarea
					id={id}
					className="form-control"
					rows={3}
					value={value}
					onChange={(e) => onChange(e.target.value)}
					disabled={disabled}
				/>
			) : as === "select" ? (
				<select
					id={id}
					className="form-control"
					value={value}
					onChange={(e) => onChange(e.target.value)}
					disabled={disabled}
				>
					{options?.map((o) => (
						<option key={o.value} value={o.value} disabled={o.disabled}>
							{o.label}
						</option>
					))}
				</select>
			) : (
				<input
					id={id}
					type={type}
					className="form-control"
					required={required}
					value={value}
					inputMode={numeric ? "numeric" : undefined}
					onChange={(e) =>
						onChange(
							numeric ? e.target.value.replace(/\D/gu, "") : e.target.value,
						)
					}
					disabled={disabled}
					aria-invalid={invalid || undefined}
					aria-describedby={hintId}
				/>
			)}
			{hint ? (
				<p
					id={hintId}
					style={{
						fontSize: 12,
						marginTop: 4,
						color: invalid ? "var(--danger)" : "var(--gray-500)",
					}}
				>
					{hint}
				</p>
			) : null}
		</div>
	);
}
