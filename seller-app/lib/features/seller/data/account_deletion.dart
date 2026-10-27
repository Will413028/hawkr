import 'package:hawkr_api/account.dart';
import 'package:hawkr_api/errors.dart' show DefaultErrorText;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../shared/utils/hk_log.dart';
import '../../auth/data/auth_repository.dart';

part 'account_deletion.g.dart';

const accountHasOpenStoreMessage = '你的帳號還有商店，請先關閉商店，再刪除帳號。';
const accountPendingOrdersMessage = '帳號刪除處理中，最後一筆未完成的訂單完成後會自動刪除；期間無法新增收藏與追蹤。';
const accountNotDeletedMessage = '帳號尚未刪除，請再試一次。';
const accountDeletionUnconfirmedMessage = '無法確認帳號是否已刪除，請檢查網路後再試一次。';
const deleteAccountFailedMessage = '刪除帳號失敗，請稍後再試';

sealed class AccountDeletionOutcome {
  const AccountDeletionOutcome();
}

/// 帳號已刪除，呼叫端登出。
final class AccountDeleted extends AccountDeletionOutcome {
  const AccountDeleted();
}

/// 還有未完成的訂單，最後一筆完成後自動刪除。
final class AccountDeletionPending extends AccountDeletionOutcome {
  const AccountDeletionPending();
}

/// 帳號還有沒關閉的店家：要先關店。
final class AccountHasOpenStore extends AccountDeletionOutcome {
  const AccountHasOpenStore();
}

final class AccountDeletionFailed extends AccountDeletionOutcome {
  const AccountDeletionFailed(this.message);

  final String message;
}

/// 刪除帳號：呼叫 `DeleteAccount`，結果由共用的 [classifyAccountDeletion] 分類（沒收到回應時以 `GetSession` 確認帳號有沒有
/// 被刪掉），這裡只對應成這個 App 的結果與文案。店家是否還開著由伺服器擋（`ACCOUNT_HAS_OPEN_STORE`）。
class AccountDeletion {
  AccountDeletion({
    required AuthRepository auth,
    required DefaultErrorText defaults,
  }) : _auth = auth,
       _defaults = defaults;

  final AuthRepository _auth;
  final DefaultErrorText _defaults;

  Future<AccountDeletionOutcome> run() async {
    final result = await classifyAccountDeletion(
      deleteAccount: _auth.deleteAccount,
      accountExists: _auth.accountExists,
      defaults: _defaults,
      onError: (step, error) =>
          HKLog.logW('[AccountDeletion] $step failed: $error'),
    );
    return switch (result) {
      AccountDeletionDeleted() => const AccountDeleted(),
      AccountDeletionPendingOrders() => const AccountDeletionPending(),
      AccountDeletionHasOpenStore() => const AccountHasOpenStore(),
      AccountDeletionNotDeleted() => const AccountDeletionFailed(
        accountNotDeletedMessage,
      ),
      AccountDeletionUnconfirmed() => const AccountDeletionFailed(
        accountDeletionUnconfirmedMessage,
      ),
      AccountDeletionFailure(:final message) => AccountDeletionFailed(
        message ?? deleteAccountFailedMessage,
      ),
    };
  }
}

@Riverpod(keepAlive: true)
AccountDeletion accountDeletion(Ref ref) => AccountDeletion(
  auth: ref.watch(authRepositoryProvider),
  defaults: ref.watch(defaultErrorTextProvider),
);
