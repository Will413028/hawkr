import 'package:hawkr_api/errors.dart' show DefaultErrorText, ErrorFacts;
import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/submit.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'account_scope.dart';
import 'current_store.dart';
import 'my_store_repository.dart';
import 'store_editor.dart';
import 'version_conflict.dart';

part 'seller_onboarding.g.dart';

const storeCreatedElsewhereMessage = '店家資料已在其他地方建立，已載入最新版本，你輸入的內容仍保留，請確認後再送出。';
const agreementOutdatedMessage = '條款已更新，請重新閱讀後再勾選。';

/// 帳號此刻不能開店的原因（伺服器的判斷，client 只用來分支）：來自 `GetMyStoreOpening` 的 `unavailabilities` 的 `reason`，
/// 或 ApplyStore 被擋下時的 `STORE_OPENING_*` ErrorReason（後備）。顯示的文字是伺服器給的 `LocalizedMessage`，不在 client 對照。
enum StoreOpeningRefusal {
  /// `TERMINATED`／`STORE_OPENING_TERMINATED`：帳號有生效中的終止合作（條款第 21.3 條）。
  terminated(offersAppeal: true),

  /// `SUSPENDED`／`STORE_OPENING_SUSPENDED`：帳號停權中。
  suspended(offersAppeal: true),

  /// `ACCOUNT_DELETING`：帳號已受理刪除申請、等待未結訂單結束；沒有可申覆的措施。
  accountDeleting(offersAppeal: false),

  /// `STORE_EXISTS`：帳號已經有目前的店家。呼叫端只在沒有店家時才問，所以出現就代表資料不同步，照伺服器的答案擋下。
  storeExists(offersAppeal: false);

  const StoreOpeningRefusal({required this.offersAppeal});

  /// 是否帶「措施與申覆」入口（終止合作與停權是措施，帳號刪除不是）。
  final bool offersAppeal;
}

/// 開店被擋下時給賣家看的：[refusal] 是原因（只用來決定帶不帶申覆入口），[message] 是伺服器給的一句話。
/// 原因是 UNSPECIFIED 或這個版本不認得的值時 [refusal] 為 null（不帶入口，有 [message] 仍顯示）；伺服器沒有句子時 [message] 為 null
/// （只擋、不顯示文字，client 不自己組字）。
final class StoreOpeningBlock {
  const StoreOpeningBlock({this.refusal, this.message});

  final StoreOpeningRefusal? refusal;
  final String? message;
}

/// `GetMyStoreOpening` 的答案是否擋下「送出開店表單」：`can_apply_store` 為 false 一律擋（回非 null），可以開店回 null。
///
/// 顯示的是伺服器依優先序排好的第一筆（不重排）：[StoreOpeningBlock.message] 是它的句子，[StoreOpeningBlock.refusal] 是它的 reason。
/// 只在帳號沒有目前店家時呼叫（有店家走更新流程，不問）。
StoreOpeningBlock? storeOpeningBlock(GetMyStoreOpeningResponse response) {
  if (response.canApplyStore) return null;
  if (response.unavailabilities.isEmpty) return const StoreOpeningBlock();
  final first = response.unavailabilities.first;
  return StoreOpeningBlock(
    refusal: switch (first.reason) {
      StoreOpeningUnavailableReason
          .STORE_OPENING_UNAVAILABLE_REASON_TERMINATED =>
        StoreOpeningRefusal.terminated,
      StoreOpeningUnavailableReason
          .STORE_OPENING_UNAVAILABLE_REASON_SUSPENDED =>
        StoreOpeningRefusal.suspended,
      StoreOpeningUnavailableReason
          .STORE_OPENING_UNAVAILABLE_REASON_ACCOUNT_DELETING =>
        StoreOpeningRefusal.accountDeleting,
      StoreOpeningUnavailableReason
          .STORE_OPENING_UNAVAILABLE_REASON_STORE_EXISTS =>
        StoreOpeningRefusal.storeExists,
      _ => null,
    },
    message: first.hasMessage() && first.message.message.isNotEmpty
        ? first.message.message
        : null,
  );
}

/// [accountId] 這個帳號在開店表單送出之前先問伺服器能不能開店（帳號層級；以帳號為鍵，換帳號不會看到上一個帳號的答案）。讀取失敗是 error 狀態，畫面不擋，送出時由 ApplyStore 的
/// ErrorReason 後備。沒有畫面在看時釋放，回到表單時重讀。
@Riverpod(retry: noRetry)
Future<StoreOpeningBlock?> storeOpeningBlockFor(
  Ref ref,
  String accountId,
) async {
  final response = await ref
      .watch(sellerClientProvider)
      .getMyStoreOpening(GetMyStoreOpeningRequest());
  return storeOpeningBlock(response);
}

sealed class OnboardingOutcome {
  const OnboardingOutcome();
}

final class OnboardingCompleted extends OnboardingOutcome {
  const OnboardingCompleted({required this.created});

  final bool created;
}

/// 沒有店家卻沒帶賣家契約的同意。
final class OnboardingTermsRequired extends OnboardingOutcome {
  const OnboardingTermsRequired();
}

/// 賣家同意的版本已不是現行版本：呼叫端重讀條款、取消勾選。
final class OnboardingAgreementOutdated extends OnboardingOutcome {
  const OnboardingAgreementOutdated();
}

/// 店家在這之間被別處建立或更新，已重讀；輸入保留，確認後再送出。
final class OnboardingConflict extends OnboardingOutcome {
  const OnboardingConflict(this.message);

  final String message;
}

/// 伺服器因停權或終止合作拒絕開店：顯示錯誤的 `LocalizedMessage`（[message]，沒有時不顯示文字）並導向「措施與申覆」；沒有可重試的動作。
final class OnboardingStoreOpeningRefused extends OnboardingOutcome {
  const OnboardingStoreOpeningRefused(this.refusal, this.message);

  final StoreOpeningRefusal refusal;
  final String? message;
}

final class OnboardingFailed extends OnboardingOutcome {
  const OnboardingFailed(this.message);

  final String message;
}

/// 開店表單送出：先問伺服器有沒有店家，沒有就 `ApplyStore`（帶賣家契約的同意），有就只更新填了的欄位
/// （`UpdateMyStore`），最後以最新的 etag 呼叫 `CompleteMyStoreOnboarding`。
class SellerOnboarding {
  SellerOnboarding({
    required SellerAppServiceClient client,
    required MyStoreRepository stores,
    required Future<bool> Function() reload,
    required DefaultErrorText defaults,
  }) : _client = client,
       _stores = stores,
       _reload = reload,
       _defaults = defaults;

  final SellerAppServiceClient _client;
  final MyStoreRepository _stores;
  final Future<bool> Function() _reload;
  final DefaultErrorText _defaults;

  Future<OnboardingOutcome> submit({
    required String storeName,
    required String townCode,
    required String addressDetail,
    AgreementAcceptance? acceptance,
  }) async {
    final existing = await _existing();
    if (existing is OnboardingOutcome) return existing;
    final store = existing as Store?;
    if (store == null && acceptance == null) {
      return const OnboardingTermsRequired();
    }
    final address = PostalAddress(
      townCode: townCode,
      addressDetail: addressDetail,
    );
    try {
      final Store written;
      if (store == null) {
        written = (await _client.applyStore(
          ApplyStoreRequest(
            storeName: storeName,
            address: address,
            acceptedAgreements: [acceptance!],
          ),
        )).store;
      } else {
        written = (await _client.updateMyStore(
          UpdateMyStoreRequest(
            store: Store(
              etag: store.etag,
              storeName: storeName,
              address: address,
            ),
            updateMask: fieldMask(Store(), ['store_name', 'address']),
          ),
        )).store;
      }
      await _client.completeMyStoreOnboarding(
        CompleteMyStoreOnboardingRequest(etag: written.etag),
      );
      await _reload();
      return OnboardingCompleted(created: store == null);
    } catch (error) {
      return _failure(error);
    }
  }

  Future<Object?> _existing() async {
    try {
      return (await _stores.fetch()).store;
    } on MyStoreUnavailable catch (error) {
      HKLog.logW('[SellerOnboarding] store state unavailable: $error');
      return const OnboardingFailed(storeStateUnavailableNotice);
    }
  }

  Future<OnboardingOutcome> _failure(Object error) async {
    HKLog.logW('[SellerOnboarding] failed: $error');
    if (hasErrorReason(error, ErrorReason.AGREEMENT_OUTDATED)) {
      return const OnboardingAgreementOutdated();
    }
    if (hasErrorReason(error, ErrorReason.STORE_OPENING_TERMINATED)) {
      return OnboardingStoreOpeningRefused(
        StoreOpeningRefusal.terminated,
        _errorMessage(error),
      );
    }
    if (hasErrorReason(error, ErrorReason.STORE_OPENING_SUSPENDED)) {
      return OnboardingStoreOpeningRefused(
        StoreOpeningRefusal.suspended,
        _errorMessage(error),
      );
    }
    if (ConnectException.from(error).code == Code.alreadyExists) {
      return await _conflict(storeCreatedElsewhereMessage);
    }
    switch (submitError(error, defaults: _defaults)) {
      case SubmitConflict():
        return await _conflict(versionConflictNotice('店家資料', '送出'));
      case SubmitFields(:final fields, :final message):
        return OnboardingFailed(message ?? fields.values.first);
      case SubmitMessage(:final message):
        return OnboardingFailed(message);
    }
  }

  /// 錯誤的 `LocalizedMessage`；沒有或是空字串時是 null。
  String? _errorMessage(Object error) {
    final text = ErrorFacts.fromException(
      ConnectException.from(error),
    ).localizedMessage;
    return text == null || text.isEmpty ? null : text;
  }

  Future<OnboardingOutcome> _conflict(String message) async => await _reload()
      ? OnboardingConflict(message)
      : const OnboardingFailed(versionReloadFailedNotice);
}

@Riverpod(keepAlive: true)
SellerOnboarding sellerOnboarding(Ref ref) => SellerOnboarding(
  client: ref.watch(sellerClientProvider),
  stores: ref.watch(myStoreRepositoryProvider),
  reload: () => ref.read(currentStoreProvider.notifier).refresh(),
  defaults: ref.watch(defaultErrorTextProvider),
);
