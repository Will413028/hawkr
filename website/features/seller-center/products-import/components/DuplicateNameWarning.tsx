"use client";

import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { Chip } from "@/components/ui/Chip";
import { UI_TEXT } from "../i18n";

interface Props {
	existingProductId: string;
	existingProductName: string;
	existingStatusLabel: string;
	allowDuplicate: boolean;
	onChange: (allow: boolean) => void;
}

/**
 * 重名警示卡 — 顯示既有同名商品的 id / 名稱 / 狀態並提供前往連結。
 *
 * v3：欄位由 backend `ImportItemBrief.existingProduct*` 直接帶入；若 gateway
 * 尚未更新（欄位皆空字串）則 fallback 到摘要文字，無詳細連結。
 */
export function DuplicateNameWarning({
	existingProductId,
	existingProductName,
	existingStatusLabel,
	allowDuplicate,
	onChange,
}: Props) {
	const hasDetails = Boolean(existingProductId && existingProductName);
	return (
		<section className="rounded-[var(--hkui-radius-md)] border border-[var(--hkui-danger)]/70 bg-[var(--hkui-danger-50)] p-4">
			<header className="flex items-start gap-2">
				<AppIcon
					name="danger"
					size={18}
					className="mt-0.5 shrink-0 text-[var(--hkui-danger)]"
				/>
				<div className="space-y-1.5 min-w-0">
					<h4 className="font-semibold text-[var(--hkui-danger-700)]">
						{UI_TEXT.duplicate.title}
					</h4>
					<p className="text-sm text-[var(--hkui-danger-700)]/85 break-words">
						{UI_TEXT.duplicate.description(
							existingProductName,
							existingProductId,
							existingStatusLabel,
						)}
					</p>
					{hasDetails ? (
						<div className="flex flex-wrap items-center gap-2 pt-1">
							{existingStatusLabel ? (
								<Chip size="sm" variant="flat" color="danger">
									{existingStatusLabel}
								</Chip>
							) : null}
							<Link
								href={`/products/${encodeURIComponent(existingProductId)}`}
								target="_blank"
								rel="noopener noreferrer"
								className="text-xs font-medium text-[var(--hkui-danger-700)] underline underline-offset-2 hover:text-[var(--hkui-danger)]"
							>
								{UI_TEXT.duplicate.viewExistingLink}
								<AppIcon name="arrow-right" size={14} />
							</Link>
						</div>
					) : null}
				</div>
			</header>
			<label className="mt-3 flex items-start gap-2 text-sm text-[var(--hkui-danger-700)]">
				<input
					type="checkbox"
					checked={allowDuplicate}
					onChange={(e) => onChange(e.target.checked)}
					className="mt-0.5 size-4 accent-[var(--hkui-danger)]"
				/>
				<span>{UI_TEXT.duplicate.checkboxLabel}</span>
			</label>
		</section>
	);
}
