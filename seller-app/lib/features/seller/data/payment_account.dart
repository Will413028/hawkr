import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import 'account_scope.dart';

part 'payment_account.g.dart';

/// 收款帳戶卡的色調（版面，留在 client）；文字（狀態、說明、動作）全部取自共用文案目錄
/// （`seller.PaymentAccountStatus`），這裡沒有「值 → 文字」的表。`UNSPECIFIED` 與不認得的值沒有色調（null）。
HkStatusTone? paymentAccountTone(
  PaymentAccountStatus status,
) => switch (status) {
  PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_NOT_CREATED ||
  PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW => HkStatusTone.info,
  PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED =>
    HkStatusTone.warning,
  PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED => HkStatusTone.success,
  PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED => HkStatusTone.danger,
  _ => null,
};

/// 賣家此刻能開始（或繼續）開通：只有 `ACTION_REQUIRED`，而且伺服器告知了介面（託管頁面或內嵌元件）。
bool paymentAccountCanOnboard(PaymentAccount account) =>
    account.status ==
        PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED &&
    (account.onboardingSurface == OnboardingSurface.ONBOARDING_SURFACE_HOSTED ||
        account.onboardingSurface ==
            OnboardingSurface.ONBOARDING_SURFACE_EMBEDDED);

/// 平台審核還沒通過時，收款帳戶區的提示（沒有收款帳戶可看）。
const paymentAccountNotYetOpenNotice = '平台審核通過後開放設定';

/// 伺服器說這個帳戶用內嵌元件開通（平台收件）時 App 的提示：App 沒有內嵌元件。
const paymentAccountEmbeddedNotice = '請到網頁版完成收款設定';

/// 一次 `CreatePaymentOnboardingSession` 的結果：託管頁面是 [HostedSession.url]；內嵌元件的 client secret 只給網頁
/// 的瀏覽器，App 不使用，所以 App 不呼叫它（見 `payment_onboarding.dart`）。
final class HostedSession {
  const HostedSession(this.url);

  final Uri url;
}

/// `GetMyPaymentAccount`、`RefreshMyPaymentAccount` 與 `CreatePaymentOnboardingSession`，登入後呼叫。
final class PaymentAccountRepository {
  const PaymentAccountRepository(this._client);

  final SellerAppServiceClient _client;

  /// 目前的收款帳戶；還沒有（平台審核還沒核准，伺服器回 not_found）是 null，不是錯誤；其他失敗丟出該錯誤。
  Future<PaymentAccount?> fetch() async {
    try {
      final response = await _client.getMyPaymentAccount(
        GetMyPaymentAccountRequest(),
      );
      return response.hasAccount() ? response.account : null;
    } catch (error) {
      if (ConnectException.from(error).code == Code.notFound) return null;
      rethrow;
    }
  }

  /// 同步向金流商重讀收款帳戶並更新伺服器的快取（從金流商的頁面回來、或賣家按「重新整理」時，先呼叫它再讀開通摘要）。失敗丟出該錯誤。
  /// 方法是 IDEMPOTENT：`resource_exhausted` 帶 RetryInfo 與 `unavailable` 由 transport 的 `retryInterceptor` 重試，這裡不重試。
  Future<void> refresh() async {
    await _client.refreshMyPaymentAccount(RefreshMyPaymentAccountRequest());
  }

  /// 託管頁面的開通連結（一次性、短效）；只收 https，[allowInsecureUrl]（只有 dev 的本機測試）才收 http。回應的介面不是 HOSTED 或沒有連結是契約被違反，丟 [FormatException]：
  /// 不猜一個預設介面。
  Future<HostedSession> createHostedSession({
    required Uri returnUrl,
    required Uri refreshUrl,
    bool allowInsecureUrl = false,
  }) async {
    final response = await _client.createPaymentOnboardingSession(
      CreatePaymentOnboardingSessionRequest(
        returnUrl: returnUrl.toString(),
        refreshUrl: refreshUrl.toString(),
      ),
    );
    if (response.surface != OnboardingSurface.ONBOARDING_SURFACE_HOSTED) {
      throw FormatException('開通介面不是託管頁面：${response.surface}');
    }
    final url = Uri.tryParse(response.url);
    if (url == null ||
        !url.isScheme('https') && !(allowInsecureUrl && url.isScheme('http'))) {
      throw FormatException('開通連結無效：${response.url}');
    }
    return HostedSession(url);
  }
}

@Riverpod(keepAlive: true)
PaymentAccountRepository paymentAccountRepository(Ref ref) =>
    PaymentAccountRepository(ref.watch(sellerClientProvider));

/// [accountId] 這個帳號的收款帳戶（`GetMyPaymentAccount`）。還沒有帳戶是 null；其他失敗是 error 狀態，不當成沒有帳戶。
/// 以帳號為鍵，換帳號不會看到上一個帳號的結果。
@Riverpod(keepAlive: true, retry: noRetry)
Future<PaymentAccount?> accountPaymentAccount(Ref ref, String accountId) async {
  try {
    return await ref.watch(paymentAccountRepositoryProvider).fetch();
  } catch (error) {
    HKLog.logW('[PaymentAccount] GetMyPaymentAccount failed: $error');
    rethrow;
  }
}

/// 目前登入帳號的收款帳戶；沒有登入時是 null（外層）。
@Riverpod(keepAlive: true)
AsyncValue<PaymentAccount?>? currentPaymentAccount(Ref ref) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  if (accountId == null) return null;
  return ref.watch(accountPaymentAccountProvider(accountId));
}

/// 重讀目前帳號的收款帳戶（`GetMyPaymentAccount`）。
@Riverpod(keepAlive: true)
class PaymentAccountReader extends _$PaymentAccountReader {
  @override
  void build() {}

  /// 重讀並回傳結果；讀取失敗時丟出該錯誤（目前的值保持 error 狀態）。沒有登入時回 null。
  Future<PaymentAccount?> reload() async {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    if (accountId == null) return null;
    ref.invalidate(accountPaymentAccountProvider(accountId));
    return ref.read(accountPaymentAccountProvider(accountId).future);
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
