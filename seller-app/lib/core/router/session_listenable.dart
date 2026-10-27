import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart'
    show CheckAppVersionResponse;
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/app_update/application/force_update_provider.dart';
import '../../features/seller/data/current_store.dart';
import '../session/session_provider.dart';

/// 橋接 Riverpod provider → ChangeNotifier (GoRouter refreshListenable)
///
/// GoRouter 的 `refreshListenable` 需要 `Listenable`，
/// 而 `sessionProvider` / `forceUpdateGateProvider` 是 Riverpod Notifier。
/// 此類別監聽兩者變化，觸發 `notifyListeners()` 讓 GoRouter 重新執行
/// `redirect`——redirect 讀哪個 provider，這裡就要聽哪個，否則 state 變了
/// 也不會重判（54 號設計書 §7.2：前景恢復重檢靠這條路徑生效）。
class SessionChangeNotifier extends ChangeNotifier {
  SessionChangeNotifier(this._ref) {
    _sub = _ref.listen<SessionModel>(sessionProvider, (previous, next) {
      debugPrint(
        '[SessionChange] loggedIn: ${previous?.isLoggedIn} → '
        '${next.isLoggedIn} (notifying GoRouter)',
      );
      notifyListeners();
    });
    // redirect 讀的是店家的衍生狀態，所以聽衍生的 provider：它們各自重算完才通知，redirect 不會讀到舊值。
    void notifyStore(Object? previous, Object? next) {
      debugPrint('[SessionChange] store state changed (notifying GoRouter)');
      notifyListeners();
    }

    _storeSubs = [
      _ref.listen(needsOnboardingProvider, notifyStore),
      _ref.listen(myStoreSnapshotProvider, notifyStore),
      _ref.listen(storeUnavailableProvider, notifyStore),
    ];
    _forceUpdateSub = _ref.listen<CheckAppVersionResponse>(
      forceUpdateGateProvider,
      (previous, next) {
        if (previous?.updateRequired == next.updateRequired) return;
        debugPrint(
          '[SessionChange] updateRequired: ${previous?.updateRequired} → '
          '${next.updateRequired} (notifying GoRouter)',
        );
        notifyListeners();
      },
    );
  }

  final Ref _ref;
  ProviderSubscription<SessionModel>? _sub;
  List<ProviderSubscription<Object?>> _storeSubs = const [];
  ProviderSubscription<CheckAppVersionResponse>? _forceUpdateSub;

  @override
  void dispose() {
    _sub?.close();
    for (final subscription in _storeSubs) {
      subscription.close();
    }
    _forceUpdateSub?.close();
    super.dispose();
  }
}
