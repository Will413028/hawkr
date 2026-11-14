/**
 * 開通任務摘要（`GetMyOnboarding`）與收款帳戶重讀（`RefreshMyPaymentAccount`）的資料層。
 *
 * 必要任務的狀態、下一步與句子都由伺服器推導（D26、D27），這裡只有讀取與「某一項是否完成」的查詢；
 * 沒有從 `GetMyKYC`、`GetMyPaymentAccount`、`GetMyStore` 組合出來的狀態。
 */

import {
	type GetMyOnboardingResponse,
	type OnboardingTask,
	OnboardingTaskId,
	OnboardingTaskState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerAppClient } from "../connect/seller";

type SellerClient = ReturnType<typeof sellerAppClient>;

export async function fetchMyOnboarding(
	client: () => SellerClient = sellerAppClient,
): Promise<GetMyOnboardingResponse> {
	return client().getMyOnboarding({});
}

/**
 * 同步向金流商重讀收款帳戶並更新伺服器的快取。方法是 IDEMPOTENT：`resource_exhausted` 帶 RetryInfo 與
 * `unavailable` 由 transport 的 retryInterceptor 重試，這裡不再重試。
 */
export async function refreshMyPaymentAccount(
	client: () => SellerClient = sellerAppClient,
): Promise<void> {
	await client().refreshMyPaymentAccount({});
}

/** 摘要裡某個任務的狀態；沒有摘要或沒有這個任務是 undefined（不當成已完成）。 */
export function onboardingTaskState(
	onboarding: GetMyOnboardingResponse | null | undefined,
	id: OnboardingTaskId,
): OnboardingTaskState | undefined {
	return onboarding?.tasks.find((task) => task.id === id)?.state;
}

/** 摘要裡的收款帳戶任務（下一步的按鈕與說明句只讀它）；沒有摘要或沒有這個任務是 undefined。 */
export function paymentAccountTask(
	onboarding: GetMyOnboardingResponse | null | undefined,
): OnboardingTask | undefined {
	return onboarding?.tasks.find(
		(task) => task.id === OnboardingTaskId.PAYMENT_ACCOUNT,
	);
}

/** 收款帳戶任務已完成（伺服器定義：轉帳能力啟用）；沒有摘要或沒有這個任務不是完成。 */
export function paymentAccountCompleted(
	onboarding: GetMyOnboardingResponse | null | undefined,
): boolean {
	return (
		onboardingTaskState(onboarding, OnboardingTaskId.PAYMENT_ACCOUNT) ===
		OnboardingTaskState.COMPLETED
	);
}

/** 平台審核已完成（伺服器定義：商家認證核准）。 */
export function platformReviewCompleted(
	onboarding: GetMyOnboardingResponse | null | undefined,
): boolean {
	return (
		onboardingTaskState(onboarding, OnboardingTaskId.PLATFORM_REVIEW) ===
		OnboardingTaskState.COMPLETED
	);
}
