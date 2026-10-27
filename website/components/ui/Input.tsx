"use client";

import * as React from "react";
import {
	CONTROL_RESET,
	floatingLabelClasses,
	type InputSize,
	type InputVariant,
	inputWrapperClasses,
	inputWrapperData,
	TEXTAREA_RESET,
	TEXTAREA_TEXT_LAYOUT,
} from "./inputWrapperClasses";

interface BaseFieldProps {
	label?: React.ReactNode;
	description?: React.ReactNode;
	errorMessage?: React.ReactNode;
	isRequired?: boolean;
	isInvalid?: boolean;
	variant?: InputVariant;
	size?: InputSize;
	startContent?: React.ReactNode;
	endContent?: React.ReactNode;
	className?: string;
	wrapperClassName?: string;
	id?: string;
	/**
	 * Floating-label mode (Material-style). The label sits as a placeholder
	 * while the field is empty and unfocused, then floats to the top of the
	 * field, shrinks to 12px and stays visible once the field has focus or
	 * content. When set, the top-of-field label is suppressed and this label
	 * absorbs the role.
	 */
	floatingLabel?: boolean;
}

interface InputProps
	extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "size">,
		BaseFieldProps {}

function useFieldId(provided?: string) {
	const generatedId = React.useId();
	return provided ?? `field-${generatedId}`;
}

/** @import import { Input } from '@/components/ui/Input'; */
export const Input = React.forwardRef<HTMLInputElement, InputProps>(
	function Input(
		{
			label,
			description,
			errorMessage,
			isRequired,
			isInvalid,
			variant = "bordered",
			size = "md",
			startContent,
			endContent,
			className = "",
			wrapperClassName = "",
			id,
			disabled,
			floatingLabel,
			value,
			defaultValue,
			placeholder,
			onFocus,
			onBlur,
			onChange,
			...rest
		},
		ref,
	) {
		const fieldId = useFieldId(id);
		const [focused, setFocused] = React.useState(false);
		const [uncontrolledFilled, setUncontrolledFilled] = React.useState(false);
		const innerRef = React.useRef<HTMLInputElement | null>(null);
		const mergeRef = React.useCallback(
			(node: HTMLInputElement | null) => {
				innerRef.current = node;
				if (typeof ref === "function") ref(node);
				else if (ref) ref.current = node;
			},
			[ref],
		);
		const invalid = isInvalid || Boolean(errorMessage);
		const isControlled = value != null;
		const valueHasContent =
			value != null
				? String(value).length > 0
				: defaultValue != null
					? String(defaultValue).length > 0
					: false;
		// Uncontrolled inputs (e.g. react-hook-form's register) keep their value
		// in the DOM, not in props — read it from the node so the floating label
		// still lifts off a pre-filled field (edit forms) instead of overlapping
		// the text.
		//
		// This used to be a dependency-LESS effect: it re-read the DOM and
		// called setState on every single render of every uncontrolled field,
		// relying on React's same-value bailout to avoid a loop. Now it syncs
		// on mount / value identity and subscribes to the node's own `input`
		// event, which also covers programmatic population (RHF `reset()`)
		// without touching the DOM on unrelated renders.
		// `value`/`defaultValue` below are deliberate re-sync TRIGGERS, not
		// values read inside the effect: an edit form that swaps `defaultValue`
		// once its data lands must re-read the node, and a field that flips
		// controlled/uncontrolled must re-subscribe.
		// biome-ignore lint/correctness/useExhaustiveDependencies: re-sync triggers
		React.useEffect(() => {
			if (isControlled) return;
			const node = innerRef.current;
			if (!node) return;
			const sync = () => setUncontrolledFilled(node.value.length > 0);
			sync();
			node.addEventListener("input", sync);
			node.addEventListener("change", sync);
			return () => {
				node.removeEventListener("input", sync);
				node.removeEventListener("change", sync);
			};
		}, [isControlled, value, defaultValue]);
		// Date/time pickers always paint a browser placeholder (年/月/日,
		// HH:MM, etc.) — treat them as "filled" for floating-label purposes
		// so the label doesn't sit on top of the format hint.
		const browserPaintsPlaceholder = [
			"date",
			"datetime-local",
			"time",
			"month",
			"week",
		].includes(rest.type ?? "text");
		const hasContent =
			valueHasContent ||
			browserPaintsPlaceholder ||
			(!isControlled && uncontrolledFilled);

		const wrapperClasses = inputWrapperClasses({
			variant,
			size,
			focused,
			invalid,
			disabled: Boolean(disabled),
			floating: Boolean(floatingLabel && label),
			wrapperClassName,
		});

		// In floating-label mode the label IS the placeholder while the field is
		// empty, and once it floats up it still names the field — so an
		// externally-passed placeholder stays suppressed either way (it would
		// read as duplicated label text under the floated label).
		const effectivePlaceholder = floatingLabel ? undefined : placeholder;

		return (
			<div
				data-slot="field"
				className={["flex flex-col gap-1.5 w-full", className]
					.filter(Boolean)
					.join(" ")}
			>
				{label && !floatingLabel && (
					<label
						htmlFor={fieldId}
						data-slot="label"
						className={`text-sm font-medium text-foreground${isRequired ? " after:content-['_*'] after:text-[var(--hkui-danger-text)]" : ""}`}
					>
						{label}
					</label>
				)}
				<div
					{...inputWrapperData({
						variant,
						focused,
						floating: Boolean(floatingLabel && label),
					})}
					className={wrapperClasses}
				>
					{startContent && (
						<span className="shrink-0 inline-flex items-center justify-center text-foreground-soft">
							{startContent}
						</span>
					)}
					<input
						ref={mergeRef}
						id={fieldId}
						data-slot="control"
						className={CONTROL_RESET}
						disabled={disabled}
						aria-invalid={invalid || undefined}
						aria-describedby={
							description || errorMessage ? `${fieldId}-desc` : undefined
						}
						required={isRequired}
						value={value}
						defaultValue={defaultValue}
						placeholder={effectivePlaceholder}
						onFocus={(e) => {
							setFocused(true);
							onFocus?.(e);
						}}
						onBlur={(e) => {
							setFocused(false);
							onBlur?.(e);
						}}
						onChange={(e) => {
							if (!isControlled)
								setUncontrolledFilled(e.target.value.length > 0);
							onChange?.(e);
						}}
						{...rest}
					/>
					{floatingLabel && label && (
						<label
							htmlFor={fieldId}
							data-slot="floating-label"
							className={floatingLabelClasses({
								active: focused || hasContent,
								focused,
								size,
								variant,
							})}
						>
							{label}
							{isRequired && (
								<span className="text-[var(--hkui-danger-text)]"> *</span>
							)}
						</label>
					)}
					{endContent && (
						<span className="shrink-0 inline-flex items-center justify-center text-foreground-soft">
							{endContent}
						</span>
					)}
				</div>
				{invalid && errorMessage ? (
					<span
						id={`${fieldId}-desc`}
						data-slot="error"
						className="text-xs text-[var(--hkui-danger-text)] flex items-center gap-1"
					>
						{errorMessage}
					</span>
				) : description ? (
					<span
						id={`${fieldId}-desc`}
						data-slot="hint"
						className="text-xs text-[var(--hkui-field-copy)]"
					>
						{description}
					</span>
				) : null}
			</div>
		);
	},
);

interface TextareaProps
	extends Omit<React.TextareaHTMLAttributes<HTMLTextAreaElement>, "size">,
		BaseFieldProps {
	minRows?: number;
	/**
	 * Renders a colour-highlight backdrop behind the textarea — e.g. to tint
	 * #hashtags as the user types. The function receives the current value and
	 * returns the same text with the parts you want coloured wrapped in <span>s.
	 * The textarea's own glyphs are made transparent (caret stays visible) so the
	 * aligned backdrop shows through. Requires a controlled `value`.
	 * Return the original text without changing its font, weight, spacing, or
	 * display layout: highlights may change colour, but must preserve caret and
	 * selection geometry. Floating-label space is outside the highlight viewport.
	 */
	renderHighlight?: (value: string) => React.ReactNode;
}

/** @import import { Textarea } from '@/components/ui/Input'; */
export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
	function Textarea(
		{
			label,
			description,
			errorMessage,
			isRequired,
			isInvalid,
			variant = "bordered",
			size = "md",
			className = "",
			wrapperClassName = "",
			id,
			disabled,
			minRows = 3,
			floatingLabel,
			value,
			defaultValue,
			placeholder,
			onFocus,
			onBlur,
			onScroll,
			style,
			renderHighlight,
			...rest
		},
		ref,
	) {
		const fieldId = useFieldId(id);
		const [focused, setFocused] = React.useState(false);
		const invalid = isInvalid || Boolean(errorMessage);

		// Highlight-overlay plumbing: keep an internal handle to the textarea (in
		// addition to the forwarded ref) so we can mirror its scroll onto the
		// backdrop, keeping coloured tokens aligned with the real glyphs.
		const innerRef = React.useRef<HTMLTextAreaElement | null>(null);
		const backdropRef = React.useRef<HTMLDivElement | null>(null);
		const setTextareaRef = React.useCallback(
			(node: HTMLTextAreaElement | null) => {
				innerRef.current = node;
				if (typeof ref === "function") ref(node);
				else if (ref)
					(ref as React.MutableRefObject<HTMLTextAreaElement | null>).current =
						node;
			},
			[ref],
		);
		const syncBackdropScroll = React.useCallback(() => {
			const ta = innerRef.current;
			const bd = backdropRef.current;
			if (ta && bd) {
				bd.scrollTop = ta.scrollTop;
				bd.scrollLeft = ta.scrollLeft;
			}
		}, []);
		React.useLayoutEffect(() => {
			const textarea = innerRef.current;
			const backdrop = backdropRef.current;
			if (!renderHighlight || !textarea || !backdrop) return;
			const syncViewport = () => {
				// These borderless layers fill the same container. Subtract only
				// native scrollbars, preserving fractional dimensions at text zoom.
				backdrop.style.width = `calc(100% - ${textarea.offsetWidth - textarea.clientWidth}px)`;
				backdrop.style.height = `calc(100% - ${textarea.offsetHeight - textarea.clientHeight}px)`;
				syncBackdropScroll();
			};
			syncViewport();
			const observer = new ResizeObserver(syncViewport);
			observer.observe(textarea);
			return () => observer.disconnect();
		}, [renderHighlight, syncBackdropScroll]);
		// biome-ignore lint/correctness/useExhaustiveDependencies: value updates can clamp native scroll without emitting a scroll event
		React.useLayoutEffect(() => {
			if (renderHighlight) syncBackdropScroll();
		}, [value, renderHighlight, syncBackdropScroll]);
		const isControlled = value != null;
		const valueHasContent =
			value != null
				? String(value).length > 0
				: defaultValue != null
					? String(defaultValue).length > 0
					: false;
		// Same uncontrolled DOM read as Input: without it a textarea populated
		// by `reset()` kept its floating label sitting on top of the text.
		const [uncontrolledFilled, setUncontrolledFilled] = React.useState(false);
		// biome-ignore lint/correctness/useExhaustiveDependencies: re-sync triggers
		React.useEffect(() => {
			if (isControlled) return;
			const node = innerRef.current;
			if (!node) return;
			const sync = () => setUncontrolledFilled(node.value.length > 0);
			sync();
			node.addEventListener("input", sync);
			node.addEventListener("change", sync);
			return () => {
				node.removeEventListener("input", sync);
				node.removeEventListener("change", sync);
			};
		}, [isControlled, value, defaultValue]);
		const hasContent = valueHasContent || (!isControlled && uncontrolledFilled);

		const wrapperClasses = inputWrapperClasses({
			variant,
			size,
			focused,
			invalid,
			disabled: Boolean(disabled),
			floating: Boolean(floatingLabel && label),
			textarea: true,
			wrapperClassName,
		});

		const effectivePlaceholder = floatingLabel ? undefined : placeholder;

		return (
			<div
				data-slot="field"
				className={["flex flex-col gap-1.5 w-full", className]
					.filter(Boolean)
					.join(" ")}
			>
				{label && !floatingLabel && (
					<label
						htmlFor={fieldId}
						data-slot="label"
						className={`text-sm font-medium text-foreground${isRequired ? " after:content-['_*'] after:text-[var(--hkui-danger-text)]" : ""}`}
					>
						{label}
					</label>
				)}
				<div
					{...inputWrapperData({
						variant,
						focused,
						floating: Boolean(floatingLabel && label),
					})}
					className={wrapperClasses}
				>
					{(() => {
						const textareaEl = (
							<textarea
								ref={setTextareaRef}
								id={fieldId}
								data-slot="textarea"
								className={
									renderHighlight
										? `${TEXTAREA_RESET} block relative z-[1]`
										: TEXTAREA_RESET
								}
								rows={minRows}
								disabled={disabled}
								aria-invalid={invalid || undefined}
								aria-describedby={
									description || errorMessage ? `${fieldId}-desc` : undefined
								}
								required={isRequired}
								value={value}
								defaultValue={defaultValue}
								placeholder={effectivePlaceholder}
								onFocus={(e) => {
									setFocused(true);
									onFocus?.(e);
								}}
								onBlur={(e) => {
									setFocused(false);
									onBlur?.(e);
								}}
								onScroll={(e) => {
									if (renderHighlight) syncBackdropScroll();
									onScroll?.(e);
								}}
								// Highlight mode: hide the real glyphs (the aligned backdrop
								// shows them), but keep the caret + selection visible.
								style={
									renderHighlight
										? {
												color: "transparent",
												caretColor: "var(--hkui-foreground)",
												...style,
											}
										: style
								}
								{...rest}
							/>
						);
						if (!renderHighlight) return textareaEl;
						return (
							<div data-slot="textarea-editor" className="flex-1 min-w-0">
								<div className="relative">
									<div
										ref={backdropRef}
										data-slot="highlight"
										aria-hidden="true"
										className={`absolute inset-0 overflow-hidden pointer-events-none text-inherit ${TEXTAREA_TEXT_LAYOUT} ${rest.wrap === "off" ? "whitespace-pre break-normal" : "whitespace-pre-wrap break-words"}`}
									>
										{renderHighlight(String(value ?? defaultValue ?? ""))}
										{"\n"}
									</div>
									{textareaEl}
								</div>
							</div>
						);
					})()}
					{floatingLabel && label && (
						<label
							htmlFor={fieldId}
							data-slot="floating-label"
							className={floatingLabelClasses({
								active: focused || hasContent,
								focused,
								textarea: true,
								variant,
								size,
							})}
						>
							{label}
							{isRequired && (
								<span className="text-[var(--hkui-danger-text)]"> *</span>
							)}
						</label>
					)}
				</div>
				{invalid && errorMessage ? (
					<span
						id={`${fieldId}-desc`}
						data-slot="error"
						className="text-xs text-[var(--hkui-danger-text)] flex items-center gap-1"
					>
						{errorMessage}
					</span>
				) : description ? (
					<span
						id={`${fieldId}-desc`}
						data-slot="hint"
						className="text-xs text-[var(--hkui-field-copy)]"
					>
						{description}
					</span>
				) : null}
			</div>
		);
	},
);

interface SelectProps
	extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, "size">,
		BaseFieldProps {
	placeholder?: string;
}

const ChevronDownIcon = () => (
	<svg
		width="16"
		height="16"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="2"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="m6 9 6 6 6-6" />
	</svg>
);

/** @import import { Select } from '@/components/ui/Input'; */
export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
	function Select(
		{
			label,
			description,
			errorMessage,
			isRequired,
			isInvalid,
			variant = "bordered",
			size = "md",
			startContent,
			className = "",
			wrapperClassName = "",
			id,
			disabled,
			placeholder,
			children,
			onFocus,
			onBlur,
			...rest
		},
		ref,
	) {
		const fieldId = useFieldId(id);
		const [focused, setFocused] = React.useState(false);
		const invalid = isInvalid || Boolean(errorMessage);

		const wrapperClasses = inputWrapperClasses({
			variant,
			size,
			focused,
			invalid,
			disabled: Boolean(disabled),
			floating: false,
			wrapperClassName,
		});

		return (
			<div
				data-slot="field"
				className={["flex flex-col gap-1.5 w-full", className]
					.filter(Boolean)
					.join(" ")}
			>
				{label && (
					<label
						htmlFor={fieldId}
						data-slot="label"
						className={`text-sm font-medium text-foreground${isRequired ? " after:content-['_*'] after:text-[var(--hkui-danger-text)]" : ""}`}
					>
						{label}
					</label>
				)}
				<div
					{...inputWrapperData({ variant, focused, floating: false })}
					className={wrapperClasses}
				>
					{startContent && (
						<span className="shrink-0 inline-flex items-center justify-center text-foreground-soft">
							{startContent}
						</span>
					)}
					<select
						ref={ref}
						id={fieldId}
						data-slot="control"
						className={`${CONTROL_RESET} appearance-none cursor-pointer`}
						disabled={disabled}
						required={isRequired}
						aria-invalid={invalid || undefined}
						aria-describedby={
							description || errorMessage ? `${fieldId}-desc` : undefined
						}
						onFocus={(e) => {
							setFocused(true);
							onFocus?.(e);
						}}
						onBlur={(e) => {
							setFocused(false);
							onBlur?.(e);
						}}
						{...rest}
					>
						{placeholder && (
							<option value="" disabled hidden>
								{placeholder}
							</option>
						)}
						{children}
					</select>
					<span
						className="shrink-0 text-foreground-soft transition-transform duration-200"
						aria-hidden="true"
					>
						<ChevronDownIcon />
					</span>
				</div>
				{invalid && errorMessage ? (
					<span
						id={`${fieldId}-desc`}
						data-slot="error"
						className="text-xs text-[var(--hkui-danger-text)] flex items-center gap-1"
					>
						{errorMessage}
					</span>
				) : description ? (
					<span
						id={`${fieldId}-desc`}
						data-slot="hint"
						className="text-xs text-[var(--hkui-field-copy)]"
					>
						{description}
					</span>
				) : null}
			</div>
		);
	},
);

export type { InputProps, InputSize, InputVariant, SelectProps, TextareaProps };
