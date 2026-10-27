import 'package:hawkr_seller/core/router/pending_wallet_focus.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class _Switchable extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);

  void switchTo(String? accountId) => state = accountId == null
      ? const SessionModel()
      : SessionModel(accountId: accountId, isLoggedIn: true);
}

void main() {
  for (final next in <String?>['acc-2', null]) {
    test('沒套用的展開意圖不會帶到下一個帳號（${next ?? '登出'}）', () {
      final container = ProviderContainer(
        overrides: [sessionProvider.overrideWith(_Switchable.new)],
      );
      addTearDown(container.dispose);
      final sub = container.listen(pendingWalletFocusProvider, (_, _) {});
      addTearDown(sub.close);
      container
          .read(pendingWalletFocusProvider.notifier)
          .request(WalletSections.foodRegistration);
      expect(
        container.read(pendingWalletFocusProvider),
        WalletSections.foodRegistration,
      );

      (container.read(sessionProvider.notifier) as _Switchable).switchTo(next);

      expect(container.read(pendingWalletFocusProvider), isNull);
    });
  }
}
