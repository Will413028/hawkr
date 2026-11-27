"use client";

import * as React from "react";

interface SellerProfileDescriptionProps {
	description: string;
}

/**
 * 店家簡介：預設夾兩行，超出時在第二行行尾接上「…查看更多」。
 *
 * 全文一直都在 DOM 裡（夾行是純 CSS），所以展開不再抓一次資料，爬蟲也讀得到
 * 完整簡介 —— 這是不用 `slice()` 截字串的理由。
 *
 * 行尾定位不靠 `-webkit-line-clamp`：那個省略號永遠黏在行尾，沒有位置能讓給
 * 按鈕，蓋上去就是吃掉字。改成 `max-height` 裁行 + 一個浮動佔位塊把第二行的
 * 尾巴讓出來，按鈕再絕對定位落進那個缺口，於是任何寬度、任何字數都剛好接在
 * 文字後面 —— 不需要量字元、不需要用底色蓋住下面的字。省略號因此由按鈕自己
 * 帶（`ELLIPSIS`），不是瀏覽器畫的。
 */
const ELLIPSIS = "…";

export default function SellerProfileDescription({
	description,
}: SellerProfileDescriptionProps) {
	const descId = React.useId();
	const [expanded, setExpanded] = React.useState(false);
	const [overflows, setOverflows] = React.useState(false);
	const [toggleWidth, setToggleWidth] = React.useState<number | null>(null);
	const descRef = React.useRef<HTMLParagraphElement>(null);
	const textRef = React.useRef<HTMLSpanElement>(null);
	const [toggleEl, setToggleEl] = React.useState<HTMLButtonElement | null>(
		null,
	);

	React.useEffect(() => {
		if (expanded) return;
		const el = descRef.current;
		const text = textRef.current;
		if (!el || !text) return;
		const measure = () => setOverflows(el.scrollHeight - el.clientHeight > 1);
		measure();
		const observer = new ResizeObserver(measure);
		observer.observe(text);
		return () => observer.disconnect();
	}, [expanded]);

	React.useEffect(() => {
		if (expanded || !toggleEl) return;
		const measure = () => setToggleWidth(toggleEl.offsetWidth);
		measure();
		const observer = new ResizeObserver(measure);
		observer.observe(toggleEl);
		return () => observer.disconnect();
	}, [expanded, toggleEl]);

	const showToggle = overflows || expanded;

	return (
		<div
			className="hkui-seller-profile__desc-wrap"
			data-expanded={expanded}
			style={
				toggleWidth
					? ({ "--desc-toggle-w": `${toggleWidth}px` } as React.CSSProperties)
					: undefined
			}
		>
			<p className="hkui-seller-profile__desc" id={descId} ref={descRef}>
				<span className="hkui-seller-profile__desc-text" ref={textRef}>
					{description}
				</span>
			</p>
			{showToggle && (
				<button
					type="button"
					className="hkui-seller-profile__desc-toggle"
					onClick={() => setExpanded((v) => !v)}
					aria-expanded={expanded}
					aria-controls={descId}
					ref={setToggleEl}
				>
					{expanded ? (
						"收合"
					) : (
						<>
							<span aria-hidden="true">{ELLIPSIS}</span>查看更多
						</>
					)}
				</button>
			)}
		</div>
	);
}
