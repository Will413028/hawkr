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
	// 佔位塊要讓出多寬，取決於按鈕實際渲染出來多寬（字重、字級、字型都會左右它）。
	// 量出來寫進 CSS 變數，比在樣式表裡猜一個 em 值可靠。
	const [toggleWidth, setToggleWidth] = React.useState<number | null>(null);
	const descRef = React.useRef<HTMLParagraphElement>(null);
	const textRef = React.useRef<HTMLSpanElement>(null);
	// callback ref 而非 useRef：按鈕是條件渲染的，要等它掛上才量得到，
	// 而 ref 物件換內容不會觸發 effect。
	const [toggleEl, setToggleEl] = React.useState<HTMLButtonElement | null>(
		null,
	);

	React.useEffect(() => {
		// 只在夾行狀態下量。展開後高度上限被拿掉，量到的永遠是「放得下」，
		// 會把按鈕從使用者手底下抽走。
		if (expanded) return;
		const el = descRef.current;
		const text = textRef.current;
		if (!el || !text) return;
		// 佔位塊只會把更多字擠出可視範圍，不會讓原本溢出的簡介變成放得下，
		// 所以量測結果不會在「顯示按鈕／收起按鈕」之間來回震盪。
		const measure = () => setOverflows(el.scrollHeight - el.clientHeight > 1);
		measure();
		// 觀察的是裡面那層沒被裁的文字，不是被 max-height 釘死的 <p>：<p> 的高度
		// 永遠是兩行，換行寬度變了、或換一家店換了簡介，它都不會動。
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
