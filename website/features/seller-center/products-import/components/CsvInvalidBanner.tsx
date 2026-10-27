"use client";

import Button from "@/components/ui/Button";
import { fieldLabel, UI_TEXT, validationErrorLabel } from "../i18n";
import { useProductsImportUiStore } from "../importUiStore";
import type { ValidationError } from "../types";

interface Props {
	errors: ValidationError[];
	onRetry: () => void;
}

export function CsvInvalidBanner({ errors, onRetry }: Props) {
	const open = useProductsImportUiStore((state) => state.csvInvalidExpanded);
	const toggleOpen = useProductsImportUiStore(
		(state) => state.toggleCsvInvalidExpanded,
	);
	return (
		<section
			role="alert"
			className="rounded-[var(--hkui-radius-md)] border border-[var(--hkui-warning)] bg-[var(--hkui-warning-50)] p-4"
		>
			<header className="flex flex-wrap items-center justify-between gap-3">
				<div className="flex items-start gap-2">
					<span
						aria-hidden
						className="mt-0.5 inline-block size-2 rounded-full bg-[var(--hkui-warning)]"
					/>
					<div>
						<h3 className="font-semibold text-[var(--hkui-warning-700)]">
							{UI_TEXT.csvInvalidBanner.title}
						</h3>
						<p className="mt-0.5 text-sm text-[var(--hkui-warning-700)]/80">
							{UI_TEXT.csvInvalidExpireHint}
						</p>
					</div>
				</div>
				<div className="flex items-center gap-2">
					<Button
						color="warning"
						variant="bordered"
						size="sm"
						onClick={onRetry}
					>
						{UI_TEXT.csvInvalidBanner.retry}
					</Button>
					<button
						type="button"
						onClick={toggleOpen}
						className="text-xs text-[var(--hkui-warning-700)] underline"
					>
						{UI_TEXT.csvInvalidBanner.toggleErrors(errors.length)}
					</button>
				</div>
			</header>
			{open && errors.length > 0 ? (
				<ul className="mt-3 max-h-64 space-y-1 overflow-y-auto rounded border border-[var(--hkui-warning)]/40 bg-white/70 p-3 text-sm">
					{errors.map((e, idx) => (
						<li
							// biome-ignore lint/suspicious/noArrayIndexKey: server payload has no stable id
							key={idx}
							className="flex flex-wrap gap-x-3 text-[var(--hkui-default-700)]"
						>
							<span className="text-xs font-medium text-[var(--hkui-default-500)]">
								第 {e.csvLine} 行 · {fieldLabel(e.field) || e.field}
							</span>
							<span>{validationErrorLabel(e.errorType, e.message)}</span>
						</li>
					))}
				</ul>
			) : null}
		</section>
	);
}
