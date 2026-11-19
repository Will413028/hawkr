import 'dart:async';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../features/auth/data/auth_repository.dart';
import '../../shared/utils/hk_log.dart';
import 'auth_session_provider.dart';

part 'account_restriction.g.dart';

/// 帳號的停權狀態（`GetSession().suspension`）。停權的帳號仍然登入、可瀏覽，寫入被伺服器擋下（A7-D17）。
class AccountSuspension {
  const AccountSuspension({
    this.suspendedAt,
    this.category = PolicyCategory.POLICY_CATEGORY_UNSPECIFIED,
    this.reason = '',
  });

  /// 只知道帳號被停權、還沒讀到細節（請求收到 `ACCOUNT_SUSPENDED`，GetSession 尚未回來）。
  const AccountSuspension.unknown() : this();

  final DateTime? suspendedAt;
  final PolicyCategory category;
  final String reason;
}

/// 受限狀態：null 是沒有停權。進入受限狀態不登出，session 與 token 都保留。
///
/// 來源有二：GetSession 的 `suspension`（登入與冷啟動），以及請求收到 `ACCOUNT_SUSPENDED`（登入期間才被停權）。
@Riverpod(keepAlive: true)
class AccountRestriction extends _$AccountRestriction {
  @override
  AccountSuspension? build() => null;

  /// 每次改寫狀態（登入、登出、GetSession 結果、請求被停權擋下）就加一；補讀細節的請求回來時，世代已變就丟棄，
  /// 避免登出後、換帳號後或較晚發出的讀取之後，把舊結果寫回。最後發出的讀取勝出。
  int _generation = 0;

  /// GetSession 的結果：有 `suspension` 就是受限，沒有就解除。
  void apply(AccountSuspension? suspension) {
    _generation++;
    state = suspension;
  }

  void clear() => apply(null);

  /// 使用者主動的寫入（Connect 或舊 REST）收到 `ACCOUNT_SUSPENDED` 的唯一入口：進入受限狀態、請畫面顯示
  /// 「帳號停權中」的提示，並補讀細節。沒有保存 token 時忽略（訪客不會被停權；登出後才回來的訊號也不算）；
  /// 以保存的 token 判斷而不是已載入的 session，登入與冷啟動還在載入 session 時收到的訊號也算。
  Future<void> rejected() async {
    final bool signedIn;
    try {
      signedIn = (await ref.read(authSessionProvider.future)).isSignedIn;
    } catch (_) {
      return;
    }
    if (!ref.mounted || !signedIn) return;
    final generation = ++_generation;
    state ??= const AccountSuspension.unknown();
    ref.read(suspendedActionNoticeProvider.notifier).raise();
    unawaited(_refresh(generation));
  }

  /// 讀取失敗（GetSession 失敗）時保留原狀態；讀到的結果只在世代沒變時套用，`suspension` 為 null 是讀到了且沒有停權。
  Future<void> _refresh(int generation) async {
    try {
      final suspension =
          (await ref.read(authRepositoryProvider).currentAccount()).suspension;
      if (!ref.mounted || generation != _generation) return;
      state = suspension;
    } catch (error) {
      HKLog.logD('[AccountRestriction] GetSession failed: $error');
    }
  }
}

/// 「帳號停權中，無法執行這個動作」提示的請求；每次 [raise] 加一，監聽的 widget 依變動顯示 bottom sheet。
@Riverpod(keepAlive: true)
class SuspendedActionNotice extends _$SuspendedActionNotice {
  @override
  int build() => 0;

  void raise() => state++;
}
