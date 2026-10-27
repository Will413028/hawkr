"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import AppIcon from "@/components/shared/AppIcon";

interface ConversationActionsMenuProps {
	/** 給 aria-label 用的對話名稱（沒有標題時是最後一則訊息）。 */
	label: string;
	onRename: () => void;
	onDelete: () => void;
}

/** 與 CSS 的 min-width 同值；用來把選單右緣對齊觸發鈕右緣。 */
const MENU_WIDTH = 148;
/** 距視窗邊緣的安全距離。 */
const VIEWPORT_EDGE = 12;

type MenuPosition = { left: number; top: number };

/**
 * 側欄 agent 列的「⋯」動作選單。就地彈出（錨定在觸發鈕下方），不從右側
 * 滑出 Drawer——ChatGPT／Codex 的列動作都是就地小選單，滑出面板對「改個
 * 名字」這種輕動作太重。
 *
 * a11y 語法沿用 seller-orders 那一組既有的列動作選單（`aria-haspopup`／
 * `aria-expanded`／`role="menu"`＋`role="menuitem"`、點外面與 Esc 關閉、
 * 關閉後焦點回觸發鈕）——同一個互動語意不長出第二套慣例。改名不在這裡
 * 開輸入框：選了之後由列本身進入就地編輯，選單只負責發號令。
 */
export default function ConversationActionsMenu({
	label,
	onRename,
	onDelete,
}: ConversationActionsMenuProps) {
	const [open, setOpen] = React.useState(false);
	const [position, setPosition] = React.useState<MenuPosition | null>(null);
	const rootRef = React.useRef<HTMLSpanElement | null>(null);
	const menuRef = React.useRef<HTMLDivElement | null>(null);
	const triggerRef = React.useRef<HTMLButtonElement | null>(null);
	const firstItemRef = React.useRef<HTMLButtonElement | null>(null);

	const close = React.useCallback((returnFocus: boolean) => {
		setOpen(false);
		if (returnFocus) triggerRef.current?.focus();
	}, []);

	/**
	 * 量觸發鈕的位置，用 fixed 座標開選單。
	 *
	 * 不能用 absolute：側欄是 `overflow: hidden`、列表是 `overflow-y: auto`
	 * （styles/features/messages.css），absolute 子層會被裁掉——實機上就是
	 * 只露出第一個項目、第二個被切掉。fixed 不受祖先 overflow 裁切。
	 * 右對齊用 `right` 而不是 `left`，就不必先量選單自己的寬度。
	 * 下方空間不足時往上開。
	 */
	/**
	 * 用 left／top 而不是 right／bottom：那兩個是相對「初始包含塊」的邊，
	 * 而它的寬高不一定等於 innerWidth／clientWidth（捲軸算法差 15px，實測
	 * 對不齊）。left／top 與 getBoundingClientRect 同一個原點，沒有這問題。
	 * 高度不用估——先往下開，落地後由 layout effect 依實測高度決定要不要翻上。
	 */
	const reposition = React.useCallback((): boolean => {
		const rect = triggerRef.current?.getBoundingClientRect();
		if (!rect) return false;
		// 觸發鈕已被捲出視窗 → 浮層留著只會飄在無關內容上。
		if (rect.bottom < 0 || rect.top > window.innerHeight) return false;
		setPosition({
			left: Math.max(VIEWPORT_EDGE, rect.right - MENU_WIDTH),
			top: rect.bottom + 4,
		});
		return true;
	}, []);

	// 落地後量真實高度：撐出視窗底部就翻到觸發鈕上方。只在需要時修一次，
	// 不進入「改了位置又重新判斷」的迴圈（判斷式只看會不會超出下緣）。
	React.useLayoutEffect(() => {
		if (!open || !position) return;
		const menu = menuRef.current;
		const rect = triggerRef.current?.getBoundingClientRect();
		if (!menu || !rect) return;
		const height = menu.offsetHeight;
		if (position.top + height <= window.innerHeight - VIEWPORT_EDGE) return;
		const flipped = rect.top - height - 4;
		if (flipped < VIEWPORT_EDGE) return;
		setPosition({ left: position.left, top: flipped });
	}, [open, position]);

	const openMenu = React.useCallback(() => {
		if (reposition()) setOpen(true);
	}, [reposition]);

	// 點外面／Esc 關閉。捕捉階段掛在 document 上，Link 的兄弟節點也蓋得到。
	React.useEffect(() => {
		if (!open) return;
		const onPointerDown = (event: PointerEvent) => {
			const target = event.target as Node;
			// 選單被 portal 到 body，不在 rootRef 底下——兩邊都要放行，否則
			// 點選單項時會先被判成「點外面」而關閉，item 的 onClick 收不到。
			if (rootRef.current?.contains(target)) return;
			if (menuRef.current?.contains(target)) return;
			close(false);
		};
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") {
				event.stopPropagation();
				close(true);
			}
		};
		// fixed 座標會跟捲動脫鉤，所以捲動／改變大小時重算，讓選單跟著觸發鈕。
		// 刻意不是「捲動就關掉」——開啟時把焦點送進第一個項目本身就會讓瀏覽器
		// 捲動祖先來露出焦點元素，那種寫法會讓選單開了瞬間自己關掉（實測踩到）。
		const onScrollOrResize = () => {
			if (!reposition()) close(false);
		};
		document.addEventListener("pointerdown", onPointerDown, true);
		document.addEventListener("keydown", onKeyDown, true);
		window.addEventListener("scroll", onScrollOrResize, true);
		window.addEventListener("resize", onScrollOrResize);
		return () => {
			document.removeEventListener("pointerdown", onPointerDown, true);
			document.removeEventListener("keydown", onKeyDown, true);
			window.removeEventListener("scroll", onScrollOrResize, true);
			window.removeEventListener("resize", onScrollOrResize);
		};
	}, [open, close, reposition]);

	// 開啟後把焦點送進第一個項目——鍵盤流程不停在觸發鈕上。
	React.useEffect(() => {
		if (open) firstItemRef.current?.focus();
	}, [open]);

	return (
		<span className="chat-sidebar__agent-menu" ref={rootRef}>
			<button
				type="button"
				ref={triggerRef}
				className="chat-sidebar__agent-action"
				aria-label={`管理對話「${label}」`}
				aria-haspopup="menu"
				aria-expanded={open}
				onClick={() => (open ? close(false) : openMenu())}
			>
				<AppIcon name="more" size={16} />
			</button>
			{open && position
				? createPortal(
						// Portal 到 body：側欄祖先鏈上任何 transform／filter／
						// contain 都會變成 fixed 的定位包含塊，讓 right/top 不再
						// 相對視窗（實測算出來的座標被推到視窗外）。掛在 body 上
						// 就只受初始包含塊影響，座標與 getBoundingClientRect 同一
						// 個座標系。
						<div
							ref={menuRef}
							className="chat-sidebar__agent-menu-list"
							role="menu"
							style={position}
						>
							<button
								type="button"
								role="menuitem"
								ref={firstItemRef}
								onClick={() => {
									close(false);
									onRename();
								}}
							>
								<AppIcon name="edit" size={16} aria-hidden="true" />
								重新命名
							</button>
							<button
								type="button"
								role="menuitem"
								className="chat-sidebar__agent-menu-item--danger"
								onClick={() => {
									close(false);
									onDelete();
								}}
							>
								<AppIcon name="delete" size={16} aria-hidden="true" />
								刪除對話
							</button>
						</div>,
						document.body,
					)
				: null}
		</span>
	);
}
