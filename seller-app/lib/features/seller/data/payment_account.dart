import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import 'account_scope.dart';

part 'payment_account.g.dart';

/// 收款帳戶卡的色調（版面，留在 client）；文字（狀態徽章）取自共用文案目錄
/// （`payment.PaymentAccountStatus`），這裡沒有「值 → 文字」的表。`UNSPECIFIED` 與不認得的值沒有色調（null）。
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

/// 平台審核還沒通過時，收款帳戶區的提示（沒有收款帳戶可看）。
const paymentAccountNotYetOpenNotice = '平台審核通過後開放設定';

/// 一次 `CreatePaymentOnboardingSession`（目的地是賣家 App）的結果：[OnboardingSession.url] 是要在外部瀏覽器開的託管連結。
/// App 不宣告內嵌執行環境，伺服器一律回託管連結。
final class OnboardingSession {
  const OnboardingSession(this.url);

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

  /// 同步向金流商重讀收款帳戶並更新伺服器的快取（進入認證頁、從金流商的頁面回來、App 回前景時，先呼叫它再讀開通摘要）。失敗丟出該錯誤，呼叫端不顯示。
  /// 方法是 IDEMPOTENT：`resource_exhausted` 帶 RetryInfo 與 `unavailable` 由 transport 的 `retryInterceptor` 重試，這裡不重試。
  Future<void> refresh() async {
    await _client.refreshMyPaymentAccount(RefreshMyPaymentAccountRequest());
  }

  /// 要在瀏覽器開啟的託管連結（一次性、短效，完成後回跳到網站給 App 的回跳頁），網址由伺服器組；只收 https，[allowInsecureUrl]（只有 dev 的本機測試）才收 http。
  /// 不宣告內嵌執行環境（空清單），語系由 transport 的 `Accept-Language` 帶。回應不是 `hosted`（內嵌或兩支都沒設）、沒有網址或網址無效都是契約被違反，丟 [FormatException]。
  Future<OnboardingSession> createSession({
    bool allowInsecureUrl = false,
  }) async {
    final response = await _client.createPaymentOnboardingSession(
      CreatePaymentOnboardingSessionRequest(
        destination: OnboardingReturnDestination
            .ONBOARDING_RETURN_DESTINATION_SELLER_APP,
      ),
    );
    if (!response.hasHosted()) {
      throw const FormatException('開通 session 不是託管連結');
    }
    final raw = response.hosted.url;
    final url = Uri.tryParse(raw);
    if (url == null ||
        !url.isScheme('https') && !(allowInsecureUrl && url.isScheme('http'))) {
      throw FormatException('開通連結無效：$raw');
    }
    return OnboardingSession(url);
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
