/**
 * 單元測試用的 `GetMyOnboardingResponse` 組裝：照 proto 的形狀建出伺服器的任務清單，測試不必各自拼 wire 形狀。
 * 句子用 client 程式碼產生不出來的字（例如「伺服器標題」），讓測試分得出「顯示伺服器的句子」與「client 自己寫的句子」。
 */

import { create } from "@bufbuild/protobuf";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	type GetMyOnboardingResponse,
	GetMyOnboardingResponseSchema,
	OnboardingAction,
	OnboardingTaskId,
	OnboardingTaskSchema,
	type OnboardingTaskState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

export interface TaskFixture {
	id: OnboardingTaskId;
	state: OnboardingTaskState;
	action?: OnboardingAction;
	title?: string;
	status?: string;
}

export function onboardingFixture(
	tasks: readonly TaskFixture[],
	complete = false,
): GetMyOnboardingResponse {
	return create(GetMyOnboardingResponseSchema, {
		complete,
		tasks: tasks.map((t) =>
			create(OnboardingTaskSchema, {
				id: t.id,
				state: t.state,
				action: t.action ?? OnboardingAction.NONE,
				title: create(LocalizedMessageSchema, {
					locale: "zh-TW",
					message: t.title ?? `伺服器標題${t.id}`,
				}),
				status: create(LocalizedMessageSchema, {
					locale: "zh-TW",
					message: t.status ?? `伺服器狀態${t.id}${t.state}`,
				}),
			}),
		),
	});
}

/** 三個必要任務依序給定狀態與下一步。 */
export function requiredFixture(
	profile: Pick<TaskFixture, "state" | "action">,
	platformReview: Pick<TaskFixture, "state" | "action">,
	paymentAccount: Pick<TaskFixture, "state" | "action">,
	complete = false,
): GetMyOnboardingResponse {
	return onboardingFixture(
		[
			{ id: OnboardingTaskId.PROFILE, ...profile },
			{ id: OnboardingTaskId.PLATFORM_REVIEW, ...platformReview },
			{ id: OnboardingTaskId.PAYMENT_ACCOUNT, ...paymentAccount },
		],
		complete,
	);
}
