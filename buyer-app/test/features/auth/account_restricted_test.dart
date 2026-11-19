import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart'
    as identity;
import 'package:hawkr_buyer/core/session/account_restriction.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/session/suspended_action_listener.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:hawkr_buyer/features/auth/presentation/account_restricted_screen.dart';
import 'package:hawkr_buyer/shared/widgets/account_restricted_banner.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _Session extends Session {
  int logouts = 0;

  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'acc-1', nickname: 'n');

  @override
  Future<void> logout() async {
    logouts++;
    state = const AsyncData(null);
    ref.read(accountRestrictionProvider.notifier).clear();
  }
}

class _Auth extends Fake implements AuthRepository {
  int deletes = 0;

  @override
  Future<identity.DeleteAccountResponse_DeletionState> deleteAccount() async {
    deletes++;
    return identity.DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED;
  }
}

const _suspension = AccountSuspension(
  category: PolicyCategory.POLICY_CATEGORY_SPAM,
  reason: '多次張貼廣告',
);

void main() {
  Future<({ProviderContainer c, _Auth auth, _Session session})> pump(
    WidgetTester tester, {
    AccountSuspension? suspension = _suspension,
  }) async {
    final auth = _Auth();
    final session = _Session();
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => session),
        authRepositoryProvider.overrideWithValue(auth),
      ],
    );
    addTearDown(c.dispose);
    await c.read(sessionProvider.future);
    c.read(accountRestrictionProvider.notifier).apply(suspension);
    final router = GoRouter(
      initialLocation: AccountRestrictedScreen.path,
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) => const Scaffold(body: Text('首頁')),
        ),
        GoRoute(
          path: AccountRestrictedScreen.path,
          builder: (_, _) => const AccountRestrictedScreen(),
        ),
      ],
    );
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: c,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, _) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return (c: c, auth: auth, session: session);
  }

  testWidgets('狀態頁：類別取自目錄、說明、停權期間的句子', (tester) async {
    await pump(tester);

    expect(find.text('帳號受限'), findsOneWidget);
    expect(find.text('垃圾訊息'), findsOneWidget);
    expect(find.text('多次張貼廣告'), findsOneWidget);
    expect(find.text(AccountRestrictedScreen.browseNotice), findsOneWidget);
    expect(find.text('停權期間可以瀏覽，無法下單、留言、按讚、追蹤或修改資料。'), findsOneWidget);
  });

  testWidgets('狀態頁：類別 UNSPECIFIED 與空說明不顯示', (tester) async {
    await pump(tester, suspension: const AccountSuspension.unknown());

    expect(find.text('類別'), findsNothing);
    expect(find.text('說明'), findsNothing);
    expect(find.text(AccountRestrictedScreen.browseNotice), findsOneWidget);
  });

  testWidgets('狀態頁可刪除帳號：確認後呼叫 DeleteAccount 並登出', (tester) async {
    final h = await pump(tester);

    await tester.tap(find.byKey(const ValueKey('account-restricted-delete')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('確認刪除'));
    await tester.pumpAndSettle();

    expect(h.auth.deletes, 1);
    expect(h.session.logouts, 1);
    expect(find.text('首頁'), findsOneWidget);
  });

  testWidgets('狀態頁刪除帳號：取消不呼叫 DeleteAccount', (tester) async {
    final h = await pump(tester);

    await tester.tap(find.byKey(const ValueKey('account-restricted-delete')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('取消'));
    await tester.pumpAndSettle();

    expect(h.auth.deletes, 0);
    expect(h.session.logouts, 0);
  });

  testWidgets('狀態頁可登出', (tester) async {
    final h = await pump(tester);

    await tester.tap(find.byKey(const ValueKey('account-restricted-sign-out')));
    await tester.pumpAndSettle();

    expect(h.session.logouts, 1);
    expect(h.auth.deletes, 0);
    expect(find.text('首頁'), findsOneWidget);
  });

  testWidgets('橫幅：文案與點擊', (tester) async {
    var taps = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: AccountRestrictedBanner(onTap: () => taps++)),
      ),
    );

    expect(find.text('帳號已停權，部分功能受限'), findsOneWidget);
    await tester.tap(find.byType(AccountRestrictedBanner));
    expect(taps, 1);
  });

  testWidgets('寫入被停權擋下：bottom sheet 提示，「查看詳情」開狀態頁', (tester) async {
    final key = GlobalKey<NavigatorState>();
    String? opened;
    final c = ProviderContainer();
    addTearDown(c.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: c,
        child: MaterialApp(
          navigatorKey: key,
          builder: (_, child) => SuspendedActionListener(
            navigatorKey: key,
            onViewDetails: () => opened = AccountRestrictedScreen.path,
            child: child!,
          ),
          home: const Scaffold(body: Text('頁面')),
        ),
      ),
    );

    c.read(suspendedActionNoticeProvider.notifier).raise();
    await tester.pumpAndSettle();

    expect(find.text('帳號停權中，無法執行此動作'), findsOneWidget);
    await tester.tap(
      find.byKey(const ValueKey('suspended-action-view-details')),
    );
    await tester.pumpAndSettle();

    expect(opened, AccountRestrictedScreen.path);
    expect(find.text('帳號停權中，無法執行此動作'), findsNothing);
  });
}
