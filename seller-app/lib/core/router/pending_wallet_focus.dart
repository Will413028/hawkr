import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../features/seller/data/account_scope.dart';
import '../session/session_provider.dart';

part 'pending_wallet_focus.g.dart';

/// 錢包頁裡可以指定展開並捲到的區塊（[PendingWalletFocus] 的值）。
abstract final class WalletSections {
  static const foodRegistration = 'food-registration';
}

/// 要在錢包頁展開並捲到的區塊（見 `WalletSections`）。呼叫端（進度卡）在導覽到錢包之前設定，錢包套用時取走並清掉：
/// 每次導覽只套用一次，重建、還原網址都不會重播。
@Riverpod(keepAlive: true)
class PendingWalletFocus extends _$PendingWalletFocus {
  /// 換帳號（含登出）時清掉，沒套用的意圖不會帶到下一個帳號。
  @override
  String? build() {
    var account = signedInAccountId(ref.read(sessionProvider));
    ref.listen(sessionProvider, (_, next) {
      final now = signedInAccountId(next);
      if (now == account) return;
      account = now;
      state = null;
    });
    return null;
  }

  void request(String section) => state = section;

  void clear() => state = null;
}
