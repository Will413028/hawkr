import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' show Store;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart'
    show SellerCapability;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/my_store.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/session/store_refresh_signal.dart';
import '../../../shared/utils/hk_log.dart';
import 'my_store_repository.dart';
import 'account_scope.dart';

part 'current_store.g.dart';

/// 一次 `GetMyStore` 讀取的結果，屬於 [accountId] 這個帳號。
///
/// 三種結果：有店家（[store] 有值）、沒有店家（[store] 是 [MyStore.absent]）、讀取失敗（[error] 有值）。
/// 讀取失敗時 [store] 是同一個帳號上一次成功的快照；沒有快照就是 null，這時不知道有沒有店家。
final class MyStoreResult {
  const MyStoreResult.loaded(this.accountId, MyStore this.store) : error = null;

  const MyStoreResult.failed(this.accountId, Object this.error, {this.store});

  final String accountId;
  final MyStore? store;
  final Object? error;

  bool get failed => error != null;
}

/// 目前登入帳號的店家（`GetMyStore`）：伺服器端的狀態獨立於 session（session 只管登入的身分）。
///
/// 值是 `null` 表示沒有登入；帳號改變時自動重讀，舊帳號的讀取結果與較舊的讀取都不會寫進來。
/// 重讀失敗時保留同一帳號上一次成功的快照。
@Riverpod(keepAlive: true)
class CurrentStore extends _$CurrentStore {
  int _generation = 0;
  bool _reading = false;
  String? _snapshotAccount;
  MyStore? _snapshot;

  @override
  Future<MyStoreResult?> build() async {
    final accountId = signedInAccountId(ref.watch(sessionProvider));
    final generation = ++_generation;
    ref.listen(storeRefreshSignalProvider, (_, _) {
      if (!_reading) ref.invalidateSelf();
    });
    if (accountId == null) {
      _snapshotAccount = null;
      _snapshot = null;
      return null;
    }
    if (_snapshotAccount != accountId) {
      _snapshotAccount = accountId;
      _snapshot = null;
    }
    _reading = true;
    try {
      final store = await ref.read(myStoreRepositoryProvider).fetch();
      if (generation == _generation) {
        _snapshot = store;
      }
      HKLog.logD(
        '[CurrentStore] storeId=${store.storeId}, '
        'suspended=${store.isSuspended}, pendingDeletion=${store.isPendingDeletion}',
      );
      return MyStoreResult.loaded(accountId, store);
    } catch (e) {
      HKLog.logE('[CurrentStore] GetMyStore failed: $e');
      return MyStoreResult.failed(
        accountId,
        e,
        store: generation == _generation ? _snapshot : null,
      );
    } finally {
      if (generation == _generation) _reading = false;
    }
  }

  /// 寫入成功後立刻採用伺服器回的店家（含新的 etag）；之後的重讀只是校正。沒有快照、帳號已變、沒有店家或店家 id 不同時不動。
  void adoptWritten(Store written) {
    final current = _snapshot;
    final result = state.value;
    if (current == null || !current.hasStore || result == null) return;
    if (result.accountId != _snapshotAccount) return;
    if (written.id != current.storeId) return;
    final next = current.replaceStore(written);
    _snapshot = next;
    state = AsyncData(MyStoreResult.loaded(result.accountId, next));
  }

  /// 關店成功後直接採用「沒有店家」，不等重讀（重讀失敗時也不會留下舊的開店快照）。帳號已變或還沒有結果時不動。
  void adoptClosed() {
    final result = state.value;
    if (result == null || result.accountId != _snapshotAccount) return;
    const next = MyStore.absent();
    _snapshot = next;
    state = AsyncData(MyStoreResult.loaded(result.accountId, next));
  }

  /// 重新讀取店家，完成後回傳這次讀取有沒有成功。
  Future<bool> refresh() async {
    ref.invalidateSelf();
    final result = await future;
    return result != null && !result.failed;
  }
}

/// 目前帳號的讀取結果；還在讀、沒有登入、或結果屬於別的帳號時是 null。
MyStoreResult? _currentResult(Ref ref) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  final result = ref.watch(currentStoreProvider).value;
  return result != null && result.accountId == accountId ? result : null;
}

/// 最近一次成功的店家快照；還沒讀到、讀取失敗過而沒有快照、沒有登入時是 null（不代表沒有店家）。
@Riverpod(keepAlive: true)
MyStore? myStoreSnapshot(Ref ref) => _currentResult(ref)?.store;

/// 店家 ID；沒有店家或還沒讀到時是 null。
@Riverpod(keepAlive: true)
String? sellerStoreId(Ref ref) => ref.watch(myStoreSnapshotProvider)?.storeId;

/// 需要完成賣家 onboarding（沒有店家，或店家沒有完成新手引導）。`null` 表示還不知道：還在讀，或讀取失敗而沒有快照。
@Riverpod(keepAlive: true)
bool? needsOnboarding(Ref ref) =>
    ref.watch(myStoreSnapshotProvider)?.needsOnboarding;

/// 賣家端的分享入口現在能不能用：店家公開（`PUBLIC_LISTING` 啟用）且有店面網址碼時才能分享。
sealed class StoreShare {
  const StoreShare();
}

/// 還沒讀到店家，或讀取失敗而沒有快照；也包含公開了卻沒有網址碼的不一致狀態。
final class StoreShareUnknown extends StoreShare {
  const StoreShareUnknown();
}

/// 店家讀到了，但現在不能公開。
final class StoreShareDisabled extends StoreShare {
  const StoreShareDisabled();
}

final class StoreShareEnabled extends StoreShare {
  const StoreShareEnabled(this.storeCode);

  final String storeCode;
}

/// 所有分享入口與「店家公開後…」提示共用的唯一判斷。
@Riverpod(keepAlive: true)
StoreShare storeShare(Ref ref) {
  final store = ref.watch(myStoreSnapshotProvider);
  if (store == null) return const StoreShareUnknown();
  if (!store.can(SellerCapability.SELLER_CAPABILITY_PUBLIC_LISTING)) {
    return const StoreShareDisabled();
  }
  final code = store.storeCode;
  return code == null ? const StoreShareUnknown() : StoreShareEnabled(code);
}

/// 讀不到店家、也沒有上一次的快照：不知道有沒有店家，router 顯示重試而不是主畫面。
@Riverpod(keepAlive: true)
bool storeUnavailable(Ref ref) {
  final result = _currentResult(ref);
  return result != null && result.failed && result.store == null;
}
