"use client";

import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { resolveCertTypeView } from "@/lib/products/certification-status";
import { certLabel } from "@/lib/products/certifications";
import { certChipText, removeCertification } from "./certifications";
import type { DelegatedProductCertification } from "./repo";

/**
 * 認證標章編輯器（建立頁與編輯頁共用，mockup image6 的 chip 列）。
 *
 * 代行入口不得繞過人工審核直接把未審核宣稱寫進商品：既有 chip 保留移除能力，但新增、
 * 換類型與改號碼都由賣家自己在商品編輯頁走申請流程；disabled 申請按鈕只說明這一面
 * 不送出，不代表任何審核狀態。
 *
 * ⚠️ **這一面不共用 `ProductCertificationRow` 的整列，只共用它的狀態判讀與徽章**：
 * 版面是 `adm-*` 的 chip 欄位（換成目錄式六列是版面變更），而且**代行的瀏覽器在 www
 * 上沒有賣家登入態**，拿不到 `certification_applications`。因此一律以
 * `mode="readonly"` 呼叫共用判讀 —— 那個模式明令忽略申請清單、不做即時驗證，
 * 能誠實說出口的只有投影本身的語意：`product.certifications` 自 19 號起是
 * approved-only 公開投影，出現在裡面就代表平台已核准。
 *
 * ⚠️ 唯讀（`disabled`）時仍要把 chip 畫出來：認證是商品頁上對買家的宣稱，看不到目前
 * 掛了什麼，操作者無從判斷自己要不要動它。
 */

/**
 * 這一顆 chip 該標什麼狀態。走共用判讀（`readonly` 模式）而不是在這裡寫死「已通過」：
 * 判讀規則改了，三個面要一起改。
 */
function certStatusLabel(cert: DelegatedProductCertification): string | null {
	return (
		resolveCertTypeView({
			code: cert.name,
			mode: "readonly",
			entry: { certNo: cert.certNo },
		}).badge?.label ?? null
	);
}

export interface CertificationEditorProps {
	value: readonly DelegatedProductCertification[];
	onChange: (next: DelegatedProductCertification[]) => void;
	disabled?: boolean;
}

export default function CertificationEditor({
	value,
	onChange,
	disabled = false,
}: CertificationEditorProps) {
	return (
		<div className="adm-product-edit__certs">
			<ul className="adm-product-edit__cert-list">
				{value.length === 0 ? (
					<li className="adm-product-edit__cert-empty">尚未掛上任何認證標章</li>
				) : (
					value.map((cert, index) => {
						const statusLabel = certStatusLabel(cert);
						return (
							<li
								// biome-ignore lint/suspicious/noArrayIndexKey: 這份清單的身分就是索引 —— 移除也以索引進行（`removeCertification`），因為認不得的歷史碼值可能重複，用 name 當鍵會讓兩列變同一列、刪一筆消失兩筆。
								key={`${cert.name}-${cert.certNo}-${index}`}
								className="adm-product-edit__chip"
							>
								<span>{certChipText(cert)}</span>
								{statusLabel ? (
									<span className="adm-product-edit__chip-status">
										{statusLabel}
									</span>
								) : null}
								{disabled ? null : (
									<button
										type="button"
										className="adm-product-edit__chip-remove"
										aria-label={`移除「${certLabel(cert.name)}」認證標章`}
										onClick={() => onChange(removeCertification(value, index))}
									>
										<AppIcon name="close" size={12} />
									</button>
								)}
							</li>
						);
					})
				)}
			</ul>

			{disabled ? null : (
				<>
					<Button
						type="button"
						variant="bordered"
						className="adm-quiet-button"
						disabled
						aria-describedby="adm-cert-manual-review-note"
					>
						申請
					</Button>
					<p
						id="adm-cert-manual-review-note"
						className="adm-product-edit__note"
					>
						{
							"這一面不送出申請：新增與換號碼要由賣家在商品編輯頁提出，通過人工審核後才會掛上。若目前的宣稱不正確，仍可在這裡移除既有標章。"
						}
					</p>
				</>
			)}
		</div>
	);
}
