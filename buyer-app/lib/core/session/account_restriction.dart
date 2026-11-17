import 'dart:async';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../features/auth/data/auth_repository.dart';
import '../../shared/utils/hk_log.dart';

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

  /// 每次整體改寫狀態（登入、登出、GetSession 結果）就加一；補讀細節的請求回來時，世代變了就丟棄，
  /// 避免登出後或換帳號後把舊帳號的結果寫回。
  int _generation = 0;

  /// GetSession 的結果：有 `suspension` 就是受限，沒有就解除。
  void apply(AccountSuspension? suspension) {
    _generation++;
    state = suspension;
  }

  void clear() => apply(null);

  /// 使用者主動的寫入收到 `ACCOUNT_SUSPENDED`：進入受限狀態、請畫面顯示「帳號停權中」的提示，並補讀細節。
  void rejected() {
    state ??= const AccountSuspension.unknown();
    ref.read(suspendedActionNoticeProvider.notifier).raise();
    unawaited(_refresh());
  }

  /// 舊 REST 的寫入收到 403：回應沒有 reason，不能直接當成停權，以 GetSession 確認，確定停權才同 [rejected]。
  Future<void> confirmRejected() async {
    final result = await _fetch();
    if (result == null) return;
    state = result.suspension;
    if (result.suspension != null) {
      ref.read(suspendedActionNoticeProvider.notifier).raise();
    }
  }

  Future<void> _refresh() async {
    final result = await _fetch();
    if (result == null) return;
    state = result.suspension;
  }

  /// null 是這次沒有讀到（GetSession 失敗，或讀取期間 session 已換）；`suspension` 為 null 的結果是
  /// 讀到了且沒有停權。
  Future<({AccountSuspension? suspension})?> _fetch() async {
    final generation = _generation;
    try {
      final suspension =
          (await ref.read(authRepositoryProvider).currentAccount()).suspension;
      if (!ref.mounted || generation != _generation) return null;
      return (suspension: suspension);
    } catch (error) {
      HKLog.logD('[AccountRestriction] GetSession failed: $error');
      return null;
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
