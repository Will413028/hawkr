import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/refresh/coalescing_refresher.dart';
import '../../../core/refresh/refresh_clock.dart';
import '../../../core/session/session_provider.dart';
import 'account_scope.dart';
import 'current_kyc.dart';
import 'current_store.dart';

part 'seller_status_refresh.g.dart';

/// 畫面因為成為可見而重讀時，這段時間內讀過就不再讀（website 的 react-query `staleTime: 30_000`）。
const statusStaleTime = Duration(seconds: 30);

/// 重讀賣家的伺服器狀態：店家（`GetMyStore`）與認證流程（`GetMyKYC`）一起，兩者不會一個新一個舊（PayUni 狀態是認證流程的一部分，隨 `GetMyKYC` 一起讀）。
@Riverpod(keepAlive: true)
class SellerStatusRefresh extends _$SellerStatusRefresh {
  @override
  void build() {}

  late final _refresher = CoalescingRefresher(
    clock: () => ref.read(refreshClockProvider)(),
    round: _round,
  );

  /// 最近一次成功的一輪（店家與認證流程都讀成功）已經超過 [maxAge]，或還沒有成功過。
  bool isStale(Duration maxAge) => _refresher.isStale(maxAge);

  /// 一輪重讀：店家、認證流程一起讀完才回傳，回傳店家有沒有讀成功（任何一個讀取失敗不影響另一個）。
  ///
  /// 進行中的一輪共用、[fresh] 的後續一輪、[maxAge] 的略過規則見 [CoalescingRefresher]。預設共用進行中的那一輪（回前景、
  /// 切回分頁、下拉重整、進度卡回來）；[fresh] 給推播、從認證流程回來；[maxAge] 只給「因為成為可見而重讀」的呼叫，
  /// 推播、回前景、重試、從認證流程回來都不帶。
  ///
  /// [settledOnly] 為真時只重讀已經有結果（值或錯誤）而且目前沒有在讀的項目：正在讀的項目已經在讀最新的，不重複讀。
  /// 畫面剛開啟或成為目前分頁時用它。
  Future<bool> refresh({
    bool fresh = false,
    bool settledOnly = false,
    Duration? maxAge,
  }) => _refresher.refresh(
    fresh: fresh,
    settledOnly: settledOnly,
    maxAge: maxAge,
  );

  Future<bool> _round(bool settledOnly) async {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    final storeReading =
        settledOnly && ref.read(currentStoreProvider).isLoading;
    final kycReading =
        settledOnly &&
        accountId != null &&
        ref.read(accountKycProvider(accountId)).isLoading;
    final results = await Future.wait<bool>([
      storeReading
          ? ref
                .read(currentStoreProvider.future)
                .then((result) => result != null && !result.failed)
          : ref.read(currentStoreProvider.notifier).refresh(),
      kycReading
          ? ref
                .read(accountKycProvider(accountId).future)
                .then((_) => true, onError: (_) => false)
          : ref.read(kycReaderProvider.notifier).refresh(),
    ]);
    if (results.every((ok) => ok)) {
      _refresher.markSucceeded();
    }
    return results.first;
  }
}
