"use client";

import * as React from "react";
import { useLocalizedHref } from "@/lib/intl/use-router";

interface AgreementCheckboxProps {
	checked: boolean;
	disabled?: boolean;
	onChange: (checked: boolean) => void;
	before: string;
	linkText: string;
	/** 文件位置，一律在新分頁開啟。 */
	href: string;
	after?: string;
}

export function AgreementCheckbox({
	checked,
	disabled,
	onChange,
	before,
	linkText,
	href,
	after = "",
}: AgreementCheckboxProps) {
	const localizedHref = useLocalizedHref(href);
	return (
		<label className="inline-flex items-center gap-2 text-[0.8125rem] text-[color:var(--hkui-foreground)]">
			<input
				type="checkbox"
				checked={checked}
				disabled={disabled}
				onChange={(e) => onChange(e.target.checked)}
			/>
			<div>
				<strong>
					{before}〈
					<a
						href={localizedHref}
						target="_blank"
						rel="noopener noreferrer"
						className="underline"
					>
						{linkText}
					</a>
					〉{after}
				</strong>
			</div>
		</label>
	);
}
