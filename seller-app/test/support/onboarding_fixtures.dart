import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';

const _completed = OnboardingTaskState.ONBOARDING_TASK_STATE_COMPLETED;
const _actionRequired =
    OnboardingTaskState.ONBOARDING_TASK_STATE_ACTION_REQUIRED;
const _locked = OnboardingTaskState.ONBOARDING_TASK_STATE_LOCKED;

const _profile = OnboardingTaskId.ONBOARDING_TASK_ID_PROFILE;
const _review = OnboardingTaskId.ONBOARDING_TASK_ID_PLATFORM_REVIEW;
const _payment = OnboardingTaskId.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT;

const _none = OnboardingAction.ONBOARDING_ACTION_NONE;

/// 伺服器回的一個開通任務；標題與狀態句是伺服器的句子（`LocalizedMessage`），測試用各自可辨識的文字。
OnboardingTask serverTask(
  OnboardingTaskId id,
  OnboardingTaskState state, {
  OnboardingAction action = _none,
  String? title,
  String? status,
}) => OnboardingTask(
  id: id,
  state: state,
  action: action,
  title: LocalizedMessage(locale: 'zh-TW', message: title ?? '標題 ${id.name}'),
  status: LocalizedMessage(
    locale: 'zh-TW',
    message: status ?? '狀態 ${id.name} ${state.name}',
  ),
);

/// `GetMyOnboarding` 的回應。
GetMyOnboardingResponse onboardingSummary(
  List<OnboardingTask> tasks, {
  bool complete = false,
}) => GetMyOnboardingResponse(tasks: tasks, complete: complete);

/// 三項都完成的摘要。
GetMyOnboardingResponse completedSummary() => onboardingSummary([
  serverTask(_profile, _completed),
  serverTask(_review, _completed),
  serverTask(_payment, _completed),
], complete: true);

/// 商店資料完成、平台審核通過、收款帳戶待開通（要賣家動作）的摘要。
GetMyOnboardingResponse paymentPendingSummary() => onboardingSummary([
  serverTask(_profile, _completed),
  serverTask(_review, _completed),
  serverTask(
    _payment,
    _actionRequired,
    action: OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING,
  ),
]);

/// 平台審核還沒通過（收款帳戶鎖定）的摘要。
GetMyOnboardingResponse reviewPendingSummary({
  OnboardingAction action =
      OnboardingAction.ONBOARDING_ACTION_SUBMIT_CERTIFICATION,
}) => onboardingSummary([
  serverTask(_profile, _completed),
  serverTask(_review, _actionRequired, action: action),
  serverTask(_payment, _locked),
]);
