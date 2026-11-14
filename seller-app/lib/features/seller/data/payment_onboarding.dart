import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../config/flavor.dart';
import '../../../shared/utils/hk_log.dart';
import 'payment_account.dart';
import 'seller_status_refresh.dart';

part 'payment_onboarding.g.dart';

/// 金流商的託管頁面結束後回到 website 靜態頁的哪一種（頁面標題讀 `?state=`）。
enum PaymentReturnState { done, expired }

/// website 的回跳頁（免登入，內容是「請回到 hawkr 賣家 App」）：`<origin>/seller-app/payment-return?state=done|expired`。
/// 預設語系沒有路徑前綴（`localePrefix: as-needed`）。App 不加 deep link：賣家自己切回來，回到前景時重讀狀態。
/// 以 URI 解析組路徑（origin 結尾有沒有斜線都一樣，不會出現 `//`）。
Uri paymentReturnUri(String webOrigin, PaymentReturnState state) =>
    Uri.parse(webOrigin)
        .resolve('/seller-app/payment-return')
        .replace(queryParameters: {'state': state.name});

/// website 的 origin（`--dart-define=HAWKR_WEB_ORIGIN`，見 [FlavorExtension.webOrigin]）。
@Riverpod(keepAlive: true)
String webOrigin(Ref ref) => F.appFlavor.webOrigin;

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

  /// 伺服器說這個帳戶用內嵌元件開通（平台收件）：App 沒有，要到網頁版完成。
  needsWeb,

  /// 取得連結或開啟瀏覽器失敗，或伺服器沒有告知可用的介面。
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

/// 從外部瀏覽器回來那一次「先重讀再讀」裡 `RefreshMyPaymentAccount` 的失敗：沒有發起的畫面，放進 [PaymentOnboarding] 的 state
/// 當一次性訊號，由第一個顯示它的畫面用 [PaymentOnboarding.takeRefreshFailure] 取走（取走即清空），其餘畫面不會再顯示。
/// 每次失敗是新的物件，連續兩次失敗畫面也都會收到。
final class PaymentRefreshFailure {
  PaymentRefreshFailure(this.error);

  final Object error;
}

/// 一次「先重讀再讀」的結果：[read] 是重讀有沒有成功，[refreshError] 是 `RefreshMyPaymentAccount` 的失敗（成功為 null）。
/// 使用者按按鈕觸發時，錯誤由發起的畫面直接從這個回傳值顯示。
typedef PaymentConfirmation = ({bool read, Object? refreshError});

/// 收款帳戶的開通與回到前景時的重讀。keepAlive：從開啟瀏覽器到賣家切回來之間畫面可能已經被關掉，
/// 「剛從瀏覽器回來」的記號放在這裡，不放在畫面上；而且自己監看 App 生命週期，從瀏覽器回來的那一次回前景不論哪個畫面
/// 還在都會以 `fresh: true` 重讀一次（畫面各自的回前景重讀走 [onResumed]，不會重複消耗記號）。
@Riverpod(keepAlive: true)
class PaymentOnboarding extends _$PaymentOnboarding {
  bool _awaitingReturn = false;

  @override
  PaymentRefreshFailure? build() {
    final binding = WidgetsFlutterBinding.ensureInitialized();
    final observer = _ResumeObserver(() {
      if (_awaitingReturn && ref.mounted) unawaited(onResumed());
    });
    binding.addObserver(observer);
    ref.onDispose(() => binding.removeObserver(observer));
    return null;
  }

  /// 依伺服器告知的 [surface] 開始開通。HOSTED：取得連結並在外部瀏覽器開啟；EMBEDDED：不呼叫伺服器，回 [needsWeb]；
  /// 其他（沒有設定、不認得）不猜預設介面，回 [failed]。
  Future<PaymentOnboardingOutcome> start(OnboardingSurface surface) async {
    switch (surface) {
      case OnboardingSurface.ONBOARDING_SURFACE_EMBEDDED:
        return PaymentOnboardingOutcome.needsWeb;
      case OnboardingSurface.ONBOARDING_SURFACE_HOSTED:
        break;
      default:
        return PaymentOnboardingOutcome.failed;
    }
    try {
      final origin = ref.read(webOriginProvider);
      final session = await ref
          .read(paymentAccountRepositoryProvider)
          .createHostedSession(
            returnUrl: paymentReturnUri(origin, PaymentReturnState.done),
            refreshUrl: paymentReturnUri(origin, PaymentReturnState.expired),
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

  /// App 回到前景。從外部瀏覽器回來時伺服器剛改變了狀態（金流商的事件可能已經到了），走 [confirmPaymentAccount]：先
  /// `RefreshMyPaymentAccount` 再 `fresh: true` 重讀（進行中的那一輪可能在變更之前就開始讀了，不共用它）。其他回前景只重讀
  /// 店家、認證、收款帳戶與開通摘要，不呼叫金流商。回傳重讀有沒有成功。
  Future<bool> onResumed() {
    final returnedFromBrowser = _awaitingReturn;
    _awaitingReturn = false;
    if (returnedFromBrowser) return _confirmAfterBrowser();
    return ref.read(sellerStatusRefreshProvider.notifier).refresh();
  }

  /// 從金流商回來與「重新整理」共用的一條路徑：先 `RefreshMyPaymentAccount`（伺服器同步向金流商重讀）再 `fresh: true` 重讀
  /// 全部狀態（含開通摘要）。重讀失敗也照樣讀（快取不能留在舊的）。失敗不經共享 state 廣播，回傳給呼叫端（發起的畫面）顯示。
  /// 一次新的 refresh 開始就清掉留著的失敗訊號：狀態一旦重新確認，之前那次失敗不能再被顯示（成功時它也就保持清空）。
  Future<PaymentConfirmation> confirmPaymentAccount() async {
    if (state != null) state = null;
    Object? failure;
    try {
      await ref.read(paymentAccountRepositoryProvider).refresh();
    } catch (error) {
      HKLog.logW('[PaymentOnboarding] RefreshMyPaymentAccount failed: $error');
      failure = error;
    }
    if (!ref.mounted) return (read: false, refreshError: failure);
    final read = await ref
        .read(sellerStatusRefreshProvider.notifier)
        .refresh(fresh: true);
    return (read: read, refreshError: failure);
  }

  /// 從外部瀏覽器回來：沒有發起的畫面，失敗放進 state 當一次性訊號。
  Future<bool> _confirmAfterBrowser() async {
    final result = await confirmPaymentAccount();
    final failure = result.refreshError;
    if (failure != null && ref.mounted) state = PaymentRefreshFailure(failure);
    return result.read;
  }

  /// 取走 state 裡的一次性失敗訊號並清空；已被別的畫面取走時回 null。
  Object? takeRefreshFailure() {
    final failure = state?.error;
    if (failure != null) state = null;
    return failure;
  }
}
