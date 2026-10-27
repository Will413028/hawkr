import {
	type Gender,
	GenderSchema,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { buyerGender, buyerProfileGenderUnset } from "@hawkr/ui-text/keys";
import type { DropdownItem } from "@/components/ui/Dropdown";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";

/** 選項由生成的 enum 推導（值號 0 不使用）；目錄要求每個非零值都有文字。 */
export const GENDER_CHOICES: readonly Gender[] = GenderSchema.values
	.filter((v) => v.number !== 0)
	.map((v) => v.number as Gender);

/** `undefined` 是沒有填。 */
export function genderText(ut: UiText, gender: Gender | undefined): string {
	if (gender === undefined) return ut(buyerProfileGenderUnset());
	// 沒有文字的值（不認得的值）呈現同沒有設值（API 規範，optional enum）。
	return optionalText(ut, buyerGender(gender)) ?? ut(buyerProfileGenderUnset());
}

/** 第一項（`empty`）代表沒有值，選了就清除性別。 */
export function genderOptions(ut: UiText): DropdownItem[] {
	return [
		{ value: "", label: genderText(ut, undefined), empty: true },
		...GENDER_CHOICES.map((g) => ({
			value: String(g),
			label: genderText(ut, g),
		})),
	];
}

export function genderFromValue(value: string): Gender | undefined {
	return GENDER_CHOICES.find((g) => String(g) === value);
}
