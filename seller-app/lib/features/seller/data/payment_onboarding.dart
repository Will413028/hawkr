import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../config/flavor.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'account_scope.dart';
import 'onboarding_summary.dart';
import 'payment_account.dart';
import 'seller_status_refresh.dart';

part 'payment_onboarding.g.dart';

/// 開通連結允許 http（本機測試）：只有 dev。stage 與 prod 的金流商頁面一律 https。
@Riverpod(keepAlive: true)
bool allowInsecureOnboardingUrl(Ref ref) =>
    F.isInitialized && F.appFlavor == Flavor.dev;

/// 在外部瀏覽器開網址（託管頁面不支援 WebView）；回傳有沒有開成功。
@Riverpod(keepAlive: true)
Future<bool> Function(Uri) externalBrowserLauncher(Ref ref) =>
    (url) => launchUrl(url, mode: LaunchMode.externalApplication);

/// 開通的結果。
enum PaymentOnboardingOutcome {
  /// 外部瀏覽器已開啟；賣家完成後切回 App。
  launched,

  /// 取得連結或開啟瀏覽器失敗。
  failed,
}

/// App 回到前景的通知（不經過任何畫面）。
final class _ResumeObserver with WidgetsBindingObserver {
  _ResumeObserver(this._onResumed);

  final void Function() _onResumed;

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) _onResumed();
  }
}

/// 收款帳戶的開通與自動重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
/// 還在都會以 `fresh: true` 重讀一次。回前景的觸發只有這裡一個：畫面用 [watchForeground] 登記「我在前景時要重讀」，
/// 一次回前景最多呼叫一次 [onResumed]。
///
/// `RefreshMyPaymentAccount` 是背景動作：失敗（含 `resource_exhausted`、停權的 `permission_denied`）不顯示，也不擋讀開通摘要。
@Riverpod(keepAlive: true)
class PaymentOnboarding extends _$PaymentOnboarding {
  bool _awaitingReturn = false;
  int _foregroundWatchers = 0;

  @override
  void build() {
    final binding = WidgetsFlutterBinding.ensureInitialized();
    final observer = _ResumeObserver(() {
      if (!ref.mounted) return;
      if (_awaitingReturn || _foregroundWatchers > 0) unawaited(onResumed());
    });
    binding.addObserver(observer);
    ref.onDispose(() => binding.removeObserver(observer));
  }

  /// 畫面登記「我在前景時，App 回前景要自動重讀」；回傳取消登記的函式（重複呼叫無效）。
  void Function() watchForeground() {
    _foregroundWatchers++;
    var released = false;
    return () {
      if (released) return;
      released = true;
      _foregroundWatchers--;
    };
  }

  /// 開始（或繼續）開通：目的地是賣家 App，取得連結並在外部瀏覽器開啟。連結是託管頁面，或帳戶用內嵌元件時伺服器回的網站認證頁
  /// （App 不判斷介面）。
  Future<PaymentOnboardingOutcome> start() async {
    try {
      final session = await ref
          .read(paymentAccountRepositoryProvider)
          .createSession(
            locale: ref.read(uiLocaleProvider).tag,
            allowInsecureUrl: ref.read(allowInsecureOnboardingUrlProvider),
          );
      if (!ref.mounted) return PaymentOnboardingOutcome.failed;
      final launched = await ref.read(externalBrowserLauncherProvider)(
        session.url,
      );
      if (!launched) return PaymentOnboardingOutcome.failed;
      _awaitingReturn = true;
      return PaymentOnboardingOutcome.launched;
    } catch (error) {
      HKLog.logW('[PaymentOnboarding] start failed: $error');
      return PaymentOnboardingOutcome.failed;
    }
  }

  /// App 回到前景。從外部瀏覽器回來時伺服器剛改變了狀態（金流商的事件可能已經到了），走 [confirmPaymentAccount]（`fresh: true`：進行中
  /// 的那一輪可能在變更之前就開始讀了，不共用它）；其他回前景走 [autoRefresh]。回傳重讀有沒有成功。
  Future<bool> onResumed() {
    final returnedFromBrowser = _awaitingReturn;
    _awaitingReturn = false;
    if (returnedFromBrowser) return confirmPaymentAccount();
    return autoRefresh();
  }

  /// 進入認證頁（[onEntry]）與回前景的自動重讀：收款帳戶任務已完成或鎖住（讀得到摘要而且這樣說）時不呼叫金流商，回前景只重讀狀態、
  /// 進入畫面什麼都不讀（畫面剛讀過）；其他情況（含摘要讀不到）走 [confirmPaymentAccount]。不輪詢。
  Future<bool> autoRefresh({bool onEntry = false}) async {
    final state = await _paymentTaskState();
    if (!ref.mounted) return false;
    if (state == OnboardingTaskState.ONBOARDING_TASK_STATE_COMPLETED ||
        state == OnboardingTaskState.ONBOARDING_TASK_STATE_LOCKED) {
      if (onEntry) return true;
      return ref.read(sellerStatusRefreshProvider.notifier).refresh();
    }
    return confirmPaymentAccount();
  }

  /// 手上的開通摘要裡收款帳戶任務的狀態（摘要還在讀就等它）；沒有登入或摘要讀不到是 null。
  Future<OnboardingTaskState?> _paymentTaskState() async {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    if (accountId == null) return null;
    try {
      final summary = await ref.read(
        accountOnboardingProvider(accountId).future,
      );
      return paymentAccountTask(summary)?.state;
    } catch (_) {
      return null;
    }
  }

  /// 先 `RefreshMyPaymentAccount`（伺服器同步向金流商重讀）再 `fresh: true` 重讀全部狀態（含開通摘要）。
  /// 重讀失敗不顯示、不丟出，照樣讀（快取不能留在舊的）。回傳重讀有沒有成功。
  Future<bool> confirmPaymentAccount() async {
    try {
      await ref.read(paymentAccountRepositoryProvider).refresh();
    } catch (error) {
      HKLog.logW('[PaymentOnboarding] RefreshMyPaymentAccount failed: $error');
    }
    if (!ref.mounted) return false;
    return ref.read(sellerStatusRefreshProvider.notifier).refresh(fresh: true);
  }
}
