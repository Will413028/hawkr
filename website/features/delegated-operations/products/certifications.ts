import { certLabel, PRODUCT_CERT_CATALOG } from "@/lib/products/certifications";
import type { DelegatedProductCertification } from "./repo";

/**
 * 代行工作區的認證標章（§6.1.12 規則表「產品驗證」，mockup image6 的 chip 列）。
 *
 * ⚠️ **`certifications[].name` 存的是 `CertificationType` 的碼值**（`3701`／`3702`…），
 * 不是中文名。直接把它印在畫面上會顯示 `3701`，而那既不是錯誤也不會有測試轉紅 ——
 * 代行工作區在開放編輯之前就是這樣顯示的。所有進畫面的字一律過 `certLabel`。
 *
 * ⚠️ **`certNo` 不可以在代行這一面被靜默清掉。** 認證是整份替換（Gateway 未送才以
 * before 回填，`admin_delegated_product.go:508-511`），所以「只保留碼值、不帶編號」
 * 送出去的結果是**賣家的證書編號整批消失**，而請求完全合法、回 `code: 0`。因此
 * chip 一律連編號一起帶著走，要改編號只能移除後重加（重打一次編號是刻意的成本：
 * 認證是法規宣稱，改錯由賣家承擔法律責任）。
 */

/** Gateway `validateProductArrays` 的上限（`admin_delegated_product.go:1245`）。 */
export const CERTIFICATION_MAX = 10;

/** 一個 chip 的顯示字：有編號就一起顯示（編號是這個宣稱的憑據）。 */
export function certChipText(cert: DelegatedProductCertification): string {
	const label = certLabel(cert.name);
	return cert.certNo ? `${label}・${cert.certNo}` : label;
}

/**
 * 三欄表用的摘要。空集合回空字串 —— 抽屜自己會把它畫成 `—`（`OperationConfirmDrawer`
 * 的 `change.before || "—"`），在這裡先寫死一個破折號會讓「全部移除」與「本來就沒有」
 * 在資料層分不出來。
 */
export function certificationsSummary(
	certs: readonly DelegatedProductCertification[],
): string {
	return certs.map(certChipText).join("、");
}

/**
 * 兩份認證是否等值。
 *
 * **順序敏感**：這是整份替換的欄位，送出去的陣列順序就是儲存的順序；把順序當雜訊
 * 忽略會讓「只調順序」變成一次不會發生的送出（按鈕看起來可按、送出後 diff 為空）。
 */
export function sameCertifications(
	a: readonly DelegatedProductCertification[],
	b: readonly DelegatedProductCertification[],
): boolean {
	if (a.length !== b.length) return false;
	return a.every(
		(cert, index) =>
			cert.name === b[index]?.name && cert.certNo === b[index]?.certNo,
	);
}

/**
 * 還可以新增的認證類型。
 *
 * 已經掛上的碼值不再出現在下拉裡：同一個類型掛兩次在 UI 上分不出來，而下游是以
 * 陣列原樣儲存、不做去重。
 */
export function availableCertOptions(
	selected: readonly DelegatedProductCertification[],
): { value: string; label: string }[] {
	const taken = new Set(selected.map((cert) => cert.name));
	return PRODUCT_CERT_CATALOG.filter((item) => !taken.has(item.code)).map(
		(item) => ({ value: item.code, label: item.label }),
	);
}

export type AddCertificationResult =
	| { ok: true; next: DelegatedProductCertification[] }
	| { ok: false; error: string };

/**
 * 新增一筆認證。
 *
 * 三個拒絕條件都不是防呆：
 * - **沒選類型**：送出 `name: ""` 會存成一個沒有語意的宣稱
 * - **沒填編號**：賣家自助端就是「沒有編號的整筆丟棄」（`productFormAdapter.ts:314`），
 *   代行這一面若允許空編號，等於代行者可以建立一個賣家自己建不出來的宣稱
 * - **超過 10 筆**：Gateway 直接 9260，而那個錯誤到畫面上只剩一個代碼
 */
export function addCertification(
	selected: readonly DelegatedProductCertification[],
	name: string,
	certNo: string,
): AddCertificationResult {
	const code = name.trim();
	const no = certNo.trim();
	if (code === "") return { ok: false, error: "請先選擇認證類型" };
	if (no === "") return { ok: false, error: "請填寫證書編號" };
	if (selected.some((cert) => cert.name === code)) {
		return { ok: false, error: "這個認證類型已經在清單裡" };
	}
	if (selected.length >= CERTIFICATION_MAX) {
		return {
			ok: false,
			error: `認證標章最多 ${CERTIFICATION_MAX} 個`,
		};
	}
	return { ok: true, next: [...selected, { name: code, certNo: no }] };
}

/** 移除第 `index` 筆。以索引而非碼值為鍵，認不得的舊碼值才移除得掉。 */
export function removeCertification(
	selected: readonly DelegatedProductCertification[],
	index: number,
): DelegatedProductCertification[] {
	return selected.filter((_, i) => i !== index);
}
