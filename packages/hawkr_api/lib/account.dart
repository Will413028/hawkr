/// 帳號相關流程的共用邏輯（identity 的 `IdentityAppService`）：兩支 App 共用的判斷在這裡，文案留在各 App。
library;

import 'package:connectrpc/connect.dart';

import 'errors.dart';
import 'gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'submit.dart';

/// [handOverAppleCredential] 的結果。
enum AppleHandOver {
  /// 伺服器已收下，換 token 與保存在背景執行。
  linked,

  /// 伺服器回 unavailable：Apple 沒有設定（本機開發），或 Apple 暫時不可用。不是錯誤，下一次原生登入會再交。
  unavailable,

  /// 其他失敗（網路、token 無效等）。不影響登入，下一次原生登入會再交。
  failed,
}

/// Apple 原生登入成功、帳號建立之後，把 identity token 與 authorization code 交給伺服器（`LinkAppleCredential`）。
/// 不丟例外：呼叫端依結果記 log，不能因此阻擋登入。
Future<({AppleHandOver result, Object? error})> handOverAppleCredential(
  IdentityAppServiceClient client, {
  required String identityToken,
  required String authorizationCode,
}) async {
  try {
    await client.linkAppleCredential(
      LinkAppleCredentialRequest(
        idToken: identityToken,
        authorizationCode: authorizationCode,
      ),
    );
    return (result: AppleHandOver.linked, error: null);
  } catch (error) {
    final code = ConnectException.from(error).code;
    return (
      result: code == Code.unavailable
          ? AppleHandOver.unavailable
          : AppleHandOver.failed,
      error: error,
    );
  }
}

/// `DeleteAccount` 的結果。顯示的文字由各 App 決定。
sealed class AccountDeletionResult {
  const AccountDeletionResult();
}

/// 帳號已刪除（含沒收到回應、之後確認 token 已失效），呼叫端登出。
final class AccountDeletionDeleted extends AccountDeletionResult {
  const AccountDeletionDeleted();
}

/// 還有未完成的訂單，最後一筆完成後自動刪除。
final class AccountDeletionPendingOrders extends AccountDeletionResult {
  const AccountDeletionPendingOrders();
}

/// 帳號還有沒關閉的店家（`ACCOUNT_HAS_OPEN_STORE`）。
final class AccountDeletionHasOpenStore extends AccountDeletionResult {
  const AccountDeletionHasOpenStore();
}

/// 沒收到回應，確認之後帳號還在：請使用者再試一次。
final class AccountDeletionNotDeleted extends AccountDeletionResult {
  const AccountDeletionNotDeleted();
}

/// 沒收到回應，確認也沒收到回應：不知道帳號有沒有刪除。
final class AccountDeletionUnconfirmed extends AccountDeletionResult {
  const AccountDeletionUnconfirmed();
}

/// 其他失敗。[message] 是伺服器給使用者的文字，沒有時為 null（呼叫端用自己的預設文字）。
final class AccountDeletionFailure extends AccountDeletionResult {
  const AccountDeletionFailure(this.message);

  final String? message;
}

/// 呼叫 [deleteAccount] 並把結果分類。沒收到回應或逾時時不知道伺服器有沒有處理，不自動重送：以 [accountExists]
/// （GetSession；token 已失效時回 false）確認帳號有沒有被刪掉。[onError] 收到每一次失敗的原始例外，給呼叫端記 log。
/// [defaults] 是沒有伺服器文字時的 code 預設文字（見 [submitError]）。
Future<AccountDeletionResult> classifyAccountDeletion({
  required Future<DeleteAccountResponse_DeletionState> Function() deleteAccount,
  required Future<bool> Function() accountExists,
  required DefaultErrorText defaults,
  void Function(String step, Object error)? onError,
}) async {
  try {
    return switch (await deleteAccount()) {
      DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED =>
        const AccountDeletionDeleted(),
      DeleteAccountResponse_DeletionState.DELETION_STATE_PENDING_ORDERS =>
        const AccountDeletionPendingOrders(),
      _ => const AccountDeletionFailure(null),
    };
  } catch (error) {
    onError?.call('DeleteAccount', error);
    final facts = ErrorFacts.fromException(ConnectException.from(error));
    if (_outcomeUnknown(facts)) {
      return _confirmDeleted(accountExists, defaults, onError);
    }
    final decision = classifyError(facts);
    if (decision.action == ErrorAction.reason &&
        decision.reason == ErrorReason.ACCOUNT_HAS_OPEN_STORE) {
      return const AccountDeletionHasOpenStore();
    }
    return AccountDeletionFailure(_message(error, defaults));
  }
}

Future<AccountDeletionResult> _confirmDeleted(
  Future<bool> Function() accountExists,
  DefaultErrorText defaults,
  void Function(String step, Object error)? onError,
) async {
  try {
    return await accountExists()
        ? const AccountDeletionNotDeleted()
        : const AccountDeletionDeleted();
  } catch (error) {
    onError?.call('GetSession', error);
    if (_outcomeUnknown(
      ErrorFacts.fromException(ConnectException.from(error)),
    )) {
      return const AccountDeletionUnconfirmed();
    }
    final message = _message(error, defaults);
    return message == null
        ? const AccountDeletionUnconfirmed()
        : AccountDeletionFailure(message);
  }
}

String? _message(Object error, DefaultErrorText defaults) =>
    switch (submitError(error, defaults: defaults)) {
      SubmitMessage(:final message) => message,
      _ => null,
    };

/// 不知道伺服器有沒有處理：沒收到回應，或逾時（client 的 deadline 沒有 cause，伺服器送來的 deadline_exceeded 也一樣
/// 不知道結果）。
bool _outcomeUnknown(ErrorFacts facts) =>
    facts.noResponse || facts.code == Code.deadlineExceeded;
