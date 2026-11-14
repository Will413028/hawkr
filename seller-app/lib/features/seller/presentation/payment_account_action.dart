import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../shared/utils/floating_toast.dart';
import '../data/payment_account.dart';
import '../data/payment_onboarding.dart';

/// 收款帳戶的動作（目錄的 `action` 文字）按下去做什麼。
enum PaymentAccountActionKind {
  /// 沒有可做的事（沒有動作文字、等金流商、或內嵌元件的帳戶要到網頁版）。
  none,

  /// 無法啟用：去聯絡客服。
  contactSupport,

  /// 開始（或繼續）開通：託管頁面在外部瀏覽器開啟。
  onboard,

  /// 重新讀取收款帳戶。
  reload,
}

/// [account] 的動作要做什麼。錢包與認證狀態畫面共用這一個判斷，兩處不會各走各的。
PaymentAccountActionKind paymentAccountActionKind(PaymentAccount account) =>
    switch (account.status) {
      PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED =>
        PaymentAccountActionKind.contactSupport,
      _ when paymentAccountCanOnboard(account) =>
        account.onboardingSurface ==
                OnboardingSurface.ONBOARDING_SURFACE_EMBEDDED
            ? PaymentAccountActionKind.none
            : PaymentAccountActionKind.onboard,
      PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED =>
        PaymentAccountActionKind.none,
      _ => PaymentAccountActionKind.reload,
    };

/// 執行收款帳戶的動作；[busy] 為真時進行中（按鈕要停用）。
typedef PaymentAccountActions = ({
  bool busy,
  void Function(PaymentAccount account, PaymentAccountActionKind kind) run,
  void Function() refresh,
});

/// 收款帳戶動作的執行與進行中狀態（hook，只在 `HookConsumerWidget` 的 build 裡用）。「重新整理」（[PaymentAccountActions.refresh]，
/// 與 `reload` 動作同一條路徑）先 `RefreshMyPaymentAccount` 再重讀；那一步的失敗以 toast 顯示伺服器的句子：按鈕觸發的由發起的畫面從回傳值顯示，
/// 從金流商頁面回來的那一次沒有發起的畫面，由第一個取走一次性訊號的消費者顯示。
PaymentAccountActions usePaymentAccountActions(
  BuildContext context,
  WidgetRef ref,
) {
  final busy = useState(false);

  void showFailure(Object error) {
    FloatingToast.error(
      context,
      errorText(error, defaults: ref.read(defaultErrorTextProvider)),
    );
  }

  // 從外部瀏覽器回來的失敗沒有發起的畫面：每個消費者都會收到通知，第一個取走的顯示，其餘取到 null。
  ref.listen(paymentOnboardingProvider, (previous, failure) {
    if (failure == null || !context.mounted) return;
    final error = ref
        .read(paymentOnboardingProvider.notifier)
        .takeRefreshFailure();
    if (error != null) showFailure(error);
  });

  Future<void> guarded(Future<void> Function() body) async {
    busy.value = true;
    try {
      await body();
    } finally {
      if (context.mounted) busy.value = false;
    }
  }

  Future<void> onboard(PaymentAccount account) => guarded(() async {
    final outcome = await ref
        .read(paymentOnboardingProvider.notifier)
        .start(account.onboardingSurface);
    if (!context.mounted) return;
    if (outcome == PaymentOnboardingOutcome.failed) {
      FloatingToast.error(
        context,
        ref.read(defaultErrorTextProvider)(Code.internal),
      );
    }
  });

  Future<void> reload() => guarded(() async {
    final result = await ref
        .read(paymentOnboardingProvider.notifier)
        .confirmPaymentAccount();
    final error = result.refreshError;
    if (error != null && context.mounted) showFailure(error);
  });

  return (
    busy: busy.value,
    run: (account, kind) {
      switch (kind) {
        case PaymentAccountActionKind.none:
          break;
        case PaymentAccountActionKind.contactSupport:
          unawaited(context.push(SettingsRoutes.help));
        case PaymentAccountActionKind.onboard:
          unawaited(onboard(account));
        case PaymentAccountActionKind.reload:
          unawaited(reload());
      }
    },
    refresh: () => unawaited(reload()),
  );
}
