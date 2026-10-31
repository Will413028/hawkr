import 'dart:async';
import 'dart:io';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_ui_text/hawkr_ui_text.dart' show UiLocale;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/auth/data/account_deletion.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:hawkr_buyer/features/notification/data/push/manager.dart';
import 'package:hawkr_buyer/features/settings/presentation/profile_edit_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/firebase_core_mock.dart';
import '../../support/secure_storage_mock.dart';
import '../../support/default_error_text.dart';

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'account-3001', nickname: 'Iris');
}

/// 登入信箱不是個人資料：identity 目前沒有把它給 App，所以不顯示；儲存時不送出。
void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  late ProviderContainer container;
  late List<String> identityCalls;

  Future<List<buyer.UpdateProfileRequest>> pumpScreen(
    WidgetTester tester, {
    FutureOr<DeleteAccountResponse> Function()? deleteAccount,
    FutureOr<GetSessionResponse> Function()? getSession,
    Gender? gender,
    UiLocale? locale,
    Date? birthDate,
  }) async {
    identityCalls = [];
    final updates = <buyer.UpdateProfileRequest>[];
    final repository = AuthRepository(
      IdentityAppServiceClient(
        FakeTransportBuilder()
            .unary(IdentityAppService.deleteAccount, (_, _) {
              identityCalls.add('delete');
              return deleteAccount?.call() ?? DeleteAccountResponse();
            })
            .unary(IdentityAppService.getSession, (_, _) {
              identityCalls.add('getSession');
              return getSession?.call() ??
                  GetSessionResponse(account: Account(id: 'account-3001'));
            })
            .build(),
      ),
      BuyerAppServiceClient(
        FakeTransportBuilder()
            .unary(
              BuyerAppService.getProfile,
              (_, _) => buyer.GetProfileResponse(
                profile: buyer.Profile(
                  id: 'buyer-7002',
                  etag: '3',
                  displayName: 'Iris',
                  phone: '0912345678',
                  gender: gender,
                  birthDate: birthDate,
                ),
              ),
            )
            .unary(BuyerAppService.updateProfile, (request, _) {
              updates.add(request);
              return buyer.UpdateProfileResponse(profile: request.profile);
            })
            .build(),
      ),
      defaults: zhDefaultErrorText,
    );
    container = ProviderContainer(
      overrides: [
        authRepositoryProvider.overrideWith((_) => repository),
        sessionProvider.overrideWith(_ActiveSession.new),
        if (locale != null) uiLocaleProvider.overrideWithValue(locale),
      ],
    );
    addTearDown(container.dispose);
    await container.read(sessionProvider.future);

    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) => const Scaffold(body: Text('home')),
        ),
        GoRoute(path: '/profile', builder: (_, _) => const ProfileEditScreen()),
      ],
    );
    addTearDown(router.dispose);
    await HttpOverrides.runZoned(() async {
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, _) =>
                MaterialApp.router(theme: AppTheme.light, routerConfig: router),
          ),
        ),
      );
      unawaited(router.push('/profile'));
      for (var i = 0; i < 10; i++) {
        await tester.pump(const Duration(milliseconds: 100));
      }
    }, createHttpClient: (_) => throw StateError('External network disabled'));
    return updates;
  }

  Future<void> save(WidgetTester tester) async {
    await tester.tap(find.byKey(const ValueKey('profile-save-primary')));
    for (var i = 0; i < 10; i++) {
      await tester.pump(const Duration(milliseconds: 100));
    }
  }

  testWidgets('identity 沒有把登入信箱給 App：不顯示信箱欄，照樣可以儲存', (tester) async {
    final updates = await pumpScreen(tester);

    expect(find.byKey(const ValueKey('profile-field-登入信箱')), findsNothing);

    await save(tester);

    expect(updates, hasLength(1));
    expect(find.text('home'), findsOneWidget);
    await tester.pump(const Duration(seconds: 5));
  });

  group('性別', () {
    void tallView(WidgetTester tester) {
      tester.view.physicalSize = const Size(800, 1600);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
    }

    Future<void> showGender(WidgetTester tester) => tester.scrollUntilVisible(
      find.byKey(const ValueKey('profile-dropdown-性別')),
      300,
      scrollable: find.byType(Scrollable).first,
    );

    Finder genderSurfaceText(String text) => find.descendant(
      of: find.byKey(const ValueKey('profile-dropdown-性別')),
      matching: find.text(text),
    );

    Future<void> openGender(WidgetTester tester) async {
      await showGender(tester);
      await tester.tap(find.byKey(const ValueKey('profile-dropdown-性別')));
      await tester.pump(const Duration(milliseconds: 100));
    }

    testWidgets('讀到女性就顯示「女性」，不改直接儲存仍送 FEMALE', (tester) async {
      tallView(tester);
      final updates = await pumpScreen(tester, gender: Gender.GENDER_FEMALE);
      await showGender(tester);
      expect(genderSurfaceText('女性'), findsOneWidget);

      await save(tester);

      expect(updates.single.profile.gender, Gender.GENDER_FEMALE);
      expect(updates.single.updateMask.paths, contains('gender'));
      await tester.pump(const Duration(seconds: 5));
    });

    testWidgets('選「尚未設定」後儲存：gender 在 mask 裡而且沒有設值', (tester) async {
      tallView(tester);
      final updates = await pumpScreen(tester, gender: Gender.GENDER_FEMALE);
      await openGender(tester);
      await tester.tap(find.byKey(const ValueKey('app-dropdown-item-0')));
      await tester.pump(const Duration(milliseconds: 100));
      expect(genderSurfaceText('尚未設定'), findsOneWidget);

      await save(tester);

      expect(updates.single.updateMask.paths, contains('gender'));
      expect(updates.single.profile.hasGender(), isFalse);
      await tester.pump(const Duration(seconds: 5));
    });

    testWidgets('沒有填性別：欄位顯示「尚未設定」，打開選單時第一項是已選', (tester) async {
      tallView(tester);
      final semantics = tester.ensureSemantics();
      try {
        await pumpScreen(tester);
        await showGender(tester);
        expect(genderSurfaceText('尚未設定'), findsOneWidget);

        await tester.tap(find.byKey(const ValueKey('profile-dropdown-性別')));
        await tester.pump(const Duration(milliseconds: 100));
        bool selectedAt(int i) => tester
            .getSemantics(find.byKey(ValueKey('app-dropdown-item-$i')))
            .getSemanticsData()
            .flagsCollection
            .isSelected
            .toBoolOrNull()!;
        expect(
          [for (var i = 0; i < 4; i++) selectedAt(i)],
          [true, false, false, false],
        );
        await tester.tap(find.byKey(const ValueKey('app-dropdown-item-0')));
        await tester.pump(const Duration(milliseconds: 100));
        await tester.pump(const Duration(seconds: 5));
      } finally {
        semantics.dispose();
      }
    });

    testWidgets('性別文字來自目錄：英文語系顯示 Female 與 Not set', (tester) async {
      tallView(tester);
      await pumpScreen(
        tester,
        gender: Gender.GENDER_FEMALE,
        locale: UiLocale.en,
      );
      await showGender(tester);
      expect(genderSurfaceText('Female'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('profile-dropdown-性別')));
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.text('Not set'), findsOneWidget);
      expect(find.text('Male'), findsOneWidget);
      expect(find.text('Other'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('app-dropdown-item-0')));
      await tester.pump(const Duration(milliseconds: 100));
      await tester.pump(const Duration(seconds: 5));
    });

    testWidgets('伺服器回 UNSPECIFIED（或不認得的值）時呈現同沒有填', (tester) async {
      tallView(tester);
      await pumpScreen(tester, gender: Gender.GENDER_UNSPECIFIED);
      await showGender(tester);
      expect(genderSurfaceText('尚未設定'), findsOneWidget);
      expect(find.text('未知'), findsNothing);
    });

    testWidgets('頂部沒有「會員等級」那一行', (tester) async {
      await pumpScreen(tester);
      expect(find.text('未設定'), findsNothing);
      expect(find.text('一般會員'), findsNothing);
    });
  });

  group('生日', () {
    testWidgets('讀到完整的生日，不改直接儲存仍送同一天', (tester) async {
      final updates = await pumpScreen(
        tester,
        birthDate: Date(year: 1990, month: 5, day: 20),
      );
      await save(tester);
      final sent = updates.single.profile.birthDate;
      expect([sent.year, sent.month, sent.day], [1990, 5, 20]);
      await tester.pump(const Duration(seconds: 5));
    });

    testWidgets('年月日有缺的生日視為沒有，儲存時不送日期值', (tester) async {
      final updates = await pumpScreen(
        tester,
        birthDate: Date(year: 0, month: 5, day: 20),
      );
      await save(tester);
      expect(updates.single.profile.hasBirthDate(), isFalse);
      await tester.pump(const Duration(seconds: 5));
    });
  });

  group('刪除帳號', () {
    setUp(() {
      mockFirebaseCore();
      PushNotificationManager.e2eDisabled = true;
      addTearDown(() => PushNotificationManager.e2eDisabled = false);
      SharedPreferences.setMockInitialValues({});
      mockSecureStorage({});
    });

    Future<void> settle(WidgetTester tester) async {
      for (var i = 0; i < 40; i++) {
        await tester.pump(const Duration(milliseconds: 100));
      }
    }

    Future<void> showSection(WidgetTester tester) async {
      await tester.scrollUntilVisible(
        find.byKey(const ValueKey('profile-delete-account')),
        300,
        scrollable: find.byType(Scrollable).first,
      );
    }

    Future<void> confirmDeletion(WidgetTester tester) async {
      await showSection(tester);
      await tester.tap(find.byKey(const ValueKey('profile-delete-account')));
      await settle(tester);
      await tester.tap(find.text('確認刪除'));
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 200)),
      );
      await settle(tester);
    }

    testWidgets('入口在所有表單欄位之後，對話框說明刪除的後果', (tester) async {
      await pumpScreen(tester);

      await showSection(tester);

      final job = tester.getTopLeft(
        find.byKey(const ValueKey('profile-field-職業')),
      );
      final section = tester.getTopLeft(
        find.byKey(const ValueKey('profile-section-刪除帳號')),
      );
      expect(section.dy, greaterThan(job.dy));
      expect(find.text('刪除後帳號資料會被移除，無法復原。'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('profile-delete-account')));
      await settle(tester);

      expect(
        find.text(
          '刪除後帳號資料會被移除，無法復原。還有未完成的訂單時，帳號會在最後一筆訂單完成後刪除，期間無法新增收藏與追蹤。\n\n若你有開設商店，商店也會一起關閉，商店與商品不再公開，賣家身分會移除。有未結訂單時，帳號與商店會在訂單結清後才刪除，期間商店不能自行關閉或換店。',
        ),
        findsOneWidget,
      );
    });

    testWidgets('取消：不呼叫 DeleteAccount', (tester) async {
      await pumpScreen(tester);
      await showSection(tester);
      await tester.tap(find.byKey(const ValueKey('profile-delete-account')));
      await settle(tester);

      await tester.tap(find.text('取消'));
      await settle(tester);

      expect(identityCalls, isEmpty);
      expect(container.read(sessionProvider).value, isNotNull);
    });

    testWidgets('DELETED：登出並回首頁', (tester) async {
      await pumpScreen(
        tester,
        deleteAccount: () => DeleteAccountResponse(
          state: DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED,
        ),
      );

      await confirmDeletion(tester);

      expect(identityCalls, ['delete']);
      expect(container.read(sessionProvider).value, isNull);
      expect(find.text('home'), findsOneWidget);
    });

    testWidgets('PENDING_ORDERS：顯示處理中並維持登入', (tester) async {
      await pumpScreen(
        tester,
        deleteAccount: () => DeleteAccountResponse(
          state:
              DeleteAccountResponse_DeletionState.DELETION_STATE_PENDING_ORDERS,
        ),
      );

      await confirmDeletion(tester);

      expect(
        find.text(accountPendingOrdersMessage, skipOffstage: false),
        findsOneWidget,
      );
      expect(container.read(sessionProvider).value, isNotNull);
    });

    testWidgets('其他錯誤：顯示失敗訊息並維持登入', (tester) async {
      await pumpScreen(
        tester,
        deleteAccount: () => throw ConnectException(Code.internal, 'boom'),
      );

      await confirmDeletion(tester);

      expect(
        find.byKey(
          const ValueKey('profile-delete-account-message'),
          skipOffstage: false,
        ),
        findsOneWidget,
      );
      expect(container.read(sessionProvider).value, isNotNull);
    });

    testWidgets('沒有回應且 GetSession unauthenticated：視為已刪除而登出', (tester) async {
      await pumpScreen(
        tester,
        deleteAccount: () =>
            throw ConnectException.from(const SocketException('offline')),
        getSession: () =>
            throw ConnectException(Code.unauthenticated, 'expired'),
      );

      await confirmDeletion(tester);

      expect(identityCalls, ['delete', 'getSession']);
      expect(container.read(sessionProvider).value, isNull);
      expect(find.text('home'), findsOneWidget);
    });

    testWidgets('沒有回應但 GetSession 有回應：帳號尚未刪除', (tester) async {
      await pumpScreen(
        tester,
        deleteAccount: () =>
            throw ConnectException.from(const SocketException('offline')),
      );

      await confirmDeletion(tester);

      expect(
        find.text(accountNotDeletedMessage, skipOffstage: false),
        findsOneWidget,
      );
      expect(container.read(sessionProvider).value, isNotNull);
    });
  });
}
