import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../shared/utils/floating_toast.dart';
import '../data/payment_onboarding.dart';

/// 收款帳戶任務的下一步（`OnboardingTask.action`）有沒有按鈕可按：只有開始開通與聯絡客服；沒有下一步、
/// UNSPECIFIED 與不認得的值都沒有按鈕。
bool paymentAccountActionHasButton(OnboardingAction action) =>
    action == OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING ||
    action == OnboardingAction.ONBOARDING_ACTION_CONTACT_SUPPORT;

/// 執行收款帳戶任務的下一步；[busy] 為真時進行中（按鈕要停用）。
typedef PaymentAccountActions = ({
  bool busy,
  void Function(OnboardingAction action) run,
});

/// 收款帳戶任務下一步的執行與進行中狀態（hook，只在 `HookConsumerWidget` 的 build 裡用）。開始開通一律呼叫
/// `CreatePaymentOnboardingSession` 並在外部瀏覽器開伺服器回的連結，失敗以一般的錯誤句顯示。
PaymentAccountActions usePaymentAccountActions(
  BuildContext context,
  WidgetRef ref,
) {
  final busy = useState(false);

  Future<void> onboard() async {
    busy.value = true;
    try {
      final outcome = await ref
          .read(paymentOnboardingProvider.notifier)
          .start();
      if (!context.mounted) return;
      if (outcome == PaymentOnboardingOutcome.failed) {
        FloatingToast.error(
          context,
          ref.read(defaultErrorTextProvider)(Code.internal),
        );
      }
    } finally {
      if (context.mounted) busy.value = false;
    }
  }

  return (
    busy: busy.value,
    run: (action) {
      switch (action) {
        case OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING:
          unawaited(onboard());
        case OnboardingAction.ONBOARDING_ACTION_CONTACT_SUPPORT:
          unawaited(context.push(SettingsRoutes.help));
        default:
          break;
      }
    },
  );
}
