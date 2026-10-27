interface MaskedValueProps {
	/** 後端遮罩後的字串（例 `/TR****`、`09**-***-333`、`275`）。 */
	value: string;
	/** 這個值是什麼（例「手機條碼」），唸給讀屏聽的前綴。 */
	label: string;
	className?: string;
}

/**
 * 遮罩值的顯示（58A §B-1.0.8）。
 *
 * 兩件事：`tabular-nums` 讓不同筆的星號與數字對齊；`aria-label` 把字元逐一唸出
 * 來，否則讀屏會把 `/TR****` 唸成一個念不出來的詞。
 *
 * 🔴 這裡**不做任何遮罩**：遮罩一律後端做（§0.4 ③）。看到 DTO 給完整值是後端
 * bug，要回報而不是在前端補上——前端補遮罩會讓明碼仍然到過瀏覽器。
 */
export function MaskedValue({ value, label, className }: MaskedValueProps) {
	const spoken = [...value].join(" ");
	return (
		<span
			// role="img" ＋ aria-label：讀屏會唸 aria-label 而不是把 `/T••••+P` 當成
			// 一個念不出來的詞逐字亂讀。span 沒有隱含 role 時 aria-label 是無效的。
			role="img"
			className={["tabular-nums", className].filter(Boolean).join(" ")}
			aria-label={`${label}，${spoken}`}
		>
			{value}
		</span>
	);
}
