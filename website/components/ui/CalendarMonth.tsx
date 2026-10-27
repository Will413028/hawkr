"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";

type WeekStartsOn = 0 | 1;
type CalendarYearDisplayMode = "gregorian" | "minguo";

type CalendarDay = {
	date: Date;
	key: string;
	day: number;
	isCurrentMonth: boolean;
	isToday: boolean;
	isSelected: boolean;
	isDisabled: boolean;
};

interface CalendarMonthProps
	extends Omit<React.HTMLAttributes<HTMLDivElement>, "onSelect"> {
	month?: Date;
	defaultMonth?: Date;
	selectedDate?: Date;
	today?: Date;
	minDate?: Date;
	maxDate?: Date;
	locale?: string;
	weekStartsOn?: WeekStartsOn;
	showAdjacentDays?: boolean;
	weekdayLabels?: string[];
	monthLabels?: string[];
	onMonthChange?: (month: Date) => void;
	onSelectDate?: (date: Date) => void;
	yearDisplayMode?: CalendarYearDisplayMode;
	showTaiwanYearFlag?: boolean;
	/**
	 * Show year + month <select> jump-pickers in the header so users can
	 * navigate years/decades without clicking the prev arrow hundreds of
	 * times (essential for birthday inputs). When omitted, the picker is
	 * shown automatically whenever the resolved year range spans more
	 * than ~2 years; set `false` to force the legacy static title.
	 */
	showJumpPickers?: boolean;
	/**
	 * Explicit min/max year for the year <select>. Defaults to:
	 *   - `minDate.year` / `maxDate.year` when provided
	 *   - otherwise current year ± 50
	 */
	yearRange?: { min: number; max: number };
	/**
	 * Element used for the month title. Popovers already sit inside a
	 * labelled dialog, so a hard-coded `h2` there injects a bogus level-2
	 * landmark into the page outline — those callers pass `"p"`.
	 */
	titleAs?: "h2" | "h3" | "p";
	/**
	 * Move DOM focus onto the active day cell as soon as the grid mounts.
	 * Used by `DatePicker` when the popover is opened from the keyboard so
	 * the arrow keys work immediately.
	 */
	autoFocusDay?: boolean;
}

const DEFAULT_WEEKDAYS = ["SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"];
const DEFAULT_MONTHS = [
	"1月",
	"2月",
	"3月",
	"4月",
	"5月",
	"6月",
	"7月",
	"8月",
	"9月",
	"10月",
	"11月",
	"12月",
];

function startOfDay(date: Date) {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function startOfMonth(date: Date) {
	return new Date(date.getFullYear(), date.getMonth(), 1);
}

function dateKey(date: Date) {
	const year = date.getFullYear();
	const month = String(date.getMonth() + 1).padStart(2, "0");
	const day = String(date.getDate()).padStart(2, "0");
	return `${year}-${month}-${day}`;
}

function formatYearLabel(year: number, mode: CalendarYearDisplayMode) {
	if (mode === "minguo") return `民國${year - 1911}年`;
	return String(year);
}

function addMonths(date: Date, amount: number) {
	return new Date(date.getFullYear(), date.getMonth() + amount, 1);
}

function addDays(date: Date, amount: number) {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate() + amount);
}

/** PageUp / PageDown：同一個「日」跳到相鄰月份，月底不足則夾到當月最後一天。 */
function addMonthsKeepingDay(date: Date, amount: number) {
	const target = addMonths(date, amount);
	const lastDayOfTarget = new Date(
		target.getFullYear(),
		target.getMonth() + 1,
		0,
	).getDate();
	return addDays(target, Math.min(date.getDate(), lastDayOfTarget) - 1);
}

function isBeforeDay(left: Date, right: Date) {
	return startOfDay(left).getTime() < startOfDay(right).getTime();
}

function isAfterDay(left: Date, right: Date) {
	return startOfDay(left).getTime() > startOfDay(right).getTime();
}

function buildMonthGrid({
	month,
	selectedDate,
	today,
	minDate,
	maxDate,
	weekStartsOn,
}: {
	month: Date;
	selectedDate?: Date;
	today: Date;
	minDate?: Date;
	maxDate?: Date;
	weekStartsOn: WeekStartsOn;
}): CalendarDay[] {
	const monthStart = startOfMonth(month);
	const firstDay = monthStart.getDay();
	const offset = (firstDay - weekStartsOn + 7) % 7;
	const gridStart = new Date(
		monthStart.getFullYear(),
		monthStart.getMonth(),
		1 - offset,
	);
	const totalDays = 42;
	const selectedKey = selectedDate ? dateKey(selectedDate) : undefined;
	const todayKey = dateKey(today);

	return Array.from({ length: totalDays }, (_, index) => {
		const date = new Date(
			gridStart.getFullYear(),
			gridStart.getMonth(),
			gridStart.getDate() + index,
		);
		const key = dateKey(date);
		const isDisabled =
			(minDate ? isBeforeDay(date, minDate) : false) ||
			(maxDate ? isAfterDay(date, maxDate) : false);

		return {
			date,
			key,
			day: date.getDate(),
			isCurrentMonth: date.getMonth() === monthStart.getMonth(),
			isToday: key === todayKey,
			isSelected: key === selectedKey,
			isDisabled,
		};
	});
}

/** @import import { CalendarMonth } from '@/components/ui/CalendarMonth'; */
export const CalendarMonth = React.forwardRef<
	HTMLDivElement,
	CalendarMonthProps
>(function CalendarMonth(
	{
		month,
		defaultMonth,
		selectedDate,
		today = new Date(),
		minDate,
		maxDate,
		locale = "en-US",
		weekStartsOn = 0,
		showAdjacentDays = true,
		weekdayLabels = DEFAULT_WEEKDAYS,
		monthLabels = DEFAULT_MONTHS,
		onMonthChange,
		onSelectDate,
		yearDisplayMode = "gregorian",
		showTaiwanYearFlag = false,
		showJumpPickers,
		yearRange,
		titleAs = "h2",
		autoFocusDay = false,
		className = "",
		...rest
	},
	ref,
) {
	const isControlled = month !== undefined;
	const [internalMonth, setInternalMonth] = React.useState(() =>
		startOfMonth(defaultMonth ?? selectedDate ?? new Date()),
	);
	const visibleMonth = startOfMonth(isControlled ? month : internalMonth);
	// One formatter per locale instead of one per cell per render: the old
	// code allocated 42 `Intl.DateTimeFormat` instances on every keystroke
	// inside DatePicker (`Intl` construction is the expensive part).
	const titleFormatter = React.useMemo(
		() => new Intl.DateTimeFormat(locale, { month: "long", year: "numeric" }),
		[locale],
	);
	const dayLabelFormatter = React.useMemo(
		() => new Intl.DateTimeFormat(locale, { dateStyle: "full" }),
		[locale],
	);
	const title = titleFormatter.format(visibleMonth);
	const labels =
		weekStartsOn === 1
			? [...weekdayLabels.slice(1), weekdayLabels[0]]
			: weekdayLabels;
	const days = buildMonthGrid({
		month: visibleMonth,
		selectedDate,
		today,
		minDate,
		maxDate,
		weekStartsOn,
	});

	// Resolve year range — explicit `yearRange` wins; otherwise derive from
	// min/maxDate; finally fall back to ±50 years around today. This lets
	// `<DatePicker maxDate={todayISO} />` (the birthday case) auto-expose a
	// "1925..2026" jump select without callers having to spell it out.
	const todayYear = today.getFullYear();
	const resolvedYearRange = React.useMemo(() => {
		if (yearRange) return yearRange;
		const min = minDate ? minDate.getFullYear() : todayYear - 100;
		const max = maxDate ? maxDate.getFullYear() : todayYear + 10;
		return { min: Math.min(min, max), max: Math.max(min, max) };
	}, [yearRange, minDate, maxDate, todayYear]);
	const years = React.useMemo(() => {
		const out: number[] = [];
		for (let y = resolvedYearRange.max; y >= resolvedYearRange.min; y--)
			out.push(y);
		return out;
	}, [resolvedYearRange]);
	const renderJumpPickers =
		showJumpPickers ?? resolvedYearRange.max - resolvedYearRange.min >= 2;

	const setVisibleMonth = (nextMonth: Date) => {
		if (!isControlled) setInternalMonth(nextMonth);
		onMonthChange?.(nextMonth);
	};
	const changeMonth = (amount: number) => {
		setVisibleMonth(addMonths(visibleMonth, amount));
	};
	const handleYearChange = (year: number) => {
		setVisibleMonth(new Date(year, visibleMonth.getMonth(), 1));
	};
	const handleMonthChange = (monthIndex: number) => {
		setVisibleMonth(new Date(visibleMonth.getFullYear(), monthIndex, 1));
	};

	/* ===== APG grid: roving tabIndex + arrow-key navigation =====
	 *
	 * Before this, all 42 cells were tabbable and there was no arrow-key
	 * handling, so crossing one month from the keyboard took 42 Tab presses.
	 * Now exactly one cell is in the tab order (the selected day, else today,
	 * else the 1st) and the arrows move a virtual cursor, pulling the visible
	 * month along when the cursor leaves it. */
	const gridRef = React.useRef<HTMLDivElement>(null);
	const [focusDate, setFocusDate] = React.useState<Date | null>(null);
	const pendingFocusRef = React.useRef(false);

	const monthTime = visibleMonth.getTime();
	const isInVisibleMonth = (candidate?: Date | null) =>
		candidate != null && startOfMonth(candidate).getTime() === monthTime;
	let activeDate = visibleMonth;
	if (isInVisibleMonth(focusDate)) activeDate = focusDate as Date;
	else if (isInVisibleMonth(selectedDate)) activeDate = selectedDate as Date;
	else if (isInVisibleMonth(today)) activeDate = today;
	const activeKey = dateKey(activeDate);
	const activeKeyRef = React.useRef(activeKey);
	activeKeyRef.current = activeKey;

	const focusDayCell = React.useCallback((key: string) => {
		gridRef.current
			?.querySelector<HTMLButtonElement>(`[data-day-key="${key}"]`)
			// preventScroll: DatePicker closes its popover when the trigger
			// scrolls out of view, and letting the browser scroll an ancestor
			// here would make the popover close the instant it opened.
			?.focus({ preventScroll: true });
	}, []);

	React.useEffect(() => {
		if (!pendingFocusRef.current) return;
		pendingFocusRef.current = false;
		focusDayCell(activeKey);
	}, [activeKey, focusDayCell]);

	React.useEffect(() => {
		if (!autoFocusDay) return;
		focusDayCell(activeKeyRef.current);
	}, [autoFocusDay, focusDayCell]);

	const moveFocusTo = (next: Date) => {
		if (minDate && isBeforeDay(next, minDate)) return;
		if (maxDate && isAfterDay(next, maxDate)) return;
		pendingFocusRef.current = true;
		setFocusDate(next);
		const nextMonth = startOfMonth(next);
		if (nextMonth.getTime() !== monthTime) setVisibleMonth(nextMonth);
	};

	const handleGridKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
		if (event.altKey || event.ctrlKey || event.metaKey) return;
		const index = days.findIndex((entry) => entry.key === activeKey);
		const rowStart = index >= 0 ? index - (index % 7) : -1;
		switch (event.key) {
			case "ArrowLeft":
				event.preventDefault();
				moveFocusTo(addDays(activeDate, -1));
				break;
			case "ArrowRight":
				event.preventDefault();
				moveFocusTo(addDays(activeDate, 1));
				break;
			case "ArrowUp":
				event.preventDefault();
				moveFocusTo(addDays(activeDate, -7));
				break;
			case "ArrowDown":
				event.preventDefault();
				moveFocusTo(addDays(activeDate, 7));
				break;
			case "Home":
				if (rowStart < 0) break;
				event.preventDefault();
				moveFocusTo(days[rowStart].date);
				break;
			case "End":
				if (rowStart < 0) break;
				event.preventDefault();
				moveFocusTo(days[rowStart + 6].date);
				break;
			case "PageUp":
				event.preventDefault();
				moveFocusTo(addMonthsKeepingDay(activeDate, -1));
				break;
			case "PageDown":
				event.preventDefault();
				moveFocusTo(addMonthsKeepingDay(activeDate, 1));
				break;
			default:
				break;
		}
	};

	const TitleTag = titleAs;

	const weeks = React.useMemo(() => {
		const out: CalendarDay[][] = [];
		for (let i = 0; i < days.length; i += 7) out.push(days.slice(i, i + 7));
		return out;
	}, [days]);

	return (
		<div
			ref={ref}
			className={["hkui-calendar", className].filter(Boolean).join(" ")}
			{...rest}
		>
			<div className="hkui-calendar__header">
				<button
					type="button"
					className="hkui-calendar__nav"
					aria-label="上一個月"
					onClick={() => changeMonth(-1)}
				>
					<span aria-hidden="true" />
				</button>
				{renderJumpPickers ? (
					<fieldset
						className={[
							"hkui-calendar__jump",
							yearDisplayMode === "minguo" ? "is-minguo" : "",
						]
							.filter(Boolean)
							.join(" ")}
						aria-label="跳轉到"
					>
						{showTaiwanYearFlag ? (
							<span className="hkui-calendar__jump-flag" aria-hidden="true">
								<AppIcon name="flag" size={16} fill />
							</span>
						) : null}
						<select
							className="hkui-calendar__jump-select hkui-calendar__jump-select--year"
							aria-label={yearDisplayMode === "minguo" ? "民國年份" : "年份"}
							value={visibleMonth.getFullYear()}
							onChange={(e) =>
								handleYearChange(Number.parseInt(e.target.value, 10))
							}
						>
							{years.map((y) => (
								<option key={y} value={y}>
									{formatYearLabel(y, yearDisplayMode)}
								</option>
							))}
						</select>
						<select
							className="hkui-calendar__jump-select hkui-calendar__jump-select--month"
							aria-label="月份"
							value={visibleMonth.getMonth()}
							onChange={(e) =>
								handleMonthChange(Number.parseInt(e.target.value, 10))
							}
						>
							{monthLabels.map((m, i) => (
								<option key={m} value={i}>
									{m}
								</option>
							))}
						</select>
					</fieldset>
				) : (
					// `aria-live`: the prev/next buttons keep their own label, so
					// without this the month silently changes under the user.
					<TitleTag className="hkui-calendar__title" aria-live="polite">
						{title}
					</TitleTag>
				)}
				<button
					type="button"
					className="hkui-calendar__nav hkui-calendar__nav--next"
					aria-label="下一個月"
					onClick={() => changeMonth(1)}
				>
					<span aria-hidden="true" />
				</button>
			</div>

			{/* 月曆是 APG 的 grid pattern。這些 role 不能換成 <table>/<tr>/<td>：
			 * 版面靠 CSS grid/flex，而一旦把 display 從 table-* 改掉，瀏覽器就會
			 * 把表格語意拿掉，還是得手動補回同一組 role（繞一圈回到原點）。
			 * grid / row / rowgroup 是容器，鍵盤焦點停在 gridcell（日期鈕）上，
			 * 所以容器不需要、也不該進入 tab 序。 */}
			{/* biome-ignore lint/a11y/useSemanticElements: 見上方註解 */}
			<div
				ref={gridRef}
				className="hkui-calendar__table"
				role="grid"
				aria-label={title}
				onKeyDown={handleGridKeyDown}
			>
				{/* biome-ignore lint/a11y/useSemanticElements: ARIA grid pattern，見上方註解 */}
				{/* biome-ignore lint/a11y/useFocusableInteractive: 表頭列不可聚焦 */}
				<div className="hkui-calendar__weekdays" role="row">
					{labels.map((label) => (
						// biome-ignore lint/a11y/useSemanticElements: <th> 需要真的 <table>，見上方註解
						// biome-ignore lint/a11y/useFocusableInteractive: 欄標題不可聚焦
						<span key={label} role="columnheader">
							{label}
						</span>
					))}
				</div>

				{/* biome-ignore lint/a11y/useSemanticElements: ARIA grid pattern，見上方註解 */}
				<div className="hkui-calendar__grid" role="rowgroup">
					{weeks.map((week) => (
						/* biome-ignore lint/a11y/useSemanticElements: ARIA grid pattern，見上方註解 */
						/* biome-ignore lint/a11y/useFocusableInteractive: 焦點在列內的 gridcell 上 */
						<div className="hkui-calendar__row" role="row" key={week[0].key}>
							{week.map((day) => {
								const hiddenAdjacent = !day.isCurrentMonth && !showAdjacentDays;
								return (
									// biome-ignore lint/a11y/useSemanticElements: <td> 需要真的 <table>，見上方註解
									<button
										key={day.key}
										type="button"
										role="gridcell"
										data-day-key={day.key}
										className={[
											"hkui-calendar__day",
											day.isCurrentMonth ? "" : "hkui-calendar__day--outside",
											day.isToday ? "hkui-calendar__day--today" : "",
											day.isSelected ? "hkui-calendar__day--selected" : "",
										]
											.filter(Boolean)
											.join(" ")}
										disabled={day.isDisabled || hiddenAdjacent}
										// Roving tabIndex: only the cursor cell is tabbable, so
										// Tab leaves the grid instead of walking 42 buttons.
										tabIndex={day.key === activeKey ? 0 : -1}
										aria-selected={day.isSelected}
										aria-current={day.isToday ? "date" : undefined}
										aria-label={dayLabelFormatter.format(day.date)}
										onClick={() => onSelectDate?.(day.date)}
									>
										<span>{hiddenAdjacent ? "" : day.day}</span>
									</button>
								);
							})}
						</div>
					))}
				</div>
			</div>
		</div>
	);
});

export type { CalendarMonthProps, CalendarYearDisplayMode, WeekStartsOn };
