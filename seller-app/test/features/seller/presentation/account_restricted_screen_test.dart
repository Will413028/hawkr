import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/router/redirect_logic.dart';
import 'package:hawkr_seller/core/session/account_restriction.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/presentation/account_restricted_screen.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../../../support/fake_my_store.dart';
import 'package:hawkr_seller/shared/time/taipei_display.dart';

class _Fixed extends AccountRestrictionState {
  _Fixed(this.value);
  final AccountRestriction? value;

  @override
  AccountRestriction? build() => value;
}

class _AuthRecorder extends Fake implements AuthRepository {
  _AuthRecorder(this.events);
  final List<String> events;

  @override
  Future<DeleteAccountResponse_DeletionState> deleteAccount() async {
    events.add('delete');
    return DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED;
  }
}

class _Session extends Session {
  _Session(this.events);
  final List<String> events;

  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);

  @override
  Future<void> logout() async => events.add('logout');
}

Future<List<String>> _pump(
  WidgetTester tester,
  AccountRestriction? restriction,
) async {
  final events = <String>[];
  final router = GoRouter(
    routes: [
      GoRoute(path: '/', builder: (_, _) => const AccountRestrictedScreen()),
      GoRoute(
        path: '/auth',
        builder: (_, _) => const Scaffold(body: Text('AUTH')),
      ),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    TaipeiDisplayScope(
      deviceOffset: const Duration(hours: 8),
      child: ProviderScope(
        overrides: [
          accountRestrictionStateProvider.overrideWith(
            () => _Fixed(restriction),
          ),
          myStoreRepositoryProvider.overrideWithValue(
            FakeMyStoreServer.absent().repository,
          ),
          authRepositoryProvider.overrideWithValue(_AuthRecorder(events)),
          sessionProvider.overrideWith(() => _Session(events)),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp.router(routerConfig: router),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return events;
}

void main() {
  const category = PolicyCategory.POLICY_CATEGORY_PROHIBITED_GOODS;

  testWidgets('顯示標題、說明、目錄文字的原因、伺服器理由與停權日', (tester) async {
    await _pump(
      tester,
      AccountRestriction(
        category: category,
        reason: '重複違規',
        suspendedAt: DateTime.utc(2026, 11, 16, 20),
      ),
    );

    expect(find.text('帳號已被停權'), findsOneWidget);
    expect(find.text('你的帳號目前受平台限制，無法使用賣家功能。'), findsOneWidget);
    final label = category.uiText(UiLocale.zhTW)!;
    expect(label, isNotEmpty);
    expect(find.text('原因：$label'), findsOneWidget);
    expect(find.text('重複違規'), findsOneWidget);
    expect(find.text('停權時間：2026/11/17'), findsOneWidget);
    expect(find.text('刪除帳號'), findsOneWidget);
    expect(find.text('登出'), findsOneWidget);
  });

  testWidgets('UNSPECIFIED 不顯示原因行，空理由與沒有時間也不顯示', (tester) async {
    await _pump(
      tester,
      const AccountRestriction(
        category: PolicyCategory.POLICY_CATEGORY_UNSPECIFIED,
        reason: '',
      ),
    );

    expect(find.textContaining('原因：'), findsNothing);
    expect(find.textContaining('停權時間：'), findsNothing);
    expect(find.text('帳號已被停權'), findsOneWidget);
  });

  testWidgets('受限頁可刪除帳號：確認後刪除並登出', (tester) async {
    final events = await _pump(
      tester,
      const AccountRestriction(category: category, reason: ''),
    );

    await tester.tap(find.byKey(const ValueKey('account-restricted-delete')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('確認刪除'));
    await tester.pumpAndSettle();
    await tester.pump(const Duration(seconds: 6));
    await tester.pumpAndSettle();

    expect(events, ['delete', 'logout']);
    expect(find.text('AUTH'), findsOneWidget);
  });

  group('resolveAuthRedirect — 帳號受限', () {
    String? redirect(String location, {bool restricted = true}) =>
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: false,
          myStore: null,
          location: location,
          accountRestricted: restricted,
        );

    for (final location in [
      '/main/dashboard',
      '/auth',
      AuthRoutes.onboard,
      SellerRoutes.suspensionNotice,
      SellerRoutes.measures,
    ]) {
      test('$location → 導去受限頁', () {
        expect(redirect(location), SellerRoutes.accountRestricted);
      });
    }

    test('受限頁本身不重導', () {
      expect(redirect(SellerRoutes.accountRestricted), isNull);
    });

    test('受限已清除而還在受限頁 → 導回 dashboard', () {
      expect(
        redirect(SellerRoutes.accountRestricted, restricted: false),
        MainRoutes.dashboard,
      );
    });

    test('沒有受限時不導向受限頁', () {
      expect(
        redirect('/main/dashboard', restricted: false),
        isNot(SellerRoutes.accountRestricted),
      );
    });
  });
}
