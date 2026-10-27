import { Code, ConnectError } from "@connectrpc/connect";
import { classifyError, factsFromError } from "@hawkr/api/connect/errors";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { AgreementKind } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { SELLER_AGREEMENTS } from "@/features/legal/documents/sellerAgreements.gen";

/** 要帶回 ApplyStore 與 SubmitKYC 的同意。 */
export interface AgreementAcceptanceInput {
	kind: AgreementKind;
	version: string;
}

export const AGREEMENT_OUTDATED_NOTICE = "條款已更新，請重新整理頁面後再勾選。";

type AgreementName = keyof typeof SELLER_AGREEMENTS;

function agreementOf(kind: AgreementKind): {
	version: string;
	url: string;
} {
	const name = AgreementKind[kind] as AgreementName | undefined;
	const found = name === undefined ? undefined : SELLER_AGREEMENTS[name];
	if (!found) throw new Error(`沒有 ${kind} 的條款版本`);
	return found;
}

/** 賣家在這個 bundle 裡實際看到的版本與位置。 */
export function agreementUrl(kind: AgreementKind): string {
	return agreementOf(kind).url;
}

/** 依 bundle 裡的版本組出要送的同意；種類不在 bundle 裡就 throw，不送出空版本。 */
export function acceptancesOf(
	kinds: readonly AgreementKind[],
): AgreementAcceptanceInput[] {
	return kinds.map((kind) => ({ kind, version: agreementOf(kind).version }));
}

/** 伺服器回 failed_precondition 加 reason AGREEMENT_OUTDATED：版本已不是現行。 */
export function isAgreementOutdated(error: unknown): boolean {
	const e = ConnectError.from(error);
	if (e.code !== Code.FailedPrecondition) return false;
	const d = classifyError(factsFromError(e));
	return d.action === "reason" && d.reason === ErrorReason.AGREEMENT_OUTDATED;
}
