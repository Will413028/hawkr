import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/presentation/store_unavailable_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/fake_my_store.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _RecordingSession extends Session {
  int logouts = 0;

  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);

  @override
  Future<void> logout() async => logouts++;
}

void main() {
  testWidgets('重試頁可以登出：確認後登出並回到登入頁', (tester) async {
    final session = _RecordingSession();
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, _) => const StoreUnavailableScreen()),
        GoRoute(
          path: '/auth',
          builder: (_, _) => const Scaffold(body: Text('AUTH')),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [sessionProvider.overrideWith(() => session)],
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          builder: (_, _) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.widgetWithText(OutlinedButton, '登出'));
    await tester.pumpAndSettle();
    expect(find.text('要登出賣家帳號嗎？'), findsOneWidget);
    await tester.tap(
      find.descendant(of: find.byType(Dialog), matching: find.text('登出')),
    );
    await tester.pumpAndSettle();

    expect(session.logouts, 1);
    expect(find.text('AUTH'), findsOneWidget);
  });

  testWidgets('顯示無法確認商店狀態的說明與「重試」；重試成功後狀態恢復', (tester) async {
    final server = FakeMyStoreServer.unavailable();
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(server.repository),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(currentStoreProvider.future);
    expect(container.read(storeUnavailableProvider), isTrue);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          builder: (_, _) => const MaterialApp(home: StoreUnavailableScreen()),
        ),
      ),
    );

    expect(find.text('目前無法確認商店狀態，請稍後再試一次。'), findsOneWidget);
    expect(find.text('重試'), findsOneWidget);

    server.failure = null;
    await tester.tap(find.text('重試'));
    await tester.pumpAndSettle();

    expect(server.calls, 2);
    expect(container.read(storeUnavailableProvider), isFalse);
    expect(container.read(sellerStoreIdProvider), 'store-1');
  });
}
