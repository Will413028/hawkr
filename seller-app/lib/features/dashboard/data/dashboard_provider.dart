import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/refresh/coalescing_refresher.dart';
import '../../../core/refresh/refresh_clock.dart';
import '../../../shared/utils/hk_log.dart';
import '../../seller/data/account_scope.dart';
import '../../seller/data/current_store.dart';

part 'dashboard_provider.g.dart';

/// [storeId] 這間店家的儀表板（`GetMyDashboard`）。`GetMyDashboard` 讀的是目前的店家，所以以店家 ID 為鍵：同一個帳號
/// 關店再開店（店家 ID 改變）也不會看到上一間店的數字，也不會被 30 秒的保鮮期略過重讀。
/// [currentDashboard]（keepAlive）在目前有店家期間一直監看這個 provider，所以它在這期間都保留（沒有畫面時
/// 也不釋放，讀到的值留給下次開啟）；換店家、換帳號、登出時才釋放（新的店家從讀取中開始，不先顯示上一間店的數字）。
///
/// 讀取失敗而且已經有值時，狀態是帶著上一次的值的 error（`AsyncValue.value` 仍是上一次的值）：畫面繼續顯示它。
/// 沒有輪詢，也不以 silent push 為依據：畫面在可見、回前景、收到訂單推播、下拉時用 [refresh] 重讀
/// （A3b 計畫第 4 刀的新鮮度決定）。四個區塊讀不到時伺服器不設，不是 0，所以這裡不做任何補值。
///
/// 每一次讀取都經過 [CoalescingRefresher]：第一次與依賴改變（例如 client 換了）的重建把自己那一次讀取登記成進行中的
/// 一輪，登記永遠指向最新的那次讀取；被取代的讀取不會改變狀態，也不會記成功（不影響保鮮期）。
@Riverpod(retry: noRetry)
class StoreDashboard extends _$StoreDashboard {
  late final _refresher = CoalescingRefresher(
    clock: () => ref.read(refreshClockProvider)(),
    round: (_) => _reload(),
  );

  /// 第幾次 build；讀取開始時記下，結束時不是最新的就是被取代（或已釋放）。
  var _generation = 0;

  /// [_reload] 要求的重建：這一次 build 已經由那一輪登記，不再另外登記。
  var _reloadRequested = false;

  @override
  Future<Dashboard> build(String storeId) {
    final client = ref.watch(sellerClientProvider);
    final generation = ++_generation;
    ref.onDispose(() => _generation++);
    final read = _read(client, generation);
    if (_reloadRequested) {
      _reloadRequested = false;
    } else {
      _refresher.adopt(read.then((_) => true, onError: (_) => false));
    }
    return read;
  }

  Future<Dashboard> _read(SellerAppServiceClient client, int generation) async {
    try {
      final response = await client.getMyDashboard(GetMyDashboardRequest());
      if (!response.hasDashboard()) {
        throw StateError('GetMyDashboard 的回應沒有儀表板');
      }
      // 被取代的讀取（之後又 build 過、或已釋放）的結果不算：狀態由最新的那次決定，保鮮期也只看它。
      if (generation == _generation) _refresher.markSucceeded();
      return response.dashboard;
    } catch (error) {
      HKLog.logW('[Dashboard] GetMyDashboard failed: $error');
      rethrow;
    }
  }

  /// 重讀儀表板一輪，回傳這一輪有沒有讀成功。已有一輪進行中（包含第一次讀取）時共用，[fresh]（訂單推播）排一輪接在
  /// 後面，[maxAge] 只給「因為成為可見而重讀」的呼叫，規則見 [CoalescingRefresher]；下拉、回前景與重試都不帶 [maxAge]。
  Future<bool> refresh({bool fresh = false, Duration? maxAge}) =>
      _refresher.refresh(fresh: fresh, maxAge: maxAge);

  Future<bool> _reload() async {
    _reloadRequested = true;
    ref.invalidateSelf();
    try {
      await future;
      return true;
    } catch (_) {
      return false;
    }
  }
}

/// 目前店家的儀表板；沒有登入、還沒讀到店家或沒有店家時是 null（和值是 null 的 `AsyncData` 不同）。
@Riverpod(keepAlive: true)
AsyncValue<Dashboard>? currentDashboard(Ref ref) {
  final storeId = ref.watch(sellerStoreIdProvider);
  if (storeId == null) return null;
  return ref.watch(storeDashboardProvider(storeId));
}
