import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'account_scope.dart';

part 'onboarding_summary.g.dart';

/// 摘要裡某個任務的狀態；沒有摘要或沒有這個任務是 null（不當成已完成）。
OnboardingTaskState? onboardingTaskState(
  GetMyOnboardingResponse? summary,
  OnboardingTaskId id,
) {
  if (summary == null) return null;
  for (final task in summary.tasks) {
    if (task.id == id) return task.state;
  }
  return null;
}

/// 摘要裡的收款帳戶任務（下一步的按鈕與說明句只讀它）；沒有摘要或沒有這個任務是 null。
OnboardingTask? paymentAccountTask(GetMyOnboardingResponse? summary) {
  if (summary == null) return null;
  for (final task in summary.tasks) {
    if (task.id == OnboardingTaskId.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT) {
      return task;
    }
  }
  return null;
}

/// 平台審核已完成（伺服器定義：商家認證核准）。沒有摘要或沒有這個任務是 false。
bool platformReviewCompleted(GetMyOnboardingResponse? summary) =>
    onboardingTaskState(
      summary,
      OnboardingTaskId.ONBOARDING_TASK_ID_PLATFORM_REVIEW,
    ) ==
    OnboardingTaskState.ONBOARDING_TASK_STATE_COMPLETED;

/// 收款帳戶任務已完成（伺服器定義：轉帳能力啟用）。沒有摘要或沒有這個任務是 false。
bool paymentAccountTaskCompleted(GetMyOnboardingResponse? summary) =>
    onboardingTaskState(
      summary,
      OnboardingTaskId.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT,
    ) ==
    OnboardingTaskState.ONBOARDING_TASK_STATE_COMPLETED;

/// [accountId] 這個帳號的開通摘要（`GetMyOnboarding`）：必要任務的狀態、下一步與文字都由伺服器推導，這裡不組合其他讀取。
/// 失敗是 error 狀態，不當成沒有任務。以帳號為鍵，換帳號不會看到上一個帳號的結果。
@Riverpod(keepAlive: true, retry: noRetry)
Future<GetMyOnboardingResponse> accountOnboarding(
  Ref ref,
  String accountId,
) async {
  try {
    return await ref
        .watch(sellerClientProvider)
        .getMyOnboarding(GetMyOnboardingRequest());
  } catch (error) {
    HKLog.logW('[Onboarding] GetMyOnboarding failed: $error');
    rethrow;
  }
}

/// 目前登入帳號的開通摘要；沒有登入時是 null（外層）。
@Riverpod(keepAlive: true)
AsyncValue<GetMyOnboardingResponse>? currentOnboarding(Ref ref) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  if (accountId == null) return null;
  return ref.watch(accountOnboardingProvider(accountId));
}

/// 重讀目前帳號的開通摘要（`GetMyOnboarding`）。
@Riverpod(keepAlive: true)
class OnboardingSummaryReader extends _$OnboardingSummaryReader {
  @override
  void build() {}

  /// 重讀並回傳結果；讀取失敗時丟出該錯誤（目前的值保持 error 狀態）。沒有登入時回 null。
  Future<GetMyOnboardingResponse?> reload() async {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    if (accountId == null) return null;
    ref.invalidate(accountOnboardingProvider(accountId));
    return ref.read(accountOnboardingProvider(accountId).future);
  }

  /// 重讀，完成後回傳有沒有成功。
  Future<bool> refresh() async {
    try {
      await reload();
      return true;
    } catch (_) {
      return false;
    }
  }
}
