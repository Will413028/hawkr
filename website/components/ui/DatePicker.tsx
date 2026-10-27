"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { wrapTabFocus } from "@/lib/a11y/focusable";
import { setInertOutside } from "@/lib/a11y/inert";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { cn } from "@/lib/utils/cn";
import { CalendarMonth, type WeekStartsOn } from "./CalendarMonth";
import { Input } from "./Input";
import { CONTROL_RESET, inputWrapperClasses } from "./inputWrapperClasses";

/**
 * HeroUI-inspired single-component date / date-time picker. Visually
 * matches the `Input` family (bordered / floating-label / sm-md-lg) so
 * it can drop into the same grid cell as a text field without
 * re-styling.
 *
 *   <DatePicker
 *     label="生日"
 *     variant="bordered"
 *     size="lg"
 *     value={value}            // "YYYY-MM-DD" (date-time: "YYYY-MM-DDTHH:MM")
 *     onChange={setValue}
 *     minDate="2026-01-01"
 *   />
 *
 * The wire shape is the same string format browser `<input type="date">`
 * and `<input type="datetime-local">` emit — so existing form
 * controllers that previously consumed `e.target.value` continue to
 * work after the swap with no parsing changes.
 */

type DateGranularity = "date" | "date-time";
type DatePickerVariant = "flat" | "bordered" | "faded" | "underlined";
type DatePickerSize = "sm" | "md" | "lg";
type DateYearMode = "gregorian" | "minguo";

interface DatePickerProps {
	value?: string;
	onChange?: (value: string) => void;
	granularity?: DateGranularity;
	yearMode?: DateYearMode;
	label?: React.ReactNode;
	description?: React.ReactNode;
	errorMessage?: React.ReactNode;
	placeholder?: string;
	minDate?: string;
	maxDate?: string;
	variant?: DatePickerVariant;
	size?: DatePickerSize;
	floatingLabel?: boolean;
	isRequired?: boolean;
	isInvalid?: boolean;
	disabled?: boolean;
	id?: string;
	name?: string;
	className?: string;
	locale?: string;
	weekStartsOn?: WeekStartsOn;
}

const TAIWAN_WEEKDAYS = ["日", "一", "二", "三", "四", "五", "六"];

/** 手動輸入解析失敗時的提示。舊版是靜默把使用者打的字換回舊值。 */
const MANUAL_FORMAT_ERROR = "格式不符，請用 YYYY/MM/DD 或民國年 YYY/MM/DD";
const MANUAL_RANGE_ERROR = "日期超出可選範圍";

function parseDate(value: string | undefined): Date | undefined {
	if (!value) return undefined;
	const datePart = value.split("T")[0];
	const [y, m, d] = datePart.split("-").map((s) => Number.parseInt(s, 10));
	if (!y || !m || !d) return undefined;
	const dt = new Date(y, m - 1, d);
	if (Number.isNaN(dt.getTime())) return undefined;
	return dt;
}

function formatDateIso(d: Date): string {
	const y = d.getFullYear();
	const m = String(d.getMonth() + 1).padStart(2, "0");
	const day = String(d.getDate()).padStart(2, "0");
	return `${y}-${m}-${day}`;
}

function isSameCalendarDate(
	d: Date,
	y: number,
	m: number,
	day: number,
): boolean {
	return d.getFullYear() === y && d.getMonth() === m - 1 && d.getDate() === day;
}

function parseCompactDateParts(
	compact: string,
	yearMode: DateYearMode,
): number[] | null {
	if (/^\d{8}$/.test(compact)) {
		return [
			Number.parseInt(compact.slice(0, 4), 10),
			Number.parseInt(compact.slice(4, 6), 10),
			Number.parseInt(compact.slice(6, 8), 10),
		];
	}
	if (yearMode === "minguo" && /^\d{7}$/.test(compact)) {
		return [
			Number.parseInt(compact.slice(0, 3), 10),
			Number.parseInt(compact.slice(3, 5), 10),
			Number.parseInt(compact.slice(5, 7), 10),
		];
	}
	if (yearMode === "minguo" && /^\d{6}$/.test(compact)) {
		return [
			Number.parseInt(compact.slice(0, 2), 10),
			Number.parseInt(compact.slice(2, 4), 10),
			Number.parseInt(compact.slice(4, 6), 10),
		];
	}
	return null;
}

function parseManualDateInput(
	input: string,
	yearMode: DateYearMode,
): string | null {
	const trimmed = input.trim();
	if (!trimmed) return "";

	const compact = trimmed.replace(/\D/g, "");
	let parts: number[] | null = null;
	if (/^\d+$/.test(trimmed)) {
		parts = parseCompactDateParts(compact, yearMode);
	} else {
		const tokens = trimmed.match(/\d+/g);
		if (tokens && tokens.length >= 3) {
			parts = tokens.slice(0, 3).map((token) => Number.parseInt(token, 10));
		}
	}
	if (!parts) return null;
	let [year, month, day] = parts;
	if (yearMode === "minguo" && year > 0 && year < 1000) {
		year += 1911;
	}
	if (!year || year < 1000 || !month || !day) return null;
	const candidate = new Date(year, month - 1, day);
	if (
		Number.isNaN(candidate.getTime()) ||
		!isSameCalendarDate(candidate, year, month, day)
	) {
		return null;
	}
	return formatDateIso(candidate);
}

function isWithinDateBounds(
	iso: string,
	minDate?: string,
	maxDate?: string,
): boolean {
	const date = parseDate(iso);
	const min = parseDate(minDate);
	const max = parseDate(maxDate);
	if (!date) return false;
	if (min && date < min) return false;
	if (max && date > max) return false;
	return true;
}

function splitDateTime(value: string | undefined) {
	if (!value) return { datePart: "", timePart: "" };
	const [datePart = "", timePartRaw = ""] = value.split("T");
	return { datePart, timePart: timePartRaw.slice(0, 5) };
}

function formatYear(year: number, yearMode: DateYearMode): string {
	if (yearMode === "minguo") return String(year - 1911).padStart(3, "0");
	return String(year);
}

function formatDisplay(
	value: string,
	granularity: DateGranularity,
	yearMode: DateYearMode,
): string {
	const date = parseDate(value);
	if (!date) return "";
	const y = formatYear(date.getFullYear(), yearMode);
	const m = String(date.getMonth() + 1).padStart(2, "0");
	const d = String(date.getDate()).padStart(2, "0");
	if (granularity === "date-time") {
		const { timePart } = splitDateTime(value);
		return timePart ? `${y}/${m}/${d} ${timePart}` : `${y}/${m}/${d}`;
	}
	return `${y}/${m}/${d}`;
}

const CalendarIcon = () => (
	<svg
		width="18"
		height="18"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="1.6"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<rect x="3.5" y="5" width="17" height="15" rx="2.5" />
		<path d="M3.5 10h17" />
		<path d="M8 3v4" />
		<path d="M16 3v4" />
	</svg>
);

/** @import import { DatePicker } from '@/components/ui/DatePicker'; */
export function DatePicker({
	value = "",
	onChange,
	granularity = "date",
	yearMode = "gregorian",
	label,
	description,
	errorMessage,
	placeholder,
	minDate,
	maxDate,
	variant = "bordered",
	size = "md",
	floatingLabel = true,
	isRequired,
	isInvalid,
	disabled,
	id,
	name,
	className,
	locale = "zh-TW",
	weekStartsOn = 0,
}: DatePickerProps) {
	const [open, setOpen] = React.useState(false);
	const rootRef = React.useRef<HTMLDivElement>(null);
	const popoverRef = React.useRef<HTMLDivElement>(null);
	const inputRef = React.useRef<HTMLInputElement>(null);
	const generatedId = React.useId();
	const popoverId = `${id ?? generatedId}-calendar`;
	/** 關閉時是否要把焦點送回觸發欄位（Escape / 面板內關閉＝要，點外面＝看情況）。 */
	const returnFocusRef = React.useRef(false);
	/** 開啟時是否要把焦點直接放到日期格（鍵盤開啟／手機 modal＝要）。 */
	const [autoFocusDay, setAutoFocusDay] = React.useState(false);
	const [timeFocused, setTimeFocused] = React.useState(false);
	const [manualError, setManualError] = React.useState<string | null>(null);

	/**
	 * 桌機 popover 的視窗座標。
	 *
	 * 原本桌機是 inline 渲染＋`position: absolute`，錨在 wrapper 上。只要有任何
	 * 祖先是捲動容器（`overflow` 非 visible），absolute 子層就會被那一層裁掉——
	 * Hawkr Agent 的問題卡住在 `.thread-page__body`（`overflow-y: auto`）裡，卡片
	 * 又貼在捲動區底部，日曆整個開在可視範圍外，使用者看到的就是「點了沒反應」。
	 *
	 * 改成一律 portal 到 body ＋ `position: fixed`，座標由觸發器的
	 * getBoundingClientRect 量出來（與 features/procurement-assistant/
	 * ConversationActionsMenu 同一套解法，那支的註解記錄了同一個坑）。fixed
	 * 不受祖先 overflow 裁切，其餘四個既有呼叫點的視覺位置維持不變：右緣對齊
	 * 觸發器右緣、頂端在觸發器下方 6px，與舊的 `right: 0; top: calc(100% + 6px)`
	 * 等價。
	 */
	const [desktopPos, setDesktopPos] = React.useState<{
		left: number;
		top: number;
	} | null>(null);

	// Track mobile vs desktop so the popover can be rendered as a centered
	// modal on mobile and an anchored popover on desktop. Both go through a
	// portal (see desktopPos).
	const [isMobile, setIsMobile] = React.useState(false);
	React.useEffect(() => {
		const mql = window.matchMedia("(max-width: 768px)");
		const sync = () => setIsMobile(mql.matches);
		sync();
		mql.addEventListener("change", sync);
		return () => mql.removeEventListener("change", sync);
	}, []);

	const { datePart, timePart } = splitDateTime(value);
	const selectedDate = parseDate(datePart);
	const displayValue = formatDisplay(value, granularity, yearMode);
	const [draftValue, setDraftValue] = React.useState(displayValue);
	const [editing, setEditing] = React.useState(false);
	const datePlaceholder = yearMode === "minguo" ? "YYY/MM/DD" : "YYYY/MM/DD";
	const placeholderText =
		placeholder ??
		(granularity === "date-time"
			? `${datePlaceholder} HH:mm`
			: datePlaceholder);

	React.useEffect(() => {
		// While a manual-entry error is showing, the user's own text stays in
		// the field — reverting it here is the silent data-loss bug this
		// component used to have.
		if (!editing && manualError === null) setDraftValue(displayValue);
	}, [displayValue, editing, manualError]);

	/**
	 * 關閉面板。`returnFocus` 為 true 時把焦點送回觸發欄位——舊版 Escape 後焦點
	 * 直接掉回 document 開頭，鍵盤使用者得從頭 Tab 回來（Dialog.tsx 做對了）。
	 * 實際的 `.focus()` 由下面的 effect 在關閉後才執行：手機版 modal 期間整個
	 * `.page-wrapper` 是 inert 的，inert 還沒解除前對輸入框 focus 不會生效。
	 */
	const closeCalendar = React.useCallback((returnFocus = true) => {
		returnFocusRef.current = returnFocus;
		setAutoFocusDay(false);
		setOpen(false);
	}, []);

	// Close on outside click + ESC. On desktop the popover is inside the
	// root wrapper, so a rootRef contains-check is sufficient. On mobile
	// the popover is portaled to document.body — outside the rootRef tree
	// — so we also exempt clicks inside the popover node itself.
	React.useEffect(() => {
		if (!open) return;
		const onClick = (e: MouseEvent) => {
			const target = e.target as Node;
			if (rootRef.current?.contains(target)) return;
			if (popoverRef.current?.contains(target)) return;
			// 焦點還在面板裡才收回來：點在別的欄位上時搶焦點反而更糟。
			const focusWasInside = popoverRef.current?.contains(
				document.activeElement,
			);
			closeCalendar(Boolean(focusWasInside));
		};
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") closeCalendar(true);
		};
		document.addEventListener("mousedown", onClick);
		document.addEventListener("keydown", onKey);
		return () => {
			document.removeEventListener("mousedown", onClick);
			document.removeEventListener("keydown", onKey);
		};
	}, [open, closeCalendar]);

	// 手機全螢幕 modal：鎖捲動＋把背後的 `.page-wrapper` 設 inert。少了 inert，
	// Tab 與讀屏都會漏到 scrim 後面的頁面（Dialog.tsx 的同一套隔離）。
	React.useEffect(() => {
		if (!open || !isMobile) return;
		const releaseScroll = acquireBodyScrollLock();
		const releaseInert = setInertOutside();
		return () => {
			releaseInert();
			releaseScroll();
		};
	}, [open, isMobile]);

	// 必須排在 inert effect 之後：同一次 commit 裡 React 先跑完所有 cleanup
	// （解除 inert）再跑 effect body，順序反了焦點會落在 inert 元素上而失效。
	React.useEffect(() => {
		if (open) return;
		if (!returnFocusRef.current) return;
		returnFocusRef.current = false;
		inputRef.current?.focus({ preventScroll: true });
	}, [open]);

	/** 與 ui-kit.css 的 `.hkui-datepicker__popover { min-width: 18rem }` 同值；
	 * 用來把 popover 右緣對齊觸發器右緣（沿用舊的 `right: 0` 語意）。 */
	const DESKTOP_POPOVER_MIN_WIDTH = 288;
	const VIEWPORT_EDGE = 12;

	/** 量觸發器位置換算成 fixed 座標。回 false＝觸發器已被捲出視窗，呼叫端
	 * 據此關閉 popover（留著只會讓日曆飄在無關內容上）。 */
	const repositionDesktop = React.useCallback((): boolean => {
		const rect = rootRef.current?.getBoundingClientRect();
		if (!rect) return false;
		if (rect.bottom < 0 || rect.top > window.innerHeight) return false;
		const width = Math.max(
			popoverRef.current?.offsetWidth ?? 0,
			DESKTOP_POPOVER_MIN_WIDTH,
		);
		setDesktopPos({
			left: Math.max(VIEWPORT_EDGE, rect.right - width),
			top: rect.bottom + 6,
		});
		return true;
	}, []);

	/**
	 * 落地後量真實高度：撐出視窗底部就翻到觸發器上方。
	 *
	 * 少了這段，觸發器一旦靠近視窗下緣，日曆（約 340px）就整個開在畫面外——
	 * 使用者看到的是「點了沒反應」。Hawkr Agent 的問題卡永遠是對話最後一則、
	 * 貼在捲動區底部，所以那條路徑 100% 會踩到。
	 *
	 * 只在需要時修一次，不進入「改了位置又重新判斷」的迴圈（判斷式只看會不會
	 * 超出下緣）；上方也放不下就維持向下開，讓它至少能捲到。
	 */
	React.useLayoutEffect(() => {
		if (!open || isMobile || !desktopPos) return;
		const height = popoverRef.current?.offsetHeight ?? 0;
		if (!height) return;
		if (desktopPos.top + height <= window.innerHeight - VIEWPORT_EDGE) return;
		const rect = rootRef.current?.getBoundingClientRect();
		if (!rect) return;
		const flipped = rect.top - height - 6;
		if (flipped < VIEWPORT_EDGE) return;
		setDesktopPos({ left: desktopPos.left, top: flipped });
	}, [open, isMobile, desktopPos]);

	// 開啟後跟著觸發器走：fixed 座標與捲動脫鉤，不重算的話捲一下日曆就留在
	// 原地。捲出視窗才關閉，不是「一捲就關」——把焦點送進日曆本身就可能讓
	// 瀏覽器捲動祖先，那種寫法會讓它開了瞬間自己關掉。
	React.useLayoutEffect(() => {
		if (!open || isMobile) return;
		repositionDesktop();
	}, [open, isMobile, repositionDesktop]);

	React.useEffect(() => {
		if (!open || isMobile) return;
		const sync = () => {
			if (!repositionDesktop()) setOpen(false);
		};
		window.addEventListener("scroll", sync, true);
		window.addEventListener("resize", sync);
		return () => {
			window.removeEventListener("scroll", sync, true);
			window.removeEventListener("resize", sync);
		};
	}, [open, isMobile, repositionDesktop]);

	/**
	 * `focusDay`＝開啟後把焦點直接送到日期格。點欄位本身時**不能**送，否則手動
	 * 輸入（民國年那條路徑）會被日曆搶走焦點；從日曆圖示鈕或 ArrowDown 開啟時
	 * 才送。手機版是真 modal，一律送。
	 */
	const openCalendar = (focusDay = false) => {
		if (disabled) return;
		setAutoFocusDay(focusDay || isMobile);
		setOpen(true);
	};

	const toggle = () => {
		if (disabled) return;
		if (open) closeCalendar(true);
		else openCalendar(true);
	};

	const commitManualInput = React.useCallback(
		(raw: string, closePopover = false) => {
			const iso = parseManualDateInput(raw, yearMode);
			if (iso === "") {
				setManualError(null);
				onChange?.("");
				setDraftValue("");
				if (closePopover) closeCalendar(false);
				return;
			}
			// 解析不出來就**留著使用者打的字**並報錯，不要靜默還原成上一個值：
			// 舊行為讓輸入憑空消失，使用者連自己打錯在哪都看不到。
			if (!iso) {
				setManualError(MANUAL_FORMAT_ERROR);
				setDraftValue(raw);
				return;
			}
			if (!isWithinDateBounds(iso, minDate, maxDate)) {
				setManualError(MANUAL_RANGE_ERROR);
				setDraftValue(raw);
				return;
			}
			setManualError(null);
			const nextValue =
				granularity === "date-time" ? `${iso}T${timePart || "00:00"}` : iso;
			onChange?.(nextValue);
			setDraftValue(formatDisplay(nextValue, granularity, yearMode));
			if (closePopover) closeCalendar(false);
		},
		[
			closeCalendar,
			granularity,
			maxDate,
			minDate,
			onChange,
			timePart,
			yearMode,
		],
	);

	const handleSelectDate = (next: Date) => {
		setManualError(null);
		const iso = formatDateIso(next);
		if (granularity === "date-time") {
			const time = timePart || "00:00";
			onChange?.(`${iso}T${time}`);
			setDraftValue(formatDisplay(`${iso}T${time}`, granularity, yearMode));
			// keep popover open so user can also set time
		} else {
			onChange?.(iso);
			setDraftValue(formatDisplay(iso, granularity, yearMode));
			closeCalendar(true);
		}
	};

	const handleTimeChange = (nextTime: string) => {
		const base = datePart || formatDateIso(new Date());
		onChange?.(`${base}T${nextTime || "00:00"}`);
	};

	const handleClear = () => {
		setManualError(null);
		onChange?.("");
		closeCalendar(true);
	};

	const handleApply = () => closeCalendar(true);

	const ariaLabel = typeof label === "string" && label ? label : "日期選擇器";

	const popoverNode = open ? (
		<>
			{/* Mobile-only scrim — hidden via CSS on desktop. Tapping the
			 * scrim closes the modal (matches the standard dialog pattern
			 * and gives users an obvious way out on touch devices). */}
			<button
				type="button"
				className="hkui-datepicker__scrim"
				aria-label="關閉日期選擇"
				onClick={() => closeCalendar(true)}
			/>
			{/* aria-modal 只在手機全螢幕 modal 那條分支成立（背後 `.page-wrapper`
			 * 真的被 inert 掉了）。桌機是非 modal 的錨定 popover，宣告 aria-modal
			 * 會讓讀屏誤以為頁面其他內容不存在，所以只留 role="dialog"。 */}
			<div
				ref={popoverRef}
				id={popoverId}
				className="hkui-datepicker__popover"
				data-portalled={isMobile ? undefined : "desktop"}
				style={isMobile ? undefined : (desktopPos ?? { visibility: "hidden" })}
				role="dialog"
				aria-modal={isMobile ? true : undefined}
				aria-label={ariaLabel}
				onKeyDown={(e) => {
					if (e.key !== "Tab") return;
					const root = popoverRef.current;
					if (root) wrapTabFocus(e, root);
				}}
			>
				<CalendarMonth
					className={
						yearMode === "minguo" ? "hkui-calendar--minguo-picker" : undefined
					}
					locale={locale}
					weekStartsOn={weekStartsOn}
					weekdayLabels={TAIWAN_WEEKDAYS}
					selectedDate={selectedDate}
					minDate={parseDate(minDate)}
					maxDate={parseDate(maxDate)}
					yearDisplayMode={yearMode}
					showTaiwanYearFlag={yearMode === "minguo"}
					titleAs="p"
					autoFocusDay={autoFocusDay}
					onSelectDate={handleSelectDate}
				/>
				{granularity === "date-time" && (
					<div className="hkui-datepicker__time">
						<label htmlFor={`${id ?? "hkui-dp"}-time`}>時間</label>
						{/* 時間欄改用與日期欄同一套 input-wrapper 樣式，尺寸／焦點環
						 * ／無效態都跟著 Input 家族走，不再是自己一份手寫規則。 */}
						<div
							className={inputWrapperClasses({
								variant,
								size,
								focused: timeFocused,
								invalid: false,
								disabled: Boolean(disabled),
								floating: false,
								wrapperClassName: "flex-1",
							})}
						>
							<input
								id={`${id ?? "hkui-dp"}-time`}
								type="time"
								className={CONTROL_RESET}
								data-slot="control"
								value={timePart}
								disabled={disabled}
								onFocus={() => setTimeFocused(true)}
								onBlur={() => setTimeFocused(false)}
								onChange={(e) => handleTimeChange(e.target.value)}
							/>
						</div>
					</div>
				)}
				<div className="hkui-datepicker__footer">
					<button
						type="button"
						className="hkui-datepicker__action hkui-datepicker__action--ghost"
						onClick={handleClear}
					>
						清除
					</button>
					<button
						type="button"
						className="hkui-datepicker__action hkui-datepicker__action--primary"
						onClick={handleApply}
					>
						完成
					</button>
				</div>
			</div>
		</>
	) : null;

	return (
		<div
			ref={rootRef}
			className={cn("hkui-datepicker", open && "is-open", className)}
		>
			<Input
				ref={inputRef}
				id={id}
				label={label}
				description={description}
				errorMessage={manualError ?? errorMessage}
				variant={variant}
				size={size}
				floatingLabel={floatingLabel}
				isRequired={isRequired}
				isInvalid={isInvalid || manualError !== null}
				disabled={disabled}
				value={draftValue}
				placeholder={placeholderText}
				inputMode="numeric"
				autoComplete="off"
				onClick={() => openCalendar()}
				onFocus={() => setEditing(true)}
				onBlur={(e) => {
					setEditing(false);
					commitManualInput(e.currentTarget.value);
				}}
				onChange={(e) => {
					setDraftValue(e.target.value);
					if (manualError !== null) setManualError(null);
				}}
				onKeyDown={(e) => {
					if (e.key === "Enter") {
						e.preventDefault();
						setEditing(false);
						commitManualInput(e.currentTarget.value, true);
					} else if (e.key === "Escape") {
						setDraftValue(displayValue);
						setManualError(null);
						setEditing(false);
						closeCalendar(false);
					} else if (e.key === "ArrowDown" && !open) {
						e.preventDefault();
						openCalendar(true);
					}
				}}
				aria-haspopup="dialog"
				aria-controls={open ? popoverId : undefined}
				endContent={
					// 鍵盤可達（舊版 tabIndex=-1，只有未文件化的 ArrowDown 能開）。
					// 44×44 點擊區靠負 margin 撐出來，28px 的圖示視覺不變。
					<button
						type="button"
						className="hkui-datepicker__trigger"
						onClick={(e) => {
							e.stopPropagation();
							toggle();
						}}
						disabled={disabled}
						aria-label="開啟日期選擇"
						aria-haspopup="dialog"
						aria-expanded={open}
						aria-controls={open ? popoverId : undefined}
					>
						<CalendarIcon />
					</button>
				}
			/>
			{name && <input type="hidden" name={name} value={value} />}

			{/* Mobile: portal the modal+scrim out to document.body so it
			 * escapes any ancestor with `transform` / `backdrop-filter` /
			 * `filter` / `contain` — those properties create a new
			 * containing block for `position: fixed`, which makes the
			 * modal stick to the input region instead of the viewport.
			 * Desktop: render inline so the popover anchors to the
			 * trigger position via the wrapper. */}
			{popoverNode && typeof document !== "undefined"
				? createPortal(popoverNode, document.body)
				: popoverNode}
		</div>
	);
}

export type { DateGranularity, DatePickerProps };
