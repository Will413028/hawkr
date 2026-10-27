"use client";

import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { UI_TEXT } from "../i18n";

interface Props {
	open: boolean;
	onClose: () => void;
}

const TEMPLATE_PATH = "/templates/hawkr_import_template.csv";

/**
 * CSV 編輯指南 modal。從 UploadCsvModal 內兩個入口點觸發：
 *  1. 範本下載條下方的「查看 CSV 編輯指南」連結
 *  2. CSV 驗證錯誤條下方的「遇到格式錯誤？查看 CSV 編輯指南」連結
 *
 * 設計目標：避免賣家手寫 CSV 時撞到 RFC 4180 引號 / 跳脫規則，
 * 結帳前一次教完。元件嚴格復用 UploadCsvModal 的 design idiom，
 * 不引入新 icon、新色系、新圓角規格。
 */
export function CsvFormatGuideModal({ open, onClose }: Props) {
	const t = UI_TEXT.csvGuideModal;
	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="lg"
			title={t.title}
			footer={
				<div className="flex w-full justify-end gap-2">
					<Button variant="light" color="default" onClick={onClose}>
						{t.close}
					</Button>
				</div>
			}
		>
			<div className="space-y-6">
				{/* § 1 推薦做法 */}
				<section className="space-y-2">
					<h3 className="text-sm font-semibold text-[var(--hkui-default-900)]">
						{t.section1.heading}
					</h3>
					<div className="rounded border border-[var(--hkui-default-200)] bg-[var(--hkui-default-50)] px-3 py-2 text-sm text-[var(--hkui-default-700)]">
						<p>{t.section1.body}</p>
						<p className="mt-1 text-xs text-[var(--hkui-default-500)]">
							{t.section1.note}
						</p>
					</div>
				</section>

				{/* § 2 手動編輯規則 + before/after 對照 */}
				<section className="space-y-2">
					<h3 className="text-sm font-semibold text-[var(--hkui-default-900)]">
						{t.section2.heading}
					</h3>
					<p className="text-sm text-[var(--hkui-default-700)]">
						{t.section2.bodyPrefix}
						<strong>{t.section2.bodyEmphasis}</strong>
						{t.section2.bodyMiddle}{" "}
						<code className="font-mono text-xs">
							{t.section2.bodyQuoteChar}
						</code>{" "}
						{t.section2.bodyAfterQuote}{" "}
						<code className="font-mono text-xs">
							{t.section2.bodyDoubleQuote}
						</code>
						{t.section2.bodyEnd}
					</p>

					<div className="grid gap-3 md:grid-cols-2">
						{/* Case 1 — 含逗號未跳脫 */}
						<div className="rounded border border-[var(--hkui-default-200)] bg-white p-3">
							<p className="text-xs text-[var(--hkui-default-500)]">
								{t.section2.caseLabelBad}
							</p>
							<pre className="mt-1 whitespace-pre-wrap break-all font-mono text-xs text-[var(--hkui-default-700)]">
								{t.section2.case1Bad}
							</pre>
							<p className="mt-1 text-xs text-[var(--hkui-danger-700)]">
								{t.section2.case1BadReason}
							</p>
						</div>
						<div className="rounded border border-[var(--hkui-default-200)] bg-white p-3">
							<p className="text-xs text-[var(--hkui-default-500)]">
								{t.section2.caseLabelGood}
							</p>
							<pre className="mt-1 whitespace-pre-wrap break-all font-mono text-xs text-[var(--hkui-default-700)]">
								{t.section2.case1Good}
							</pre>
						</div>

						{/* Case 2 — 含雙引號未跳脫 */}
						<div className="rounded border border-[var(--hkui-default-200)] bg-white p-3">
							<p className="text-xs text-[var(--hkui-default-500)]">
								{t.section2.caseLabelBad}
							</p>
							<pre className="mt-1 whitespace-pre-wrap break-all font-mono text-xs text-[var(--hkui-default-700)]">
								{t.section2.case2Bad}
							</pre>
							<p className="mt-1 text-xs text-[var(--hkui-danger-700)]">
								{t.section2.case2BadReason}
							</p>
						</div>
						<div className="rounded border border-[var(--hkui-default-200)] bg-white p-3">
							<p className="text-xs text-[var(--hkui-default-500)]">
								{t.section2.caseLabelGood}
							</p>
							<pre className="mt-1 whitespace-pre-wrap break-all font-mono text-xs text-[var(--hkui-default-700)]">
								{t.section2.case2Good}
							</pre>
						</div>
					</div>
				</section>

				{/* § 3 描述特別提醒 */}
				<section className="space-y-2">
					<h3 className="text-sm font-semibold text-[var(--hkui-default-900)]">
						{t.section3.heading}
					</h3>
					<div className="rounded border border-[var(--hkui-default-200)] bg-[var(--hkui-default-50)] px-3 py-2 text-sm text-[var(--hkui-default-700)]">
						{t.section3.bodyPrefix}
						<strong>{t.section3.bodyEmphasis}</strong>
						{t.section3.bodySuffix}
					</div>
				</section>

				{/* § 4 範本實例 + 下載入口 */}
				<section className="space-y-2">
					<h3 className="text-sm font-semibold text-[var(--hkui-default-900)]">
						{t.section4.heading}
					</h3>
					<div className="rounded border border-[var(--hkui-default-200)] bg-[var(--hkui-default-50)] px-3 py-2 text-sm">
						<p className="text-[var(--hkui-default-700)]">
							{t.section4.bodyPrefix}
							<code className="font-mono text-xs">{t.section4.bodyCode}</code>
							{t.section4.bodyMiddle}
						</p>
						<div className="mt-2 flex justify-end">
							<a
								href={TEMPLATE_PATH}
								download
								className="text-[var(--hkui-primary)] underline"
							>
								{UI_TEXT.downloadTemplate}
							</a>
						</div>
					</div>
				</section>
			</div>
		</Dialog>
	);
}
